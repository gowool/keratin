package keratin

import (
	"sort"

	"github.com/google/uuid"
)

type Middleware[H any] struct {
	ID       string
	Priority int
	Func     func(H) H
}

type Middlewares[H any] []*Middleware[H]

func (mws Middlewares[H]) build(handler H) H {
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
