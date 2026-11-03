package core

// SecretInfo — публичные сведения о сохранённом сервисе: без значения
// секрета, поэтому их безопасно показывать и передавать в UI.
type SecretInfo struct {
	ID  int64
	Tag string
}

// SecretRecord — сервис вместе с ЗАШИФРОВАННЫМ значением секрета, как он
// лежит в хранилище. Расшифровка — ответственность сервисного слоя.
type SecretRecord struct {
	SecretInfo
	Encrypted string
}
