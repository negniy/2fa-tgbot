package core

import "errors"

// Доменные ошибки, не зависящие от конкретной БД или транспорта.
// Реализации репозиториев обязаны маппить свои низкоуровневые ошибки
// (например, sql.ErrNoRows или нарушение уникального индекса) в эти.
var (
	// ErrNotFound — сущность (секрет, пользователь) не найдена.
	ErrNotFound = errors.New("not found")

	// ErrAlreadyExists — попытка создать дубликат (например, сервис с уже
	// существующим именем у того же владельца).
	ErrAlreadyExists = errors.New("already exists")

	// ErrGroupsDisabled — работа в группах отключена конфигурацией.
	ErrGroupsDisabled = errors.New("groups are disabled")
)
