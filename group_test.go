package keratin

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouterGroup_Group(t *testing.T) {
	tests := []struct {
		name        string
		prefix      string
		group       *RouterGroup
		expectedLen int
	}{
		{
			name:        "creates group with prefix",
			prefix:      "/api",
			group:       &RouterGroup{},
			expectedLen: 1,
		},
		{
			name:        "creates group with host prefix",
			prefix:      "example.com/",
			group:       &RouterGroup{},
			expectedLen: 1,
		},
		{
			name:        "creates group with path and host",
			prefix:      "example.com/api",
			group:       &RouterGroup{},
			expectedLen: 1,
		},
		{
			name:   "appends to existing children",
			prefix: "/api",
			group: &RouterGroup{
				children: []any{
					&Route{},
					&Route{},
				},
			},
			expectedLen: 3,
		},
		{
			name:        "creates group with empty prefix",
			prefix:      "",
			group:       &RouterGroup{},
			expectedLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialLen := len(tt.group.children)
			newGroup := tt.group.Group(tt.prefix)

			require.NotNil(t, newGroup)
			assert.Equal(t, tt.prefix, newGroup.prefix)
			assert.Len(t, tt.group.children, tt.expectedLen)
			assert.Same(t, tt.group.children[initialLen], newGroup)
		})
	}
}

func TestRouterGroup_Group_Nesting(t *testing.T) {
	root := &RouterGroup{prefix: "/"}

	api := root.Group("/api")
	v1 := api.Group("/v1")
	users := v1.Group("/users")

	assert.Equal(t, "/", root.prefix)
	assert.Equal(t, "/api", api.prefix)
	assert.Equal(t, "/v1", v1.prefix)
	assert.Equal(t, "/users", users.prefix)

	assert.Len(t, root.children, 1)
	assert.Len(t, api.children, 1)
	assert.Len(t, v1.children, 1)
	assert.Len(t, users.children, 0)
}

func TestRouterGroup_UseFunc(t *testing.T) {
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
						return h.ServeHTTP(w, r)
					})
				},
				func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
						return h.ServeHTTP(w, r)
					})
				},
				func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
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
					ID:       "existing",
					Priority: 0,
					Func: func(h Handler) Handler {
						return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
							return h.ServeHTTP(w, r)
						})
					},
				},
			},
			middlewareFuncs: []func(Handler) Handler{
				func(h Handler) Handler {
					return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
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
			group := &RouterGroup{
				Middlewares: tt.initialMiddlewares,
			}

			result := group.UseFunc(tt.middlewareFuncs...)

			require.NotNil(t, result)
			assert.Same(t, group, result, "UseFunc should return the same group instance")
			assert.Len(t, group.Middlewares, tt.expectedCount)
		})
	}
}

func TestRouterGroup_UseFunc_Chaining(t *testing.T) {
	group := &RouterGroup{}

	result := group.UseFunc(func(h Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			return h.ServeHTTP(w, r)
		})
	}).UseFunc(func(h Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			return h.ServeHTTP(w, r)
		})
	}).UseFunc(func(h Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			return h.ServeHTTP(w, r)
		})
	})

	assert.Len(t, group.Middlewares, 3)
	assert.Same(t, group, result)
}

func TestRouterGroup_Use(t *testing.T) {
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
							return h.ServeHTTP(w, r)
						})
					},
				},
			},
			expectedCount: 1,
		},
		{
			name:               "adds multiple middlewares",
			initialMiddlewares: Middlewares[Handler]{},
			middlewares: []*Middleware[Handler]{
				{
					ID:       "mw-1",
					Priority: 10,
					Func: func(h Handler) Handler {
						return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
							return h.ServeHTTP(w, r)
						})
					},
				},
				{
					ID:       "mw-2",
					Priority: 5,
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
			name: "appends to existing middlewares",
			initialMiddlewares: Middlewares[Handler]{
				&Middleware[Handler]{
					ID:       "existing",
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
					ID:       "new",
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
			name:               "empty middlewares list",
			initialMiddlewares: Middlewares[Handler]{},
			middlewares:        []*Middleware[Handler]{},
			expectedCount:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := &RouterGroup{
				Middlewares: tt.initialMiddlewares,
			}

			result := group.Use(tt.middlewares...)

			require.NotNil(t, result)
			assert.Same(t, group, result, "Use should return the same group instance")
			assert.Len(t, group.Middlewares, tt.expectedCount)
		})
	}
}

func TestRouterGroup_Use_Chaining(t *testing.T) {
	group := &RouterGroup{}

	result := group.Use(
		&Middleware[Handler]{
			ID:       "mw-1",
			Priority: 10,
			Func: func(h Handler) Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					return h.ServeHTTP(w, r)
				})
			},
		},
	).Use(
		&Middleware[Handler]{
			ID:       "mw-2",
			Priority: 5,
			Func: func(h Handler) Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					return h.ServeHTTP(w, r)
				})
			},
		},
	)

	assert.Len(t, group.Middlewares, 2)
	assert.Same(t, group, result)
}

