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
		middlewares   []*Middleware[Handler]
		expectedCount int
	}{
		{
			name: "adds single middleware",
			middlewares: []*Middleware[Handler]{
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
			middlewares: []*Middleware[Handler]{
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
			middlewares:   []*Middleware[Handler]{},
			expectedCount: 0,
		},
		{
			name: "Middlewares with nil funcs",
			middlewares: []*Middleware[Handler]{
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

	router.Pre(&Middleware[Handler]{
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
		&Middleware[Handler]{
			ID:       "priority-10",
			Priority: 10,
			Func: func(h Handler) Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					w.Header().Add("X-Order", "priority-10")
					return h.ServeHTTP(w, r)
				})
			},
		},
		&Middleware[Handler]{
			ID:       "priority-5",
			Priority: 5,
			Func: func(h Handler) Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					w.Header().Add("X-Order", "priority-5")
					return h.ServeHTTP(w, r)
				})
			},
		},
		&Middleware[Handler]{
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

func TestRouter_WithIPExtractor(t *testing.T) {
	tests := []struct {
		name        string
		ipExtractor IPExtractor
		wantNil     bool
	}{
		{
			name: "valid IP extractor is set",
			ipExtractor: func(r *http.Request) string {
				return "custom-ip"
			},
			wantNil: false,
		},
		{
			name:        "nil IP extractor keeps default",
			ipExtractor: nil,
			wantNil:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter(WithIPExtractor(tt.ipExtractor))

			if tt.ipExtractor != nil {
				assert.NotNil(t, router.ipExtractor)
			}
		})
	}
}

func TestRouter_WithResponseInterceptor(t *testing.T) {
	tests := []struct {
		name        string
		interceptor func(w http.ResponseWriter) (http.ResponseWriter, func())
		wantNotNil  bool
	}{
		{
			name: "valid response interceptor is added",
			interceptor: func(w http.ResponseWriter) (http.ResponseWriter, func()) {
				return w, func() {}
			},
			wantNotNil: true,
		},
		{
			name:        "nil interceptor is ignored",
			interceptor: nil,
			wantNotNil:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter()

			routerWithInterceptor := NewRouter(WithResponseInterceptor(tt.interceptor))

			if tt.interceptor != nil {
				assert.Greater(t, len(routerWithInterceptor.rwInterceptors), len(router.rwInterceptors))
			} else {
				assert.Equal(t, len(routerWithInterceptor.rwInterceptors), len(router.rwInterceptors))
			}
		})
	}
}

func TestRouter_WithRequestInterceptor(t *testing.T) {
	tests := []struct {
		name        string
		interceptor func(r *http.Request) (*http.Request, func())
		wantNotNil  bool
	}{
		{
			name: "valid request interceptor is added",
			interceptor: func(r *http.Request) (*http.Request, func()) {
				return r, func() {}
			},
			wantNotNil: true,
		},
		{
			name:        "nil interceptor is ignored",
			interceptor: nil,
			wantNotNil:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter()

			routerWithInterceptor := NewRouter(WithRequestInterceptor(tt.interceptor))

			if tt.interceptor != nil {
				assert.Greater(t, len(routerWithInterceptor.reqInterceptors), len(router.reqInterceptors))
			} else {
				assert.Equal(t, len(routerWithInterceptor.reqInterceptors), len(router.reqInterceptors))
			}
		})
	}
}

func TestRouter_PreHTTPFunc(t *testing.T) {
	tests := []struct {
		name            string
		middlewareFuncs []func(next http.Handler) http.Handler
		expectedCount   int
	}{
		{
			name: "adds single HTTP middleware function",
			middlewareFuncs: []func(next http.Handler) http.Handler{
				func(h http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("X-Pre-HTTP-1", "executed")
						h.ServeHTTP(w, r)
					})
				},
			},
			expectedCount: 1,
		},
		{
			name: "adds multiple HTTP middleware functions",
			middlewareFuncs: []func(next http.Handler) http.Handler{
				func(h http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Add("X-HTTP-Order", "1")
						h.ServeHTTP(w, r)
					})
				},
				func(h http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Add("X-HTTP-Order", "2")
						h.ServeHTTP(w, r)
					})
				},
			},
			expectedCount: 2,
		},
		{
			name:            "empty middleware funcs list",
			middlewareFuncs: []func(next http.Handler) http.Handler{},
			expectedCount:   0,
		},
		{
			name: "nil middleware funcs should not be added",
			middlewareFuncs: []func(next http.Handler) http.Handler{
				nil,
				func(h http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						h.ServeHTTP(w, r)
					})
				},
			},
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter()

			router.PreHTTPFunc(tt.middlewareFuncs...)

			assert.Len(t, router.HTTPMiddlewares, tt.expectedCount)
		})
	}
}

