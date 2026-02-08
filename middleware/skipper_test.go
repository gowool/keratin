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

func createRequest(method, path string) *http.Request {
	return &http.Request{
		Method: method,
		URL:    &url.URL{Path: path},
	}
}
