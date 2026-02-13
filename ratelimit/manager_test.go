package ratelimit

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStorage struct {
	mu     sync.Mutex
	data   map[string][]byte
	getErr error
	setErr error
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		data: make(map[string][]byte),
	}
}

func (m *mockStorage) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.data[key], nil
}

func (m *mockStorage) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.setErr != nil {
		return m.setErr
	}
	m.data[key] = value
	return nil
}

func TestManager_newManager(t *testing.T) {
	t.Run("creates manager with storage and redactKeys true", func(t *testing.T) {
		storage := newMockStorage()
		m := newManager(storage, true)

		assert.NotNil(t, m)
		assert.Same(t, storage, m.storage)
		assert.True(t, m.redactKeys)
		assert.NotNil(t, m.acquire())
	})

	t.Run("creates manager with storage and redactKeys false", func(t *testing.T) {
		storage := newMockStorage()
		m := newManager(storage, false)

		assert.NotNil(t, m)
		assert.Same(t, storage, m.storage)
		assert.False(t, m.redactKeys)
		assert.NotNil(t, m.acquire())
	})

	t.Run("pool creates new items", func(t *testing.T) {
		storage := newMockStorage()
		m := newManager(storage, false)

		it := m.acquire()
		assert.NotNil(t, it)
		assert.IsType(t, &item{}, it)

		m.release(it)

		it2 := m.acquire()
		assert.NotNil(t, it2)
		assert.IsType(t, &item{}, it2)

		m.release(it2)
	})
}

func TestManager_logKey(t *testing.T) {
	t.Run("redacts key when redactKeys is true", func(t *testing.T) {
		storage := newMockStorage()
		m := newManager(storage, true)

		key := "sensitive-key-123"
		logKey := m.logKey(key)

		assert.Equal(t, redactedKey, logKey)
	})

	t.Run("returns original key when redactKeys is false", func(t *testing.T) {
		storage := newMockStorage()
		m := newManager(storage, false)

		key := "normal-key-456"
		logKey := m.logKey(key)

		assert.Equal(t, key, logKey)
	})
}

func TestManager_acquire_release(t *testing.T) {
	t.Run("acquire and release items correctly", func(t *testing.T) {
		storage := newMockStorage()
		m := newManager(storage, false)

		it := m.acquire()
		assert.NotNil(t, it)

		it.currHits = 10
		it.prevHits = 5
		it.exp = 1234567890

		m.release(it)

		it2 := m.acquire()
		assert.NotNil(t, it2)
		assert.Equal(t, 0, it2.currHits)
		assert.Equal(t, 0, it2.prevHits)
		assert.Equal(t, uint64(0), it2.exp)

		m.release(it2)
	})

	t.Run("pool reuses items", func(t *testing.T) {
		storage := newMockStorage()
		m := newManager(storage, false)

		it1 := m.acquire()
		m.release(it1)

		it2 := m.acquire()
		assert.NotNil(t, it2)
		assert.Equal(t, 0, it2.currHits)
		m.release(it2)
	})
}

func TestManager_get(t *testing.T) {
	t.Run("gets item from storage with data", func(t *testing.T) {
		storage := newMockStorage()
		m := newManager(storage, false)

		ctx := context.Background()
		key := "test-key"

		it := m.acquire()
		it.currHits = 5
		it.prevHits = 3
		it.exp = 1234567890

		raw, err := it.MarshalMsg(nil)
		require.NoError(t, err)
		require.NoError(t, storage.Set(ctx, key, raw, time.Minute))
		m.release(it)

		result, err := m.get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 5, result.currHits)
		assert.Equal(t, 3, result.prevHits)
		assert.Equal(t, uint64(1234567890), result.exp)

		m.release(result)
	})

	t.Run("gets item from storage with empty data", func(t *testing.T) {
		storage := newMockStorage()
		m := newManager(storage, false)

		ctx := context.Background()
		key := "test-key"

		result, err := m.get(ctx, key)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 0, result.currHits)
		assert.Equal(t, 0, result.prevHits)
		assert.Equal(t, uint64(0), result.exp)

		m.release(result)
	})

	t.Run("returns error when storage get fails", func(t *testing.T) {
		storage := newMockStorage()
		storage.getErr = errors.New("storage error")
		m := newManager(storage, true)

		ctx := context.Background()
		key := "test-key"

		result, err := m.get(ctx, key)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "rate_limiter: failed to get key")
		assert.Contains(t, err.Error(), "storage error")
		assert.Contains(t, err.Error(), redactedKey)
	})

	t.Run("returns error when unmarshal fails with redactKeys true", func(t *testing.T) {
		storage := newMockStorage()
		m := newManager(storage, true)

		ctx := context.Background()
		key := "test-key"

		require.NoError(t, storage.Set(ctx, key, []byte("invalid data"), time.Minute))

		result, err := m.get(ctx, key)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "rate_limiter: failed to unmarshal key")
		assert.Contains(t, err.Error(), redactedKey)
	})

	t.Run("returns error when unmarshal fails with redactKeys false", func(t *testing.T) {
		storage := newMockStorage()
		m := newManager(storage, false)

		ctx := context.Background()
		key := "test-key"

		require.NoError(t, storage.Set(ctx, key, []byte("invalid data"), time.Minute))

		result, err := m.get(ctx, key)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "rate_limiter: failed to unmarshal key")
		assert.Contains(t, err.Error(), key)
	})
}

