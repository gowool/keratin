package middleware

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gowool/keratin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockReadCloserWithReread struct {
	io.ReadCloser
	rereadCalled bool
}

func (m *mockReadCloserWithReread) Reread() {
	m.rereadCalled = true
}

func TestBodyLimitConfig_SetDefaults(t *testing.T) {
	tests := []struct {
		name        string
		config      BodyLimitConfig
		expectedLim int64
	}{
		{
			name:        "sets default when limit is zero",
			config:      BodyLimitConfig{},
			expectedLim: maxBodySize,
		},
		{
			name:        "preserves positive limit",
			config:      BodyLimitConfig{Limit: 1024},
			expectedLim: 1024,
		},
		{
			name:        "preserves negative limit (no limit)",
			config:      BodyLimitConfig{Limit: -1},
			expectedLim: -1,
		},
		{
			name:        "preserves custom large limit",
			config:      BodyLimitConfig{Limit: 100 << 20},
			expectedLim: 100 << 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.SetDefaults()
			assert.Equal(t, tt.expectedLim, tt.config.Limit)
		})
	}
}

func TestBodyLimit_Skipper(t *testing.T) {
	t.Run("skips when skipper returns true", func(t *testing.T) {
		skipped := false
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			skipped = true
			return nil
		})

		middleware := BodyLimit(BodyLimitConfig{Limit: 100}, func(r *http.Request) bool {
			return true
		})

		wrapped := middleware(handler)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("test data"))
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)
		require.NoError(t, err)
		assert.True(t, skipped)
	})

	t.Run("doesn't skip when skipper returns false", func(t *testing.T) {
		processed := false
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			processed = true
			w.WriteHeader(http.StatusOK)
			return nil
		})

		middleware := BodyLimit(BodyLimitConfig{Limit: 100}, func(r *http.Request) bool {
			return false
		})

		wrapped := middleware(handler)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("test"))
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)
		require.NoError(t, err)
		assert.True(t, processed)
	})
}

func TestBodyLimit_NoLimit(t *testing.T) {
	t.Run("no limit when config limit is negative", func(t *testing.T) {
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			body, _ := io.ReadAll(r.Body)
			_, _ = w.Write(body)
			return nil
		})

		middleware := BodyLimit(BodyLimitConfig{Limit: -1})

		wrapped := middleware(handler)
		body := strings.Repeat("x", 1000)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)
		require.NoError(t, err)
		assert.Equal(t, body, rec.Body.String())
	})
}

func TestBodyLimit_ContentLengthCheck(t *testing.T) {
	t.Run("optimistic check with ContentLength exceeding limit", func(t *testing.T) {
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			return nil
		})

		middleware := BodyLimit(BodyLimitConfig{Limit: 100})

		wrapped := middleware(handler)
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.ContentLength = 200

		rec := httptest.NewRecorder()
		err := wrapped.ServeHTTP(rec, req)

		assert.Error(t, err)
		assert.Equal(t, keratin.ErrRequestEntityTooLarge, err)
	})

	t.Run("optimistic check with ContentLength within limit", func(t *testing.T) {
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		middleware := BodyLimit(BodyLimitConfig{Limit: 1000})

		wrapped := middleware(handler)
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.ContentLength = 100

		rec := httptest.NewRecorder()
		err := wrapped.ServeHTTP(rec, req)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestBodyLimit_ReadWithinLimit(t *testing.T) {
	tests := []struct {
		name     string
		limit    int64
		body     string
		expected string
	}{
		{
			name:     "small body within limit",
			limit:    100,
			body:     "hello world",
			expected: "hello world",
		},
		{
			name:     "body exactly at limit",
			limit:    10,
			body:     "0123456789",
			expected: "0123456789",
		},
		{
			name:     "large body within large limit",
			limit:    1024,
			body:     strings.Repeat("x", 500),
			expected: strings.Repeat("x", 500),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					return err
				}
				_, _ = w.Write(body)
				return nil
			})

			middleware := BodyLimit(BodyLimitConfig{Limit: tt.limit})

			wrapped := middleware(handler)
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			err := wrapped.ServeHTTP(rec, req)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, rec.Body.String())
		})
	}
}

