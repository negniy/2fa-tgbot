package config

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type Config struct {
	TelegramToken string
	DBConn        *sql.DB
}

func Load() (Config, error) {

	envPath := filepath.Join(".", ".env")
	if err := godotenv.Load(envPath); err != nil {
		log.Println("⚠️ Не удалось загрузить .env:", err)
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DB_HOST"), os.Getenv("PORT"), os.Getenv("USER"),
		os.Getenv("PASSWORD"), os.Getenv("DB_NAME"))

	var err error
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return Config{}, err
	}

	err = db.Ping()
	if err != nil {
		return Config{}, err
	}

	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS secret (
			id SERIAL PRIMARY KEY,
			telegram_id BIGINT NOT NULL,
			tag TEXT NOT NULL,
			secret TEXT NOT NULL
		);
    `)
	if err != nil {
		return Config{}, err
	}

	return Config{
		TelegramToken: os.Getenv("TELEGRAM_TOKEN"),
		DBConn:        db,
	}, nil
}
