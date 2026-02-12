package session

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gowool/keratin"
	"github.com/gowool/keratin/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestHTTPMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	t.Run("skips when registry is empty", func(t *testing.T) {
		registry := NewRegistry()

		mw := HTTPMiddleware(registry, nil)
		wrapped := mw(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "OK", rec.Body.String())
	})

	t.Run("skips when skipper returns true", func(t *testing.T) {
		session := createTestSession("test")
		registry := NewRegistry(session)

		skipper := func(r *http.Request) bool {
			return r.URL.Path == "/skip"
		}

		mw := HTTPMiddleware(registry, nil, skipper)
		wrapped := mw(handler)

		req := httptest.NewRequest(http.MethodGet, "/skip", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "OK", rec.Body.String())
	})

	t.Run("skips when chain skipper returns true", func(t *testing.T) {
		session := createTestSession("test")
		registry := NewRegistry(session)

		skipper1 := func(r *http.Request) bool { return false }
		skipper2 := func(r *http.Request) bool { return r.URL.Path == "/skip" }

		mw := HTTPMiddleware(registry, nil, skipper1, skipper2)
		wrapped := mw(handler)

		req := httptest.NewRequest(http.MethodGet, "/skip", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "OK", rec.Body.String())
	})

	t.Run("uses nil logger when provided", func(t *testing.T) {
		session := createTestSession("test")
		registry := NewRegistry(session)

		mw := HTTPMiddleware(registry, nil)
		wrapped := mw(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "OK", rec.Body.String())
	})

	t.Run("logs error when ReadSessions fails", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, nil))

		mockStore := &MockStore{}
		mockStore.On("Find", mock.Anything, mock.Anything).Return([]byte(nil), false, errors.New("read error"))

		session := createTestSessionWithStore("test", mockStore)
		registry := NewRegistry(session)

		mw := HTTPMiddleware(registry, logger)
		wrapped := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "test", Value: "some-token"})
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Contains(t, logBuffer.String(), "failed to read sessions")
		mockStore.AssertExpectations(t)
	})

	t.Run("creates sessionWriter and resets it", func(t *testing.T) {
		session := createTestSession("test")
		registry := NewRegistry(session)

		var called bool
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, ok := w.(*sessionWriter)
			called = ok
			w.WriteHeader(http.StatusOK)
		})

		mw := HTTPMiddleware(registry, nil)
		wrapped := mw(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		assert.True(t, called)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("pools sessionWriter for reuse", func(t *testing.T) {
		session := createTestSession("test")
		registry := NewRegistry(session)

		var poolCalls int32

		mw := HTTPMiddleware(registry, nil)
		wrapped := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&poolCalls, 1)
			w.WriteHeader(http.StatusOK)
		}))

		for i := 0; i < 5; i++ {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			wrapped.ServeHTTP(rec, req)
		}

		assert.Equal(t, int32(5), atomic.LoadInt32(&poolCalls))
	})

	t.Run("logs error when WriteSessions fails", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, nil))

		mockStore := &MockStore{}
		mockCodec := &MockCodec{}
		mockCodec.On("Encode", mock.Anything, mock.Anything).Return([]byte("encoded"), nil)
		mockStore.On("Commit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("write error"))

		session := NewWithCodec(Config{Cookie: Cookie{Name: "test"}}, mockStore, mockCodec)
		registry := NewRegistry(session)

		handlerCalled := false

		mw := HTTPMiddleware(registry, logger)
		wrapped := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			ctx := r.Context()
			s := registry.Get("test")
			s.Put(ctx, "key", "value")
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		assert.True(t, handlerCalled)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, logBuffer.String(), "failed to write sessions")
		mockStore.AssertExpectations(t)
		mockCodec.AssertExpectations(t)
	})
}

