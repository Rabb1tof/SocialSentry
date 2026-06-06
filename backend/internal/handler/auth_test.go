package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/rabb1tof/socialsentry/backend/internal/config"
	"github.com/rabb1tof/socialsentry/backend/internal/db/generated"
	"github.com/rabb1tof/socialsentry/backend/internal/handler"
	"github.com/rabb1tof/socialsentry/backend/internal/middleware"
	"github.com/rabb1tof/socialsentry/backend/internal/repository"
	"github.com/rabb1tof/socialsentry/backend/internal/service"
)

// TestAuthFlow walks register → login → /me → refresh → logout → refresh-after-logout.
// Requires TEST_DATABASE_URL and TEST_REDIS_URL to point at a running Postgres + Redis;
// the test truncates `users` (CASCADE) and flushes Redis keys before running.
func TestAuthFlow(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	redisURL := os.Getenv("TEST_REDIS_URL")
	if dbURL == "" || redisURL == "" {
		t.Skip("TEST_DATABASE_URL and TEST_REDIS_URL must be set; skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE users CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	rOpt, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("redis url: %v", err)
	}
	rdb := redis.NewClient(rOpt)
	defer func() { _ = rdb.Close() }()
	_ = rdb.FlushDB(ctx).Err()

	queries := generated.New(pool)
	userRepo := repository.NewUserRepo(queries)
	refreshRepo := repository.NewRefreshTokenRepo(queries)
	jwtCfg := config.JWTConfig{
		Secret:     []byte("integration-test-secret-32-bytes!"),
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 7 * 24 * time.Hour,
	}
	authSvc := service.NewAuthService(userRepo, refreshRepo, jwtCfg)
	authH := handler.NewAuthHandler(authSvc, userRepo, false)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api/v1")
	auth := api.Group("/auth")
	auth.POST("/register", authH.Register)
	auth.POST("/login", authH.Login)
	auth.POST("/refresh", authH.Refresh)
	auth.POST("/logout", middleware.RequireAuth(jwtCfg.Secret), authH.Logout)
	api.GET("/me", middleware.RequireAuth(jwtCfg.Secret), authH.Me)

	srv := httptest.NewServer(r)
	defer srv.Close()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 5 * time.Second}

	const (
		email    = "integration@example.com"
		password = "longpass1"
	)

	// 1. Register
	if resp := doJSON(t, client, http.MethodPost, srv.URL+"/api/v1/auth/register", map[string]string{
		"email": email, "password": password,
	}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: got %d, want 201", resp.StatusCode)
	}

	// 2. Login
	var loginBody struct {
		Data struct {
			AccessToken string `json:"access_token"`
			User        struct {
				Email string `json:"email"`
			} `json:"user"`
		} `json:"data"`
	}
	decodeBody(t, doJSON(t, client, http.MethodPost, srv.URL+"/api/v1/auth/login", map[string]string{
		"email": email, "password": password,
	}), &loginBody)
	access := loginBody.Data.AccessToken
	if access == "" {
		t.Fatal("login: empty access_token")
	}
	if loginBody.Data.User.Email != email {
		t.Fatalf("login: user.email=%q want %q", loginBody.Data.User.Email, email)
	}

	// 3. /me with access token
	var meBody struct {
		Data struct {
			Email string `json:"email"`
		} `json:"data"`
	}
	decodeBody(t, doBearer(t, client, http.MethodGet, srv.URL+"/api/v1/me", access, nil), &meBody)
	if meBody.Data.Email != email {
		t.Fatalf("me: email=%q want %q", meBody.Data.Email, email)
	}

	// 4. Refresh via cookie
	var refBody struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	decodeBody(t, doJSON(t, client, http.MethodPost, srv.URL+"/api/v1/auth/refresh", nil), &refBody)
	if refBody.Data.AccessToken == "" {
		t.Fatal("refresh: empty access_token")
	}
	newAccess := refBody.Data.AccessToken

	// 5. Logout (uses the new access token)
	if resp := doBearer(t, client, http.MethodPost, srv.URL+"/api/v1/auth/logout", newAccess, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("logout: got %d, want 204", resp.StatusCode)
	}

	// 6. Refresh again — must fail because the refresh row was rotated then deleted at logout.
	// Note: the new refresh cookie set by step 4 was deleted at logout. Cookie jar reflects that.
	if resp := doJSON(t, client, http.MethodPost, srv.URL+"/api/v1/auth/refresh", nil); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("refresh after logout: got %d, want 401", resp.StatusCode)
	}
}

func doJSON(t *testing.T, c *http.Client, method, url string, body interface{}) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

func doBearer(t *testing.T, c *http.Client, method, url, token string, body interface{}) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

func decodeBody(t *testing.T, resp *http.Response, out interface{}) {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("decode body: %v (status %d)", err, resp.StatusCode)
	}
}
