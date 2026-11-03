package state

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"2fa-tgbot/internal/core"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestRedisStore(t *testing.T) (*RedisStore, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewRedisStore(rdb, log), mr
}

func TestRedisAddStateLifecycle(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestRedisStore(t)
	owner := core.NewUserOwner(42)

	if _, ok, _ := s.GetAddState(ctx, owner); ok {
		t.Fatal("expected no state initially")
	}

	if err := s.SetAddState(ctx, owner, core.AddState{Step: 2, Service: "gh"}); err != nil {
		t.Fatal(err)
	}
	st, ok, _ := s.GetAddState(ctx, owner)
	if !ok || st.Step != 2 || st.Service != "gh" {
		t.Fatalf("unexpected state: %+v ok=%v", st, ok)
	}

	_ = s.DeleteAddState(ctx, owner)
	if _, ok, _ := s.GetAddState(ctx, owner); ok {
		t.Fatal("expected deleted")
	}
}

func TestRedisAddStateTTL(t *testing.T) {
	ctx := context.Background()
	s, mr := newTestRedisStore(t)
	owner := core.NewUserOwner(1)

	_ = s.SetAddState(ctx, owner, core.AddState{Step: 1})
	// Прокручиваем время за пределы TTL.
	mr.FastForward(addStateTTL + time.Second)

	if _, ok, _ := s.GetAddState(ctx, owner); ok {
		t.Fatal("state should expire after TTL")
	}
}

func TestRedisUserGroupNoCollision(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestRedisStore(t)
	user := core.NewUserOwner(100)
	group := core.NewGroupOwner(100)

	_ = s.SetAddState(ctx, user, core.AddState{Service: "user-svc"})
	_ = s.SetAddState(ctx, group, core.AddState{Service: "group-svc"})

	u, _, _ := s.GetAddState(ctx, user)
	g, _, _ := s.GetAddState(ctx, group)
	if u.Service != "user-svc" || g.Service != "group-svc" {
		t.Fatalf("collision: user=%q group=%q", u.Service, g.Service)
	}
}

func TestRedisLastCodeMessage(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestRedisStore(t)
	owner := core.NewUserOwner(7)

	_ = s.SetLastCodeMessage(ctx, owner, core.LastCodeMsg{MessageID: 555, SecretID: 9})
	msg, ok, _ := s.GetLastCodeMessage(ctx, owner)
	if !ok || msg.MessageID != 555 || msg.SecretID != 9 {
		t.Fatalf("unexpected: %+v ok=%v", msg, ok)
	}
	_ = s.DeleteLastCodeMessage(ctx, owner)
	if _, ok, _ := s.GetLastCodeMessage(ctx, owner); ok {
		t.Fatal("expected deleted")
	}
}

func TestRedisLastBotMessage(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestRedisStore(t)

	_ = s.SetLastBotMessage(ctx, 321, 999)
	id, ok, _ := s.GetLastBotMessage(ctx, 321)
	if !ok || id != 999 {
		t.Fatalf("unexpected: id=%d ok=%v", id, ok)
	}
}

// TestRedisAddStateFailClosed: когда Redis недоступен, операции flow
// добавления возвращают ошибку — вызывающий код обязан об этом узнать,
// чтобы не потерять сообщение с секретом.
func TestRedisAddStateFailClosed(t *testing.T) {
	ctx := context.Background()
	s, mr := newTestRedisStore(t)
	owner := core.NewUserOwner(5)

	_ = s.SetAddState(ctx, owner, core.AddState{Step: 1})
	mr.Close() // Redis «упал».

	if _, _, err := s.GetAddState(ctx, owner); err == nil {
		t.Fatal("get with redis down should return error")
	}
	if err := s.SetAddState(ctx, owner, core.AddState{Step: 2}); err == nil {
		t.Fatal("set with redis down should return error")
	}
	if err := s.DeleteAddState(ctx, owner); err == nil {
		t.Fatal("delete with redis down should return error")
	}
}

// TestRedisMessageRefsDegradation: ссылки на сообщения — косметика, при
// недоступном Redis они деградируют мягко: «пусто» и без ошибок.
func TestRedisMessageRefsDegradation(t *testing.T) {
	ctx := context.Background()
	s, mr := newTestRedisStore(t)
	owner := core.NewUserOwner(5)
	mr.Close()

	if _, ok, err := s.GetLastBotMessage(ctx, 1); err != nil || ok {
		t.Fatalf("lastmsg: ok=%v err=%v", ok, err)
	}
	if err := s.SetLastBotMessage(ctx, 1, 2); err != nil {
		t.Fatalf("set lastmsg: %v", err)
	}
	if _, ok, err := s.GetLastCodeMessage(ctx, owner); err != nil || ok {
		t.Fatalf("lastcode: ok=%v err=%v", ok, err)
	}
	if err := s.SetLastCodeMessage(ctx, owner, core.LastCodeMsg{}); err != nil {
		t.Fatalf("set lastcode: %v", err)
	}
}
