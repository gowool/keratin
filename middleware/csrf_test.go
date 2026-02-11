package middleware

import (
	"cmp"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gowool/keratin"
	"github.com/stretchr/testify/assert"
)

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

func TestCSRF_tokenExtractors(t *testing.T) {
	var testCases = []struct {
		name              string
		whenTokenLookup   string
		whenCookieName    string
		givenCSRFCookie   string
		givenMethod       string
		givenQueryTokens  map[string][]string
		givenFormTokens   map[string][]string
		givenHeaderTokens map[string][]string
		expectError       string
		expectPanic       string
	}{
		{
			name:            "ok, multiple token lookups sources, succeeds on last one",
			whenTokenLookup: "header:X-CSRF-Token,form:csrf",
			givenCSRFCookie: "token",
			givenMethod:     http.MethodPost,
			givenHeaderTokens: map[string][]string{
				keratin.HeaderXCSRFToken: {"invalid_token"},
			},
			givenFormTokens: map[string][]string{
				"csrf": {"token"},
			},
		},
		{
			name:            "ok, token from POST form",
			whenTokenLookup: "form:csrf",
			givenCSRFCookie: "token",
			givenMethod:     http.MethodPost,
			givenFormTokens: map[string][]string{
				"csrf": {"token"},
			},
		},
		{
			name:            "ok, token from POST form, second token passes",
			whenTokenLookup: "form:csrf",
			givenCSRFCookie: "token",
			givenMethod:     http.MethodPost,
			givenFormTokens: map[string][]string{
				"csrf": {"invalid", "token"},
			},
			expectError: "code=403, message=invalid csrf token",
		},
		{
			name:            "nok, invalid token from POST form",
			whenTokenLookup: "form:csrf",
			givenCSRFCookie: "token",
			givenMethod:     http.MethodPost,
			givenFormTokens: map[string][]string{
				"csrf": {"invalid_token"},
			},
			expectError: "code=403, message=invalid csrf token",
		},
		{
			name:            "nok, missing token from POST form",
			whenTokenLookup: "form:csrf",
			givenCSRFCookie: "token",
			givenMethod:     http.MethodPost,
			givenFormTokens: map[string][]string{},
			expectError:     "code=400, message=Bad Request, err=missing value in the form",
		},
		{
			name:            "ok, token from POST header",
			whenTokenLookup: "", // will use defaults
			givenCSRFCookie: "token",
			givenMethod:     http.MethodPost,
			givenHeaderTokens: map[string][]string{
				keratin.HeaderXCSRFToken: {"token"},
			},
		},
		{
			name:            "nok, token from POST header, tokens limited to 1, second token would pass",
			whenTokenLookup: "header:" + keratin.HeaderXCSRFToken,
			givenCSRFCookie: "token",
			givenMethod:     http.MethodPost,
			givenHeaderTokens: map[string][]string{
				keratin.HeaderXCSRFToken: {"invalid", "token"},
			},
			expectError: "code=403, message=invalid csrf token",
		},
		{
			name:            "nok, invalid token from POST header",
			whenTokenLookup: "header:" + keratin.HeaderXCSRFToken,
			givenCSRFCookie: "token",
			givenMethod:     http.MethodPost,
			givenHeaderTokens: map[string][]string{
				keratin.HeaderXCSRFToken: {"invalid_token"},
			},
			expectError: "code=403, message=invalid csrf token",
		},
		{
			name:              "nok, missing token from POST header",
			whenTokenLookup:   "header:" + keratin.HeaderXCSRFToken,
			givenCSRFCookie:   "token",
			givenMethod:       http.MethodPost,
			givenHeaderTokens: map[string][]string{},
			expectError:       "code=400, message=Bad Request, err=missing value in request header",
		},
		{
			name:            "ok, token from PUT query param",
			whenTokenLookup: "query:csrf-param",
			givenCSRFCookie: "token",
			givenMethod:     http.MethodPut,
			givenQueryTokens: map[string][]string{
				"csrf-param": {"token"},
			},
		},
		{
			name:            "nok, token from PUT query form, second token would pass",
			whenTokenLookup: "query:csrf",
			givenCSRFCookie: "token",
			givenMethod:     http.MethodPut,
			givenQueryTokens: map[string][]string{
				"csrf": {"invalid", "token"},
			},
			expectError: "code=403, message=invalid csrf token",
		},
		{
			name:            "nok, invalid token from PUT query form",
			whenTokenLookup: "query:csrf",
			givenCSRFCookie: "token",
			givenMethod:     http.MethodPut,
			givenQueryTokens: map[string][]string{
				"csrf": {"invalid_token"},
			},
			expectError: "code=403, message=invalid csrf token",
		},
		{
			name:             "nok, missing token from PUT query form",
			whenTokenLookup:  "query:csrf",
			givenCSRFCookie:  "token",
			givenMethod:      http.MethodPut,
			givenQueryTokens: map[string][]string{},
			expectError:      "code=400, message=Bad Request, err=missing value in the query string",
		},
		{
			name:             "nok, invalid TokenLookup",
			whenTokenLookup:  "q",
			givenCSRFCookie:  "token",
			givenMethod:      http.MethodPut,
			givenQueryTokens: map[string][]string{},
			expectPanic:      "middleware: csrf: extractor source for lookup could not be split into needed parts: q",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			q := make(url.Values)
			for queryParam, values := range tc.givenQueryTokens {
				for _, v := range values {
					q.Add(queryParam, v)
				}
			}

			f := make(url.Values)
			for formKey, values := range tc.givenFormTokens {
				for _, v := range values {
					f.Add(formKey, v)
				}
			}

			var req *http.Request
			switch tc.givenMethod {
			case http.MethodGet:
				req = httptest.NewRequest(http.MethodGet, "/?"+q.Encode(), nil)
			case http.MethodPost, http.MethodPut:
				req = httptest.NewRequest(http.MethodPost, "/?"+q.Encode(), strings.NewReader(f.Encode()))
				req.Header.Add(keratin.HeaderContentType, keratin.MIMEApplicationForm)
			}

			for header, values := range tc.givenHeaderTokens {
				for _, v := range values {
					req.Header.Add(header, v)
				}
			}

			if tc.givenCSRFCookie != "" {
				req.Header.Set(keratin.HeaderCookie, "_csrf="+tc.givenCSRFCookie)
			}

			rec := httptest.NewRecorder()

			config := CSRFConfig{
				TokenLookup: tc.whenTokenLookup,
				CookieName:  tc.whenCookieName,
			}
			if tc.expectPanic != "" {
				assert.PanicsWithError(t, tc.expectPanic, func() {
					CSRF(config)
				})
				return
			}

			csrf := CSRF(config)
			h := csrf(keratin.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) error {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("test"))
				return nil
			}))

			err := h.ServeHTTP(rec, req)
			if tc.expectError != "" {
				assert.EqualError(t, err, tc.expectError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCSRFWithConfig(t *testing.T) {
	token := randomString(16)

	var testCases = []struct {
		name                 string
		givenConfig          *CSRFConfig
		whenMethod           string
		whenHeaders          map[string]string
		expectEmptyBody      bool
		expectPanic          string
		expectCookieContains string
		expectErr            string
	}{
		{
			name:                 "ok, GET",
			whenMethod:           http.MethodGet,
			expectCookieContains: "_csrf",
		},
		{
			name: "ok, POST valid token",
			whenHeaders: map[string]string{
				keratin.HeaderCookie:     "_csrf=" + token,
				keratin.HeaderXCSRFToken: token,
			},
			whenMethod:           http.MethodPost,
			expectCookieContains: "_csrf",
		},
		{
			name:            "nok, POST without token",
			whenMethod:      http.MethodPost,
			expectEmptyBody: true,
			expectErr:       `code=400, message=Bad Request, err=missing value in request header`,
		},
		{
			name:            "nok, POST empty token",
			whenHeaders:     map[string]string{keratin.HeaderXCSRFToken: ""},
			whenMethod:      http.MethodPost,
			expectEmptyBody: true,
			expectErr:       `code=403, message=invalid csrf token`,
		},
		{
			name: "nok, invalid trusted origin in Config",
			givenConfig: &CSRFConfig{
				TrustedOrigins: []string{"http://example.com", "invalid"},
			},
			expectPanic: `middleware: csrf: trusted origin is missing scheme or host: invalid`,
		},
		{
			name: "ok, TokenLength",
			givenConfig: &CSRFConfig{
				TokenLength: 16,
			},
			whenMethod:           http.MethodGet,
			expectCookieContains: "_csrf",
		},
		{
			name: "ok, unsafe method + SecFetchSite=same-origin passes",
			whenHeaders: map[string]string{
				keratin.HeaderSecFetchSite: "same-origin",
			},
			whenMethod: http.MethodPost,
		},
		{
			name: "nok, unsafe method + SecFetchSite=same-cross blocked",
			whenHeaders: map[string]string{
				keratin.HeaderSecFetchSite: "same-cross",
			},
			whenMethod:      http.MethodPost,
			expectEmptyBody: true,
			expectErr:       `code=403, message=cross-site request blocked by CSRF`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(cmp.Or(tc.whenMethod, http.MethodPost), "/", nil)
			rec := httptest.NewRecorder()

			for key, value := range tc.whenHeaders {
				req.Header.Set(key, value)
			}

			var config CSRFConfig
			if tc.givenConfig != nil {
				config = *tc.givenConfig
			}
			if tc.expectPanic != "" {
				assert.PanicsWithError(t, tc.expectPanic, func() {
					CSRF(config)
				})
				return
			}

			mw := CSRF(config)
			h := mw(keratin.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) error {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("test"))
				return nil
			}))

			err := h.ServeHTTP(rec, req)
			if tc.expectErr != "" {
				assert.EqualError(t, err, tc.expectErr)
			} else {
				assert.NoError(t, err)
			}

			expect := "test"
			if tc.expectEmptyBody {
				expect = ""
			}
			assert.Equal(t, expect, rec.Body.String())

			if tc.expectCookieContains != "" {
				assert.Contains(t, rec.Header().Get(keratin.HeaderSetCookie), tc.expectCookieContains)
			}
		})
	}
}

