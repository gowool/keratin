package adapter

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gowool/keratin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRouter struct {
	routes      []*keratin.Route
	servedCount atomic.Int32
}

func (m *mockRouter) Route(method string, path string, handler keratin.Handler) *keratin.Route {
	route := &keratin.Route{Method: method, Path: path, Handler: handler}
	m.routes = append(m.routes, route)
	return route
}

func (m *mockRouter) ServeHTTP(http.ResponseWriter, *http.Request) {
	m.servedCount.Add(1)
}

func TestNewAdapter(t *testing.T) {
	t.Run("creates adapter with router", func(t *testing.T) {
		mockRouter := &mockRouter{}
		adapter := NewAdapter(mockRouter)

		assert.NotNil(t, adapter)
		assert.Equal(t, mockRouter, adapter.router)
		assert.NotNil(t, adapter.pool)
	})

	t.Run("pool creates new contexts", func(t *testing.T) {
		mockRouter := &mockRouter{}
		adapter := NewAdapter(mockRouter)

		ctx := adapter.pool.Get()
		assert.NotNil(t, ctx)
		assert.IsType(t, &rContext{}, ctx)
	})
}

func TestAdapter_Handle(t *testing.T) {
	t.Run("routes handler with correct method and path", func(t *testing.T) {
		mockRouter := &mockRouter{}
		adapter := NewAdapter(mockRouter)

		op := &huma.Operation{
			Method: "POST",
			Path:   "/users",
		}

		handlerCalled := false
		adapter.Handle(op, func(ctx huma.Context) {
			handlerCalled = true
		})

		assert.Len(t, mockRouter.routes, 1)
		route := mockRouter.routes[0]
		assert.Equal(t, "POST", route.Method)
		assert.Equal(t, "/users", route.Path)
		assert.NotNil(t, route.Handler)
		assert.False(t, handlerCalled)
	})

	t.Run("calls handler on request", func(t *testing.T) {
		mockRouter := &mockRouter{}
		adapter := NewAdapter(mockRouter)

		op := &huma.Operation{
			Method: "GET",
			Path:   "/test",
		}

		handlerCalled := false
		var capturedCtx huma.Context

		adapter.Handle(op, func(ctx huma.Context) {
			handlerCalled = true
			capturedCtx = ctx
		})

		route := mockRouter.routes[0]
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)

		err := route.Handler.ServeHTTP(w, req)
		assert.NoError(t, err)
		assert.True(t, handlerCalled)
		assert.NotNil(t, capturedCtx)
	})

	t.Run("handler receives correct context", func(t *testing.T) {
		mockRouter := &mockRouter{}
		adapter := NewAdapter(mockRouter)

		op := &huma.Operation{
			Method: "GET",
			Path:   "/test",
		}

		var capturedOp *huma.Operation
		var capturedReq *http.Request
		var capturedWriter http.ResponseWriter

		adapter.Handle(op, func(ctx huma.Context) {
			rc := ctx.(*rContext)
			capturedOp = rc.op
			capturedReq = rc.r
			capturedWriter = rc.w
		})

		route := mockRouter.routes[0]
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)

		err := route.Handler.ServeHTTP(w, req)
		require.NoError(t, err)

		assert.NotNil(t, capturedOp)
		assert.NotNil(t, capturedReq)
		assert.NotNil(t, capturedWriter)
		assert.Equal(t, op, capturedOp)
		assert.Equal(t, req, capturedReq)
	})

	t.Run("handler can write response", func(t *testing.T) {
		mockRouter := &mockRouter{}
		adapter := NewAdapter(mockRouter)

		op := &huma.Operation{
			Method: "GET",
			Path:   "/test",
		}

		adapter.Handle(op, func(ctx huma.Context) {
			ctx.SetStatus(http.StatusOK)
			ctx.SetHeader("Content-Type", "text/plain")
			_, _ = ctx.BodyWriter().Write([]byte("hello world"))
		})

		route := mockRouter.routes[0]
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)

		err := route.Handler.ServeHTTP(w, req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
		assert.Equal(t, "hello world", w.Body.String())
	})

	t.Run("handler can read request body", func(t *testing.T) {
		mockRouter := &mockRouter{}
		adapter := NewAdapter(mockRouter)

		op := &huma.Operation{
			Method: "POST",
			Path:   "/test",
		}

		var bodyRead bool

		adapter.Handle(op, func(ctx huma.Context) {
			body := ctx.BodyReader()
			data := make([]byte, 5)
			_, _ = body.Read(data)
			bodyRead = string(data) == "hello"
			ctx.SetStatus(http.StatusOK)
		})

		route := mockRouter.routes[0]
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/test", nil)
		req.Header.Set("Content-Type", "application/json")

		err := route.Handler.ServeHTTP(w, req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.False(t, bodyRead)
	})

	t.Run("multiple handlers work independently", func(t *testing.T) {
		mockRouter := &mockRouter{}
		adapter := NewAdapter(mockRouter)

		op1 := &huma.Operation{Method: "GET", Path: "/users"}
		op2 := &huma.Operation{Method: "POST", Path: "/users"}
		op3 := &huma.Operation{Method: "DELETE", Path: "/users/{id}"}

		handler1Called := false
		handler2Called := false
		handler3Called := false

		adapter.Handle(op1, func(ctx huma.Context) {
			handler1Called = true
			ctx.SetStatus(http.StatusOK)
		})

		adapter.Handle(op2, func(ctx huma.Context) {
			handler2Called = true
			ctx.SetStatus(http.StatusCreated)
		})

		adapter.Handle(op3, func(ctx huma.Context) {
			handler3Called = true
			ctx.SetStatus(http.StatusNoContent)
		})

		assert.Len(t, mockRouter.routes, 3)

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/users", nil)
		_ = mockRouter.routes[0].Handler.ServeHTTP(w, req)
		assert.True(t, handler1Called)
		assert.Equal(t, http.StatusOK, w.Code)

		handler1Called = false
		w = httptest.NewRecorder()
		req = httptest.NewRequest("POST", "/users", nil)
		_ = mockRouter.routes[1].Handler.ServeHTTP(w, req)
		assert.True(t, handler2Called)
		assert.Equal(t, http.StatusCreated, w.Code)

		w = httptest.NewRecorder()
		req = httptest.NewRequest("DELETE", "/users/123", nil)
		req.SetPathValue("id", "123")
		_ = mockRouter.routes[2].Handler.ServeHTTP(w, req)
		assert.True(t, handler3Called)
		assert.Equal(t, http.StatusNoContent, w.Code)
	})
}

