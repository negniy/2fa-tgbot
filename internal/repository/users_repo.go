package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// UsersRepo реализует core.UserResolver поверх таблиц users/identities.
type UsersRepo struct {
	db *sql.DB
}

// NewUsersRepo создаёт репозиторий пользователей.
func NewUsersRepo(db *sql.DB) *UsersRepo {
	return &UsersRepo{db: db}
}

const providerTelegram = "telegram"

// ResolveTelegram возвращает внутренний user_id по telegram_user_id.
// Если пользователя ещё нет — атомарно создаёт users + identity(telegram).
//
// Гонка (два параллельных первых сообщения одного пользователя) закрыта
// уникальным индексом (provider, provider_uid): при конфликте на вставке
// делаем повторный SELECT и возвращаем уже существующий user_id.
func (r *UsersRepo) ResolveTelegram(ctx context.Context, telegramUserID int64) (int64, error) {
	provID := fmt.Sprintf("%d", telegramUserID)

	// Быстрый путь: пользователь уже существует.
	if userID, ok, err := r.findByIdentity(ctx, providerTelegram, provID); err != nil {
		return 0, err
	} else if ok {
		return userID, nil
	}

	// Медленный путь: создаём в транзакции.
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	var userID int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO users (created_at) VALUES (now()) RETURNING id`,
	).Scan(&userID)
	if err != nil {
		return 0, fmt.Errorf("create user: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO identities (user_id, provider, provider_uid)
		 VALUES ($1, $2, $3)`,
		userID, providerTelegram, provID,
	)
	if err != nil {
		// Кто-то успел создать identity параллельно — откатываемся и
		// читаем существующего пользователя.
		_ = tx.Rollback()
		if existingID, ok, ferr := r.findByIdentity(ctx, providerTelegram, provID); ferr == nil && ok {
			return existingID, nil
		}
		return 0, fmt.Errorf("create identity: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return userID, nil
}

// findByIdentity ищет user_id по паре (provider, provider_uid).
func (r *UsersRepo) findByIdentity(ctx context.Context, provider, provUID string) (int64, bool, error) {
	var userID int64
	err := r.db.QueryRowContext(ctx,
		`SELECT user_id FROM identities WHERE provider = $1 AND provider_uid = $2`,
		provider, provUID,
	).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return userID, true, nil
}
