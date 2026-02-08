package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gowool/keratin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRateLimiterStorage struct {
	mu     sync.Mutex
	data   map[string][]byte
	getErr error
	setErr error
}

func newMockRateLimiterStorage() *mockRateLimiterStorage {
	return &mockRateLimiterStorage{
		data: make(map[string][]byte),
	}
}

func (m *mockRateLimiterStorage) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.data[key], nil
}

func (m *mockRateLimiterStorage) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.setErr != nil {
		return m.setErr
	}
	m.data[key] = value
	return nil
}

func TestRateLimiterConfig_SetDefaults(t *testing.T) {
	t.Run("sets all default values", func(t *testing.T) {
		cfg := RateLimiterConfig{}
		cfg.SetDefaults()

		assert.NotNil(t, cfg.Storage)
		assert.NotNil(t, cfg.TimestampFunc)
		assert.NotNil(t, cfg.IdentifierExtractor)
		assert.NotNil(t, cfg.MaxFunc)
		assert.NotNil(t, cfg.ExpirationFunc)
		assert.Equal(t, uint(5), cfg.Max)
		assert.Equal(t, 1*time.Minute, cfg.Expiration)
		assert.False(t, cfg.DisableHeaders)
		assert.False(t, cfg.DisableValueRedaction)
	})

	t.Run("preserves storage when set", func(t *testing.T) {
		storage := newMockRateLimiterStorage()
		cfg := RateLimiterConfig{Storage: storage}
		cfg.SetDefaults()

		assert.Same(t, storage, cfg.Storage)
	})

	t.Run("preserves timestamp func when set", func(t *testing.T) {
		timestampFunc := func() uint32 { return 123 }
		cfg := RateLimiterConfig{TimestampFunc: timestampFunc}
		cfg.SetDefaults()

		assert.NotNil(t, cfg.TimestampFunc)
	})

	t.Run("preserves identifier extractor when set", func(t *testing.T) {
		extractor := func(r *http.Request) (string, error) { return "test", nil }
		cfg := RateLimiterConfig{IdentifierExtractor: extractor}
		cfg.SetDefaults()

		assert.NotNil(t, cfg.IdentifierExtractor)
	})

	t.Run("preserves max when set", func(t *testing.T) {
		cfg := RateLimiterConfig{Max: 10}
		cfg.SetDefaults()

		assert.Equal(t, uint(10), cfg.Max)
	})

	t.Run("preserves expiration when set", func(t *testing.T) {
		cfg := RateLimiterConfig{Expiration: 30 * time.Second}
		cfg.SetDefaults()

		assert.Equal(t, 30*time.Second, cfg.Expiration)
	})

	t.Run("preserves disable headers when set", func(t *testing.T) {
		cfg := RateLimiterConfig{DisableHeaders: true}
		cfg.SetDefaults()

		assert.True(t, cfg.DisableHeaders)
	})

	t.Run("preserves disable value redaction when set", func(t *testing.T) {
		cfg := RateLimiterConfig{DisableValueRedaction: true}
		cfg.SetDefaults()

		assert.True(t, cfg.DisableValueRedaction)
	})
}

func TestRateLimiter_WithinLimit(t *testing.T) {
	t.Run("allows requests within limit", func(t *testing.T) {
		cfg := RateLimiterConfig{
			Max:        3,
			Expiration: 1 * time.Minute,
		}
		middleware := RateLimiter(cfg)

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("success"))
			return nil
		})

		wrapped := middleware(handler)

		for i := 0; i < 3; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			rec := httptest.NewRecorder()

			err := wrapped.ServeHTTP(rec, req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "3", rec.Header().Get(keratin.HeaderXRateLimitLimit))
			assert.NotEmpty(t, rec.Header().Get(keratin.HeaderXRateLimitRemaining))
		}
	})
}

