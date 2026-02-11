package keratin

import (
	"errors"
	"iter"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRouter(t *testing.T) {
	tests := []struct {
		name         string
		errorHandler ErrorHandlerFunc
		wantNotNil   bool
	}{
		{
			name: "valid error handler creates router",
			errorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantNotNil: true,
		},
		{
			name: "error handler with custom implementation",
			errorHandler: (&mockErrorHandler{
				statusCode: http.StatusBadRequest,
			}).ServeHTTP,
			wantNotNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter(WithErrorHandler(tt.errorHandler))
			assert.NotNil(t, router)
			assert.NotNil(t, router.RouterGroup)
			assert.NotNil(t, router.patterns)
			assert.NotNil(t, router.errorHandler)
		})
	}
}

func TestRouter_Patterns(t *testing.T) {
	tests := []struct {
		name             string
		setupRouter      func(*Router)
		expectedCount    int
		expectedPatterns []string
	}{
		{
			name:             "empty router has no patterns",
			setupRouter:      func(r *Router) {},
			expectedCount:    0,
			expectedPatterns: []string{},
		},
		{
			name: "single route pattern",
			setupRouter: func(r *Router) {
				r.GET("/users", func(w http.ResponseWriter, req *http.Request) error {
					return nil
				})
			},
			expectedCount:    1,
			expectedPatterns: []string{"GET /users"},
		},
		{
			name: "multiple route patterns with different methods",
			setupRouter: func(r *Router) {
				r.GET("/users", func(w http.ResponseWriter, req *http.Request) error {
					return nil
				})
				r.POST("/users", func(w http.ResponseWriter, req *http.Request) error {
					return nil
				})
				r.DELETE("/users/{id}", func(w http.ResponseWriter, req *http.Request) error {
					return nil
				})
			},
			expectedCount:    3,
			expectedPatterns: []string{"GET /users", "POST /users", "DELETE /users/{id}"},
		},
		{
			name: "patterns with nested groups",
			setupRouter: func(r *Router) {
				api := r.Group("/api")
				v1 := api.Group("/v1")
				v1.GET("/users", func(w http.ResponseWriter, req *http.Request) error {
					return nil
				})
				v1.GET("/posts", func(w http.ResponseWriter, req *http.Request) error {
					return nil
				})
			},
			expectedCount:    2,
			expectedPatterns: []string{"GET /api/v1/users", "GET /api/v1/posts"},
		},
		{
			name: "patterns with method-agnostic routes",
			setupRouter: func(r *Router) {
				r.Any("/health", func(w http.ResponseWriter, req *http.Request) error {
					return nil
				})
				r.GET("/info", func(w http.ResponseWriter, req *http.Request) error {
					return nil
				})
			},
			expectedCount:    2,
			expectedPatterns: []string{"/health", "GET /info"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter(WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
				w.WriteHeader(http.StatusInternalServerError)
			}))
			tt.setupRouter(router)

			handler := router.Build()
			require.NotNil(t, handler)

			patterns := collectPatterns(router.Patterns())
			assert.Len(t, patterns, tt.expectedCount)

			for _, expected := range tt.expectedPatterns {
				assert.Contains(t, patterns, expected)
			}
		})
	}
}

func TestRouter_PreFunc(t *testing.T) {
	tests := []struct {
		name            string
		middlewareFuncs []func(Handler) Handler
		expectedCount   int
	}{
		{
			name: "adds single middleware function",
			middlewareFuncs: []func(Handler) Handler{
				func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						w.Header().Set("X-Pre-1", "executed")
						return h.ServeHTTP(w, r)
					})
				},
			},
			expectedCount: 1,
		},
		{
			name: "adds multiple middleware functions",
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
			name:            "empty middleware funcs list",
			middlewareFuncs: []func(Handler) Handler{},
			expectedCount:   0,
		},
		{
			name: "nil middleware funcs should not be added",
			middlewareFuncs: []func(Handler) Handler{
				nil,
				func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						return h.ServeHTTP(w, r)
					})
				},
			},
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter(WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
				w.WriteHeader(http.StatusInternalServerError)
			}))

			router.PreFunc(tt.middlewareFuncs...)

			assert.Len(t, router.PreMiddlewares, tt.expectedCount)
		})
	}
}

