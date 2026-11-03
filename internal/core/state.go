package core

// AddState — состояние пошагового добавления сервиса.
// Step: 1 — ждём имя сервиса, 2 — ждём секрет.
//
// Ввод всегда идёт в личке с ботом и хранится под личным владельцем
// пользователя — даже когда сервис добавляется для группы: так секрет
// никогда не попадает в групповой чат, а ввод одного участника не
// перемешивается с сообщениями других. GroupChatID != 0 означает, что
// сервис сохраняется под владельцем-группой.
type AddState struct {
	Step    int
	Service string

	GroupChatID int64  `json:",omitempty"`
	GroupTitle  string `json:",omitempty"`
}

// ForGroup сообщает, добавляется ли сервис для группы.
func (s AddState) ForGroup() bool { return s.GroupChatID != 0 }

// LastCodeMsg — ссылка на последнее отправленное сообщение с кодом,
// чтобы инвалидировать его при удалении сервиса.
type LastCodeMsg struct {
	MessageID int
	SecretID  int64
}
