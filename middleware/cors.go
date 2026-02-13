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
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gowool/keratin"
)

type CORSConfig struct {
	// AllowOrigins determines the value of the Access-Control-Allow-Origin
	// response header.  This header defines a list of origins that may access the
	// resource.
	//
	// Origin consist of following parts: `scheme + "://" + host + optional ":" + port`
	// Wildcard can be used, but has to be set explicitly []string{"*"}
	// Example: `https://example.com`, `http://example.com:8080`, `*`
	//
	// Security: use extreme caution when handling the origin, and carefully
	// validate any logic. Remember that attackers may register hostile domain names.
	// See https://blog.portswigger.net/2016/10/exploiting-cors-misconfigurations-for.html
	// See also: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Origin
	//
	// Mandatory.
	AllowOrigins []string `env:"ALLOW_ORIGINS" json:"allowOrigins,omitempty" yaml:"allowOrigins,omitempty"`

	// UnsafeAllowOriginFunc is an optional custom function to validate the origin. It takes the
	// origin as an argument and returns
	// - string, allowed origin
	// - bool, true if allowed or false otherwise.
	// - error, if an error is returned, it is returned immediately by the handler.
	// If this option is set, AllowOrigins is ignored.
	//
	// Security: use extreme caution when handling the origin, and carefully
	// validate any logic. Remember that attackers may register hostile (sub)domain names.
	// See https://blog.portswigger.net/2016/10/exploiting-cors-misconfigurations-for.html
	//
	// Sub-domain checks example:
	// 		UnsafeAllowOriginFunc: func(r *http.Request, origin string) (string, bool, error) {
	//			if strings.HasSuffix(origin, ".example.com") {
	//				return origin, true, nil
	//			}
	//			return "", false, nil
	//		},
	//
	// Optional.
	UnsafeAllowOriginFunc func(r *http.Request, origin string) (allowedOrigin string, allowed bool, err error) `json:"-" yaml:"-"`

	// AllowMethods determines the value of the Access-Control-Allow-Methods
	// response header. This header specified the list of methods allowed when
	// accessing the resource. This is used in response to a preflight request.
	//
	// If `allowMethods` is left empty, this middleware will fill for preflight
	// request `Access-Control-Allow-Methods` header value
	// from `Allow` header that keratin.Router set into request context.
	//
	// See also: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Methods
	AllowMethods []string `env:"ALLOW_METHODS" json:"allowMethods,omitempty" yaml:"allowMethods,omitempty"`

	// AllowHeaders determines the value of the Access-Control-Allow-Headers
	// response header.  This header is used in response to a preflight request to
	// indicate which HTTP headers can be used when making the actual request.
	//
	// Optional. Defaults to empty list. No domains allowed for CORS.
	//
	// See also: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Headers
	AllowHeaders []string `env:"ALLOW_HEADERS" json:"allowHeaders,omitempty" yaml:"allowHeaders,omitempty"`

	// AllowCredentials determines the value of the
	// Access-Control-Allow-Credentials response header.  This header indicates
	// whether or not the response to the request can be exposed when the
	// credentials mode (Request.credentials) is true. When used as part of a
	// response to a preflight request, this indicates whether or not the actual
	// request can be made using credentials.  See also
	// [MDN: Access-Control-Allow-Credentials].
	//
	// Optional. Default value false, in which case the header is not set.
	//
	// Security: avoid using `AllowCredentials = true` with `AllowOrigins = *`.
	// See "Exploiting CORS misconfigurations for Bitcoins and bounties",
	// https://blog.portswigger.net/2016/10/exploiting-cors-misconfigurations-for.html
	//
	// See also: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Credentials
	AllowCredentials bool `env:"ALLOW_CREDENTIALS" json:"allowCredentials,omitempty" yaml:"allowCredentials,omitempty"`

	// ExposeHeaders determines the value of Access-Control-Expose-Headers, which
	// defines a list of headers that clients are allowed to access.
	//
	// Optional. Default value []string{}, in which case the header is not set.
	//
	// See also: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Expose-Header
	ExposeHeaders []string `env:"EXPOSE_HEADERS" json:"exposeHeaders,omitempty" yaml:"exposeHeaders,omitempty"`

	// MaxAge determines the value of the Access-Control-Max-Age response header.
	// This header indicates how long (in seconds) the results of a preflight
	// request can be cached.
	// The header is set only if MaxAge != 0, negative value sends "0" which instructs browsers not to cache that response.
	//
	// Optional. Default value 0 - meaning header is not sent.
	//
	// See also: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Max-Age
	MaxAge int `env:"MAX_AGE" json:"maxAge,omitempty" yaml:"maxAge,omitempty"`
}

