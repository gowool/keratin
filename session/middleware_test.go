package session

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gowool/keratin"
	"github.com/gowool/keratin/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMiddleware(t *testing.T) {
	type testCase struct {
		name         string
		registry     *Registry
		logger       *slog.Logger
		skippers     []middleware.Skipper
		handlerSetup func(t *testing.T, registry *Registry) keratin.Handler
		requestSetup func(t *testing.T) *http.Request
		validate     func(t *testing.T, w *httptest.ResponseRecorder, err error, cookies []*http.Cookie)
		wantErr      bool
	}

	tests := []testCase{
		{
			name:     "empty registry passes through to next handler",
			registry: NewRegistry(),
			logger:   slog.New(slog.DiscardHandler),
			handlerSetup: func(t *testing.T, registry *Registry) keratin.Handler {
				return keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("success"))
					return nil
				})
			},
			requestSetup: func(t *testing.T) *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
			validate: func(t *testing.T, w *httptest.ResponseRecorder, err error, cookies []*http.Cookie) {
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Equal(t, "success", w.Body.String())
				assert.Len(t, cookies, 0)
			},
		},
		{
			name: "skipper returns true passes through to next handler",
			registry: NewRegistry(New(Config{
				Cookie: Cookie{Name: "session"},
			}, &MockStore{})),
			logger: slog.New(slog.DiscardHandler),
			skippers: []middleware.Skipper{
				func(r *http.Request) bool {
					return r.URL.Path == "/skip"
				},
			},
			handlerSetup: func(t *testing.T, registry *Registry) keratin.Handler {
				return keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("skipped"))
					return nil
				})
			},
			requestSetup: func(t *testing.T) *http.Request {
				return httptest.NewRequest(http.MethodGet, "/skip", nil)
			},
			validate: func(t *testing.T, w *httptest.ResponseRecorder, err error, cookies []*http.Cookie) {
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Equal(t, "skipped", w.Body.String())
				assert.Len(t, cookies, 0)
			},
		},
		{
			name: "nil logger uses discard handler",
			registry: NewRegistry(New(Config{
				Cookie: Cookie{Name: "session"},
			}, &MockStore{})),
			logger: nil,
			handlerSetup: func(t *testing.T, registry *Registry) keratin.Handler {
				return keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					w.WriteHeader(http.StatusOK)
					return nil
				})
			},
			requestSetup: func(t *testing.T) *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
			validate: func(t *testing.T, w *httptest.ResponseRecorder, err error, cookies []*http.Cookie) {
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, w.Code)
			},
		},
		{
			name: "session reads successfully and writes when modified",
			registry: func() *Registry {
				store := &MockStore{}
				store.On("Commit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
				return NewRegistry(New(Config{
					Cookie: Cookie{Name: "session"},
				}, store))
			}(),
			logger: slog.New(slog.DiscardHandler),
			handlerSetup: func(t *testing.T, registry *Registry) keratin.Handler {
				return keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					session := registry.Get("session")
					session.Put(r.Context(), "key", "value")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("success"))
					return nil
				})
			},
			requestSetup: func(t *testing.T) *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
			validate: func(t *testing.T, w *httptest.ResponseRecorder, err error, cookies []*http.Cookie) {
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Len(t, cookies, 1)
				assert.Equal(t, "session", cookies[0].Name)
			},
		},
		{
			name: "destroyed session writes empty cookie",
			registry: func() *Registry {
				store := &MockStore{}
				store.On("Delete", mock.Anything, mock.Anything).Return(nil)
				return NewRegistry(New(Config{
					Cookie: Cookie{Name: "session"},
				}, store))
			}(),
			logger: slog.New(slog.DiscardHandler),
			handlerSetup: func(t *testing.T, registry *Registry) keratin.Handler {
				return keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					session := registry.Get("session")
					_ = session.Destroy(r.Context())
					w.WriteHeader(http.StatusOK)
					return nil
				})
			},
			requestSetup: func(t *testing.T) *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
			validate: func(t *testing.T, w *httptest.ResponseRecorder, err error, cookies []*http.Cookie) {
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Len(t, cookies, 1)
				assert.Equal(t, "session", cookies[0].Name)
				assert.Equal(t, "", cookies[0].Value)
				assert.Equal(t, -1, cookies[0].MaxAge)
			},
		},
		{
			name: "unmodified session does not write cookie",
			registry: func() *Registry {
				store := &MockStore{}
				codec := NewGobCodec()
				deadline := time.Now().Add(1 * time.Hour).UTC()
				values := map[string]any{}
				data, _ := codec.Encode(deadline, values)
				store.On("Find", mock.Anything, "existing-token").Return(data, true, nil)
				return NewRegistry(New(Config{
					Cookie:   Cookie{Name: "session"},
					Lifetime: 1 * time.Hour,
				}, store))
			}(),
			logger: slog.New(slog.DiscardHandler),
			handlerSetup: func(t *testing.T, registry *Registry) keratin.Handler {
				return keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					w.WriteHeader(http.StatusOK)
					return nil
				})
			},
			requestSetup: func(t *testing.T) *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.AddCookie(&http.Cookie{
					Name:  "session",
					Value: "existing-token",
				})
				return req
			},
			validate: func(t *testing.T, w *httptest.ResponseRecorder, err error, cookies []*http.Cookie) {
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Len(t, cookies, 0)
			},
		},
		{
			name: "multiple sessions in registry all process",
			registry: func() *Registry {
				store1 := &MockStore{}
				store1.On("Commit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
				store2 := &MockStore{}
				store2.On("Commit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
				store3 := &MockStore{}
				store3.On("Commit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
				return NewRegistry(
					New(Config{
						Cookie: Cookie{Name: "session1"},
					}, store1),
					New(Config{
						Cookie: Cookie{Name: "session2"},
					}, store2),
					New(Config{
						Cookie: Cookie{Name: "session3"},
					}, store3),
				)
			}(),
			logger: slog.New(slog.DiscardHandler),
			handlerSetup: func(t *testing.T, registry *Registry) keratin.Handler {
				return keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					session1 := registry.Get("session1")
					session1.Put(r.Context(), "key", "value1")
					session2 := registry.Get("session2")
					session2.Put(r.Context(), "key", "value2")
					session3 := registry.Get("session3")
					session3.Put(r.Context(), "key", "value3")
					w.WriteHeader(http.StatusOK)
					return nil
				})
			},
			requestSetup: func(t *testing.T) *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
			validate: func(t *testing.T, w *httptest.ResponseRecorder, err error, cookies []*http.Cookie) {
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Len(t, cookies, 3)
				cookieNames := make(map[string]bool)
				for _, cookie := range cookies {
					cookieNames[cookie.Name] = true
				}
				assert.True(t, cookieNames["session1"])
				assert.True(t, cookieNames["session2"])
				assert.True(t, cookieNames["session3"])
			},
		},
		{
			name: "commit error is logged but response continues",
			registry: NewRegistry(New(Config{
				Cookie: Cookie{Name: "session"},
			}, &MockStore{})),
			logger: slog.New(slog.DiscardHandler),
			handlerSetup: func(t *testing.T, registry *Registry) keratin.Handler {
				return keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("success"))
					return nil
				})
			},
			requestSetup: func(t *testing.T) *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
			validate: func(t *testing.T, w *httptest.ResponseRecorder, err error, cookies []*http.Cookie) {
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Equal(t, "success", w.Body.String())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := tt.handlerSetup(t, tt.registry)
			mw := Middleware(tt.registry, tt.logger, tt.skippers...)
			wrapped := mw(handler)

			req := tt.requestSetup(t)
			w := httptest.NewRecorder()
			err := wrapped.ServeHTTP(w, req)

			cookies := w.Result().Cookies()
			tt.validate(t, w, err, cookies)

			if tt.wantErr {
				require.Error(t, err)
			}
		})
	}
}

