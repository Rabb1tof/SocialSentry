package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/rabb1tof/socialsentry/backend/internal/config"
	"github.com/rabb1tof/socialsentry/backend/internal/db/generated"
	"github.com/rabb1tof/socialsentry/backend/internal/domain"
	"github.com/rabb1tof/socialsentry/backend/internal/handler"
	"github.com/rabb1tof/socialsentry/backend/internal/middleware"
	"github.com/rabb1tof/socialsentry/backend/internal/platform/instagram"
	"github.com/rabb1tof/socialsentry/backend/internal/queue"
	"github.com/rabb1tof/socialsentry/backend/internal/repository"
	"github.com/rabb1tof/socialsentry/backend/internal/service"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	logger := newLogger(cfg.Server.Environment)
	defer func() { _ = logger.Sync() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Postgres
	pool, err := pgxpool.New(ctx, cfg.DB.URL)
	if err != nil {
		return fmt.Errorf("pgxpool: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("postgres ping: %w", err)
	}
	logger.Info("postgres connected")

	// Redis
	redisOpt, err := redis.ParseURL(cfg.Redis.URL)
	if err != nil {
		return fmt.Errorf("redis url: %w", err)
	}
	if cfg.Redis.Password != "" {
		redisOpt.Password = cfg.Redis.Password
	}
	rdb := redis.NewClient(redisOpt)
	defer func() { _ = rdb.Close() }()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}
	logger.Info("redis connected")

	// Composition root
	queries := generated.New(pool)
	userRepo := repository.NewUserRepo(queries)
	refreshRepo := repository.NewRefreshTokenRepo(queries)
	subRepo := repository.NewSubscriptionRepo(queries)
	accountRepo := repository.NewAccountRepo(queries)
	triggerRepo := repository.NewTriggerRepo(queries)
	logRepo := repository.NewLogRepo(queries)

	pubsub := queue.NewPublisher(rdb, logger)

	authSvc := service.NewAuthService(userRepo, refreshRepo, cfg.JWT)
	adminSvc := service.NewAdminService(userRepo, queries)
	subSvc := service.NewSubscriptionService(subRepo, userRepo)
	accountSvc := service.NewAccountService(accountRepo, subRepo, cfg.Encryption.Key, pubsub)
	triggerSvc := service.NewTriggerService(triggerRepo, accountRepo, subRepo, pubsub)
	settingsSvc := service.NewSettingsService(queries, rdb, pubsub, logger)

	igClient := instagram.NewClient(rdb)
	queueClient, err := queue.NewClient(cfg.Redis.URL)
	if err != nil {
		return fmt.Errorf("queue client: %w", err)
	}
	defer func() { _ = queueClient.Close() }()

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	authH := handler.NewAuthHandler(authSvc, userRepo, cfg.Server.Environment == "production")
	adminH := handler.NewAdminHandler(adminSvc, subSvc)
	settingsH := handler.NewSettingsHandler(settingsSvc)
	accountH := handler.NewAccountHandler(accountSvc)
	triggerH := handler.NewTriggerHandler(triggerSvc, accountSvc, logRepo)
	igConnectH := handler.NewInstagramConnectHandler(*cfg, igClient, accountSvc, rdb, frontendURL, logger)
	vkConnectH := handler.NewVKConnectHandler(accountSvc, rdb, cfg.VK.APIVersion, logger)
	webhookH := handler.NewWebhookHandler(cfg.Meta.WebhookVerifyToken, cfg.Meta.WebhookAppSecret, queueClient, logger)

	// Realtime: a Hub subscribes once to the trigger-fired Redis channel (published by the
	// worker on every log write) and fans events out to each user's WebSocket clients.
	wsHub := handler.NewHub(rdb, logger)
	go wsHub.Run(ctx)
	wsH := handler.NewWSHandler(wsHub, accountRepo, cfg.JWT.Secret, logger)

	if cfg.Server.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Logger(logger))

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Public Meta webhook endpoints (no JWT — signature verified instead).
	r.GET("/webhooks/instagram", webhookH.Verify)
	r.POST("/webhooks/instagram", webhookH.Receive)

	// Public Instagram OAuth callback (state-based, no JWT).
	r.GET("/api/v1/accounts/instagram/callback", igConnectH.Callback)

	// Realtime WebSocket. Authenticated via ?token=<jwt> (browsers can't set headers on
	// the WS handshake); the handler scopes events to the user's own accounts.
	r.GET("/ws", wsH.Serve)

	requireAuth := middleware.RequireAuth(cfg.JWT.Secret)
	requireAdmin := middleware.RequireAdmin()
	requireSub := middleware.RequireActiveSubscription(subSvc)

	api := r.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", middleware.RateLimit(rdb, "register", 5, time.Minute), authH.Register)
			auth.POST("/login", middleware.RateLimit(rdb, "login", 5, time.Minute), authH.Login)
			auth.POST("/refresh", authH.Refresh)
			auth.POST("/logout", requireAuth, authH.Logout)
		}
		api.GET("/me", requireAuth, authH.Me)
		api.GET("/subscription", requireAuth, adminH.GetMySubscription)
		// Platform availability — auth-only, drives the connect UI gating.
		api.GET("/platform-settings", requireAuth, settingsH.PlatformAvailability)
		// Recent activity across the user's accounts — auth-only, drives the dashboard feed.
		api.GET("/logs/recent", requireAuth, triggerH.RecentLogs)

		// Read-only account views — auth-only, no subscription gate.
		api.GET("/accounts", requireAuth, accountH.List)
		api.GET("/accounts/:id", requireAuth, accountH.Get)
		api.GET("/accounts/:id/triggers", requireAuth, triggerH.List)
		api.GET("/accounts/:id/triggers/:tid", requireAuth, triggerH.Get)
		// Test trigger is auth-only too — users without an active sub can still
		// preview triggers in the editor before purchasing.
		api.POST("/accounts/:id/triggers/:tid/test", requireAuth, triggerH.Test)

		// Mutations + log views — subscription required.
		gated := api.Group("", requireAuth, requireSub)
		{
			gated.POST("/accounts/instagram/connect", middleware.RequirePlatformEnabled(settingsSvc, domain.PlatformInstagram), igConnectH.Connect)
			gated.POST("/accounts/vk/connect", middleware.RequirePlatformEnabled(settingsSvc, domain.PlatformVK), vkConnectH.Connect)
			gated.DELETE("/accounts/:id", accountH.Delete)
			gated.PATCH("/accounts/:id/status", accountH.PatchStatus)
			gated.GET("/accounts/:id/logs", triggerH.AccountLogs)

			gated.POST("/accounts/:id/triggers", triggerH.Create)
			gated.PUT("/accounts/:id/triggers/:tid", triggerH.Update)
			gated.DELETE("/accounts/:id/triggers/:tid", triggerH.Delete)
			gated.PATCH("/accounts/:id/triggers/:tid/toggle", triggerH.Toggle)
			gated.GET("/accounts/:id/triggers/:tid/logs", triggerH.Logs)
		}

		admin := api.Group("/admin", requireAuth, requireAdmin)
		{
			admin.GET("/users", adminH.ListUsers)
			admin.POST("/users", adminH.CreateUser)
			admin.GET("/users/:id", adminH.GetUser)
			admin.PATCH("/users/:id", adminH.PatchUser)
			admin.GET("/subscriptions", adminH.ListSubscriptions)
			admin.POST("/subscriptions", adminH.GrantSubscription)
			admin.PATCH("/subscriptions/:id", adminH.UpdateSubscription)
			admin.DELETE("/subscriptions/:id", adminH.RevokeSubscription)
			admin.GET("/stats", adminH.GetStats)
			admin.GET("/platform-settings", settingsH.ListPlatformSettings)
			admin.PATCH("/platform-settings/:platform", settingsH.SetPlatformEnabled)
		}
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Server.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("api server starting", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("server failed", zap.Error(err))
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down")
	shutdownCtx, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	}
	logger.Info("server stopped")
	return nil
}

func newLogger(env string) *zap.Logger {
	if env == "production" {
		l, _ := zap.NewProduction()
		return l
	}
	l, _ := zap.NewDevelopment()
	return l
}
