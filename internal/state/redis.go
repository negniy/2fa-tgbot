package state

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"time"

	"2fa-tgbot/internal/core"

	"github.com/redis/go-redis/v9"
)

// TTL для разных видов эфемерного состояния.
const (
	// addStateTTL ограничивает время «висения» в полу-введённом состоянии
	// добавления сервиса. Заодно чистит брошенные flow.
	addStateTTL = 10 * time.Minute
	// msgTTL — для косметических ссылок на последние сообщения.
	msgTTL = 1 * time.Hour
)

// RedisStore реализует core.StateStore поверх Redis.
//
// Мягкая деградация: ошибки Redis не роняют бота и не мешают чтению
// секретов (оно идёт в Postgres, а не сюда). Ошибки логируются; операции
// чтения при сбое возвращают «пусто», операции записи — тихо не срабатывают.
type RedisStore struct {
	rdb *redis.Client
	log *slog.Logger
}

// NewRedisStore создаёт хранилище состояния поверх go-redis.
func NewRedisStore(rdb *redis.Client, log *slog.Logger) *RedisStore {
	return &RedisStore{rdb: rdb, log: log}
}

// --- построение ключей ---

func addStateKey(o core.Owner) string {
	return "addstate:" + string(o.Kind) + ":" + strconv.FormatInt(o.ID, 10)
}

func lastCodeKey(o core.Owner) string {
	return "lastcode:" + string(o.Kind) + ":" + strconv.FormatInt(o.ID, 10)
}

func lastMsgKey(chatID int64) string {
	return "lastmsg:" + strconv.FormatInt(chatID, 10)
}

// --- AddState ---

func (s *RedisStore) GetAddState(ctx context.Context, owner core.Owner) (*core.AddState, bool, error) {
	raw, err := s.rdb.Get(ctx, addStateKey(owner)).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		// Деградация: считаем, что активного flow нет. Так сообщение будет
		// обработано как обычная команда, а не как шаг ввода секрета.
		s.log.Warn("redis get addstate failed", "err", err)
		return nil, false, nil
	}
	var st core.AddState
	if err := json.Unmarshal(raw, &st); err != nil {
		s.log.Warn("addstate unmarshal failed", "err", err)
		return nil, false, nil
	}
	return &st, true, nil
}

func (s *RedisStore) SetAddState(ctx context.Context, owner core.Owner, st core.AddState) error {
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	if err := s.rdb.Set(ctx, addStateKey(owner), data, addStateTTL).Err(); err != nil {
		s.log.Warn("redis set addstate failed", "err", err)
		// Деградация: не пробрасываем — flow добавления просто не сработает.
	}
	return nil
}

func (s *RedisStore) DeleteAddState(ctx context.Context, owner core.Owner) error {
	if err := s.rdb.Del(ctx, addStateKey(owner)).Err(); err != nil {
		s.log.Warn("redis del addstate failed", "err", err)
	}
	return nil
}

// --- last bot message ---

func (s *RedisStore) GetLastBotMessage(ctx context.Context, chatID int64) (int, bool, error) {
	v, err := s.rdb.Get(ctx, lastMsgKey(chatID)).Int()
	if err == redis.Nil {
		return 0, false, nil
	}
	if err != nil {
		s.log.Warn("redis get lastmsg failed", "err", err)
		return 0, false, nil
	}
	return v, true, nil
}

func (s *RedisStore) SetLastBotMessage(ctx context.Context, chatID int64, msgID int) error {
	if err := s.rdb.Set(ctx, lastMsgKey(chatID), msgID, msgTTL).Err(); err != nil {
		s.log.Warn("redis set lastmsg failed", "err", err)
	}
	return nil
}

// --- last code message ---

func (s *RedisStore) GetLastCodeMessage(ctx context.Context, owner core.Owner) (core.LastCodeMsg, bool, error) {
	raw, err := s.rdb.Get(ctx, lastCodeKey(owner)).Bytes()
	if err == redis.Nil {
		return core.LastCodeMsg{}, false, nil
	}
	if err != nil {
		s.log.Warn("redis get lastcode failed", "err", err)
		return core.LastCodeMsg{}, false, nil
	}
	var msg core.LastCodeMsg
	if err := json.Unmarshal(raw, &msg); err != nil {
		s.log.Warn("lastcode unmarshal failed", "err", err)
		return core.LastCodeMsg{}, false, nil
	}
	return msg, true, nil
}

func (s *RedisStore) SetLastCodeMessage(ctx context.Context, owner core.Owner, msg core.LastCodeMsg) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if err := s.rdb.Set(ctx, lastCodeKey(owner), data, msgTTL).Err(); err != nil {
		s.log.Warn("redis set lastcode failed", "err", err)
	}
	return nil
}

func (s *RedisStore) DeleteLastCodeMessage(ctx context.Context, owner core.Owner) error {
	if err := s.rdb.Del(ctx, lastCodeKey(owner)).Err(); err != nil {
		s.log.Warn("redis del lastcode failed", "err", err)
	}
	return nil
}
