package keratin

type Route struct {
	Method      string
	Path        string
	Handler     Handler
	Middlewares Middlewares
}

// UseFunc registers one or multiple middleware functions to the current route.
//
// The registered middleware functions are "anonymous" and with default priority,
// aka. executes in the order they were registered.
//
// If you need to specify a named middleware or middleware with custom exec prirority,
// use the [Route.Use] method.
func (route *Route) UseFunc(middlewareFuncs ...func(Handler) Handler) *Route {
	for _, mdw := range middlewareFuncs {
		route.Middlewares = append(route.Middlewares, &Middleware{Func: mdw})
	}

	return route
}

// Use registers one or multiple middleware handlers to the current route.
func (route *Route) Use(middlewares ...*Middleware) *Route {
	route.Middlewares = append(route.Middlewares, middlewares...)

	return route
}