func TestRouter_Pre(t *testing.T) {
	tests := []struct {
		name          string
		middlewares   []*Middleware
		expectedCount int
	}{
		{
			name: "adds single middleware",
			middlewares: []*Middleware{
				{
					ID:       "middleware-1",
					Priority: 0,
					Func: func(h Handler) Handler {
						return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
							w.Header().Set("X-Pre-1", "executed")
							return h.ServeHTTP(w, r)
						})
					},
				},
			},
			expectedCount: 1,
		},
		{
			name: "adds multiple Middlewares with different priorities",
			middlewares: []*Middleware{
				{
					ID:       "priority-10",
					Priority: 10,
					Func: func(h Handler) Handler {
						return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
							w.Header().Set("X-Priority-10", "executed")
							return h.ServeHTTP(w, r)
						})
					},
				},
				{
					ID:       "priority-5",
					Priority: 5,
					Func: func(h Handler) Handler {
						return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
							w.Header().Set("X-Priority-5", "executed")
							return h.ServeHTTP(w, r)
						})
					},
				},
				{
					ID:       "priority-0",
					Priority: 0,
					Func: func(h Handler) Handler {
						return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
							w.Header().Set("X-Priority-0", "executed")
							return h.ServeHTTP(w, r)
						})
					},
				},
			},
			expectedCount: 3,
		},
		{
			name:          "empty Middlewares list",
			middlewares:   []*Middleware{},
			expectedCount: 0,
		},
		{
			name: "Middlewares with nil funcs",
			middlewares: []*Middleware{
				{
					ID:       "nil-func",
					Priority: 0,
					Func:     nil,
				},
			},
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter(WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
				w.WriteHeader(http.StatusInternalServerError)
			}))

			router.Pre(tt.middlewares...)

			assert.Len(t, router.PreMiddlewares, tt.expectedCount)
		})
	}
}

func TestRouter_PreAndPreFunc_Combined(t *testing.T) {
	router := NewRouter(WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	router.PreFunc(func(h Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Set("X-PreFunc-1", "executed")
			return h.ServeHTTP(w, r)
		})
	})

	router.Pre(&Middleware{
		ID:       "named-middleware",
		Priority: 5,
		Func: func(h Handler) Handler {
			return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
				w.Header().Set("X-Pre-1", "executed")
				return h.ServeHTTP(w, r)
			})
		},
	})

	router.PreFunc(func(h Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Set("X-PreFunc-2", "executed")
			return h.ServeHTTP(w, r)
		})
	})

	assert.Len(t, router.PreMiddlewares, 3)
}

func TestRouter_Build(t *testing.T) {
	tests := []struct {
		name        string
		setupRouter func(*Router)
	}{
		{
			name:        "empty router builds successfully",
			setupRouter: func(r *Router) {},
		},
		{
			name: "router with single route builds successfully",
			setupRouter: func(r *Router) {
				r.GET("/health", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusOK)
					return nil
				})
			},
		},
		{
			name: "router with multiple routes builds successfully",
			setupRouter: func(r *Router) {
				r.GET("/users", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusOK)
					return nil
				})
				r.POST("/users", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusCreated)
					return nil
				})
				r.DELETE("/users/{id}", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusNoContent)
					return nil
				})
			},
		},
		{
			name: "router with nested groups builds successfully",
			setupRouter: func(r *Router) {
				api := r.Group("/api")
				v1 := api.Group("/v1")
				v1.GET("/users", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusOK)
					return nil
				})
				v1.GET("/posts", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusOK)
					return nil
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter(WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
				w.WriteHeader(http.StatusInternalServerError)
			}))
			tt.setupRouter(router)

			handler := router.Build()

			assert.NotNil(t, handler)
		})
	}
}

func TestRouter_BuildWithMux(t *testing.T) {
	tests := []struct {
		name        string
		setupRouter func(*Router)
		setupMux    func() *http.ServeMux
	}{
		{
			name: "builds with new serve mux",
			setupRouter: func(r *Router) {
				r.GET("/test", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusOK)
					return nil
				})
			},
			setupMux: func() *http.ServeMux {
				return http.NewServeMux()
			},
		},
		{
			name: "builds with pre-configured serve mux",
			setupRouter: func(r *Router) {
				r.GET("/router-route", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusOK)
					return nil
				})
			},
			setupMux: func() *http.ServeMux {
				mux := http.NewServeMux()
				mux.HandleFunc("/existing-route", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				})
				return mux
			},
		},
		{
			name:        "empty router with custom mux",
			setupRouter: func(r *Router) {},
			setupMux: func() *http.ServeMux {
				return http.NewServeMux()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter(WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
				w.WriteHeader(http.StatusInternalServerError)
			}))
			tt.setupRouter(router)
			mux := tt.setupMux()

			handler := router.BuildWithMux(mux)

			assert.NotNil(t, handler)
		})
	}
}