func TestCSRF(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	csrf := CSRF(CSRFConfig{})
	h := csrf(keratin.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test"))
		return nil
	}))

	// Generate CSRF token
	_ = h.ServeHTTP(rec, req)
	assert.Contains(t, rec.Header().Get(keratin.HeaderSetCookie), "_csrf")

}

func TestCSRFSetSameSiteMode(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	csrf := CSRF(CSRFConfig{
		CookieSameSite: http.SameSiteStrictMode,
	})

	h := csrf(keratin.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test"))
		return nil
	}))

	r := h.ServeHTTP(rec, req)
	assert.NoError(t, r)
	assert.Regexp(t, "SameSite=Strict", rec.Header()["Set-Cookie"])
}

func TestCSRFWithoutSameSiteMode(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	csrf := CSRF(CSRFConfig{})

	h := csrf(keratin.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test"))
		return nil
	}))

	r := h.ServeHTTP(rec, req)
	assert.NoError(t, r)
	assert.NotRegexp(t, "SameSite=", rec.Header()["Set-Cookie"])
}

func TestCSRFWithSameSiteDefaultMode(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	csrf := CSRF(CSRFConfig{
		CookieSameSite: http.SameSiteDefaultMode,
	})

	h := csrf(keratin.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test"))
		return nil
	}))

	r := h.ServeHTTP(rec, req)
	assert.NoError(t, r)
	assert.NotRegexp(t, "SameSite=", rec.Header()["Set-Cookie"])
}

