package state

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient создаёт клиент go-redis и делает пробный ping.
// Возвращает клиент всегда; ошибка ping возвращается отдельно, чтобы
// вызывающая сторона могла решить — падать или деградировать. При выбранной
// стратегии мягкой деградации main лишь логирует эту ошибку, но продолжает.
func NewRedisClient(addr, password string, db int) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := rdb.Ping(ctx).Err()
	return rdb, err
}