func TestBodyLimit_ExceedsLimit(t *testing.T) {
	tests := []struct {
		name  string
		limit int64
		body  string
	}{
		{
			name:  "body exceeds limit on single read",
			limit: 10,
			body:  "this is too long",
		},
		{
			name:  "body exceeds limit on multiple reads",
			limit: 20,
			body:  "this is a very long body that exceeds the limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
				_, err := io.ReadAll(r.Body)
				if err != nil {
					return err
				}
				return nil
			})

			middleware := BodyLimit(BodyLimitConfig{Limit: tt.limit})

			wrapped := middleware(handler)
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			err := wrapped.ServeHTTP(rec, req)
			assert.Error(t, err)
			assert.Equal(t, keratin.ErrRequestEntityTooLarge, err)
		})
	}
}

func TestBodyLimit_DefaultLimit(t *testing.T) {
	t.Run("uses default 32MB limit", func(t *testing.T) {
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		middleware := BodyLimit(BodyLimitConfig{})

		wrapped := middleware(handler)
		body := strings.Repeat("x", 1000)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestLimitedReader_Read(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		limit       int64
		expectedErr bool
		expectedN   int
	}{
		{
			name:        "read within limit",
			input:       "hello",
			limit:       10,
			expectedErr: false,
			expectedN:   5,
		},
		{
			name:        "read exceeds limit",
			input:       "hello world",
			limit:       5,
			expectedErr: true,
			expectedN:   11,
		},
		{
			name:        "empty read",
			input:       "",
			limit:       10,
			expectedErr: true,
			expectedN:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := &limitedReader{
				ReadCloser: io.NopCloser(strings.NewReader(tt.input)),
				limit:      tt.limit,
			}

			buf := make([]byte, 100)
			n, err := reader.Read(buf)

			if tt.expectedErr {
				assert.Error(t, err)
				if tt.name != "empty read" {
					assert.Equal(t, keratin.ErrRequestEntityTooLarge, err)
				}
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expectedN, n)
		})
	}
}

func TestLimitedReader_MultipleReads(t *testing.T) {
	t.Run("multiple reads accumulate to exceed limit", func(t *testing.T) {
		reader := &limitedReader{
			ReadCloser: io.NopCloser(strings.NewReader("1234567890")),
			limit:      6,
		}

		buf1 := make([]byte, 4)
		n1, err1 := reader.Read(buf1)
		require.NoError(t, err1)
		assert.Equal(t, 4, n1)
		assert.Equal(t, "1234", string(buf1[:n1]))

		buf2 := make([]byte, 4)
		n2, err2 := reader.Read(buf2)
		assert.Error(t, err2)
		assert.Equal(t, keratin.ErrRequestEntityTooLarge, err2)
		assert.Equal(t, 4, n2)
		assert.Equal(t, "5678", string(buf2[:n2]))
	})

	t.Run("multiple reads within limit", func(t *testing.T) {
		reader := &limitedReader{
			ReadCloser: io.NopCloser(strings.NewReader("123456")),
			limit:      10,
		}

		buf1 := make([]byte, 3)
		n1, err1 := reader.Read(buf1)
		require.NoError(t, err1)
		assert.Equal(t, 3, n1)

		buf2 := make([]byte, 3)
		n2, err2 := reader.Read(buf2)
		require.NoError(t, err2)
		assert.Equal(t, 3, n2)

		assert.Equal(t, int64(6), reader.totalRead)
	})
}

func TestLimitedReader_Reread(t *testing.T) {
	t.Run("reread supported", func(t *testing.T) {
		mock := &mockReadCloserWithReread{
			ReadCloser: io.NopCloser(strings.NewReader("test")),
		}

		reader := &limitedReader{
			ReadCloser: mock,
			limit:      10,
		}

		reader.Reread()

		assert.True(t, mock.rereadCalled)
	})

	t.Run("reread not supported", func(t *testing.T) {
		reader := &limitedReader{
			ReadCloser: io.NopCloser(strings.NewReader("test")),
			limit:      10,
		}

		assert.NotPanics(t, func() {
			reader.Reread()
		})
	})
}

func TestBodyLimit_RealWorldScenarios(t *testing.T) {
	t.Run("multipart form data within limit", func(t *testing.T) {
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		middleware := BodyLimit(BodyLimitConfig{Limit: 1024})

		wrapped := middleware(handler)
		body := strings.NewReader("------boundary\r\nContent-Disposition: form-data; name=\"field\"\r\n\r\nvalue\r\n------boundary--")
		req := httptest.NewRequest(http.MethodPost, "/", body)
		req.Header.Set("Content-Type", "multipart/form-data; boundary=----boundary")
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)
		require.NoError(t, err)
	})

	t.Run("JSON body within limit", func(t *testing.T) {
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return nil
		})

		middleware := BodyLimit(BodyLimitConfig{Limit: 100})

		wrapped := middleware(handler)
		body := `{"name":"test","value":123}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)
		require.NoError(t, err)
		assert.Equal(t, `{"status":"ok"}`, rec.Body.String())
	})
}

func TestBodyLimit_MiddlewareChaining(t *testing.T) {
	t.Run("body limit can be chained with other middleware", func(t *testing.T) {
		called := false
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			called = true
			w.WriteHeader(http.StatusOK)
			return nil
		})

		bodyLimitMiddleware := BodyLimit(BodyLimitConfig{Limit: 1000})

		wrapped := func(next keratin.Handler) keratin.Handler {
			return keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
				w.Header().Set("X-Middleware", "1")
				return next.ServeHTTP(w, r)
			})
		}

		chain := wrapped(bodyLimitMiddleware(handler))

		body := strings.Repeat("x", 100)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		rec := httptest.NewRecorder()

		err := chain.ServeHTTP(rec, req)
		require.NoError(t, err)
		assert.True(t, called)
		assert.Equal(t, "1", rec.Header().Get("X-Middleware"))
	})
}

func TestBodyLimit_BodyReplacement(t *testing.T) {
	t.Run("replaces request body with limitedReader", func(t *testing.T) {
		originalBody := strings.NewReader("test body")
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			_, ok := r.Body.(*limitedReader)
			assert.True(t, ok, "body should be replaced with limitedReader")
			w.WriteHeader(http.StatusOK)
			return nil
		})

		middleware := BodyLimit(BodyLimitConfig{Limit: 1000})

		wrapped := middleware(handler)
		req := httptest.NewRequest(http.MethodPost, "/", originalBody)
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)
		require.NoError(t, err)
	})
}

func TestLimitedReader_ErrorPropagation(t *testing.T) {
	t.Run("propagates read errors", func(t *testing.T) {
		expectedErr := errors.New("read error")
		errorReader := &errorReader{
			err: expectedErr,
		}

		reader := &limitedReader{
			ReadCloser: io.NopCloser(errorReader),
			limit:      10,
		}

		buf := make([]byte, 100)
		_, err := reader.Read(buf)

		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
	})
}

type errorReader struct {
	err error
}

func (e *errorReader) Read([]byte) (n int, err error) {
	return 0, e.err
}

func TestBodyLimit_EmptyBody(t *testing.T) {
	t.Run("handles empty body", func(t *testing.T) {
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				return err
			}
			_, _ = w.Write(body)
			return nil
		})

		middleware := BodyLimit(BodyLimitConfig{Limit: 100})

		wrapped := middleware(handler)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)
		require.NoError(t, err)
		assert.Equal(t, "", rec.Body.String())
	})
}

func TestBodyLimit_ChunkedEncoding(t *testing.T) {
	t.Run("handles chunked encoding without Content-Length", func(t *testing.T) {
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				return err
			}
			_, _ = w.Write(body)
			return nil
		})

		middleware := BodyLimit(BodyLimitConfig{Limit: 100})

		wrapped := middleware(handler)
		body := strings.Repeat("x", 50)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req.ContentLength = -1
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)
		require.NoError(t, err)
		assert.Equal(t, body, rec.Body.String())
	})
}

func TestBodyLimit_EdgeCases(t *testing.T) {
	t.Run("limit of 1 byte", func(t *testing.T) {
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			_, err := io.ReadAll(r.Body)
			return err
		})

		middleware := BodyLimit(BodyLimitConfig{Limit: 1})

		wrapped := middleware(handler)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("ab"))
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)
		assert.Error(t, err)
		assert.Equal(t, keratin.ErrRequestEntityTooLarge, err)
	})

	t.Run("very large limit", func(t *testing.T) {
		handler := keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		middleware := BodyLimit(BodyLimitConfig{Limit: 1 << 30})

		wrapped := middleware(handler)
		body := strings.Repeat("x", 10000)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		rec := httptest.NewRecorder()

		err := wrapped.ServeHTTP(rec, req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}
