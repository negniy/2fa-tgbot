package bot

import (
	"context"
	"errors"
	"strings"
	"time"

	"2fa-tgbot/internal/core"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleMessage обрабатывает входящие сообщения: сначала проверяет активный
// flow добавления, иначе — команды.
func (b *Bot) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	owner, ok := b.resolveOwner(ctx, msg.Chat, msg.From.ID)
	if !ok {
		b.reply(ctx, msg.Chat.ID, "Работа в группах отключена.")
		return
	}

	// Активный пошаговый ввод?
	st, exists, err := b.state.GetAddState(ctx, owner)
	switch {
	case err != nil && !msg.IsCommand():
		// Не знаем, идёт ли ввод: сообщение может оказаться секретом.
		b.log.Error("get add state failed", "err", err)
		b.handleUnknownFlowMessage(ctx, msg)
		return
	case err == nil && exists:
		b.handleAddFlow(ctx, msg, owner, *st)
		return
	}

	if !msg.IsCommand() {
		return
	}

	switch msg.Command() {
	case "menu":
		b.sendMainMenu(ctx, msg.Chat.ID)
	case "show":
		b.showServices(ctx, msg.Chat.ID, owner)
	case "help":
		b.sendHelp(msg.Chat.ID)
	default:
		b.reply(ctx, msg.Chat.ID, "Неизвестная команда. Введите /menu или /help")
	}
}

// handleCallback обрабатывает нажатия инлайн-кнопок.
func (b *Bot) handleCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	defer b.api.Request(tgbotapi.NewCallback(cb.ID, ""))

	chatID := cb.Message.Chat.ID
	owner, ok := b.resolveOwner(ctx, cb.Message.Chat, cb.From.ID)
	if !ok {
		b.reply(ctx, chatID, "Работа в группах отключена.")
		return
	}

	data := cb.Data
	switch {
	case strings.HasPrefix(data, "menu:"):
		b.handleMenu(ctx, chatID, owner, strings.TrimPrefix(data, "menu:"))
	case strings.HasPrefix(data, "service:"):
		b.showServiceActions(ctx, chatID, strings.TrimPrefix(data, "service:"))
	case strings.HasPrefix(data, "code:"):
		b.sendCode(ctx, chatID, owner, strings.TrimPrefix(data, "code:"))
	case strings.HasPrefix(data, "delete:"):
		b.deleteService(ctx, chatID, owner, strings.TrimPrefix(data, "delete:"))
	}
}

func (b *Bot) handleMenu(ctx context.Context, chatID int64, owner core.Owner, action string) {
	switch action {
	case "main":
		b.sendMainMenu(ctx, chatID)
	case "show":
		b.showServices(ctx, chatID, owner)
	case "add":
		if err := b.state.SetAddState(ctx, owner, core.AddState{Step: 1}); err != nil {
			b.log.Error("set add state failed", "err", err)
			b.reply(ctx, chatID, msgTemporarilyUnavailable)
			return
		}
		b.reply(ctx, chatID, "Введите имя сервиса:")
	case "help":
		b.sendHelp(chatID)
		b.sendBackButton(ctx, chatID)
	}
}

func (b *Bot) showServiceActions(ctx context.Context, chatID int64, service string) {
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
	b.sendWithCleanup(ctx, chatID, msg)
}

func (b *Bot) sendCode(ctx context.Context, chatID int64, owner core.Owner, service string) {
	res, err := b.svc.GetCode(ctx, owner, service)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			b.reply(ctx, chatID, "Сервис не найден")
		} else {
			b.reply(ctx, chatID, "Ошибка получения кода")
		}
		return
	}

	text := "Код для " + service + ": <pre>" + res.Code + "</pre>"
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	sent, err := b.api.Send(msg)
	if err != nil {
		return
	}

	_ = b.state.SetLastCodeMessage(ctx, owner, core.LastCodeMsg{
		MessageID: sent.MessageID,
		Service:   service,
	})

	// По истечении окна кода затираем сообщение.
	go b.expireCode(chatID, sent.MessageID, service, res.SecondsRemaining)
}

// expireCode затирает сообщение с кодом по истечении TOTP-окна.
func (b *Bot) expireCode(chatID int64, msgID int, service string, after int64) {
	time.Sleep(time.Duration(after) * time.Second)
	edit := tgbotapi.NewEditMessageText(chatID, msgID,
		"🔄 <b>Код обновился, повторите попытку:</b>\n<pre>••••••</pre>")
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
			{tgbotapi.NewInlineKeyboardButtonData("🔁 Отправить ещё раз", "code:"+service)},
		},
	}
	b.api.Send(edit)
}

