# 🤖 2FA-TGBot — Телеграм-бот для генерации TOTP-кодов

Проект представляет собой Telegram-бота, который позволяет пользователям получать одноразовые коды 2FA (TOTP) по заранее сохранённым секретным ключам. Поддерживаются команды для добавления, удаления и отображения сервисов.

## Функциональность

- Генерация TOTP-кодов по секретному ключу (/code)
- Добавление новых сервисов с ключами (/add)
- Удаление сервисов (/delete)
- Просмотр списка сохранённых сервисов (/show)
- Отдельная база ключей на каждого пользователя или группу
- Поддержка PostgreSQL
- Простая и безопасная архитектура на Go

## Пример использования

```

/add github JBSWY3DPEHPK3PXP
/code github
/delete github
/show
/help

````

## Установка и запуск

### 1. Клонируйте репозиторий

```bash
git clone https://github.com/negniy/2fa-tgbot.git
cd 2fa-tgbot
````

### 2. Создайте `.env` файл

```env
TELEGRAM_TOKEN=ваш_токен_бота
DB_HOST=localhost
PORT=5432
USER=postgres
PASSWORD=yourpassword
DB_NAME=totpbot
```

### 3. Установите зависимости

```bash
go mod tidy
```

### 4. Запустите бота

```bash
go run ./cmd/main.go
```

При первом запуске автоматически создаётся таблица `secret` в вашей базе данных PostgreSQL.

## Архитектура проекта

```
2fa-tgbot/
├── cmd/              # Точка входа в приложение
├── internal/
│   ├── bot/          # Обработка команд Telegram-бота
│   ├── config/       # Загрузка конфигурации из .env
│   ├── repository/   # Работа с базой данных (PostgreSQL)
│   └── totp/         # Генерация TOTP-кодов
├── .env              # Переменные окружения (в .gitignore)
├── go.mod / go.sum   # Зависимости
└── README.md
```

## Зависимости

* [go-telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api)
* [pquerna/otp](https://github.com/pquerna/otp) — генерация TOTP
* [joho/godotenv](https://github.com/joho/godotenv) — загрузка .env
* PostgreSQL

## Безопасность

* Доступ к сервисам привязан к Telegram ID пользователя или группы
* Хранение секретов — в открытом виде (в будущем можно добавить шифрование)
* Сообщения с секретами удаляются после добавления (`/add`)

