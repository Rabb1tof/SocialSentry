package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/rabb1tof/socialsentry/backend/internal/config"
	"github.com/rabb1tof/socialsentry/backend/internal/db/generated"
	"github.com/rabb1tof/socialsentry/backend/internal/domain"
	"github.com/rabb1tof/socialsentry/backend/internal/engine"
	"github.com/rabb1tof/socialsentry/backend/internal/platform/instagram"
	"github.com/rabb1tof/socialsentry/backend/internal/platform/vk"
	"github.com/rabb1tof/socialsentry/backend/internal/queue"
	"github.com/rabb1tof/socialsentry/backend/internal/queue/handlers"
	"github.com/rabb1tof/socialsentry/backend/internal/repository"
	"github.com/rabb1tof/socialsentry/backend/internal/service"
)

// subscriptionChecker implements engine.SubscriptionChecker by dispatching on platform:
// VK uses groups.isMember; Instagram uses the messaging is_user_follow_business field. The
// caller must pre-decrypt account.AccessToken (the handlers do this before reaching the gate).
type subscriptionChecker struct {
	vk     *vk.SubscriptionChecker
	ig     *instagram.Client
	logger *zap.Logger
}

func (c *subscriptionChecker) IsSubscribed(ctx context.Context, account domain.ConnectedAccount, senderID string) (bool, error) {
	var (
		sub bool
		err error
	)
	switch account.Platform {
	case domain.PlatformVK:
		sub, err = c.vk.IsSubscribed(ctx, account, senderID)
	case domain.PlatformInstagram:
		sub, err = c.ig.IsFollower(ctx, senderID, account.AccessToken)
	default:
		return false, fmt.Errorf("subscriptionChecker: unsupported platform %q", account.Platform)
	}
	c.logger.Debug("subscription check",
		zap.String("platform", account.Platform),
		zap.String("sender_id", senderID),
		zap.Bool("subscribed", sub),
		zap.Error(err),
	)
	return sub, err
}

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

	var logger *zap.Logger
	if cfg.Server.Environment == "production" {
		logger, _ = zap.NewProduction()
	} else {
		logger, _ = zap.NewDevelopment()
	}
	defer func() { _ = logger.Sync() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DB.URL)
	if err != nil {
		return fmt.Errorf("pgxpool: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("postgres ping: %w", err)
	}

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

	queries := generated.New(pool)
	subRepo := repository.NewSubscriptionRepo(queries)
	accountRepo := repository.NewAccountRepo(queries)
	triggerRepo := repository.NewTriggerRepo(queries)
	logRepo := repository.NewLogRepo(queries)

	pubsub := queue.NewPublisher(rdb, logger)
	// Wrap the log repo so every trigger-log write also emits a realtime event over Redis
	// for the API's WebSocket hub. Both the IG handler and the VK dispatcher write through
	// this, so one decorator covers both platforms.
	logRepo = queue.NewPublishingLogRepo(logRepo, pubsub)
	accountSvc := service.NewAccountService(accountRepo, subRepo, cfg.Encryption.Key, pubsub)
	settingsSvc := service.NewSettingsService(queries, rdb, pubsub, logger)
	igClient := instagram.NewClient(rdb)
	// Subscription gate: VK uses groups.isMember, Instagram uses the messaging
	// is_user_follow_business field. The caller pre-decrypts account.AccessToken.
	subChecker := &subscriptionChecker{
		vk:     vk.NewSubscriptionChecker(rdb, cfg.VK.APIVersion),
		ig:     igClient,
		logger: logger,
	}
	matcher := engine.NewTriggerMatcher(triggerRepo, rdb, subChecker)

	asynqOpt := asynqRedisOpt(cfg.Redis.URL, cfg.Redis.Password)
	srv := asynq.NewServer(asynqOpt, asynq.Config{
		Concurrency: 5,
		Queues:      map[string]int{"default": 5},
		Logger:      zapAsynqLogger{logger: logger},
	})

	mux := asynq.NewServeMux()
	igHandler := handlers.NewInstagramHandler(accountRepo, logRepo, matcher, igClient, accountSvc, settingsSvc, logger)
	mux.HandleFunc(queue.TaskInstagramEvent, igHandler.Handle)

	// Daily maintenance jobs: IG token refresh + log retention pruning.
	maint := handlers.NewMaintenanceHandler(accountRepo, queries, accountSvc, igClient, cfg.Meta, logger)
	mux.HandleFunc(handlers.TaskRefreshIGTokens, maint.RefreshIGTokens)
	mux.HandleFunc(handlers.TaskLogRetention, maint.LogRetention)

	// VK uses a long-poll WorkerManager rather than Asynq, so it runs alongside the asynq server.
	vkDispatcher := vk.NewDispatcher(accountRepo, logRepo, matcher, accountSvc, rdb, cfg.VK.APIVersion, logger)
	vkManager := vk.NewWorkerManager(accountRepo, vkDispatcher, matcher, settingsSvc, rdb, logger)

	// asynq.Scheduler fires the periodic tasks above. Cron spec uses asynq's robfig/cron syntax.
	scheduler := asynq.NewScheduler(asynqOpt, &asynq.SchedulerOpts{
		Logger: zapAsynqLogger{logger: logger},
	})
	if _, err := scheduler.Register("@daily", asynq.NewTask(handlers.TaskRefreshIGTokens, nil)); err != nil {
		return fmt.Errorf("scheduler register ig refresh: %w", err)
	}
	if _, err := scheduler.Register("@daily", asynq.NewTask(handlers.TaskLogRetention, nil)); err != nil {
		return fmt.Errorf("scheduler register log retention: %w", err)
	}

	logger.Info("worker starting",
		zap.Strings("asynq_tasks", []string{
			queue.TaskInstagramEvent,
			handlers.TaskRefreshIGTokens,
			handlers.TaskLogRetention,
		}),
		zap.String("vk_manager", "enabled"),
		zap.String("scheduler", "@daily x2"),
	)

	go func() {
		if err := srv.Run(mux); err != nil {
			logger.Fatal("worker run", zap.Error(err))
		}
	}()
	go func() {
		if err := vkManager.StartAll(ctx); err != nil {
			logger.Error("vk manager", zap.Error(err))
		}
	}()
	go func() {
		if err := scheduler.Run(); err != nil {
			logger.Fatal("scheduler run", zap.Error(err))
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info("worker shutting down")
	scheduler.Shutdown()
	vkManager.Shutdown()
	srv.Shutdown()
	cancel()
	return nil
}

func asynqRedisOpt(rawURL, password string) asynq.RedisClientOpt {
	opt := asynq.RedisClientOpt{}
	parsed, err := redis.ParseURL(rawURL)
	if err == nil {
		opt.Addr = parsed.Addr
		opt.DB = parsed.DB
		if parsed.Password != "" {
			opt.Password = parsed.Password
		}
	}
	if password != "" {
		opt.Password = password
	}
	return opt
}

// zapAsynqLogger adapts zap to asynq's Logger interface.
type zapAsynqLogger struct{ logger *zap.Logger }

func (z zapAsynqLogger) Debug(args ...interface{}) { z.logger.Sugar().Debug(args...) }
func (z zapAsynqLogger) Info(args ...interface{})  { z.logger.Sugar().Info(args...) }
func (z zapAsynqLogger) Warn(args ...interface{})  { z.logger.Sugar().Warn(args...) }
func (z zapAsynqLogger) Error(args ...interface{}) { z.logger.Sugar().Error(args...) }
func (z zapAsynqLogger) Fatal(args ...interface{}) { z.logger.Sugar().Fatal(args...) }
