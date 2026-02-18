package ratelimit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gowool/keratin"
	"github.com/stretchr/testify/assert"
)

var fixedTimestamp uint32 = 1000000

func fixedTimestampFunc() uint32 {
	return fixedTimestamp
}

type contextTestStorage struct {
	*mockStorage
	getFunc func(context.Context, string) ([]byte, error)
}

func (s *contextTestStorage) Get(ctx context.Context, key string) ([]byte, error) {
	if s.getFunc != nil {
		return s.getFunc(ctx, key)
	}
	return s.mockStorage.Get(ctx, key)
}

func TestNewLimiter(t *testing.T) {
	t.Run("creates limiter with default config", func(t *testing.T) {
		cfg := Config{}
		limiter := NewLimiter(cfg)

		assert.NotNil(t, limiter)
		assert.NotNil(t, limiter.manager)
		assert.NotNil(t, limiter.mu)
		assert.Equal(t, uint(5), limiter.cfg.Max)
		assert.Equal(t, time.Minute, limiter.cfg.Expiration)
	})

	t.Run("creates limiter with custom config", func(t *testing.T) {
		cfg := Config{
			Max:        10,
			Expiration: 30 * time.Second,
		}
		limiter := NewLimiter(cfg)

		assert.NotNil(t, limiter)
		assert.Equal(t, uint(10), limiter.cfg.Max)
		assert.Equal(t, 30*time.Second, limiter.cfg.Expiration)
	})
}

func TestNewLimiterWithStorage(t *testing.T) {
	t.Run("creates limiter with custom storage", func(t *testing.T) {
		storage := newMockStorage()
		cfg := Config{}

		limiter := NewLimiterWithStorage(cfg, storage)

		assert.NotNil(t, limiter)
		assert.Same(t, storage, limiter.manager.storage)
	})

	t.Run("creates limiter with nil storage uses memory storage", func(t *testing.T) {
		cfg := Config{}
		cfg.TimestampFunc = fixedTimestampFunc

		limiter := NewLimiterWithStorage(cfg, nil)

		assert.NotNil(t, limiter)
		assert.NotNil(t, limiter.manager.storage)
		_, isMemStorage := limiter.manager.storage.(*MemoryStorage)
		assert.True(t, isMemStorage)
	})
}

func TestLimiter_Allow_FirstRequest(t *testing.T) {
	t.Run("allows first request and sets headers", func(t *testing.T) {
		cfg := Config{
			Max:            5,
			Expiration:     time.Minute,
			DisableHeaders: false,
			TimestampFunc:  fixedTimestampFunc,
		}
		limiter := NewLimiter(cfg)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		w := httptest.NewRecorder()

		err := limiter.Allow(w, req)

		assert.NoError(t, err)
		assert.Equal(t, "5", w.Header().Get(keratin.HeaderXRateLimitLimit))
		assert.Equal(t, "4", w.Header().Get(keratin.HeaderXRateLimitRemaining))
		assert.NotEmpty(t, w.Header().Get(keratin.HeaderXRateLimitReset))
	})

	t.Run("allows first request without headers", func(t *testing.T) {
		cfg := Config{
			Max:            5,
			Expiration:     time.Minute,
			DisableHeaders: true,
			TimestampFunc:  fixedTimestampFunc,
		}
		limiter := NewLimiter(cfg)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		w := httptest.NewRecorder()

		err := limiter.Allow(w, req)

		assert.NoError(t, err)
		assert.Empty(t, w.Header().Get(keratin.HeaderXRateLimitLimit))
		assert.Empty(t, w.Header().Get(keratin.HeaderXRateLimitRemaining))
		assert.Empty(t, w.Header().Get(keratin.HeaderXRateLimitReset))
	})
}

func TestLimiter_Allow_MultipleRequests(t *testing.T) {
	t.Run("allows requests within limit", func(t *testing.T) {
		cfg := Config{
			Max:            5,
			Expiration:     time.Minute,
			TimestampFunc:  fixedTimestampFunc,
			DisableHeaders: false,
		}
		limiter := NewLimiter(cfg)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"

		expectedRemaining := []string{"4", "3", "2", "1", "0"}

		for i := range 5 {
			w := httptest.NewRecorder()
			err := limiter.Allow(w, req)
			assert.NoError(t, err)
			assert.Equal(t, expectedRemaining[i], w.Header().Get(keratin.HeaderXRateLimitRemaining))
		}
	})
}