func TestRouter_RouteRegistrationAndExecution(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		setupRouter    func(*Router)
		requestPath    string
		requestMethod  string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:   "GET route execution",
			method: http.MethodGet,
			path:   "/users",
			setupRouter: func(r *Router) {
				r.GET("/users", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("users list"))
					return nil
				})
			},
			requestPath:    "/users",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusOK,
			expectedBody:   "users list",
		},
		{
			name:   "POST route execution",
			method: http.MethodPost,
			path:   "/users",
			setupRouter: func(r *Router) {
				r.POST("/users", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte("user created"))
					return nil
				})
			},
			requestPath:    "/users",
			requestMethod:  http.MethodPost,
			expectedStatus: http.StatusCreated,
			expectedBody:   "user created",
		},
		{
			name:   "DELETE route execution",
			method: http.MethodDelete,
			path:   "/users/{id}",
			setupRouter: func(r *Router) {
				r.DELETE("/users/{id}", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusNoContent)
					return nil
				})
			},
			requestPath:    "/users/123",
			requestMethod:  http.MethodDelete,
			expectedStatus: http.StatusNoContent,
			expectedBody:   "",
		},
		{
			name:   "PUT route execution",
			method: http.MethodPut,
			path:   "/users/{id}",
			setupRouter: func(r *Router) {
				r.PUT("/users/{id}", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("user updated"))
					return nil
				})
			},
			requestPath:    "/users/123",
			requestMethod:  http.MethodPut,
			expectedStatus: http.StatusOK,
			expectedBody:   "user updated",
		},
		{
			name:   "PATCH route execution",
			method: http.MethodPatch,
			path:   "/users/{id}",
			setupRouter: func(r *Router) {
				r.PATCH("/users/{id}", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("user patched"))
					return nil
				})
			},
			requestPath:    "/users/123",
			requestMethod:  http.MethodPatch,
			expectedStatus: http.StatusOK,
			expectedBody:   "user patched",
		},
		{
			name:   "method-agnostic route execution",
			method: "",
			path:   "/health",
			setupRouter: func(r *Router) {
				r.Any("/health", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("healthy"))
					return nil
				})
			},
			requestPath:    "/health",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusOK,
			expectedBody:   "healthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter(WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
				w.WriteHeader(http.StatusInternalServerError)
			}))
			tt.setupRouter(router)

			handler := router.Build()

			req := httptest.NewRequest(tt.requestMethod, tt.requestPath, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedBody, w.Body.String())
		})
	}
}

func TestRouter_NestedGroups(t *testing.T) {
	tests := []struct {
		name           string
		setupRouter    func(*Router)
		requestPath    string
		requestMethod  string
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "single nested group",
			setupRouter: func(r *Router) {
				api := r.Group("/api")
				api.GET("/users", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("api users"))
					return nil
				})
			},
			requestPath:    "/api/users",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusOK,
			expectedBody:   "api users",
		},
		{
			name: "double nested groups",
			setupRouter: func(r *Router) {
				api := r.Group("/api")
				v1 := api.Group("/v1")
				v1.GET("/users", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("api v1 users"))
					return nil
				})
			},
			requestPath:    "/api/v1/users",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusOK,
			expectedBody:   "api v1 users",
		},
		{
			name: "triple nested groups",
			setupRouter: func(r *Router) {
				api := r.Group("/api")
				v1 := api.Group("/v1")
				users := v1.Group("/users")
				users.GET("/{id}", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("api v1 user"))
					return nil
				})
			},
			requestPath:    "/api/v1/users/123",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusOK,
			expectedBody:   "api v1 user",
		},
		{
			name: "multiple routes in nested groups",
			setupRouter: func(r *Router) {
				api := r.Group("/api")
				v1 := api.Group("/v1")
				v1.GET("/users", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("users list"))
					return nil
				})
				v1.GET("/posts", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("posts list"))
					return nil
				})
			},
			requestPath:    "/api/v1/posts",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusOK,
			expectedBody:   "posts list",
		},
		{
			name: "sibling groups",
			setupRouter: func(r *Router) {
				api := r.Group("/api")
				v1 := api.Group("/v1")
				v2 := api.Group("/v2")
				v1.GET("/users", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("v1 users"))
					return nil
				})
				v2.GET("/users", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("v2 users"))
					return nil
				})
			},
			requestPath:    "/api/v2/users",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusOK,
			expectedBody:   "v2 users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter(WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
				w.WriteHeader(http.StatusInternalServerError)
			}))
			tt.setupRouter(router)

			handler := router.Build()

			req := httptest.NewRequest(tt.requestMethod, tt.requestPath, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedBody, w.Body.String())
		})
	}
}

