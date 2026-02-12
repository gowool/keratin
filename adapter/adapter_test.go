package adapter

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gowool/keratin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRouter struct {
	mu                 sync.Mutex
	routeFuncCalls     []routeFuncCall
	routeFuncCallCount int
}

type routeFuncCall struct {
	method  string
	path    string
	handler func(http.ResponseWriter, *http.Request) error
}

func (m *mockRouter) RouteFunc(method string, path string, handler func(http.ResponseWriter, *http.Request) error) *keratin.Route {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.routeFuncCalls = append(m.routeFuncCalls, routeFuncCall{
		method:  method,
		path:    path,
		handler: handler,
	})
	m.routeFuncCallCount++
	return &keratin.Route{}
}

func (m *mockRouter) GetCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.routeFuncCallCount
}

func (m *mockRouter) GetCalls() []routeFuncCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.routeFuncCalls
}

func (m *mockRouter) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.routeFuncCalls = nil
	m.routeFuncCallCount = 0
}

func TestNewAdapter(t *testing.T) {
	tests := []struct {
		name     string
		handler  http.Handler
		router   router
		expected bool
	}{
		{
			name:     "creates adapter with handler and router",
			handler:  http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
			router:   &mockRouter{},
			expected: true,
		},
		{
			name:     "creates adapter with nil handler",
			handler:  nil,
			router:   &mockRouter{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewAdapter(tt.handler, tt.router)

			assert.NotNil(t, adapter)
			assert.NotNil(t, adapter.pool)
		})
	}
}

func TestAdapter_Handle(t *testing.T) {
	tests := []struct {
		name        string
		operation   *huma.Operation
		handler     func(huma.Context)
		expectCall  bool
		expectCount int
	}{
		{
			name: "handles operation with GET method",
			operation: &huma.Operation{
				Method: "GET",
				Path:   "/users",
			},
			handler: func(ctx huma.Context) {
				ctx.SetStatus(http.StatusOK)
			},
			expectCall:  true,
			expectCount: 1,
		},
		{
			name: "handles operation with POST method",
			operation: &huma.Operation{
				Method: "POST",
				Path:   "/users",
			},
			handler: func(ctx huma.Context) {
				ctx.SetStatus(http.StatusCreated)
			},
			expectCall:  true,
			expectCount: 1,
		},
		{
			name: "handles operation with DELETE method",
			operation: &huma.Operation{
				Method: "DELETE",
				Path:   "/users/{id}",
			},
			handler: func(ctx huma.Context) {
				ctx.SetStatus(http.StatusNoContent)
			},
			expectCall:  true,
			expectCount: 1,
		},
		{
			name: "handles multiple operations",
			operation: &huma.Operation{
				Method: "GET",
				Path:   "/posts",
			},
			handler: func(ctx huma.Context) {
				ctx.SetStatus(http.StatusOK)
			},
			expectCall:  true,
			expectCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRouter := &mockRouter{}
			adapter := NewAdapter(nil, mockRouter)

			if tt.expectCount > 1 {
				for i := 0; i < tt.expectCount; i++ {
					adapter.Handle(tt.operation, tt.handler)
				}
			} else {
				adapter.Handle(tt.operation, tt.handler)
			}

			assert.Equal(t, tt.expectCount, mockRouter.GetCallCount())

			calls := mockRouter.GetCalls()
			if tt.expectCall {
				require.Len(t, calls, tt.expectCount)
				assert.Equal(t, tt.operation.Method, calls[0].method)
				assert.Equal(t, tt.operation.Path, calls[0].path)
			}
		})
	}
}

