package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gowool/keratin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockHandler struct{}

func (m *mockHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (m *mockHandler) Handle(context.Context, slog.Record) error {
	return nil
}

func (m *mockHandler) WithAttrs([]slog.Attr) slog.Handler {
	return m
}

func (m *mockHandler) WithGroup(string) slog.Handler {
	return m
}

func TestRequestLoggerConfig_SetDefaults(t *testing.T) {
	t.Run("sets default values for nil config", func(t *testing.T) {
		cfg := RequestLoggerConfig{}
		cfg.SetDefaults()

		assert.NotNil(t, cfg.RequestLoggerAttrsFunc)
		assert.NotNil(t, cfg.ErrorStatusFunc)
		assert.NotNil(t, cfg.Logger)
	})

	t.Run("preserves existing RequestLoggerAttrsFunc", func(t *testing.T) {
		customFuncCalled := false
		customFunc := RequestLoggerAttrsFunc(func(w http.ResponseWriter, r *http.Request, metadata RequestMetadata) []slog.Attr {
			customFuncCalled = true
			return []slog.Attr{slog.String("custom", "value")}
		})
		cfg := RequestLoggerConfig{
			RequestLoggerAttrsFunc: customFunc,
			Logger:                 slog.New(&mockHandler{}),
		}
		cfg.SetDefaults()

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})
		middleware := RequestLogger(cfg)
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		_ = wrapped.ServeHTTP(rec, req)

		assert.True(t, customFuncCalled, "custom RequestLoggerAttrsFunc should be called")
	})

	t.Run("preserves existing ErrorStatusFunc", func(t *testing.T) {
		customFuncCalled := false
		customFunc := ErrorStatusFunc(func(ctx context.Context, err error) int {
			customFuncCalled = true
			return 999
		})
		cfg := RequestLoggerConfig{
			ErrorStatusFunc: customFunc,
			Logger:          slog.New(&mockHandler{}),
		}
		cfg.SetDefaults()

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			return errors.New("test error")
		})
		middleware := RequestLogger(cfg)
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		_ = wrapped.ServeHTTP(rec, req)

		assert.True(t, customFuncCalled, "custom ErrorStatusFunc should be called")
	})

	t.Run("preserves existing Logger", func(t *testing.T) {
		logger := slog.New(&mockHandler{})
		cfg := RequestLoggerConfig{
			Logger: logger,
		}
		cfg.SetDefaults()

		assert.NotNil(t, cfg.Logger)
	})
}

func TestRequestLogger_Success(t *testing.T) {
	t.Run("logs successful request with status 200", func(t *testing.T) {
		var loggedAttrs []slog.Attr
		mockLogAttrs := func(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
			assert.Equal(t, slog.LevelInfo, level)
			assert.Equal(t, "incoming request", msg)
			loggedAttrs = attrs
		}

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("success"))
			return nil
		})

		cfg := RequestLoggerConfig{
			Logger: slog.New(&testLogHandler{logAttrs: mockLogAttrs}),
		}
		middleware := RequestLogger(cfg)
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		req.Header.Set("User-Agent", "test-agent")
		req.Header.Set("Referer", "http://example.com")
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		assert.Contains(t, attrsToString(loggedAttrs), "method: GET")
		assert.Contains(t, attrsToString(loggedAttrs), "path: /test")
		assert.Contains(t, attrsToString(loggedAttrs), "remote_addr: 127.0.0.1:12345")
		assert.Contains(t, attrsToString(loggedAttrs), "user_agent: test-agent")
		assert.Contains(t, attrsToString(loggedAttrs), "referer: http://example.com")
	})

	t.Run("logs successful request with status 201", func(t *testing.T) {
		var loggedLevel slog.Level
		mockLogAttrs := func(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
			loggedLevel = level
		}

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusCreated)
			return nil
		})

		cfg := RequestLoggerConfig{
			Logger: slog.New(&testLogHandler{logAttrs: mockLogAttrs}),
		}
		middleware := RequestLogger(cfg)
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodPost, "/users", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.Equal(t, slog.LevelInfo, loggedLevel)
	})
}

