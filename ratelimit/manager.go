package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const redactedKey = "[redacted]"

//go:generate go tool msgp -o=manager_msgp.go -unexported
type item struct {
	currHits int
	prevHits int
	exp      uint64
}

//msgp:ignore manager
type manager struct {
	pool       sync.Pool
	storage    Storage
	redactKeys bool
}

func newManager(storage Storage, redactKeys bool) *manager {
	return &manager{
		pool: sync.Pool{
			New: func() any {
				return new(item)
			},
		},
		storage:    storage,
		redactKeys: redactKeys,
	}
}

// acquire returns an *item from the sync.Pool
func (m *manager) acquire() *item {
	return m.pool.Get().(*item) //nolint:forcetypeassert,errcheck // We store nothing else in the pool
}

// release and reset *item to sync.Pool
func (m *manager) release(e *item) {
	e.prevHits = 0
	e.currHits = 0
	e.exp = 0
	m.pool.Put(e)
}

// get data from storage or memory
func (m *manager) get(ctx context.Context, key string) (*item, error) {
	raw, err := m.storage.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("rate_limiter: failed to get key %q from storage: %w", m.logKey(key), err)
	}

	it := m.acquire()

	if len(raw) > 0 {
		if _, err := it.UnmarshalMsg(raw); err != nil {
			m.release(it)
			return nil, fmt.Errorf("rate_limiter: failed to unmarshal key %q: %w", m.logKey(key), err)
		}
	}

	return it, nil
}

// set data to storage or memory
func (m *manager) set(ctx context.Context, key string, it *item, exp time.Duration) error {
	defer m.release(it)

	raw, err := it.MarshalMsg(nil)
	if err != nil {
		return fmt.Errorf("rate_limiter: failed to marshal key %q: %w", m.logKey(key), err)
	}

	if err := m.storage.Set(ctx, key, raw, exp); err != nil {
		return fmt.Errorf("rate_limiter: failed to store key %q: %w", m.logKey(key), err)
	}
	return nil
}

func (m *manager) logKey(key string) string {
	if m.redactKeys {
		return redactedKey
	}
	return key
}
