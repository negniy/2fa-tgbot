package core

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeServiceName(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"github", "github", true},
		{"  GitHub  ", "GitHub", true},
		{"work \t  mail", "work mail", true},
		{"work_gmail*<b>", "work_gmail*<b>", true}, // разметка экранируется при выводе
		{"Госуслуги", "Госуслуги", true},
		{strings.Repeat("я", MaxServiceNameLen), strings.Repeat("я", MaxServiceNameLen), true},
		{"", "", false},
		{"   \n\t ", "", false},
		{strings.Repeat("я", MaxServiceNameLen+1), "", false},
		{"/start", "", false},
		{"bad\x00name", "", false},
		{"a‍b", "a‍b", true}, // ZWJ (эмодзи-последовательности) допустим
		{"bad\x7Fname", "", false},
		{"\xff\xfe", "", false}, // невалидный UTF-8
	}
	for _, c := range cases {
		got, err := NormalizeServiceName(c.in)
		if c.ok {
			if err != nil || got != c.want {
				t.Errorf("NormalizeServiceName(%q) = %q, %v; want %q", c.in, got, err, c.want)
			}
			continue
		}
		if !errors.Is(err, ErrInvalidName) {
			t.Errorf("NormalizeServiceName(%q) = %q, %v; want ErrInvalidName", c.in, got, err)
		}
	}
}

func TestNormalizeSecret(t *testing.T) {
	const canonical = "JBSWY3DPEHPK3PXP" // 16 символов = 80 бит
	valid := []struct{ in, want string }{
		{canonical, canonical},
		{"jbswy3dpehpk3pxp", canonical},
		{"jbsw y3dp ehpk 3pxp", canonical},
		{"JBSW-Y3DP-EHPK-3PXP", canonical},
		{"  JBSWY3DPEHPK3PXP\n", canonical},
		// 32 символа (160 бит) — длина, которую рекомендует RFC 4226.
		{"GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"},
		// Паддинг срезается.
		{"JBSWY3DPEHPK3PXPJBSWY3DPEE======", "JBSWY3DPEHPK3PXPJBSWY3DPEE"},
	}
	for _, c := range valid {
		got, err := NormalizeSecret(NewSecret(c.in))
		if err != nil || got.Reveal() != c.want {
			t.Errorf("NormalizeSecret(%q) = %q, %v; want %q", c.in, got.Reveal(), err, c.want)
		}
	}

	invalid := []string{
		"",
		"   ",
		"JBSWY3DP",                         // 40 бит — слишком коротко
		"JBSWY3DPEHPK3PX1",                 // 1 — не из алфавита base32
		"JBSWY3DPEHPK3PX0",                 // 0 — тоже
		"otpauth://totp/x?secret=JBSWY3DP", // URI пока не поддерживается
		"ЖБСВ Ы3ДП ЕХПК 3ПХП",
		strings.Repeat("A", 300), // > maxSecretBytes
	}
	for _, in := range invalid {
		if got, err := NormalizeSecret(NewSecret(in)); !errors.Is(err, ErrInvalidSecret) {
			t.Errorf("NormalizeSecret(%q) = %q, %v; want ErrInvalidSecret", in, got.Reveal(), err)
		}
	}
}

// TestNormalizeSecretErrorHasNoSecret: текст ошибки не раскрывает секрет.
func TestNormalizeSecretErrorHasNoSecret(t *testing.T) {
	const leaked = "SECRETVALUE1"
	_, err := NormalizeSecret(NewSecret(leaked))
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), leaked) {
		t.Fatalf("error leaks secret: %v", err)
	}
}
