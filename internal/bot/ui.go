package bot

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Все сообщения с разметкой отправляются в режиме HTML, а пользовательские
// строки (имена сервисов) экранируются html.EscapeString. Markdown не
// используем: в нём «_» и «*» в имени ломали разбор, и сообщение не уходило.

// msgTemporarilyUnavailable — ответ при сбое инфраструктуры (Redis/БД).
const msgTemporarilyUnavailable = "⏳ Сервис временно недоступен, попробуйте позже."

// welcomeText — приветствие на /start.
const welcomeText = "👋 Привет! Я храню секреты двухфакторной аутентификации " +
	"(в зашифрованном виде) и выдаю по ним одноразовые коды."

// reply отправляет простое текстовое сообщение с очисткой предыдущего.
func (b *Bot) reply(ctx context.Context, chatID int64, text string) {
	b.sendWithCleanup(ctx, chatID, tgbotapi.NewMessage(chatID, text))
}

// sendHTML отправляет HTML-сообщение (с необязательной клавиатурой) с
// очисткой предыдущего. Пользовательские строки в text должны быть уже
// экранированы.
func (b *Bot) sendHTML(ctx context.Context, chatID int64, text string, markup *tgbotapi.InlineKeyboardMarkup) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	if markup != nil {
		msg.ReplyMarkup = *markup
	}
	b.sendWithCleanup(ctx, chatID, msg)
}

// sendPrompt задаёт вопрос шага flow добавления с кнопкой отмены.
func (b *Bot) sendPrompt(ctx context.Context, chatID int64, text string) {
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✖️ Отмена", cbMenu+"cancel"),
		),
	)
	b.sendHTML(ctx, chatID, text, &markup)
}

func (b *Bot) sendHelp(ctx context.Context, chatID int64) {
	help := "🤖 <b>Доступные команды:</b>\n\n" +
		"/menu – открыть меню\n" +
		"/show – показать все сервисы\n" +
		"/cancel – отменить добавление сервиса\n" +
		"/help – помощь\n\n" +
		"Добавление и получение кодов — через кнопки меню."
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", cbMenu+"main"),
		),
	)
	b.sendHTML(ctx, chatID, help, &markup)
}

// sendMainMenu показывает главное меню. header (HTML) выводится над ним.
func (b *Bot) sendMainMenu(ctx context.Context, chatID int64, header string) {
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("📋 Мои сервисы", cbMenu+"show")),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("➕ Добавить", cbMenu+"add")),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("❓ Помощь", cbMenu+"help")),
	)
	text := "📱 Главное меню:"
	if header != "" {
		text = header + "\n\n" + text
	}
	b.sendHTML(ctx, chatID, text, &markup)
}

// sendWithCleanup отправляет сообщение, предварительно удалив предыдущее
// сообщение бота в этом чате (id хранится в StateStore).
func (b *Bot) sendWithCleanup(ctx context.Context, chatID int64, msg tgbotapi.Chattable) {
	if lastID, ok, _ := b.state.GetLastBotMessage(ctx, chatID); ok {
		b.api.Request(tgbotapi.NewDeleteMessage(chatID, lastID))
	}

	switch m := msg.(type) {
	case tgbotapi.MessageConfig:
		sent, err := b.api.Send(m)
		if err != nil {
			b.log.Warn("send message failed", "err", err)
			return
		}
		_ = b.state.SetLastBotMessage(ctx, chatID, sent.MessageID)
	default:
		b.api.Send(msg)
	}
}
