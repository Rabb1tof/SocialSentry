// Package config loads SocialSentry configuration from environment variables.
package config

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	DB         DBConfig
	Redis      RedisConfig
	JWT        JWTConfig
	Encryption EncryptionConfig
	Meta       MetaConfig
	VK         VKConfig
	Server     ServerConfig
}

type DBConfig struct {
	URL string
}

type RedisConfig struct {
	URL      string
	Password string
}

type JWTConfig struct {
	Secret     []byte
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

type EncryptionConfig struct {
	Key []byte
}

type MetaConfig struct {
	AppID     string
	AppSecret string
	// WebhookAppSecret verifies inbound webhook HMAC signatures. Meta signs each
	// webhook with the secret of the app that OWNS the subscription, which is not
	// necessarily the app used for OAuth (AppSecret). When the Instagram webhook
	// subscription lives under a different app than OAuth, set META_WEBHOOK_APP_SECRET
	// to that app's secret. Defaults to AppSecret when unset (single-app setups).
	WebhookAppSecret   string
	WebhookVerifyToken string
	CallbackURL        string
}

type VKConfig struct {
	APIVersion string
}

type ServerConfig struct {
	Port        string
	Environment string
	LogLevel    string
}

// Load reads configuration from environment variables.
// Defaults are applied for non-critical values; required secrets cause an error if missing.
func Load() (*Config, error) {
	v := viper.New()
	v.AutomaticEnv()

	v.SetDefault("JWT_ACCESS_TTL", "15m")
	v.SetDefault("JWT_REFRESH_TTL", "168h")
	v.SetDefault("PORT", "8080")
	v.SetDefault("ENVIRONMENT", "development")
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("VK_API_VERSION", "5.199")

	accessTTL, err := time.ParseDuration(v.GetString("JWT_ACCESS_TTL"))
	if err != nil {
		return nil, fmt.Errorf("config.Load: JWT_ACCESS_TTL: %w", err)
	}
	refreshTTL, err := time.ParseDuration(v.GetString("JWT_REFRESH_TTL"))
	if err != nil {
		return nil, fmt.Errorf("config.Load: JWT_REFRESH_TTL: %w", err)
	}

	jwtSecret := v.GetString("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("config.Load: JWT_SECRET is required")
	}

	encKeyHex := v.GetString("ENCRYPTION_KEY")
	if encKeyHex == "" {
		return nil, fmt.Errorf("config.Load: ENCRYPTION_KEY is required")
	}
	encKey, err := hex.DecodeString(encKeyHex)
	if err != nil {
		return nil, fmt.Errorf("config.Load: ENCRYPTION_KEY must be hex: %w", err)
	}
	if len(encKey) != 32 {
		return nil, fmt.Errorf("config.Load: ENCRYPTION_KEY must be 32 bytes (64 hex chars), got %d bytes", len(encKey))
	}

	dbURL := v.GetString("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("config.Load: DATABASE_URL is required")
	}

	redisURL := v.GetString("REDIS_URL")
	if redisURL == "" {
		return nil, fmt.Errorf("config.Load: REDIS_URL is required")
	}

	return &Config{
		DB:    DBConfig{URL: dbURL},
		Redis: RedisConfig{URL: redisURL, Password: v.GetString("REDIS_PASSWORD")},
		JWT: JWTConfig{
			Secret:     []byte(jwtSecret),
			AccessTTL:  accessTTL,
			RefreshTTL: refreshTTL,
		},
		Encryption: EncryptionConfig{Key: encKey},
		Meta: MetaConfig{
			AppID:              v.GetString("META_APP_ID"),
			AppSecret:          v.GetString("META_APP_SECRET"),
			WebhookAppSecret:   firstNonEmpty(v.GetString("META_WEBHOOK_APP_SECRET"), v.GetString("META_APP_SECRET")),
			WebhookVerifyToken: v.GetString("META_WEBHOOK_VERIFY_TOKEN"),
			CallbackURL:        v.GetString("META_CALLBACK_URL"),
		},
		VK: VKConfig{APIVersion: v.GetString("VK_API_VERSION")},
		Server: ServerConfig{
			Port:        v.GetString("PORT"),
			Environment: v.GetString("ENVIRONMENT"),
			LogLevel:    v.GetString("LOG_LEVEL"),
		},
	}, nil
}

// firstNonEmpty returns the first non-empty string from the arguments, or "".
func firstNonEmpty(vals ...string) string {
	for _, s := range vals {
		if s != "" {
			return s
		}
	}
	return ""
}
