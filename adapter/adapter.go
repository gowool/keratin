package adapter

import (
	"net/http"
	"sync"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gowool/keratin"
)

var _ huma.Adapter = (*Adapter)(nil)

type router interface {
	RouteFunc(method string, path string, handler func(http.ResponseWriter, *http.Request) error) *keratin.Route
}

type Adapter struct {
	http.Handler
	router router
	pool   *sync.Pool
}

func NewAdapter(handler http.Handler, router router) *Adapter {
	return &Adapter{
		Handler: handler,
		router:  router,
		pool:    &sync.Pool{New: func() any { return new(kContext) }},
	}
}

func (a *Adapter) Handle(op *huma.Operation, handler func(huma.Context)) {
	a.router.RouteFunc(op.Method, op.Path, func(w http.ResponseWriter, r *http.Request) error {
		ctx := a.pool.Get().(*kContext)
		ctx.reset(op, r, w)

		defer func() {
			ctx.reset(nil, nil, nil)
			a.pool.Put(ctx)
		}()

		handler(ctx)

		return nil
	})
}
