package ratelimit

import (
	"net/http"

	"github.com/gowool/keratin"
	"github.com/gowool/keratin/middleware"
)

func Middleware(limiter *Limiter, skippers ...middleware.Skipper) func(http.Handler) http.Handler {
	if limiter == nil {
		panic("ratelimit: middleware: limiter is required")
	}

	skip := middleware.ChainSkipper(skippers...)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skip(r) {
				next.ServeHTTP(w, r)
				return
			}

			if err := limiter.Allow(w, r); err != nil {
				keratin.DefaultErrorHandler(w, r, err)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
