package bot

import (
	"2fa-tgbot/internal/config"
	"2fa-tgbot/internal/repository"
	"2fa-tgbot/internal/totp"
	"context"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type AddState struct {
	Step    int
	Service string
}

type LastCodeMsg struct {
	MessageID int
	Service   string
}

var userAddState = make(map[int64]*AddState)
var lastBotMessage = make(map[int64]int)
var lastCodeMessages = make(map[int64]LastCodeMsg)

func Start(ctx context.Context, cfg config.Config) error {
	bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		log.Println("Ошибка запуска бота")
		return err
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for {
		select {
		case update := <-updates:
			if update.Message != nil {
				baseHandleMessage(bot, cfg, &update)
			} else if update.CallbackQuery != nil {
				handleCallback(bot, update.CallbackQuery, cfg)
			}
		case <-ctx.Done():
			log.Println("Shutting down bot...")
		}
	}
}

func baseHandleMessage(bot *tgbotapi.BotAPI, cfg config.Config, update *tgbotapi.Update) {
	tgID := resolveID(update.Message)
	if state, ok := userAddState[tgID]; ok {
		handleAddFlow(bot, update.Message, cfg, state)
		return
	}
	handleMessage(bot, update.Message, cfg)
}

func handleMessage(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, cfg config.Config) {
	if !msg.IsCommand() {
		return
	}

	switch msg.Command() {
	case "menu":
		sendMainMenu(bot, msg.Chat.ID)

	case "show":
		showServices(bot, msg.Chat.ID, resolveID(msg), cfg)

	case "help":
		sendHelp(bot, msg.Chat.ID)

	default:
		sendWithCleanup(bot, msg.Chat.ID, tgbotapi.NewMessage(msg.Chat.ID, "Неизвестная команда. Введите /menu или /help"))
	}
}

func handleCallback(bot *tgbotapi.BotAPI, cb *tgbotapi.CallbackQuery, cfg config.Config) {
	data := cb.Data
	chatID := cb.Message.Chat.ID

	var contextID int64
	if cb.Message.Chat.IsPrivate() {
		contextID = cb.From.ID
	} else {
		contextID = cb.Message.Chat.ID
	}

	switch {
	case strings.HasPrefix(data, "menu:"):
		action := strings.TrimPrefix(data, "menu:")
		switch action {
		case "main":
			sendMainMenu(bot, chatID)
		case "show":
			showServices(bot, chatID, contextID, cfg)
		case "add":
			userAddState[contextID] = &AddState{Step: 1}
			sendWithCleanup(bot, chatID, tgbotapi.NewMessage(chatID, "Введите имя сервиса:"))
		case "help":
			sendHelp(bot, chatID)
			sendBackButton(bot, chatID)
		}

	case strings.HasPrefix(data, "service:"):
		service := strings.TrimPrefix(data, "service:")

		row := tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔐 Получить код", "code:"+service),
			tgbotapi.NewInlineKeyboardButtonData("🗑 Удалить", "delete:"+service),
		)
		back := tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "menu:show"),
		)

		msg := tgbotapi.NewMessage(chatID, "Выберите действие для *"+service+"*:")
		msg.ParseMode = "Markdown"
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(row, back)
		sendWithCleanup(bot, chatID, msg)

	case strings.HasPrefix(data, "code:"):
		service := strings.TrimPrefix(data, "code:")
		secret, err := repository.FindService(cfg.DBConn, service, contextID)
		if err != nil {
			sendWithCleanup(bot, chatID, tgbotapi.NewMessage(chatID, "Ошибка поиска секрета"))
			break
		}
		code, err := totp.GenerateCode(secret)
		if err != nil {
			sendWithCleanup(bot, chatID, tgbotapi.NewMessage(chatID, "Ошибка генерации"))
			break
		}

		text := "Код для " + service + ": <pre>" + code + "</pre>"
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "HTML"
		sent, err := bot.Send(msg)
		if err == nil {
			lastCodeMessages[contextID] = LastCodeMsg{
				MessageID: sent.MessageID,
				Service:   service,
			}

			// Рассчитываем оставшееся время жизни кода
			period := int64(30)
			now := time.Now()
			remaining := period - (now.Unix() % period)

			go func(chatID int64, msgID int, service string) {
				time.Sleep(time.Duration(remaining) * time.Second)

				edit := tgbotapi.NewEditMessageText(chatID, msgID,
					"🔄 *Код обновился, повторите попытку:*\n<pre>••••••</pre>")
				edit.ParseMode = "HTML"
				edit.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{
					InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
						{
							tgbotapi.NewInlineKeyboardButtonData("🔁 Отправить ещё раз", "code:"+service),
						},
					},
				}
				bot.Send(edit)
			}(chatID, sent.MessageID, service)
		}

	case strings.HasPrefix(data, "delete:"):
		service := strings.TrimPrefix(data, "delete:")
		err := repository.DeleteService(cfg.DBConn, service, contextID)
		if err != nil {
			sendWithCleanup(bot, chatID, tgbotapi.NewMessage(chatID, "Ошибка удаления"))
			break
		}
		sendWithCleanup(bot, chatID, tgbotapi.NewMessage(chatID, "❌ Сервис "+service+" удалён"))

		if prev, ok := lastCodeMessages[contextID]; ok && prev.Service == service {
			delete(lastCodeMessages, contextID)
		}
	}

	bot.Request(tgbotapi.NewCallback(cb.ID, ""))
}

