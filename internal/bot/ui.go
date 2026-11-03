package bot

import (
	"context"
	"fmt"

	"2fa-tgbot/internal/core"

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

// nameRulesText — правила имени сервиса (см. core.NormalizeServiceName).
var nameRulesText = fmt.Sprintf("⚠️ Имя должно быть от 1 до %d символов, "+
	"без управляющих символов и не начинаться с «/».", core.MaxServiceNameLen)

// secretPromptText — запрос секрета на шаге 2 flow добавления.
const secretPromptText = "Теперь введите секретный ключ (base32). Его показывают " +
	"при включении 2FA рядом с QR-кодом, обычно по ссылке «Не удаётся " +
	"отсканировать?». Пробелы и регистр не важны.\n\n" +
	"Сообщение с ключом будет сразу удалено."

// reply отправляет простое текстовое сообщение с очисткой предыдущего.
func (b *Bot) reply(ctx context.Context, chatID int64, text string) {
	b.sendWithCleanup(ctx, chatID, tgbotapi.NewMessage(chatID, text))
}

// sendHTML отправляет HTML-сообщение (с необязательной клавиатурой) с
// очисткой предыдущего. Пользовательские строки в text должны быть уже
// экранированы.
func (b *Bot) sendHTML(ctx context.Context, chatID int64, text string, markup *tgbotapi.InlineKeyboardMarkup) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	if markup != nil {
		msg.ReplyMarkup = *markup
	}
	return b.sendWithCleanup(ctx, chatID, msg)
}

// sendPrompt задаёт вопрос шага flow добавления с кнопкой отмены.
func (b *Bot) sendPrompt(ctx context.Context, chatID int64, text string) error {
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✖️ Отмена", cbMenu+"cancel"),
		),
	)
	return b.sendHTML(ctx, chatID, text, &markup)
}

func (b *Bot) sendHelp(ctx context.Context, chatID int64) {
	help := "🤖 <b>Доступные команды:</b>\n\n" +
		"/menu – открыть меню\n" +
		"/show – показать все сервисы\n" +
		"/cancel – отменить добавление сервиса\n" +
		"/help – помощь\n\n" +
		"Добавление и получение кодов — через кнопки меню.\n\n" +
		"<b>В группе</b> сервисы общие: коды доступны всем участникам, " +
		"а добавлять и удалять сервисы могут только администраторы. " +
		"Секрет вводится в личном чате с ботом и в группе не появляется."
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
// сообщение бота в этом чате (id хранится в StateStore). Ошибку отправки
// возвращает: большинству вызывающих она не нужна, но по ней, например,
// видно, что пользователь ещё не открывал личный чат с ботом.
func (b *Bot) sendWithCleanup(ctx context.Context, chatID int64, msg tgbotapi.Chattable) error {
	if lastID, ok, _ := b.state.GetLastBotMessage(ctx, chatID); ok {
		b.api.Request(tgbotapi.NewDeleteMessage(chatID, lastID))
	}

	switch m := msg.(type) {
	case tgbotapi.MessageConfig:
		sent, err := b.api.Send(m)
		if err != nil {
			b.log.Warn("send message failed", "err", err)
			return err
		}
		_ = b.state.SetLastBotMessage(ctx, chatID, sent.MessageID)
		return nil
	default:
		_, err := b.api.Send(msg)
		return err
	}
}
