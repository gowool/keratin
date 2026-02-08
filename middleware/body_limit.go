package middleware

import (
	"io"
	"net/http"

	"github.com/gowool/keratin"
)

const maxBodySize int64 = 32 << 20

type BodyLimitConfig struct {
	// Maximum allowed size for a request body, default is 32MB.
	// If Limit is less to 0, no limit is applied.
	Limit int64 `env:"LIMIT" json:"limit,omitempty" yaml:"limit,omitempty"`
}

func (c *BodyLimitConfig) SetDefaults() {
	if c.Limit == 0 {
		c.Limit = maxBodySize
	}
}

func BodyLimit(cfg BodyLimitConfig, skippers ...Skipper) func(keratin.Handler) keratin.Handler {
	cfg.SetDefaults()

	skip := ChainSkipper(skippers...)

	return func(next keratin.Handler) keratin.Handler {
		return keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			if skip(r) || cfg.Limit <= 0 {
				return next.ServeHTTP(w, r)
			}

			// optimistically check the submitted request content length
			if r.ContentLength > cfg.Limit {
				return keratin.ErrRequestEntityTooLarge
			}

			// replace the request body
			//
			// note: we don't use sync.Pool since the size of the elements could vary too much
			// and it might not be efficient (see https://github.com/golang/go/issues/23199)
			r.Body = &limitedReader{ReadCloser: r.Body, limit: cfg.Limit}

			return next.ServeHTTP(w, r)
		})
	}
}

type limitedReader struct {
	io.ReadCloser
	limit     int64
	totalRead int64
}

func (lr *limitedReader) Read(b []byte) (int, error) {
	n, err := lr.ReadCloser.Read(b)
	if err != nil {
		return n, err
	}

	lr.totalRead += int64(n)
	if lr.totalRead > lr.limit {
		return n, keratin.ErrRequestEntityTooLarge
	}
	return n, nil
}

func (lr *limitedReader) Reread() {
	if rr, ok := lr.ReadCloser.(interface{ Reread() }); ok {
		rr.Reread()
	}
}
