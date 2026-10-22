package repository

import (
	"context"

	"2fa-tgbot/internal/core"
)

// NoopEventSink — заглушка статистики. Точки вызова уже расставлены в
// сервисном слое; на этапе фичи 5 эта заглушка заменяется на реализацию
// поверх таблицы событий без правки хендлеров.
type NoopEventSink struct{}

// NewNoopEventSink создаёт заглушку.
func NewNoopEventSink() *NoopEventSink { return &NoopEventSink{} }

// Record ничего не делает.
func (NoopEventSink) Record(ctx context.Context, e core.Event) error { return nil }
