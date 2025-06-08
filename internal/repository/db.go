package repository

import (
	"database/sql"
	"fmt"
)

func FindService(db *sql.DB, serviceName string, tg_id int64) (string, error) {
	var secret string

	err := db.QueryRow(`
		SELECT secret FROM secret
		WHERE telegram_id = $1 AND tag = $2
	`, tg_id, serviceName).Scan(&secret)
	if err != nil {
		return "", err
	}

	return secret, nil
}

func AddService(db *sql.DB, serviceName string, secret string, tg_id int64) error {
	_, err := db.Exec(`
		INSERT INTO secret (telegram_id, tag, secret)
		VALUES ($1, $2, $3)
	`, tg_id, serviceName, secret)
	if err != nil {
		return err
	}
	return nil
}

func DeleteService(db *sql.DB, serviceName string, tg_id int64) error {
	result, err := db.Exec(`
		DELETE FROM secret
		WHERE telegram_id = $1 AND tag = $2
	`, tg_id, serviceName)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("сервис %q не найден", serviceName)
	}

	return nil
}

func AllService(db *sql.DB, tg_id int64) ([]string, error) {
	var services []string

	rows, err := db.Query(`
		SELECT tag FROM secret
		WHERE telegram_id = $1
	`, tg_id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		services = append(services, tag)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return services, nil
}
