package adapter

import (
	"context"
	"crypto/tls"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/queryparam"
	"github.com/gowool/keratin"
)

// Unwrap extracts the underlying HTTP request and response writer from a Huma
// context. If passed a context from a different adapter it will panic.
func Unwrap(ctx huma.Context) (*http.Request, http.ResponseWriter) {
	for {
		if c, ok := ctx.(interface{ Unwrap() huma.Context }); ok {
			ctx = c.Unwrap()
			continue
		}
		break
	}
	if c, ok := ctx.(*kContext); ok {
		return c.Unwrap()
	}
	panic(`not a "keratin" context`)
}

type kContext struct {
	op     *huma.Operation
	r      *http.Request
	w      http.ResponseWriter
	status int
}

// NewContext creates a new Huma context from an HTTP request and response.
func NewContext(op *huma.Operation, req *http.Request, w http.ResponseWriter) huma.Context {
	return &kContext{op: op, r: req, w: w}
}

func (c *kContext) reset(op *huma.Operation, req *http.Request, w http.ResponseWriter) {
	c.op = op
	c.r = req
	c.w = w
	c.status = 0
}

func (c *kContext) Unwrap() (*http.Request, http.ResponseWriter) {
	return c.r, c.w
}

func (c *kContext) Operation() *huma.Operation {
	return c.op
}

func (c *kContext) Context() context.Context {
	return c.r.Context()
}

func (c *kContext) Method() string {
	return c.r.Method
}

func (c *kContext) Host() string {
	return c.r.Host
}

func (c *kContext) RemoteAddr() string {
	return c.r.RemoteAddr
}

func (c *kContext) URL() url.URL {
	return *c.r.URL
}

func (c *kContext) Param(name string) string {
	return c.r.PathValue(name)
}

func (c *kContext) Query(name string) string {
	return queryparam.Get(c.r.URL.RawQuery, name)
}

func (c *kContext) Header(name string) string {
	return c.r.Header.Get(name)
}

func (c *kContext) EachHeader(cb func(name, value string)) {
	for name, values := range c.r.Header {
		for _, value := range values {
			cb(name, value)
		}
	}
}

func (c *kContext) BodyReader() io.Reader {
	return c.r.Body
}

func (c *kContext) GetMultipartForm() (*multipart.Form, error) {
	err := c.r.ParseMultipartForm(keratin.MultipartMaxMemory)
	return c.r.MultipartForm, err
}

func (c *kContext) SetReadDeadline(deadline time.Time) error {
	return huma.SetReadDeadline(c.w, deadline)
}

func (c *kContext) SetStatus(code int) {
	c.status = code
	c.w.WriteHeader(code)
}

func (c *kContext) Status() int {
	return c.status
}

func (c *kContext) AppendHeader(name string, value string) {
	c.w.Header().Add(name, value)
}

func (c *kContext) SetHeader(name string, value string) {
	c.w.Header().Set(name, value)
}

func (c *kContext) BodyWriter() io.Writer {
	return c.w
}

func (c *kContext) TLS() *tls.ConnectionState {
	return c.r.TLS
}

func (c *kContext) Version() huma.ProtoVersion {
	return huma.ProtoVersion{
		Proto:      c.r.Proto,
		ProtoMajor: c.r.ProtoMajor,
		ProtoMinor: c.r.ProtoMinor,
	}
}
