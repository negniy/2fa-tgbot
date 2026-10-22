package bot

import (
	"context"
	"hash/fnv"
	"log/slog"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// workerPool распределяет обработку апдейтов по N шардам так, что все
// апдейты одного владельца (пользователя или группы) попадают в один шард
// и обрабатываются последовательно, а разные владельцы — параллельно.
//
// Это одновременно даёт параллелизм и закрывает логическую гонку в
// пошаговом flow добавления (шаги не могут перемешаться, т.к. идут по
// очереди в рамках одного шарда).
type workerPool struct {
	queues  []chan tgbotapi.Update
	handler func(ctx context.Context, u tgbotapi.Update)
	wg      sync.WaitGroup
	log     *slog.Logger
}

// newWorkerPool создаёт пул из shards воркеров с очередью глубиной
// queueSize на каждый. handler вызывается для каждого апдейта.
func newWorkerPool(
	shards, queueSize int,
	handler func(ctx context.Context, u tgbotapi.Update),
	log *slog.Logger,
) *workerPool {
	if shards < 1 {
		shards = 1
	}
	if queueSize < 1 {
		queueSize = 1
	}
	p := &workerPool{
		queues:  make([]chan tgbotapi.Update, shards),
		handler: handler,
		log:     log,
	}
	for i := range p.queues {
		p.queues[i] = make(chan tgbotapi.Update, queueSize)
	}
	return p
}

// start запускает воркеры. ctx передаётся в обработчики; при его отмене
// диспетчер перестаёт слать апдейты и закрывает очереди (см. stop).
func (p *workerPool) start(ctx context.Context) {
	for i := range p.queues {
		p.wg.Add(1)
		go func(q chan tgbotapi.Update) {
			defer p.wg.Done()
			for u := range q {
				p.handler(ctx, u)
			}
		}(p.queues[i])
	}
}

// submit кладёт апдейт в шард по ключу маршрутизации. Возвращает false,
// если очередь переполнена (защита от неограниченного роста при флуде) —
// апдейт в этом случае отбрасывается с логом.
func (p *workerPool) submit(u tgbotapi.Update) bool {
	idx := p.shardIndex(routingKey(u))
	select {
	case p.queues[idx] <- u:
		return true
	default:
		p.log.Warn("worker queue full, dropping update", "shard", idx)
		return false
	}
}

// stop закрывает очереди и дожидается завершения воркеров (обрабатываются
// уже принятые апдейты, новые не принимаются).
func (p *workerPool) stop() {
	for _, q := range p.queues {
		close(q)
	}
	p.wg.Wait()
}

func (p *workerPool) shardIndex(key int64) int {
	h := fnv.New64a()
	var b [8]byte
	for i := 0; i < 8; i++ {
		b[i] = byte(key >> (8 * i))
	}
	_, _ = h.Write(b[:])
	return int(h.Sum64() % uint64(len(p.queues)))
}

// routingKey возвращает стабильный ключ маршрутизации, доступный СРАЗУ из
// апдейта (без похода в БД за user_id): в личке — From.ID, в группе —
// Chat.ID. Один и тот же человек/группа всегда даёт один и тот же ключ,
// значит попадает в один шард — последовательность обработки сохранена.
func routingKey(u tgbotapi.Update) int64 {
	switch {
	case u.Message != nil:
		if u.Message.Chat != nil && !u.Message.Chat.IsPrivate() {
			return u.Message.Chat.ID
		}
		if u.Message.From != nil {
			return u.Message.From.ID
		}
		if u.Message.Chat != nil {
			return u.Message.Chat.ID
		}
	case u.CallbackQuery != nil:
		if u.CallbackQuery.Message != nil && u.CallbackQuery.Message.Chat != nil &&
			!u.CallbackQuery.Message.Chat.IsPrivate() {
			return u.CallbackQuery.Message.Chat.ID
		}
		if u.CallbackQuery.From != nil {
			return u.CallbackQuery.From.ID
		}
	}
	return 0
}