func TestRouterGroup_Use_UseFunc_Mixed(t *testing.T) {
	group := &RouterGroup{}

	group.Use(
		&Middleware[Handler]{
			ID:       "named-1",
			Priority: 10,
			Func: func(h Handler) Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					return h.ServeHTTP(w, r)
				})
			},
		},
	).UseFunc(func(h Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			return h.ServeHTTP(w, r)
		})
	}).Use(
		&Middleware[Handler]{
			ID:       "named-2",
			Priority: 5,
			Func: func(h Handler) Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					return h.ServeHTTP(w, r)
				})
			},
		},
	).UseFunc(func(h Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			return h.ServeHTTP(w, r)
		})
	})

	assert.Len(t, group.Middlewares, 4)
	assert.Equal(t, "named-1", group.Middlewares[0].ID)
	assert.Empty(t, group.Middlewares[1].ID)
	assert.Equal(t, "named-2", group.Middlewares[2].ID)
	assert.Empty(t, group.Middlewares[3].ID)
}

func TestRouterGroup_Route(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        string
		handler     func(w http.ResponseWriter, r *http.Request) error
		group       func() *RouterGroup
		expectedLen int
	}{
		{
			name:        "creates route with method and path",
			method:      http.MethodGet,
			path:        "/users",
			handler:     func(w http.ResponseWriter, r *http.Request) error { return nil },
			group:       func() *RouterGroup { return &RouterGroup{} },
			expectedLen: 1,
		},
		{
			name:        "creates route with custom method",
			method:      "SEARCH",
			path:        "/search",
			handler:     func(w http.ResponseWriter, r *http.Request) error { return nil },
			group:       func() *RouterGroup { return &RouterGroup{} },
			expectedLen: 1,
		},
		{
			name:        "creates route with empty method",
			method:      "",
			path:        "/any",
			handler:     func(w http.ResponseWriter, r *http.Request) error { return nil },
			group:       func() *RouterGroup { return &RouterGroup{} },
			expectedLen: 1,
		},
		{
			name:    "appends to existing children",
			method:  http.MethodGet,
			path:    "/new",
			handler: func(w http.ResponseWriter, r *http.Request) error { return nil },
			group: func() *RouterGroup {
				return &RouterGroup{
					children: []any{
						&Route{},
						&Route{},
					},
				}
			},
			expectedLen: 3,
		},
		{
			name:        "creates route with host",
			method:      http.MethodGet,
			path:        "example.com/api",
			handler:     func(w http.ResponseWriter, r *http.Request) error { return nil },
			group:       func() *RouterGroup { return &RouterGroup{} },
			expectedLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run("[Route] "+tt.name, func(t *testing.T) {
			group := tt.group()
			initialLen := len(group.children)
			route := group.Route(tt.method, tt.path, HandlerFunc(tt.handler))

			require.NotNil(t, route)
			assert.Equal(t, tt.method, route.Method)
			assert.Equal(t, tt.path, route.Path)
			assert.NotNil(t, route.Handler)
			assert.Len(t, group.children, tt.expectedLen)
			assert.Equal(t, route, group.children[initialLen])
		})

		t.Run("[RouteFunc] "+tt.name, func(t *testing.T) {
			group := tt.group()
			initialLen := len(group.children)
			route := group.RouteFunc(tt.method, tt.path, tt.handler)

			require.NotNil(t, route)
			assert.Equal(t, tt.method, route.Method)
			assert.Equal(t, tt.path, route.Path)
			assert.NotNil(t, route.Handler)
			assert.Len(t, group.children, tt.expectedLen)
			assert.Equal(t, route, group.children[initialLen])
		})
	}
}

func TestRouterGroup_Any(t *testing.T) {
	group := &RouterGroup{}
	handler := func(w http.ResponseWriter, r *http.Request) error { return nil }

	route := group.Any("/any", handler)

	require.NotNil(t, route)
	assert.Equal(t, "", route.Method)
	assert.Equal(t, "/any", route.Path)
	assert.NotNil(t, route.Handler)
	assert.Len(t, group.children, 1)
}

func TestRouterGroup_GET(t *testing.T) {
	group := &RouterGroup{}
	handler := func(w http.ResponseWriter, r *http.Request) error { return nil }

	route := group.GET("/users", handler)

	require.NotNil(t, route)
	assert.Equal(t, http.MethodGet, route.Method)
	assert.Equal(t, "/users", route.Path)
	assert.NotNil(t, route.Handler)
	assert.Len(t, group.children, 1)
}

