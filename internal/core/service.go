package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// CodeResult — результат генерации кода: сам код и сколько секунд он ещё
// действует (для UI, который затирает код по истечении окна).
type CodeResult struct {
	Tag              string
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

// CheckNewName нормализует имя нового сервиса и проверяет, что у владельца
// такого ещё нет. Позволяет отвергнуть имя ДО того, как пользователь
// пришлёт секрет. Возвращает ErrInvalidName или ErrAlreadyExists; во
// втором случае нормализованное имя тоже возвращается — для сообщения.
func (s *SecretService) CheckNewName(ctx context.Context, owner Owner, rawName string) (string, error) {
	name, err := NormalizeServiceName(rawName)
	if err != nil {
		return "", err
	}
	// Сервисов у владельца единицы-десятки, отдельный запрос в порт ради
	// проверки существования не нужен. Окончательно дубль всё равно
	// отсекает уникальный индекс при Add.
	services, err := s.repo.List(ctx, owner)
	if err != nil {
		return "", err
	}
	for _, svc := range services {
		if svc.Tag == name {
			return name, ErrAlreadyExists
		}
	}
	return name, nil
}

// AddSecret проверяет и нормализует имя и секрет, шифрует секрет и
// сохраняет его под владельцем. rawSecret приходит уже как SecretValue,
// поэтому не может утечь в логи. Ошибки валидации — ErrInvalidName и
// ErrInvalidSecret.
func (s *SecretService) AddSecret(ctx context.Context, owner Owner, rawName string, rawSecret SecretValue) error {
	tag, err := NormalizeServiceName(rawName)
	if err != nil {
		return err
	}
	secret, err := NormalizeSecret(rawSecret)
	if err != nil {
		return err
	}
	encrypted, err := s.crypter.Encrypt(secret)
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
func (s *SecretService) GetCode(ctx context.Context, owner Owner, id int64) (CodeResult, error) {
	rec, err := s.repo.Find(ctx, owner, id)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			s.record(ctx, owner, EventError)
		}
		return CodeResult{}, err
	}
	secret, err := s.crypter.Decrypt(rec.Encrypted)
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
	return CodeResult{Tag: rec.Tag, Code: code, SecondsRemaining: remaining}, nil
}

// GetSecretInfo возвращает сведения о сервисе владельца без расшифровки
// секрета (для экранов, где нужно лишь имя).
func (s *SecretService) GetSecretInfo(ctx context.Context, owner Owner, id int64) (SecretInfo, error) {
	rec, err := s.repo.Find(ctx, owner, id)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			s.record(ctx, owner, EventError)
		}
		return SecretInfo{}, err
	}
	return rec.SecretInfo, nil
}

// DeleteSecret удаляет сервис владельца и возвращает его имя.
func (s *SecretService) DeleteSecret(ctx context.Context, owner Owner, id int64) (string, error) {
	tag, err := s.repo.Delete(ctx, owner, id)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			s.record(ctx, owner, EventError)
		}
		return "", err
	}
	s.record(ctx, owner, EventSecretDeleted)
	return tag, nil
}

// ListSecrets возвращает сервисы владельца.
func (s *SecretService) ListSecrets(ctx context.Context, owner Owner) ([]SecretInfo, error) {
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