func TestRouter_PreHTTP(t *testing.T) {
	tests := []struct {
		name          string
		middlewares   []*Middleware[http.Handler]
		expectedCount int
	}{
		{
			name: "adds single HTTP middleware",
			middlewares: []*Middleware[http.Handler]{
				{
					ID:       "http-middleware-1",
					Priority: 0,
					Func: func(h http.Handler) http.Handler {
						return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							w.Header().Set("X-Pre-HTTP-1", "executed")
							h.ServeHTTP(w, r)
						})
					},
				},
			},
			expectedCount: 1,
		},
		{
			name: "adds multiple HTTP Middlewares with different priorities",
			middlewares: []*Middleware[http.Handler]{
				{
					ID:       "priority-10",
					Priority: 10,
					Func: func(h http.Handler) http.Handler {
						return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							w.Header().Set("X-HTTP-Priority-10", "executed")
							h.ServeHTTP(w, r)
						})
					},
				},
				{
					ID:       "priority-5",
					Priority: 5,
					Func: func(h http.Handler) http.Handler {
						return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							w.Header().Set("X-HTTP-Priority-5", "executed")
							h.ServeHTTP(w, r)
						})
					},
				},
			},
			expectedCount: 2,
		},
		{
			name:          "empty Middleware[http.Handler]s list",
			middlewares:   []*Middleware[http.Handler]{},
			expectedCount: 0,
		},
		{
			name: "Middleware[http.Handler]s with nil funcs",
			middlewares: []*Middleware[http.Handler]{
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
			router := NewRouter()

			router.PreHTTP(tt.middlewares...)

			assert.Len(t, router.HTTPMiddlewares, tt.expectedCount)
		})
	}
}

func TestRouter_PreHTTPAndPreHTTPFunc_Combined(t *testing.T) {
	router := NewRouter()

	router.PreHTTPFunc(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-PreHTTPFunc-1", "executed")
			h.ServeHTTP(w, r)
		})
	})

	router.PreHTTP(&Middleware[http.Handler]{
		ID:       "named-http-middleware",
		Priority: 5,
		Func: func(h http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-PreHTTP-1", "executed")
				h.ServeHTTP(w, r)
			})
		},
	})

	router.PreHTTPFunc(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-PreHTTPFunc-2", "executed")
			h.ServeHTTP(w, r)
		})
	})

	assert.Len(t, router.HTTPMiddlewares, 3)
}

func TestRouter_HTTPMiddlewareExecution(t *testing.T) {
	tests := []struct {
		name            string
		setupRouter     func(*Router)
		requestPath     string
		requestMethod   string
		expectedStatus  int
		expectedHeaders map[string][]string
	}{
		{
			name: "HTTP middleware executes before handler",
			setupRouter: func(r *Router) {
				r.PreHTTPFunc(func(h http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						w.Header().Set("X-HTTP-Middleware", "executed")
						h.ServeHTTP(w, req)
					})
				})

				r.GET("/test", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusOK)
					return nil
				})
			},
			requestPath:    "/test",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusOK,
			expectedHeaders: map[string][]string{
				"X-HTTP-Middleware": {"executed"},
			},
		},
		{
			name: "multiple HTTP middlewares execute in order",
			setupRouter: func(r *Router) {
				r.PreHTTPFunc(func(h http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						w.Header().Add("X-HTTP-Order", "1")
						h.ServeHTTP(w, req)
					})
				})

				r.PreHTTP(&Middleware[http.Handler]{
					ID:       "http-mw-2",
					Priority: 5,
					Func: func(h http.Handler) http.Handler {
						return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
							w.Header().Add("X-HTTP-Order", "2")
							h.ServeHTTP(w, req)
						})
					},
				})

				r.GET("/test", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusOK)
					return nil
				})
			},
			requestPath:    "/test",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusOK,
			expectedHeaders: map[string][]string{
				"X-HTTP-Order": {"1", "2"},
			},
		},
		{
			name: "HTTP middleware can short-circuit",
			setupRouter: func(r *Router) {
				r.PreHTTP(&Middleware[http.Handler]{
					ID:       "short-circuit",
					Priority: 0,
					Func: func(h http.Handler) http.Handler {
						return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
							w.WriteHeader(http.StatusForbidden)
							_, _ = w.Write([]byte("Access denied"))
						})
					},
				})

				r.GET("/test", func(w http.ResponseWriter, req *http.Request) error {
					w.WriteHeader(http.StatusOK)
					return nil
				})
			},
			requestPath:    "/test",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "HTTP middleware handles handler errors",
			setupRouter: func(r *Router) {
				r.PreHTTP(&Middleware[http.Handler]{
					ID:       "error-recovery",
					Priority: 0,
					Func: func(h http.Handler) http.Handler {
						return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
							defer func() {
								if rec := recover(); rec != nil {
									w.WriteHeader(http.StatusInternalServerError)
									_, _ = w.Write([]byte("Panic recovered"))
								}
							}()
							h.ServeHTTP(w, req)
						})
					},
				})

				r.GET("/test", func(w http.ResponseWriter, req *http.Request) error {
					return errors.New("handler error")
				})
			},
			requestPath:    "/test",
			requestMethod:  http.MethodGet,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter(WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(err.Error()))
			}))
			tt.setupRouter(router)

			handler := router.Build()

			req := httptest.NewRequest(tt.requestMethod, tt.requestPath, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			for key, expectedValues := range tt.expectedHeaders {
				assert.Equal(t, expectedValues, w.Header().Values(key))
			}
		})
	}
}

