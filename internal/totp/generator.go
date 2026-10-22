package totp

import (
	"time"

	"2fa-tgbot/internal/core"

	"github.com/pquerna/otp/totp"
)

// Generator реализует core.TOTPGenerator поверх pquerna/otp.
type Generator struct{}

// New создаёт генератор TOTP-кодов.
func New() *Generator {
	return &Generator{}
}

// Generate возвращает текущий TOTP-код по секрету. Открытый секрет
// раскрывается только здесь, локально, и не логируется.
func (g *Generator) Generate(secret core.SecretValue) (string, error) {
	return totp.GenerateCode(secret.Reveal(), time.Now())
}