func TestRouterGroup_SEARCH(t *testing.T) {
	group := &RouterGroup{}
	handler := func(w http.ResponseWriter, r *http.Request) error { return nil }

	route := group.SEARCH("/search", handler)

	require.NotNil(t, route)
	assert.Equal(t, "SEARCH", route.Method)
	assert.Equal(t, "/search", route.Path)
	assert.NotNil(t, route.Handler)
	assert.Len(t, group.children, 1)
}

func TestRouterGroup_POST(t *testing.T) {
	group := &RouterGroup{}
	handler := func(w http.ResponseWriter, r *http.Request) error { return nil }

	route := group.POST("/users", handler)

	require.NotNil(t, route)
	assert.Equal(t, http.MethodPost, route.Method)
	assert.Equal(t, "/users", route.Path)
	assert.NotNil(t, route.Handler)
	assert.Len(t, group.children, 1)
}

func TestRouterGroup_DELETE(t *testing.T) {
	group := &RouterGroup{}
	handler := func(w http.ResponseWriter, r *http.Request) error { return nil }

	route := group.DELETE("/users/:id", handler)

	require.NotNil(t, route)
	assert.Equal(t, http.MethodDelete, route.Method)
	assert.Equal(t, "/users/:id", route.Path)
	assert.NotNil(t, route.Handler)
	assert.Len(t, group.children, 1)
}

func TestRouterGroup_PATCH(t *testing.T) {
	group := &RouterGroup{}
	handler := func(w http.ResponseWriter, r *http.Request) error { return nil }

	route := group.PATCH("/users/:id", handler)

	require.NotNil(t, route)
	assert.Equal(t, http.MethodPatch, route.Method)
	assert.Equal(t, "/users/:id", route.Path)
	assert.NotNil(t, route.Handler)
	assert.Len(t, group.children, 1)
}

func TestRouterGroup_PUT(t *testing.T) {
	group := &RouterGroup{}
	handler := func(w http.ResponseWriter, r *http.Request) error { return nil }

	route := group.PUT("/users/:id", handler)

	require.NotNil(t, route)
	assert.Equal(t, http.MethodPut, route.Method)
	assert.Equal(t, "/users/:id", route.Path)
	assert.NotNil(t, route.Handler)
	assert.Len(t, group.children, 1)
}

func TestRouterGroup_HEAD(t *testing.T) {
	group := &RouterGroup{}
	handler := func(w http.ResponseWriter, r *http.Request) error { return nil }

	route := group.HEAD("/users", handler)

	require.NotNil(t, route)
	assert.Equal(t, http.MethodHead, route.Method)
	assert.Equal(t, "/users", route.Path)
	assert.NotNil(t, route.Handler)
	assert.Len(t, group.children, 1)
}

func TestRouterGroup_OPTIONS(t *testing.T) {
	group := &RouterGroup{}
	handler := func(w http.ResponseWriter, r *http.Request) error { return nil }

	route := group.OPTIONS("/users", handler)

	require.NotNil(t, route)
	assert.Equal(t, http.MethodOptions, route.Method)
	assert.Equal(t, "/users", route.Path)
	assert.NotNil(t, route.Handler)
	assert.Len(t, group.children, 1)
}

func TestRouterGroup_CONNECT(t *testing.T) {
	group := &RouterGroup{}
	handler := func(w http.ResponseWriter, r *http.Request) error { return nil }

	route := group.CONNECT("/users", handler)

	require.NotNil(t, route)
	assert.Equal(t, http.MethodConnect, route.Method)
	assert.Equal(t, "/users", route.Path)
	assert.NotNil(t, route.Handler)
	assert.Len(t, group.children, 1)
}

func TestRouterGroup_TRACE(t *testing.T) {
	group := &RouterGroup{}
	handler := func(w http.ResponseWriter, r *http.Request) error { return nil }

	route := group.TRACE("/users", handler)

	require.NotNil(t, route)
	assert.Equal(t, http.MethodTrace, route.Method)
	assert.Equal(t, "/users", route.Path)
	assert.NotNil(t, route.Handler)
	assert.Len(t, group.children, 1)
}

func TestRouterGroup_AllHTTPMethods(t *testing.T) {
	group := &RouterGroup{}
	handler := func(w http.ResponseWriter, r *http.Request) error { return nil }

	group.Any("/any", handler)
	group.GET("/get", handler)
	group.HEAD("/head", handler)
	group.POST("/post", handler)
	group.PUT("/put", handler)
	group.PATCH("/patch", handler)
	group.DELETE("/delete", handler)
	group.CONNECT("/connect", handler)
	group.OPTIONS("/options", handler)
	group.TRACE("/trace", handler)
	group.SEARCH("/search", handler)

	assert.Len(t, group.children, 11)

	routes := 0
	groups := 0
	for _, child := range group.children {
		switch child.(type) {
		case *Route:
			routes++
		case *RouterGroup:
			groups++
		}
	}

	assert.Equal(t, 11, routes)
	assert.Equal(t, 0, groups)
}

