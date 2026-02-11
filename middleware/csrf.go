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
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gowool/keratin"
)

type csrfKey = struct{}

func CtxCSRF(ctx context.Context) string {
	value, _ := ctx.Value(csrfKey{}).(string)
	return value
}

// ErrCSRFInvalid is returned when CSRF check fails
var ErrCSRFInvalid = keratin.NewHTTPError(http.StatusForbidden, "invalid csrf token")

type CSRFConfig struct {
	// TrustedOrigin permits any request with `Sec-Fetch-Site` header whose `Origin` header
	// exactly matches the specified value.
	// Values should be formated as Origin header "scheme://host[:port]".
	//
	// See [Origin]: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Origin
	// See [Sec-Fetch-Site]: https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html#fetch-metadata-headers
	TrustedOrigins []string `env:"TRUSTED_ORIGINS" json:"trustedOrigins,omitempty" yaml:"trustedOrigins,omitempty"`

	// AllowSecFetchSameSite allows custom behaviour for `Sec-Fetch-Site` requests that are about to
	// fail with CRSF error, to be allowed or replaced with custom error.
	// This function applies to `Sec-Fetch-Site` values:
	// - `same-site` 		same registrable domain (subdomain and/or different port)
	// - `cross-site`		request originates from different site
	// See [Sec-Fetch-Site]: https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html#fetch-metadata-headers
	AllowSecFetchSiteFunc func(r *http.Request) (bool, error) `json:"-" yaml:"-"`

	// TokenLength is the length of the generated token.
	TokenLength uint8 `env:"TOKEN_LENGTH" json:"tokenLength,omitempty" yaml:"tokenLength,omitempty"`
	// Optional. Default value 32.

	// TokenLookup is a string in the form of "<source>:<name>" or "<source>:<name>,<source>:<name>" that is used
	// to extract token from the request.
	// Optional. Default value "header:X-CSRF-Token".
	// Possible values:
	// - "header:<name>" or "header:<name>:<cut-prefix>"
	// - "query:<name>"
	// - "form:<name>"
	// Multiple sources example:
	// - "header:X-CSRF-Token,query:csrf"
	TokenLookup string `env:"TOKEN_LOOKUP" json:"tokenLookup,omitempty" yaml:"tokenLookup,omitempty"`

	// Generator defines a function to generate token.
	// Optional. Defaults tp randomString(TokenLength).
	Generator func() string `json:"-" yaml:"-"`

	// Name of the CSRF cookie. This cookie will store CSRF token.
	// Optional. Default value "csrf".
	CookieName string `env:"COOKIE_NAME" json:"cookieName,omitempty" yaml:"cookieName,omitempty"`

	// Domain of the CSRF cookie.
	// Optional. Default value none.
	CookieDomain string `env:"COOKIE_DOMAIN" json:"cookieDomain,omitempty" yaml:"cookieDomain,omitempty"`

	// Path of the CSRF cookie.
	// Optional. Default value none.
	CookiePath string `env:"COOKIE_PATH" json:"cookiePath,omitempty" yaml:"cookiePath,omitempty"`

	// Max age (in seconds) of the CSRF cookie.
	// Optional. Default value 86400 (24hr).
	CookieMaxAge int `env:"COOKIE_MAX_AGE" json:"cookieMaxAge,omitempty" yaml:"cookieMaxAge,omitempty"`

	// Indicates if CSRF cookie is secure.
	// Optional. Default value false.
	CookieSecure bool `env:"COOKIE_SECURE" json:"cookieSecure,omitempty" yaml:"cookieSecure,omitempty"`

	// Indicates if CSRF cookie is HTTP only.
	// Optional. Default value false.
	CookieHTTPOnly bool `env:"COOKIE_HTTP_ONLY" json:"cookieHTTPOnly,omitempty" yaml:"cookieHTTPOnly,omitempty"`

	// Indicates SameSite mode of the CSRF cookie.
	// Optional. Default value SameSiteDefaultMode.
	CookieSameSite http.SameSite `env:"COOKIE_SAME_SITE" json:"cookieSameSite,omitempty" yaml:"cookieSameSite,omitempty"`

	// ErrorHandler defines a function which is executed for returning custom errors.
	ErrorHandler func(r *http.Request, err error) error `json:"-" yaml:"-"`
}

func (c *CSRFConfig) SetDefaults() {
	if c.TokenLength == 0 {
		c.TokenLength = 32
	}
	if c.TokenLookup == "" {
		c.TokenLookup = "header:" + keratin.HeaderXCSRFToken
	}
	if c.CookieName == "" {
		c.CookieName = "_csrf"
	}
	if c.CookieMaxAge <= 0 {
		c.CookieMaxAge = 86400
	}
	if c.CookieSameSite <= 0 {
		c.CookieSameSite = http.SameSiteDefaultMode
	}
	if c.Generator == nil {
		c.Generator = createRandomStringGenerator(c.TokenLength)
	}
}

