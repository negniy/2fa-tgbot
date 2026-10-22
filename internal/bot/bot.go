package bot

import (
	"context"
	"log/slog"

	"2fa-tgbot/internal/core"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot — Telegram-адаптер. Тонкий слой поверх core.SecretService: разбирает
// апдейты, резолвит владельца и вызывает сервис. Никакой доменной логики
// (шифрование, генерация кодов, доступ к БД) здесь нет.
type Bot struct {
	api          *tgbotapi.BotAPI
	svc          *core.SecretService
	users        core.UserResolver
	state        core.StateStore
	log          *slog.Logger
	enableGroups bool
	shards       int
	queueSize    int
}

// Deps — зависимости бота, собираемые в main.
type Deps struct {
	Token        string
	Service      *core.SecretService
	Users        core.UserResolver
	State        core.StateStore
	Log          *slog.Logger
	EnableGroups bool
	// Shards — число воркеров пула (по умолчанию 8). QueueSize — глубина
	// очереди на воркер (по умолчанию 64).
	Shards    int
	QueueSize int
}

// New создаёт бота и подключается к Telegram.
func New(d Deps) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(d.Token)
	if err != nil {
		return nil, err
	}
	shards := d.Shards
	if shards < 1 {
		shards = 8
	}
	queueSize := d.QueueSize
	if queueSize < 1 {
		queueSize = 64
	}
	return &Bot{
		api:          api,
		svc:          d.Service,
		users:        d.Users,
		state:        d.State,
		log:          d.Log,
		enableGroups: d.EnableGroups,
		shards:       shards,
		queueSize:    queueSize,
	}, nil
}

// Run запускает воркер-пул и цикл диспетчеризации апдейтов. Апдейты
// распределяются по шардам (последовательность на владельца, параллельность
// между владельцами). При отмене контекста бот перестаёт принимать апдейты,
// дожидается завершения уже принятых и выходит.
func (b *Bot) Run(ctx context.Context) error {
	pool := newWorkerPool(b.shards, b.queueSize, b.dispatch, b.log)
	pool.start(ctx)
	b.log.Info("worker pool started", "shards", b.shards, "queue_size", b.queueSize)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case update := <-updates:
			pool.submit(update)
		case <-ctx.Done():
			b.log.Info("shutting down bot")
			b.api.StopReceivingUpdates()
			// Дожидаемся, пока воркеры дочистят принятые апдейты.
			pool.stop()
			b.log.Info("worker pool drained")
			return nil
		}
	}
}

// dispatch направляет апдейт нужному обработчику.
func (b *Bot) dispatch(ctx context.Context, update tgbotapi.Update) {
	switch {
	case update.Message != nil:
		b.handleMessage(ctx, update.Message)
	case update.CallbackQuery != nil:
		b.handleCallback(ctx, update.CallbackQuery)
	}
}

// resolveOwner превращает сообщение/чат в core.Owner. В личке резолвит
// telegram_user_id во внутренний user_id; в группе владелец — сама группа.
// Возвращает (owner, ok): ok=false, если группы отключены.
func (b *Bot) resolveOwner(ctx context.Context, chat *tgbotapi.Chat, fromID int64) (core.Owner, bool) {
	if chat.IsPrivate() {
		userID, err := b.users.ResolveTelegram(ctx, fromID)
		if err != nil {
			b.log.Error("resolve telegram user failed", "err", err)
			return core.Owner{}, false
		}
		return core.NewUserOwner(userID), true
	}
	if !b.enableGroups {
		return core.Owner{}, false
	}
	return core.NewGroupOwner(chat.ID), true
}
