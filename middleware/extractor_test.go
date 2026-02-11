package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateExtractors(t *testing.T) {
	tests := []struct {
		name      string
		lookups   string
		limit     uint
		wantCount int
		wantErr   bool
	}{
		{
			name:      "empty lookups returns nil",
			lookups:   "",
			wantCount: 0,
		},
		{
			name:      "single header lookup",
			lookups:   "header:Authorization",
			wantCount: 1,
		},
		{
			name:      "multiple lookups",
			lookups:   "header:Authorization,query:token",
			wantCount: 2,
		},
		{
			name:      "all source types",
			lookups:   "header:X,query:q,param:p,cookie:c,form:f",
			wantCount: 5,
		},
		{
			name:      "limit 0 defaults to 1",
			lookups:   "header:X",
			limit:     0,
			wantCount: 1,
		},
		{
			name:      "limit greater than extractorLimit is capped",
			lookups:   "header:X",
			limit:     100,
			wantCount: 1,
		},
		{
			name:      "header with prefix",
			lookups:   "header:Authorization:Bearer ",
			wantCount: 1,
		},
		{
			name:    "invalid lookup format",
			lookups: "invalid",
			wantErr: true,
		},
		{
			name:    "invalid lookup format missing colon",
			lookups: "headerAuthorization",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateExtractors(tt.lookups, tt.limit)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			if tt.wantCount == 0 {
				assert.Nil(t, got)
			} else {
				assert.Len(t, got, tt.wantCount)
			}
		})
	}
}

func TestValuesFromHeader(t *testing.T) {
	tests := []struct {
		name        string
		header      string
		headerValue []string
		prefix      string
		limit       uint
		wantValues  []string
		wantSource  ExtractorSource
		wantErr     error
	}{
		{
			name:        "single header value without prefix",
			header:      "Authorization",
			headerValue: []string{"Bearer token123"},
			wantValues:  []string{"Bearer token123"},
			wantSource:  ExtractorSourceHeader,
		},
		{
			name:        "single header value with prefix",
			header:      "Authorization",
			headerValue: []string{"Bearer token123"},
			prefix:      "Bearer ",
			wantValues:  []string{"token123"},
			wantSource:  ExtractorSourceHeader,
		},
		{
			name:        "multiple header values",
			header:      "X-Custom",
			headerValue: []string{"value1", "value2", "value3"},
			limit:       2,
			wantValues:  []string{"value1", "value2"},
			wantSource:  ExtractorSourceHeader,
		},
		{
			name:        "case insensitive prefix matching",
			header:      "Authorization",
			headerValue: []string{"BEARER token123"},
			prefix:      "bearer ",
			wantValues:  []string{"token123"},
			wantSource:  ExtractorSourceHeader,
		},
		{
			name:        "value shorter than prefix",
			header:      "Authorization",
			headerValue: []string{"Bearer"},
			prefix:      "Bearer ",
			wantErr:     errHeaderExtractorValueInvalid,
			wantSource:  ExtractorSourceHeader,
		},
		{
			name:        "prefix does not match",
			header:      "Authorization",
			headerValue: []string{"Basic token123"},
			prefix:      "Bearer ",
			wantErr:     errHeaderExtractorValueInvalid,
			wantSource:  ExtractorSourceHeader,
		},
		{
			name:        "missing header",
			header:      "X-Missing",
			headerValue: []string{},
			wantErr:     errHeaderExtractorValueMissing,
			wantSource:  ExtractorSourceHeader,
		},
		{
			name:        "canonical header name conversion",
			header:      "authorization",
			headerValue: []string{"Bearer token"},
			prefix:      "Bearer ",
			wantValues:  []string{"token"},
			wantSource:  ExtractorSourceHeader,
		},
		{
			name:        "limit 0 returns single value",
			header:      "X-Test",
			headerValue: []string{"val1", "val2"},
			limit:       0,
			wantValues:  []string{"val1"},
			wantSource:  ExtractorSourceHeader,
		},
		{
			name:        "empty prefix returns full value",
			header:      "X-Test",
			headerValue: []string{"Bearer token"},
			prefix:      "",
			wantValues:  []string{"Bearer token"},
			wantSource:  ExtractorSourceHeader,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := valuesFromHeader(tt.header, tt.prefix, tt.limit)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			for _, val := range tt.headerValue {
				req.Header.Add(tt.header, val)
			}

			got, source, err := extractor(req)

			assert.Equal(t, tt.wantSource, source)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantValues, got)
		})
	}
}

