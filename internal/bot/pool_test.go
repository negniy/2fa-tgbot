package bot

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func privateUpdate(userID int64) tgbotapi.Update {
	return tgbotapi.Update{
		Message: &tgbotapi.Message{
			From: &tgbotapi.User{ID: userID},
			Chat: &tgbotapi.Chat{ID: userID, Type: "private"},
		},
	}
}

// TestRoutingKeyStable: один и тот же пользователь всегда даёт один ключ,
// значит один шард — основа последовательной обработки.
func TestSameOwnerSameShard(t *testing.T) {
	p := newWorkerPool(8, 16, func(ctx context.Context, u tgbotapi.Update) {}, testLogger())
	key := routingKey(privateUpdate(12345))
	first := p.shardIndex(key)
	for i := 0; i < 100; i++ {
		if got := p.shardIndex(routingKey(privateUpdate(12345))); got != first {
			t.Fatalf("same owner routed to different shards: %d vs %d", first, got)
		}
	}
}

// TestSequentialPerOwner: апдейты одного владельца обрабатываются строго по
// порядку (не перемешиваются), даже при параллельных воркерах.
func TestSequentialPerOwner(t *testing.T) {
	var mu sync.Mutex
	var order []int

	handler := func(ctx context.Context, u tgbotapi.Update) {
		// Эмулируем работу переменной длительности.
		time.Sleep(time.Millisecond)
		mu.Lock()
		order = append(order, u.Message.MessageID)
		mu.Unlock()
	}

	p := newWorkerPool(8, 100, handler, testLogger())
	p.start(context.Background())

	// 20 апдейтов одного пользователя с возрастающим MessageID.
	for i := 1; i <= 20; i++ {
		u := privateUpdate(777)
		u.Message.MessageID = i
		p.submit(u)
	}
	p.stop()

	if len(order) != 20 {
		t.Fatalf("expected 20 processed, got %d", len(order))
	}
	for i, id := range order {
		if id != i+1 {
			t.Fatalf("out-of-order at %d: got MessageID %d", i, id)
		}
	}
}

// TestGracefulDrain: stop() дожидается обработки всех принятых апдейтов.
func TestGracefulDrain(t *testing.T) {
	var processed int64
	handler := func(ctx context.Context, u tgbotapi.Update) {
		time.Sleep(time.Millisecond)
		atomic.AddInt64(&processed, 1)
	}

	p := newWorkerPool(4, 100, handler, testLogger())
	p.start(context.Background())

	const n = 50
	for i := 0; i < n; i++ {
		p.submit(privateUpdate(int64(i))) // разные владельцы → разные шарды
	}
	p.stop()

	if got := atomic.LoadInt64(&processed); got != n {
		t.Fatalf("drain incomplete: processed %d of %d", got, n)
	}
}

// TestPanicDoesNotKillWorker: паника в обработчике одного апдейта не
// останавливает воркер — следующие апдейты того же шарда обрабатываются.
func TestPanicDoesNotKillWorker(t *testing.T) {
	var processed int64
	handler := func(ctx context.Context, u tgbotapi.Update) {
		if u.Message.MessageID == 1 {
			panic("boom")
		}
		atomic.AddInt64(&processed, 1)
	}

	p := newWorkerPool(1, 10, handler, testLogger())
	p.start(context.Background())
	for i := 1; i <= 3; i++ {
		u := privateUpdate(1)
		u.Message.MessageID = i
		p.submit(u)
	}
	p.stop()

	if got := atomic.LoadInt64(&processed); got != 2 {
		t.Fatalf("expected 2 processed after panic, got %d", got)
	}
}

// TestDrainWithCancelledContext: после отмены корневого контекста уже
// принятые апдейты дообрабатываются с НЕотменённым контекстом.
func TestDrainWithCancelledContext(t *testing.T) {
	var cancelledSeen int64
	handler := func(ctx context.Context, u tgbotapi.Update) {
		if ctx.Err() != nil {
			atomic.AddInt64(&cancelledSeen, 1)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	p := newWorkerPool(2, 10, handler, testLogger())
	p.start(ctx)
	cancel()
	for i := 0; i < 5; i++ {
		p.submit(privateUpdate(int64(i)))
	}
	p.stop()

	if n := atomic.LoadInt64(&cancelledSeen); n != 0 {
		t.Fatalf("handlers saw cancelled context %d times", n)
	}
}

// TestSubmitDropsWhenFull: переполнение очереди не блокирует, апдейт
// отбрасывается (защита от флуда).
func TestSubmitDropsWhenFull(t *testing.T) {
	block := make(chan struct{})
	handler := func(ctx context.Context, u tgbotapi.Update) {
		<-block // держим воркер занятым
	}
	// 1 шард, очередь глубиной 1.
	p := newWorkerPool(1, 1, handler, testLogger())
	p.start(context.Background())

	// Первый апдейт займёт воркер, второй ляжет в очередь, третий не влезет.
	_ = p.submit(privateUpdate(1))
	time.Sleep(10 * time.Millisecond) // дать воркеру подхватить первый
	_ = p.submit(privateUpdate(1))
	dropped := !p.submit(privateUpdate(1))

	close(block)
	p.stop()

	if !dropped {
		t.Fatal("expected third submit to be dropped when queue full")
	}
}