func (b *Bot) deleteService(ctx context.Context, chatID int64, owner core.Owner, service string) {
	err := b.svc.DeleteSecret(ctx, owner, service)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			b.reply(ctx, chatID, "Сервис не найден")
		} else {
			b.reply(ctx, chatID, "Ошибка удаления")
		}
		return
	}
	b.reply(ctx, chatID, "❌ Сервис "+service+" удалён")

	if prev, ok, _ := b.state.GetLastCodeMessage(ctx, owner); ok && prev.Service == service {
		_ = b.state.DeleteLastCodeMessage(ctx, owner)
	}
}

// handleAddFlow ведёт пошаговое добавление сервиса.
func (b *Bot) handleAddFlow(ctx context.Context, msg *tgbotapi.Message, owner core.Owner, st core.AddState) {
	switch st.Step {
	case 1:
		st.Service = strings.TrimSpace(msg.Text)
		st.Step = 2
		if err := b.state.SetAddState(ctx, owner, st); err != nil {
			b.log.Error("set add state failed", "err", err)
			b.reply(ctx, msg.Chat.ID, msgTemporarilyUnavailable)
			return
		}
		b.reply(ctx, msg.Chat.ID, "Теперь введите секрет (base32):")

	case 2:
		rawSecret := core.NewSecret(strings.TrimSpace(msg.Text))

		// Удаляем сообщение пользователя с секретом сразу (в группах
		// требуются права на удаление).
		deleted := b.deleteUserMessage(msg)

		err := b.svc.AddSecret(ctx, owner, st.Service, rawSecret)
		if derr := b.state.DeleteAddState(ctx, owner); derr != nil {
			// Не критично: состояние истечёт по TTL.
			b.log.Warn("delete add state failed", "err", derr)
		}
		if !deleted {
			b.warnSecretNotDeleted(msg.Chat.ID)
		}

		switch {
		case err == nil:
			b.reply(ctx, msg.Chat.ID, "✅ Сервис "+st.Service+" добавлен!")
			b.showServices(ctx, msg.Chat.ID, owner)
		case errors.Is(err, core.ErrAlreadyExists):
			b.reply(ctx, msg.Chat.ID, "⚠️ Сервис с таким именем уже существует")
		default:
			b.reply(ctx, msg.Chat.ID, "❌ Ошибка при добавлении сервиса")
		}
	}
}

// handleUnknownFlowMessage обрабатывает текст, пришедший, когда состояние
// flow добавления прочитать не удалось. В личке сообщение может быть
// секретом, поэтому удаляем его (обычный текст боту в личку не нужен, так
// что лишнее удаление безвредно). В группах чужие сообщения не трогаем.
func (b *Bot) handleUnknownFlowMessage(ctx context.Context, msg *tgbotapi.Message) {
	if !msg.Chat.IsPrivate() {
		return
	}
	if !b.deleteUserMessage(msg) {
		b.warnSecretNotDeleted(msg.Chat.ID)
	}
	b.reply(ctx, msg.Chat.ID, msgTemporarilyUnavailable)
}

// deleteUserMessage удаляет сообщение пользователя и сообщает, удалось ли.
func (b *Bot) deleteUserMessage(msg *tgbotapi.Message) bool {
	if _, err := b.api.Request(tgbotapi.NewDeleteMessage(msg.Chat.ID, msg.MessageID)); err != nil {
		b.log.Warn("delete user message failed", "chat_type", msg.Chat.Type, "err", err)
		return false
	}
	return true
}

// warnSecretNotDeleted просит пользователя удалить сообщение с секретом
// вручную. Отправляется в обход sendWithCleanup, чтобы предупреждение не
// стёрлось следующим ответом бота.
func (b *Bot) warnSecretNotDeleted(chatID int64) {
	b.api.Send(tgbotapi.NewMessage(chatID,
		"⚠️ Не удалось удалить сообщение с секретом — удалите его вручную. "+
			"В группе боту нужно право администратора на удаление сообщений."))
}

func (b *Bot) showServices(ctx context.Context, chatID int64, owner core.Owner) {
	services, err := b.svc.ListSecrets(ctx, owner)
	if err != nil || len(services) == 0 {
		b.reply(ctx, chatID, "⚠️ Нет доступных сервисов")
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
	b.sendWithCleanup(ctx, chatID, msg)
}
