package core

import "context"

// SecretRepository — доступ к зашифрованным секретам, привязанным к Owner.
// Реализация (Postgres) маппит низкоуровневые ошибки в доменные
// (ErrNotFound, ErrAlreadyExists). В хранилище секрет всегда лежит
// зашифрованным, поэтому методы оперируют строкой-шифротекстом, а не
// SecretValue: шифрование/дешифровка — ответственность сервисного слоя.
type SecretRepository interface {
	// Find возвращает ЗАШИФРОВАННЫЙ секрет по владельцу и имени сервиса.
	Find(ctx context.Context, owner Owner, tag string) (encrypted string, err error)
	// Add сохраняет ЗАШИФРОВАННЫЙ секрет. При дубликате имени —
	// ErrAlreadyExists.
	Add(ctx context.Context, owner Owner, tag, encrypted string) error
	// Delete удаляет сервис. Если сервиса нет — ErrNotFound.
	Delete(ctx context.Context, owner Owner, tag string) error
	// List возвращает имена всех сервисов владельца.
	List(ctx context.Context, owner Owner) ([]string, error)
}

// Crypter — симметричное шифрование секретов. SecretValue на входе/выходе
// защищает от утечки открытого текста в логи.
type Crypter interface {
	Encrypt(plain SecretValue) (string, error)
	Decrypt(encrypted string) (SecretValue, error)
}

// UserResolver превращает внешнюю идентичность (сейчас — telegram_user_id)
// во внутренний user_id, создавая пользователя при первом обращении.
// Позже сюда добавятся резолверы для веб/OAuth (фича 6), но ядро уже
// оперирует внутренним user_id, а не telegram-специфичным.
type UserResolver interface {
	// ResolveTelegram ищет пользователя по telegram_user_id; если такого
	// нет — создаёт users + identity(provider=telegram) и возвращает
	// внутренний user_id.
	ResolveTelegram(ctx context.Context, telegramUserID int64) (userID int64, err error)
}

// TOTPGenerator генерирует одноразовый код по секрету.
type TOTPGenerator interface {
	Generate(secret SecretValue) (code string, err error)
}

// StateStore хранит эфемерное состояние диалога (шаги добавления, id
// последних сообщений). Сейчас реализуется in-memory, на шаге 3 —
// поверх Redis с TTL. Интерфейс намеренно узкий и атомарный, чтобы
// Redis-реализация не потребовала менять сигнатуры.
type StateStore interface {
	// --- flow добавления сервиса ---
	GetAddState(ctx context.Context, owner Owner) (*AddState, bool, error)
	SetAddState(ctx context.Context, owner Owner, st AddState) error
	DeleteAddState(ctx context.Context, owner Owner) error

	// --- последнее сообщение бота в чате (для очистки) ---
	GetLastBotMessage(ctx context.Context, chatID int64) (msgID int, ok bool, err error)
	SetLastBotMessage(ctx context.Context, chatID int64, msgID int) error

	// --- последнее сообщение с кодом (для инвалидации при удалении) ---
	GetLastCodeMessage(ctx context.Context, owner Owner) (LastCodeMsg, bool, error)
	SetLastCodeMessage(ctx context.Context, owner Owner, msg LastCodeMsg) error
	DeleteLastCodeMessage(ctx context.Context, owner Owner) error
}

// EventSink принимает обезличенные события для статистики. Сейчас —
// no-op реализация; на этапе фичи 5 появится реализация поверх БД.
// Точки вызова уже расставлены в сервисном слое, поэтому статистику
// можно будет включить без правки хендлеров.
type EventSink interface {
	Record(ctx context.Context, e Event) error
}
