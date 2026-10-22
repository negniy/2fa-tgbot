package logger

import (
	"log/slog"
	"os"
	"strings"
)

// New создаёт структурный JSON-логгер. level — "debug"|"info"|"warn"|"error".
// Секреты защищены на уровне доменного типа (core.SecretValue реализует
// slog.LogValuer и печатается как [REDACTED]), поэтому отдельной чистки
// здесь не требуется — но правило «не логировать Reveal()» остаётся в силе.
func New(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return slog.New(handler)
}