func TestLimiter_Allow_ExceedsLimit(t *testing.T) {
	t.Run("returns rate limit exceeded after max requests", func(t *testing.T) {
		cfg := Config{
			Max:            3,
			Expiration:     time.Minute,
			TimestampFunc:  fixedTimestampFunc,
			DisableHeaders: false,
		}
		limiter := NewLimiter(cfg)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"

		for range 3 {
			w := httptest.NewRecorder()
			err := limiter.Allow(w, req)
			assert.NoError(t, err)
		}

		w := httptest.NewRecorder()
		err := limiter.Allow(w, req)

		assert.Error(t, err)
		assert.Equal(t, ErrRateLimitExceeded, err)
		assert.NotEmpty(t, w.Header().Get(keratin.HeaderRetryAfter))
	})

	t.Run("does not set retry-after header when disabled", func(t *testing.T) {
		cfg := Config{
			Max:            2,
			Expiration:     time.Minute,
			TimestampFunc:  fixedTimestampFunc,
			DisableHeaders: true,
		}
		limiter := NewLimiter(cfg)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"

		for range 2 {
			w := httptest.NewRecorder()
			_ = limiter.Allow(w, req)
		}

		w := httptest.NewRecorder()
		err := limiter.Allow(w, req)

		assert.Error(t, err)
		assert.Equal(t, ErrRateLimitExceeded, err)
		assert.Empty(t, w.Header().Get(keratin.HeaderRetryAfter))
	})
}

func TestLimiter_Allow_DifferentKeys(t *testing.T) {
	t.Run("tracks requests separately for different keys", func(t *testing.T) {
		cfg := Config{
			Max:            2,
			Expiration:     time.Minute,
			TimestampFunc:  fixedTimestampFunc,
			DisableHeaders: false,
		}
		limiter := NewLimiter(cfg)

		req1 := httptest.NewRequest(http.MethodGet, "/", nil)
		req1.RemoteAddr = "127.0.0.1:11111"

		req2 := httptest.NewRequest(http.MethodGet, "/", nil)
		req2.RemoteAddr = "127.0.0.1:22222"

		for range 2 {
			w := httptest.NewRecorder()
			err := limiter.Allow(w, req1)
			assert.NoError(t, err)
		}

		for range 2 {
			w := httptest.NewRecorder()
			err := limiter.Allow(w, req2)
			assert.NoError(t, err)
		}

		w1 := httptest.NewRecorder()
		err1 := limiter.Allow(w1, req1)
		assert.Error(t, err1)
		assert.Equal(t, ErrRateLimitExceeded, err1)

		w2 := httptest.NewRecorder()
		err2 := limiter.Allow(w2, req2)
		assert.Error(t, err2)
		assert.Equal(t, ErrRateLimitExceeded, err2)
	})
}

func TestLimiter_Allow_Expiration(t *testing.T) {
	t.Run("resets counter after window expires", func(t *testing.T) {
		cfg := Config{
			Max:            2,
			Expiration:     10 * time.Second,
			TimestampFunc:  fixedTimestampFunc,
			DisableHeaders: false,
		}
		limiter := NewLimiter(cfg)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"

		for range 2 {
			w := httptest.NewRecorder()
			_ = limiter.Allow(w, req)
		}

		fixedTimestamp += 20
		w := httptest.NewRecorder()
		err := limiter.Allow(w, req)

		assert.NoError(t, err)
		assert.Equal(t, "1", w.Header().Get(keratin.HeaderXRateLimitRemaining))
	})
}

