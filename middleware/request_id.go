package middleware

import (
	"context"
	"net/http"

	"github.com/gowool/keratin"
)

type reqIDKey struct{}

func CtxRequestID(ctx context.Context) string {
	value, _ := ctx.Value(reqIDKey{}).(string)
	return value
}

// RequestIDConfig defines the config for RequestID middleware.
type RequestIDConfig struct {
	// Generator defines a function to generate an ID.
	// Optional. Default value random.String(32).
	Generator func() string

	// TargetHeader defines what header to look for to populate the id.
	// Optional. Default value is `X-Request-Id`
	TargetHeader string
}

func (c *RequestIDConfig) SetDefaults() {
	if c.Generator == nil {
		c.Generator = createRandomStringGenerator(32)
	}
	if c.TargetHeader == "" {
		c.TargetHeader = keratin.HeaderXRequestID
	}
}

// RequestID returns a middleware that reads RequestIDConfig.TargetHeader (`X-Request-ID`) header value or when
// the header value is empty, generates that value and sets request ID to response
// as RequestIDConfig.TargetHeader (`X-Request-Id`) value.
func RequestID(cfg RequestIDConfig, skippers ...Skipper) func(keratin.Handler) keratin.Handler {
	cfg.SetDefaults()

	skip := ChainSkipper(skippers...)

	return func(next keratin.Handler) keratin.Handler {
		return keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			if skip(r) {
				return next.ServeHTTP(w, r)
			}

			rid := r.Header.Get(cfg.TargetHeader)
			if rid == "" {
				rid = cfg.Generator()
			}

			w.Header().Set(cfg.TargetHeader, rid)

			ctx := context.WithValue(r.Context(), reqIDKey{}, rid)

			return next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
