package bot

import (
	"2fa-tgbot/internal/config"
	"2fa-tgbot/internal/totp"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func Start(cfg config.Config) error {

	bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		return err
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		switch update.Message.Text {
		case "/code":
			code, err := totp.GenerateCode(cfg.TotpSecret)
			if err != nil {
				log.Println(err)
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка генерации кода"))
				continue
			}
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Ваш TOTP код:<pre>"+code+"</pre>")
			msg.ParseMode = "HTML"
			bot.Send(msg)

		default:
			continue
		}
	}

	return nil
}