func handleAddFlow(bot *tgbotapi.BotAPI, msg *tgbotapi.Message, cfg config.Config, state *AddState) {
	tgID := resolveID(msg)

	switch state.Step {
	case 1:
		state.Service = strings.TrimSpace(msg.Text)
		state.Step = 2
		sendWithCleanup(bot, msg.Chat.ID, tgbotapi.NewMessage(msg.Chat.ID, "Теперь введите секрет (base32):"))

	case 2:
		secret := strings.TrimSpace(msg.Text)
		err := repository.AddService(cfg.DBConn, state.Service, secret, tgID)
		if err != nil {
			sendWithCleanup(bot, msg.Chat.ID, tgbotapi.NewMessage(msg.Chat.ID, "❌ Ошибка при добавлении сервиса"))
		} else {
			sendWithCleanup(bot, msg.Chat.ID, tgbotapi.NewMessage(msg.Chat.ID, "✅ Сервис "+state.Service+" добавлен!"))
			showServices(bot, msg.Chat.ID, tgID, cfg)
		}
		delete(userAddState, tgID)
	}
}

func showServices(bot *tgbotapi.BotAPI, chatID int64, userID int64, cfg config.Config) {
	services, err := repository.AllService(cfg.DBConn, userID)
	if err != nil || len(services) == 0 {
		sendWithCleanup(bot, chatID, tgbotapi.NewMessage(chatID, "⚠️ Нет доступных сервисов"))
		return
	}
	var buttons [][]tgbotapi.InlineKeyboardButton
	for _, name := range services {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔧 "+name, "service:"+name),
		))
	}
	buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "menu:main"),
	))
	msg := tgbotapi.NewMessage(chatID, "📋 Ваши сервисы:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
	sendWithCleanup(bot, chatID, msg)
}

func sendHelp(bot *tgbotapi.BotAPI, chatID int64) {
	help := "🤖 *Доступные команды:*\n\n" +
		"/menu – открыть меню\n" +
		"/add `<сервис>` `<секрет>` – добавить вручную\n" +
		"/code `<сервис>` – получить код\n" +
		"/delete `<сервис>` – удалить\n" +
		"/show – показать все\n" +
		"/help – помощь"
	msg := tgbotapi.NewMessage(chatID, help)
	msg.ParseMode = "Markdown"
	bot.Send(msg)
}

func sendMainMenu(bot *tgbotapi.BotAPI, chatID int64) {
	buttons := [][]tgbotapi.InlineKeyboardButton{
		{tgbotapi.NewInlineKeyboardButtonData("📋 Мои сервисы", "menu:show")},
		{tgbotapi.NewInlineKeyboardButtonData("➕ Добавить", "menu:add")},
		{tgbotapi.NewInlineKeyboardButtonData("❓ Помощь", "menu:help")},
	}
	msg := tgbotapi.NewMessage(chatID, "📱 Главное меню:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
	sendWithCleanup(bot, chatID, msg)
}

func sendBackButton(bot *tgbotapi.BotAPI, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, " ")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "menu:main"),
		),
	)
	sendWithCleanup(bot, chatID, msg)
}

func sendWithCleanup(bot *tgbotapi.BotAPI, chatID int64, msg tgbotapi.Chattable) {
	if lastID, ok := lastBotMessage[chatID]; ok {
		bot.Request(tgbotapi.NewDeleteMessage(chatID, lastID))
	}

	switch m := msg.(type) {
	case tgbotapi.MessageConfig:
		sent, err := bot.Send(m)
		if err == nil {
			lastBotMessage[chatID] = sent.MessageID
		}
	default:
		bot.Send(msg)
	}
}

func resolveID(msg *tgbotapi.Message) int64 {
	if msg.Chat.IsPrivate() {
		return msg.From.ID
	}
	return msg.Chat.ID
}
