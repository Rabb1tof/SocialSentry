package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/rabb1tof/socialsentry/backend/internal/queue"
	"github.com/rabb1tof/socialsentry/backend/internal/repository"
	jwtpkg "github.com/rabb1tof/socialsentry/backend/pkg/jwt"
)

const (
	wsWriteWait  = 10 * time.Second
	wsPongWait   = 60 * time.Second
	wsPingPeriod = 50 * time.Second // must be < wsPongWait
	wsSendBuffer = 32
)

// wsClient is one connected browser. accounts is the set of account IDs the user owns,
// captured at connect time; the hub only forwards events for those accounts.
type wsClient struct {
	accounts map[string]bool
	send     chan []byte
}

// Hub fans Redis trigger-fired events out to the WebSocket clients that own the account.
// One Hub.Run goroutine holds a single Redis subscription for the whole API process.
type Hub struct {
	rdb     *redis.Client
	logger  *zap.Logger
	mu      sync.RWMutex
	clients map[*wsClient]struct{}
}

// NewHub wires the hub.
func NewHub(rdb *redis.Client, logger *zap.Logger) *Hub {
	return &Hub{rdb: rdb, logger: logger, clients: make(map[*wsClient]struct{})}
}

// Run subscribes to the realtime channel and forwards each event to matching clients.
// Blocks until ctx is cancelled; intended to run in its own goroutine.
func (h *Hub) Run(ctx context.Context) {
	sub := h.rdb.Subscribe(ctx, queue.ChannelTriggerFired)
	defer func() { _ = sub.Close() }()
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var evt queue.TriggerFiredEvent
			if err := json.Unmarshal([]byte(msg.Payload), &evt); err != nil {
				continue
			}
			h.mu.RLock()
			for c := range h.clients {
				if !c.accounts[evt.AccountID] {
					continue
				}
				select {
				case c.send <- []byte(msg.Payload):
				default: // slow client — drop the event rather than block the hub
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) register(c *wsClient) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

// unregister removes the client and closes its send channel exactly once.
func (h *Hub) unregister(c *wsClient) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
}

// WSHandler authenticates the WebSocket handshake and bridges one connection to the Hub.
type WSHandler struct {
	hub      *Hub
	accounts repository.AccountRepo
	secret   []byte
	logger   *zap.Logger
	upgrader websocket.Upgrader
}

// NewWSHandler wires the handler.
func NewWSHandler(hub *Hub, accounts repository.AccountRepo, secret []byte, logger *zap.Logger) *WSHandler {
	return &WSHandler{
		hub:      hub,
		accounts: accounts,
		secret:   secret,
		logger:   logger,
		// Auth is via the ?token= query param (a signed JWT), not cookies, so cross-origin
		// CSRF isn't a concern — accept any origin.
		upgrader: websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
	}
}

// Serve handles GET /ws?token=<jwt>. Browsers can't set Authorization headers on a
// WebSocket handshake, so the access token rides in the query string.
func (h *WSHandler) Serve(c *gin.Context) {
	claims, err := jwtpkg.Parse(c.Query("token"), h.secret)
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	accs, err := h.accounts.ListByUser(c.Request.Context(), claims.UserID)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	accSet := make(map[string]bool, len(accs))
	for _, a := range accs {
		accSet[a.ID] = true
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return // Upgrade already wrote the HTTP error
	}
	client := &wsClient{accounts: accSet, send: make(chan []byte, wsSendBuffer)}
	h.hub.register(client)

	go h.writePump(conn, client)
	h.readPump(conn, client)
}

// readPump blocks reading control frames until the client disconnects, then unregisters.
// We don't expect inbound application messages — this exists to drive pong deadlines and
// detect close.
func (h *WSHandler) readPump(conn *websocket.Conn, client *wsClient) {
	defer func() {
		h.hub.unregister(client)
		_ = conn.Close()
	}()
	conn.SetReadLimit(512)
	_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

// writePump forwards hub events to the socket and sends periodic pings. Closing the conn
// on exit unblocks readPump, which owns unregistration.
func (h *WSHandler) writePump(conn *websocket.Conn, client *wsClient) {
	ticker := time.NewTicker(wsPingPeriod)
	defer func() {
		ticker.Stop()
		_ = conn.Close()
	}()
	for {
		select {
		case msg, ok := <-client.send:
			_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if !ok { // hub closed the channel — client was unregistered
				_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
