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
	"fmt"
	"net/http"

	"github.com/gowool/keratin"
)

var DefaultSecureConfig = SecureConfig{
	XSSProtection:      "1; mode=block",
	ContentTypeNosniff: "nosniff",
	XFrameOptions:      "SAMEORIGIN",
}

type SecureConfig struct {
	// XSSProtection provides protection against cross-site scripting attack (XSS)
	// by setting the `X-XSS-Protection` header.
	// Optional. Default value "".
	XSSProtection string `env:"XSS_PROTECTION" json:"xssProtection,omitempty" yaml:"xssProtection,omitempty"`

	// ContentTypeNosniff provides protection against overriding Content-Type
	// header by setting the `X-Content-Type-Options` header.
	// Optional. Default value "".
	ContentTypeNosniff string `env:"CONTENT_TYPE_NOSNIFF" json:"contentTypeNosniff,omitempty" yaml:"contentTypeNosniff,omitempty"`

	// XFrameOptions can be used to indicate whether or not a browser should
	// be allowed to render a page in a <frame>, <iframe> or <object> .
	// Sites can use this to avoid clickjacking attacks, by ensuring that their
	// content is not embedded into other sites.provides protection against
	// clickjacking.
	// Optional. Default value "".
	// Possible values:
	// - "SAMEORIGIN" - The page can only be displayed in a frame on the same origin as the page itself.
	// - "DENY" - The page cannot be displayed in a frame, regardless of the site attempting to do so.
	// - "ALLOW-FROM uri" - The page can only be displayed in a frame on the specified origin.
	XFrameOptions string `env:"X_FRAME_OPTIONS" json:"xFrameOptions,omitempty" yaml:"xFrameOptions,omitempty"`

	// HSTSMaxAge sets the `Strict-Transport-Security` header to indicate how
	// long (in seconds) browsers should remember that this site is only to
	// be accessed using HTTPS. This reduces your exposure to some SSL-stripping
	// man-in-the-middle (MITM) attacks.
	// Optional. Default value 0.
	HSTSMaxAge int `env:"HSTS_MAX_AGE" json:"hstsMaxAge,omitempty" yaml:"hstsMaxAge,omitempty"`

	// HSTSExcludeSubdomains won't include subdomains tag in the `Strict Transport Security`
	// header, excluding all subdomains from security policy. It has no effect
	// unless HSTSMaxAge is set to a non-zero value.
	// Optional. Default value false.
	HSTSExcludeSubdomains bool `env:"HSTS_EXCLUDE_SUBDOMAINS" json:"hstsExcludeSubdomains,omitempty" yaml:"hstsExcludeSubdomains,omitempty"`

	// ContentSecurityPolicy sets the `Content-Security-Policy` header providing
	// security against cross-site scripting (XSS), clickjacking and other code
	// injection attacks resulting from execution of malicious content in the
	// trusted web page context.
	// Optional. Default value "".
	ContentSecurityPolicy string `env:"CONTENT_SECURITY_POLICY" json:"contentSecurityPolicy,omitempty" yaml:"contentSecurityPolicy,omitempty"`

	// CSPReportOnly would use the `Content-Security-Policy-Report-Only` header instead
	// of the `Content-Security-Policy` header. This allows iterative updates of the
	// content security policy by only reporting the violations that would
	// have occurred instead of blocking the resource.
	// Optional. Default value false.
	CSPReportOnly bool `env:"CSP_REPORT_ONLY" json:"cspReportOnly,omitempty" yaml:"cspReportOnly,omitempty"`

	// HSTSPreloadEnabled will add the preload tag in the `Strict Transport Security`
	// header, which enables the domain to be included in the HSTS preload list
	// maintained by Chrome (and used by Firefox and Safari): https://hstspreload.org/
	// Optional.  Default value false.
	HSTSPreloadEnabled bool `env:"HSTS_PRELOAD_ENABLED" json:"hstsPreloadEnabled,omitempty" yaml:"hstsPreloadEnabled,omitempty"`

	// ReferrerPolicy sets the `Referrer-Policy` header providing security against
	// leaking potentially sensitive request paths to third parties.
	// Optional. Default value "".
	ReferrerPolicy string `env:"REFERRER_POLICY" json:"referrerPolicy,omitempty" yaml:"referrerPolicy,omitempty"`
}

func Secure(cfg SecureConfig, skippers ...Skipper) func(keratin.Handler) keratin.Handler {
	skip := ChainSkipper(skippers...)

	return func(next keratin.Handler) keratin.Handler {
		return keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			if skip(r) {
				return next.ServeHTTP(w, r)
			}

			if cfg.XSSProtection != "" {
				w.Header().Set(keratin.HeaderXXSSProtection, cfg.XSSProtection)
			}
			if cfg.ContentTypeNosniff != "" {
				w.Header().Set(keratin.HeaderXContentTypeOptions, cfg.ContentTypeNosniff)
			}
			if cfg.XFrameOptions != "" {
				w.Header().Set(keratin.HeaderXFrameOptions, cfg.XFrameOptions)
			}
			if (r.TLS != nil || (r.Header.Get(keratin.HeaderXForwardedProto) == "https")) && cfg.HSTSMaxAge != 0 {
				subdomains := ""
				if !cfg.HSTSExcludeSubdomains {
					subdomains = "; includeSubdomains"
				}
				if cfg.HSTSPreloadEnabled {
					subdomains = fmt.Sprintf("%s; preload", subdomains)
				}
				w.Header().Set(keratin.HeaderStrictTransportSecurity, fmt.Sprintf("max-age=%d%s", cfg.HSTSMaxAge, subdomains))
			}
			if cfg.ContentSecurityPolicy != "" {
				if cfg.CSPReportOnly {
					w.Header().Set(keratin.HeaderContentSecurityPolicyReportOnly, cfg.ContentSecurityPolicy)
				} else {
					w.Header().Set(keratin.HeaderContentSecurityPolicy, cfg.ContentSecurityPolicy)
				}
			}
			if cfg.ReferrerPolicy != "" {
				w.Header().Set(keratin.HeaderReferrerPolicy, cfg.ReferrerPolicy)
			}

			return next.ServeHTTP(w, r)
		})
	}
}
