package bot

import (
	"2fa-tgbot/internal/config"
	"2fa-tgbot/internal/repository"
	"2fa-tgbot/internal/totp"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func Start(cfg config.Config) error {

	bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		log.Println("Ошибка запуска бота")
		return err
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		switch update.Message.Command() {
		case "code":

			args := update.Message.CommandArguments()
			parts := strings.Fields(args)

			if len(parts) != 1 {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID,
					"Формат: /code <сервис> \nПример: /code github"))
				break
			}

			var totpSecret string = ""
			chat := update.Message.Chat
			serviceName := parts[0]

			if chat.IsGroup() || chat.IsSuperGroup() {
				totpSecret, err = repository.FindService(cfg.DBConn, serviceName, update.Message.Chat.ID)
				if err != nil {
					log.Println(err)
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка при поиске сервиса"))
					continue
				}
			} else {
				if chat.IsPrivate() {
					totpSecret, err = repository.FindService(cfg.DBConn, serviceName, update.Message.From.ID)
					if err != nil {
						log.Println(err)
						bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка при поиске сервиса"))
						continue
					} else {
						continue
					}
				}
			}

			code, err := totp.GenerateCode(totpSecret)
			if err != nil {
				log.Println(err)
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка генерации кода"))
				continue
			}
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Ваш код:<pre>"+code+"</pre>")
			msg.ParseMode = "HTML"
			bot.Send(msg)

		case "add":
			args := update.Message.CommandArguments()
			parts := strings.Fields(args)

			if len(parts) != 2 {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID,
					"Формат: /add <сервис> <секрет>\nПример: /add github AAAAABBBBCC"))
				break
			}

			serviceName := parts[0]
			secret := parts[1]

			chat := update.Message.Chat

			if chat.IsGroup() || chat.IsSuperGroup() {
				err = repository.AddService(cfg.DBConn, serviceName, secret, update.Message.Chat.ID)
				if err != nil {
					log.Println(err)
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка при добавлении сервиса"))
					continue
				}
			} else {
				if chat.IsPrivate() {
					err = repository.AddService(cfg.DBConn, serviceName, secret, update.Message.From.ID)
					if err != nil {
						log.Println(err)
						bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка при добавлении сервиса"))
						continue
					}
				} else {
					continue
				}
			}

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "✅ Сервис "+serviceName+" добавлен!")
			bot.Send(msg)

			bot.Request(tgbotapi.DeleteMessageConfig{
				ChatID:    update.Message.Chat.ID,
				MessageID: update.Message.MessageID,
			})

		case "show":
			chat := update.Message.Chat
			var serviceNames []string

			if chat.IsGroup() || chat.IsSuperGroup() {
				serviceNames, err = repository.AllService(cfg.DBConn, update.Message.Chat.ID)
				if err != nil {
					log.Println(err)
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка при чтении сервисов"))
					continue
				}
			} else {
				if chat.IsPrivate() {
					serviceNames, err = repository.AllService(cfg.DBConn, update.Message.From.ID)
					if err != nil {
						log.Println(err)
						bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка при чтении сервисов"))
						continue
					}
				} else {
					continue
				}
			}

			if len(serviceNames) == 0 {
				msg := tgbotapi.NewMessage(chat.ID, "⚠️ Нет доступных сервисов.")
				bot.Send(msg)
				continue
			}

			response := "📋 Список сервисов:\n"
			for _, name := range serviceNames {
				response += "- " + name + "\n"
			}

			msg := tgbotapi.NewMessage(chat.ID, response)
			bot.Send(msg)

		case "delete":
			args := update.Message.CommandArguments()
			parts := strings.Fields(args)

			if len(parts) != 1 {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID,
					"Формат: /delete <сервис>\nПример: /delete github"))
				break
			}

			serviceName := parts[0]
			chat := update.Message.Chat

			if chat.IsGroup() || chat.IsSuperGroup() {
				err = repository.DeleteService(cfg.DBConn, serviceName, update.Message.Chat.ID)
				if err != nil {
					log.Println(err)
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка при удалении сервиса"))
					continue
				}
			} else {
				if chat.IsPrivate() {
					err = repository.DeleteService(cfg.DBConn, serviceName, update.Message.From.ID)
					if err != nil {
						log.Println(err)
						bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка при удалении сервиса"))
						continue
					}
				} else {
					continue
				}
			}

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Сервис "+serviceName+" удален!")
			bot.Send(msg)

		case "help":
			helpText := "🤖 *Доступные команды:*\n\n" +
				"/add `<сервис>` `<секрет>` – добавить новый сервис с 2FA (TOTP)\n" +
				"Пример: `/add github JBSWY3DPEHPK3PXP`\n\n" +
				"/code `<сервис>` – получить одноразовый код для сервиса\n" +
				"Пример: `/code github`\n\n" +
				"/delete `<сервис>` – удалить сохранённый сервис\n" +
				"Пример: `/delete github`\n\n" +
				"/show – показать все сохранённые сервисы\n" +
				"/help – вывести эту справку"

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, helpText)
			msg.ParseMode = "Markdown"
			bot.Send(msg)

		default:
			continue
		}
	}

	return nil
}