func TestAdapter_Pool(t *testing.T) {
	t.Run("context is reused from pool", func(t *testing.T) {
		mockRouter := &mockRouter{}
		adapter := NewAdapter(mockRouter)

		op := &huma.Operation{
			Method: "GET",
			Path:   "/test",
		}

		var capturedContexts []*rContext

		adapter.Handle(op, func(ctx huma.Context) {
			capturedContexts = append(capturedContexts, ctx.(*rContext))
		})

		route := mockRouter.routes[0]

		for i := 0; i < 3; i++ {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/test", nil)

			err := route.Handler.ServeHTTP(w, req)
			require.NoError(t, err)
		}

		assert.Len(t, capturedContexts, 3)
		assert.Equal(t, capturedContexts[0], capturedContexts[1])
		assert.Equal(t, capturedContexts[1], capturedContexts[2])
	})

	t.Run("context is reset before handler", func(t *testing.T) {
		mockRouter := &mockRouter{}
		adapter := NewAdapter(mockRouter)

		op := &huma.Operation{
			Method: "GET",
			Path:   "/test",
		}

		var firstRequest *http.Request
		var secondRequest *http.Request

		adapter.Handle(op, func(ctx huma.Context) {
			rc := ctx.(*rContext)
			if firstRequest == nil {
				firstRequest = rc.r
			} else {
				secondRequest = rc.r
			}
		})

		route := mockRouter.routes[0]

		w1 := httptest.NewRecorder()
		req1 := httptest.NewRequest("GET", "/test1", nil)

		_ = route.Handler.ServeHTTP(w1, req1)

		w2 := httptest.NewRecorder()
		req2 := httptest.NewRequest("POST", "/test2", nil)

		_ = route.Handler.ServeHTTP(w2, req2)

		assert.Equal(t, req1, firstRequest)
		assert.Equal(t, req2, secondRequest)
	})

	t.Run("context is cleared after handler", func(t *testing.T) {
		mockRouter := &mockRouter{}
		adapter := NewAdapter(mockRouter)

		op := &huma.Operation{
			Method: "GET",
			Path:   "/test",
		}

		adapter.Handle(op, func(ctx huma.Context) {
			rc := ctx.(*rContext)
			rc.SetStatus(http.StatusOK)
			rc.SetHeader("X-Test", "value")
		})

		route := mockRouter.routes[0]
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)

		_ = route.Handler.ServeHTTP(w, req)

		ctx := adapter.pool.Get().(*rContext)
		defer adapter.pool.Put(ctx)

		assert.Nil(t, ctx.op)
		assert.Nil(t, ctx.r)
		assert.Nil(t, ctx.w)
		assert.Equal(t, 0, ctx.status)
	})
}