func TestValuesFromQuery(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		param      string
		limit      uint
		wantValues []string
		wantSource ExtractorSource
		wantErr    error
	}{
		{
			name:       "single query value",
			query:      "token=abc123",
			param:      "token",
			wantValues: []string{"abc123"},
			wantSource: ExtractorSourceQuery,
		},
		{
			name:       "multiple query values",
			query:      "id=1&id=2&id=3",
			param:      "id",
			limit:      3,
			wantValues: []string{"1", "2", "3"},
			wantSource: ExtractorSourceQuery,
		},
		{
			name:       "multiple query values with limit",
			query:      "id=1&id=2&id=3&id=4",
			param:      "id",
			limit:      2,
			wantValues: []string{"1", "2"},
			wantSource: ExtractorSourceQuery,
		},
		{
			name:       "missing query parameter",
			query:      "other=value",
			param:      "token",
			wantErr:    errQueryExtractorValueMissing,
			wantSource: ExtractorSourceQuery,
		},
		{
			name:       "empty query string",
			query:      "",
			param:      "token",
			wantErr:    errQueryExtractorValueMissing,
			wantSource: ExtractorSourceQuery,
		},
		{
			name:       "parameter with empty value",
			query:      "token=",
			param:      "token",
			wantValues: []string{""},
			wantSource: ExtractorSourceQuery,
		},
		{
			name:       "url encoded values",
			query:      "token=abc%20def",
			param:      "token",
			wantValues: []string{"abc def"},
			wantSource: ExtractorSourceQuery,
		},
		{
			name:       "limit 0 returns single value",
			query:      "id=1&id=2",
			param:      "id",
			limit:      0,
			wantValues: []string{"1"},
			wantSource: ExtractorSourceQuery,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := valuesFromQuery(tt.param, tt.limit)

			req := httptest.NewRequest(http.MethodGet, "/?"+tt.query, nil)

			got, source, err := extractor(req)

			assert.Equal(t, tt.wantSource, source)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantValues, got)
		})
	}
}

func TestValuesFromParam(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		param      string
		wantValues []string
		wantSource ExtractorSource
		wantErr    error
	}{
		{
			name:       "missing path parameter returns error",
			path:       "/users/123",
			param:      "id",
			wantErr:    errParamExtractorValueMissing,
			wantSource: ExtractorSourcePathParam,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := valuesFromParam(tt.param)

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)

			got, source, err := extractor(req)

			assert.Equal(t, tt.wantSource, source)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantValues, got)
		})
	}
}

