package core

import "time"

// EventAction — тип действия, фиксируемого в статистике. Специально
// перечислен как набор констант, чтобы EventSink писал обезличенные
// агрегаты, а не произвольные строки.
type EventAction string

const (
	EventCodeRequested  EventAction = "code_requested"
	EventSecretAdded    EventAction = "secret_added"
	EventSecretDeleted  EventAction = "secret_deleted"
	EventServicesListed EventAction = "services_listed"
	EventError          EventAction = "error"
)

// Event — обезличенное событие для статистики. Намеренно НЕ содержит ни
// секрета, ни кода, ни имени сервиса — только владельца, действие и время.
// Этого достаточно для агрегатов (число запросов, ошибок и т.п.) и не
// раскрывает, какими сервисами пользуется человек.
type Event struct {
	Owner  Owner
	Action EventAction
	At     time.Time
}
