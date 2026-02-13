package keratin

import "context"

var nilKCtx = new(kContext)

type Context interface {
	Scheme() string
	RealIP() string
	Pattern() string
	Methods() string
	AnyMethods() bool
}

func FromContext(ctx context.Context) Context {
	if c, ok := ctx.Value(ctxKey{}).(*kContext); ok {
		return c
	}
	return nilKCtx
}

type ctxKey struct{}

type kContext struct {
	scheme     string
	realIP     string
	pattern    string
	methods    string
	anyMethods bool
	err        error
}

func (c *kContext) reset() {
	c.scheme = ""
	c.realIP = ""
	c.pattern = ""
	c.methods = ""
	c.anyMethods = false
	c.err = nil
}

func (c *kContext) Scheme() string {
	return c.scheme
}

func (c *kContext) RealIP() string {
	return c.realIP
}

func (c *kContext) Pattern() string {
	return c.pattern
}

func (c *kContext) Methods() string {
	return c.methods
}

func (c *kContext) AnyMethods() bool {
	return c.anyMethods
}
