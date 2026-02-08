package keratin

import "net/http"

type RouterGroup struct {
	Prefix      string
	Middlewares Middlewares
	children    []any // Route or Group
}

// Group creates and register a new child RouterGroup into the current one
// with the specified prefix.
//
// The prefix follows the standard Go net/http http.ServeMux pattern format ("[HOST]/[PATH]")
// and will be concatenated recursively into the final route path, meaning that
// only the root level group could have HOST as part of the prefix.
//
// Returns the newly created group to allow chaining and registering
// sub-routes and group specific middlewares.
func (group *RouterGroup) Group(prefix string) *RouterGroup {
	newGroup := new(RouterGroup)
	newGroup.Prefix = prefix

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
		group.Middlewares = append(group.Middlewares, &Middleware{Func: mdw})
	}

	return group
}

// Use registers one or multiple middleware handlers to the current group.
func (group *RouterGroup) Use(middlewares ...*Middleware) *RouterGroup {
	group.Middlewares = append(group.Middlewares, middlewares...)

	return group
}

// Route registers a single route into the current group.
//
// Note that the final route path will be the concatenation of all parent groups prefixes + the route path.
// The path follows the standard Go net/http http.ServeMux format ("[HOST]/[PATH]"),
// meaning that only a top level group route could have HOST as part of the prefix.
//
// Returns the newly created route to allow attaching route-only middlewares.
func (group *RouterGroup) Route(method string, path string, handler Handler) *Route {
	route := &Route{
		Method:  method,
		Path:    path,
		Handler: handler,
	}

	group.children = append(group.children, route)

	return route
}

// Any is a shorthand for [RouterGroup.Route] with "" as route method (aka. matches any method).
func (group *RouterGroup) Any(path string, handler Handler) *Route {
	return group.Route("", path, handler)
}

// GET is a shorthand for [RouterGroup.Route] with GET as route method.
func (group *RouterGroup) GET(path string, handler Handler) *Route {
	return group.Route(http.MethodGet, path, handler)
}

// SEARCH is a shorthand for [RouterGroup.Route] with SEARCH as route method.
func (group *RouterGroup) SEARCH(path string, handler Handler) *Route {
	return group.Route("SEARCH", path, handler)
}

// POST is a shorthand for [RouterGroup.Route] with POST as route method.
func (group *RouterGroup) POST(path string, handler Handler) *Route {
	return group.Route(http.MethodPost, path, handler)
}

// DELETE is a shorthand for [RouterGroup.Route] with DELETE as route method.
func (group *RouterGroup) DELETE(path string, handler Handler) *Route {
	return group.Route(http.MethodDelete, path, handler)
}

// PATCH is a shorthand for [RouterGroup.Route] with PATCH as route method.
func (group *RouterGroup) PATCH(path string, handler Handler) *Route {
	return group.Route(http.MethodPatch, path, handler)
}

// PUT is a shorthand for [RouterGroup.Route] with PUT as route method.
func (group *RouterGroup) PUT(path string, handler Handler) *Route {
	return group.Route(http.MethodPut, path, handler)
}

// HEAD is a shorthand for [RouterGroup.Route] with HEAD as route method.
func (group *RouterGroup) HEAD(path string, handler Handler) *Route {
	return group.Route(http.MethodHead, path, handler)
}

// OPTIONS is a shorthand for [RouterGroup.Route] with OPTIONS as route method.
func (group *RouterGroup) OPTIONS(path string, handler Handler) *Route {
	return group.Route(http.MethodOptions, path, handler)
}
