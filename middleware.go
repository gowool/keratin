package keratin

import (
	"net/http"
	"sort"

	"github.com/google/uuid"
)

type Middleware struct {
	ID       string
	Priority int
	Func     func(Handler) Handler
}

type Middlewares []*Middleware

func (mws Middlewares) build(handler Handler) Handler {
	sort.SliceStable(mws, func(i, j int) bool {
		return mws[i].Priority < mws[j].Priority
	})

	for i := len(mws) - 1; i >= 0; i-- {
		if mws[i].ID == "" {
			mws[i].ID = uuid.NewString()
		}
		handler = mws[i].Func(handler)
	}

	return handler
}

type HTTPMiddleware struct {
	ID       string
	Priority int
	Func     func(http.Handler) http.Handler
}

type HTTPMiddlewares []*HTTPMiddleware

func (mws HTTPMiddlewares) build(handler http.Handler) http.Handler {
	sort.SliceStable(mws, func(i, j int) bool {
		return mws[i].Priority < mws[j].Priority
	})

	for i := len(mws) - 1; i >= 0; i-- {
		if mws[i].ID == "" {
			mws[i].ID = uuid.NewString()
		}
		handler = mws[i].Func(handler)
	}

	return handler
}