func TestMiddleware(t *testing.T) {
	handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
		return nil
	})

	t.Run("skips when registry is empty", func(t *testing.T) {
		registry := NewRegistry()

		mw := Middleware(registry, nil)
		wrapped := mw(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "OK", rec.Body.String())
	})

	t.Run("skips when skipper returns true", func(t *testing.T) {
		session := createTestSession("test")
		registry := NewRegistry(session)

		skipper := func(r *http.Request) bool {
			return r.URL.Path == "/skip"
		}

		mw := Middleware(registry, nil, skipper)
		wrapped := mw(handler)

		req := httptest.NewRequest(http.MethodGet, "/skip", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "OK", rec.Body.String())
	})

	t.Run("returns error when ReadSessions fails", func(t *testing.T) {
		mockStore := &MockStore{}
		mockStore.On("Find", mock.Anything, mock.Anything).Return([]byte(nil), false, errors.New("read error"))

		session := createTestSessionWithStore("test", mockStore)
		registry := NewRegistry(session)

		mw := Middleware(registry, nil)
		wrapped := mw(keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			return nil
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "test", Value: "some-token"})
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read sessions")
		mockStore.AssertExpectations(t)
	})

	t.Run("creates sessionWriter and calls handler", func(t *testing.T) {
		session := createTestSession("test")
		registry := NewRegistry(session)

		var called bool

		mw := Middleware(registry, nil)
		wrapped := mw(keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			_, ok := w.(*sessionWriter)
			called = ok
			return nil
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("uses pool for sessionWriter reuse", func(t *testing.T) {
		session := createTestSession("test")
		registry := NewRegistry(session)

		var poolCalls int32

		mw := Middleware(registry, nil)
		wrapped := mw(keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			atomic.AddInt32(&poolCalls, 1)
			return nil
		}))

		for i := 0; i < 5; i++ {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			err := wrapped.ServeHTTP(rec, req)
			require.NoError(t, err)
		}

		assert.Equal(t, int32(5), atomic.LoadInt32(&poolCalls))
	})

	t.Run("logs error when WriteSessions fails", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, nil))

		mockStore := &MockStore{}
		mockCodec := &MockCodec{}
		mockCodec.On("Encode", mock.Anything, mock.Anything).Return([]byte("encoded"), nil)
		mockStore.On("Commit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("write error"))

		session := NewWithCodec(Config{Cookie: Cookie{Name: "test"}}, mockStore, mockCodec)
		registry := NewRegistry(session)

		handlerCalled := false

		mw := Middleware(registry, logger)
		wrapped := mw(keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			handlerCalled = true
			ctx := r.Context()
			s := registry.Get("test")
			s.Put(ctx, "key", "value")
			w.WriteHeader(http.StatusOK)
			return nil
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.True(t, handlerCalled)
		assert.Contains(t, logBuffer.String(), "failed to write sessions")
		mockStore.AssertExpectations(t)
		mockCodec.AssertExpectations(t)
	})
}

