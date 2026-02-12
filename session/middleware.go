package session

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gowool/keratin"
	"github.com/gowool/keratin/middleware"
)

func Middleware(registry *Registry, logger *slog.Logger, skippers ...middleware.Skipper) func(keratin.Handler) keratin.Handler {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	logger = logger.WithGroup("session")

	skip := middleware.ChainSkipper(skippers...)

	pool := &sync.Pool{New: func() any { return new(sessionWriter) }}

	return func(next keratin.Handler) keratin.Handler {
		return keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			if len(registry.All()) == 0 || skip(r) {
				return next.ServeHTTP(w, r)
			}

			response := pool.Get().(*sessionWriter)
			response.reset(w)

			defer func() {
				response.reset(nil)
				pool.Put(response)
			}()

			req, err := registry.ReadSessions(r)
			if err != nil {
				return fmt.Errorf("failed to read sessions: %w", err)
			}
			r = req

			response.before = append(response.before, func() {
				if err := registry.WriteSessions(response, req); err != nil {
					logger.Error("failed to write sessions", "error", err)
				}
			})

			return next.ServeHTTP(response, r)
		})
	}
}

type sessionWriter struct {
	http.ResponseWriter
	before []func()
}

func (sw *sessionWriter) reset(w http.ResponseWriter) {
	sw.ResponseWriter = w
	sw.before = nil
}

func (sw *sessionWriter) WriteHeader(code int) {
	for _, fn := range sw.before {
		fn()
	}
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *sessionWriter) Unwrap() http.ResponseWriter {
	return sw.ResponseWriter
}