func TestRateLimiter_ExceedsLimit(t *testing.T) {
	t.Run("blocks requests exceeding limit", func(t *testing.T) {
		cfg := RateLimiterConfig{
			Max:        2,
			Expiration: 1 * time.Minute,
		}
		middleware := RateLimiter(cfg)

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		wrapped := middleware(handler)

		var errCount atomic.Int32
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			rec := httptest.NewRecorder()

			err := wrapped.ServeHTTP(rec, req)
			if i < 2 {
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, rec.Code)
			} else {
				assert.Error(t, err)
				assert.IsType(t, &keratin.HTTPError{}, err)
				httpErr, ok := err.(*keratin.HTTPError)
				require.True(t, ok)
				assert.Equal(t, http.StatusTooManyRequests, httpErr.StatusCode())
				errCount.Add(1)
				assert.NotEmpty(t, rec.Header().Get(keratin.HeaderRetryAfter))
			}
		}

		assert.Equal(t, int32(3), errCount.Load())
	})
}

func TestRateLimiter_Skipper(t *testing.T) {
	t.Run("skips rate limiting when skipper returns true", func(t *testing.T) {
		cfg := RateLimiterConfig{
			Max:        1,
			Expiration: 1 * time.Minute,
		}

		skipper := func(r *http.Request) bool {
			return r.URL.Path == "/skip"
		}

		middleware := RateLimiter(cfg, skipper)

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		wrapped := middleware(handler)

		for i := 0; i < 10; i++ {
			req := httptest.NewRequest(http.MethodGet, "/skip", nil)
			rec := httptest.NewRecorder()

			err := wrapped.ServeHTTP(rec, req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Empty(t, rec.Header().Get(keratin.HeaderXRateLimitLimit))
		}
	})

	t.Run("applies rate limiting when skipper returns false", func(t *testing.T) {
		cfg := RateLimiterConfig{
			Max:        1,
			Expiration: 1 * time.Minute,
		}

		skipper := func(r *http.Request) bool {
			return r.URL.Path == "/skip"
		}

		middleware := RateLimiter(cfg, skipper)

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NotEmpty(t, rec.Header().Get(keratin.HeaderXRateLimitLimit))
	})
}

func TestRateLimiter_CustomIdentifierExtractor(t *testing.T) {
	t.Run("uses custom identifier extractor", func(t *testing.T) {
		cfg := RateLimiterConfig{
			Max:        2,
			Expiration: 1 * time.Minute,
			IdentifierExtractor: func(r *http.Request) (string, error) {
				return r.Header.Get("X-API-Key"), nil
			},
		}
		middleware := RateLimiter(cfg)

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		wrapped := middleware(handler)

		apiKey := "test-api-key-123"

		for i := 0; i < 5; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("X-API-Key", apiKey)
			rec := httptest.NewRecorder()

			err := wrapped.ServeHTTP(rec, req)
			if i < 2 {
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, rec.Code)
			} else {
				assert.Error(t, err)
			}
		}
	})
}

func TestRateLimiter_IdentifierExtractorError(t *testing.T) {
	t.Run("returns forbidden error when identifier extraction fails", func(t *testing.T) {
		cfg := RateLimiterConfig{
			IdentifierExtractor: func(r *http.Request) (string, error) {
				return "", errors.New("invalid identifier")
			},
		}
		middleware := RateLimiter(cfg)

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)
		require.Error(t, err)
		assert.IsType(t, &keratin.HTTPError{}, err)
		httpErr, ok := err.(*keratin.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusForbidden, httpErr.StatusCode())
	})
}

func TestRateLimiter_DynamicMaxFunc(t *testing.T) {
	t.Run("uses dynamic max func", func(t *testing.T) {
		cfg := RateLimiterConfig{
			Max:        5,
			Expiration: 1 * time.Minute,
			MaxFunc: func(r *http.Request) uint {
				if r.URL.Path == "/premium" {
					return 10
				}
				return 5
			},
		}
		middleware := RateLimiter(cfg)

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		wrapped := middleware(handler)

		for i := 0; i < 8; i++ {
			req := httptest.NewRequest(http.MethodGet, "/premium", nil)
			rec := httptest.NewRecorder()

			err := wrapped.ServeHTTP(rec, req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "10", rec.Header().Get(keratin.HeaderXRateLimitLimit))
		}
	})
}

