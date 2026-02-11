package keratin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMiddlewares_build(t *testing.T) {
	tests := []struct {
		name        string
		middlewares Middlewares
		setup       func() Handler
		validate    func(t *testing.T, result Handler, middlewares Middlewares)
	}{
		{
			name:        "empty Middlewares returns original handler",
			middlewares: Middlewares{},
			setup: func() Handler {
				calls := 0
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					calls++
					w.WriteHeader(http.StatusOK)
					return nil
				})
			},
			validate: func(t *testing.T, result Handler, middlewares Middlewares) {
				require.NotNil(t, result)
				w := httptest.NewRecorder()
				r := httptest.NewRequest(http.MethodGet, "/", nil)
				err := result.ServeHTTP(w, r)
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, w.Code)
			},
		},
		{
			name: "single middleware wraps handler",
			middlewares: Middlewares{
				&Middleware{
					ID:       "middleware-1",
					Priority: 0,
					Func: func(h Handler) Handler {
						return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
							w.Header().Set("X-Middleware", "1")
							return h.ServeHTTP(w, r)
						})
					},
				},
			},
			setup: func() Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					w.WriteHeader(http.StatusOK)
					return nil
				})
			},
			validate: func(t *testing.T, result Handler, middlewares Middlewares) {
				require.NotNil(t, result)
				w := httptest.NewRecorder()
				r := httptest.NewRequest(http.MethodGet, "/", nil)
				err := result.ServeHTTP(w, r)
				require.NoError(t, err)
				assert.Equal(t, "1", w.Header().Get("X-Middleware"))
				assert.Equal(t, http.StatusOK, w.Code)
			},
		},
		{
			name: "multiple Middlewares wrap in priority order",
			middlewares: Middlewares{
				&Middleware{
					ID:       "middleware-1",
					Priority: 1,
					Func: func(h Handler) Handler {
						return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
							w.Header().Add("X-Order", "1")
							return h.ServeHTTP(w, r)
						})
					},
				},
				&Middleware{
					ID:       "middleware-2",
					Priority: 0,
					Func: func(h Handler) Handler {
						return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
							w.Header().Add("X-Order", "2")
							return h.ServeHTTP(w, r)
						})
					},
				},
				&Middleware{
					ID:       "middleware-3",
					Priority: 2,
					Func: func(h Handler) Handler {
						return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
							w.Header().Add("X-Order", "3")
							return h.ServeHTTP(w, r)
						})
					},
				},
			},
			setup: func() Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					w.WriteHeader(http.StatusOK)
					return nil
				})
			},
			validate: func(t *testing.T, result Handler, middlewares Middlewares) {
				require.NotNil(t, result)
				w := httptest.NewRecorder()
				r := httptest.NewRequest(http.MethodGet, "/", nil)
				err := result.ServeHTTP(w, r)
				require.NoError(t, err)

				order := w.Header().Values("X-Order")
				assert.Equal(t, []string{"2", "1", "3"}, order)
			},
		},
		{
			name: "empty ID generates UUID",
			middlewares: Middlewares{
				&Middleware{
					ID:       "",
					Priority: 0,
					Func: func(h Handler) Handler {
						return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
							return h.ServeHTTP(w, r)
						})
					},
				},
			},
			setup: func() Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					w.WriteHeader(http.StatusOK)
					return nil
				})
			},
			validate: func(t *testing.T, result Handler, middlewares Middlewares) {
				require.NotNil(t, result)
				assert.NotEmpty(t, middlewares[0].ID)
			},
		},
		{
			name: "existing ID is preserved",
			middlewares: Middlewares{
				&Middleware{
					ID:       "custom-id-123",
					Priority: 0,
					Func: func(h Handler) Handler {
						return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
							return h.ServeHTTP(w, r)
						})
					},
				},
			},
			setup: func() Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					w.WriteHeader(http.StatusOK)
					return nil
				})
			},
			validate: func(t *testing.T, result Handler, middlewares Middlewares) {
				require.NotNil(t, result)
				assert.Equal(t, "custom-id-123", middlewares[0].ID)
			},
		},
		{
			name: "Middlewares with same priority maintain stable order",
			middlewares: Middlewares{
				&Middleware{
					ID:       "first",
					Priority: 0,
					Func: func(h Handler) Handler {
						return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
							w.Header().Add("X-Same-Priority", "first")
							return h.ServeHTTP(w, r)
						})
					},
				},
				&Middleware{
					ID:       "second",
					Priority: 0,
					Func: func(h Handler) Handler {
						return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
							w.Header().Add("X-Same-Priority", "second")
							return h.ServeHTTP(w, r)
						})
					},
				},
				&Middleware{
					ID:       "third",
					Priority: 0,
					Func: func(h Handler) Handler {
						return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
							w.Header().Add("X-Same-Priority", "third")
							return h.ServeHTTP(w, r)
						})
					},
				},
			},
			setup: func() Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					w.WriteHeader(http.StatusOK)
					return nil
				})
			},
			validate: func(t *testing.T, result Handler, middlewares Middlewares) {
				require.NotNil(t, result)
				w := httptest.NewRecorder()
				r := httptest.NewRequest(http.MethodGet, "/", nil)
				err := result.ServeHTTP(w, r)
				require.NoError(t, err)

				order := w.Header().Values("X-Same-Priority")
				assert.Equal(t, []string{"first", "second", "third"}, order)
			},
		},
		{
			name: "negative priority Middlewares execute first",
			middlewares: Middlewares{
				&Middleware{
					ID:       "zero",
					Priority: 0,
					Func: func(h Handler) Handler {
						return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
							w.Header().Add("X-Priority", "zero")
							return h.ServeHTTP(w, r)
						})
					},
				},
				&Middleware{
					ID:       "negative",
					Priority: -1,
					Func: func(h Handler) Handler {
						return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
							w.Header().Add("X-Priority", "negative")
							return h.ServeHTTP(w, r)
						})
					},
				},
				&Middleware{
					ID:       "positive",
					Priority: 1,
					Func: func(h Handler) Handler {
						return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
							w.Header().Add("X-Priority", "positive")
							return h.ServeHTTP(w, r)
						})
					},
				},
			},
			setup: func() Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					w.WriteHeader(http.StatusOK)
					return nil
				})
			},
			validate: func(t *testing.T, result Handler, middlewares Middlewares) {
				require.NotNil(t, result)
				w := httptest.NewRecorder()
				r := httptest.NewRequest(http.MethodGet, "/", nil)
				err := result.ServeHTTP(w, r)
				require.NoError(t, err)

				order := w.Header().Values("X-Priority")
				assert.Equal(t, []string{"negative", "zero", "positive"}, order)
			},
		},
		{
			name: "middleware can short-circuit request",
			middlewares: Middlewares{
				&Middleware{
					ID:       "short-circuit",
					Priority: 0,
					Func: func(h Handler) Handler {
						return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
							w.WriteHeader(http.StatusForbidden)
							_, _ = w.Write([]byte("Access denied"))
							return nil
						})
					},
				},
			},
			setup: func() Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("Success"))
					return nil
				})
			},
			validate: func(t *testing.T, result Handler, middlewares Middlewares) {
				require.NotNil(t, result)
				w := httptest.NewRecorder()
				r := httptest.NewRequest(http.MethodGet, "/", nil)
				err := result.ServeHTTP(w, r)
				require.NoError(t, err)
				assert.Equal(t, http.StatusForbidden, w.Code)
				assert.Equal(t, "Access denied", w.Body.String())
			},
		},
		{
			name: "middleware handles and passes errors",
			middlewares: Middlewares{
				&Middleware{
					ID:       "error-handler",
					Priority: 0,
					Func: func(h Handler) Handler {
						return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
							err := h.ServeHTTP(w, r)
							if err != nil {
								w.WriteHeader(http.StatusInternalServerError)
								_, _ = w.Write([]byte("Error: " + err.Error()))
							}
							return nil
						})
					},
				},
			},
			setup: func() Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					return assert.AnError
				})
			},
			validate: func(t *testing.T, result Handler, middlewares Middlewares) {
				require.NotNil(t, result)
				w := httptest.NewRecorder()
				r := httptest.NewRequest(http.MethodGet, "/", nil)
				err := result.ServeHTTP(w, r)
				require.NoError(t, err)
				assert.Equal(t, http.StatusInternalServerError, w.Code)
				assert.True(t, strings.Contains(w.Body.String(), "Error:"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := tt.setup()
			result := tt.middlewares.build(handler)
			tt.validate(t, result, tt.middlewares)
		})
	}

	t.Run("multiple Middlewares with empty IDs generate unique UUIDs", func(t *testing.T) {
		middlewares := Middlewares{
			&Middleware{
				ID:       "",
				Priority: 0,
				Func: func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						return h.ServeHTTP(w, r)
					})
				},
			},
			&Middleware{
				ID:       "",
				Priority: 1,
				Func: func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						return h.ServeHTTP(w, r)
					})
				},
			},
			&Middleware{
				ID:       "",
				Priority: 2,
				Func: func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						return h.ServeHTTP(w, r)
					})
				},
			},
		}

		handler := HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})

		result := middlewares.build(handler)
		require.NotNil(t, result)

		assert.NotEmpty(t, middlewares[0].ID)
		assert.NotEmpty(t, middlewares[1].ID)
		assert.NotEmpty(t, middlewares[2].ID)

		assert.NotEqual(t, middlewares[0].ID, middlewares[1].ID)
		assert.NotEqual(t, middlewares[1].ID, middlewares[2].ID)
		assert.NotEqual(t, middlewares[0].ID, middlewares[2].ID)
	})
}