func TestRequestLogger_ClientError(t *testing.T) {
	t.Run("logs client error with warn level", func(t *testing.T) {
		var loggedLevel slog.Level
		mockLogAttrs := func(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
			loggedLevel = level
		}

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("bad request"))
			return nil
		})

		cfg := RequestLoggerConfig{
			Logger: slog.New(&testLogHandler{logAttrs: mockLogAttrs}),
		}
		middleware := RequestLogger(cfg)
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := newTestRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, slog.LevelWarn, loggedLevel)
	})

	t.Run("logs not found with warn level", func(t *testing.T) {
		var loggedLevel slog.Level
		mockLogAttrs := func(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
			loggedLevel = level
		}

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusNotFound)
			return nil
		})

		cfg := RequestLoggerConfig{
			Logger: slog.New(&testLogHandler{logAttrs: mockLogAttrs}),
		}
		middleware := RequestLogger(cfg)
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/notfound", nil)
		rec := newTestRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Equal(t, slog.LevelWarn, loggedLevel)
	})
}

func TestRequestLogger_ServerError(t *testing.T) {
	t.Run("logs server error with error level", func(t *testing.T) {
		var loggedLevel slog.Level
		mockLogAttrs := func(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
			loggedLevel = level
		}

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusInternalServerError)
			return nil
		})

		cfg := RequestLoggerConfig{
			Logger: slog.New(&testLogHandler{logAttrs: mockLogAttrs}),
		}
		middleware := RequestLogger(cfg)
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := newTestRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Equal(t, slog.LevelError, loggedLevel)
	})

	t.Run("logs 503 service unavailable with error level", func(t *testing.T) {
		var loggedLevel slog.Level
		mockLogAttrs := func(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
			loggedLevel = level
		}

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusServiceUnavailable)
			return nil
		})

		cfg := RequestLoggerConfig{
			Logger: slog.New(&testLogHandler{logAttrs: mockLogAttrs}),
		}
		middleware := RequestLogger(cfg)
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := newTestRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Equal(t, slog.LevelError, loggedLevel)
	})
}

func TestRequestLogger_HandlerError(t *testing.T) {
	t.Run("logs handler error with appropriate level", func(t *testing.T) {
		var loggedLevel slog.Level
		var loggedAttrs []slog.Attr
		mockLogAttrs := func(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
			loggedLevel = level
			loggedAttrs = attrs
		}

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			return keratin.ErrNotFound
		})

		cfg := RequestLoggerConfig{
			Logger: slog.New(&testLogHandler{logAttrs: mockLogAttrs}),
		}
		middleware := RequestLogger(cfg)
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.Error(t, err)
		assert.Same(t, keratin.ErrNotFound, err)
		assert.Equal(t, slog.LevelWarn, loggedLevel, "404 errors should log at WARN level")
		assert.Contains(t, attrsToString(loggedAttrs), "error")
	})

	t.Run("returns error from handler", func(t *testing.T) {
		expectedErr := keratin.ErrBadRequest
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			return expectedErr
		})

		cfg := RequestLoggerConfig{}
		middleware := RequestLogger(cfg)
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		assert.Error(t, err)
		assert.Same(t, expectedErr, err)
	})
}

