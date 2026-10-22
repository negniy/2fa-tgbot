package state

import (
	"context"
	"testing"

	"2fa-tgbot/internal/core"
)

func TestAddStateLifecycle(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	owner := core.NewUserOwner(42)

	if _, ok, _ := s.GetAddState(ctx, owner); ok {
		t.Fatal("expected no state initially")
	}

	if err := s.SetAddState(ctx, owner, core.AddState{Step: 1, Service: "gh"}); err != nil {
		t.Fatal(err)
	}
	st, ok, _ := s.GetAddState(ctx, owner)
	if !ok || st.Step != 1 || st.Service != "gh" {
		t.Fatalf("unexpected state: %+v ok=%v", st, ok)
	}

	_ = s.DeleteAddState(ctx, owner)
	if _, ok, _ := s.GetAddState(ctx, owner); ok {
		t.Fatal("expected state deleted")
	}
}

func TestUserAndGroupOwnersDoNotCollide(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	user := core.NewUserOwner(100)
	group := core.NewGroupOwner(100) // тот же числовой id, другой Kind

	_ = s.SetAddState(ctx, user, core.AddState{Step: 1, Service: "user-svc"})
	_ = s.SetAddState(ctx, group, core.AddState{Step: 2, Service: "group-svc"})

	u, _, _ := s.GetAddState(ctx, user)
	g, _, _ := s.GetAddState(ctx, group)
	if u.Service != "user-svc" || g.Service != "group-svc" {
		t.Fatalf("owner collision: user=%q group=%q", u.Service, g.Service)
	}
}

func TestLastCodeMessage(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	owner := core.NewUserOwner(7)

	_ = s.SetLastCodeMessage(ctx, owner, core.LastCodeMsg{MessageID: 555, Service: "gh"})
	msg, ok, _ := s.GetLastCodeMessage(ctx, owner)
	if !ok || msg.MessageID != 555 || msg.Service != "gh" {
		t.Fatalf("unexpected: %+v ok=%v", msg, ok)
	}
	_ = s.DeleteLastCodeMessage(ctx, owner)
	if _, ok, _ := s.GetLastCodeMessage(ctx, owner); ok {
		t.Fatal("expected deleted")
	}
}
