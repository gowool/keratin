package middleware

// Copied from https://github.com/labstack/echo to avoid nuances around the specific
//
// -------------------------------------------------------------------
// The MIT License (MIT)
//
// Copyright (c) 2022 LabStack
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
// -------------------------------------------------------------------

import (
	"cmp"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gowool/keratin"
	"github.com/stretchr/testify/assert"
)

func TestCORS(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/", nil) // Preflight request
	req.Header.Set(keratin.HeaderOrigin, "http://example.com")
	rec := httptest.NewRecorder()

	mw := CORS(CORSConfig{AllowOrigins: []string{"*"}})
	handler := mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "*", rec.Header().Get(keratin.HeaderAccessControlAllowOrigin))
}

func TestCORSConfig(t *testing.T) {
	var testCases = []struct {
		name             string
		givenConfig      CORSConfig
		skippers         []Skipper
		whenMethod       string
		whenHeaders      map[string]string
		expectHeaders    map[string]string
		notExpectHeaders map[string]string
		expectErr        string
	}{
		{
			name: "ok, wildcard origin",
			givenConfig: CORSConfig{
				AllowOrigins: []string{"*"},
			},
			whenHeaders:   map[string]string{keratin.HeaderOrigin: "localhost"},
			expectHeaders: map[string]string{keratin.HeaderAccessControlAllowOrigin: "*"},
		},
		{
			name: "ok, wildcard AllowedOrigin with no Origin header in request",
			givenConfig: CORSConfig{
				AllowOrigins: []string{"*"},
			},
			notExpectHeaders: map[string]string{keratin.HeaderAccessControlAllowOrigin: ""},
		},
		{
			name: "ok, specific AllowOrigins and AllowCredentials",
			givenConfig: CORSConfig{
				AllowOrigins:     []string{"http://localhost", "http://localhost:8080"},
				AllowCredentials: true,
				MaxAge:           3600,
			},
			whenHeaders: map[string]string{keratin.HeaderOrigin: "http://localhost"},
			expectHeaders: map[string]string{
				keratin.HeaderAccessControlAllowOrigin:      "http://localhost",
				keratin.HeaderAccessControlAllowCredentials: "true",
			},
		},
		{
			name: "ok, preflight request with matching origin for `AllowOrigins`",
			givenConfig: CORSConfig{
				AllowOrigins:     []string{"http://localhost"},
				AllowCredentials: true,
				MaxAge:           3600,
			},
			whenMethod: http.MethodOptions,
			whenHeaders: map[string]string{
				keratin.HeaderOrigin:      "http://localhost",
				keratin.HeaderContentType: keratin.MIMEApplicationJSON,
			},
			expectHeaders: map[string]string{
				keratin.HeaderAccessControlAllowOrigin:      "http://localhost",
				keratin.HeaderAccessControlAllowMethods:     "GET,HEAD,POST,PUT,PATCH,DELETE,OPTIONS",
				keratin.HeaderAccessControlAllowCredentials: "true",
				keratin.HeaderAccessControlMaxAge:           "3600",
			},
		},
		{
			name: "ok, preflight request when `Access-Control-Max-Age` is set",
			givenConfig: CORSConfig{
				AllowOrigins:     []string{"http://localhost"},
				AllowCredentials: true,
				MaxAge:           1,
			},
			whenMethod: http.MethodOptions,
			whenHeaders: map[string]string{
				keratin.HeaderOrigin:      "http://localhost",
				keratin.HeaderContentType: keratin.MIMEApplicationJSON,
			},
			expectHeaders: map[string]string{
				keratin.HeaderAccessControlMaxAge: "1",
			},
		},
		{
			name: "ok, preflight request when `Access-Control-Max-Age` is set to 0 - not to cache response",
			givenConfig: CORSConfig{
				AllowOrigins:     []string{"http://localhost"},
				AllowCredentials: true,
				MaxAge:           -1, // forces `Access-Control-Max-Age: 0`
			},
			whenMethod: http.MethodOptions,
			whenHeaders: map[string]string{
				keratin.HeaderOrigin:      "http://localhost",
				keratin.HeaderContentType: keratin.MIMEApplicationJSON,
			},
			expectHeaders: map[string]string{
				keratin.HeaderAccessControlMaxAge: "0",
			},
		},
		{
			name: "ok, CORS check are skipped",
			givenConfig: CORSConfig{
				AllowOrigins:     []string{"http://localhost"},
				AllowCredentials: true,
			},
			skippers: []Skipper{func(*http.Request) bool {
				return true
			}},
			whenMethod: http.MethodOptions,
			whenHeaders: map[string]string{
				keratin.HeaderOrigin:      "http://localhost",
				keratin.HeaderContentType: keratin.MIMEApplicationJSON,
			},
			notExpectHeaders: map[string]string{
				keratin.HeaderAccessControlAllowOrigin:      "localhost",
				keratin.HeaderAccessControlAllowMethods:     "GET,HEAD,POST,PUT,PATCH,DELETE,OPTIONS",
				keratin.HeaderAccessControlAllowCredentials: "true",
				keratin.HeaderAccessControlMaxAge:           "3600",
			},
		},
		{
			name: "nok, preflight request with wildcard `AllowOrigins` and `AllowCredentials` true",
			givenConfig: CORSConfig{
				AllowOrigins:     []string{"*"},
				AllowCredentials: true,
				MaxAge:           3600,
			},
			whenMethod: http.MethodOptions,
			whenHeaders: map[string]string{
				keratin.HeaderOrigin:      "localhost",
				keratin.HeaderContentType: keratin.MIMEApplicationJSON,
			},
			expectErr: `middleware: cors: * as allowed origin and AllowCredentials=true is insecure and not allowed. Use custom UnsafeAllowOriginFunc`,
		},
		{
			name: "nok, preflight request with invalid `AllowOrigins` value",
			givenConfig: CORSConfig{
				AllowOrigins: []string{"http://server", "missing-scheme"},
			},
			expectErr: `middleware: cors: allow origin is missing scheme or host: missing-scheme`,
		},
		{
			name: "ok, preflight request with wildcard `AllowOrigins` and `AllowCredentials` false",
			givenConfig: CORSConfig{
				AllowOrigins:     []string{"*"},
				AllowCredentials: false, // important for this testcase
				MaxAge:           3600,
			},
			whenMethod: http.MethodOptions,
			whenHeaders: map[string]string{
				keratin.HeaderOrigin:      "localhost",
				keratin.HeaderContentType: keratin.MIMEApplicationJSON,
			},
			expectHeaders: map[string]string{
				keratin.HeaderAccessControlAllowOrigin:  "*",
				keratin.HeaderAccessControlAllowMethods: "GET,HEAD,POST,PUT,PATCH,DELETE,OPTIONS",
				keratin.HeaderAccessControlMaxAge:       "3600",
			},
			notExpectHeaders: map[string]string{
				keratin.HeaderAccessControlAllowCredentials: "",
			},
		},
		{
			name: "ok, INSECURE preflight request with wildcard `AllowOrigins` and `AllowCredentials` true",
			givenConfig: CORSConfig{
				AllowOrigins:     []string{"*"},
				AllowCredentials: true,
				MaxAge:           3600,
			},
			whenMethod: http.MethodOptions,
			whenHeaders: map[string]string{
				keratin.HeaderOrigin:      "localhost",
				keratin.HeaderContentType: keratin.MIMEApplicationJSON,
			},
			expectErr: `middleware: cors: * as allowed origin and AllowCredentials=true is insecure and not allowed. Use custom UnsafeAllowOriginFunc`,
		},
		{
			name: "ok, preflight request with Access-Control-Request-Headers",
			givenConfig: CORSConfig{
				AllowOrigins: []string{"*"},
			},
			whenMethod: http.MethodOptions,
			whenHeaders: map[string]string{
				keratin.HeaderOrigin:                      "localhost",
				keratin.HeaderContentType:                 keratin.MIMEApplicationJSON,
				keratin.HeaderAccessControlRequestHeaders: "Special-Request-Header",
			},
			expectHeaders: map[string]string{
				keratin.HeaderAccessControlAllowOrigin:  "*",
				keratin.HeaderAccessControlAllowHeaders: "Special-Request-Header",
				keratin.HeaderAccessControlAllowMethods: "GET,HEAD,POST,PUT,PATCH,DELETE,OPTIONS",
			},
		},
		{
			name: "ok, preflight request with `AllowOrigins` which allow all subdomains aaa with *",
			givenConfig: CORSConfig{
				UnsafeAllowOriginFunc: func(_ *http.Request, origin string) (allowedOrigin string, allowed bool, err error) {
					if strings.HasSuffix(origin, ".example.com") {
						allowed = true
					}
					return origin, allowed, nil
				},
			},
			whenMethod:    http.MethodOptions,
			whenHeaders:   map[string]string{keratin.HeaderOrigin: "http://aaa.example.com"},
			expectHeaders: map[string]string{keratin.HeaderAccessControlAllowOrigin: "http://aaa.example.com"},
		},
		{
			name: "ok, preflight request with `AllowOrigins` which allow all subdomains bbb with *",
			givenConfig: CORSConfig{
				UnsafeAllowOriginFunc: func(_ *http.Request, origin string) (string, bool, error) {
					if strings.HasSuffix(origin, ".example.com") {
						return origin, true, nil
					}
					return "", false, nil
				},
			},
			whenMethod:    http.MethodOptions,
			whenHeaders:   map[string]string{keratin.HeaderOrigin: "http://bbb.example.com"},
			expectHeaders: map[string]string{keratin.HeaderAccessControlAllowOrigin: "http://bbb.example.com"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectErr != "" {
				assert.PanicsWithError(t, tc.expectErr, func() {
					CORS(tc.givenConfig, tc.skippers...)
				})
				return
			}

			mw := CORS(tc.givenConfig, tc.skippers...)
			h := mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

			method := cmp.Or(tc.whenMethod, http.MethodGet)
			req := httptest.NewRequest(method, "/", nil)
			rec := httptest.NewRecorder()
			for k, v := range tc.whenHeaders {
				req.Header.Set(k, v)
			}

			h.ServeHTTP(rec, req)

			header := rec.Header()
			for k, v := range tc.expectHeaders {
				assert.Equal(t, v, header.Get(k), "header: `%v` should be `%v`", k, v)
			}
			for k, v := range tc.notExpectHeaders {
				if v == "" {
					assert.Len(t, header.Values(k), 0, "header: `%v` should not be set", k)
				} else {
					assert.NotEqual(t, v, header.Get(k), "header: `%v` should not be `%v`", k, v)
				}
			}
		})
	}
}