func TestLimiter_Allow_SlidingWindow(t *testing.T) {
	t.Run("window resets after full expiration", func(t *testing.T) {
		cfg := Config{
			Max:            10,
			Expiration:     10 * time.Second,
			TimestampFunc:  fixedTimestampFunc,
			DisableHeaders: false,
		}
		limiter := NewLimiter(cfg)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"

		for range 10 {
			w := httptest.NewRecorder()
			err := limiter.Allow(w, req)
			assert.NoError(t, err)
		}

		w := httptest.NewRecorder()
		err := limiter.Allow(w, req)
		assert.Error(t, err)
		assert.Equal(t, ErrRateLimitExceeded, err)

		fixedTimestamp += 15

		w = httptest.NewRecorder()
		err = limiter.Allow(w, req)
		assert.NoError(t, err)
	})
}

func TestLimiter_Allow_DynamicMax(t *testing.T) {
	t.Run("uses MaxFunc for dynamic max requests", func(t *testing.T) {
		cfg := Config{
			Expiration:    time.Minute,
			TimestampFunc: fixedTimestampFunc,
			MaxFunc: func(r *http.Request) uint {
				if r.Header.Get("X-Premium") == "true" {
					return 10
				}
				return 2
			},
			DisableHeaders: false,
		}
		limiter := NewLimiter(cfg)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"

		for range 2 {
			w := httptest.NewRecorder()
			_ = limiter.Allow(w, req)
		}

		w := httptest.NewRecorder()
		err := limiter.Allow(w, req)
		assert.Error(t, err)
		assert.Equal(t, ErrRateLimitExceeded, err)

		req.Header.Set("X-Premium", "true")
		for range 10 {
			w = httptest.NewRecorder()
			_ = limiter.Allow(w, req)
		}

		w = httptest.NewRecorder()
		err = limiter.Allow(w, req)
		assert.Error(t, err)
		assert.Equal(t, ErrRateLimitExceeded, err)
	})
}

func TestLimiter_Allow_DynamicExpiration(t *testing.T) {
	t.Run("uses ExpirationFunc for dynamic expiration", func(t *testing.T) {
		cfg := Config{
			Max:           2,
			TimestampFunc: fixedTimestampFunc,
			ExpirationFunc: func(r *http.Request) time.Duration {
				if r.Header.Get("X-Long-Window") == "true" {
					return 20 * time.Second
				}
				return 5 * time.Second
			},
			DisableHeaders: false,
		}
		limiter := NewLimiter(cfg)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"

		for range 2 {
			w := httptest.NewRecorder()
			_ = limiter.Allow(w, req)
		}

		fixedTimestamp += 10
		w := httptest.NewRecorder()
		err := limiter.Allow(w, req)
		assert.NoError(t, err)
		assert.Equal(t, "1", w.Header().Get(keratin.HeaderXRateLimitRemaining))
	})
}

func TestLimiter_Allow_CustomIdentifierExtractor(t *testing.T) {
	t.Run("uses custom identifier extractor", func(t *testing.T) {
		cfg := Config{
			Max:           2,
			Expiration:    time.Minute,
			TimestampFunc: fixedTimestampFunc,
			IdentifierExtractor: func(r *http.Request) (string, error) {
				apiKey := r.Header.Get("X-API-Key")
				if apiKey == "" {
					return "", errors.New("missing API key")
				}
				return apiKey, nil
			},
			DisableHeaders: false,
		}
		limiter := NewLimiter(cfg)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		req.Header.Set("X-API-Key", "key-123")

		for range 2 {
			w := httptest.NewRecorder()
			err := limiter.Allow(w, req)
			assert.NoError(t, err)
		}

		w := httptest.NewRecorder()
		err := limiter.Allow(w, req)
		assert.Error(t, err)
		assert.Equal(t, ErrRateLimitExceeded, err)

		req.Header.Set("X-API-Key", "key-456")
		w = httptest.NewRecorder()
		err = limiter.Allow(w, req)
		assert.NoError(t, err)
		assert.Equal(t, "1", w.Header().Get(keratin.HeaderXRateLimitRemaining))
	})
}

func TestLimiter_Allow_IdentifierExtractorError(t *testing.T) {
	t.Run("returns error when identifier extractor fails", func(t *testing.T) {
		cfg := Config{
			Max:           2,
			Expiration:    time.Minute,
			TimestampFunc: fixedTimestampFunc,
			IdentifierExtractor: func(r *http.Request) (string, error) {
				return "", errors.New("authentication failed")
			},
		}
		limiter := NewLimiter(cfg)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		err := limiter.Allow(w, req)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rate_limiter: failed to extract identifier")
		httpErr, ok := err.(*keratin.HTTPError)
		assert.True(t, ok)
		assert.Equal(t, http.StatusForbidden, httpErr.StatusCode())
	})
}

