package main

import (
	"2fa-tgbot/internal/bot"
	"2fa-tgbot/internal/config"
	"log"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if err := bot.Start(cfg); err != nil {
		log.Fatal(err)
	}
}
