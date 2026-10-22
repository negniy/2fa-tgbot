package bot

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// reply отправляет простое текстовое сообщение с очисткой предыдущего.
func (b *Bot) reply(ctx context.Context, chatID int64, text string) {
	b.sendWithCleanup(ctx, chatID, tgbotapi.NewMessage(chatID, text))
}

func (b *Bot) sendHelp(chatID int64) {
	help := "🤖 *Доступные команды:*\n\n" +
		"/menu – открыть меню\n" +
		"/show – показать все сервисы\n" +
		"/help – помощь\n\n" +
		"Добавление и получение кодов — через кнопки меню."
	msg := tgbotapi.NewMessage(chatID, help)
	msg.ParseMode = "Markdown"
	b.api.Send(msg)
}

func (b *Bot) sendMainMenu(ctx context.Context, chatID int64) {
	buttons := [][]tgbotapi.InlineKeyboardButton{
		{tgbotapi.NewInlineKeyboardButtonData("📋 Мои сервисы", "menu:show")},
		{tgbotapi.NewInlineKeyboardButtonData("➕ Добавить", "menu:add")},
		{tgbotapi.NewInlineKeyboardButtonData("❓ Помощь", "menu:help")},
	}
	msg := tgbotapi.NewMessage(chatID, "📱 Главное меню:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
	b.sendWithCleanup(ctx, chatID, msg)
}

func (b *Bot) sendBackButton(ctx context.Context, chatID int64) {
	msg := tgbotapi.NewMessage(chatID, " ")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "menu:main"),
		),
	)
	b.sendWithCleanup(ctx, chatID, msg)
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
		if err == nil {
			_ = b.state.SetLastBotMessage(ctx, chatID, sent.MessageID)
		}
	default:
		b.api.Send(msg)
	}
}