func TestRouter_CustomIPExtractor(t *testing.T) {
	tests := []struct {
		name        string
		ipExtractor IPExtractor
		request     *http.Request
		expectedIP  string
	}{
		{
			name: "custom IP extractor returns custom value",
			ipExtractor: func(r *http.Request) string {
				return "custom-ip-123"
			},
			request:    httptest.NewRequest(http.MethodGet, "/test", nil),
			expectedIP: "custom-ip-123",
		},
		{
			name: "custom IP extractor extracts from header",
			ipExtractor: func(r *http.Request) string {
				return r.Header.Get("X-Forwarded-For")
			},
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				req.Header.Set("X-Forwarded-For", "10.0.0.1")
				return req
			}(),
			expectedIP: "10.0.0.1",
		},
		{
			name:        "default IP extractor uses RemoteIP",
			ipExtractor: nil,
			request:     httptest.NewRequest(http.MethodGet, "/test", nil),
			expectedIP:  "192.0.2.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var extractedIP string

			router := NewRouter(WithIPExtractor(tt.ipExtractor))
			router.GET("/test", func(w http.ResponseWriter, req *http.Request) error {
				extractedIP = FromContext(req.Context()).RealIP()
				w.WriteHeader(http.StatusOK)
				return nil
			})

			handler := router.Build()

			handler.ServeHTTP(httptest.NewRecorder(), tt.request)

			assert.Equal(t, tt.expectedIP, extractedIP)
		})
	}
}

func TestRouter_ResponseInterceptor(t *testing.T) {
	tests := []struct {
		name        string
		interceptor func(w http.ResponseWriter) (http.ResponseWriter, func())
		expectValue string
	}{
		{
			name: "response interceptor wraps response",
			interceptor: func(w http.ResponseWriter) (http.ResponseWriter, func()) {
				return &responseWrapper{ResponseWriter: w}, func() {}
			},
			expectValue: "intercepted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter(WithResponseInterceptor(tt.interceptor))
			router.GET("/test", func(w http.ResponseWriter, req *http.Request) error {
				w.WriteHeader(http.StatusOK)
				return nil
			})

			handler := router.Build()
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)

			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestRouter_RequestInterceptor(t *testing.T) {
	tests := []struct {
		name        string
		interceptor func(r *http.Request) (*http.Request, func())
		expectValue string
	}{
		{
			name: "request interceptor adds custom header",
			interceptor: func(r *http.Request) (*http.Request, func()) {
				r.Header.Set("X-Intercepted", "true")
				return r, func() {}
			},
			expectValue: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var headerValue string

			router := NewRouter(WithRequestInterceptor(tt.interceptor))
			router.GET("/test", func(w http.ResponseWriter, req *http.Request) error {
				headerValue = req.Header.Get("X-Intercepted")
				w.WriteHeader(http.StatusOK)
				return nil
			})

			handler := router.Build()
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectValue, headerValue)
		})
	}
}

type responseWrapper struct {
	http.ResponseWriter
}

func collectPatterns(seq iter.Seq[string]) []string {
	var patterns []string
	seq(func(pattern string) bool {
		patterns = append(patterns, pattern)
		return true
	})
	return patterns
}