func TestSessionWriter(t *testing.T) {
	t.Run("reset sets all fields", func(t *testing.T) {
		sw := &sessionWriter{}
		registry := NewRegistry(createTestSession("test"))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		logger := slog.New(slog.DiscardHandler)

		sw.reset(rec, req, registry, logger)

		assert.Equal(t, rec, sw.ResponseWriter)
		assert.Equal(t, req, sw.request)
		assert.Equal(t, registry, sw.registry)
		assert.Equal(t, logger, sw.logger)
	})

	t.Run("reset with nil values", func(t *testing.T) {
		sw := &sessionWriter{
			ResponseWriter: httptest.NewRecorder(),
			request:        httptest.NewRequest(http.MethodGet, "/", nil),
			registry:       NewRegistry(createTestSession("test")),
			logger:         slog.New(slog.DiscardHandler),
		}

		sw.reset(nil, nil, nil, nil)

		assert.Nil(t, sw.ResponseWriter)
		assert.Nil(t, sw.request)
		assert.Nil(t, sw.registry)
		assert.Nil(t, sw.logger)
	})

	t.Run("WriteHeader calls WriteSessions", func(t *testing.T) {
		mockStore := &MockStore{}
		mockCodec := &MockCodec{}
		mockCodec.On("Encode", mock.Anything, mock.Anything).Return([]byte("encoded"), nil)
		mockStore.On("Commit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

		session := NewWithCodec(Config{Cookie: Cookie{Name: "test"}}, mockStore, mockCodec)
		registry := NewRegistry(session)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		logger := slog.New(slog.DiscardHandler)

		ctx, err := session.Load(req.Context(), "")
		require.NoError(t, err)
		req = req.WithContext(ctx)

		sw := &sessionWriter{}
		sw.reset(rec, req, registry, logger)

		s := registry.Get("test")
		s.Put(req.Context(), "key", "value")

		sw.WriteHeader(http.StatusOK)

		assert.Equal(t, http.StatusOK, rec.Code)
		mockStore.AssertExpectations(t)
		mockCodec.AssertExpectations(t)
	})

	t.Run("WriteHeader logs error on WriteSessions failure", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, nil))

		mockStore := &MockStore{}
		mockCodec := &MockCodec{}
		mockCodec.On("Encode", mock.Anything, mock.Anything).Return([]byte("encoded"), nil)
		mockStore.On("Commit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("write error"))

		session := NewWithCodec(Config{Cookie: Cookie{Name: "test"}}, mockStore, mockCodec)
		registry := NewRegistry(session)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		ctx, err := session.Load(req.Context(), "")
		require.NoError(t, err)
		req = req.WithContext(ctx)

		sw := &sessionWriter{}
		sw.reset(rec, req, registry, logger)

		s := registry.Get("test")
		s.Put(req.Context(), "key", "value")

		sw.WriteHeader(http.StatusOK)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, logBuffer.String(), "failed to write sessions")
		mockStore.AssertExpectations(t)
		mockCodec.AssertExpectations(t)
	})

	t.Run("Unwrap returns underlying ResponseWriter", func(t *testing.T) {
		rec := httptest.NewRecorder()
		sw := &sessionWriter{ResponseWriter: rec}

		unwrapped := sw.Unwrap()

		assert.Same(t, rec, unwrapped)
	})

	t.Run("Unwrap returns nil when ResponseWriter is nil", func(t *testing.T) {
		sw := &sessionWriter{ResponseWriter: nil}

		unwrapped := sw.Unwrap()

		assert.Nil(t, unwrapped)
	})

	t.Run("WriteHeader with destroyed session", func(t *testing.T) {
		mockStore := &MockStore{}
		mockCodec := &MockCodec{}
		mockStore.On("Delete", mock.Anything, mock.Anything).Return(nil)

		session := NewWithCodec(Config{Cookie: Cookie{Name: "test"}}, mockStore, mockCodec)
		registry := NewRegistry(session)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		logger := slog.New(slog.DiscardHandler)

		ctx, err := session.Load(req.Context(), "")
		require.NoError(t, err)
		req = req.WithContext(ctx)

		sw := &sessionWriter{}
		sw.reset(rec, req, registry, logger)

		s := registry.Get("test")
		s.Put(req.Context(), "key", "value")
		_ = s.Destroy(req.Context())

		sw.WriteHeader(http.StatusOK)

		assert.Equal(t, http.StatusOK, rec.Code)

		cookies := rec.Result().Cookies()
		assert.Len(t, cookies, 1)
		assert.Equal(t, "test", cookies[0].Name)
		assert.Equal(t, -1, cookies[0].MaxAge)
		mockStore.AssertExpectations(t)
		mockCodec.AssertExpectations(t)
	})

	t.Run("WriteHeader with unmodified session", func(t *testing.T) {
		mockStore := &MockStore{}
		mockCodec := &MockCodec{}

		session := NewWithCodec(Config{Cookie: Cookie{Name: "test"}}, mockStore, mockCodec)
		registry := NewRegistry(session)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		logger := slog.New(slog.DiscardHandler)

		ctx, err := session.Load(req.Context(), "")
		require.NoError(t, err)
		req = req.WithContext(ctx)

		sw := &sessionWriter{}
		sw.reset(rec, req, registry, logger)

		sw.WriteHeader(http.StatusOK)

		assert.Equal(t, http.StatusOK, rec.Code)

		cookies := rec.Result().Cookies()
		assert.Len(t, cookies, 0)
		mockStore.AssertExpectations(t)
	})
}

