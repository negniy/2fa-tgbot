package core

import (
	"encoding/base32"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// MaxServiceNameLen — максимальная длина имени сервиса в символах.
	MaxServiceNameLen = 64

	// minSecretBytes — минимальная длина ключа после декодирования: 80 бит.
	// RFC 4226 требует от 128 бит, но многие сервисы (в том числе Google)
	// выдают 80-битные ключи, а отвергать их нельзя.
	minSecretBytes = 10
	// maxSecretBytes — разумный верхний предел против мусорного ввода.
	maxSecretBytes = 128
)

// base32NoPad — алфавит RFC 4648 без паддинга: «=» срезается при
// нормализации, так что ключ любой длины декодируется единообразно.
var base32NoPad = base32.StdEncoding.WithPadding(base32.NoPadding)

// NormalizeServiceName приводит имя сервиса к каноничному виду: обрезает
// пробелы по краям и схлопывает внутренние пробельные последовательности.
// Возвращает ErrInvalidName, если имя пустое, длиннее MaxServiceNameLen
// символов, содержит управляющие символы или начинается с «/» (в Telegram
// такое сообщение было бы командой).
func NormalizeServiceName(raw string) (string, error) {
	if !utf8.ValidString(raw) {
		return "", ErrInvalidName
	}
	name := strings.Join(strings.Fields(raw), " ")
	if name == "" || strings.HasPrefix(name, "/") {
		return "", ErrInvalidName
	}
	if utf8.RuneCountInString(name) > MaxServiceNameLen {
		return "", ErrInvalidName
	}
	for _, r := range name {
		if unicode.IsControl(r) || r == utf8.RuneError {
			return "", ErrInvalidName
		}
	}
	return name, nil
}

// NormalizeSecret приводит TOTP-ключ к каноничному виду и проверяет его.
// Сервисы часто показывают ключ группами в нижнем регистре
// («abcd efgh ...») — пробелы и дефисы удаляются, регистр поднимается,
// паддинг «=» срезается. Результат обязан декодироваться как base32 и
// давать от minSecretBytes до maxSecretBytes байт, иначе — ErrInvalidSecret.
func NormalizeSecret(raw SecretValue) (SecretValue, error) {
	var sb strings.Builder
	for _, r := range raw.Reveal() {
		if unicode.IsSpace(r) || r == '-' {
			continue
		}
		sb.WriteRune(unicode.ToUpper(r))
	}
	s := strings.TrimRight(sb.String(), "=")

	decoded, err := base32NoPad.DecodeString(s)
	if err != nil || len(decoded) < minSecretBytes || len(decoded) > maxSecretBytes {
		return SecretValue{}, ErrInvalidSecret
	}
	return NewSecret(s), nil
}
