package keratin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoute_UseFunc(t *testing.T) {
	tests := []struct {
		name               string
		initialMiddlewares Middlewares[Handler]
		middlewareFuncs    []func(Handler) Handler
		expectedCount      int
	}{
		{
			name:               "adds single middleware",
			initialMiddlewares: Middlewares[Handler]{},
			middlewareFuncs: []func(Handler) Handler{
				func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						w.Header().Set("X-Middleware", "1")
						return h.ServeHTTP(w, r)
					})
				},
			},
			expectedCount: 1,
		},
		{
			name:               "adds multiple Middlewares[Handler]",
			initialMiddlewares: Middlewares[Handler]{},
			middlewareFuncs: []func(Handler) Handler{
				func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						w.Header().Add("X-Order", "1")
						return h.ServeHTTP(w, r)
					})
				},
				func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						w.Header().Add("X-Order", "2")
						return h.ServeHTTP(w, r)
					})
				},
				func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						w.Header().Add("X-Order", "3")
						return h.ServeHTTP(w, r)
					})
				},
			},
			expectedCount: 3,
		},
		{
			name: "appends to existing Middlewares[Handler]",
			initialMiddlewares: Middlewares[Handler]{
				&Middleware[Handler]{
					ID:       "existing-1",
					Priority: 0,
					Func: func(h Handler) Handler {
						return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
							w.Header().Add("X-Existing", "true")
							return h.ServeHTTP(w, r)
						})
					},
				},
			},
			middlewareFuncs: []func(Handler) Handler{
				func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						w.Header().Add("X-New", "true")
						return h.ServeHTTP(w, r)
					})
				},
			},
			expectedCount: 2,
		},
		{
			name:               "empty middleware funcs list",
			initialMiddlewares: Middlewares[Handler]{},
			middlewareFuncs:    []func(Handler) Handler{},
			expectedCount:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := &Route{
				Method: http.MethodGet,
				Path:   "/test",
				Handler: HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					w.WriteHeader(http.StatusOK)
					return nil
				}),
				Middlewares: tt.initialMiddlewares,
			}

			result := route.UseFunc(tt.middlewareFuncs...)

			require.NotNil(t, result)
			assert.Same(t, route, result, "UseFunc should return the same route instance")
			assert.Len(t, route.Middlewares, tt.expectedCount)
		})
	}
}

func TestRoute_UseFunc_Execution(t *testing.T) {
	route := &Route{
		Method: http.MethodGet,
		Path:   "/test",
		Handler: HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}),
		Middlewares: Middlewares[Handler]{},
	}

	route.UseFunc(
		func(h Handler) Handler {
			return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
				w.Header().Set("X-First", "1")
				return h.ServeHTTP(w, r)
			})
		},
		func(h Handler) Handler {
			return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
				w.Header().Set("X-Second", "2")
				return h.ServeHTTP(w, r)
			})
		},
	)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	handler := route.Middlewares.build(route.Handler)
	err := handler.ServeHTTP(w, r)
	require.NoError(t, err)

	assert.Equal(t, "1", w.Header().Get("X-First"))
	assert.Equal(t, "2", w.Header().Get("X-Second"))
}

func TestRoute_Use(t *testing.T) {
	tests := []struct {
		name               string
		initialMiddlewares Middlewares[Handler]
		middlewares        []*Middleware[Handler]
		expectedCount      int
	}{
		{
			name:               "adds single middleware",
			initialMiddlewares: Middlewares[Handler]{},
			middlewares: []*Middleware[Handler]{
				{
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
			expectedCount: 1,
		},
		{
			name:               "adds multiple Middlewares[Handler]",
			initialMiddlewares: Middlewares[Handler]{},
			middlewares: []*Middleware[Handler]{
				{
					ID:       "middleware-1",
					Priority: 10,
					Func: func(h Handler) Handler {
						return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
							return h.ServeHTTP(w, r)
						})
					},
				},
				{
					ID:       "middleware-2",
					Priority: 5,
					Func: func(h Handler) Handler {
						return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
							return h.ServeHTTP(w, r)
						})
					},
				},
				{
					ID:       "middleware-3",
					Priority: 0,
					Func: func(h Handler) Handler {
						return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
							return h.ServeHTTP(w, r)
						})
					},
				},
			},
			expectedCount: 3,
		},
		{
			name: "appends to existing Middlewares[Handler]",
			initialMiddlewares: Middlewares[Handler]{
				&Middleware[Handler]{
					ID:       "existing-1",
					Priority: 0,
					Func: func(h Handler) Handler {
						return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
							return h.ServeHTTP(w, r)
						})
					},
				},
			},
			middlewares: []*Middleware[Handler]{
				{
					ID:       "new-1",
					Priority: 1,
					Func: func(h Handler) Handler {
						return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
							return h.ServeHTTP(w, r)
						})
					},
				},
			},
			expectedCount: 2,
		},
		{
			name:               "empty Middlewares[Handler] list",
			initialMiddlewares: Middlewares[Handler]{},
			middlewares:        []*Middleware[Handler]{},
			expectedCount:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := &Route{
				Method: http.MethodGet,
				Path:   "/test",
				Handler: HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					w.WriteHeader(http.StatusOK)
					return nil
				}),
				Middlewares: tt.initialMiddlewares,
			}

			result := route.Use(tt.middlewares...)

			require.NotNil(t, result)
			assert.Same(t, route, result, "Use should return the same route instance")
			assert.Len(t, route.Middlewares, tt.expectedCount)
		})
	}
}