func TestRouter_MiddlewareExecutionOrder(t *testing.T) {
	tests := []struct {
		name          string
		setupRouter   func(*Router)
		requestPath   string
		requestMethod string
		expectedOrder []string
	}{
		{
			name: "pre middleware executes before route middleware",
			setupRouter: func(r *Router) {
				r.PreFunc(func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						w.Header().Add("X-Order", "pre-1")
						return h.ServeHTTP(w, r)
					})
				})

				r.GET("/test", func(w http.ResponseWriter, r *http.Request) error {
					w.Header().Add("X-Order", "handler")
					w.WriteHeader(http.StatusOK)
					return nil
				}).UseFunc(func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						w.Header().Add("X-Order", "route-mw-1")
						return h.ServeHTTP(w, r)
					})
				})
			},
			requestPath:   "/test",
			requestMethod: http.MethodGet,
			expectedOrder: []string{"pre-1", "route-mw-1", "handler"},
		},
		{
			name: "multiple pre Middlewares execute in order",
			setupRouter: func(r *Router) {
				r.PreFunc(func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						w.Header().Add("X-Order", "pre-1")
						return h.ServeHTTP(w, r)
					})
				})
				r.PreFunc(func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						w.Header().Add("X-Order", "pre-2")
						return h.ServeHTTP(w, r)
					})
				})

				r.GET("/test", func(w http.ResponseWriter, r *http.Request) error {
					w.Header().Add("X-Order", "handler")
					w.WriteHeader(http.StatusOK)
					return nil
				})
			},
			requestPath:   "/test",
			requestMethod: http.MethodGet,
			expectedOrder: []string{"pre-1", "pre-2", "handler"},
		},
		{
			name: "group Middlewares execute before route Middlewares",
			setupRouter: func(r *Router) {
				r.UseFunc(func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						w.Header().Add("X-Order", "root-group")
						return h.ServeHTTP(w, r)
					})
				})

				api := r.Group("/api")
				api.UseFunc(func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						w.Header().Add("X-Order", "api-group")
						return h.ServeHTTP(w, r)
					})
				})

				api.GET("/test", func(w http.ResponseWriter, r *http.Request) error {
					w.Header().Add("X-Order", "handler")
					w.WriteHeader(http.StatusOK)
					return nil
				}).UseFunc(func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						w.Header().Add("X-Order", "route-mw")
						return h.ServeHTTP(w, r)
					})
				})
			},
			requestPath:   "/api/test",
			requestMethod: http.MethodGet,
			expectedOrder: []string{"root-group", "api-group", "route-mw", "handler"},
		},
		{
			name: "nested group Middlewares execute in order",
			setupRouter: func(r *Router) {
				r.UseFunc(func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						w.Header().Add("X-Order", "root")
						return h.ServeHTTP(w, r)
					})
				})

				api := r.Group("/api")
				api.UseFunc(func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						w.Header().Add("X-Order", "api")
						return h.ServeHTTP(w, r)
					})
				})

				v1 := api.Group("/v1")
				v1.UseFunc(func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						w.Header().Add("X-Order", "v1")
						return h.ServeHTTP(w, r)
					})
				})

				v1.GET("/test", func(w http.ResponseWriter, r *http.Request) error {
					w.Header().Add("X-Order", "handler")
					w.WriteHeader(http.StatusOK)
					return nil
				})
			},
			requestPath:   "/api/v1/test",
			requestMethod: http.MethodGet,
			expectedOrder: []string{"root", "api", "v1", "handler"},
		},
		{
			name: "all middleware layers combined",
			setupRouter: func(r *Router) {
				r.PreFunc(func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						w.Header().Add("X-Order", "pre-1")
						return h.ServeHTTP(w, r)
					})
				})

				r.UseFunc(func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						w.Header().Add("X-Order", "root-group")
						return h.ServeHTTP(w, r)
					})
				})

				api := r.Group("/api")
				api.UseFunc(func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						w.Header().Add("X-Order", "api-group")
						return h.ServeHTTP(w, r)
					})
				})

				api.GET("/test", func(w http.ResponseWriter, r *http.Request) error {
					w.Header().Add("X-Order", "handler")
					w.WriteHeader(http.StatusOK)
					return nil
				}).UseFunc(func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						w.Header().Add("X-Order", "route-mw")
						return h.ServeHTTP(w, r)
					})
				})
			},
			requestPath:   "/api/test",
			requestMethod: http.MethodGet,
			expectedOrder: []string{"pre-1", "root-group", "api-group", "route-mw", "handler"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter(WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
				w.WriteHeader(http.StatusInternalServerError)
			}))
			tt.setupRouter(router)

			handler := router.Build()

			req := httptest.NewRequest(tt.requestMethod, tt.requestPath, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			order := w.Header().Values("X-Order")
			assert.Equal(t, tt.expectedOrder, order)
		})
	}
}

