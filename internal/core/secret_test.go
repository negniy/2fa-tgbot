package core

import (
	"fmt"
	"testing"
)

func TestSecretValueRedactedInString(t *testing.T) {
	s := NewSecret("TOPSECRET")
	if got := s.String(); got != "[REDACTED]" {
		t.Fatalf("String() = %q, want [REDACTED]", got)
	}
	if got := fmt.Sprintf("%v", s); got != "[REDACTED]" {
		t.Fatalf("%%v = %q, want [REDACTED]", got)
	}
	if got := fmt.Sprintf("%s", s); got != "[REDACTED]" {
		t.Fatalf("%%s = %q, want [REDACTED]", got)
	}
}

func TestSecretValueReveal(t *testing.T) {
	s := NewSecret("TOPSECRET")
	if s.Reveal() != "TOPSECRET" {
		t.Fatalf("Reveal() = %q", s.Reveal())
	}
}

func TestSecretValueEmpty(t *testing.T) {
	if !NewSecret("").IsEmpty() {
		t.Fatal("empty secret should report IsEmpty")
	}
	if NewSecret("x").IsEmpty() {
		t.Fatal("non-empty secret should not report IsEmpty")
	}
}
