package bot

import (
	"strconv"
	"strings"
)

// Префиксы callback-данных инлайн-кнопок. Сервис адресуется по ID, а не по
// имени: Telegram ограничивает callback_data 64 байтами, и длинное имя
// ломало отправку всего списка. ID приходит от клиента и может быть
// подделан — поэтому сервисный слой всегда ищет секрет в паре с владельцем.
const (
	cbMenu          = "menu:"  // menu:<main|show|add|help|cancel>
	cbService       = "svc:"   // svc:<id> — экран действий с сервисом
	cbCode          = "code:"  // code:<id> — получить код
	cbDelete        = "del:"   // del:<id> — запрос подтверждения удаления
	cbDeleteConfirm = "delok:" // delok:<id> — подтверждённое удаление
)

// idCallback собирает callback-данные вида <prefix><id>.
func idCallback(prefix string, id int64) string {
	return prefix + strconv.FormatInt(id, 10)
}

// parseIDCallback извлекает ID из данных вида <prefix><id>. ok=false, если
// префикс не совпал или ID некорректен.
func parseIDCallback(data, prefix string) (id int64, ok bool) {
	rest, found := strings.CutPrefix(data, prefix)
	if !found {
		return 0, false
	}
	id, err := strconv.ParseInt(rest, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}