func TestRouter_ErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		setupRouter    func(*Router)
		requestPath    string
		requestMethod  string
		expectedStatus int
	}{
		{
			name: "handler error triggers error handler",
			setupRouter: func(r *Router) {
				r.GET("/error", func(w http.ResponseWriter, req *http.Request) error {
					return errors.New("test error")
				})
			},
			requestPath:    "/error",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "middleware error triggers error handler",
			setupRouter: func(r *Router) {
				r.GET("/test", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusOK)
					return nil
				}).UseFunc(func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						return errors.New("middleware error")
					})
				})
			},
			requestPath:    "/test",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "group middleware error triggers error handler",
			setupRouter: func(r *Router) {
				r.UseFunc(func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						return errors.New("group middleware error")
					})
				})

				r.GET("/test", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusOK)
					return nil
				})
			},
			requestPath:    "/test",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "pre middleware error triggers error handler",
			setupRouter: func(r *Router) {
				r.PreFunc(func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						return errors.New("pre middleware error")
					})
				})

				r.GET("/test", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusOK)
					return nil
				})
			},
			requestPath:    "/test",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "no error returns 404 for unregistered route",
			setupRouter: func(r *Router) {
				r.GET("/registered", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusOK)
					return nil
				})
			},
			requestPath:    "/unregistered",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter(WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(err.Error()))
				}
			}))
			tt.setupRouter(router)

			handler := router.Build()

			req := httptest.NewRequest(tt.requestMethod, tt.requestPath, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestRouter_MethodNotAllowed(t *testing.T) {
	router := NewRouter(WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	router.GET("/test", func(w http.ResponseWriter, req *http.Request) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})

	handler := router.Build()

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestRouter_PriorityMiddlewareOrder(t *testing.T) {
	router := NewRouter(WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	router.Pre(
		&Middleware{
			ID:       "priority-10",
			Priority: 10,
			Func: func(h Handler) Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					w.Header().Add("X-Order", "priority-10")
					return h.ServeHTTP(w, r)
				})
			},
		},
		&Middleware{
			ID:       "priority-5",
			Priority: 5,
			Func: func(h Handler) Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					w.Header().Add("X-Order", "priority-5")
					return h.ServeHTTP(w, r)
				})
			},
		},
		&Middleware{
			ID:       "priority-0",
			Priority: 0,
			Func: func(h Handler) Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					w.Header().Add("X-Order", "priority-0")
					return h.ServeHTTP(w, r)
				})
			},
		},
	)

	router.GET("/test", func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Add("X-Order", "handler")
		w.WriteHeader(http.StatusOK)
		return nil
	})

	handler := router.Build()

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	order := w.Header().Values("X-Order")
	assert.Equal(t, []string{"priority-0", "priority-5", "priority-10", "handler"}, order)
}

type mockErrorHandler struct {
	statusCode int
}

func (m *mockErrorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, err error) {
	w.WriteHeader(m.statusCode)
}

func collectPatterns(seq iter.Seq[string]) []string {
	var patterns []string
	seq(func(pattern string) bool {
		patterns = append(patterns, pattern)
		return true
	})
	return patterns
}