func TestMiddlewareIntegration(t *testing.T) {
	t.Run("full session lifecycle with HTTPMiddleware", func(t *testing.T) {
		mockStore := &MockStore{}
		mockCodec := &MockCodec{}

		mockCodec.On("Encode", mock.Anything, mock.Anything).Return([]byte("encoded"), nil)
		mockStore.On("Commit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

		session := NewWithCodec(Config{Cookie: Cookie{Name: "user"}}, mockStore, mockCodec)
		registry := NewRegistry(session)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			s := registry.Get("user")
			s.Put(ctx, "userID", "123")
			w.WriteHeader(http.StatusOK)
		})

		mw := HTTPMiddleware(registry, nil)
		wrapped := mw(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		cookies := rec.Result().Cookies()
		assert.Len(t, cookies, 1)
		assert.Equal(t, "user", cookies[0].Name)
		mockStore.AssertExpectations(t)
		mockCodec.AssertExpectations(t)
	})

	t.Run("full session lifecycle with Middleware", func(t *testing.T) {
		mockStore := &MockStore{}
		mockCodec := &MockCodec{}

		mockCodec.On("Encode", mock.Anything, mock.Anything).Return([]byte("encoded"), nil)
		mockStore.On("Commit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

		session := NewWithCodec(Config{Cookie: Cookie{Name: "user"}}, mockStore, mockCodec)
		registry := NewRegistry(session)

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			ctx := r.Context()
			s := registry.Get("user")
			s.Put(ctx, "userID", "123")
			w.WriteHeader(http.StatusOK)
			return nil
		})

		mw := Middleware(registry, nil)
		wrapped := mw(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		cookies := rec.Result().Cookies()
		assert.Len(t, cookies, 1)
		assert.Equal(t, "user", cookies[0].Name)
		mockStore.AssertExpectations(t)
		mockCodec.AssertExpectations(t)
	})

	t.Run("multiple sessions in registry", func(t *testing.T) {
		mockStore := &MockStore{}
		mockCodec := &MockCodec{}

		mockCodec.On("Encode", mock.Anything, mock.Anything).Return([]byte("encoded"), nil).Twice()
		mockStore.On("Commit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Twice()

		session1 := NewWithCodec(Config{Cookie: Cookie{Name: "user"}}, mockStore, mockCodec)
		session2 := NewWithCodec(Config{Cookie: Cookie{Name: "admin"}}, mockStore, mockCodec)
		registry := NewRegistry(session1, session2)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			userSession := registry.Get("user")
			adminSession := registry.Get("admin")
			userSession.Put(ctx, "userID", "123")
			adminSession.Put(ctx, "adminID", "456")
			w.WriteHeader(http.StatusOK)
		})

		mw := HTTPMiddleware(registry, nil)
		wrapped := mw(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		cookies := rec.Result().Cookies()
		assert.Len(t, cookies, 2)

		cookieNames := make(map[string]bool)
		for _, cookie := range cookies {
			cookieNames[cookie.Name] = true
		}
		assert.True(t, cookieNames["user"])
		assert.True(t, cookieNames["admin"])
		mockStore.AssertExpectations(t)
		mockCodec.AssertExpectations(t)
	})
}

func TestMiddlewareSkipper(t *testing.T) {
	t.Run("uses PrefixPathSkipper", func(t *testing.T) {
		session := createTestSession("test")
		registry := NewRegistry(session)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		skipper := middleware.PrefixPathSkipper("/health", "/metrics")

		mw := HTTPMiddleware(registry, nil, skipper)
		wrapped := mw(handler)

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("uses SuffixPathSkipper", func(t *testing.T) {
		session := createTestSession("test")
		registry := NewRegistry(session)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		skipper := middleware.SuffixPathSkipper(".js", ".css")

		mw := HTTPMiddleware(registry, nil, skipper)
		wrapped := mw(handler)

		req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("uses EqualPathSkipper", func(t *testing.T) {
		session := createTestSession("test")
		registry := NewRegistry(session)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		skipper := middleware.EqualPathSkipper("/health", "/ready")

		mw := HTTPMiddleware(registry, nil, skipper)
		wrapped := mw(handler)

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func createTestSessionWithStore(name string, store Store) *Session {
	config := Config{
		Cookie: Cookie{
			Name: name,
		},
	}
	return New(config, store)
}