func TestRequestLogger_Skipper(t *testing.T) {
	t.Run("skips logging when skipper returns true", func(t *testing.T) {
		called := false
		mockLogAttrs := func(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
			called = true
		}

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		cfg := RequestLoggerConfig{
			Logger: slog.New(&testLogHandler{logAttrs: mockLogAttrs}),
		}
		middleware := RequestLogger(cfg, PrefixPathSkipper("/health"))
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.False(t, called, "logging should be skipped")
	})

	t.Run("logs when skipper returns false", func(t *testing.T) {
		called := false
		mockLogAttrs := func(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
			called = true
		}

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		cfg := RequestLoggerConfig{
			Logger: slog.New(&testLogHandler{logAttrs: mockLogAttrs}),
		}
		middleware := RequestLogger(cfg, PrefixPathSkipper("/health"))
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.True(t, called, "logging should occur")
	})

	t.Run("handles multiple skippers", func(t *testing.T) {
		called := false
		mockLogAttrs := func(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
			called = true
		}

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		cfg := RequestLoggerConfig{
			Logger: slog.New(&testLogHandler{logAttrs: mockLogAttrs}),
		}
		middleware := RequestLogger(cfg, PrefixPathSkipper("/health"), PrefixPathSkipper("/metrics"))
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.False(t, called, "logging should be skipped by second skipper")
	})
}

func TestRequestLogger_CustomErrorStatusFunc(t *testing.T) {
	t.Run("uses custom error status function", func(t *testing.T) {
		var loggedStatus int
		mockLogAttrs := func(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
			for _, attr := range attrs {
				if attr.Key == "status_code" {
					loggedStatus = int(attr.Value.Int64())
				}
			}
		}

		customErr := errors.New("custom error")
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			return customErr
		})

		customErrorStatusFunc := ErrorStatusFunc(func(ctx context.Context, err error) int {
			return 418
		})

		cfg := RequestLoggerConfig{
			ErrorStatusFunc: customErrorStatusFunc,
			Logger:          slog.New(&testLogHandler{logAttrs: mockLogAttrs}),
		}
		middleware := RequestLogger(cfg)
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.Error(t, err)
		assert.Equal(t, 418, loggedStatus)
	})
}

func TestRequestLogger_RequestID(t *testing.T) {
	t.Run("includes request_id from request header", func(t *testing.T) {
		var loggedAttrs []slog.Attr
		mockLogAttrs := func(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
			loggedAttrs = attrs
		}

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		cfg := RequestLoggerConfig{
			Logger: slog.New(&testLogHandler{logAttrs: mockLogAttrs}),
		}
		middleware := RequestLogger(cfg)
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set(keratin.HeaderXRequestID, "req-123")
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.Contains(t, attrsToString(loggedAttrs), "request_id: req-123")
	})

	t.Run("includes request_id from response header", func(t *testing.T) {
		var loggedAttrs []slog.Attr
		mockLogAttrs := func(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
			loggedAttrs = attrs
		}

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Set(keratin.HeaderXRequestID, "req-456")
			w.WriteHeader(http.StatusOK)
			return nil
		})

		cfg := RequestLoggerConfig{
			Logger: slog.New(&testLogHandler{logAttrs: mockLogAttrs}),
		}
		middleware := RequestLogger(cfg)
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.Contains(t, attrsToString(loggedAttrs), "request_id: req-456")
	})

	t.Run("prioritizes request header over response header", func(t *testing.T) {
		var loggedAttrs []slog.Attr
		mockLogAttrs := func(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
			loggedAttrs = attrs
		}

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Set(keratin.HeaderXRequestID, "response-id")
			w.WriteHeader(http.StatusOK)
			return nil
		})

		cfg := RequestLoggerConfig{
			Logger: slog.New(&testLogHandler{logAttrs: mockLogAttrs}),
		}
		middleware := RequestLogger(cfg)
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set(keratin.HeaderXRequestID, "request-id")
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.Contains(t, attrsToString(loggedAttrs), "request_id: request-id")
		assert.NotContains(t, attrsToString(loggedAttrs), "request_id: response-id")
	})
}

