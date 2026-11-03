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

// Find возвращает секрет владельца по ID. Условие на владельца в WHERE
// обязательно: чужой ID даёт core.ErrNotFound (sql.ErrNoRows).
func (r *SecretRepo) Find(ctx context.Context, owner core.Owner, id int64) (core.SecretRecord, error) {
	var rec core.SecretRecord
	err := r.db.QueryRowContext(ctx,
		`SELECT id, tag, secret FROM secret
		 WHERE owner_kind = $1 AND owner_id = $2 AND id = $3`,
		string(owner.Kind), owner.ID, id,
	).Scan(&rec.ID, &rec.Tag, &rec.Encrypted)
	if errors.Is(err, sql.ErrNoRows) {
		return core.SecretRecord{}, core.ErrNotFound
	}
	if err != nil {
		return core.SecretRecord{}, err
	}
	return rec, nil
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

// Delete удаляет сервис владельца по ID и возвращает его имя. Если ничего
// не удалено — core.ErrNotFound.
func (r *SecretRepo) Delete(ctx context.Context, owner core.Owner, id int64) (string, error) {
	var tag string
	err := r.db.QueryRowContext(ctx,
		`DELETE FROM secret
		 WHERE owner_kind = $1 AND owner_id = $2 AND id = $3
		 RETURNING tag`,
		string(owner.Kind), owner.ID, id,
	).Scan(&tag)
	if errors.Is(err, sql.ErrNoRows) {
		return "", core.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return tag, nil
}

// List возвращает все сервисы владельца, отсортированные по имени.
func (r *SecretRepo) List(ctx context.Context, owner core.Owner) ([]core.SecretInfo, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, tag FROM secret
		 WHERE owner_kind = $1 AND owner_id = $2
		 ORDER BY tag, id`,
		string(owner.Kind), owner.ID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []core.SecretInfo
	for rows.Next() {
		var info core.SecretInfo
		if err := rows.Scan(&info.ID, &info.Tag); err != nil {
			return nil, err
		}
		services = append(services, info)
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
