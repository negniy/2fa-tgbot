package config

import "os"

type Config struct {
	TelegramToken string
	TotpSecret    string
}

func Load() Config {
	return Config{
		TelegramToken: os.Getenv("TELEGRAM_TOKEN"),
		TotpSecret:    os.Getenv("TOTP_SECRET"),
	}
}
