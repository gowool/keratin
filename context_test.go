package keratin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKContext_RealIP(t *testing.T) {
	tests := []struct {
		name string
		ip   string
	}{
		{"IPv4 address", "192.168.1.1"},
		{"IPv6 address", "2001:0db8:0000:0000:0000:0000:0000:0001"},
		{"localhost", "127.0.0.1"},
		{"empty IP", ""},
		{"private IP", "10.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &kContext{realIP: tt.ip}
			got := ctx.RealIP()
			require.Equal(t, tt.ip, got)
		})
	}
}

func TestKContext_Pattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
	}{
		{"simple path", "/users"},
		{"path with parameter", "/users/:id"},
		{"root path", "/"},
		{"wildcard path", "/*"},
		{"nested path", "/api/v1/users"},
		{"complex pattern", "/api/:version/:resource/:id"},
		{"empty pattern", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &kContext{pattern: tt.pattern}
			got := ctx.Pattern()
			require.Equal(t, tt.pattern, got)
		})
	}
}

func TestKContext_Methods(t *testing.T) {
	tests := []struct {
		name    string
		methods string
	}{
		{"single method", "GET"},
		{"multiple methods", "GET,POST,PUT,DELETE"},
		{"empty methods", ""},
		{"all methods", "GET,POST,PUT,PATCH,DELETE,HEAD,OPTIONS"},
		{"two methods", "GET,POST"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &kContext{methods: tt.methods}
			got := ctx.Methods()
			require.Equal(t, tt.methods, got)
		})
	}
}

func TestKContext_AnyMethods(t *testing.T) {
	tests := []struct {
		name       string
		anyMethods bool
	}{
		{"any methods true", true},
		{"any methods false", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &kContext{anyMethods: tt.anyMethods}
			got := ctx.AnyMethods()
			require.Equal(t, tt.anyMethods, got)
		})
	}
}

func TestKContext_reset(t *testing.T) {
	tests := []struct {
		name  string
		input *kContext
		want  *kContext
	}{
		{
			name: "reset populated context",
			input: &kContext{
				realIP:     "192.168.1.1",
				pattern:    "/users/:id",
				methods:    "GET,POST",
				anyMethods: true,
				err:        nil,
			},
			want: &kContext{
				realIP:     "",
				pattern:    "",
				methods:    "",
				anyMethods: false,
				err:        nil,
			},
		},
		{
			name:  "reset empty context",
			input: &kContext{},
			want: &kContext{
				realIP:     "",
				pattern:    "",
				methods:    "",
				anyMethods: false,
				err:        nil,
			},
		},
		{
			name: "reset with error",
			input: &kContext{
				realIP:  "10.0.0.1",
				pattern: "/",
				methods: "GET",
				err:     context.Canceled,
			},
			want: &kContext{
				realIP:     "",
				pattern:    "",
				methods:    "",
				anyMethods: false,
				err:        nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.input.reset()
			require.Equal(t, tt.want.realIP, tt.input.realIP)
			require.Equal(t, tt.want.pattern, tt.input.pattern)
			require.Equal(t, tt.want.methods, tt.input.methods)
			require.Equal(t, tt.want.anyMethods, tt.input.anyMethods)
			require.Equal(t, tt.want.err, tt.input.err)
		})
	}
}

func TestFromContext(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		wantReal string
		wantNil  bool
	}{
		{
			name:     "context with kContext returns it",
			ctx:      context.WithValue(context.Background(), ctxKey{}, &kContext{realIP: "192.168.1.1"}),
			wantReal: "192.168.1.1",
			wantNil:  false,
		},
		{
			name:     "context without kContext returns nilKCtx",
			ctx:      context.Background(),
			wantReal: "",
			wantNil:  false,
		},
		{
			name:     "context with different key type returns nilKCtx",
			ctx:      context.WithValue(context.Background(), "different-key", &kContext{realIP: "10.0.0.1"}),
			wantReal: "",
			wantNil:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromContext(tt.ctx)
			require.NotNil(t, got)
			require.Equal(t, tt.wantReal, got.RealIP())
		})
	}
}