func TestRequestLogger_ResponseSize(t *testing.T) {
	t.Run("response is written correctly", func(t *testing.T) {
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			_, _ = w.Write([]byte("hello world"))
			return nil
		})

		cfg := RequestLoggerConfig{
			Logger: slog.New(&mockHandler{}),
		}
		middleware := RequestLogger(cfg)
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.Equal(t, 11, rec.Body.Len(), "body should be written")
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("logs correct response size with custom recorder", func(t *testing.T) {
		var loggedSize int64
		mockLogAttrs := func(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
			for _, attr := range attrs {
				if attr.Key == "response_size" {
					loggedSize = attr.Value.Int64()
				}
			}
		}

		responseBody := strings.Repeat("x", 100)
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			_, _ = w.Write([]byte(responseBody))
			return nil
		})

		cfg := RequestLoggerConfig{
			Logger: slog.New(&testLogHandler{logAttrs: mockLogAttrs}),
		}
		middleware := RequestLogger(cfg)
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := newTestRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.Equal(t, int64(100), loggedSize)
	})
}

func TestRequestLogger_Latency(t *testing.T) {
	t.Run("logs request latency", func(t *testing.T) {
		var loggedLatency string
		mockLogAttrs := func(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
			for _, attr := range attrs {
				if attr.Key == "latency" {
					loggedLatency = attr.Value.String()
				}
			}
		}

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			return nil
		})

		cfg := RequestLoggerConfig{
			Logger: slog.New(&testLogHandler{logAttrs: mockLogAttrs}),
		}
		middleware := RequestLogger(cfg)
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.NotEmpty(t, loggedLatency)
		assert.True(t, strings.Contains(loggedLatency, "ms") || strings.Contains(loggedLatency, "s"))
	})
}

func TestRequestLogger_Timestamps(t *testing.T) {
	t.Run("logs start and end time", func(t *testing.T) {
		var startTime, endTime time.Time
		mockLogAttrs := func(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
			for _, attr := range attrs {
				if attr.Key == "start_time" {
					startTime = attr.Value.Time()
				}
				if attr.Key == "end_time" {
					endTime = attr.Value.Time()
				}
			}
		}

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			time.Sleep(5 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			return nil
		})

		cfg := RequestLoggerConfig{
			Logger: slog.New(&testLogHandler{logAttrs: mockLogAttrs}),
		}
		middleware := RequestLogger(cfg)
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.False(t, startTime.IsZero())
		assert.False(t, endTime.IsZero())
		assert.True(t, endTime.After(startTime) || endTime.Equal(startTime))
	})
}

func TestRequestLogger_VariousMethods(t *testing.T) {
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodHead,
		http.MethodOptions,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			var loggedMethod string
			mockLogAttrs := func(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
				for _, attr := range attrs {
					if attr.Key == "method" {
						loggedMethod = attr.Value.String()
					}
				}
			}

			handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
				w.WriteHeader(http.StatusOK)
				return nil
			})

			cfg := RequestLoggerConfig{
				Logger: slog.New(&testLogHandler{logAttrs: mockLogAttrs}),
			}
			middleware := RequestLogger(cfg)
			wrapped := middleware(handler)

			req := httptest.NewRequest(method, "/test", nil)
			rec := httptest.NewRecorder()

			err := wrapped.ServeHTTP(rec, req)

			require.NoError(t, err)
			assert.Equal(t, method, loggedMethod)
		})
	}
}

func TestRequestLogger_Chaining(t *testing.T) {
	t.Run("works in middleware chain", func(t *testing.T) {
		var loggedAttrs []slog.Attr
		mockLogAttrs := func(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
			loggedAttrs = attrs
		}

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("response"))
			return nil
		})

		customMiddleware := func(next keratin.Handler) keratin.Handler {
			return keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
				w.Header().Set("X-Custom", "value")
				return next.ServeHTTP(w, r)
			})
		}

		cfg := RequestLoggerConfig{
			Logger: slog.New(&testLogHandler{logAttrs: mockLogAttrs}),
		}
		requestLogger := RequestLogger(cfg)
		wrapped := customMiddleware(requestLogger(handler))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.Equal(t, "value", rec.Header().Get("X-Custom"))
		assert.NotEmpty(t, loggedAttrs)
	})
}