func TestRoute_Use_Execution(t *testing.T) {
	route := &Route{
		Method: http.MethodGet,
		Path:   "/test",
		Handler: HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}),
		Middlewares: Middlewares[Handler]{},
	}

	route.Use(
		&Middleware[Handler]{
			ID:       "priority-10",
			Priority: 10,
			Func: func(h Handler) Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					w.Header().Set("X-Priority-10", "executed")
					return h.ServeHTTP(w, r)
				})
			},
		},
		&Middleware[Handler]{
			ID:       "priority-5",
			Priority: 5,
			Func: func(h Handler) Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					w.Header().Set("X-Priority-5", "executed")
					return h.ServeHTTP(w, r)
				})
			},
		},
		&Middleware[Handler]{
			ID:       "priority-0",
			Priority: 0,
			Func: func(h Handler) Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					w.Header().Set("X-Priority-0", "executed")
					return h.ServeHTTP(w, r)
				})
			},
		},
	)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	handler := route.Middlewares.build(route.Handler)
	err := handler.ServeHTTP(w, r)
	require.NoError(t, err)

	assert.Equal(t, "executed", w.Header().Get("X-Priority-10"))
	assert.Equal(t, "executed", w.Header().Get("X-Priority-5"))
	assert.Equal(t, "executed", w.Header().Get("X-Priority-0"))
}

func TestRoute_UseFunc_Chaining(t *testing.T) {
	route := &Route{
		Method: http.MethodGet,
		Path:   "/test",
		Handler: HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}),
		Middlewares: Middlewares[Handler]{},
	}

	route.UseFunc(func(h Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Set("X-Chain-1", "1")
			return h.ServeHTTP(w, r)
		})
	}).UseFunc(func(h Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Set("X-Chain-2", "2")
			return h.ServeHTTP(w, r)
		})
	}).UseFunc(func(h Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Set("X-Chain-3", "3")
			return h.ServeHTTP(w, r)
		})
	})

	assert.Len(t, route.Middlewares, 3)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	handler := route.Middlewares.build(route.Handler)
	err := handler.ServeHTTP(w, r)
	require.NoError(t, err)

	assert.Equal(t, "1", w.Header().Get("X-Chain-1"))
	assert.Equal(t, "2", w.Header().Get("X-Chain-2"))
	assert.Equal(t, "3", w.Header().Get("X-Chain-3"))
}

func TestRoute_Use_Chaining(t *testing.T) {
	route := &Route{
		Method: http.MethodGet,
		Path:   "/test",
		Handler: HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}),
		Middlewares: Middlewares[Handler]{},
	}

	route.Use(
		&Middleware[Handler]{
			ID:       "chain-1",
			Priority: 10,
			Func: func(h Handler) Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					w.Header().Set("X-Use-Chain-1", "1")
					return h.ServeHTTP(w, r)
				})
			},
		},
	).Use(
		&Middleware[Handler]{
			ID:       "chain-2",
			Priority: 5,
			Func: func(h Handler) Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					w.Header().Set("X-Use-Chain-2", "2")
					return h.ServeHTTP(w, r)
				})
			},
		},
	)

	assert.Len(t, route.Middlewares, 2)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	handler := route.Middlewares.build(route.Handler)
	err := handler.ServeHTTP(w, r)
	require.NoError(t, err)

	assert.Equal(t, "1", w.Header().Get("X-Use-Chain-1"))
	assert.Equal(t, "2", w.Header().Get("X-Use-Chain-2"))
}