func Test_AllowOriginScheme(t *testing.T) {
	tests := []struct {
		domain, pattern string
		expected        bool
	}{
		{
			domain:   "http://example.com",
			pattern:  "http://example.com",
			expected: true,
		},
		{
			domain:   "https://example.com",
			pattern:  "https://example.com",
			expected: true,
		},
		{
			domain:   "http://example.com",
			pattern:  "https://example.com",
			expected: false,
		},
		{
			domain:   "https://example.com",
			pattern:  "http://example.com",
			expected: false,
		},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		rec := httptest.NewRecorder()
		req.Header.Set(keratin.HeaderOrigin, tt.domain)

		mw := CORS(CORSConfig{
			AllowOrigins: []string{tt.pattern},
		})
		h := mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

		h.ServeHTTP(rec, req)

		if tt.expected {
			assert.Equal(t, tt.domain, rec.Header().Get(keratin.HeaderAccessControlAllowOrigin))
		} else {
			assert.NotContains(t, rec.Header(), keratin.HeaderAccessControlAllowOrigin)
		}
	}
}

func TestCorsHeaders(t *testing.T) {
	tests := []struct {
		name          string
		originDomain  string
		method        string
		allowedOrigin string
		expected      bool
		expectStatus  int
	}{
		{
			name:          "non-preflight request, allow any origin, missing origin header = no CORS logic done",
			originDomain:  "",
			allowedOrigin: "*",
			method:        http.MethodGet,
			expected:      false,
			expectStatus:  http.StatusOK,
		},
		{
			name:          "non-preflight request, allow any origin, specific origin domain",
			originDomain:  "http://example.com",
			allowedOrigin: "*",
			method:        http.MethodGet,
			expected:      true,
			expectStatus:  http.StatusOK,
		},
		{
			name:          "non-preflight request, allow specific origin, missing origin header = no CORS logic done",
			originDomain:  "", // Request does not have Origin header
			allowedOrigin: "http://example.com",
			method:        http.MethodGet,
			expected:      false,
			expectStatus:  http.StatusOK,
		},
		{
			name:          "non-preflight request, allow specific origin, different origin header = CORS logic failure",
			originDomain:  "http://bar.com",
			allowedOrigin: "http://example.com",
			method:        http.MethodGet,
			expected:      false,
			expectStatus:  http.StatusOK,
		},
		{
			name:          "non-preflight request, allow specific origin, matching origin header = CORS logic done",
			originDomain:  "http://example.com",
			allowedOrigin: "http://example.com",
			method:        http.MethodGet,
			expected:      true,
			expectStatus:  http.StatusOK,
		},
		{
			name:          "preflight, allow any origin, missing origin header = no CORS logic done",
			originDomain:  "", // Request does not have Origin header
			allowedOrigin: "*",
			method:        http.MethodOptions,
			expected:      false,
			expectStatus:  http.StatusNoContent,
		},
		{
			name:          "preflight, allow any origin, existing origin header = CORS logic done",
			originDomain:  "http://example.com",
			allowedOrigin: "*",
			method:        http.MethodOptions,
			expected:      true,
			expectStatus:  http.StatusNoContent,
		},
		{
			name:          "preflight, allow any origin, missing origin header = no CORS logic done",
			originDomain:  "", // Request does not have Origin header
			allowedOrigin: "http://example.com",
			method:        http.MethodOptions,
			expected:      false,
			expectStatus:  http.StatusNoContent,
		},
		{
			name:          "preflight, allow specific origin, different origin header = no CORS logic done",
			originDomain:  "http://bar.com",
			allowedOrigin: "http://example.com",
			method:        http.MethodOptions,
			expected:      false,
			expectStatus:  http.StatusNoContent,
		},
		{
			name:          "preflight, allow specific origin, matching origin header = CORS logic done",
			originDomain:  "http://example.com",
			allowedOrigin: "http://example.com",
			method:        http.MethodOptions,
			expected:      true,
			expectStatus:  http.StatusNoContent,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := keratin.NewRouter()

			router.PreHTTPFunc(CORS(CORSConfig{
				AllowOrigins: []string{tc.allowedOrigin},
			}))

			router.GET("/{$}", func(w http.ResponseWriter, _ *http.Request) error {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("OK"))
				return nil
			})
			router.POST("/{$}", func(w http.ResponseWriter, _ *http.Request) error {
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte("OK"))
				return nil
			})
			router.OPTIONS("/{$}", func(http.ResponseWriter, *http.Request) error {
				return keratin.ErrNotFound
			})

			req := httptest.NewRequest(tc.method, "/", nil)
			rec := httptest.NewRecorder()

			if tc.originDomain != "" {
				req.Header.Set(keratin.HeaderOrigin, tc.originDomain)
			}

			router.Build().ServeHTTP(rec, req)

			assert.Equal(t, keratin.HeaderOrigin, rec.Header().Get(keratin.HeaderVary))
			assert.Equal(t, tc.expectStatus, rec.Code)

			expectedAllowOrigin := ""
			if tc.allowedOrigin == "*" {
				expectedAllowOrigin = "*"
			} else {
				expectedAllowOrigin = tc.originDomain
			}
			switch {
			case tc.expected && tc.method == http.MethodOptions:
				assert.Contains(t, rec.Header(), keratin.HeaderAccessControlAllowMethods)
				assert.Equal(t, expectedAllowOrigin, rec.Header().Get(keratin.HeaderAccessControlAllowOrigin))

				assert.Equal(t, 3, len(rec.Header()[keratin.HeaderVary]))

			case tc.expected && tc.method == http.MethodGet:
				assert.Equal(t, expectedAllowOrigin, rec.Header().Get(keratin.HeaderAccessControlAllowOrigin))
				assert.Equal(t, 1, len(rec.Header()[keratin.HeaderVary])) // Vary: Origin
			default:
				assert.NotContains(t, rec.Header(), keratin.HeaderAccessControlAllowOrigin)
				assert.Equal(t, 1, len(rec.Header()[keratin.HeaderVary])) // Vary: Origin
			}
		})

	}
}

