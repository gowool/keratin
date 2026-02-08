package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gowool/keratin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecoverConfig_SetDefaults(t *testing.T) {
	tests := []struct {
		name         string
		config       RecoverConfig
		expectedSize int
	}{
		{
			name:         "sets default when StackSize is zero",
			config:       RecoverConfig{},
			expectedSize: 2 << 10,
		},
		{
			name:         "preserves positive StackSize",
			config:       RecoverConfig{StackSize: 4096},
			expectedSize: 4096,
		},
		{
			name:         "preserves custom small StackSize",
			config:       RecoverConfig{StackSize: 512},
			expectedSize: 512,
		},
		{
			name:         "preserves large StackSize",
			config:       RecoverConfig{StackSize: 1 << 20},
			expectedSize: 1 << 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.SetDefaults()
			assert.Equal(t, tt.expectedSize, tt.config.StackSize)
		})
	}
}

func TestRecover_PanicRecovery(t *testing.T) {
	tests := []struct {
		name       string
		panicValue interface{}
	}{
		{
			name:       "recovers from string panic",
			panicValue: "test panic",
		},
		{
			name:       "recovers from error panic",
			panicValue: errors.New("test error"),
		},
		{
			name:       "recovers from int panic",
			panicValue: 42,
		},
		{
			name:       "recovers from nil panic",
			panicValue: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
				panic(tt.panicValue)
			})

			middleware := Recover(RecoverConfig{})
			wrapped := middleware(handler)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()

			err := wrapped.ServeHTTP(rec, req)

			require.Error(t, err)
			assert.IsType(t, &keratin.HTTPError{}, err)

			httpErr, ok := err.(*keratin.HTTPError)
			require.True(t, ok)
			assert.Equal(t, http.StatusInternalServerError, httpErr.StatusCode())

			unwrapped := httpErr.Unwrap()
			require.NotNil(t, unwrapped)
			assert.Contains(t, unwrapped.Error(), "[PANIC RECOVER]")
		})
	}
}

func TestRecover_ErrAbortHandler(t *testing.T) {
	t.Run("re-panics http.ErrAbortHandler", func(t *testing.T) {
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			panic(http.ErrAbortHandler)
		})

		middleware := Recover(RecoverConfig{})
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		assert.Panics(t, func() {
			_ = wrapped.ServeHTTP(rec, req)
		})
	})
}

func TestRecover_NormalExecution(t *testing.T) {
	t.Run("passes through normal execution without panic", func(t *testing.T) {
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("success"))
			return nil
		})

		middleware := Recover(RecoverConfig{})
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "success", rec.Body.String())
	})
}

func TestRecover_HandlerError(t *testing.T) {
	t.Run("returns handler error when no panic", func(t *testing.T) {
		expectedErr := keratin.ErrNotFound
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			return expectedErr
		})

		middleware := Recover(RecoverConfig{})
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		assert.Error(t, err)
		assert.Same(t, expectedErr, err)
	})
}

func TestRecover_StackTrace(t *testing.T) {
	t.Run("captures stack trace in error", func(t *testing.T) {
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			panic("stack trace test")
		})

		middleware := Recover(RecoverConfig{})
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.Error(t, err)
		httpErr, ok := err.(*keratin.HTTPError)
		require.True(t, ok)

		unwrapped := httpErr.Unwrap()
		require.NotNil(t, unwrapped)
		assert.Contains(t, unwrapped.Error(), "[PANIC RECOVER]")
		assert.Contains(t, unwrapped.Error(), "stack trace test")
	})
}

func TestRecover_CustomStackSize(t *testing.T) {
	t.Run("uses custom StackSize", func(t *testing.T) {
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			panic("custom stack size test")
		})

		middleware := Recover(RecoverConfig{StackSize: 512})
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.Error(t, err)
		httpErr, ok := err.(*keratin.HTTPError)
		require.True(t, ok)

		unwrapped := httpErr.Unwrap()
		require.NotNil(t, unwrapped)
		errStr := unwrapped.Error()
		assert.Contains(t, errStr, "[PANIC RECOVER]")
	})
}

