package keratin

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type IPExtractor func(r *http.Request) string

// RemoteIP returns the IP address of the client that sent the request.
//
// IPv6 addresses are returned expanded.
// For example, "2001:db8::1" becomes "2001:0db8:0000:0000:0000:0000:0000:0001".
//
// Note that if you are behind reverse proxy(ies), this method returns
// the IP of the last connecting proxy.
func RemoteIP(r *http.Request) string {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	parsed, _ := netip.ParseAddr(ip)
	return parsed.StringExpanded()
}

// TrustedProxy defines the trusted proxy settings.
// https://developers.cloudflare.com/fundamentals/reference/http-headers/#x-forwarded-for
type TrustedProxy struct {
	// Headers is a list of explicit trusted header(s) to check.
	Headers []string `env:"HEADERS" json:"headers,omitempty" yaml:"headers,omitempty"`

	// UseLeftmostIP specifies to use the left-mostish IP from the trusted headers.
	//
	// Note that this could be insecure when used with X-Forwarded-For header
	// because some proxies like AWS ELB allow users to prepend their own header value
	// before appending the trusted ones.
	UseLeftmostIP bool `env:"USE_LEFTMOST_IP" json:"useLeftmostIP,omitempty" yaml:"useLeftmostIP,omitempty"`
}

func RealIP(fn func(ctx context.Context) (*TrustedProxy, error)) IPExtractor {
	return func(r *http.Request) string {
		if trusted, err := fn(r.Context()); err == nil {
			for _, h := range trusted.Headers {
				headerValues := r.Header.Values(h)
				if len(headerValues) == 0 {
					continue
				}

				// extract the last header value as it is expected to be the one controlled by the proxy
				ipsList := headerValues[len(headerValues)-1]
				if ipsList == "" {
					continue
				}

				ips := strings.Split(ipsList, ",")

				if trusted.UseLeftmostIP {
					for _, ip := range ips {
						if parsed, err := netip.ParseAddr(strings.TrimSpace(ip)); err == nil {
							return parsed.StringExpanded()
						}
					}
				} else {
					for i := len(ips) - 1; i >= 0; i-- {
						if parsed, err := netip.ParseAddr(strings.TrimSpace(ips[i])); err == nil {
							return parsed.StringExpanded()
						}
					}
				}
			}
		}

		return RemoteIP(r)
	}
}