func TestAdapter_ConcurrentRequests(t *testing.T) {
	mockRouter := &mockRouter{}
	adapter := NewAdapter(mockRouter)

	op := &huma.Operation{
		Method: "GET",
		Path:   "/test",
	}

	var wg sync.WaitGroup
	var counter atomic.Int32

	adapter.Handle(op, func(ctx huma.Context) {
		counter.Add(1)
		ctx.SetStatus(http.StatusOK)
	})

	route := mockRouter.routes[0]

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/test", nil)

			_ = route.Handler.ServeHTTP(w, req)
		}()
	}

	wg.Wait()

	assert.Equal(t, int32(10), counter.Load())
}

func TestAdapter_StatusHandling(t *testing.T) {
	tests := []struct {
		name           string
		status         int
		expectedStatus int
	}{
		{
			name:           "200 OK",
			status:         http.StatusOK,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "201 Created",
			status:         http.StatusCreated,
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "400 Bad Request",
			status:         http.StatusBadRequest,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "404 Not Found",
			status:         http.StatusNotFound,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "500 Internal Server Error",
			status:         http.StatusInternalServerError,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRouter := &mockRouter{}
			adapter := NewAdapter(mockRouter)

			op := &huma.Operation{
				Method: "GET",
				Path:   "/test",
			}

			adapter.Handle(op, func(ctx huma.Context) {
				ctx.SetStatus(tt.status)
			})

			route := mockRouter.routes[0]
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/test", nil)

			err := route.Handler.ServeHTTP(w, req)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestAdapter_HeaderHandling(t *testing.T) {
	mockRouter := &mockRouter{}
	adapter := NewAdapter(mockRouter)

	op := &huma.Operation{
		Method: "GET",
		Path:   "/test",
	}

	adapter.Handle(op, func(ctx huma.Context) {
		ctx.SetHeader("Content-Type", "application/json")
		ctx.SetHeader("X-Custom-Header", "custom-value")
		ctx.AppendHeader("X-Multi-Value", "value1")
		ctx.AppendHeader("X-Multi-Value", "value2")
	})

	route := mockRouter.routes[0]
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)

	err := route.Handler.ServeHTTP(w, req)
	require.NoError(t, err)

	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, "custom-value", w.Header().Get("X-Custom-Header"))

	multiValues := w.Header().Values("X-Multi-Value")
	assert.Len(t, multiValues, 2)
	assert.Contains(t, multiValues, "value1")
	assert.Contains(t, multiValues, "value2")
}
