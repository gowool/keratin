package session

import (
	"log/slog"
	"net/http"
	"sync"

	"github.com/gowool/keratin/middleware"
)

func Middleware(registry *Registry, logger *slog.Logger, skippers ...middleware.Skipper) func(next http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	logger = logger.WithGroup("session")

	skip := middleware.ChainSkipper(skippers...)

	pool := &sync.Pool{New: func() any { return new(sessionWriter) }}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(registry.All()) == 0 || skip(r) {
				next.ServeHTTP(w, r)
				return
			}

			r, err := registry.ReadSessions(r)
			if err != nil {
				logger.Error("failed to read sessions", "error", err)

				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

				return
			}

			response := pool.Get().(*sessionWriter)
			response.reset(w, r, registry, logger)

			defer func() {
				response.reset(nil, nil, nil, nil)
				pool.Put(response)
			}()

			next.ServeHTTP(response, r)
		})
	}
}

type sessionWriter struct {
	http.ResponseWriter
	request  *http.Request
	registry *Registry
	logger   *slog.Logger
}

func (sw *sessionWriter) reset(w http.ResponseWriter, request *http.Request, registry *Registry, logger *slog.Logger) {
	sw.ResponseWriter = w
	sw.request = request
	sw.registry = registry
	sw.logger = logger
}

func (sw *sessionWriter) WriteHeader(code int) {
	if err := sw.registry.WriteSessions(sw, sw.request); err != nil {
		sw.logger.Error("failed to write sessions", "error", err)
	}
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *sessionWriter) Unwrap() http.ResponseWriter {
	return sw.ResponseWriter
}
