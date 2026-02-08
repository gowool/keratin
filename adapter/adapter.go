package adapter

import (
	"net/http"
	"sync"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gowool/keratin"
)

var _ huma.Adapter = (*Adapter)(nil)

type router interface {
	Route(method string, path string, handler keratin.Handler) *keratin.Route
	ServeHTTP(http.ResponseWriter, *http.Request)
}

type Adapter struct {
	router
	pool *sync.Pool
}

func NewAdapter(router router) *Adapter {
	return &Adapter{
		router: router,
		pool:   &sync.Pool{New: func() any { return new(rContext) }},
	}
}

func (a *Adapter) Handle(op *huma.Operation, handler func(huma.Context)) {
	a.Route(op.Method, op.Path, keratin.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		ctx := a.pool.Get().(*rContext)
		ctx.reset(op, r, w)

		defer func() {
			ctx.reset(nil, nil, nil)
			a.pool.Put(ctx)
		}()

		handler(ctx)
		return nil
	}))
}