func TestManager_set(t *testing.T) {
	t.Run("sets item to storage successfully", func(t *testing.T) {
		storage := newMockStorage()
		m := newManager(storage, false)

		ctx := context.Background()
		key := "test-key"

		it := m.acquire()
		it.currHits = 10
		it.prevHits = 5
		it.exp = 9876543210

		err := m.set(ctx, key, it, 5*time.Minute)
		require.NoError(t, err)

		raw, err := storage.Get(ctx, key)
		require.NoError(t, err)
		assert.NotEmpty(t, raw)
	})

	t.Run("releases item after successful set", func(t *testing.T) {
		storage := newMockStorage()
		m := newManager(storage, false)

		ctx := context.Background()
		key := "test-key"

		it := m.acquire()
		it.currHits = 10

		err := m.set(ctx, key, it, time.Minute)
		require.NoError(t, err)

		it2 := m.acquire()
		assert.NotNil(t, it2)
		assert.Equal(t, 0, it2.currHits)

		m.release(it2)
	})

	t.Run("returns error when storage set fails with redactKeys true", func(t *testing.T) {
		storage := newMockStorage()
		storage.setErr = errors.New("set error")
		m := newManager(storage, true)

		ctx := context.Background()
		key := "test-key"

		it := m.acquire()

		err := m.set(ctx, key, it, time.Minute)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rate_limiter: failed to store key")
		assert.Contains(t, err.Error(), redactedKey)
	})

	t.Run("returns error when storage set fails with redactKeys false", func(t *testing.T) {
		storage := newMockStorage()
		storage.setErr = errors.New("set error")
		m := newManager(storage, false)

		ctx := context.Background()
		key := "test-key"

		it := m.acquire()

		err := m.set(ctx, key, it, time.Minute)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rate_limiter: failed to store key")
		assert.Contains(t, err.Error(), key)
	})
}

func TestManager_integration(t *testing.T) {
	t.Run("full get-set cycle with redactKeys true", func(t *testing.T) {
		storage := newMockStorage()
		m := newManager(storage, true)

		ctx := context.Background()
		key := "sensitive-key"

		it1 := m.acquire()
		it1.currHits = 15
		it1.prevHits = 7
		it1.exp = 1111111111

		err := m.set(ctx, key, it1, time.Minute)
		require.NoError(t, err)

		it2, err := m.get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 15, it2.currHits)
		assert.Equal(t, 7, it2.prevHits)
		assert.Equal(t, uint64(1111111111), it2.exp)

		m.release(it2)
	})

	t.Run("full get-set cycle with redactKeys false", func(t *testing.T) {
		storage := newMockStorage()
		m := newManager(storage, false)

		ctx := context.Background()
		key := "normal-key"

		it1 := m.acquire()
		it1.currHits = 20
		it1.prevHits = 10
		it1.exp = 2222222222

		err := m.set(ctx, key, it1, 2*time.Minute)
		require.NoError(t, err)

		it2, err := m.get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 20, it2.currHits)
		assert.Equal(t, 10, it2.prevHits)
		assert.Equal(t, uint64(2222222222), it2.exp)

		m.release(it2)
	})

	t.Run("concurrent get and set operations", func(t *testing.T) {
		storage := newMockStorage()
		m := newManager(storage, false)

		ctx := context.Background()

		for i := 0; i < 100; i++ {
			it := m.acquire()
			it.currHits = i

			key := "concurrent-key"
			err := m.set(ctx, key, it, time.Minute)
			require.NoError(t, err)

			result, err := m.get(ctx, key)
			require.NoError(t, err)
			assert.Equal(t, i, result.currHits)

			m.release(result)
		}
	})
}
