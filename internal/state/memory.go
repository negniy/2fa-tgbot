package state

import (
	"context"
	"strconv"
	"sync"

	"2fa-tgbot/internal/core"
)

// MemoryStore — потокобезопасная in-memory реализация core.StateStore.
// Временная: на шаге 3 заменяется на Redis-реализацию того же интерфейса
// без изменения хендлеров. Ключи владельца сериализуются в строку, чтобы
// user и group с одинаковым числовым id не пересекались.
type MemoryStore struct {
	mu       sync.RWMutex
	addState map[string]core.AddState
	lastBot  map[int64]int
	lastCode map[string]core.LastCodeMsg
}

// NewMemoryStore создаёт пустое in-memory хранилище состояния.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		addState: make(map[string]core.AddState),
		lastBot:  make(map[int64]int),
		lastCode: make(map[string]core.LastCodeMsg),
	}
}

func ownerKey(o core.Owner) string {
	return string(o.Kind) + ":" + strconv.FormatInt(o.ID, 10)
}

// --- AddState ---

func (m *MemoryStore) GetAddState(ctx context.Context, owner core.Owner) (*core.AddState, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	st, ok := m.addState[ownerKey(owner)]
	if !ok {
		return nil, false, nil
	}
	cp := st
	return &cp, true, nil
}

func (m *MemoryStore) SetAddState(ctx context.Context, owner core.Owner, st core.AddState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addState[ownerKey(owner)] = st
	return nil
}

func (m *MemoryStore) DeleteAddState(ctx context.Context, owner core.Owner) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.addState, ownerKey(owner))
	return nil
}

// --- last bot message ---

func (m *MemoryStore) GetLastBotMessage(ctx context.Context, chatID int64) (int, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.lastBot[chatID]
	return id, ok, nil
}

func (m *MemoryStore) SetLastBotMessage(ctx context.Context, chatID int64, msgID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastBot[chatID] = msgID
	return nil
}

// --- last code message ---

func (m *MemoryStore) GetLastCodeMessage(ctx context.Context, owner core.Owner) (core.LastCodeMsg, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	msg, ok := m.lastCode[ownerKey(owner)]
	return msg, ok, nil
}

func (m *MemoryStore) SetLastCodeMessage(ctx context.Context, owner core.Owner, msg core.LastCodeMsg) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastCode[ownerKey(owner)] = msg
	return nil
}

func (m *MemoryStore) DeleteLastCodeMessage(ctx context.Context, owner core.Owner) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.lastCode, ownerKey(owner))
	return nil
}
