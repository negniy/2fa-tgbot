package core

// OwnerKind различает, кому принадлежат секреты: конкретному пользователю
// (личка с ботом или веб-аккаунт) или группе Telegram (общие секреты).
type OwnerKind string

const (
	OwnerUser  OwnerKind = "user"
	OwnerGroup OwnerKind = "group"
)

// Owner — value-объект, отвечающий на вопрос «чьи это секреты».
// Для OwnerUser поле ID — это внутренний users.id (НЕ telegram_id).
// Для OwnerGroup поле ID — это chat_id группы Telegram.
type Owner struct {
	Kind OwnerKind
	ID   int64
}

// NewUserOwner строит владельца-пользователя по внутреннему user_id.
func NewUserOwner(userID int64) Owner {
	return Owner{Kind: OwnerUser, ID: userID}
}

// NewGroupOwner строит владельца-группу по chat_id.
func NewGroupOwner(chatID int64) Owner {
	return Owner{Kind: OwnerGroup, ID: chatID}
}

// IsUser сообщает, принадлежит ли владелец конкретному человеку.
func (o Owner) IsUser() bool { return o.Kind == OwnerUser }

// IsGroup сообщает, принадлежит ли владелец группе.
func (o Owner) IsGroup() bool { return o.Kind == OwnerGroup }
