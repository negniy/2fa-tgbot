package main

import (
	"2fa-tgbot/internal/bot"
	"2fa-tgbot/internal/config"
	"log"
)

func main() {
	cfg := config.Load()
	if err := bot.Start(cfg); err != nil {
		log.Fatal(err)
	}
}