func TestMiddlewares_build_ExecutionOrder(t *testing.T) {
	var executionOrder []string

	middlewares := Middlewares{
		&Middleware{
			ID:       "mw-1",
			Priority: 10,
			Func: func(h Handler) Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					executionOrder = append(executionOrder, "mw-1-before")
					err := h.ServeHTTP(w, r)
					executionOrder = append(executionOrder, "mw-1-after")
					return err
				})
			},
		},
		&Middleware{
			ID:       "mw-2",
			Priority: 5,
			Func: func(h Handler) Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					executionOrder = append(executionOrder, "mw-2-before")
					err := h.ServeHTTP(w, r)
					executionOrder = append(executionOrder, "mw-2-after")
					return err
				})
			},
		},
		&Middleware{
			ID:       "mw-3",
			Priority: 0,
			Func: func(h Handler) Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					executionOrder = append(executionOrder, "mw-3-before")
					err := h.ServeHTTP(w, r)
					executionOrder = append(executionOrder, "mw-3-after")
					return err
				})
			},
		},
	}

	handler := HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		executionOrder = append(executionOrder, "handler")
		w.WriteHeader(http.StatusOK)
		return nil
	})

	result := middlewares.build(handler)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	err := result.ServeHTTP(w, r)
	require.NoError(t, err)

	expected := []string{
		"mw-3-before",
		"mw-2-before",
		"mw-1-before",
		"handler",
		"mw-1-after",
		"mw-2-after",
		"mw-3-after",
	}
	assert.Equal(t, expected, executionOrder)
}