func CSRF(cfg CSRFConfig, skippers ...Skipper) func(keratin.Handler) keratin.Handler {
	cfg.SetDefaults()

	if len(cfg.TrustedOrigins) > 0 {
		if err := validateOrigins(cfg.TrustedOrigins, "trusted origin"); err != nil {
			panic(fmt.Errorf("middleware: csrf: %w", err))
		}
	}

	extractors, err := CreateExtractors(cfg.TokenLookup, 1)
	if err != nil {
		panic(fmt.Errorf("middleware: csrf: %w", err))
	}

	skip := ChainSkipper(skippers...)

	return func(next keratin.Handler) keratin.Handler {
		return keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			if skip(r) {
				return next.ServeHTTP(w, r)
			}

			// use the `Sec-Fetch-Site` header as part of a modern approach to CSRF protection
			allow, err := cfg.checkSecFetchSiteRequest(r)
			if err != nil {
				return err
			}
			if allow {
				return next.ServeHTTP(w, r)
			}

			// Fallback to legacy token based CSRF protection

			token := ""
			if k, err := r.Cookie(cfg.CookieName); err != nil {
				token = cfg.Generator() // Generate token
			} else {
				token = k.Value // Reuse token
			}

			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
			default:
				// Validate token only for requests which are not defined as 'safe' by RFC7231
				var lastExtractorErr error
				var lastTokenErr error
			outer:
				for _, extractor := range extractors {
					clientTokens, _, err := extractor(r)
					if err != nil {
						lastExtractorErr = err
						continue
					}

					for _, clientToken := range clientTokens {
						if validateCSRFToken(token, clientToken) {
							lastTokenErr = nil
							lastExtractorErr = nil
							break outer
						}
						lastTokenErr = ErrCSRFInvalid
					}
				}
				var finalErr error
				if lastTokenErr != nil {
					finalErr = lastTokenErr
				} else if lastExtractorErr != nil {
					finalErr = keratin.ErrBadRequest.Wrap(lastExtractorErr)
				}
				if finalErr != nil {
					if cfg.ErrorHandler != nil {
						return cfg.ErrorHandler(r, finalErr)
					}
					return finalErr
				}
			}

			// Set CSRF cookie
			cookie := new(http.Cookie)
			cookie.Name = cfg.CookieName
			cookie.Value = token
			if cfg.CookiePath != "" {
				cookie.Path = cfg.CookiePath
			}
			if cfg.CookieDomain != "" {
				cookie.Domain = cfg.CookieDomain
			}
			if cfg.CookieSameSite != http.SameSiteDefaultMode {
				cookie.SameSite = cfg.CookieSameSite
			}
			cookie.Expires = time.Now().Add(time.Duration(cfg.CookieMaxAge) * time.Second)
			cookie.Secure = cfg.CookieSecure
			cookie.HttpOnly = cfg.CookieHTTPOnly
			http.SetCookie(w, cookie)

			// Store token in the context
			ctx := context.WithValue(r.Context(), csrfKey{}, token)
			r = r.WithContext(ctx)

			// Protect clients from caching the response
			w.Header().Set(keratin.HeaderVary, keratin.HeaderCookie)

			return next.ServeHTTP(w, r)
		})
	}
}

func validateCSRFToken(token, clientToken string) bool {
	return subtle.ConstantTimeCompare([]byte(token), []byte(clientToken)) == 1
}

var safeMethods = []string{http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace}

func (c *CSRFConfig) checkSecFetchSiteRequest(r *http.Request) (bool, error) {
	// https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html#fetch-metadata-headers
	// Sec-Fetch-Site values are:
	// - `same-origin` 	exact origin match - allow always
	// - `same-site` 	same registrable domain (subdomain and/or different port) - block, unless explicitly trusted
	// - `cross-site`	request originates from different site - block, unless explicitly trusted
	// - `none`			direct navigation (URL bar, bookmark) - allow always
	secFetchSite := r.Header.Get(keratin.HeaderSecFetchSite)
	if secFetchSite == "" {
		return false, nil
	}

	if len(c.TrustedOrigins) > 0 {
		// trusted sites ala OAuth callbacks etc. should be let through
		origin := r.Header.Get(keratin.HeaderOrigin)
		if origin != "" {
			for _, trustedOrigin := range c.TrustedOrigins {
				if strings.EqualFold(origin, trustedOrigin) {
					return true, nil
				}
			}
		}
	}
	isSafe := slices.Contains(safeMethods, r.Method)
	if !isSafe { // for state-changing request check SecFetchSite value
		isSafe = secFetchSite == "same-origin" || secFetchSite == "none"
	}

	if isSafe {
		return true, nil
	}
	// we are here when request is state-changing and `cross-site` or `same-site`

	// Note: if you want to allow `same-site` use c.TrustedOrigins or `c.AllowSecFetchSiteFunc`
	if c.AllowSecFetchSiteFunc != nil {
		return c.AllowSecFetchSiteFunc(r)
	}

	if secFetchSite == "same-site" {
		return false, keratin.NewHTTPError(http.StatusForbidden, "same-site request blocked by CSRF")
	}
	return false, keratin.NewHTTPError(http.StatusForbidden, "cross-site request blocked by CSRF")
}