func TestRequestLogger_CustomRequestLoggerAttrsFunc(t *testing.T) {
	t.Run("uses custom attrs function", func(t *testing.T) {
		var loggedAttrs []slog.Attr
		mockLogAttrs := func(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
			loggedAttrs = attrs
		}

		customAttrsFunc := RequestLoggerAttrsFunc(func(w http.ResponseWriter, r *http.Request, metadata RequestMetadata) []slog.Attr {
			return []slog.Attr{
				slog.String("custom_attr", "custom_value"),
				slog.Int("status_code", metadata.StatusCode),
			}
		})

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		cfg := RequestLoggerConfig{
			RequestLoggerAttrsFunc: customAttrsFunc,
			Logger:                 slog.New(&testLogHandler{logAttrs: mockLogAttrs}),
		}
		middleware := RequestLogger(cfg)
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		attrsStr := attrsToString(loggedAttrs)
		assert.Contains(t, attrsStr, "custom_attr: custom_value")
	})
}

func TestRequestLoggerAttrs(t *testing.T) {
	t.Run("generates all default attributes", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/users/123?query=test", nil)
		req.Host = "example.com"
		req.RemoteAddr = "192.168.1.1:54321"
		req.Header.Set("User-Agent", "test-browser")
		req.Header.Set("Referer", "http://referrer.com")
		req.Header.Set("Content-Length", "100")
		req.Proto = "HTTP/1.1"

		rec := httptest.NewRecorder()
		rec.WriteHeader(http.StatusOK)
		_, _ = rec.Write([]byte("test response"))

		metadata := RequestMetadata{
			StatusCode: http.StatusOK,
			Error:      nil,
			StartTime:  time.Now().UTC().Add(-10 * time.Millisecond),
			EndTime:    time.Now().UTC(),
		}

		attrsFunc := RequestLoggerAttrs()
		attrs := attrsFunc(rec, req, metadata)

		attrMap := attrsToMap(attrs)

		assert.Contains(t, attrMap, "latency")
		assert.Equal(t, "POST", attrMap["method"].(string))
		assert.Equal(t, int64(200), attrMap["status_code"].(int64))
		assert.Equal(t, "HTTP/1.1", attrMap["protocol"].(string))
		assert.Equal(t, "example.com", attrMap["host"].(string))
		assert.Equal(t, "/api/users/123", attrMap["path"].(string))
		assert.Contains(t, attrMap["uri"].(string), "/api/users/123?query=test")
		assert.Equal(t, "192.168.1.1:54321", attrMap["remote_addr"].(string))
		assert.Equal(t, "http://referrer.com", attrMap["referer"].(string))
		assert.Equal(t, "test-browser", attrMap["user_agent"].(string))
		assert.Equal(t, "100", attrMap["request_size"].(string))
		assert.False(t, attrMap["start_time"].(time.Time).IsZero())
		assert.False(t, attrMap["end_time"].(time.Time).IsZero())
	})

	t.Run("includes error in attributes when present", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		testErr := errors.New("test error")
		metadata := RequestMetadata{
			StatusCode: http.StatusInternalServerError,
			Error:      testErr,
			StartTime:  time.Now().UTC(),
			EndTime:    time.Now().UTC(),
		}

		attrsFunc := RequestLoggerAttrs()
		attrs := attrsFunc(rec, req, metadata)

		attrMap := attrsToMap(attrs)
		assert.Contains(t, attrMap, "error")
	})

	t.Run("does not include error when nil", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		metadata := RequestMetadata{
			StatusCode: http.StatusOK,
			Error:      nil,
			StartTime:  time.Now().UTC(),
			EndTime:    time.Now().UTC(),
		}

		attrsFunc := RequestLoggerAttrs()
		attrs := attrsFunc(rec, req, metadata)

		attrMap := attrsToMap(attrs)
		assert.NotContains(t, attrMap, "error")
	})

	t.Run("includes response_size when sizer available", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := newTestRecorder()
		_, _ = rec.Write([]byte("test data"))

		metadata := RequestMetadata{
			StatusCode: http.StatusOK,
			StartTime:  time.Now().UTC(),
			EndTime:    time.Now().UTC(),
		}

		attrsFunc := RequestLoggerAttrs()
		attrs := attrsFunc(rec, req, metadata)

		attrMap := attrsToMap(attrs)
		assert.Contains(t, attrMap, "response_size")
		assert.Equal(t, int64(9), attrMap["response_size"].(int64))
	})
}