func TestAdapter_HandlerExecution(t *testing.T) {
	t.Run("handler is called with correct context", func(t *testing.T) {
		mockRouter := &mockRouter{}
		adapter := NewAdapter(nil, mockRouter)

		operation := &huma.Operation{
			Method: "GET",
			Path:   "/test",
		}

		var capturedCtx huma.Context
		handlerCalled := false

		adapter.Handle(operation, func(ctx huma.Context) {
			handlerCalled = true
			capturedCtx = ctx
			ctx.SetStatus(http.StatusOK)
		})

		calls := mockRouter.GetCalls()
		require.Len(t, calls, 1)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		err := calls[0].handler(w, req)
		require.NoError(t, err)

		assert.True(t, handlerCalled)
		assert.NotNil(t, capturedCtx)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("handler can write response", func(t *testing.T) {
		mockRouter := &mockRouter{}
		adapter := NewAdapter(nil, mockRouter)

		operation := &huma.Operation{
			Method: "GET",
			Path:   "/test",
		}

		adapter.Handle(operation, func(ctx huma.Context) {
			ctx.SetStatus(http.StatusOK)
			writer := ctx.BodyWriter()
			_, _ = writer.Write([]byte("hello world"))
		})

		calls := mockRouter.GetCalls()
		require.Len(t, calls, 1)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		err := calls[0].handler(w, req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "hello world", w.Body.String())
	})

	t.Run("handler can set headers", func(t *testing.T) {
		mockRouter := &mockRouter{}
		adapter := NewAdapter(nil, mockRouter)

		operation := &huma.Operation{
			Method: "GET",
			Path:   "/test",
		}

		adapter.Handle(operation, func(ctx huma.Context) {
			ctx.SetHeader("X-Custom-Header", "custom-value")
			ctx.AppendHeader("X-Multi-Header", "value1")
			ctx.AppendHeader("X-Multi-Header", "value2")
			ctx.SetStatus(http.StatusOK)
		})

		calls := mockRouter.GetCalls()
		require.Len(t, calls, 1)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		err := calls[0].handler(w, req)
		require.NoError(t, err)

		assert.Equal(t, "custom-value", w.Header().Get("X-Custom-Header"))
		assert.Equal(t, []string{"value1", "value2"}, w.Header()["X-Multi-Header"])
	})
}

func TestAdapter_ContextPooling(t *testing.T) {
	t.Run("context is properly reset before handler execution", func(t *testing.T) {
		mockRouter := &mockRouter{}
		adapter := NewAdapter(nil, mockRouter)

		operation := &huma.Operation{
			Method: "GET",
			Path:   "/test",
		}

		handler := func(ctx huma.Context) {
			rctx := ctx.(*kContext)

			assert.Equal(t, operation, rctx.op)
			assert.NotNil(t, rctx.r)
			assert.NotNil(t, rctx.w)
			assert.Equal(t, 0, rctx.status, "Status should start at 0 for each request")

			ctx.SetStatus(http.StatusCreated)
		}

		adapter.Handle(operation, handler)
		adapter.Handle(operation, handler)
		adapter.Handle(operation, handler)

		calls := mockRouter.GetCalls()
		require.Len(t, calls, 3)

		for _, call := range calls {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			err := call.handler(w, req)
			require.NoError(t, err)

			assert.Equal(t, http.StatusCreated, w.Code)
		}
	})

	t.Run("context state is isolated between requests", func(t *testing.T) {
		mockRouter := &mockRouter{}
		adapter := NewAdapter(nil, mockRouter)

		operation := &huma.Operation{
			Method: "GET",
			Path:   "/test",
		}

		adapter.Handle(operation, func(ctx huma.Context) {
			rctx := ctx.(*kContext)

			assert.Equal(t, 0, rctx.status, "Status should start at 0")

			ctx.SetStatus(http.StatusCreated)
		})

		calls := mockRouter.GetCalls()
		require.Len(t, calls, 1)

		req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
		w1 := httptest.NewRecorder()

		err := calls[0].handler(w1, req1)
		require.NoError(t, err)

		req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
		w2 := httptest.NewRecorder()

		err = calls[0].handler(w2, req2)
		require.NoError(t, err)

		assert.Equal(t, http.StatusCreated, w1.Code)
		assert.Equal(t, http.StatusCreated, w2.Code)
	})
}

func TestAdapter_ContextUnwrap(t *testing.T) {
	mockRouter := &mockRouter{}
	adapter := NewAdapter(nil, mockRouter)

	operation := &huma.Operation{
		Method: "GET",
		Path:   "/test",
	}

	adapter.Handle(operation, func(ctx huma.Context) {
		r, w := Unwrap(ctx)

		assert.NotNil(t, r)
		assert.NotNil(t, w)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("unwrapped"))
	})

	calls := mockRouter.GetCalls()
	require.Len(t, calls, 1)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	err := calls[0].handler(w, req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "unwrapped", w.Body.String())
}

func TestAdapter_MultipleOperations(t *testing.T) {
	mockRouter := &mockRouter{}
	adapter := NewAdapter(nil, mockRouter)

	operations := []*huma.Operation{
		{Method: "GET", Path: "/users"},
		{Method: "POST", Path: "/users"},
		{Method: "GET", Path: "/users/{id}"},
		{Method: "PUT", Path: "/users/{id}"},
		{Method: "DELETE", Path: "/users/{id}"},
	}

	for _, op := range operations {
		adapter.Handle(op, func(ctx huma.Context) {
			ctx.SetStatus(http.StatusOK)
		})
	}

	assert.Equal(t, len(operations), mockRouter.GetCallCount())

	calls := mockRouter.GetCalls()
	for i, call := range calls {
		assert.Equal(t, operations[i].Method, call.method)
		assert.Equal(t, operations[i].Path, call.path)
	}
}

func TestAdapter_PanicInHandler(t *testing.T) {
	mockRouter := &mockRouter{}
	adapter := NewAdapter(nil, mockRouter)

	operation := &huma.Operation{
		Method: "GET",
		Path:   "/test",
	}

	adapter.Handle(operation, func(ctx huma.Context) {
		panic("test panic")
	})

	calls := mockRouter.GetCalls()
	require.Len(t, calls, 1)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	assert.Panics(t, func() {
		_ = calls[0].handler(w, req)
	})
}

func TestAdapter_HandlerReadsRequestBody(t *testing.T) {
	mockRouter := &mockRouter{}
	adapter := NewAdapter(nil, mockRouter)

	operation := &huma.Operation{
		Method: "POST",
		Path:   "/users",
	}

	adapter.Handle(operation, func(ctx huma.Context) {
		body := ctx.BodyReader()
		data := make([]byte, 100)
		n, _ := body.Read(data)

		ctx.SetStatus(http.StatusOK)
		_, _ = ctx.BodyWriter().Write(data[:n])
	})

	calls := mockRouter.GetCalls()
	require.Len(t, calls, 1)

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader("request body"))
	w := httptest.NewRecorder()

	err := calls[0].handler(w, req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "request body", w.Body.String())
}