func TestSessionWriter(t *testing.T) {
	t.Run("reset sets new response writer and clears before functions", func(t *testing.T) {
		sw := &sessionWriter{}
		sw.ResponseWriter = httptest.NewRecorder()
		sw.before = []func(){
			func() {},
			func() {},
		}

		newW := httptest.NewRecorder()
		sw.reset(newW)

		assert.Equal(t, newW, sw.ResponseWriter)
		assert.Nil(t, sw.before)
	})

	t.Run("WriteHeader executes all before functions", func(t *testing.T) {
		calls := []string{}
		sw := &sessionWriter{
			ResponseWriter: httptest.NewRecorder(),
			before: []func(){
				func() { calls = append(calls, "before1") },
				func() { calls = append(calls, "before2") },
				func() { calls = append(calls, "before3") },
			},
		}

		sw.WriteHeader(http.StatusOK)

		assert.Equal(t, []string{"before1", "before2", "before3"}, calls)
		assert.Equal(t, http.StatusOK, sw.ResponseWriter.(*httptest.ResponseRecorder).Code)
	})

	t.Run("WriteHeader with empty before functions", func(t *testing.T) {
		sw := &sessionWriter{
			ResponseWriter: httptest.NewRecorder(),
			before:         nil,
		}

		sw.WriteHeader(http.StatusOK)

		assert.Equal(t, http.StatusOK, sw.ResponseWriter.(*httptest.ResponseRecorder).Code)
	})

	t.Run("Unwrap returns underlying response writer", func(t *testing.T) {
		underlying := httptest.NewRecorder()
		sw := &sessionWriter{
			ResponseWriter: underlying,
		}

		unwrapped := sw.Unwrap()

		assert.Same(t, underlying, unwrapped)
	})
}

