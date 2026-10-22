package security

import (
	"testing"

	"2fa-tgbot/internal/core"
)

func newTestService(t *testing.T) *CryptoService {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	cs, err := NewCryptoService(key)
	if err != nil {
		t.Fatalf("NewCryptoService: %v", err)
	}
	return cs
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	cs := newTestService(t)
	plain := core.NewSecret("JBSWY3DPEHPK3PXP")

	enc, err := cs.Encrypt(plain)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if enc == plain.Reveal() {
		t.Fatal("ciphertext equals plaintext")
	}

	dec, err := cs.Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if dec.Reveal() != plain.Reveal() {
		t.Fatalf("round-trip mismatch: got %q", dec.Reveal())
	}
}

func TestEncryptProducesDifferentNonces(t *testing.T) {
	cs := newTestService(t)
	plain := core.NewSecret("same-secret")

	a, _ := cs.Encrypt(plain)
	b, _ := cs.Encrypt(plain)
	if a == b {
		t.Fatal("two encryptions produced identical output (nonce reuse)")
	}
}

func TestNewCryptoServiceRejectsBadKey(t *testing.T) {
	if _, err := NewCryptoService(make([]byte, 16)); err == nil {
		t.Fatal("expected error for 16-byte key")
	}
}

func TestDecryptRejectsShortCiphertext(t *testing.T) {
	cs := newTestService(t)
	if _, err := cs.Decrypt("AAAA"); err == nil {
		t.Fatal("expected error for too-short ciphertext")
	}
}
