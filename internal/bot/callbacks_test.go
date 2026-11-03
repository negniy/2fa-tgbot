package bot

import (
	"math"
	"testing"
)

func TestIDCallbackRoundTrip(t *testing.T) {
	for _, prefix := range []string{cbService, cbCode, cbDelete, cbDeleteConfirm} {
		data := idCallback(prefix, 42)
		id, ok := parseIDCallback(data, prefix)
		if !ok || id != 42 {
			t.Fatalf("%q: got id=%d ok=%v", data, id, ok)
		}
	}
}

// TestIDCallbackFitsLimit: даже максимальный ID укладывается в лимит
// Telegram на callback_data (64 байта).
func TestIDCallbackFitsLimit(t *testing.T) {
	for _, prefix := range []string{cbService, cbCode, cbDelete, cbDeleteConfirm} {
		if n := len(idCallback(prefix, math.MaxInt64)); n > 64 {
			t.Fatalf("%q: callback data is %d bytes", prefix, n)
		}
	}
}

func TestParseIDCallbackRejectsGarbage(t *testing.T) {
	cases := []struct{ data, prefix string }{
		{"code:", cbCode},
		{"code:abc", cbCode},
		{"code:-5", cbCode},
		{"code:0", cbCode},
		{"code:1 OR 1=1", cbCode},
		{"del:5", cbCode},                     // чужой префикс
		{"delok:5", cbDelete},                 // del: не должен матчить delok:
		{"code:99999999999999999999", cbCode}, // переполнение int64
	}
	for _, c := range cases {
		if id, ok := parseIDCallback(c.data, c.prefix); ok {
			t.Fatalf("parseIDCallback(%q, %q) = %d, want rejection", c.data, c.prefix, id)
		}
	}
}