func TestLimiter_Allow_StorageError(t *testing.T) {
	t.Run("returns error when storage get fails", func(t *testing.T) {
		storage := newMockStorage()
		storage.getErr = errors.New("storage unavailable")

		cfg := Config{
			Max:           2,
			Expiration:    time.Minute,
			TimestampFunc: fixedTimestampFunc,
		}
		limiter := NewLimiterWithStorage(cfg, storage)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		w := httptest.NewRecorder()

		err := limiter.Allow(w, req)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rate_limiter: failed to get key")
	})

	t.Run("returns error when storage set fails", func(t *testing.T) {
		storage := newMockStorage()
		storage.setErr = errors.New("storage full")

		cfg := Config{
			Max:           2,
			Expiration:    time.Minute,
			TimestampFunc: fixedTimestampFunc,
		}
		limiter := NewLimiterWithStorage(cfg, storage)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		w := httptest.NewRecorder()

		err := limiter.Allow(w, req)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rate_limiter: failed to persist state")
	})
}

func TestLimiter_Allow_ConcurrentRequests(t *testing.T) {
	t.Run("handles concurrent requests safely", func(t *testing.T) {
		cfg := Config{
			Max:            100,
			Expiration:     time.Minute,
			TimestampFunc:  fixedTimestampFunc,
			DisableHeaders: false,
		}
		limiter := NewLimiter(cfg)

		var wg sync.WaitGroup
		errChan := make(chan error, 200)

		for range 100 {
			wg.Go(func() {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.RemoteAddr = "127.0.0.1:12345"
				w := httptest.NewRecorder()

				err := limiter.Allow(w, req)
				errChan <- err
			})
		}

		wg.Wait()
		close(errChan)

		rateLimitExceededCount := 0
		for err := range errChan {
			if errors.Is(err, ErrRateLimitExceeded) {
				rateLimitExceededCount++
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}

		assert.Equal(t, 0, rateLimitExceededCount, "should not exceed limit for concurrent requests")
	})
}

func TestLimiter_Allow_ContextCancellation(t *testing.T) {
	t.Run("respects context cancellation", func(t *testing.T) {
		callCount := 0
		storage := &contextTestStorage{
			mockStorage: newMockStorage(),
			getFunc: func(ctx context.Context, key string) ([]byte, error) {
				callCount++
				if callCount > 1 {
					<-ctx.Done()
					return nil, ctx.Err()
				}
				return newMockStorage().Get(ctx, key)
			},
		}

		cfg := Config{
			Max:           2,
			Expiration:    time.Minute,
			TimestampFunc: fixedTimestampFunc,
		}
		limiter := NewLimiterWithStorage(cfg, storage)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"

		cancelledCtx, cancel := context.WithCancel(req.Context())
		req = req.WithContext(cancelledCtx)

		w := httptest.NewRecorder()
		err := limiter.Allow(w, req)
		assert.NoError(t, err)

		cancel()
	})
}

func TestLimiter_maxFunc(t *testing.T) {
	t.Run("returns MaxFunc value when set", func(t *testing.T) {
		cfg := Config{
			Max: 5,
			MaxFunc: func(r *http.Request) uint {
				return 10
			},
		}
		limiter := NewLimiter(cfg)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		max := limiter.maxFunc(req)

		assert.Equal(t, 10, max)
	})

	t.Run("returns Max when MaxFunc not set or returns 0", func(t *testing.T) {
		cfg := Config{
			Max: 7,
		}
		limiter := NewLimiter(cfg)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		max := limiter.maxFunc(req)

		assert.Equal(t, 7, max)
	})

	t.Run("returns Max when MaxFunc returns 0", func(t *testing.T) {
		cfg := Config{
			Max: 8,
			MaxFunc: func(r *http.Request) uint {
				return 0
			},
		}
		limiter := NewLimiter(cfg)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		max := limiter.maxFunc(req)

		assert.Equal(t, 8, max)
	})
}

func TestLimiter_expirationFunc(t *testing.T) {
	t.Run("returns ExpirationFunc value when set", func(t *testing.T) {
		cfg := Config{
			Expiration: time.Minute,
			ExpirationFunc: func(r *http.Request) time.Duration {
				return 2 * time.Minute
			},
		}
		limiter := NewLimiter(cfg)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		exp := limiter.expirationFunc(req)

		assert.Equal(t, uint64(120), exp)
	})

	t.Run("returns Expiration when ExpirationFunc not set or returns 0", func(t *testing.T) {
		cfg := Config{
			Expiration: 30 * time.Second,
		}
		limiter := NewLimiter(cfg)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		exp := limiter.expirationFunc(req)

		assert.Equal(t, uint64(30), exp)
	})
}

func TestLimiter_DisableValueRedaction(t *testing.T) {
	t.Run("redacts keys in error messages when enabled", func(t *testing.T) {
		storage := newMockStorage()
		storage.getErr = errors.New("storage error")

		cfg := Config{
			Max:                   2,
			Expiration:            time.Minute,
			TimestampFunc:         fixedTimestampFunc,
			DisableValueRedaction: false,
		}
		limiter := NewLimiterWithStorage(cfg, storage)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		w := httptest.NewRecorder()

		err := limiter.Allow(w, req)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), redactedKey)
		assert.NotContains(t, err.Error(), "127.0.0.1:12345")
	})

	t.Run("does not redact keys in error messages when disabled", func(t *testing.T) {
		storage := newMockStorage()
		storage.getErr = errors.New("storage error")

		cfg := Config{
			Max:                   2,
			Expiration:            time.Minute,
			TimestampFunc:         fixedTimestampFunc,
			DisableValueRedaction: true,
		}
		limiter := NewLimiterWithStorage(cfg, storage)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		w := httptest.NewRecorder()

		err := limiter.Allow(w, req)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "127.0.0.1:12345")
		assert.NotContains(t, err.Error(), redactedKey)
	})
}