func TestRateLimiter_DynamicExpirationFunc(t *testing.T) {
	t.Run("uses dynamic expiration func", func(t *testing.T) {
		cfg := RateLimiterConfig{
			Max:        3,
			Expiration: 1 * time.Minute,
			ExpirationFunc: func(r *http.Request) time.Duration {
				if r.URL.Path == "/fast" {
					return 10 * time.Second
				}
				return 1 * time.Minute
			},
		}
		middleware := RateLimiter(cfg)

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		wrapped := middleware(handler)

		for i := 0; i < 3; i++ {
			req := httptest.NewRequest(http.MethodGet, "/fast", nil)
			rec := httptest.NewRecorder()

			err := wrapped.ServeHTTP(rec, req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)
		}

		time.Sleep(11 * time.Second)

		req := httptest.NewRequest(http.MethodGet, "/fast", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestRateLimiter_CustomStorage(t *testing.T) {
	t.Run("uses custom storage", func(t *testing.T) {
		storage := newMockRateLimiterStorage()
		cfg := RateLimiterConfig{
			Max:        3,
			Expiration: 1 * time.Minute,
			Storage:    storage,
		}
		middleware := RateLimiter(cfg)

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		storage.mu.Lock()
		dataStored := len(storage.data) > 0
		storage.mu.Unlock()

		assert.True(t, dataStored, "storage should have data")
	})

	t.Run("returns error when storage get fails", func(t *testing.T) {
		storage := newMockRateLimiterStorage()
		storage.getErr = errors.New("storage get error")

		cfg := RateLimiterConfig{
			Max:        3,
			Expiration: 1 * time.Minute,
			Storage:    storage,
		}
		middleware := RateLimiter(cfg)

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rate_limiter")
	})

	t.Run("returns error when storage set fails", func(t *testing.T) {
		storage := newMockRateLimiterStorage()
		storage.setErr = errors.New("storage set error")

		cfg := RateLimiterConfig{
			Max:        3,
			Expiration: 1 * time.Minute,
			Storage:    storage,
		}
		middleware := RateLimiter(cfg)

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rate_limiter")
	})
}

func TestRateLimiter_DisableHeaders(t *testing.T) {
	t.Run("disables rate limit headers", func(t *testing.T) {
		cfg := RateLimiterConfig{
			Max:            3,
			Expiration:     1 * time.Minute,
			DisableHeaders: true,
		}
		middleware := RateLimiter(cfg)

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)
		require.NoError(t, err)

		assert.Empty(t, rec.Header().Get(keratin.HeaderXRateLimitLimit))
		assert.Empty(t, rec.Header().Get(keratin.HeaderXRateLimitRemaining))
		assert.Empty(t, rec.Header().Get(keratin.HeaderXRateLimitReset))
	})

	t.Run("does not include retry-after when disabled and limit exceeded", func(t *testing.T) {
		cfg := RateLimiterConfig{
			Max:            1,
			Expiration:     1 * time.Minute,
			DisableHeaders: true,
		}
		middleware := RateLimiter(cfg)

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		wrapped := middleware(handler)

		for i := 0; i < 3; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()

			err := wrapped.ServeHTTP(rec, req)
			if i > 0 {
				assert.Error(t, err)
			}
			assert.Empty(t, rec.Header().Get(keratin.HeaderRetryAfter))
		}
	})
}

func TestRateLimiter_SlidingWindow(t *testing.T) {
	t.Run("implements sliding window behavior", func(t *testing.T) {
		cfg := RateLimiterConfig{
			Max:        10,
			Expiration: 1 * time.Minute,
		}
		middleware := RateLimiter(cfg)

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		wrapped := middleware(handler)

		for i := 0; i < 10; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			rec := httptest.NewRecorder()

			err := wrapped.ServeHTTP(rec, req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)
		}

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)
		assert.Error(t, err)
		assert.IsType(t, &keratin.HTTPError{}, err)
		httpErr, ok := err.(*keratin.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusTooManyRequests, httpErr.StatusCode())
	})
}

