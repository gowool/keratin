package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gowool/keratin"
	"github.com/stretchr/testify/assert"
)

const minute = time.Minute

func TestMiddleware(t *testing.T) {
	t.Run("panics when limiter is nil", func(t *testing.T) {
		assert.Panics(t, func() {
			Middleware(nil)
		})
	})

	t.Run("allows request when under limit", func(t *testing.T) {
		cfg := Config{
			Max:        5,
			Expiration: minute,
		}
		limiter := NewLimiter(cfg)

		mw := Middleware(limiter)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("success"))
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		mw(handler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "success", w.Body.String())
	})

	t.Run("returns 429 when rate limit exceeded", func(t *testing.T) {
		cfg := Config{
			Max:        1,
			Expiration: minute,
		}
		limiter := NewLimiter(cfg)

		mw := Middleware(limiter)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("success"))
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"

		w := httptest.NewRecorder()
		mw(handler).ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		w = httptest.NewRecorder()
		mw(handler).ServeHTTP(w, req)
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
	})

	t.Run("returns JSON response when accept header is JSON", func(t *testing.T) {
		cfg := Config{
			Max:        1,
			Expiration: minute,
		}
		limiter := NewLimiter(cfg)

		mw := Middleware(limiter)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		req.Header.Set(keratin.HeaderAccept, keratin.MIMEApplicationJSON)

		w := httptest.NewRecorder()
		mw(handler).ServeHTTP(w, req)

		w = httptest.NewRecorder()
		mw(handler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		assert.Equal(t, keratin.MIMEApplicationJSON, w.Header().Get(keratin.HeaderContentType))
		assert.Contains(t, w.Body.String(), "Rate limit exceeded")
	})

	t.Run("returns text response when accept header is not JSON", func(t *testing.T) {
		cfg := Config{
			Max:        1,
			Expiration: minute,
		}
		limiter := NewLimiter(cfg)

		mw := Middleware(limiter)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		req.Header.Set(keratin.HeaderAccept, keratin.MIMETextPlain)

		w := httptest.NewRecorder()
		mw(handler).ServeHTTP(w, req)

		w = httptest.NewRecorder()
		mw(handler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		assert.Contains(t, w.Body.String(), "Rate limit exceeded")
	})

	t.Run("skips middleware when skipper returns true", func(t *testing.T) {
		cfg := Config{
			Max:        1,
			Expiration: minute,
		}
		limiter := NewLimiter(cfg)

		skipper := func(r *http.Request) bool {
			return r.Header.Get("X-Skip-RateLimit") == "true"
		}

		mw := Middleware(limiter, skipper)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("skipped"))
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		req.Header.Set("X-Skip-RateLimit", "true")

		for range 5 {
			w := httptest.NewRecorder()
			mw(handler).ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}
	})

	t.Run("chains multiple skippers correctly", func(t *testing.T) {
		cfg := Config{
			Max:        1,
			Expiration: minute,
		}
		limiter := NewLimiter(cfg)

		skipper1 := func(r *http.Request) bool {
			return r.Header.Get("X-Skip-1") == "true"
		}

		skipper2 := func(r *http.Request) bool {
			return r.Header.Get("X-Skip-2") == "true"
		}

		mw := Middleware(limiter, skipper1, skipper2)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req1 := httptest.NewRequest(http.MethodGet, "/", nil)
		req1.RemoteAddr = "127.0.0.1:11111"
		req1.Header.Set("X-Skip-1", "true")

		for range 3 {
			w := httptest.NewRecorder()
			mw(handler).ServeHTTP(w, req1)
			assert.Equal(t, http.StatusOK, w.Code)
		}

		req2 := httptest.NewRequest(http.MethodGet, "/", nil)
		req2.RemoteAddr = "127.0.0.1:22222"
		req2.Header.Set("X-Skip-2", "true")

		for range 3 {
			w := httptest.NewRecorder()
			mw(handler).ServeHTTP(w, req2)
			assert.Equal(t, http.StatusOK, w.Code)
		}
	})

	t.Run("sets retry-after header when rate limited", func(t *testing.T) {
		cfg := Config{
			Max:            1,
			Expiration:     minute,
			DisableHeaders: false,
		}
		limiter := NewLimiter(cfg)

		mw := Middleware(limiter)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"

		w := httptest.NewRecorder()
		mw(handler).ServeHTTP(w, req)

		w = httptest.NewRecorder()
		mw(handler).ServeHTTP(w, req)

		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		assert.NotEmpty(t, w.Header().Get(keratin.HeaderRetryAfter))
	})

	t.Run("handles complex request flow", func(t *testing.T) {
		cfg := Config{
			Max:            3,
			Expiration:     minute,
			DisableHeaders: false,
		}
		limiter := NewLimiter(cfg)

		mw := Middleware(limiter)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		})

		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.RemoteAddr = "192.168.1.100:8080"

		for range 3 {
			w := httptest.NewRecorder()
			mw(handler).ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
			assert.NotEmpty(t, w.Header().Get(keratin.HeaderXRateLimitRemaining))
			assert.NotEmpty(t, w.Header().Get(keratin.HeaderXRateLimitLimit))
		}

		w := httptest.NewRecorder()
		mw(handler).ServeHTTP(w, req)
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		assert.NotEmpty(t, w.Header().Get(keratin.HeaderRetryAfter))
	})
}
