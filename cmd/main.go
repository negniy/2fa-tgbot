package main

import (
	"2fa-tgbot/internal/bot"
	"2fa-tgbot/internal/config"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Сonfig error: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)

	go func() {
		errCh <- bot.Start(ctx, cfg)
	}()

	select {
	case <-ctx.Done():
		log.Println("Shutdown signal received")

	case err := <-errCh:
		if err != nil {
			log.Printf("Bot stopped with error: %v", err)
		}
	}

	log.Println("Application stopped gracefully")
}