func TestLimiter_Allow_Scenarios(t *testing.T) {
	t.Run("exceeds limit with low max", func(t *testing.T) {
		cfg := Config{
			Max:            1,
			Expiration:     time.Minute,
			TimestampFunc:  fixedTimestampFunc,
			DisableHeaders: false,
		}
		limiter := NewLimiter(cfg)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"

		w := httptest.NewRecorder()
		err := limiter.Allow(w, req)
		assert.NoError(t, err)
		assert.Equal(t, "0", w.Header().Get(keratin.HeaderXRateLimitRemaining))

		w = httptest.NewRecorder()
		err = limiter.Allow(w, req)
		assert.Error(t, err)
		assert.Equal(t, ErrRateLimitExceeded, err)
		assert.NotEmpty(t, w.Header().Get(keratin.HeaderRetryAfter))
	})

	t.Run("handles high limit requests", func(t *testing.T) {
		cfg := Config{
			Max:            100,
			Expiration:     time.Minute,
			TimestampFunc:  fixedTimestampFunc,
			DisableHeaders: false,
		}
		limiter := NewLimiter(cfg)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"

		for i := range 50 {
			w := httptest.NewRecorder()
			err := limiter.Allow(w, req)
			assert.NoError(t, err)
			expectedRemaining := strconv.Itoa(100 - (i + 1))
			assert.Equal(t, expectedRemaining, w.Header().Get(keratin.HeaderXRateLimitRemaining))
		}
	})

	t.Run("reaches exactly at limit", func(t *testing.T) {
		cfg := Config{
			Max:            5,
			Expiration:     time.Minute,
			TimestampFunc:  fixedTimestampFunc,
			DisableHeaders: false,
		}
		limiter := NewLimiter(cfg)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"

		for range 5 {
			w := httptest.NewRecorder()
			err := limiter.Allow(w, req)
			assert.NoError(t, err)
		}

		w := httptest.NewRecorder()
		err := limiter.Allow(w, req)
		assert.Error(t, err)
		assert.Equal(t, ErrRateLimitExceeded, err)
	})
}
