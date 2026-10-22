package repository

import (
	"context"
	"database/sql"
	"errors"

	"2fa-tgbot/internal/core"

	"github.com/lib/pq"
)

// SecretRepo реализует core.SecretRepository поверх таблицы secret.
// Владелец хранится парой (owner_kind, owner_id).
type SecretRepo struct {
	db *sql.DB
}

// NewSecretRepo создаёт репозиторий секретов.
func NewSecretRepo(db *sql.DB) *SecretRepo {
	return &SecretRepo{db: db}
}

// Find возвращает зашифрованный секрет. sql.ErrNoRows → core.ErrNotFound.
func (r *SecretRepo) Find(ctx context.Context, owner core.Owner, tag string) (string, error) {
	var encrypted string
	err := r.db.QueryRowContext(ctx,
		`SELECT secret FROM secret
		 WHERE owner_kind = $1 AND owner_id = $2 AND tag = $3`,
		string(owner.Kind), owner.ID, tag,
	).Scan(&encrypted)
	if errors.Is(err, sql.ErrNoRows) {
		return "", core.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return encrypted, nil
}

// Add сохраняет зашифрованный секрет. Нарушение уникального индекса
// (owner_kind, owner_id, tag) → core.ErrAlreadyExists.
func (r *SecretRepo) Add(ctx context.Context, owner core.Owner, tag, encrypted string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO secret (owner_kind, owner_id, tag, secret)
		 VALUES ($1, $2, $3, $4)`,
		string(owner.Kind), owner.ID, tag, encrypted,
	)
	if isUniqueViolation(err) {
		return core.ErrAlreadyExists
	}
	return err
}

// Delete удаляет сервис. Если ничего не удалено — core.ErrNotFound.
func (r *SecretRepo) Delete(ctx context.Context, owner core.Owner, tag string) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM secret
		 WHERE owner_kind = $1 AND owner_id = $2 AND tag = $3`,
		string(owner.Kind), owner.ID, tag,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return core.ErrNotFound
	}
	return nil
}

// List возвращает имена всех сервисов владельца.
func (r *SecretRepo) List(ctx context.Context, owner core.Owner) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT tag FROM secret
		 WHERE owner_kind = $1 AND owner_id = $2
		 ORDER BY tag`,
		string(owner.Kind), owner.ID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		services = append(services, tag)
	}
	return services, rows.Err()
}

// isUniqueViolation распознаёт ошибку нарушения уникальности PostgreSQL
// (код 23505).
func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == "23505"
	}
	return false
}
