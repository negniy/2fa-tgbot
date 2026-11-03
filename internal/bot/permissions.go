package bot

import (
	"context"
	"html"
	"strings"

	"2fa-tgbot/internal/core"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Права в группах: получать коды и смотреть список могут все участники,
// добавлять и удалять сервисы — только администраторы чата. Права
// проверяются через getChatMember при каждом таком действии (а не
// кэшируются), чтобы снятие админки действовало сразу.

// adminOnlyText — ответ участнику без прав администратора.
const adminOnlyText = "Добавлять и удалять сервисы группы могут только её администраторы."

// startAddPayload — параметр deep link (t.me/<bot>?start=add), которым
// пользователь из группы переходит в личку продолжить добавление.
const startAddPayload = "add"

// requiresGroupAdmin сообщает, требует ли действие кнопки прав
// администратора, когда оно выполняется в группе.
func requiresGroupAdmin(data string) bool {
	return data == cbMenu+"add" ||
		strings.HasPrefix(data, cbDelete) ||
		strings.HasPrefix(data, cbDeleteConfirm)
}

// isAdminMember сообщает, является ли участник администратором чата.
func isAdminMember(m tgbotapi.ChatMember) bool {
	return m.IsCreator() || m.IsAdministrator()
}

// isChatAdmin проверяет через Telegram, администратор ли пользователь в чате.
func (b *Bot) isChatAdmin(chatID, userID int64) (bool, error) {
	m, err := b.api.GetChatMember(tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{ChatID: chatID, UserID: userID},
	})
	if err != nil {
		return false, err
	}
	return isAdminMember(m), nil
}

// startGroupAdd начинает добавление сервиса для группы. Ввод имени и
// секрета переносится в личку администратора: секрет в группе увидели бы
// все участники, а без отключённого privacy mode бот и не получил бы
// обычный текст из группы.
func (b *Bot) startGroupAdd(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	group := cb.Message.Chat
	userID, err := b.users.ResolveTelegram(ctx, cb.From.ID)
	if err != nil {
		b.log.Error("resolve telegram user failed", "err", err)
		b.reply(ctx, group.ID, msgTemporarilyUnavailable)
		return
	}
	st := core.AddState{Step: 1, GroupChatID: group.ID, GroupTitle: group.Title}
	if err := b.state.SetAddState(ctx, core.NewUserOwner(userID), st); err != nil {
		b.log.Error("set add state failed", "err", err)
		b.reply(ctx, group.ID, msgTemporarilyUnavailable)
		return
	}

	// Личный чат с пользователем имеет chat_id, равный его user_id. Если
	// пользователь ещё не запускал бота, Telegram не даст написать первым —
	// тогда остаётся только ссылка.
	text := "✉️ Продолжите добавление сервиса в личных сообщениях с ботом."
	if err := b.sendPrompt(ctx, cb.From.ID, addPromptText(st)); err != nil {
		text = "✉️ Чтобы добавить сервис, откройте личный чат с ботом: " +
			"вводить секрет в группе небезопасно."
	}
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("Открыть чат с ботом",
				"https://t.me/"+b.api.Self.UserName+"?start="+startAddPayload),
		),
	)
	b.sendHTML(ctx, group.ID, text, &markup)
}

// resumeAddFlow повторяет вопрос текущего шага flow добавления (переход по
// deep link из группы). Если flow нет — показывает обычное приветствие.
func (b *Bot) resumeAddFlow(ctx context.Context, chatID int64, owner core.Owner) {
	st, exists, err := b.state.GetAddState(ctx, owner)
	switch {
	case err != nil:
		b.log.Error("get add state failed", "err", err)
		b.reply(ctx, chatID, msgTemporarilyUnavailable)
	case !exists:
		b.sendMainMenu(ctx, chatID, welcomeText)
	case st.Step == 2:
		b.sendPrompt(ctx, chatID, secretPromptText)
	default:
		b.sendPrompt(ctx, chatID, addPromptText(*st))
	}
}

// addPromptText — вопрос шага 1 с указанием, куда добавляется сервис.
func addPromptText(st core.AddState) string {
	if st.ForGroup() {
		return "➕ Добавление сервиса в группу «<b>" + html.EscapeString(st.GroupTitle) +
			"</b>».\n\nВведите имя сервиса:"
	}
	return "Введите имя сервиса:"
}