func CORS(cfg CORSConfig, skippers ...Skipper) func(http.Handler) http.Handler {
	skip := ChainSkipper(skippers...)

	if len(cfg.AllowMethods) == 0 {
		cfg.AllowMethods = []string{
			http.MethodGet,
			http.MethodHead,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		}
	}

	allowMethods := strings.Join(cfg.AllowMethods, ",")
	allowHeaders := strings.Join(cfg.AllowHeaders, ",")
	exposeHeaders := strings.Join(cfg.ExposeHeaders, ",")

	maxAge := "0"
	if cfg.MaxAge > 0 {
		maxAge = strconv.Itoa(cfg.MaxAge)
	}

	allowOriginFunc := cfg.UnsafeAllowOriginFunc
	if cfg.UnsafeAllowOriginFunc == nil {
		if len(cfg.AllowOrigins) == 0 {
			panic(errors.New("middleware: cors: at least one AllowOrigins is required or UnsafeAllowOriginFunc must be provided"))
		}

		allowOriginFunc = cfg.defaultAllowOriginFunc
		for _, origin := range cfg.AllowOrigins {
			if origin == "*" {
				if cfg.AllowCredentials {
					panic(errors.New("middleware: cors: * as allowed origin and AllowCredentials=true is insecure and not allowed. Use custom UnsafeAllowOriginFunc"))
				}
				allowOriginFunc = cfg.starAllowOriginFunc
				break
			}
			if err := validateOrigin(origin, "allow origin"); err != nil {
				panic(fmt.Errorf("middleware: cors: %w", err))
			}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skip(r) {
				next.ServeHTTP(w, r)
				return
			}

			origin := r.Header.Get(keratin.HeaderOrigin)

			w.Header().Add(keratin.HeaderVary, keratin.HeaderOrigin)

			// Preflight request is an OPTIONS request, using three HTTP request headers: Access-Control-Request-Method,
			// Access-Control-Request-Headers, and the Origin header. See: https://developer.mozilla.org/en-US/docs/Glossary/Preflight_request
			// For simplicity we just consider method type and later `Origin` header.
			preflight := r.Method == http.MethodOptions

			// No Origin provided. This is (probably) not request from actual browser - proceed executing middleware chain
			if origin == "" {
				if preflight { // req.Method=OPTIONS
					w.WriteHeader(http.StatusNoContent)
					return
				}

				next.ServeHTTP(w, r) // let non-browser calls through
				return
			}

			allowedOrigin, allowed, err := allowOriginFunc(r, origin)
			if err != nil {
				keratin.DefaultErrorHandler(w, r, err)
				return
			}
			if !allowed {
				// Origin existed and was NOT allowed
				if preflight {
					// If the request's origin isn't allowed by the CORS configuration,
					// the middleware should simply omit the relevant CORS headers from the response
					// and let the browser fail the CORS check (if any).
					w.WriteHeader(http.StatusNoContent)
					return
				}
				// no CORS middleware should block non-preflight requests;
				// such requests should be let through. One reason is that not all requests that
				// carry an Origin header participate in the CORS protocol.
				next.ServeHTTP(w, r)
				return
			}

			// Origin existed and was allowed

			w.Header().Set(keratin.HeaderAccessControlAllowOrigin, allowedOrigin)
			if cfg.AllowCredentials {
				w.Header().Set(keratin.HeaderAccessControlAllowCredentials, "true")
			}

			// Simple request will be let though
			if !preflight {
				if exposeHeaders != "" {
					w.Header().Set(keratin.HeaderAccessControlExposeHeaders, exposeHeaders)
				}

				next.ServeHTTP(w, r)
				return
			}

			// Below code is for Preflight (OPTIONS) request
			//
			// Preflight will end with c.NoContent(http.StatusNoContent) as we do not know if
			// at the end of handler chain is actual OPTIONS route or 404/405 route which
			// response code will confuse browsers
			w.Header().Add(keratin.HeaderVary, keratin.HeaderAccessControlRequestMethod)
			w.Header().Add(keratin.HeaderVary, keratin.HeaderAccessControlRequestHeaders)
			w.Header().Set(keratin.HeaderAccessControlAllowMethods, allowMethods)

			if allowHeaders != "" {
				w.Header().Set(keratin.HeaderAccessControlAllowHeaders, allowHeaders)
			} else {
				h := r.Header.Get(keratin.HeaderAccessControlRequestHeaders)
				if h != "" {
					w.Header().Set(keratin.HeaderAccessControlAllowHeaders, h)
				}
			}

			if cfg.MaxAge != 0 {
				w.Header().Set(keratin.HeaderAccessControlMaxAge, maxAge)
			}

			w.WriteHeader(http.StatusNoContent)
		})
	}
}

func (*CORSConfig) starAllowOriginFunc(*http.Request, string) (string, bool, error) {
	return "*", true, nil
}

func (c *CORSConfig) defaultAllowOriginFunc(_ *http.Request, origin string) (string, bool, error) {
	for _, allowedOrigin := range c.AllowOrigins {
		if strings.EqualFold(allowedOrigin, origin) {
			return allowedOrigin, true, nil
		}
	}
	return "", false, nil
}
