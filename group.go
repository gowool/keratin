package keratin

import (
	"net/http"
	"strings"
)

type RouterGroup struct {
	prefix      string
	children    []any // Route or Group
	Middlewares Middlewares[Handler]
}

// Group creates and register a new child RouterGroup into the current one
// with the specified prefix.
//
// The prefix follows the standard Go net/http http.ServeMux pattern format ("[HOST]/[PATH]")
// and will be concatenated recursively into the final route path, meaning that
// only the root level group could have HOST as part of the prefix.
//
// Returns the newly created group to allow chaining and registering
// sub-routes and group specific Middlewares.
func (group *RouterGroup) Group(prefix string) *RouterGroup {
	newGroup := new(RouterGroup)
	newGroup.prefix = prefix

	group.children = append(group.children, newGroup)

	return newGroup
}

// UseFunc registers one or multiple middleware functions to the current group.
//
// The registered middleware functions are "anonymous" and with default priority,
// aka. executes in the order they were registered.
//
// If you need to specify a named middleware or middleware with custom exec priority,
// use [RouterGroup.Use] method.
func (group *RouterGroup) UseFunc(middlewareFuncs ...func(Handler) Handler) *RouterGroup {
	for _, mdw := range middlewareFuncs {
		group.Middlewares = append(group.Middlewares, &Middleware[Handler]{Func: mdw})
	}

	return group
}

// Use registers one or multiple middleware handlers to the current group.
func (group *RouterGroup) Use(middlewares ...*Middleware[Handler]) *RouterGroup {
	group.Middlewares = append(group.Middlewares, middlewares...)

	return group
}

// Route registers a single route into the current group.
//
// Note that the final route path will be the concatenation of all parent groups prefixes + the route path.
// The path follows the standard Go net/http http.ServeMux format ("[HOST]/[PATH]"),
// meaning that only a top level group route could have HOST as part of the prefix.
//
// Returns the newly created route to allow attaching route-only Middlewares.
func (group *RouterGroup) Route(method string, path string, handler Handler) *Route {
	route := &Route{
		Method:  strings.ToUpper(method),
		Path:    path,
		Handler: handler,
	}

	group.children = append(group.children, route)

	return route
}

// RouteFunc registers a route in the current group using a function matching the http.HandlerFunc signature.
func (group *RouterGroup) RouteFunc(method string, path string, handler func(http.ResponseWriter, *http.Request) error) *Route {
	return group.Route(method, path, HandlerFunc(handler))
}

// Any is a shorthand for [RouterGroup.RouteFunc] with "" as route method (aka. matches any method).
func (group *RouterGroup) Any(path string, handler func(http.ResponseWriter, *http.Request) error) *Route {
	return group.RouteFunc("", path, handler)
}

// GET is a shorthand for [RouterGroup.RouteFunc] with GET as route method.
func (group *RouterGroup) GET(path string, handler func(http.ResponseWriter, *http.Request) error) *Route {
	return group.RouteFunc(http.MethodGet, path, handler)
}

// HEAD is a shorthand for [RouterGroup.RouteFunc] with HEAD as route method.
func (group *RouterGroup) HEAD(path string, handler func(http.ResponseWriter, *http.Request) error) *Route {
	return group.RouteFunc(http.MethodHead, path, handler)
}

// POST is a shorthand for [RouterGroup.RouteFunc] with POST as route method.
func (group *RouterGroup) POST(path string, handler func(http.ResponseWriter, *http.Request) error) *Route {
	return group.RouteFunc(http.MethodPost, path, handler)
}

// PUT is a shorthand for [RouterGroup.RouteFunc] with PUT as route method.
func (group *RouterGroup) PUT(path string, handler func(http.ResponseWriter, *http.Request) error) *Route {
	return group.RouteFunc(http.MethodPut, path, handler)
}

// PATCH is a shorthand for [RouterGroup.RouteFunc] with PATCH as route method.
func (group *RouterGroup) PATCH(path string, handler func(http.ResponseWriter, *http.Request) error) *Route {
	return group.RouteFunc(http.MethodPatch, path, handler)
}

// DELETE is a shorthand for [RouterGroup.RouteFunc] with DELETE as route method.
func (group *RouterGroup) DELETE(path string, handler func(http.ResponseWriter, *http.Request) error) *Route {
	return group.RouteFunc(http.MethodDelete, path, handler)
}

// CONNECT is a shorthand for [RouterGroup.RouteFunc] with CONNECT as route method.
func (group *RouterGroup) CONNECT(path string, handler func(http.ResponseWriter, *http.Request) error) *Route {
	return group.RouteFunc(http.MethodConnect, path, handler)
}

// OPTIONS is a shorthand for [RouterGroup.RouteFunc] with OPTIONS as route method.
func (group *RouterGroup) OPTIONS(path string, handler func(http.ResponseWriter, *http.Request) error) *Route {
	return group.RouteFunc(http.MethodOptions, path, handler)
}

// TRACE is a shorthand for [RouterGroup.RouteFunc] with TRACE as route method.
func (group *RouterGroup) TRACE(path string, handler func(http.ResponseWriter, *http.Request) error) *Route {
	return group.RouteFunc(http.MethodTrace, path, handler)
}

// SEARCH is a shorthand for [RouterGroup.RouteFunc] with SEARCH as route method.
func (group *RouterGroup) SEARCH(path string, handler func(http.ResponseWriter, *http.Request) error) *Route {
	return group.RouteFunc("SEARCH", path, handler)
}
