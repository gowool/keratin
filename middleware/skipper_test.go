package middleware

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChainSkipper(t *testing.T) {
	tests := []struct {
		name     string
		skippers []Skipper
		path     string
		method   string
		want     bool
	}{
		{
			name:     "no skippers",
			skippers: []Skipper{},
			path:     "/test",
			method:   http.MethodGet,
			want:     false,
		},
		{
			name: "first skipper returns true",
			skippers: []Skipper{
				PrefixPathSkipper("/api"),
				PrefixPathSkipper("/admin"),
			},
			path:   "/api/users",
			method: http.MethodGet,
			want:   true,
		},
		{
			name: "second skipper returns true",
			skippers: []Skipper{
				PrefixPathSkipper("/api"),
				PrefixPathSkipper("/admin"),
			},
			path:   "/admin/users",
			method: http.MethodGet,
			want:   true,
		},
		{
			name: "no skipper matches",
			skippers: []Skipper{
				PrefixPathSkipper("/api"),
				PrefixPathSkipper("/admin"),
			},
			path:   "/public/page",
			method: http.MethodGet,
			want:   false,
		},
		{
			name: "multiple skippers all return false",
			skippers: []Skipper{
				EqualPathSkipper("/exact"),
				SuffixPathSkipper(".json"),
			},
			path:   "/public/page",
			method: http.MethodGet,
			want:   false,
		},
		{
			name:     "nil skipper list",
			skippers: nil,
			path:     "/test",
			method:   http.MethodGet,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := ChainSkipper(tt.skippers...)
			req := createRequest(tt.method, tt.path)
			got := chain(req)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPrefixPathSkipper(t *testing.T) {
	tests := []struct {
		name     string
		prefixes []string
		path     string
		method   string
		want     bool
	}{
		{
			name:     "matches single prefix",
			prefixes: []string{"/api"},
			path:     "/api/users",
			method:   http.MethodGet,
			want:     true,
		},
		{
			name:     "case insensitive match",
			prefixes: []string{"/API"},
			path:     "/api/users",
			method:   http.MethodGet,
			want:     true,
		},
		{
			name:     "case insensitive path",
			prefixes: []string{"/api"},
			path:     "/API/users",
			method:   http.MethodGet,
			want:     true,
		},
		{
			name:     "matches one of multiple prefixes",
			prefixes: []string{"/api", "/admin", "/public"},
			path:     "/admin/settings",
			method:   http.MethodGet,
			want:     true,
		},
		{
			name:     "no prefix match",
			prefixes: []string{"/api", "/admin"},
			path:     "/public/page",
			method:   http.MethodGet,
			want:     false,
		},
		{
			name:     "exact match with prefix",
			prefixes: []string{"/api"},
			path:     "/api",
			method:   http.MethodGet,
			want:     true,
		},
		{
			name:     "partial path after prefix",
			prefixes: []string{"/api/v1"},
			path:     "/api/v1/users",
			method:   http.MethodGet,
			want:     true,
		},
		{
			name:     "empty prefixes list",
			prefixes: []string{},
			path:     "/api/users",
			method:   http.MethodGet,
			want:     false,
		},
		{
			name:     "prefix with method specifier",
			prefixes: []string{"GET /api", "POST /api"},
			path:     "/api/users",
			method:   http.MethodGet,
			want:     true,
		},
		{
			name:     "prefix with wrong method",
			prefixes: []string{"GET /api"},
			path:     "/api/users",
			method:   http.MethodPost,
			want:     false,
		},
		{
			name:     "case insensitive method",
			prefixes: []string{"get /api"},
			path:     "/api/users",
			method:   http.MethodGet,
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skipper := PrefixPathSkipper(tt.prefixes...)
			req := createRequest(tt.method, tt.path)
			got := skipper(req)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSuffixPathSkipper(t *testing.T) {
	tests := []struct {
		name     string
		suffixes []string
		path     string
		method   string
		want     bool
	}{
		{
			name:     "matches single suffix",
			suffixes: []string{".json"},
			path:     "/api/data.json",
			method:   http.MethodGet,
			want:     true,
		},
		{
			name:     "case insensitive match",
			suffixes: []string{".JSON"},
			path:     "/api/data.json",
			method:   http.MethodGet,
			want:     true,
		},
		{
			name:     "case insensitive path",
			suffixes: []string{".json"},
			path:     "/api/data.JSON",
			method:   http.MethodGet,
			want:     true,
		},
		{
			name:     "matches one of multiple suffixes",
			suffixes: []string{".json", ".xml", ".html"},
			path:     "/api/data.xml",
			method:   http.MethodGet,
			want:     true,
		},
		{
			name:     "no suffix match",
			suffixes: []string{".json", ".xml"},
			path:     "/api/data.txt",
			method:   http.MethodGet,
			want:     false,
		},
		{
			name:     "exact match with suffix",
			suffixes: []string{".json"},
			path:     "/.json",
			method:   http.MethodGet,
			want:     true,
		},
		{
			name:     "path before suffix",
			suffixes: []string{"/data.json"},
			path:     "/api/data.json",
			method:   http.MethodGet,
			want:     true,
		},
		{
			name:     "empty suffixes list",
			suffixes: []string{},
			path:     "/api/data.json",
			method:   http.MethodGet,
			want:     false,
		},
		{
			name:     "suffix with method specifier",
			suffixes: []string{"GET .json", "POST .json"},
			path:     "/api/data.json",
			method:   http.MethodGet,
			want:     true,
		},
		{
			name:     "suffix with wrong method",
			suffixes: []string{"GET .json"},
			path:     "/api/data.json",
			method:   http.MethodPost,
			want:     false,
		},
		{
			name:     "case insensitive method",
			suffixes: []string{"get .json"},
			path:     "/api/data.json",
			method:   http.MethodGet,
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skipper := SuffixPathSkipper(tt.suffixes...)
			req := createRequest(tt.method, tt.path)
			got := skipper(req)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEqualPathSkipper(t *testing.T) {
	tests := []struct {
		name   string
		paths  []string
		path   string
		method string
		want   bool
	}{
		{
			name:   "matches single path",
			paths:  []string{"/health"},
			path:   "/health",
			method: http.MethodGet,
			want:   true,
		},
		{
			name:   "case insensitive match",
			paths:  []string{"/HEALTH"},
			path:   "/health",
			method: http.MethodGet,
			want:   true,
		},
		{
			name:   "case insensitive path",
			paths:  []string{"/health"},
			path:   "/HEALTH",
			method: http.MethodGet,
			want:   true,
		},
		{
			name:   "matches one of multiple paths",
			paths:  []string{"/health", "/metrics", "/ready"},
			path:   "/metrics",
			method: http.MethodGet,
			want:   true,
		},
		{
			name:   "no path match",
			paths:  []string{"/health", "/metrics"},
			path:   "/status",
			method: http.MethodGet,
			want:   false,
		},
		{
			name:   "prefix does not match",
			paths:  []string{"/api"},
			path:   "/api/users",
			method: http.MethodGet,
			want:   false,
		},
		{
			name:   "suffix does not match",
			paths:  []string{".json"},
			path:   "/data.json",
			method: http.MethodGet,
			want:   false,
		},
		{
			name:   "empty paths list",
			paths:  []string{},
			path:   "/health",
			method: http.MethodGet,
			want:   false,
		},
		{
			name:   "path with method specifier",
			paths:  []string{"GET /health", "POST /health"},
			path:   "/health",
			method: http.MethodGet,
			want:   true,
		},
		{
			name:   "path with wrong method",
			paths:  []string{"GET /health"},
			path:   "/health",
			method: http.MethodPost,
			want:   false,
		},
		{
			name:   "case insensitive method doesn't work for EqualPathSkipper",
			paths:  []string{"get /health"},
			path:   "/health",
			method: http.MethodGet,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skipper := EqualPathSkipper(tt.paths...)
			req := createRequest(tt.method, tt.path)
			got := skipper(req)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCheckMethod(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		skip         string
		expectedPath string
		expectedOK   bool
	}{
		{
			name:         "no method specified in skip",
			method:       "GET",
			skip:         "/api/users",
			expectedPath: "/api/users",
			expectedOK:   true,
		},
		{
			name:         "matching method",
			method:       "GET",
			skip:         "GET /api/users",
			expectedPath: "/api/users",
			expectedOK:   true,
		},
		{
			name:         "non-matching method",
			method:       "POST",
			skip:         "GET /api/users",
			expectedPath: "",
			expectedOK:   false,
		},
		{
			name:         "case sensitive method matching",
			method:       "get",
			skip:         "GET /api/users",
			expectedPath: "",
			expectedOK:   false,
		},
		{
			name:         "different matching method",
			method:       "POST",
			skip:         "POST /api/users",
			expectedPath: "/api/users",
			expectedOK:   true,
		},
		{
			name:         "PUT method matching",
			method:       "PUT",
			skip:         "PUT /api/users",
			expectedPath: "/api/users",
			expectedOK:   true,
		},
		{
			name:         "DELETE method matching",
			method:       "DELETE",
			skip:         "DELETE /api/users",
			expectedPath: "/api/users",
			expectedOK:   true,
		},
		{
			name:         "PATCH method matching",
			method:       "PATCH",
			skip:         "PATCH /api/users",
			expectedPath: "/api/users",
			expectedOK:   true,
		},
		{
			name:         "HEAD method matching",
			method:       "HEAD",
			skip:         "HEAD /api/users",
			expectedPath: "/api/users",
			expectedOK:   true,
		},
		{
			name:         "OPTIONS method matching",
			method:       "OPTIONS",
			skip:         "OPTIONS /api/users",
			expectedPath: "/api/users",
			expectedOK:   true,
		},
		{
			name:         "method with empty path",
			method:       "GET",
			skip:         "GET ",
			expectedPath: "",
			expectedOK:   true,
		},
		{
			name:         "malformed pattern - missing path",
			method:       "GET",
			skip:         "GET",
			expectedPath: "GET",
			expectedOK:   true,
		},
		{
			name:         "malformed pattern - extra spaces",
			method:       "GET",
			skip:         "GET   /api/users",
			expectedPath: "/api/users",
			expectedOK:   true,
		},
		{
			name:         "empty skip string",
			method:       "GET",
			skip:         "",
			expectedPath: "",
			expectedOK:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, ok := CheckMethod(tt.method, tt.skip)
			assert.Equal(t, tt.expectedPath, path)
			assert.Equal(t, tt.expectedOK, ok)
		})
	}
}

func TestExpressionSkipper(t *testing.T) {
	type RequestEnv struct {
		Method   string
		Path     string
		Header   map[string]string
		RemoteIP string
	}

	extractEnv := func(r *http.Request) RequestEnv {
		headers := make(map[string]string)
		for k, v := range r.Header {
			if len(v) > 0 {
				headers[k] = v[0]
			}
		}
		return RequestEnv{
			Method:   r.Method,
			Path:     r.URL.Path,
			Header:   headers,
			RemoteIP: r.RemoteAddr,
		}
	}

	tests := []struct {
		name        string
		envFunc     func(*http.Request) RequestEnv
		expressions []string
		method      string
		path        string
		headers     map[string]string
		want        bool
	}{
		{
			name:        "single expression evaluates to true",
			envFunc:     extractEnv,
			expressions: []string{"Method == 'GET'"},
			method:      http.MethodGet,
			path:        "/api/users",
			want:        true,
		},
		{
			name:        "single expression evaluates to false",
			envFunc:     extractEnv,
			expressions: []string{"Method == 'POST'"},
			method:      http.MethodGet,
			path:        "/api/users",
			want:        false,
		},
		{
			name:        "path matching expression",
			envFunc:     extractEnv,
			expressions: []string{"Path contains '/admin'"},
			method:      http.MethodGet,
			path:        "/admin/settings",
			want:        true,
		},
		{
			name:        "path not matching",
			envFunc:     extractEnv,
			expressions: []string{"Path contains '/admin'"},
			method:      http.MethodGet,
			path:        "/api/users",
			want:        false,
		},
		{
			name:        "multiple expressions - first matches",
			envFunc:     extractEnv,
			expressions: []string{"Method == 'GET'", "Path contains '/admin'"},
			method:      http.MethodGet,
			path:        "/api/users",
			want:        true,
		},
		{
			name:        "multiple expressions - second matches",
			envFunc:     extractEnv,
			expressions: []string{"Method == 'DELETE'", "Path contains '/api'"},
			method:      http.MethodGet,
			path:        "/api/users",
			want:        true,
		},
		{
			name:        "multiple expressions - none match",
			envFunc:     extractEnv,
			expressions: []string{"Method == 'DELETE'", "Path contains '/admin'"},
			method:      http.MethodGet,
			path:        "/api/users",
			want:        false,
		},
		{
			name:        "complex expression with AND",
			envFunc:     extractEnv,
			expressions: []string{"Method == 'GET' and Path startsWith '/api'"},
			method:      http.MethodGet,
			path:        "/api/users",
			want:        true,
		},
		{
			name:        "complex expression with AND - fails",
			envFunc:     extractEnv,
			expressions: []string{"Method == 'GET' and Path startsWith '/admin'"},
			method:      http.MethodGet,
			path:        "/api/users",
			want:        false,
		},
		{
			name:        "complex expression with OR",
			envFunc:     extractEnv,
			expressions: []string{"Method == 'GET' or Method == 'POST'"},
			method:      http.MethodPost,
			path:        "/api/users",
			want:        true,
		},
		{
			name:        "expression with header check",
			envFunc:     extractEnv,
			expressions: []string{"Header['X-Auth-Token'] != ''"},
			method:      http.MethodGet,
			path:        "/api/users",
			headers:     map[string]string{"X-Auth-Token": "secret"},
			want:        true,
		},
		{
			name:        "expression with missing header",
			envFunc:     extractEnv,
			expressions: []string{"Header['X-Auth-Token'] != ''"},
			method:      http.MethodGet,
			path:        "/api/users",
			want:        false,
		},
		{
			name:        "invalid expression is ignored",
			envFunc:     extractEnv,
			expressions: []string{"invalid syntax", "Method == 'GET'"},
			method:      http.MethodGet,
			path:        "/api/users",
			want:        true,
		},
		{
			name:        "all invalid expressions",
			envFunc:     extractEnv,
			expressions: []string{"invalid syntax 1", "invalid syntax 2"},
			method:      http.MethodGet,
			path:        "/api/users",
			want:        false,
		},
		{
			name:        "empty expressions list",
			envFunc:     extractEnv,
			expressions: []string{},
			method:      http.MethodGet,
			path:        "/api/users",
			want:        false,
		},
		{
			name:        "path ends with check",
			envFunc:     extractEnv,
			expressions: []string{"Path endsWith '.json'"},
			method:      http.MethodGet,
			path:        "/api/data.json",
			want:        true,
		},
		{
			name:        "path regex-like matching",
			envFunc:     extractEnv,
			expressions: []string{"Path matches '^/api/.*'"},
			method:      http.MethodGet,
			path:        "/api/v1/users",
			want:        true,
		},
		{
			name:        "method in list",
			envFunc:     extractEnv,
			expressions: []string{"Method in ['GET', 'POST', 'PUT']"},
			method:      http.MethodPost,
			path:        "/api/users",
			want:        true,
		},
		{
			name:        "method not in list",
			envFunc:     extractEnv,
			expressions: []string{"Method in ['GET', 'POST', 'PUT']"},
			method:      http.MethodDelete,
			path:        "/api/users",
			want:        false,
		},
		{
			name:        "combined conditions",
			envFunc:     extractEnv,
			expressions: []string{"Method in ['GET', 'POST'] and (Path startsWith '/api' or Path startsWith '/admin')"},
			method:      http.MethodPost,
			path:        "/admin/settings",
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skipper := ExpressionSkipper(tt.envFunc, tt.expressions...)

			req := &http.Request{
				Method: tt.method,
				URL:    &url.URL{Path: tt.path},
				Header: make(http.Header),
			}
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			got := skipper(req)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExpressionSkipper_DifferentEnvTypes(t *testing.T) {
	t.Run("struct environment with boolean", func(t *testing.T) {
		type SimpleEnv struct {
			IsAdmin bool
		}

		extractEnv := func(r *http.Request) SimpleEnv {
			return SimpleEnv{IsAdmin: r.URL.Path == "/admin"}
		}

		skipper := ExpressionSkipper(extractEnv, "IsAdmin == true")
		req := &http.Request{URL: &url.URL{Path: "/admin"}}

		got := skipper(req)
		assert.True(t, got)
	})

	t.Run("struct environment with nested struct", func(t *testing.T) {
		type Nested struct {
			Role string
		}
		type ComplexEnv struct {
			User   Nested
			Method string
		}

		extractEnv := func(r *http.Request) ComplexEnv {
			return ComplexEnv{
				User:   Nested{Role: "admin"},
				Method: r.Method,
			}
		}

		skipper := ExpressionSkipper(extractEnv, "User.Role == 'admin' and Method == 'GET'")
		req := &http.Request{Method: http.MethodGet, URL: &url.URL{Path: "/test"}}

		got := skipper(req)
		assert.True(t, got)
	})

	t.Run("struct environment with integer field", func(t *testing.T) {
		type CountEnv struct {
			Count int
		}

		extractEnv := func(r *http.Request) CountEnv {
			count := 0
			if r.URL.Path == "/popular" {
				count = 100
			}
			return CountEnv{Count: count}
		}

		skipper := ExpressionSkipper(extractEnv, "Count > 50")
		req := &http.Request{URL: &url.URL{Path: "/popular"}}

		got := skipper(req)
		assert.True(t, got)
	})

	t.Run("struct environment with slice field", func(t *testing.T) {
		type TagsEnv struct {
			Tags []string
		}

		extractEnv := func(r *http.Request) TagsEnv {
			tags := []string{"public"}
			if r.URL.Path == "/admin" {
				tags = []string{"admin", "protected"}
			}
			return TagsEnv{Tags: tags}
		}

		skipper := ExpressionSkipper(extractEnv, "'admin' in Tags")
		req := &http.Request{URL: &url.URL{Path: "/admin"}}

		got := skipper(req)
		assert.True(t, got)
	})
}

func TestExpressionSkipper_ErrorHandling(t *testing.T) {
	t.Run("runtime expression error", func(t *testing.T) {
		type Env struct {
			Value *int
		}

		extractEnv := func(r *http.Request) Env {
			return Env{Value: nil}
		}

		skipper := ExpressionSkipper(extractEnv, "Value > 10")
		req := &http.Request{URL: &url.URL{Path: "/test"}}

		got := skipper(req)
		assert.False(t, got)
	})
}

func TestExpressionSkipper_Parallel(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		method     string
		path       string
		want       bool
	}{
		{"GET request", "Method == 'GET'", http.MethodGet, "/api", true},
		{"POST request", "Method == 'POST'", http.MethodPost, "/api", true},
		{"wrong method", "Method == 'GET'", http.MethodPost, "/api", false},
		{"path check", "Path == '/health'", http.MethodGet, "/health", true},
		{"wrong path", "Path == '/health'", http.MethodGet, "/api", false},
	}

	type RequestEnv struct {
		Method string
		Path   string
	}

	extractEnv := func(r *http.Request) RequestEnv {
		return RequestEnv{
			Method: r.Method,
			Path:   r.URL.Path,
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			skipper := ExpressionSkipper(extractEnv, tt.expression)
			req := &http.Request{Method: tt.method, URL: &url.URL{Path: tt.path}}

			got := skipper(req)
			assert.Equal(t, tt.want, got)
		})
	}
}

func createRequest(method, path string) *http.Request {
	return &http.Request{
		Method: method,
		URL:    &url.URL{Path: path},
	}
}
