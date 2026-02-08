package keratin

import (
	"sort"

	"github.com/google/uuid"
)

type Middleware struct {
	ID       string
	Priority int
	Func     func(Handler) Handler
}

type Middlewares []*Middleware

func (mdw Middlewares) build(handler Handler) Handler {
	sort.SliceStable(mdw, func(i, j int) bool {
		return mdw[i].Priority < mdw[j].Priority
	})

	for i := len(mdw) - 1; i >= 0; i-- {
		if mdw[i].ID == "" {
			mdw[i].ID = uuid.NewString()
		}
		handler = mdw[i].Func(handler)
	}

	return handler
}
