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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gowool/keratin"
	"github.com/stretchr/testify/assert"
)

func TestSecure_DefaultConfig(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	// Default
	mw := Secure(DefaultSecureConfig)
	h := mw(keratin.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test"))
		return nil
	}))
	err := h.ServeHTTP(rec, req)
	assert.NoError(t, err)

	assert.Equal(t, "1; mode=block", rec.Header().Get(keratin.HeaderXXSSProtection))
	assert.Equal(t, "nosniff", rec.Header().Get(keratin.HeaderXContentTypeOptions))
	assert.Equal(t, "SAMEORIGIN", rec.Header().Get(keratin.HeaderXFrameOptions))
	assert.Equal(t, "", rec.Header().Get(keratin.HeaderStrictTransportSecurity))
	assert.Equal(t, "", rec.Header().Get(keratin.HeaderContentSecurityPolicy))
	assert.Equal(t, "", rec.Header().Get(keratin.HeaderReferrerPolicy))
}

func TestSecure(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(keratin.HeaderXForwardedProto, "https")
	rec := httptest.NewRecorder()
	mw := Secure(SecureConfig{
		XSSProtection:         "",
		ContentTypeNosniff:    "",
		XFrameOptions:         "",
		HSTSMaxAge:            3600,
		ContentSecurityPolicy: "default-src 'self'",
		ReferrerPolicy:        "origin",
	})
	h := mw(keratin.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test"))
		return nil
	}))
	err := h.ServeHTTP(rec, req)
	assert.NoError(t, err)

	assert.Equal(t, "", rec.Header().Get(keratin.HeaderXXSSProtection))
	assert.Equal(t, "", rec.Header().Get(keratin.HeaderXContentTypeOptions))
	assert.Equal(t, "", rec.Header().Get(keratin.HeaderXFrameOptions))
	assert.Equal(t, "max-age=3600; includeSubdomains", rec.Header().Get(keratin.HeaderStrictTransportSecurity))
	assert.Equal(t, "default-src 'self'", rec.Header().Get(keratin.HeaderContentSecurityPolicy))
	assert.Equal(t, "", rec.Header().Get(keratin.HeaderContentSecurityPolicyReportOnly))
	assert.Equal(t, "origin", rec.Header().Get(keratin.HeaderReferrerPolicy))

}

func TestSecure_CSPReportOnly(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(keratin.HeaderXForwardedProto, "https")
	rec := httptest.NewRecorder()
	mw := Secure(SecureConfig{
		XSSProtection:         "",
		ContentTypeNosniff:    "",
		XFrameOptions:         "",
		HSTSMaxAge:            3600,
		ContentSecurityPolicy: "default-src 'self'",
		CSPReportOnly:         true,
		ReferrerPolicy:        "origin",
	})
	h := mw(keratin.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test"))
		return nil
	}))
	err := h.ServeHTTP(rec, req)
	assert.NoError(t, err)

	assert.Equal(t, "", rec.Header().Get(keratin.HeaderXXSSProtection))
	assert.Equal(t, "", rec.Header().Get(keratin.HeaderXContentTypeOptions))
	assert.Equal(t, "", rec.Header().Get(keratin.HeaderXFrameOptions))
	assert.Equal(t, "max-age=3600; includeSubdomains", rec.Header().Get(keratin.HeaderStrictTransportSecurity))
	assert.Equal(t, "default-src 'self'", rec.Header().Get(keratin.HeaderContentSecurityPolicyReportOnly))
	assert.Equal(t, "", rec.Header().Get(keratin.HeaderContentSecurityPolicy))
	assert.Equal(t, "origin", rec.Header().Get(keratin.HeaderReferrerPolicy))
}

func TestSecure_HSTSPreloadEnabled(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// Custom, with preload option enabled
	req.Header.Set(keratin.HeaderXForwardedProto, "https")
	rec := httptest.NewRecorder()

	mw := Secure(SecureConfig{
		HSTSMaxAge:         3600,
		HSTSPreloadEnabled: true,
	})
	h := mw(keratin.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test"))
		return nil
	}))
	err := h.ServeHTTP(rec, req)
	assert.NoError(t, err)

	assert.Equal(t, "max-age=3600; includeSubdomains; preload", rec.Header().Get(keratin.HeaderStrictTransportSecurity))

}

func TestSecure_HSTSExcludeSubdomains(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	// Custom, with preload option enabled and subdomains excluded
	req.Header.Set(keratin.HeaderXForwardedProto, "https")
	rec := httptest.NewRecorder()

	mw := Secure(SecureConfig{
		HSTSMaxAge:            3600,
		HSTSPreloadEnabled:    true,
		HSTSExcludeSubdomains: true,
	})
	h := mw(keratin.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) error {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test"))
		return nil
	}))
	err := h.ServeHTTP(rec, req)
	assert.NoError(t, err)

	assert.Equal(t, "max-age=3600; preload", rec.Header().Get(keratin.HeaderStrictTransportSecurity))
}