func TestMiddleware_WriteBeforeWriteHeader(t *testing.T) {
	t.Run("Write before WriteHeader triggers before functions", func(t *testing.T) {
		var called bool
		store := &MockStore{}
		store.On("Commit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
		registry := NewRegistry(New(Config{
			Cookie: Cookie{Name: "test"},
		}, store))

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			session := registry.Get("test")
			session.Put(r.Context(), "key", "value")
			w.WriteHeader(http.StatusOK)
			return nil
		})

		mw := Middleware(registry, slog.New(slog.DiscardHandler))
		wrapped := mw(handler)

		w := &customResponseWriter{
			ResponseWriter:    httptest.NewRecorder(),
			beforeWriteHeader: func() { called = true },
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		err := wrapped.ServeHTTP(w, req)

		require.NoError(t, err)
		assert.True(t, called)
	})
}

func TestMiddleware_Logging(t *testing.T) {
	t.Run("commit error is logged", func(t *testing.T) {
		var logMessages []string
		logHandler := &testLogHandler{
			logFunc: func(r slog.Record) {
				logMessages = append(logMessages, r.Message)
			},
		}
		logger := slog.New(logHandler)

		store := &MockStore{}
		store.On("Commit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(assert.AnError)
		session := New(Config{
			Cookie: Cookie{Name: "test"},
		}, store)

		registry := NewRegistry(session)
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			session := registry.Get("test")
			session.Put(r.Context(), "key", "value")
			w.WriteHeader(http.StatusOK)
			return nil
		})

		mw := Middleware(registry, logger)
		wrapped := mw(handler)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		err := wrapped.ServeHTTP(w, req)
		require.NoError(t, err)

		assert.Contains(t, logMessages, "failed to commit session")
	})
}

func TestMiddleware_ObjectPool(t *testing.T) {
	t.Run("sessionWriter is reused from pool", func(t *testing.T) {
		store := &MockStore{}
		store.On("Commit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
		registry := NewRegistry(New(Config{
			Cookie: Cookie{Name: "test"},
		}, store))

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			session := registry.Get("test")
			session.Put(r.Context(), "key", "value")
			w.WriteHeader(http.StatusOK)
			return nil
		})

		mw := Middleware(registry, slog.New(slog.DiscardHandler))
		wrapped := mw(handler)

		var firstWriter *sessionWriter

		for i := 0; i < 3; i++ {
			w := &captureWriter{ResponseWriter: httptest.NewRecorder()}
			req := httptest.NewRequest(http.MethodGet, "/", nil)

			err := wrapped.ServeHTTP(w, req)
			require.NoError(t, err)

			if i == 0 {
				firstWriter = w.capturedWriter
			} else {
				assert.Equal(t, firstWriter, w.capturedWriter, "sessionWriter should be reused from pool")
			}
		}
	})
}

type customResponseWriter struct {
	http.ResponseWriter
	beforeWriteHeader func()
}

func (w *customResponseWriter) WriteHeader(code int) {
	w.beforeWriteHeader()
	w.ResponseWriter.WriteHeader(code)
}

type testLogHandler struct {
	slog.Handler
	logFunc func(r slog.Record)
}

func (h *testLogHandler) Handle(ctx context.Context, r slog.Record) error {
	h.logFunc(r)
	return nil
}

func (h *testLogHandler) WithGroup(name string) slog.Handler {
	return h
}

func (h *testLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

type captureWriter struct {
	http.ResponseWriter
	capturedWriter *sessionWriter
}

func (w *captureWriter) WriteHeader(code int) {
	if sw, ok := w.ResponseWriter.(*sessionWriter); ok {
		w.capturedWriter = sw
	}
	w.ResponseWriter.WriteHeader(code)
}
