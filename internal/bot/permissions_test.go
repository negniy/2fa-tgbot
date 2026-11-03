package bot

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestRequiresGroupAdmin(t *testing.T) {
	admin := []string{
		cbMenu + "add",
		idCallback(cbDelete, 1),
		idCallback(cbDeleteConfirm, 1),
	}
	for _, data := range admin {
		if !requiresGroupAdmin(data) {
			t.Errorf("%q must require admin", data)
		}
	}

	everyone := []string{
		cbMenu + "main",
		cbMenu + "show",
		cbMenu + "help",
		cbMenu + "cancel",
		idCallback(cbService, 1),
		idCallback(cbCode, 1),
	}
	for _, data := range everyone {
		if requiresGroupAdmin(data) {
			t.Errorf("%q must be available to every member", data)
		}
	}
}

func TestIsAdminMember(t *testing.T) {
	cases := map[string]bool{
		"creator":       true,
		"administrator": true,
		"member":        false,
		"restricted":    false,
		"left":          false,
		"kicked":        false,
	}
	for status, want := range cases {
		if got := isAdminMember(tgbotapi.ChatMember{Status: status}); got != want {
			t.Errorf("status %q: got %v, want %v", status, got, want)
		}
	}
}
