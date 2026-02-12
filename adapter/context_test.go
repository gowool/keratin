package adapter

import (
	"bytes"
	"crypto/tls"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestContext() *kContext {
	ctx := &kContext{}

	// Create a test HTTP request
	req := httptest.NewRequest("GET", "http://example.com/test?foo=bar", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Custom-Header", "test-value")

	// Create a test response
	resp := httptest.NewRecorder()

	// Create a test operation
	op := &huma.Operation{
		Method: "GET",
		Path:   "/test",
	}

	ctx.reset(op, req, resp)
	return ctx
}

type wrapContext struct {
	*kContext
}

func (c *wrapContext) Unwrap() huma.Context {
	return c.kContext
}

func TestNewContext(t *testing.T) {
	ctx := NewContext(nil, nil, nil)
	assert.NotNil(t, ctx)
}

func Test_Unwrap(t *testing.T) {
	t.Run("unwrap request and response", func(t *testing.T) {
		ctx := createTestContext()

		r, w := Unwrap(&wrapContext{kContext: ctx})

		assert.NotNil(t, r)
		assert.NotNil(t, w)
	})

	t.Run("unwrap request and response panic", func(t *testing.T) {
		assert.Panics(t, func() {
			Unwrap(nil)
		})
	})
}

func TestWoContext_reset(t *testing.T) {
	// Create a test HTTP request
	req := httptest.NewRequest("GET", "http://example.com/test?foo=bar", nil)

	// Create a test response
	resp := httptest.NewRecorder()

	op := &huma.Operation{Method: "POST", Path: "/api"}

	ctx := &kContext{}
	ctx.reset(op, req, resp)

	assert.Equal(t, op, ctx.op)
	assert.Equal(t, resp, ctx.w)
	assert.Equal(t, req, ctx.r)
}

func TestWoContext_Unwrap(t *testing.T) {
	ctx := createTestContext()

	r, w := ctx.Unwrap()
	assert.NotNil(t, r)
	assert.NotNil(t, w)
}

func TestWoContext_Operation(t *testing.T) {
	ctx := createTestContext()

	op := ctx.Operation()
	assert.Equal(t, "GET", op.Method)
	assert.Equal(t, "/test", op.Path)
}

func TestWoContext_Context(t *testing.T) {
	ctx := createTestContext()

	reqCtx := ctx.Context()
	assert.NotNil(t, reqCtx)
	assert.Equal(t, ctx.r.Context(), reqCtx)
}

func TestWoContext_TLS(t *testing.T) {
	t.Run("WithTLS", func(t *testing.T) {
		ctx := createTestContext()

		// Add TLS to the request
		tlsState := &tls.ConnectionState{
			Version: tls.VersionTLS12,
		}
		ctx.r.TLS = tlsState

		result := ctx.TLS()
		require.NotNil(t, result)
		assert.Equal(t, uint16(tls.VersionTLS12), result.Version)
	})

	t.Run("WithoutTLS", func(t *testing.T) {
		ctx := createTestContext()

		result := ctx.TLS()
		assert.Nil(t, result)
	})
}

func TestWoContext_Version(t *testing.T) {
	ctx := createTestContext()

	version := ctx.Version()
	assert.Equal(t, "HTTP/1.1", version.Proto)
	assert.Equal(t, 1, version.ProtoMajor)
	assert.Equal(t, 1, version.ProtoMinor)
}

func TestWoContext_Method(t *testing.T) {
	ctx := createTestContext()

	method := ctx.Method()
	assert.Equal(t, "GET", method)
}

func TestWoContext_Host(t *testing.T) {
	ctx := createTestContext()

	host := ctx.Host()
	assert.Equal(t, "example.com", host)
}

func TestWoContext_RemoteAddr(t *testing.T) {
	ctx := createTestContext()

	remoteAddr := ctx.RemoteAddr()
	assert.NotEmpty(t, remoteAddr)
}

func TestWoContext_URL(t *testing.T) {
	ctx := createTestContext()

	parsedURL := ctx.URL()
	assert.Equal(t, "http", parsedURL.Scheme)
	assert.Equal(t, "example.com", parsedURL.Host)
	assert.Equal(t, "/test", parsedURL.Path)
	assert.Equal(t, "bar", parsedURL.Query().Get("foo"))
}

func TestWoContext_Param(t *testing.T) {
	t.Run("WithExistingParam", func(t *testing.T) {
		ctx := createTestContext()

		// Set a path parameter
		ctx.r.SetPathValue("id", "123")

		param := ctx.Param("id")
		assert.Equal(t, "123", param)
	})

	t.Run("WithNonExistentParam", func(t *testing.T) {
		ctx := createTestContext()

		param := ctx.Param("nonexistent")
		assert.Empty(t, param)
	})
}

func TestWoContext_Query(t *testing.T) {
	t.Run("WithExistingQuery", func(t *testing.T) {
		ctx := createTestContext()

		query := ctx.Query("foo")
		assert.Equal(t, "bar", query)
	})

	t.Run("WithNonExistentQuery", func(t *testing.T) {
		ctx := createTestContext()

		query := ctx.Query("nonexistent")
		assert.Empty(t, query)
	})
}

func TestWoContext_Header(t *testing.T) {
	t.Run("WithExistingHeader", func(t *testing.T) {
		ctx := createTestContext()

		header := ctx.Header("Content-Type")
		assert.Equal(t, "application/json", header)
	})

	t.Run("WithCustomHeader", func(t *testing.T) {
		ctx := createTestContext()

		header := ctx.Header("X-Custom-Header")
		assert.Equal(t, "test-value", header)
	})

	t.Run("WithNonExistentHeader", func(t *testing.T) {
		ctx := createTestContext()

		header := ctx.Header("Non-Existent")
		assert.Empty(t, header)
	})

	t.Run("CaseInsensitive", func(t *testing.T) {
		ctx := createTestContext()

		header := ctx.Header("content-type")
		assert.Equal(t, "application/json", header)
	})
}

func TestWoContext_EachHeader(t *testing.T) {
	ctx := createTestContext()

	// Add multiple values for the same header
	ctx.r.Header.Add("X-Multi", "value1")
	ctx.r.Header.Add("X-Multi", "value2")

	var headers []struct {
		name  string
		value string
	}

	ctx.EachHeader(func(name, value string) {
		headers = append(headers, struct {
			name  string
			value string
		}{name: name, value: value})
	})

	// Check that we got all headers
	assert.Greater(t, len(headers), 0)

	// Check for specific headers
	foundMulti := false
	foundContentType := false

	for _, h := range headers {
		if h.name == "X-Multi" && (h.value == "value1" || h.value == "value2") {
			foundMulti = true
		}
		if h.name == "Content-Type" && h.value == "application/json" {
			foundContentType = true
		}
	}

	assert.True(t, foundMulti, "X-Multi header not found")
	assert.True(t, foundContentType, "Content-Type header not found")
}

func TestWoContext_BodyReader(t *testing.T) {
	t.Run("WithBody", func(t *testing.T) {
		ctx := createTestContext()

		body := []byte("test body content")
		ctx.r.Body = io.NopCloser(bytes.NewReader(body))

		reader := ctx.BodyReader()
		assert.NotNil(t, reader)

		data, err := io.ReadAll(reader)
		require.NoError(t, err)
		assert.Equal(t, body, data)
	})

	t.Run("WithoutBody", func(t *testing.T) {
		ctx := createTestContext()

		reader := ctx.BodyReader()
		assert.NotNil(t, reader)
	})
}

func TestWoContext_GetMultipartForm(t *testing.T) {
	t.Run("WithValidMultipartForm", func(t *testing.T) {
		ctx := createTestContext()

		// Create a multipart form body
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		err := writer.WriteField("field1", "value1")
		require.NoError(t, err)
		err = writer.Close()
		require.NoError(t, err)

		ctx.r.Body = io.NopCloser(body)
		ctx.r.Header.Set("Content-Type", writer.FormDataContentType())

		form, err := ctx.GetMultipartForm()
		require.NoError(t, err)
		require.NotNil(t, form)

		values := form.Value["field1"]
		assert.Len(t, values, 1)
		assert.Equal(t, "value1", values[0])
	})

	t.Run("WithInvalidContentType", func(t *testing.T) {
		ctx := createTestContext()

		ctx.r.Body = io.NopCloser(strings.NewReader("not multipart"))
		ctx.r.Header.Set("Content-Type", "text/plain")

		form, err := ctx.GetMultipartForm()
		assert.Error(t, err)
		assert.Nil(t, form)
	})
}

func TestWoContext_SetReadDeadline(t *testing.T) {
	ctx := createTestContext()

	deadline := time.Now().Add(5 * time.Second)
	err := ctx.SetReadDeadline(deadline)

	// Note: httptest.ResponseRecorder doesn't support setting read deadline
	// so we expect an error, but we can test that it doesn't panic
	assert.Error(t, err)
}

func TestWoContext_Status(t *testing.T) {
	t.Run("DefaultStatus", func(t *testing.T) {
		ctx := createTestContext()

		status := ctx.Status()
		assert.Equal(t, 0, status) // Default status before any write
	})

	t.Run("SetStatus", func(t *testing.T) {
		ctx := createTestContext()

		ctx.SetStatus(http.StatusBadRequest)

		assert.Equal(t, http.StatusBadRequest, ctx.Status())

		// The status should be set on the response
		assert.Equal(t, http.StatusBadRequest, ctx.w.(*httptest.ResponseRecorder).Code)
	})
}

func TestWoContext_SetHeader(t *testing.T) {
	ctx := createTestContext()

	ctx.SetHeader("X-Test-Header", "test-value")

	header := ctx.w.Header().Get("X-Test-Header")
	assert.Equal(t, "test-value", header)
}

func TestWoContext_SetHeader_Overwrite(t *testing.T) {
	ctx := createTestContext()

	// Set initial value
	ctx.SetHeader("X-Test-Header", "initial-value")

	// Overwrite with new value
	ctx.SetHeader("X-Test-Header", "new-value")

	header := ctx.w.Header().Get("X-Test-Header")
	assert.Equal(t, "new-value", header)
}

func TestWoContext_AppendHeader(t *testing.T) {
	ctx := createTestContext()

	// Set initial value
	ctx.SetHeader("X-Test-Header", "value1")

	// Append another value
	ctx.AppendHeader("X-Test-Header", "value2")

	// Check both values exist
	headers := ctx.w.Header()["X-Test-Header"]
	assert.Len(t, headers, 2)
	assert.Contains(t, headers, "value1")
	assert.Contains(t, headers, "value2")
}

func TestWoContext_BodyWriter(t *testing.T) {
	ctx := createTestContext()

	writer := ctx.BodyWriter()
	assert.NotNil(t, writer)
	assert.Equal(t, ctx.w, writer)

	// Test writing to the body
	testData := []byte("test response data")
	n, err := writer.Write(testData)
	require.NoError(t, err)
	assert.Equal(t, len(testData), n)

	// Verify the data was written
	assert.Equal(t, testData, ctx.w.(*httptest.ResponseRecorder).Body.Bytes())
}

func TestWoContext_ImplementsHumaContext(t *testing.T) {
	// This test ensures that woContext implements huma.Context interface
	var _ huma.Context = (*kContext)(nil)

	ctx := createTestContext()
	assert.Implements(t, (*huma.Context)(nil), ctx)
}

func TestWoContext_Clear(t *testing.T) {
	ctx := createTestContext()

	// Clear the context
	ctx.reset(nil, nil, nil)

	// Verify state is cleared
	assert.Nil(t, ctx.op)
	assert.Nil(t, ctx.w)
	assert.Nil(t, ctx.r)
}

// Benchmark tests
func BenchmarkWoContext_Query(b *testing.B) {
	ctx := createTestContext()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ctx.Query("foo")
	}
}

func BenchmarkWoContext_Header(b *testing.B) {
	ctx := createTestContext()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ctx.Header("Content-Type")
	}
}

func BenchmarkWoContext_BodyWriter(b *testing.B) {
	ctx := createTestContext()
	data := []byte("test data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ctx.BodyWriter().Write(data)
	}
}
