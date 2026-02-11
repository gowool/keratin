package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gowool/keratin"
	"github.com/stretchr/testify/assert"
)

func TestCtxRequestID(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want string
	}{
		{
			name: "returns request ID from context",
			ctx:  context.WithValue(context.Background(), reqIDKey{}, "test-id-123"),
			want: "test-id-123",
		},
		{
			name: "returns empty string when not in context",
			ctx:  context.Background(),
			want: "",
		},
		{
			name: "returns empty string when value is not string",
			ctx:  context.WithValue(context.Background(), reqIDKey{}, 123),
			want: "",
		},
		{
			name: "returns empty string when value is nil",
			ctx:  context.WithValue(context.Background(), reqIDKey{}, nil),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CtxRequestID(tt.ctx)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRequestIDConfig_SetDefaults(t *testing.T) {
	tests := []struct {
		name          string
		config        RequestIDConfig
		wantGenerator bool
		wantHeader    string
	}{
		{
			name: "sets default generator when nil",
			config: RequestIDConfig{
				Generator:    nil,
				TargetHeader: "",
			},
			wantGenerator: true,
			wantHeader:    "X-Request-Id",
		},
		{
			name: "preserves custom generator",
			config: RequestIDConfig{
				Generator: func() string {
					return "custom-generator-id"
				},
				TargetHeader: "",
			},
			wantGenerator: true,
			wantHeader:    "X-Request-Id",
		},
		{
			name: "sets default header when empty",
			config: RequestIDConfig{
				Generator:    nil,
				TargetHeader: "",
			},
			wantGenerator: true,
			wantHeader:    "X-Request-Id",
		},
		{
			name: "preserves custom header",
			config: RequestIDConfig{
				Generator:    nil,
				TargetHeader: "X-Custom-Request-ID",
			},
			wantGenerator: true,
			wantHeader:    "X-Custom-Request-ID",
		},
		{
			name: "preserves both generator and header",
			config: RequestIDConfig{
				Generator: func() string {
					return "custom-id"
				},
				TargetHeader: "X-Custom-Header",
			},
			wantGenerator: true,
			wantHeader:    "X-Custom-Header",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.SetDefaults()

			if tt.wantGenerator {
				assert.NotNil(t, tt.config.Generator)
			}

			assert.Equal(t, tt.wantHeader, tt.config.TargetHeader)
		})
	}
}

func TestRequestID(t *testing.T) {
	tests := []struct {
		name           string
		config         RequestIDConfig
		skippers       []Skipper
		requestHeader  map[string]string
		expectedID     string
		expectedHeader string
		shouldSkip     bool
	}{
		{
			name:   "uses existing X-Request-ID header",
			config: RequestIDConfig{},
			requestHeader: map[string]string{
				"X-Request-Id": "existing-request-id-123",
			},
			expectedID:     "existing-request-id-123",
			expectedHeader: "existing-request-id-123",
		},
		{
			name:           "generates ID when header is empty",
			config:         RequestIDConfig{},
			requestHeader:  map[string]string{},
			expectedID:     "",
			expectedHeader: "",
		},
		{
			name:           "generates ID when header is not present",
			config:         RequestIDConfig{},
			requestHeader:  nil,
			expectedID:     "",
			expectedHeader: "",
		},
		{
			name: "uses custom header name",
			config: RequestIDConfig{
				TargetHeader: "X-Custom-Request-ID",
			},
			requestHeader: map[string]string{
				"X-Custom-Request-ID": "custom-header-id",
			},
			expectedID:     "custom-header-id",
			expectedHeader: "custom-header-id",
		},
		{
			name: "uses custom generator",
			config: RequestIDConfig{
				Generator: func() string {
					return "custom-generated-id-456"
				},
			},
			requestHeader:  map[string]string{},
			expectedID:     "custom-generated-id-456",
			expectedHeader: "custom-generated-id-456",
		},
		{
			name:   "skips middleware when skipper returns true",
			config: RequestIDConfig{},
			skippers: []Skipper{
				func(r *http.Request) bool {
					return r.URL.Path == "/health"
				},
			},
			requestHeader:  map[string]string{},
			expectedID:     "",
			expectedHeader: "",
			shouldSkip:     true,
		},
		{
			name: "processes middleware when skipper returns false",
			config: RequestIDConfig{
				Generator: func() string {
					return "generated-id"
				},
			},
			skippers: []Skipper{
				func(r *http.Request) bool {
					return r.URL.Path == "/health"
				},
			},
			requestHeader:  map[string]string{},
			expectedID:     "generated-id",
			expectedHeader: "generated-id",
			shouldSkip:     false,
		},
		{
			name: "chains multiple skippers",
			config: RequestIDConfig{
				Generator: func() string {
					return "generated-id"
				},
			},
			skippers: []Skipper{
				func(r *http.Request) bool {
					return r.URL.Path == "/health"
				},
				func(r *http.Request) bool {
					return r.URL.Path == "/metrics"
				},
			},
			requestHeader:  map[string]string{},
			expectedID:     "generated-id",
			expectedHeader: "generated-id",
			shouldSkip:     false,
		},
		{
			name:   "skips when first skipper matches",
			config: RequestIDConfig{},
			skippers: []Skipper{
				func(r *http.Request) bool {
					return r.URL.Path == "/health"
				},
				func(r *http.Request) bool {
					return r.URL.Path == "/metrics"
				},
			},
			requestHeader:  map[string]string{},
			expectedID:     "",
			expectedHeader: "",
			shouldSkip:     true,
		},
		{
			name:   "skips when second skipper matches",
			config: RequestIDConfig{},
			skippers: []Skipper{
				func(r *http.Request) bool {
					return r.URL.Path == "/admin"
				},
				func(r *http.Request) bool {
					return r.URL.Path == "/health"
				},
			},
			requestHeader:  map[string]string{},
			expectedID:     "",
			expectedHeader: "",
			shouldSkip:     true,
		},
		{
			name: "empty skippers list processes request",
			config: RequestIDConfig{
				Generator: func() string {
					return "generated-id"
				},
			},
			skippers:       []Skipper{},
			requestHeader:  map[string]string{},
			expectedID:     "generated-id",
			expectedHeader: "generated-id",
		},
		{
			name: "custom header with custom generator",
			config: RequestIDConfig{
				Generator: func() string {
					return "custom-gen-id"
				},
				TargetHeader: "X-Trace-ID",
			},
			requestHeader:  map[string]string{},
			expectedID:     "custom-gen-id",
			expectedHeader: "custom-gen-id",
		},
		{
			name: "prefers header over generator",
			config: RequestIDConfig{
				Generator: func() string {
					return "generated-id"
				},
			},
			requestHeader: map[string]string{
				"X-Request-Id": "header-id",
			},
			expectedID:     "header-id",
			expectedHeader: "header-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := RequestID(tt.config, tt.skippers...)

			handler := middleware(keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
				w.WriteHeader(http.StatusOK)
				return nil
			}))

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			for k, v := range tt.requestHeader {
				req.Header.Set(k, v)
			}

			if tt.shouldSkip {
				req.URL.Path = "/health"
			}

			w := httptest.NewRecorder()
			_ = handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			if tt.shouldSkip {
				headerName := tt.config.TargetHeader
				if headerName == "" {
					headerName = "X-Request-Id"
				}
				headerValue := w.Header().Get(headerName)
				assert.Equal(t, "", headerValue)
			} else {
				if tt.expectedHeader != "" {
					headerName := tt.config.TargetHeader
					if headerName == "" {
						headerName = "X-Request-Id"
					}
					headerValue := w.Header().Get(headerName)
					assert.Equal(t, tt.expectedHeader, headerValue)
				}
			}
		})
	}
}

func TestRequestID_ContextPropagation(t *testing.T) {
	tests := []struct {
		name      string
		config    RequestIDConfig
		requestID string
		expectID  string
	}{
		{
			name:      "propagates request ID to context",
			config:    RequestIDConfig{},
			requestID: "propagate-id",
			expectID:  "propagate-id",
		},
		{
			name: "propagates generated ID to context",
			config: RequestIDConfig{
				Generator: func() string {
					return "generated-context-id"
				},
			},
			requestID: "",
			expectID:  "generated-context-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := RequestID(tt.config)

			var contextID string
			handler := middleware(keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
				contextID = CtxRequestID(r.Context())
				w.WriteHeader(http.StatusOK)
				return nil
			}))

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.requestID != "" {
				req.Header.Set("X-Request-Id", tt.requestID)
			}

			w := httptest.NewRecorder()
			_ = handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, tt.expectID, contextID)
		})
	}
}