func TestRateLimiter_MultipleIdentifiers(t *testing.T) {
	t.Run("limits requests per identifier independently", func(t *testing.T) {
		cfg := RateLimiterConfig{
			Max:        2,
			Expiration: 1 * time.Minute,
		}
		middleware := RateLimiter(cfg)

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		wrapped := middleware(handler)

		for _, addr := range []string{"192.168.1.1:12345", "192.168.1.2:54321", "192.168.1.3:99999"} {
			for i := 0; i < 2; i++ {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				req.RemoteAddr = addr
				rec := httptest.NewRecorder()

				err := wrapped.ServeHTTP(rec, req)
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, rec.Code)
			}
		}
	})
}

func TestRateLimiter_ConcurrentRequests(t *testing.T) {
	t.Run("handles concurrent requests safely", func(t *testing.T) {
		cfg := RateLimiterConfig{
			Max:        100,
			Expiration: 1 * time.Minute,
		}
		middleware := RateLimiter(cfg)

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		wrapped := middleware(handler)

		var wg sync.WaitGroup
		var successCount atomic.Int32
		var errorCount atomic.Int32

		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				rec := httptest.NewRecorder()

				err := wrapped.ServeHTTP(rec, req)
				if err == nil {
					successCount.Add(1)
				} else {
					errorCount.Add(1)
				}
			}()
		}

		wg.Wait()

		assert.Equal(t, int32(50), successCount.Load())
		assert.Equal(t, int32(0), errorCount.Load())
	})
}

func TestRateLimiter_MaxFuncZero(t *testing.T) {
	t.Run("uses default max when MaxFunc returns zero", func(t *testing.T) {
		cfg := RateLimiterConfig{
			Max:        3,
			Expiration: 1 * time.Minute,
			MaxFunc: func(r *http.Request) uint {
				return 0
			},
		}
		middleware := RateLimiter(cfg)

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		wrapped := middleware(handler)

		for i := 0; i < 3; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()

			err := wrapped.ServeHTTP(rec, req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)
		}

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)
		assert.Error(t, err)
	})
}

func TestRateLimiter_ExpirationFuncZero(t *testing.T) {
	t.Run("uses default expiration when ExpirationFunc returns zero", func(t *testing.T) {
		cfg := RateLimiterConfig{
			Max:        3,
			Expiration: 1 * time.Minute,
			ExpirationFunc: func(r *http.Request) time.Duration {
				return 0
			},
		}
		middleware := RateLimiter(cfg)

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestRateLimiter_MiddlewareChaining(t *testing.T) {
	t.Run("works in middleware chain", func(t *testing.T) {
		cfg := RateLimiterConfig{
			Max:        3,
			Expiration: 1 * time.Minute,
		}
		rateLimiterMiddleware := RateLimiter(cfg)

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		customMiddleware := func(next keratin.Handler) keratin.Handler {
			return keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
				w.Header().Set("X-Custom", "value")
				return next.ServeHTTP(w, r)
			})
		}

		wrapped := customMiddleware(rateLimiterMiddleware(handler))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "value", rec.Header().Get("X-Custom"))
		assert.NotEmpty(t, rec.Header().Get(keratin.HeaderXRateLimitLimit))
	})
}

func TestRateLimiter_ErrRateLimitExceeded(t *testing.T) {
	t.Run("ErrRateLimitExceeded is correct", func(t *testing.T) {
		err := ErrRateLimitExceeded

		assert.Equal(t, http.StatusTooManyRequests, err.StatusCode())
		assert.Equal(t, "Rate limit exceeded.", err.Message)
	})
}

func TestRateLimiter_CustomTimestampFunc(t *testing.T) {
	t.Run("uses custom timestamp function", func(t *testing.T) {
		var timestamp uint32
		cfg := RateLimiterConfig{
			Max:        3,
			Expiration: 1 * time.Minute,
			TimestampFunc: func() uint32 {
				timestamp++
				return timestamp
			},
		}
		middleware := RateLimiter(cfg)

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		wrapped := middleware(handler)

		for i := 0; i < 3; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()

			err := wrapped.ServeHTTP(rec, req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)
		}

		assert.GreaterOrEqual(t, timestamp, uint32(3))
	})
}
