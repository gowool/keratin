package keratin

import (
	"crypto/tls"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    string
	}{
		{
			name:    "pattern without method",
			pattern: "/",
			want:    "/",
		},
		{
			name:    "pattern with GET method",
			pattern: "GET /",
			want:    "/",
		},
		{
			name:    "pattern with POST method",
			pattern: "POST /blog/posts",
			want:    "/blog/posts",
		},
		{
			name:    "pattern with PUT method",
			pattern: "PUT /api/users/123",
			want:    "/api/users/123",
		},
		{
			name:    "pattern with DELETE method",
			pattern: "DELETE /api/users/123",
			want:    "/api/users/123",
		},
		{
			name:    "pattern with PATCH method",
			pattern: "PATCH /api/users/123",
			want:    "/api/users/123",
		},
		{
			name:    "pattern with HEAD method",
			pattern: "HEAD /api/health",
			want:    "/api/health",
		},
		{
			name:    "pattern with OPTIONS method",
			pattern: "OPTIONS /api/cors",
			want:    "/api/cors",
		},
		{
			name:    "pattern with dynamic path",
			pattern: "GET /posts/{id}",
			want:    "/posts/{id}",
		},
		{
			name:    "pattern with multiple dynamic segments",
			pattern: "GET /posts/{year}/{month}/{slug}",
			want:    "/posts/{year}/{month}/{slug}",
		},
		{
			name:    "pattern with rest parameter",
			pattern: "GET /api/{...rest}",
			want:    "/api/{...rest}",
		},
		{
			name:    "pattern with lowercase method",
			pattern: "get /test",
			want:    "/test",
		},
		{
			name:    "pattern with mixed case method",
			pattern: "Post /test",
			want:    "/test",
		},
		{
			name:    "complex api path",
			pattern: "GET /api/v1/users/{id}/posts/{postId}",
			want:    "/api/v1/users/{id}/posts/{postId}",
		},
		{
			name:    "pattern with trailing slash",
			pattern: "GET /blog/posts/",
			want:    "/blog/posts/",
		},
		{
			name:    "pattern with query parameters in pattern",
			pattern: "GET /search?q=test",
			want:    "/search?q=test",
		},
		{
			name:    "empty pattern",
			pattern: "",
			want:    "",
		},
		{
			name:    "pattern with only method",
			pattern: "GET",
			want:    "GET",
		},
		{
			name:    "pattern with multiple spaces",
			pattern: "GET  /test/path",
			want:    " /test/path",
		},
		{
			name:    "pattern with method and space only",
			pattern: "GET ",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{Pattern: tt.pattern}
			got := Pattern(req)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestScheme(t *testing.T) {
	tests := []struct {
		name string
		req  *http.Request
		want string
	}{
		{
			name: "TLS connection returns https",
			req: &http.Request{
				TLS: &tls.ConnectionState{},
			},
			want: "https",
		},
		{
			name: "no TLS or headers returns http",
			req: &http.Request{
				Header: http.Header{},
			},
			want: "http",
		},
		{
			name: "X-Forwarded-Proto header with https",
			req: &http.Request{
				Header: http.Header{
					HeaderXForwardedProto: []string{"https"},
				},
			},
			want: "https",
		},
		{
			name: "X-Forwarded-Proto header with http",
			req: &http.Request{
				Header: http.Header{
					HeaderXForwardedProto: []string{"http"},
				},
			},
			want: "http",
		},
		{
			name: "X-Forwarded-Protocol header with https",
			req: &http.Request{
				Header: http.Header{
					HeaderXForwardedProtocol: []string{"https"},
				},
			},
			want: "https",
		},
		{
			name: "X-Forwarded-Protocol header with http",
			req: &http.Request{
				Header: http.Header{
					HeaderXForwardedProtocol: []string{"http"},
				},
			},
			want: "http",
		},
		{
			name: "X-Forwarded-Ssl header with on",
			req: &http.Request{
				Header: http.Header{
					HeaderXForwardedSsl: []string{"on"},
				},
			},
			want: "https",
		},
		{
			name: "X-Forwarded-Ssl header with off",
			req: &http.Request{
				Header: http.Header{
					HeaderXForwardedSsl: []string{"off"},
				},
			},
			want: "http",
		},
		{
			name: "X-Url-Scheme header with https",
			req: &http.Request{
				Header: http.Header{
					HeaderXUrlScheme: []string{"https"},
				},
			},
			want: "https",
		},
		{
			name: "X-Url-Scheme header with http",
			req: &http.Request{
				Header: http.Header{
					HeaderXUrlScheme: []string{"http"},
				},
			},
			want: "http",
		},
		{
			name: "TLS takes precedence over X-Forwarded-Proto",
			req: &http.Request{
				TLS: &tls.ConnectionState{},
				Header: http.Header{
					HeaderXForwardedProto: []string{"http"},
				},
			},
			want: "https",
		},
		{
			name: "X-Forwarded-Proto takes precedence over X-Forwarded-Protocol",
			req: &http.Request{
				Header: http.Header{
					HeaderXForwardedProto:    []string{"https"},
					HeaderXForwardedProtocol: []string{"http"},
				},
			},
			want: "https",
		},
		{
			name: "X-Forwarded-Protocol takes precedence over X-Forwarded-Ssl",
			req: &http.Request{
				Header: http.Header{
					HeaderXForwardedProtocol: []string{"https"},
					HeaderXForwardedSsl:      []string{"off"},
				},
			},
			want: "https",
		},
		{
			name: "X-Forwarded-Ssl takes precedence over X-Url-Scheme",
			req: &http.Request{
				Header: http.Header{
					HeaderXForwardedSsl: []string{"on"},
					HeaderXUrlScheme:    []string{"http"},
				},
			},
			want: "https",
		},
		{
			name: "empty X-Forwarded-Proto is ignored",
			req: &http.Request{
				Header: http.Header{
					HeaderXForwardedProto: []string{""},
				},
			},
			want: "http",
		},
		{
			name: "case sensitive header values",
			req: &http.Request{
				Header: http.Header{
					HeaderXForwardedProto: []string{"HTTPS"},
				},
			},
			want: "HTTPS",
		},
		{
			name: "nil request header map",
			req: &http.Request{
				Header: nil,
			},
			want: "http",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Scheme(tt.req)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseAcceptLanguage(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty header",
			input: "",
			want:  []string{},
		},
		{
			name:  "single language",
			input: "en-US",
			want:  []string{"en-US"},
		},
		{
			name:  "multiple languages",
			input: "en-US, fr-FR, de-DE",
			want:  []string{"en-US", "fr-FR", "de-DE"},
		},
		{
			name:  "language with quality value",
			input: "en-US;q=0.9",
			want:  []string{"en-US"},
		},
		{
			name:  "multiple languages with quality values",
			input: "en-US;q=0.9, fr-FR;q=0.8, de-DE;q=0.7",
			want:  []string{"en-US", "fr-FR", "de-DE"},
		},
		{
			name:  "mixed with and without quality values",
			input: "en-US, fr-FR;q=0.8, de-DE",
			want:  []string{"en-US", "fr-FR", "de-DE"},
		},
		{
			name:  "wildcard language",
			input: "*",
			want:  []string{"*"},
		},
		{
			name:  "wildcard with quality",
			input: "*;q=0.5",
			want:  []string{"*"},
		},
		{
			name:  "multiple parameters after semicolon",
			input: "en-US;q=0.9;level=1",
			want:  []string{"en-US"},
		},
		{
			name:  "whitespace handling",
			input: " en-US , fr-FR , de-DE ",
			want:  []string{"en-US", "fr-FR", "de-DE"},
		},
		{
			name:  "trailing comma",
			input: "en-US, fr-FR,",
			want:  []string{"en-US", "fr-FR", ""},
		},
		{
			name:  "leading comma",
			input: ",en-US, fr-FR",
			want:  []string{"", "en-US", "fr-FR"},
		},
		{
			name:  "empty language with semicolon",
			input: "en-US;, fr-FR",
			want:  []string{"en-US", "fr-FR"},
		},
		{
			name:  "language only code",
			input: "en, fr, de",
			want:  []string{"en", "fr", "de"},
		},
		{
			name:  "mixed formats",
			input: "en, en-US, fr, fr-FR",
			want:  []string{"en", "en-US", "fr", "fr-FR"},
		},
		{
			name:  "browser header example",
			input: "en-US,en;q=0.9",
			want:  []string{"en-US", "en"},
		},
		{
			name:  "complex browser header",
			input: "en-US,en;q=0.9,fr;q=0.8,de;q=0.7,es;q=0.6",
			want:  []string{"en-US", "en", "fr", "de", "es"},
		},
		{
			name:  "only whitespace",
			input: "   ",
			want:  []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseAcceptLanguage(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNegotiateFormat(t *testing.T) {
	tests := []struct {
		name         string
		acceptHeader string
		offered      []string
		want         string
	}{
		{
			name:         "exact match",
			acceptHeader: "application/json",
			offered:      []string{MIMEApplicationJSON, MIMETextHTML},
			want:         MIMEApplicationJSON,
		},
		{
			name:         "wildcard matches first offered",
			acceptHeader: "*/*",
			offered:      []string{MIMEApplicationJSON, MIMETextHTML},
			want:         MIMEApplicationJSON,
		},
		{
			name:         "empty accept header returns first offered",
			acceptHeader: "",
			offered:      []string{MIMEApplicationJSON, MIMETextHTML},
			want:         MIMEApplicationJSON,
		},
		{
			name:         "quality values are ignored",
			acceptHeader: "application/json;q=0.9, text/html;q=0.8",
			offered:      []string{MIMETextHTML, MIMEApplicationJSON},
			want:         MIMEApplicationJSON,
		},
		{
			name:         "no match returns empty string",
			acceptHeader: "text/xml",
			offered:      []string{MIMEApplicationJSON, MIMETextHTML},
			want:         "",
		},
		{
			name:         "browser header negotiation",
			acceptHeader: "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			offered:      []string{MIMEApplicationJSON, MIMETextHTML},
			want:         MIMETextHTML,
		},
		{
			name:         "API client prefers JSON",
			acceptHeader: "application/json, application/xml;q=0.9, */*;q=0.8",
			offered:      []string{MIMETextHTML, MIMEApplicationJSON, "application/xml"},
			want:         MIMEApplicationJSON,
		},
		{
			name:         "multiple formats with charset",
			acceptHeader: "application/json;charset=utf-8, text/html;charset=utf-8",
			offered:      []string{MIMEApplicationJSON, MIMETextHTML},
			want:         MIMEApplicationJSON,
		},
		{
			name:         "wildcard subtype",
			acceptHeader: "application/*",
			offered:      []string{MIMEApplicationJSON, MIMETextHTML},
			want:         MIMEApplicationJSON,
		},
		{
			name:         "single offered format",
			acceptHeader: "application/json",
			offered:      []string{MIMEApplicationJSON},
			want:         MIMEApplicationJSON,
		},
		{
			name:         "trailing comma",
			acceptHeader: "application/json,",
			offered:      []string{MIMEApplicationJSON},
			want:         MIMEApplicationJSON,
		},
		{
			name:         "empty format in list",
			acceptHeader: ",application/json",
			offered:      []string{MIMEApplicationJSON},
			want:         MIMEApplicationJSON,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NegotiateFormat(tt.acceptHeader, tt.offered...)
			assert.Equal(t, tt.want, got)
		})
	}
}
