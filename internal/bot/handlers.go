package bot

import (
	"context"
	"errors"
	"html"
	"strings"
	"time"

	"2fa-tgbot/internal/core"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleMessage обрабатывает входящие сообщения: обычный текст — шаг
// активного flow добавления, команда — команда.
func (b *Bot) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	owner, ok := b.resolveOwner(ctx, msg.Chat, msg.From.ID)
	if !ok {
		b.reply(ctx, msg.Chat.ID, "Работа в группах отключена.")
		return
	}

	if !msg.IsCommand() {
		st, exists, err := b.state.GetAddState(ctx, owner)
		switch {
		case err != nil:
			// Не знаем, идёт ли ввод: сообщение может оказаться секретом.
			b.log.Error("get add state failed", "err", err)
			b.handleUnknownFlowMessage(ctx, msg)
		case exists:
			b.handleAddFlow(ctx, msg, owner, *st)
		}
		return
	}

	// Команда прерывает незавершённый ввод: иначе следующий текст был бы
	// принят за имя сервиса или секрет.
	b.cancelAddFlow(ctx, owner)

	switch msg.Command() {
	case "start":
		b.sendMainMenu(ctx, msg.Chat.ID, welcomeText)
	case "menu":
		b.sendMainMenu(ctx, msg.Chat.ID, "")
	case "show":
		b.showServices(ctx, msg.Chat.ID, owner, "")
	case "cancel":
		b.sendMainMenu(ctx, msg.Chat.ID, "Действие отменено.")
	case "help":
		b.sendHelp(ctx, msg.Chat.ID)
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
	// Любая кнопка, кроме «Добавить», означает, что пользователь ушёл из
	// flow добавления, — сбрасываем его, чтобы следующий текст не был
	// принят за имя сервиса или секрет.
	if data != cbMenu+"add" {
		b.cancelAddFlow(ctx, owner)
	}

	if action, found := strings.CutPrefix(data, cbMenu); found {
		b.handleMenu(ctx, chatID, owner, action)
		return
	}
	if id, ok := parseIDCallback(data, cbService); ok {
		b.showServiceActions(ctx, chatID, owner, id)
	} else if id, ok := parseIDCallback(data, cbCode); ok {
		b.sendCode(ctx, chatID, owner, id)
	} else if id, ok := parseIDCallback(data, cbDelete); ok {
		b.confirmDelete(ctx, chatID, owner, id)
	} else if id, ok := parseIDCallback(data, cbDeleteConfirm); ok {
		b.deleteService(ctx, chatID, owner, id)
	}
}

func (b *Bot) handleMenu(ctx context.Context, chatID int64, owner core.Owner, action string) {
	switch action {
	case "main":
		b.sendMainMenu(ctx, chatID, "")
	case "show":
		b.showServices(ctx, chatID, owner, "")
	case "add":
		if err := b.state.SetAddState(ctx, owner, core.AddState{Step: 1}); err != nil {
			b.log.Error("set add state failed", "err", err)
			b.reply(ctx, chatID, msgTemporarilyUnavailable)
			return
		}
		b.sendPrompt(ctx, chatID, "Введите имя сервиса:")
	case "cancel":
		b.sendMainMenu(ctx, chatID, "Действие отменено.")
	case "help":
		b.sendHelp(ctx, chatID)
	}
}

// cancelAddFlow сбрасывает незавершённый flow добавления владельца. Ошибка
// не критична: состояние всё равно истечёт по TTL.
func (b *Bot) cancelAddFlow(ctx context.Context, owner core.Owner) {
	if err := b.state.DeleteAddState(ctx, owner); err != nil {
		b.log.Warn("delete add state failed", "err", err)
	}
}

func (b *Bot) showServiceActions(ctx context.Context, chatID int64, owner core.Owner, id int64) {
	info, err := b.svc.GetSecretInfo(ctx, owner, id)
	if err != nil {
		b.replyServiceError(ctx, chatID, err)
		return
	}
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔐 Получить код", idCallback(cbCode, id)),
			tgbotapi.NewInlineKeyboardButtonData("🗑 Удалить", idCallback(cbDelete, id)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", cbMenu+"show"),
		),
	)
	b.sendHTML(ctx, chatID, "Выберите действие для <b>"+html.EscapeString(info.Tag)+"</b>:", &markup)
}

func (b *Bot) sendCode(ctx context.Context, chatID int64, owner core.Owner, id int64) {
	res, err := b.svc.GetCode(ctx, owner, id)
	if err != nil {
		b.replyServiceError(ctx, chatID, err)
		return
	}

	text := "Код для <b>" + html.EscapeString(res.Tag) + "</b>: <pre>" + res.Code + "</pre>"
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	sent, err := b.api.Send(msg)
	if err != nil {
		b.log.Warn("send code failed", "err", err)
		return
	}

	_ = b.state.SetLastCodeMessage(ctx, owner, core.LastCodeMsg{
		MessageID: sent.MessageID,
		SecretID:  id,
	})

	// По истечении окна кода затираем сообщение.
	go b.expireCode(chatID, sent.MessageID, id, res.SecondsRemaining)
}

// expireCode затирает сообщение с кодом по истечении TOTP-окна.
func (b *Bot) expireCode(chatID int64, msgID int, id int64, after int64) {
	time.Sleep(time.Duration(after) * time.Second)
	edit := tgbotapi.NewEditMessageText(chatID, msgID,
		"🔄 <b>Код обновился, повторите попытку:</b>\n<pre>••••••</pre>")
	edit.ParseMode = tgbotapi.ModeHTML
	edit.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
			{tgbotapi.NewInlineKeyboardButtonData("🔁 Отправить ещё раз", idCallback(cbCode, id))},
		},
	}
	b.api.Send(edit)
}

