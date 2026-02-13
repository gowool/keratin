package ratelimit

import (
	"net/http"
	"strings"

	"github.com/gowool/keratin"
	"github.com/gowool/keratin/middleware"
)

func HTTPMiddleware(limiter *Limiter, skippers ...middleware.Skipper) func(http.Handler) http.Handler {
	if limiter == nil {
		panic("ratelimit: http middleware: limiter is required")
	}

	skip := middleware.ChainSkipper(skippers...)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skip(r) {
				next.ServeHTTP(w, r)
				return
			}

			if err := limiter.Allow(w, r); err != nil {
				code := keratin.HTTPErrorStatusCode(err)

				message := http.StatusText(code)
				if code == ErrRateLimitExceeded.Code {
					message = ErrRateLimitExceeded.Message
				}

				if strings.Contains(r.Header.Get(keratin.HeaderAccept), keratin.MIMEApplicationJSON) {
					if err := keratin.JSON(w, code, map[string]string{"message": message}); err == nil || keratin.ResponseCommitted(w) {
						return
					}
				}

				http.Error(w, message, code)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func Middleware(limiter *Limiter, skippers ...middleware.Skipper) func(keratin.Handler) keratin.Handler {
	if limiter == nil {
		panic("ratelimit: middleware: limiter is required")
	}

	skip := middleware.ChainSkipper(skippers...)

	return func(next keratin.Handler) keratin.Handler {
		return keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			if skip(r) {
				return next.ServeHTTP(w, r)
			}

			if err := limiter.Allow(w, r); err != nil {
				return err
			}

			return next.ServeHTTP(w, r)
		})
	}
}