func Test_AllowOriginFunc(t *testing.T) {
	returnTrue := func(_ *http.Request, origin string) (string, bool, error) {
		return origin, true, nil
	}
	returnFalse := func(_ *http.Request, origin string) (string, bool, error) {
		return origin, false, nil
	}
	returnError := func(_ *http.Request, origin string) (string, bool, error) {
		return origin, true, errors.New("this is a test error")
	}

	allowOriginFuncs := []func(*http.Request, string) (string, bool, error){
		returnTrue,
		returnFalse,
		returnError,
	}

	const origin = "http://example.com"

	for _, allowOriginFunc := range allowOriginFuncs {
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		rec := httptest.NewRecorder()

		req.Header.Set(keratin.HeaderOrigin, origin)

		mw := CORS(CORSConfig{UnsafeAllowOriginFunc: allowOriginFunc})
		h := mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		h.ServeHTTP(rec, req)

		allowedOrigin, allowed, expectedErr := allowOriginFunc(req, origin)
		if expectedErr != nil {
			assert.Equal(t, http.StatusInternalServerError, rec.Code)
			assert.Equal(t, "", rec.Header().Get(keratin.HeaderAccessControlAllowOrigin))
			continue
		}

		if allowed {
			assert.Equal(t, allowedOrigin, rec.Header().Get(keratin.HeaderAccessControlAllowOrigin))
		} else {
			assert.Equal(t, "", rec.Header().Get(keratin.HeaderAccessControlAllowOrigin))
		}
	}
}
