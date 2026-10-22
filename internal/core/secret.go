package core

import "log/slog"

// SecretValue — обёртка над строкой секрета (TOTP base32 или расшифрованный
// секрет). Её задача — сделать невозможной случайную утечку секрета в логи
// и текстовые представления. Реальное значение достаётся только явным
// вызовом Reveal().
//
// String() и LogValue() возвращают "[REDACTED]", поэтому даже случайный
// fmt.Sprintf("%v", secret) или slog.Any("secret", secret) ничего не раскроют.
type SecretValue struct {
	value string
}

// NewSecret оборачивает сырую строку секрета.
func NewSecret(raw string) SecretValue {
	return SecretValue{value: raw}
}

// Reveal возвращает настоящее значение секрета. Вызывать только там, где
// секрет реально нужен (шифрование, генерация кода). Никогда не логировать
// результат Reveal().
func (s SecretValue) Reveal() string {
	return s.value
}

// IsEmpty сообщает, пуст ли секрет.
func (s SecretValue) IsEmpty() bool {
	return s.value == ""
}

// String реализует fmt.Stringer и намеренно скрывает значение.
func (s SecretValue) String() string {
	return "[REDACTED]"
}

// LogValue реализует slog.LogValuer — секрет в структурных логах скрыт.
func (s SecretValue) LogValue() slog.Value {
	return slog.StringValue("[REDACTED]")
}