func TestRecover_ErrorWrapping(t *testing.T) {
	t.Run("wraps error correctly", func(t *testing.T) {
		originalErr := fmt.Errorf("database connection failed")
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			panic(originalErr)
		})

		middleware := Recover(RecoverConfig{})
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.Error(t, err)
		httpErr, ok := err.(*keratin.HTTPError)
		require.True(t, ok)

		assert.Equal(t, http.StatusInternalServerError, httpErr.StatusCode())

		unwrapped := httpErr.Unwrap()
		require.NotNil(t, unwrapped)
		assert.Contains(t, unwrapped.Error(), "[PANIC RECOVER]")
		assert.Contains(t, unwrapped.Error(), "database connection failed")
	})
}

func TestRecover_MiddlewareChaining(t *testing.T) {
	t.Run("works in middleware chain", func(t *testing.T) {
		called := false
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			called = true
			panic("chained panic")
		})

		recoverMiddleware := Recover(RecoverConfig{})
		loggingMiddleware := func(next keratin.Handler) keratin.Handler {
			return keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
				w.Header().Set("X-Logged", "true")
				return next.ServeHTTP(w, r)
			})
		}

		wrapped := loggingMiddleware(recoverMiddleware(handler))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.Error(t, err)
		assert.True(t, called)
		assert.Equal(t, "true", rec.Header().Get("X-Logged"))
	})
}

func TestRecover_RealWorldScenarios(t *testing.T) {
	t.Run("handles nil pointer dereference", func(t *testing.T) {
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			var ptr *int
			_ = *ptr
			return nil
		})

		middleware := Recover(RecoverConfig{})
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.Error(t, err)
		httpErr, ok := err.(*keratin.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusInternalServerError, httpErr.StatusCode())
	})

	t.Run("handles slice index out of bounds", func(t *testing.T) {
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			slice := []int{1, 2, 3}
			_ = slice[10]
			return nil
		})

		middleware := Recover(RecoverConfig{})
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.Error(t, err)
		httpErr, ok := err.(*keratin.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusInternalServerError, httpErr.StatusCode())
	})

	t.Run("handles type assertion failure", func(t *testing.T) {
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			var i interface{} = "string"
			_ = i.(int)
			return nil
		})

		middleware := Recover(RecoverConfig{})
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.Error(t, err)
		httpErr, ok := err.(*keratin.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusInternalServerError, httpErr.StatusCode())
	})
}

func TestRecover_EdgeCases(t *testing.T) {
	t.Run("handles empty string panic", func(t *testing.T) {
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			panic("")
		})

		middleware := Recover(RecoverConfig{})
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.Error(t, err)
		httpErr, ok := err.(*keratin.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusInternalServerError, httpErr.StatusCode())
	})

	t.Run("handles struct panic", func(t *testing.T) {
		type customPanic struct {
			Code    int
			Message string
		}

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			panic(customPanic{Code: 500, Message: "custom error"})
		})

		middleware := Recover(RecoverConfig{})
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.Error(t, err)
		httpErr, ok := err.(*keratin.HTTPError)
		require.True(t, ok)
		unwrapped := httpErr.Unwrap()
		require.NotNil(t, unwrapped)
		assert.Contains(t, unwrapped.Error(), "custom error")
	})

	t.Run("handles multiple panics in sequence", func(t *testing.T) {
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			_, _ = w.Write([]byte("before panic"))
			panic("sequential panic")
		})

		middleware := Recover(RecoverConfig{})
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.Error(t, err)
		httpErr, ok := err.(*keratin.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusInternalServerError, httpErr.StatusCode())
	})
}

