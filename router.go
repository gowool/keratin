package keratin

import (
	"context"
	"iter"
	"maps"
	"net/http"
	"sync"

	"github.com/gowool/keratin/internal"
)

// MultipartMaxMemory is the maximum memory to use when parsing multipart form data.
var MultipartMaxMemory int64 = 8 * 1024

type Option func(*Router)

func WithErrorHandler(errorHandler ErrorHandlerFunc) Option {
	return func(router *Router) {
		if errorHandler != nil {
			router.errorHandler = errorHandler
		}
	}
}

func WithIPExtractor(ipExtractor IPExtractor) Option {
	return func(router *Router) {
		if ipExtractor != nil {
			router.ipExtractor = ipExtractor
		}
	}
}

func WithResponseInterceptor(interceptor func(w http.ResponseWriter) (http.ResponseWriter, func())) Option {
	return func(router *Router) {
		if interceptor != nil {
			router.rwInterceptors = append(router.rwInterceptors, interceptor)
		}
	}
}

func WithRequestInterceptor(interceptor func(r *http.Request) (*http.Request, func())) Option {
	return func(router *Router) {
		if interceptor != nil {
			router.reqInterceptors = append(router.reqInterceptors, interceptor)
		}
	}
}

type rPattern struct {
	pattern    string
	methods    string
	anyMethods bool
}

type Router struct {
	*RouterGroup

	rwInterceptors  internal.Interceptors[http.ResponseWriter]
	reqInterceptors internal.Interceptors[*http.Request]
	patterns        map[string]struct{}
	rPatterns       map[string]*rPattern
	ctxPool         sync.Pool
	resPool         sync.Pool
	ipExtractor     IPExtractor
	errorHandler    ErrorHandlerFunc
	PreMiddlewares  Middlewares[Handler]
	HTTPMiddlewares Middlewares[http.Handler]
}

func NewRouter(options ...Option) *Router {
	r := &Router{
		RouterGroup:  new(RouterGroup),
		patterns:     make(map[string]struct{}),
		rPatterns:    make(map[string]*rPattern),
		resPool:      sync.Pool{New: func() any { return new(response) }},
		ctxPool:      sync.Pool{New: func() any { return new(kContext) }},
		errorHandler: DefaultErrorHandler,
		ipExtractor:  RemoteIP,
	}

	r.rwInterceptors = append(r.rwInterceptors, r.responseInterceptor)
	r.reqInterceptors = append(r.reqInterceptors, r.requestInterceptor)

	for _, option := range options {
		option(r)
	}

	return r
}

// Patterns returns a sequence of all route patterns currently registered in the router as strings.
func (r *Router) Patterns() iter.Seq[string] {
	return maps.Keys(r.patterns)
}

// PreHTTPFunc registers one or multiple HTTP middleware to be executed before all middlewares.
func (r *Router) PreHTTPFunc(middlewareFuncs ...func(next http.Handler) http.Handler) {
	for _, mdw := range middlewareFuncs {
		r.HTTPMiddlewares = append(r.HTTPMiddlewares, &Middleware[http.Handler]{Func: mdw})
	}
}

// PreHTTP registers one or multiple HTTP middleware to be executed before all middlewares.
func (r *Router) PreHTTP(middlewares ...*Middleware[http.Handler]) {
	r.HTTPMiddlewares = append(r.HTTPMiddlewares, middlewares...)
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
		r.PreMiddlewares = append(r.PreMiddlewares, &Middleware[Handler]{Func: mdw})
	}
}

// Pre registers one or multiple middleware handlers which are run before router
// tries to find matching route.
func (r *Router) Pre(middlewares ...*Middleware[Handler]) {
	r.PreMiddlewares = append(r.PreMiddlewares, middlewares...)
}

func (r *Router) Build() http.Handler {
	return r.BuildWithMux(http.NewServeMux())
}

func (r *Router) BuildWithMux(mux *http.ServeMux) http.Handler {
	r.build(mux, r.RouterGroup, nil)

	handler := r.PreMiddlewares.build(HandlerFunc(func(w http.ResponseWriter, req *http.Request) error {
		mux.ServeHTTP(w, req)

		return req.Context().Value(ctxKey{}).(*kContext).err
	}))

	httpHandler := r.HTTPMiddlewares.build(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if err := handler.ServeHTTP(w, req); err != nil {
			r.errorHandler(w, req, err)
		}
	}))

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w, cancelW := r.rwInterceptors.Apply(w)
		defer cancelW()

		req, cancelReq := r.reqInterceptors.Apply(req)
		defer cancelReq()

		httpHandler.ServeHTTP(w, req)
	})
}

func (r *Router) build(mux *http.ServeMux, group *RouterGroup, parents []*RouterGroup) {
	for _, child := range group.children {
		switch v := child.(type) {
		case *RouterGroup:
			r.build(mux, v, append(parents, group))
		case *Route:
			var (
				pattern     string
				middlewares Middlewares[Handler]
			)

			// add parent groups Middlewares
			for _, p := range parents {
				pattern += p.prefix
				middlewares = append(middlewares, p.Middlewares...)
			}

			// add current groups Middlewares
			pattern += group.prefix
			middlewares = append(middlewares, group.Middlewares...)

			// add current route Middlewares
			pattern += v.Path
			middlewares = append(middlewares, v.Middlewares...)

			rp, ok := r.rPatterns[pattern]
			if !ok {
				rp = &rPattern{pattern: pattern}
				r.rPatterns[pattern] = rp
			}

			if v.Method == "" {
				rp.anyMethods = true
			} else {
				if rp.methods == "" {
					rp.methods = v.Method
				} else {
					rp.methods += "," + v.Method
				}

				pattern = v.Method + " " + pattern
			}

			r.patterns[pattern] = struct{}{}

			handler := middlewares.build(v.Handler)

			mux.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
				c := req.Context().Value(ctxKey{}).(*kContext)

				if current, ok := r.rPatterns[Pattern(req)]; ok {
					c.pattern = current.pattern
					c.methods = current.methods
					c.anyMethods = current.anyMethods
				}

				c.err = handler.ServeHTTP(w, req)
			})
		}
	}
}

func (r *Router) responseInterceptor(w http.ResponseWriter) (http.ResponseWriter, func()) {
	res := r.resPool.Get().(*response)
	res.reset(w)

	cancel := func() {
		res.reset(nil)
		r.resPool.Put(res)
	}

	return res, cancel
}

func (r *Router) requestInterceptor(req *http.Request) (*http.Request, func()) {
	c := r.ctxPool.Get().(*kContext)
	cancel := func() {
		c.reset()
		r.ctxPool.Put(c)
	}

	c.scheme = Scheme(req)
	c.realIP = r.ipExtractor(req)

	ctx := context.WithValue(req.Context(), ctxKey{}, c)
	req = req.WithContext(ctx)

	return req, cancel
}
