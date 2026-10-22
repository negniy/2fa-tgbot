package core

// AddState — состояние пошагового добавления сервиса.
// Step: 1 — ждём имя сервиса, 2 — ждём секрет.
type AddState struct {
	Step    int
	Service string
}

// LastCodeMsg — ссылка на последнее отправленное сообщение с кодом,
// чтобы инвалидировать его при удалении сервиса.
type LastCodeMsg struct {
	MessageID int
	Service   string
}
