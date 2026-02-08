package middleware

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiterManager_newRateLimiterManager(t *testing.T) {
	t.Run("creates manager with storage and redactKeys true", func(t *testing.T) {
		storage := newMockRateLimiterStorage()
		manager := newRateLimiterManager(storage, true)

		assert.NotNil(t, manager)
		assert.Same(t, storage, manager.storage)
		assert.True(t, manager.redactKeys)
		assert.NotNil(t, manager.acquire())
	})

	t.Run("creates manager with storage and redactKeys false", func(t *testing.T) {
		storage := newMockRateLimiterStorage()
		manager := newRateLimiterManager(storage, false)

		assert.NotNil(t, manager)
		assert.Same(t, storage, manager.storage)
		assert.False(t, manager.redactKeys)
		assert.NotNil(t, manager.acquire())
	})

	t.Run("pool creates new items", func(t *testing.T) {
		storage := newMockRateLimiterStorage()
		manager := newRateLimiterManager(storage, false)

		it := manager.acquire()
		assert.NotNil(t, it)
		assert.IsType(t, &item{}, it)

		manager.release(it)

		it2 := manager.acquire()
		assert.NotNil(t, it2)
		assert.IsType(t, &item{}, it2)

		manager.release(it2)
	})
}

func TestRateLimiterManager_logKey(t *testing.T) {
	t.Run("redacts key when redactKeys is true", func(t *testing.T) {
		storage := newMockRateLimiterStorage()
		manager := newRateLimiterManager(storage, true)

		key := "sensitive-key-123"
		logKey := manager.logKey(key)

		assert.Equal(t, redactedKey, logKey)
	})

	t.Run("returns original key when redactKeys is false", func(t *testing.T) {
		storage := newMockRateLimiterStorage()
		manager := newRateLimiterManager(storage, false)

		key := "normal-key-456"
		logKey := manager.logKey(key)

		assert.Equal(t, key, logKey)
	})
}

func TestRateLimiterManager_acquire_release(t *testing.T) {
	t.Run("acquire and release items correctly", func(t *testing.T) {
		storage := newMockRateLimiterStorage()
		manager := newRateLimiterManager(storage, false)

		it := manager.acquire()
		assert.NotNil(t, it)

		it.currHits = 10
		it.prevHits = 5
		it.exp = 1234567890

		manager.release(it)

		it2 := manager.acquire()
		assert.NotNil(t, it2)
		assert.Equal(t, 0, it2.currHits)
		assert.Equal(t, 0, it2.prevHits)
		assert.Equal(t, uint64(0), it2.exp)

		manager.release(it2)
	})

	t.Run("pool reuses items", func(t *testing.T) {
		storage := newMockRateLimiterStorage()
		manager := newRateLimiterManager(storage, false)

		it1 := manager.acquire()
		manager.release(it1)

		it2 := manager.acquire()
		assert.NotNil(t, it2)
		assert.Equal(t, 0, it2.currHits)
		manager.release(it2)
	})
}

func TestRateLimiterManager_get(t *testing.T) {
	t.Run("gets item from storage with data", func(t *testing.T) {
		storage := newMockRateLimiterStorage()
		manager := newRateLimiterManager(storage, false)

		ctx := context.Background()
		key := "test-key"

		it := manager.acquire()
		it.currHits = 5
		it.prevHits = 3
		it.exp = 1234567890

		raw, err := it.MarshalMsg(nil)
		require.NoError(t, err)
		require.NoError(t, storage.Set(ctx, key, raw, time.Minute))
		manager.release(it)

		result, err := manager.get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 5, result.currHits)
		assert.Equal(t, 3, result.prevHits)
		assert.Equal(t, uint64(1234567890), result.exp)

		manager.release(result)
	})

	t.Run("gets item from storage with empty data", func(t *testing.T) {
		storage := newMockRateLimiterStorage()
		manager := newRateLimiterManager(storage, false)

		ctx := context.Background()
		key := "test-key"

		result, err := manager.get(ctx, key)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 0, result.currHits)
		assert.Equal(t, 0, result.prevHits)
		assert.Equal(t, uint64(0), result.exp)

		manager.release(result)
	})

	t.Run("returns error when storage get fails", func(t *testing.T) {
		storage := newMockRateLimiterStorage()
		storage.getErr = errors.New("storage error")
		manager := newRateLimiterManager(storage, true)

		ctx := context.Background()
		key := "test-key"

		result, err := manager.get(ctx, key)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "rate_limiter: failed to get key")
		assert.Contains(t, err.Error(), "storage error")
		assert.Contains(t, err.Error(), redactedKey)
	})

	t.Run("returns error when unmarshal fails with redactKeys true", func(t *testing.T) {
		storage := newMockRateLimiterStorage()
		manager := newRateLimiterManager(storage, true)

		ctx := context.Background()
		key := "test-key"

		require.NoError(t, storage.Set(ctx, key, []byte("invalid data"), time.Minute))

		result, err := manager.get(ctx, key)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "rate_limiter: failed to unmarshal key")
		assert.Contains(t, err.Error(), redactedKey)
	})

	t.Run("returns error when unmarshal fails with redactKeys false", func(t *testing.T) {
		storage := newMockRateLimiterStorage()
		manager := newRateLimiterManager(storage, false)

		ctx := context.Background()
		key := "test-key"

		require.NoError(t, storage.Set(ctx, key, []byte("invalid data"), time.Minute))

		result, err := manager.get(ctx, key)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "rate_limiter: failed to unmarshal key")
		assert.Contains(t, err.Error(), key)
	})
}

