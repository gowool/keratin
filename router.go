package keratin

import (
	"context"
	"iter"
	"maps"
	"net/http"
	"sync"
)

var _ http.Handler = (*Router)(nil)

type ctxErrKey struct{}

type ctxErr struct {
	err error
}

type Router struct {
	*RouterGroup

	patterns       map[string]struct{}
	once           sync.Once
	errPool        sync.Pool
	resPool        sync.Pool
	handler        http.Handler
	errorHandler   ErrorHandler
	PreMiddlewares Middlewares
}

func NewRouter(errorHandler ErrorHandler) *Router {
	if errorHandler == nil {
		panic("router: error handler is required")
	}

	return &Router{
		RouterGroup:  new(RouterGroup),
		patterns:     make(map[string]struct{}),
		resPool:      sync.Pool{New: func() any { return new(response) }},
		errPool:      sync.Pool{New: func() any { return new(ctxErr) }},
		errorHandler: errorHandler,
	}
}

// ServeHTTP handles HTTP requests by initializing the router's handler and delegating the request to it.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.once.Do(func() { r.handler = r.Build() })
	r.handler.ServeHTTP(w, req)
}

// Patterns returns a sequence of all route patterns currently registered in the router as strings.
func (r *Router) Patterns() iter.Seq[string] {
	return maps.Keys(r.patterns)
}

// PreFunc registers one or multiple middleware functions which are run before router
// tries to find matching route.
//
// The registered middleware functions are "anonymous" and with default priority,
// aka. executes in the order they were registered.
//
// If you need to specify a named middleware or middleware with custom exec priority,
// use [Router.Pre] method.
func (r *Router) PreFunc(middlewareFuncs ...func(Handler) Handler) {
	for _, mdw := range middlewareFuncs {
		r.PreMiddlewares = append(r.PreMiddlewares, &Middleware{Func: mdw})
	}
}

// Pre registers one or multiple middleware handlers which are run before router
// tries to find matching route.
func (r *Router) Pre(middlewares ...*Middleware) {
	r.PreMiddlewares = append(r.PreMiddlewares, middlewares...)
}

func (r *Router) Build() http.Handler {
	return r.BuildWithMux(http.NewServeMux())
}

func (r *Router) BuildWithMux(mux *http.ServeMux) http.Handler {
	r.build(mux, r.RouterGroup, nil)

	handler := r.PreMiddlewares.build(HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		mux.ServeHTTP(w, r)

		return r.Context().Value(ctxErrKey{}).(*ctxErr).err
	}))

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		res := r.resPool.Get().(*response)
		res.reset(w)
		defer func() {
			res.reset(nil)
			r.resPool.Put(res)
		}()

		c := r.errPool.Get().(*ctxErr)
		defer func() {
			c.err = nil
			r.errPool.Put(c)
		}()

		ctx := context.WithValue(req.Context(), ctxErrKey{}, c)

		req = req.WithContext(ctx)

		if err := handler.ServeHTTP(res, req); err != nil {
			r.errorHandler.ServeHTTP(res, req, err)
		}
	})
}

func (r *Router) build(mux *http.ServeMux, group *RouterGroup, parents []*RouterGroup) {
	for _, child := range group.children {
		switch v := child.(type) {
		case *RouterGroup:
			r.build(mux, v, append(parents, group))
		case *Route:
			var (
				routeMiddlewares Middlewares
				pattern          string
			)

			// add parent groups middlewares
			for _, p := range parents {
				pattern += p.Prefix
				routeMiddlewares = append(routeMiddlewares, p.Middlewares...)
			}

			// add current groups middlewares
			pattern += group.Prefix
			routeMiddlewares = append(routeMiddlewares, group.Middlewares...)

			// add current route middlewares
			pattern += v.Path
			routeMiddlewares = append(routeMiddlewares, v.Middlewares...)

			handler := routeMiddlewares.build(v.Handler)

			if v.Method != "" {
				pattern = v.Method + " " + pattern
			}

			r.patterns[pattern] = struct{}{}

			mux.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
				c := req.Context().Value(ctxErrKey{}).(*ctxErr)
				c.err = handler.ServeHTTP(w, req)
			})
		}
	}
}