func TestRequestLogger_EdgeCases(t *testing.T) {
	t.Run("handles request without User-Agent", func(t *testing.T) {
		var loggedUA string
		mockLogAttrs := func(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
			for _, attr := range attrs {
				if attr.Key == "user_agent" {
					loggedUA = attr.Value.String()
				}
			}
		}

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		cfg := RequestLoggerConfig{
			Logger: slog.New(&testLogHandler{logAttrs: mockLogAttrs}),
		}
		middleware := RequestLogger(cfg)
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.Equal(t, "", loggedUA)
	})

	t.Run("handles request without Referer", func(t *testing.T) {
		var loggedReferer string
		mockLogAttrs := func(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
			for _, attr := range attrs {
				if attr.Key == "referer" {
					loggedReferer = attr.Value.String()
				}
			}
		}

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		cfg := RequestLoggerConfig{
			Logger: slog.New(&testLogHandler{logAttrs: mockLogAttrs}),
		}
		middleware := RequestLogger(cfg)
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.Equal(t, "", loggedReferer)
	})

	t.Run("handles request without Content-Length", func(t *testing.T) {
		var loggedCL string
		mockLogAttrs := func(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
			for _, attr := range attrs {
				if attr.Key == "request_size" {
					loggedCL = attr.Value.String()
				}
			}
		}

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		cfg := RequestLoggerConfig{
			Logger: slog.New(&testLogHandler{logAttrs: mockLogAttrs}),
		}
		middleware := RequestLogger(cfg)
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.Equal(t, "", loggedCL)
	})

	t.Run("handles empty pattern", func(t *testing.T) {
		var loggedPattern string
		mockLogAttrs := func(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
			for _, attr := range attrs {
				if attr.Key == "pattern" {
					loggedPattern = attr.Value.String()
				}
			}
		}

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		cfg := RequestLoggerConfig{
			Logger: slog.New(&testLogHandler{logAttrs: mockLogAttrs}),
		}
		middleware := RequestLogger(cfg)
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.Equal(t, "", loggedPattern)
	})
}

type testLogHandler struct {
	logAttrs func(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr)
}

func (h *testLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

func (h *testLogHandler) Handle(ctx context.Context, r slog.Record) error {
	var attrs []slog.Attr
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a)
		return true
	})
	h.logAttrs(ctx, r.Level, r.Message, attrs...)
	return nil
}

func (h *testLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *testLogHandler) WithGroup(name string) slog.Handler {
	return h
}

type testRecorder struct {
	*httptest.ResponseRecorder
}

func newTestRecorder() *testRecorder {
	return &testRecorder{
		ResponseRecorder: httptest.NewRecorder(),
	}
}

func (r *testRecorder) StatusCode() int {
	return r.Code
}

func (r *testRecorder) Size() int64 {
	return int64(r.Body.Len())
}

func (r *testRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseRecorder
}

func attrsToString(attrs []slog.Attr) string {
	var sb strings.Builder
	for _, attr := range attrs {
		sb.WriteString(attr.Key)
		sb.WriteString(": ")
		sb.WriteString(attr.Value.String())
		sb.WriteString(", ")
	}
	return sb.String()
}

func attrsToMap(attrs []slog.Attr) map[string]any {
	m := make(map[string]any)
	for _, attr := range attrs {
		m[attr.Key] = attr.Value.Any()
	}
	return m
}