func TestRouterGroup_ComplexHierarchy(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) error { return nil }

	root := &RouterGroup{prefix: "/"}

	api := root.Group("/api").UseFunc(func(h Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			return h.ServeHTTP(w, r)
		})
	})

	v1 := api.Group("/v1").Use(
		&Middleware[Handler]{
			ID:       "v1-auth",
			Priority: 10,
			Func: func(h Handler) Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					return h.ServeHTTP(w, r)
				})
			},
		},
	)

	users := v1.Group("/users")
	users.GET("", handler)
	users.POST("", handler)
	users.GET("/:id", handler)
	users.DELETE("/:id", handler)

	posts := v1.Group("/posts")
	posts.GET("", handler)
	posts.POST("", handler)

	v2 := api.Group("/v2")

	assert.Len(t, root.children, 1)
	assert.Len(t, api.children, 2)
	assert.Len(t, v1.children, 2)
	assert.Len(t, users.children, 4)
	assert.Len(t, posts.children, 2)
	assert.Len(t, v2.children, 0)

	assert.Len(t, api.Middlewares, 1)
	assert.Len(t, v1.Middlewares, 1)
	assert.Len(t, users.Middlewares, 0)
	assert.Len(t, posts.Middlewares, 0)
	assert.Len(t, v2.Middlewares, 0)
}

func TestRouterGroup_RouteWithMiddleware(t *testing.T) {
	group := &RouterGroup{}

	route := group.GET("/users", func(w http.ResponseWriter, r *http.Request) error {
		return nil
	}).UseFunc(func(h Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			return h.ServeHTTP(w, r)
		})
	}).Use(
		&Middleware[Handler]{
			ID:       "route-mw",
			Priority: 5,
			Func: func(h Handler) Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					return h.ServeHTTP(w, r)
				})
			},
		},
	)

	assert.NotNil(t, route)
	assert.Equal(t, http.MethodGet, route.Method)
	assert.Len(t, route.Middlewares, 2)
	assert.Equal(t, "route-mw", route.Middlewares[1].ID)
}

func TestRouterGroup_Chaining(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) error { return nil }

	group := &RouterGroup{prefix: "/"}

	result := group.
		Group("/api").
		Group("/v1").
		GET("/users", handler).
		UseFunc(func(h Handler) Handler {
			return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
				return h.ServeHTTP(w, r)
			})
		})

	assert.NotNil(t, result)
	assert.IsType(t, &Route{}, result)
	assert.Same(t, result, result)
}

func TestRouterGroup_MixedChildren(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) error { return nil }

	group := &RouterGroup{prefix: "/"}

	group.GET("/route1", handler)
	group.Group("/group1")
	group.POST("/route2", handler)
	group.Group("/group2")
	group.PUT("/route3", handler)

	assert.Len(t, group.children, 5)

	routeCount := 0
	groupCount := 0

	for _, child := range group.children {
		switch child.(type) {
		case *Route:
			routeCount++
		case *RouterGroup:
			groupCount++
		}
	}

	assert.Equal(t, 3, routeCount)
	assert.Equal(t, 2, groupCount)
}

func TestRouterGroup_EmptyPrefix(t *testing.T) {
	group := &RouterGroup{}

	subGroup := group.Group("")
	assert.Equal(t, "", subGroup.prefix)
	assert.Len(t, group.children, 1)
}

func TestRouterGroup_NilHandler(t *testing.T) {
	group := &RouterGroup{}

	route := group.GET("/test", nil)

	require.NotNil(t, route)
	assert.Nil(t, route.Handler)
}

func TestRouterGroup_UseFunc_NoIDs(t *testing.T) {
	group := &RouterGroup{}

	group.UseFunc(func(h Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			return h.ServeHTTP(w, r)
		})
	}, func(h Handler) Handler {
		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			return h.ServeHTTP(w, r)
		})
	})

	assert.Len(t, group.Middlewares, 2)
	assert.Empty(t, group.Middlewares[0].ID)
	assert.Empty(t, group.Middlewares[1].ID)
}

func TestRouterGroup_Use_NamedMiddlewares(t *testing.T) {
	group := &RouterGroup{}

	group.Use(
		&Middleware[Handler]{
			ID:       "auth",
			Priority: 10,
			Func: func(h Handler) Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					return h.ServeHTTP(w, r)
				})
			},
		},
		&Middleware[Handler]{
			ID:       "logger",
			Priority: 5,
			Func: func(h Handler) Handler {
				return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
					return h.ServeHTTP(w, r)
				})
			},
		},
	)

	assert.Len(t, group.Middlewares, 2)
	assert.Equal(t, "auth", group.Middlewares[0].ID)
	assert.Equal(t, "logger", group.Middlewares[1].ID)
}
