package session

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

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

			for _, s := range registry.All() {
				s := s

				req, err := s.ReadSessionCookie(r)
				if err != nil {
					return err
				}

				r = req

				response.before = append(response.before, func() {
					ctx := req.Context()

					switch s.Status(ctx) {
					case Modified:
						token, expiry, err := s.Commit(ctx)
						if err != nil {
							if logger != nil {
								logger.Error("failed to commit session", "name", s.config.Cookie.Name, "error", err)
							}
							return
						}

						s.WriteSessionCookie(ctx, response, token, expiry)
					case Destroyed:
						s.WriteSessionCookie(ctx, response, "", time.Time{})
					default:
					}
				})
			}

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