func TestCSRFWithSameSiteModeNone(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	csrf := CSRF(CSRFConfig{
		CookieSameSite: http.SameSiteNoneMode,
		CookieSecure:   true,
	})

	h := csrf(keratin.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test"))
		return nil
	}))

	r := h.ServeHTTP(rec, req)
	assert.NoError(t, r)
	assert.Regexp(t, "SameSite=None", rec.Header()["Set-Cookie"])
	assert.Regexp(t, "Secure", rec.Header()["Set-Cookie"])
}

func TestCSRFConfig_skipper(t *testing.T) {
	var testCases = []struct {
		name          string
		whenSkip      bool
		expectCookies int
	}{
		{
			name:          "do skip",
			whenSkip:      true,
			expectCookies: 0,
		},
		{
			name:          "do not skip",
			whenSkip:      false,
			expectCookies: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()

			csrf := CSRF(CSRFConfig{}, func(*http.Request) bool {
				return tc.whenSkip
			})

			h := csrf(keratin.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) error {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("test"))
				return nil
			}))

			r := h.ServeHTTP(rec, req)
			assert.NoError(t, r)
			cookie := rec.Header()["Set-Cookie"]
			assert.Len(t, cookie, tc.expectCookies)
		})
	}
}

func TestCSRFErrorHandling(t *testing.T) {
	cfg := CSRFConfig{
		ErrorHandler: func(_ *http.Request, err error) error {
			return keratin.NewHTTPError(http.StatusTeapot, "error_handler_executed")
		},
	}

	router := keratin.NewRouter()
	router.POST("/{$}", func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte("should not end up here"))
		return nil
	})
	router.UseFunc(CSRF(cfg))

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(keratin.HeaderAccept, keratin.MIMEApplicationJSON)
	res := httptest.NewRecorder()
	router.Build().ServeHTTP(res, req)

	assert.Equal(t, http.StatusTeapot, res.Code)
	assert.Equal(t, "{\"message\":\"error_handler_executed\"}\n", res.Body.String())
}

