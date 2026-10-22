package config

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// Config — разобранная конфигурация приложения. Здесь только «сырые»
// значения и открытое соединение с БД; сборка зависимостей (DI) живёт в
// cmd/main.go.
type Config struct {
	TelegramToken string
	MasterKey     []byte // 32 байта для AES-256
	DB            *sql.DB
	EnableGroups  bool
	LogLevel      string

	// Воркер-пул обработки апдейтов.
	WorkerShards    int
	WorkerQueueSize int

	// Redis-настройки читаются уже сейчас, чтобы шаг 3 подключил Redis без
	// изменения конфигурации. На шаге 1 не используются.
	RedisAddr     string
	RedisPassword string
	RedisDB       int
}

// Load читает .env (если есть) и переменные окружения, открывает БД и
// прогоняет миграции.
func Load() (*Config, error) {
	// .env не обязателен (в проде переменные приходят из окружения).
	_ = godotenv.Load(filepath.Join(".", ".env"))

	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		env("DB_HOST", "localhost"),
		env("DB_PORT", "5432"),
		env("DB_USER", "postgres"),
		os.Getenv("DB_PASSWORD"),
		env("DB_NAME", "totpbot"),
		env("DB_SSLMODE", "disable"),
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	if err := RunMigrations(db, "migrations"); err != nil {
		return nil, fmt.Errorf("migrations: %w", err)
	}

	masterKey, err := base64.StdEncoding.DecodeString(os.Getenv("MASTER_KEY"))
	if err != nil {
		return nil, fmt.Errorf("invalid MASTER_KEY (expect base64): %w", err)
	}

	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("TELEGRAM_TOKEN is required")
	}

	return &Config{
		TelegramToken: token,
		MasterKey:     masterKey,
		DB:            db,
		EnableGroups:  envBool("ENABLE_GROUPS", true),
		LogLevel:      env("LOG_LEVEL", "info"),

		WorkerShards:    envInt("WORKER_SHARDS", 8),
		WorkerQueueSize: envInt("WORKER_QUEUE_SIZE", 64),
		RedisAddr:       env("REDIS_ADDR", "localhost:6379"),
		RedisPassword:   os.Getenv("REDIS_PASSWORD"),
		RedisDB:         envInt("REDIS_DB", 0),
	}, nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
