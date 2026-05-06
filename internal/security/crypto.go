package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

type CryptoService struct {
	key []byte
}

func NewCryptoService(key []byte) (*CryptoService, error) {
	if len(key) != 32 {
		return nil, errors.New("invalid key length")
	}
	keyCopy := make([]byte, len(key))
	copy(keyCopy, key)
	return &CryptoService{key: keyCopy}, nil
}

func (cs *CryptoService) Encrypt(plaintext string) (string, error) {
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
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil

}

func (cs *CryptoService) Decrypt(ciphertext string) (string, error) {
	fullData, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(cs.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(fullData) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, data := fullData[:nonceSize], fullData[nonceSize:]
	decrypted, err := gcm.Open(nil, nonce, data, nil)
	return string(decrypted), err
}