func TestAdapter_HandlerReadsPathParams(t *testing.T) {
	mockRouter := &mockRouter{}
	adapter := NewAdapter(nil, mockRouter)

	operation := &huma.Operation{
		Method: "GET",
		Path:   "/users/{id}",
	}

	adapter.Handle(operation, func(ctx huma.Context) {
		id := ctx.Param("id")

		ctx.SetStatus(http.StatusOK)
		_, _ = ctx.BodyWriter().Write([]byte(id))
	})

	calls := mockRouter.GetCalls()
	require.Len(t, calls, 1)

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	req.SetPathValue("id", "123")
	w := httptest.NewRecorder()

	err := calls[0].handler(w, req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "123", w.Body.String())
}

func TestAdapter_HandlerReadsQueryParams(t *testing.T) {
	mockRouter := &mockRouter{}
	adapter := NewAdapter(nil, mockRouter)

	operation := &huma.Operation{
		Method: "GET",
		Path:   "/users",
	}

	adapter.Handle(operation, func(ctx huma.Context) {
		page := ctx.Query("page")
		limit := ctx.Query("limit")

		ctx.SetStatus(http.StatusOK)
		_, _ = ctx.BodyWriter().Write([]byte(page + ":" + limit))
	})

	calls := mockRouter.GetCalls()
	require.Len(t, calls, 1)

	req := httptest.NewRequest(http.MethodGet, "/users?page=2&limit=10", nil)
	w := httptest.NewRecorder()

	err := calls[0].handler(w, req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "2:10", w.Body.String())
}

func BenchmarkAdapter_Handle(b *testing.B) {
	mockRouter := &mockRouter{}
	adapter := NewAdapter(nil, mockRouter)

	operation := &huma.Operation{
		Method: "GET",
		Path:   "/test",
	}

	handler := func(ctx huma.Context) {
		ctx.SetStatus(http.StatusOK)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adapter.Handle(operation, handler)
	}
}

func BenchmarkAdapter_HandlerExecution(b *testing.B) {
	mockRouter := &mockRouter{}
	adapter := NewAdapter(nil, mockRouter)

	operation := &huma.Operation{
		Method: "GET",
		Path:   "/test",
	}

	adapter.Handle(operation, func(ctx huma.Context) {
		ctx.SetStatus(http.StatusOK)
		_, _ = ctx.BodyWriter().Write([]byte("ok"))
	})

	calls := mockRouter.GetCalls()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		_ = calls[0].handler(w, req)
	}
}