func TestKContext_ImplementsContext(t *testing.T) {
	ctx := &kContext{
		realIP:     "192.168.1.1",
		pattern:    "/users/:id",
		methods:    "GET,POST",
		anyMethods: false,
	}

	var _ Context = ctx

	require.Equal(t, "192.168.1.1", ctx.RealIP())
	require.Equal(t, "/users/:id", ctx.Pattern())
	require.Equal(t, "GET,POST", ctx.Methods())
	require.Equal(t, false, ctx.AnyMethods())
}

func TestKContext_FullCycle(t *testing.T) {
	ctx := &kContext{
		realIP:     "2001:0db8:0000:0000:0000:0000:0000:0001",
		pattern:    "/api/v1/resources/:id",
		methods:    "GET,POST,PUT,PATCH,DELETE",
		anyMethods: false,
	}

	require.Equal(t, "2001:0db8:0000:0000:0000:0000:0000:0001", ctx.RealIP())
	require.Equal(t, "/api/v1/resources/:id", ctx.Pattern())
	require.Equal(t, "GET,POST,PUT,PATCH,DELETE", ctx.Methods())
	require.Equal(t, false, ctx.AnyMethods())

	ctx.reset()

	require.Equal(t, "", ctx.RealIP())
	require.Equal(t, "", ctx.Pattern())
	require.Equal(t, "", ctx.Methods())
	require.Equal(t, false, ctx.AnyMethods())
}

func TestNilKCtx(t *testing.T) {
	tests := []struct {
		name       string
		method     func() interface{}
		wantNil    bool
		wantString string
	}{
		{
			name:       "RealIP returns empty string",
			method:     func() interface{} { return nilKCtx.RealIP() },
			wantNil:    false,
			wantString: "",
		},
		{
			name:       "Pattern returns empty string",
			method:     func() interface{} { return nilKCtx.Pattern() },
			wantNil:    false,
			wantString: "",
		},
		{
			name:       "Methods returns empty string",
			method:     func() interface{} { return nilKCtx.Methods() },
			wantNil:    false,
			wantString: "",
		},
		{
			name:       "AnyMethods returns false",
			method:     func() interface{} { return nilKCtx.AnyMethods() },
			wantNil:    false,
			wantString: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.method()
			if tt.wantString != "" {
				require.Equal(t, tt.wantString, got)
			}
			if tt.wantNil {
				require.Nil(t, got)
			} else {
				require.NotNil(t, got)
			}
		})
	}
}

func TestKContext_ContextKeyUniqueness(t *testing.T) {
	ctx := context.Background()
	ctx1 := context.WithValue(ctx, ctxKey{}, &kContext{realIP: "192.168.1.1"})
	ctx2 := context.WithValue(ctx, ctxKey{}, &kContext{realIP: "10.0.0.1"})

	got1 := FromContext(ctx1)
	got2 := FromContext(ctx2)

	require.NotEqual(t, got1.RealIP(), got2.RealIP())
	require.Equal(t, "192.168.1.1", got1.RealIP())
	require.Equal(t, "10.0.0.1", got2.RealIP())
}

func TestKContext_WithAllFields(t *testing.T) {
	ctx := &kContext{
		realIP:     "203.0.113.1",
		pattern:    "/api/v2/organizations/:org/users/:user",
		methods:    "GET,HEAD,OPTIONS",
		anyMethods: true,
		err:        context.DeadlineExceeded,
	}

	require.Equal(t, "203.0.113.1", ctx.RealIP())
	require.Equal(t, "/api/v2/organizations/:org/users/:user", ctx.Pattern())
	require.Equal(t, "GET,HEAD,OPTIONS", ctx.Methods())
	require.Equal(t, true, ctx.AnyMethods())
}
