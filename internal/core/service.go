package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// CodeResult — результат генерации кода: сам код и сколько секунд он ещё
// действует (для UI, который затирает код по истечении окна).
type CodeResult struct {
	Code             string
	SecondsRemaining int64
}

// SecretService — сердце приложения. Ничего не знает про Telegram, HTTP,
// Redis или Postgres: работает только через порты и Owner. И бот, и будущий
// сайт — тонкие адаптеры поверх этого сервиса.
type SecretService struct {
	repo    SecretRepository
	crypter Crypter
	totp    TOTPGenerator
	events  EventSink
	log     *slog.Logger
	// period — длина окна TOTP в секундах (стандартно 30).
	period int64
}

// NewSecretService собирает сервис из зависимостей-портов.
func NewSecretService(
	repo SecretRepository,
	crypter Crypter,
	totp TOTPGenerator,
	events EventSink,
	log *slog.Logger,
) *SecretService {
	return &SecretService{
		repo:    repo,
		crypter: crypter,
		totp:    totp,
		events:  events,
		log:     log,
		period:  30,
	}
}

// AddSecret шифрует сырой секрет и сохраняет его под владельцем.
// rawSecret приходит уже как SecretValue, поэтому не может утечь в логи.
func (s *SecretService) AddSecret(ctx context.Context, owner Owner, tag string, rawSecret SecretValue) error {
	encrypted, err := s.crypter.Encrypt(rawSecret)
	if err != nil {
		s.record(ctx, owner, EventError)
		return fmt.Errorf("encrypt: %w", err)
	}
	if err := s.repo.Add(ctx, owner, tag, encrypted); err != nil {
		// ErrAlreadyExists прокидываем как есть — адаптер покажет
		// осмысленное сообщение.
		s.log.Info("add secret failed", "owner_kind", owner.Kind, "err", err)
		return err
	}
	s.record(ctx, owner, EventSecretAdded)
	s.log.Info("secret added", "owner_kind", owner.Kind)
	return nil
}

// GetCode находит секрет, дешифрует его и генерирует TOTP-код с остатком
// времени. Открытый секрет живёт только внутри этого метода.
func (s *SecretService) GetCode(ctx context.Context, owner Owner, tag string) (CodeResult, error) {
	encrypted, err := s.repo.Find(ctx, owner, tag)
	if err != nil {
		if err != ErrNotFound {
			s.record(ctx, owner, EventError)
		}
		return CodeResult{}, err
	}
	secret, err := s.crypter.Decrypt(encrypted)
	if err != nil {
		s.record(ctx, owner, EventError)
		return CodeResult{}, fmt.Errorf("decrypt: %w", err)
	}
	code, err := s.totp.Generate(secret)
	if err != nil {
		s.record(ctx, owner, EventError)
		return CodeResult{}, fmt.Errorf("totp: %w", err)
	}
	remaining := s.period - (time.Now().Unix() % s.period)
	s.record(ctx, owner, EventCodeRequested)
	return CodeResult{Code: code, SecondsRemaining: remaining}, nil
}

// DeleteSecret удаляет сервис владельца.
func (s *SecretService) DeleteSecret(ctx context.Context, owner Owner, tag string) error {
	if err := s.repo.Delete(ctx, owner, tag); err != nil {
		if err != ErrNotFound {
			s.record(ctx, owner, EventError)
		}
		return err
	}
	s.record(ctx, owner, EventSecretDeleted)
	return nil
}

// ListSecrets возвращает имена сервисов владельца.
func (s *SecretService) ListSecrets(ctx context.Context, owner Owner) ([]string, error) {
	services, err := s.repo.List(ctx, owner)
	if err != nil {
		s.record(ctx, owner, EventError)
		return nil, err
	}
	s.record(ctx, owner, EventServicesListed)
	return services, nil
}

// record — единая точка отправки событий статистики. Ошибку сбора
// статистики намеренно не пробрасываем: аналитика не должна ронять
// основную операцию.
func (s *SecretService) record(ctx context.Context, owner Owner, action EventAction) {
	if s.events == nil {
		return
	}
	if err := s.events.Record(ctx, Event{Owner: owner, Action: action, At: time.Now()}); err != nil {
		s.log.Warn("event sink failed", "action", action, "err", err)
	}
}