func TestValuesFromCookie(t *testing.T) {
	tests := []struct {
		testName   string
		cookies    []*http.Cookie
		cookieName string
		limit      uint
		wantValues []string
		wantSource ExtractorSource
		wantErr    error
	}{
		{
			testName:   "single cookie",
			cookies:    []*http.Cookie{{Name: "session", Value: "abc123"}},
			cookieName: "session",
			wantValues: []string{"abc123"},
			wantSource: ExtractorSourceCookie,
		},
		{
			testName: "multiple cookies same name",
			cookies: []*http.Cookie{
				{Name: "token", Value: "val1"},
				{Name: "token", Value: "val2"},
				{Name: "token", Value: "val3"},
			},
			cookieName: "token",
			limit:      3,
			wantValues: []string{"val1", "val2", "val3"},
			wantSource: ExtractorSourceCookie,
		},
		{
			testName: "multiple cookies with limit",
			cookies: []*http.Cookie{
				{Name: "token", Value: "val1"},
				{Name: "token", Value: "val2"},
				{Name: "token", Value: "val3"},
			},
			cookieName: "token",
			limit:      2,
			wantValues: []string{"val1", "val2"},
			wantSource: ExtractorSourceCookie,
		},
		{
			testName:   "missing cookie",
			cookies:    []*http.Cookie{{Name: "other", Value: "value"}},
			cookieName: "session",
			wantErr:    errCookieExtractorValueMissing,
			wantSource: ExtractorSourceCookie,
		},
		{
			testName:   "no cookies",
			cookies:    []*http.Cookie{},
			cookieName: "session",
			wantErr:    errCookieExtractorValueMissing,
			wantSource: ExtractorSourceCookie,
		},
		{
			testName:   "cookie with empty value",
			cookies:    []*http.Cookie{{Name: "session", Value: ""}},
			cookieName: "session",
			wantValues: []string{""},
			wantSource: ExtractorSourceCookie,
		},
		{
			testName:   "cookie with special characters",
			cookies:    []*http.Cookie{{Name: "session", Value: "abc%20def"}},
			cookieName: "session",
			wantValues: []string{"abc%20def"},
			wantSource: ExtractorSourceCookie,
		},
		{
			testName:   "limit 0 returns single value",
			cookies:    []*http.Cookie{{Name: "token", Value: "val1"}, {Name: "token", Value: "val2"}},
			cookieName: "token",
			limit:      0,
			wantValues: []string{"val1"},
			wantSource: ExtractorSourceCookie,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			extractor := valuesFromCookie(tt.cookieName, tt.limit)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			for _, cookie := range tt.cookies {
				req.AddCookie(cookie)
			}

			got, source, err := extractor(req)

			assert.Equal(t, tt.wantSource, source)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantValues, got)
		})
	}
}

func TestValuesFromForm(t *testing.T) {
	tests := []struct {
		testName      string
		formData      url.Values
		formFieldName string
		limit         uint
		wantValues    []string
		wantSource    ExtractorSource
		wantErr       error
	}{
		{
			testName:      "single form value",
			formData:      url.Values{"username": []string{"john"}},
			formFieldName: "username",
			wantValues:    []string{"john"},
			wantSource:    ExtractorSourceForm,
		},
		{
			testName:      "multiple form values",
			formData:      url.Values{"tags": []string{"go", "testing", "tdd"}},
			formFieldName: "tags",
			limit:         3,
			wantValues:    []string{"go", "testing", "tdd"},
			wantSource:    ExtractorSourceForm,
		},
		{
			testName:      "multiple form values with limit",
			formData:      url.Values{"tags": []string{"go", "testing", "tdd", "code"}},
			formFieldName: "tags",
			limit:         2,
			wantValues:    []string{"go", "testing"},
			wantSource:    ExtractorSourceForm,
		},
		{
			testName:      "missing form field",
			formData:      url.Values{"other": []string{"value"}},
			formFieldName: "username",
			wantErr:       errFormExtractorValueMissing,
			wantSource:    ExtractorSourceForm,
		},
		{
			testName:      "form field with empty value",
			formData:      url.Values{"username": []string{""}},
			formFieldName: "username",
			wantValues:    []string{""},
			wantSource:    ExtractorSourceForm,
		},
		{
			testName:      "empty form data",
			formData:      url.Values{},
			formFieldName: "username",
			wantErr:       errFormExtractorValueMissing,
			wantSource:    ExtractorSourceForm,
		},
		{
			testName:      "form value with special characters",
			formData:      url.Values{"content": []string{"hello & world"}},
			formFieldName: "content",
			wantValues:    []string{"hello & world"},
			wantSource:    ExtractorSourceForm,
		},
		{
			testName:      "limit 0 returns single value",
			formData:      url.Values{"id": []string{"1", "2", "3"}},
			formFieldName: "id",
			limit:         0,
			wantValues:    []string{"1"},
			wantSource:    ExtractorSourceForm,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			extractor := valuesFromForm(tt.formFieldName, tt.limit)

			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			got, source, err := extractor(req)

			assert.Equal(t, tt.wantSource, source)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantValues, got)
		})
	}
}

func TestValueExtractorError(t *testing.T) {
	tests := []struct {
		name    string
		err     *ValueExtractorError
		message string
	}{
		{
			name:    "error message is returned correctly",
			err:     &ValueExtractorError{message: "test error"},
			message: "test error",
		},
		{
			name:    "empty error message",
			err:     &ValueExtractorError{message: ""},
			message: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.message, tt.err.Error())
		})
	}
}