func TestRoute_UseFunc_Use_Mixed(t *testing.T) {
	route := &Route{
		Method: http.MethodGet,
		Path:   "/test",
		Handler: HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}),
		Middlewares: Middlewares[Handler]{},
	}

	route.UseFunc(func(h Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Set("X-UseFunc", "1")
			return h.ServeHTTP(w, r)
		})
	}).Use(
		&Middleware[Handler]{
			ID:       "use-method",
			Priority: 5,
			Func: func(h Handler) Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					w.Header().Set("X-Use", "1")
					return h.ServeHTTP(w, r)
				})
			},
		},
	).UseFunc(func(h Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Set("X-UseFunc-2", "1")
			return h.ServeHTTP(w, r)
		})
	})

	assert.Len(t, route.Middlewares, 3)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	handler := route.Middlewares.build(route.Handler)
	err := handler.ServeHTTP(w, r)
	require.NoError(t, err)

	assert.Equal(t, "1", w.Header().Get("X-UseFunc"))
	assert.Equal(t, "1", w.Header().Get("X-Use"))
	assert.Equal(t, "1", w.Header().Get("X-UseFunc-2"))
}

func TestRoute_UseFunc_NoIDs(t *testing.T) {
	route := &Route{
		Method: http.MethodGet,
		Path:   "/test",
		Handler: HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}),
		Middlewares: Middlewares[Handler]{},
	}

	route.UseFunc(func(h Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			return h.ServeHTTP(w, r)
		})
	}, func(h Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			return h.ServeHTTP(w, r)
		})
	})

	assert.Len(t, route.Middlewares, 2)
	assert.Empty(t, route.Middlewares[0].ID)
	assert.Empty(t, route.Middlewares[1].ID)
}

func TestRoute_Use_NamedMiddlewares(t *testing.T) {
	route := &Route{
		Method: http.MethodGet,
		Path:   "/test",
		Handler: HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}),
		Middlewares: Middlewares[Handler]{},
	}

	route.Use(
		&Middleware[Handler]{
			ID:       "auth-middleware",
			Priority: 0,
			Func: func(h Handler) Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					return h.ServeHTTP(w, r)
				})
			},
		},
		&Middleware[Handler]{
			ID:       "logging-middleware",
			Priority: 1,
			Func: func(h Handler) Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					return h.ServeHTTP(w, r)
				})
			},
		},
	)

	assert.Len(t, route.Middlewares, 2)
	assert.Equal(t, "auth-middleware", route.Middlewares[0].ID)
	assert.Equal(t, "logging-middleware", route.Middlewares[1].ID)
}

func TestRoute_MiddlewareExecutionOrder(t *testing.T) {
	var executionOrder []string

	route := &Route{
		Method: http.MethodGet,
		Path:   "/test",
		Handler: HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			executionOrder = append(executionOrder, "handler")
			w.WriteHeader(http.StatusOK)
			return nil
		}),
		Middlewares: Middlewares[Handler]{},
	}

	route.Use(
		&Middleware[Handler]{
			ID:       "priority-10",
			Priority: 10,
			Func: func(h Handler) Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					executionOrder = append(executionOrder, "priority-10-before")
					err := h.ServeHTTP(w, r)
					executionOrder = append(executionOrder, "priority-10-after")
					return err
				})
			},
		},
	).UseFunc(func(h Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			executionOrder = append(executionOrder, "usefunc-before")
			err := h.ServeHTTP(w, r)
			executionOrder = append(executionOrder, "usefunc-after")
			return err
		})
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	handler := route.Middlewares.build(route.Handler)
	err := handler.ServeHTTP(w, r)
	require.NoError(t, err)

	expected := []string{
		"usefunc-before",
		"priority-10-before",
		"handler",
		"priority-10-after",
		"usefunc-after",
	}
	assert.Equal(t, expected, executionOrder)
}

func TestRoute_UseFunc_NilHandler(t *testing.T) {
	route := &Route{
		Method:      http.MethodGet,
		Path:        "/test",
		Handler:     nil,
		Middlewares: Middlewares[Handler]{},
	}

	route.UseFunc(func(h Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		})
	})

	assert.Len(t, route.Middlewares, 1)
}

func TestRoute_Use_NilMiddleware(t *testing.T) {
	route := &Route{
		Method: http.MethodGet,
		Path:   "/test",
		Handler: HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}),
		Middlewares: Middlewares[Handler]{},
	}

	route.Use(
		&Middleware[Handler]{
			ID:       "test",
			Priority: 0,
			Func:     nil,
		},
	)

	assert.Len(t, route.Middlewares, 1)
	assert.Nil(t, route.Middlewares[0].Func)
}