func TestRecover_VariousMethods(t *testing.T) {
	tests := []struct {
		name   string
		method string
	}{
		{"GET request", http.MethodGet},
		{"POST request", http.MethodPost},
		{"PUT request", http.MethodPut},
		{"DELETE request", http.MethodDelete},
		{"PATCH request", http.MethodPatch},
		{"HEAD request", http.MethodHead},
		{"OPTIONS request", http.MethodOptions},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
				panic("method test")
			})

			middleware := Recover(RecoverConfig{})
			wrapped := middleware(handler)

			req := httptest.NewRequest(tt.method, "/", nil)
			rec := httptest.NewRecorder()

			err := wrapped.ServeHTTP(rec, req)

			require.Error(t, err)
			httpErr, ok := err.(*keratin.HTTPError)
			require.True(t, ok)
			assert.Equal(t, http.StatusInternalServerError, httpErr.StatusCode())
		})
	}
}

func TestRecover_PanicInNestedCalls(t *testing.T) {
	t.Run("recovers panic in deeply nested function calls", func(t *testing.T) {
		nestedFunc := func() {
			panic("nested panic")
		}

		middleFunc := func() {
			nestedFunc()
		}

		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			middleFunc()
			return nil
		})

		middleware := Recover(RecoverConfig{})
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.Error(t, err)
		httpErr, ok := err.(*keratin.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusInternalServerError, httpErr.StatusCode())
	})
}

func TestRecover_ErrorUnwrapping(t *testing.T) {
	t.Run("allows error unwrapping", func(t *testing.T) {
		originalErr := errors.New("wrapped error")
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			panic(originalErr)
		})

		middleware := Recover(RecoverConfig{})
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.Error(t, err)
		httpErr, ok := err.(*keratin.HTTPError)
		require.True(t, ok)

		unwrapped := httpErr.Unwrap()
		require.NotNil(t, unwrapped)
		assert.Contains(t, unwrapped.Error(), "[PANIC RECOVER]")
		assert.Contains(t, unwrapped.Error(), "wrapped error")
	})
}

func TestRecover_FormattedError(t *testing.T) {
	t.Run("formats panic string correctly", func(t *testing.T) {
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			panic("formatted panic message")
		})

		middleware := Recover(RecoverConfig{})
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.Error(t, err)
		httpErr, ok := err.(*keratin.HTTPError)
		require.True(t, ok)

		unwrapped := httpErr.Unwrap()
		require.NotNil(t, unwrapped)
		errStr := unwrapped.Error()
		assert.Contains(t, errStr, "[PANIC RECOVER]")
		assert.Contains(t, errStr, "formatted panic message")
		assert.Contains(t, errStr, "runtime") // should have stack trace
	})
}

func TestRecover_ConcurrentPanics(t *testing.T) {
	t.Run("handles concurrent requests with panics", func(t *testing.T) {
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			panic("concurrent panic")
		})

		middleware := Recover(RecoverConfig{})
		wrapped := middleware(handler)

		errChan := make(chan error, 10)

		for i := 0; i < 10; i++ {
			go func() {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				rec := httptest.NewRecorder()
				errChan <- wrapped.ServeHTTP(rec, req)
			}()
		}

		for i := 0; i < 10; i++ {
			err := <-errChan
			require.Error(t, err)
			httpErr, ok := err.(*keratin.HTTPError)
			require.True(t, ok)
			assert.Equal(t, http.StatusInternalServerError, httpErr.StatusCode())
		}
	})
}

func TestRecover_PanicAfterResponseWrite(t *testing.T) {
	t.Run("recovers panic after response write", func(t *testing.T) {
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			_, _ = w.Write([]byte("partial response"))
			panic("panic after write")
		})

		middleware := Recover(RecoverConfig{})
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.Error(t, err)
		assert.Contains(t, rec.Body.String(), "partial response")
	})
}

func TestRecover_LongPanicMessage(t *testing.T) {
	t.Run("handles long panic messages", func(t *testing.T) {
		longMsg := strings.Repeat("a", 10000)
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			panic(longMsg)
		})

		middleware := Recover(RecoverConfig{})
		wrapped := middleware(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)

		require.Error(t, err)
		httpErr, ok := err.(*keratin.HTTPError)
		require.True(t, ok)
		unwrapped := httpErr.Unwrap()
		require.NotNil(t, unwrapped)
		assert.Contains(t, unwrapped.Error(), "a")
	})
}