// confirmDelete спрашивает подтверждение: удаление 2FA-секрета необратимо
// и может лишить пользователя доступа к аккаунту.
func (b *Bot) confirmDelete(ctx context.Context, chatID int64, owner core.Owner, id int64) {
	info, err := b.svc.GetSecretInfo(ctx, owner, id)
	if err != nil {
		b.replyServiceError(ctx, chatID, err)
		return
	}
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🗑 Да, удалить", idCallback(cbDeleteConfirm, id)),
			tgbotapi.NewInlineKeyboardButtonData("Отмена", idCallback(cbService, id)),
		),
	)
	b.sendHTML(ctx, chatID,
		"Удалить сервис <b>"+html.EscapeString(info.Tag)+"</b>?\n\n"+
			"Получить коды для него больше не получится. Убедитесь, что 2FA "+
			"отключена в самом сервисе или у вас есть резервные коды.",
		&markup)
}

func (b *Bot) deleteService(ctx context.Context, chatID int64, owner core.Owner, id int64) {
	tag, err := b.svc.DeleteSecret(ctx, owner, id)
	if err != nil {
		b.replyServiceError(ctx, chatID, err)
		return
	}

	if prev, ok, _ := b.state.GetLastCodeMessage(ctx, owner); ok && prev.SecretID == id {
		_ = b.state.DeleteLastCodeMessage(ctx, owner)
	}

	b.showServices(ctx, chatID, owner, "❌ Сервис <b>"+html.EscapeString(tag)+"</b> удалён.")
}

// handleAddFlow ведёт пошаговое добавление сервиса.
func (b *Bot) handleAddFlow(ctx context.Context, msg *tgbotapi.Message, owner core.Owner, st core.AddState) {
	switch st.Step {
	case 1:
		// Имя проверяем сразу, чтобы не просить секрет впустую.
		name, err := b.svc.CheckNewName(ctx, owner, msg.Text)
		switch {
		case errors.Is(err, core.ErrInvalidName):
			b.sendPrompt(ctx, msg.Chat.ID, nameRulesText+"\n\nВведите имя сервиса:")
			return
		case errors.Is(err, core.ErrAlreadyExists):
			b.sendPrompt(ctx, msg.Chat.ID, "⚠️ Сервис <b>"+html.EscapeString(name)+
				"</b> уже есть. Введите другое имя:")
			return
		case err != nil:
			b.log.Error("check service name failed", "err", err)
			b.reply(ctx, msg.Chat.ID, msgTemporarilyUnavailable)
			return
		}

		st.Service = name
		st.Step = 2
		if err := b.state.SetAddState(ctx, owner, st); err != nil {
			b.log.Error("set add state failed", "err", err)
			b.reply(ctx, msg.Chat.ID, msgTemporarilyUnavailable)
			return
		}
		b.sendPrompt(ctx, msg.Chat.ID, secretPromptText)

	case 2:
		rawSecret := core.NewSecret(msg.Text)

		// Удаляем сообщение пользователя с секретом сразу (в группах
		// требуются права на удаление).
		deleted := b.deleteUserMessage(msg)
		if !deleted {
			b.warnSecretNotDeleted(msg.Chat.ID)
		}

		err := b.svc.AddSecret(ctx, owner, st.Service, rawSecret)
		if errors.Is(err, core.ErrInvalidSecret) {
			// Опечатка в ключе — частый случай: остаёмся на шаге 2 и
			// просим ввести секрет ещё раз.
			b.sendPrompt(ctx, msg.Chat.ID, "⚠️ Это не похоже на секретный ключ.\n\n"+secretPromptText)
			return
		}
		b.cancelAddFlow(ctx, owner)

		switch {
		case err == nil:
			b.showServices(ctx, msg.Chat.ID, owner,
				"✅ Сервис <b>"+html.EscapeString(st.Service)+"</b> добавлен!")
		case errors.Is(err, core.ErrAlreadyExists):
			b.reply(ctx, msg.Chat.ID, "⚠️ Сервис с таким именем уже существует")
		default:
			b.log.Error("add secret failed", "err", err)
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

// showServices показывает список сервисов владельца. header (HTML, уже
// экранированный) выводится над списком — например, результат действия.
func (b *Bot) showServices(ctx context.Context, chatID int64, owner core.Owner, header string) {
	services, err := b.svc.ListSecrets(ctx, owner)
	if err != nil {
		b.log.Error("list secrets failed", "err", err)
		b.reply(ctx, chatID, msgTemporarilyUnavailable)
		return
	}
	if header != "" {
		header += "\n\n"
	}

	var buttons [][]tgbotapi.InlineKeyboardButton
	for _, s := range services {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔧 "+s.Tag, idCallback(cbService, s.ID)),
		))
	}
	text := header + "📋 Ваши сервисы:"
	if len(services) == 0 {
		text = header + "Сохранённых сервисов пока нет."
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Добавить", cbMenu+"add"),
		))
	}
	buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", cbMenu+"main"),
	))
	markup := tgbotapi.NewInlineKeyboardMarkup(buttons...)
	b.sendHTML(ctx, chatID, text, &markup)
}

// replyServiceError показывает пользователю ошибку сервисного слоя.
func (b *Bot) replyServiceError(ctx context.Context, chatID int64, err error) {
	if errors.Is(err, core.ErrNotFound) {
		b.reply(ctx, chatID, "Сервис не найден")
		return
	}
	b.log.Error("service operation failed", "err", err)
	b.reply(ctx, chatID, "❌ Ошибка, попробуйте позже")
}