func TestCSRFConfig_checkSecFetchSiteRequest(t *testing.T) {
	var testCases = []struct {
		name             string
		givenConfig      CSRFConfig
		whenMethod       string
		whenSecFetchSite string
		whenOrigin       string
		expectAllow      bool
		expectErr        string
	}{
		{
			name:             "ok, unsafe POST, no SecFetchSite is not blocked",
			givenConfig:      CSRFConfig{},
			whenMethod:       http.MethodPost,
			whenSecFetchSite: "",
			expectAllow:      false, // should fall back to token CSRF
		},
		{
			name:             "ok, safe GET + same-origin passes",
			givenConfig:      CSRFConfig{},
			whenMethod:       http.MethodGet,
			whenSecFetchSite: "same-origin",
			expectAllow:      true,
		},
		{
			name:             "ok, safe GET + none passes",
			givenConfig:      CSRFConfig{},
			whenMethod:       http.MethodGet,
			whenSecFetchSite: "none",
			expectAllow:      true,
		},
		{
			name:             "ok, safe GET + same-site passes",
			givenConfig:      CSRFConfig{},
			whenMethod:       http.MethodGet,
			whenSecFetchSite: "same-site",
			expectAllow:      true,
		},
		{
			name:             "ok, safe GET + cross-site passes",
			givenConfig:      CSRFConfig{},
			whenMethod:       http.MethodGet,
			whenSecFetchSite: "cross-site",
			expectAllow:      true,
		},
		{
			name:             "nok, unsafe POST + cross-site is blocked",
			givenConfig:      CSRFConfig{},
			whenMethod:       http.MethodPost,
			whenSecFetchSite: "cross-site",
			expectAllow:      false,
			expectErr:        `code=403, message=cross-site request blocked by CSRF`,
		},
		{
			name:             "nok, unsafe POST + same-site is blocked",
			givenConfig:      CSRFConfig{},
			whenMethod:       http.MethodPost,
			whenSecFetchSite: "same-site",
			expectAllow:      false,
			expectErr:        `code=403, message=same-site request blocked by CSRF`,
		},
		{
			name:             "ok, unsafe POST + same-origin passes",
			givenConfig:      CSRFConfig{},
			whenMethod:       http.MethodPost,
			whenSecFetchSite: "same-origin",
			expectAllow:      true,
		},
		{
			name:             "ok, unsafe POST + none passes",
			givenConfig:      CSRFConfig{},
			whenMethod:       http.MethodPost,
			whenSecFetchSite: "none",
			expectAllow:      true,
		},
		{
			name:             "ok, unsafe PUT + same-origin passes",
			givenConfig:      CSRFConfig{},
			whenMethod:       http.MethodPut,
			whenSecFetchSite: "same-origin",
			expectAllow:      true,
		},
		{
			name:             "ok, unsafe PUT + none passes",
			givenConfig:      CSRFConfig{},
			whenMethod:       http.MethodPut,
			whenSecFetchSite: "none",
			expectAllow:      true,
		},
		{
			name:             "ok, unsafe DELETE + same-origin passes",
			givenConfig:      CSRFConfig{},
			whenMethod:       http.MethodDelete,
			whenSecFetchSite: "same-origin",
			expectAllow:      true,
		},
		{
			name:             "ok, unsafe PATCH + same-origin passes",
			givenConfig:      CSRFConfig{},
			whenMethod:       http.MethodPatch,
			whenSecFetchSite: "same-origin",
			expectAllow:      true,
		},
		{
			name:             "nok, unsafe PUT + cross-site is blocked",
			givenConfig:      CSRFConfig{},
			whenMethod:       http.MethodPut,
			whenSecFetchSite: "cross-site",
			expectAllow:      false,
			expectErr:        `code=403, message=cross-site request blocked by CSRF`,
		},
		{
			name:             "nok, unsafe PUT + same-site is blocked",
			givenConfig:      CSRFConfig{},
			whenMethod:       http.MethodPut,
			whenSecFetchSite: "same-site",
			expectAllow:      false,
			expectErr:        `code=403, message=same-site request blocked by CSRF`,
		},
		{
			name:             "nok, unsafe DELETE + cross-site is blocked",
			givenConfig:      CSRFConfig{},
			whenMethod:       http.MethodDelete,
			whenSecFetchSite: "cross-site",
			expectAllow:      false,
			expectErr:        `code=403, message=cross-site request blocked by CSRF`,
		},
		{
			name:             "nok, unsafe DELETE + same-site is blocked",
			givenConfig:      CSRFConfig{},
			whenMethod:       http.MethodDelete,
			whenSecFetchSite: "same-site",
			expectAllow:      false,
			expectErr:        `code=403, message=same-site request blocked by CSRF`,
		},
		{
			name:             "nok, unsafe PATCH + cross-site is blocked",
			givenConfig:      CSRFConfig{},
			whenMethod:       http.MethodPatch,
			whenSecFetchSite: "cross-site",
			expectAllow:      false,
			expectErr:        `code=403, message=cross-site request blocked by CSRF`,
		},
		{
			name:             "ok, safe HEAD + same-origin passes",
			givenConfig:      CSRFConfig{},
			whenMethod:       http.MethodHead,
			whenSecFetchSite: "same-origin",
			expectAllow:      true,
		},
		{
			name:             "ok, safe HEAD + cross-site passes",
			givenConfig:      CSRFConfig{},
			whenMethod:       http.MethodHead,
			whenSecFetchSite: "cross-site",
			expectAllow:      true,
		},
		{
			name:             "ok, safe OPTIONS + cross-site passes",
			givenConfig:      CSRFConfig{},
			whenMethod:       http.MethodOptions,
			whenSecFetchSite: "cross-site",
			expectAllow:      true,
		},
		{
			name:             "ok, safe TRACE + cross-site passes",
			givenConfig:      CSRFConfig{},
			whenMethod:       http.MethodTrace,
			whenSecFetchSite: "cross-site",
			expectAllow:      true,
		},
		{
			name: "ok, unsafe POST + cross-site + matching trusted origin passes",
			givenConfig: CSRFConfig{
				TrustedOrigins: []string{"https://trusted.example.com"},
			},
			whenMethod:       http.MethodPost,
			whenSecFetchSite: "cross-site",
			whenOrigin:       "https://trusted.example.com",
			expectAllow:      true,
		},
		{
			name: "ok, unsafe POST + same-site + matching trusted origin passes",
			givenConfig: CSRFConfig{
				TrustedOrigins: []string{"https://trusted.example.com"},
			},
			whenMethod:       http.MethodPost,
			whenSecFetchSite: "same-site",
			whenOrigin:       "https://trusted.example.com",
			expectAllow:      true,
		},
		{
			name: "nok, unsafe POST + cross-site + non-matching origin is blocked",
			givenConfig: CSRFConfig{
				TrustedOrigins: []string{"https://trusted.example.com"},
			},
			whenMethod:       http.MethodPost,
			whenSecFetchSite: "cross-site",
			whenOrigin:       "https://evil.example.com",
			expectAllow:      false,
			expectErr:        `code=403, message=cross-site request blocked by CSRF`,
		},
		{
			name: "ok, unsafe POST + cross-site + case-insensitive trusted origin match passes",
			givenConfig: CSRFConfig{
				TrustedOrigins: []string{"https://trusted.example.com"},
			},
			whenMethod:       http.MethodPost,
			whenSecFetchSite: "cross-site",
			whenOrigin:       "https://TRUSTED.example.com",
			expectAllow:      true,
		},
		{
			name: "ok, unsafe POST + same-origin + trusted origins configured but not matched passes",
			givenConfig: CSRFConfig{
				TrustedOrigins: []string{"https://trusted.example.com"},
			},
			whenMethod:       http.MethodPost,
			whenSecFetchSite: "same-origin",
			whenOrigin:       "https://different.example.com",
			expectAllow:      true,
		},
		{
			name: "nok, unsafe POST + cross-site + empty origin + trusted origins configured is blocked",
			givenConfig: CSRFConfig{
				TrustedOrigins: []string{"https://trusted.example.com"},
			},
			whenMethod:       http.MethodPost,
			whenSecFetchSite: "cross-site",
			whenOrigin:       "",
			expectAllow:      false,
			expectErr:        `code=403, message=cross-site request blocked by CSRF`,
		},
		{
			name: "ok, unsafe POST + cross-site + multiple trusted origins, second one matches",
			givenConfig: CSRFConfig{
				TrustedOrigins: []string{"https://first.example.com", "https://second.example.com"},
			},
			whenMethod:       http.MethodPost,
			whenSecFetchSite: "cross-site",
			whenOrigin:       "https://second.example.com",
			expectAllow:      true,
		},
		{
			name: "ok, unsafe POST + same-site + custom func allows",
			givenConfig: CSRFConfig{
				AllowSecFetchSiteFunc: func(*http.Request) (bool, error) {
					return true, nil
				},
			},
			whenMethod:       http.MethodPost,
			whenSecFetchSite: "same-site",
			expectAllow:      true,
		},
		{
			name: "ok, unsafe POST + cross-site + custom func allows",
			givenConfig: CSRFConfig{
				AllowSecFetchSiteFunc: func(*http.Request) (bool, error) {
					return true, nil
				},
			},
			whenMethod:       http.MethodPost,
			whenSecFetchSite: "cross-site",
			expectAllow:      true,
		},
		{
			name: "nok, unsafe POST + same-site + custom func returns custom error",
			givenConfig: CSRFConfig{
				AllowSecFetchSiteFunc: func(*http.Request) (bool, error) {
					return false, keratin.NewHTTPError(http.StatusTeapot, "custom error from func")
				},
			},
			whenMethod:       http.MethodPost,
			whenSecFetchSite: "same-site",
			expectAllow:      false,
			expectErr:        `code=418, message=custom error from func`,
		},
		{
			name: "nok, unsafe POST + cross-site + custom func returns false with nil error",
			givenConfig: CSRFConfig{
				AllowSecFetchSiteFunc: func(*http.Request) (bool, error) {
					return false, nil
				},
			},
			whenMethod:       http.MethodPost,
			whenSecFetchSite: "cross-site",
			expectAllow:      false,
			expectErr:        "", // custom func returns nil error, so no error expected
		},
		{
			name:             "nok, unsafe POST + invalid Sec-Fetch-Site value treated as cross-site",
			givenConfig:      CSRFConfig{},
			whenMethod:       http.MethodPost,
			whenSecFetchSite: "invalid-value",
			expectAllow:      false,
			expectErr:        `code=403, message=cross-site request blocked by CSRF`,
		},
		{
			name: "ok, unsafe POST + cross-site + trusted origin takes precedence over custom func",
			givenConfig: CSRFConfig{
				TrustedOrigins: []string{"https://trusted.example.com"},
				AllowSecFetchSiteFunc: func(*http.Request) (bool, error) {
					return false, keratin.NewHTTPError(http.StatusTeapot, "should not be called")
				},
			},
			whenMethod:       http.MethodPost,
			whenSecFetchSite: "cross-site",
			whenOrigin:       "https://trusted.example.com",
			expectAllow:      true,
		},
		{
			name: "nok, unsafe POST + cross-site + trusted origin not matched, custom func blocks",
			givenConfig: CSRFConfig{
				TrustedOrigins: []string{"https://trusted.example.com"},
				AllowSecFetchSiteFunc: func(*http.Request) (bool, error) {
					return false, keratin.NewHTTPError(http.StatusTeapot, "custom block")
				},
			},
			whenMethod:       http.MethodPost,
			whenSecFetchSite: "cross-site",
			whenOrigin:       "https://evil.example.com",
			expectAllow:      false,
			expectErr:        `code=418, message=custom block`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.whenMethod, "/", nil)
			if tc.whenSecFetchSite != "" {
				req.Header.Set(keratin.HeaderSecFetchSite, tc.whenSecFetchSite)
			}
			if tc.whenOrigin != "" {
				req.Header.Set(keratin.HeaderOrigin, tc.whenOrigin)
			}

			allow, err := tc.givenConfig.checkSecFetchSiteRequest(req)

			assert.Equal(t, tc.expectAllow, allow)
			if tc.expectErr != "" {
				assert.EqualError(t, err, tc.expectErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