func TestRateLimiterManager_set(t *testing.T) {
	t.Run("sets item to storage successfully", func(t *testing.T) {
		storage := newMockRateLimiterStorage()
		manager := newRateLimiterManager(storage, false)

		ctx := context.Background()
		key := "test-key"

		it := manager.acquire()
		it.currHits = 10
		it.prevHits = 5
		it.exp = 9876543210

		err := manager.set(ctx, key, it, 5*time.Minute)
		require.NoError(t, err)

		raw, err := storage.Get(ctx, key)
		require.NoError(t, err)
		assert.NotEmpty(t, raw)
	})

	t.Run("releases item after successful set", func(t *testing.T) {
		storage := newMockRateLimiterStorage()
		manager := newRateLimiterManager(storage, false)

		ctx := context.Background()
		key := "test-key"

		it := manager.acquire()
		it.currHits = 10

		err := manager.set(ctx, key, it, time.Minute)
		require.NoError(t, err)

		it2 := manager.acquire()
		assert.NotNil(t, it2)
		assert.Equal(t, 0, it2.currHits)

		manager.release(it2)
	})

	t.Run("returns error when storage set fails with redactKeys true", func(t *testing.T) {
		storage := newMockRateLimiterStorage()
		storage.setErr = errors.New("set error")
		manager := newRateLimiterManager(storage, true)

		ctx := context.Background()
		key := "test-key"

		it := manager.acquire()

		err := manager.set(ctx, key, it, time.Minute)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rate_limiter: failed to store key")
		assert.Contains(t, err.Error(), redactedKey)
	})

	t.Run("returns error when storage set fails with redactKeys false", func(t *testing.T) {
		storage := newMockRateLimiterStorage()
		storage.setErr = errors.New("set error")
		manager := newRateLimiterManager(storage, false)

		ctx := context.Background()
		key := "test-key"

		it := manager.acquire()

		err := manager.set(ctx, key, it, time.Minute)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rate_limiter: failed to store key")
		assert.Contains(t, err.Error(), key)
	})
}

func TestRateLimiterManager_integration(t *testing.T) {
	t.Run("full get-set cycle with redactKeys true", func(t *testing.T) {
		storage := newMockRateLimiterStorage()
		manager := newRateLimiterManager(storage, true)

		ctx := context.Background()
		key := "sensitive-key"

		it1 := manager.acquire()
		it1.currHits = 15
		it1.prevHits = 7
		it1.exp = 1111111111

		err := manager.set(ctx, key, it1, time.Minute)
		require.NoError(t, err)

		it2, err := manager.get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 15, it2.currHits)
		assert.Equal(t, 7, it2.prevHits)
		assert.Equal(t, uint64(1111111111), it2.exp)

		manager.release(it2)
	})

	t.Run("full get-set cycle with redactKeys false", func(t *testing.T) {
		storage := newMockRateLimiterStorage()
		manager := newRateLimiterManager(storage, false)

		ctx := context.Background()
		key := "normal-key"

		it1 := manager.acquire()
		it1.currHits = 20
		it1.prevHits = 10
		it1.exp = 2222222222

		err := manager.set(ctx, key, it1, 2*time.Minute)
		require.NoError(t, err)

		it2, err := manager.get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, 20, it2.currHits)
		assert.Equal(t, 10, it2.prevHits)
		assert.Equal(t, uint64(2222222222), it2.exp)

		manager.release(it2)
	})

	t.Run("concurrent get and set operations", func(t *testing.T) {
		storage := newMockRateLimiterStorage()
		manager := newRateLimiterManager(storage, false)

		ctx := context.Background()

		for i := 0; i < 100; i++ {
			it := manager.acquire()
			it.currHits = i

			key := "concurrent-key"
			err := manager.set(ctx, key, it, time.Minute)
			require.NoError(t, err)

			result, err := manager.get(ctx, key)
			require.NoError(t, err)
			assert.Equal(t, i, result.currHits)

			manager.release(result)
		}
	})
}
