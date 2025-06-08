package totp

import (
	"time"

	"github.com/pquerna/otp/totp"
)

func GenerateCode(secret string) (string, error) {
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		return "", err
	}
	return code, nil
}