func TestExtractorConstants(t *testing.T) {
	tests := []struct {
		name  string
		value ExtractorSource
		want  string
	}{
		{
			name:  "ExtractorSourceHeader",
			value: ExtractorSourceHeader,
			want:  "header",
		},
		{
			name:  "ExtractorSourceQuery",
			value: ExtractorSourceQuery,
			want:  "query",
		},
		{
			name:  "ExtractorSourcePathParam",
			value: ExtractorSourcePathParam,
			want:  "param",
		},
		{
			name:  "ExtractorSourceCookie",
			value: ExtractorSourceCookie,
			want:  "cookie",
		},
		{
			name:  "ExtractorSourceForm",
			value: ExtractorSourceForm,
			want:  "form",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, string(tt.value))
		})
	}
}

func TestExtractorLimit(t *testing.T) {
	tests := []struct {
		name  string
		limit uint
		want  uint
	}{
		{
			name:  "limit 0 defaults to 1",
			limit: 0,
			want:  1,
		},
		{
			name:  "limit below extractorLimit is unchanged",
			limit: 10,
			want:  10,
		},
		{
			name:  "limit equal to extractorLimit is unchanged",
			limit: extractorLimit,
			want:  extractorLimit,
		},
		{
			name:  "limit above extractorLimit is capped",
			limit: 100,
			want:  extractorLimit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractors, err := CreateExtractors("header:X", tt.limit)
			require.NoError(t, err)
			require.Len(t, extractors, 1)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			for i := 0; i < 25; i++ {
				req.Header.Add("X", "value"+string(rune('1'+i)))
			}

			values, _, err := extractors[0](req)
			require.NoError(t, err)
			assert.Len(t, values, int(tt.want))
		})
	}
}

func TestMultipleExtractors(t *testing.T) {
	tests := []struct {
		name      string
		lookups   string
		setupReq  func(*http.Request)
		wantCount int
	}{
		{
			name:    "try header then query",
			lookups: "header:X-API-Key,query:token",
			setupReq: func(r *http.Request) {
				r.Header.Set("X-API-Key", "header-value")
			},
			wantCount: 1,
		},
		{
			name:    "try query then header",
			lookups: "query:token,header:X-API-Key",
			setupReq: func(r *http.Request) {
				r.URL.RawQuery = "token=query-value"
			},
			wantCount: 1,
		},
		{
			name:    "header fails, query succeeds",
			lookups: "header:Missing,query:token",
			setupReq: func(r *http.Request) {
				r.URL.RawQuery = "token=value"
			},
			wantCount: 1,
		},
		{
			name:     "all sources fail",
			lookups:  "header:Missing,query:missing,param:missing",
			setupReq: func(r *http.Request) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractors, err := CreateExtractors(tt.lookups, 0)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			tt.setupReq(req)

			successCount := 0
			for _, extractor := range extractors {
				values, _, err := extractor(req)
				if err == nil && len(values) > 0 {
					successCount++
				}
			}

			assert.Equal(t, tt.wantCount, successCount)
		})
	}
}

func BenchmarkValuesFromHeader(b *testing.B) {
	extractor := valuesFromHeader("X-Custom", "Bearer ", 1)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Custom", "Bearer token123")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = extractor(req)
	}
}

func BenchmarkValuesFromQuery(b *testing.B) {
	extractor := valuesFromQuery("token", 1)
	req := httptest.NewRequest(http.MethodGet, "/?token=abc123", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = extractor(req)
	}
}

func BenchmarkValuesFromParam(b *testing.B) {
	extractor := valuesFromParam("id")
	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = extractor(req)
	}
}

func BenchmarkValuesFromCookie(b *testing.B) {
	extractor := valuesFromCookie("session", 1)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "abc123"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = extractor(req)
	}
}

func BenchmarkValuesFromForm(b *testing.B) {
	extractor := valuesFromForm("username", 1)
	formData := url.Values{"username": []string{"john"}}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = extractor(req)
	}
}
