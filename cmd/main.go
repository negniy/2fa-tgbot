package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"2fa-tgbot/internal/bot"
	"2fa-tgbot/internal/config"
	"2fa-tgbot/internal/core"
	"2fa-tgbot/internal/logger"
	"2fa-tgbot/internal/repository"
	"2fa-tgbot/internal/security"
	"2fa-tgbot/internal/state"
	"2fa-tgbot/internal/totp"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		// Логгер ещё не собран — используем временный.
		logger.New("info").Error("config load failed", "err", err)
		os.Exit(1)
	}
	defer cfg.DB.Close()

	log := logger.New(cfg.LogLevel)

	// --- сборка зависимостей (DI) ---
	crypter, err := security.NewCryptoService(cfg.MasterKey)
	if err != nil {
		log.Error("crypto init failed", "err", err)
		os.Exit(1)
	}

	secretRepo := repository.NewSecretRepo(cfg.DB)
	usersRepo := repository.NewUsersRepo(cfg.DB)
	events := repository.NewNoopEventSink() // фича 5: заменить на БД-реализацию
	totpGen := totp.New()

	// StateStore поверх Redis. При недоступности Redis выбрана стратегия
	// мягкой деградации: бот стартует, чтение кодов (Postgres) работает,
	// временно недоступен лишь flow добавления и очистка сообщений.
	redisClient, err := state.NewRedisClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Warn("redis unreachable at startup, running in degraded mode", "err", err)
	}
	defer redisClient.Close()
	stateStore := state.NewRedisStore(redisClient, log)

	svc := core.NewSecretService(secretRepo, crypter, totpGen, events, log)

	b, err := bot.New(bot.Deps{
		Token:        cfg.TelegramToken,
		Service:      svc,
		Users:        usersRepo,
		State:        stateStore,
		Log:          log,
		EnableGroups: cfg.EnableGroups,
		Shards:       cfg.WorkerShards,
		QueueSize:    cfg.WorkerQueueSize,
	})
	if err != nil {
		log.Error("bot init failed", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("bot starting", "groups_enabled", cfg.EnableGroups)
	if err := b.Run(ctx); err != nil {
		log.Error("bot stopped with error", "err", err)
		os.Exit(1)
	}
	log.Info("application stopped gracefully")
}
