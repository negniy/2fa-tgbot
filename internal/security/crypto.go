package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"

	"2fa-tgbot/internal/core"
)

// CryptoService реализует core.Crypter поверх AES-256-GCM.
type CryptoService struct {
	key []byte
}

// NewCryptoService создаёт сервис шифрования. Ключ обязан быть ровно 32
// байта (AES-256).
func NewCryptoService(key []byte) (*CryptoService, error) {
	if len(key) != 32 {
		return nil, errors.New("invalid key length: expected 32 bytes")
	}
	keyCopy := make([]byte, len(key))
	copy(keyCopy, key)
	return &CryptoService{key: keyCopy}, nil
}

// Encrypt шифрует открытый секрет и возвращает base64(nonce||ciphertext).
func (cs *CryptoService) Encrypt(plain core.SecretValue) (string, error) {
	block, err := aes.NewCipher(cs.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plain.Reveal()), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt расшифровывает base64(nonce||ciphertext) в открытый секрет.
func (cs *CryptoService) Decrypt(encrypted string) (core.SecretValue, error) {
	fullData, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return core.SecretValue{}, err
	}
	block, err := aes.NewCipher(cs.key)
	if err != nil {
		return core.SecretValue{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return core.SecretValue{}, err
	}
	nonceSize := gcm.NonceSize()
	if len(fullData) < nonceSize {
		return core.SecretValue{}, errors.New("ciphertext too short")
	}
	nonce, data := fullData[:nonceSize], fullData[nonceSize:]
	decrypted, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return core.SecretValue{}, err
	}
	return core.NewSecret(string(decrypted)), nil
}
