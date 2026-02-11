package keratin

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRemoteIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		want       string
	}{
		{
			name:       "IPv4 address",
			remoteAddr: "192.168.1.1:12345",
			want:       "192.168.1.1",
		},
		{
			name:       "IPv6 address full expansion",
			remoteAddr: "[2001:db8::1]:8080",
			want:       "2001:0db8:0000:0000:0000:0000:0000:0001",
		},
		{
			name:       "IPv6 address partial expansion",
			remoteAddr: "[2001:db8:0:0:0:0:0:1]:8080",
			want:       "2001:0db8:0000:0000:0000:0000:0000:0001",
		},
		{
			name:       "IPv6 address multiple zeroes",
			remoteAddr: "[::1]:8080",
			want:       "0000:0000:0000:0000:0000:0000:0000:0001",
		},
		{
			name:       "IPv6 loopback",
			remoteAddr: "[::1]:1234",
			want:       "0000:0000:0000:0000:0000:0000:0000:0001",
		},
		{
			name:       "IPv6 all zeros",
			remoteAddr: "[::]:8080",
			want:       "0000:0000:0000:0000:0000:0000:0000:0000",
		},
		{
			name:       "IPv6 with multiple expansions",
			remoteAddr: "[2001:0db8::0001]:8080",
			want:       "2001:0db8:0000:0000:0000:0000:0000:0001",
		},
		{
			name:       "IPv4 localhost",
			remoteAddr: "127.0.0.1:8000",
			want:       "127.0.0.1",
		},
		{
			name:       "IPv4 private address",
			remoteAddr: "10.0.0.1:34567",
			want:       "10.0.0.1",
		},
		{
			name:       "IPv4 with high port",
			remoteAddr: "192.168.1.100:65535",
			want:       "192.168.1.100",
		},
		{
			name:       "invalid address returns invalid IP",
			remoteAddr: "invalid-address",
			want:       "invalid IP",
		},
		{
			name:       "empty remote address returns invalid IP",
			remoteAddr: "",
			want:       "invalid IP",
		},
		{
			name:       "IPv6 without port but with brackets returns invalid IP",
			remoteAddr: "[2001:db8::1]",
			want:       "invalid IP",
		},
		{
			name:       "IPv6 public address",
			remoteAddr: "[2607:f8b0:4005:805::200e]:443",
			want:       "2607:f8b0:4005:0805:0000:0000:0000:200e",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				RemoteAddr: tt.remoteAddr,
			}
			got := RemoteIP(req)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestRealIP(t *testing.T) {
	tests := []struct {
		name          string
		trustedProxy  *TrustedProxy
		getTrustedErr error
		headers       map[string]string
		remoteAddr    string
		want          string
	}{
		{
			name: "X-Forwarded-For single IP",
			trustedProxy: &TrustedProxy{
				Headers:       []string{HeaderXForwardedFor},
				UseLeftmostIP: false,
			},
			headers: map[string]string{
				HeaderXForwardedFor: "192.168.1.100",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "192.168.1.100",
		},
		{
			name: "X-Forwarded-For multiple IPs - rightmost",
			trustedProxy: &TrustedProxy{
				Headers:       []string{HeaderXForwardedFor},
				UseLeftmostIP: false,
			},
			headers: map[string]string{
				HeaderXForwardedFor: "203.0.113.1, 192.168.1.100, 10.0.0.1",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "10.0.0.1",
		},
		{
			name: "X-Forwarded-For multiple IPs - leftmost",
			trustedProxy: &TrustedProxy{
				Headers:       []string{HeaderXForwardedFor},
				UseLeftmostIP: true,
			},
			headers: map[string]string{
				HeaderXForwardedFor: "203.0.113.1, 192.168.1.100, 10.0.0.1",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "203.0.113.1",
		},
		{
			name: "X-Real-IP header",
			trustedProxy: &TrustedProxy{
				Headers:       []string{HeaderXRealIP},
				UseLeftmostIP: false,
			},
			headers: map[string]string{
				HeaderXRealIP: "192.168.1.100",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "192.168.1.100",
		},
		{
			name: "CF-Connecting-IP header",
			trustedProxy: &TrustedProxy{
				Headers:       []string{HeaderCFIPCountry},
				UseLeftmostIP: false,
			},
			headers: map[string]string{
				HeaderCFIPCountry: "192.168.1.100",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "192.168.1.100",
		},
		{
			name: "multiple headers with priority",
			trustedProxy: &TrustedProxy{
				Headers:       []string{HeaderXRealIP, HeaderXForwardedFor},
				UseLeftmostIP: false,
			},
			headers: map[string]string{
				HeaderXRealIP:       "192.168.1.100",
				HeaderXForwardedFor: "10.0.0.1",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "192.168.1.100",
		},
		{
			name: "no matching headers falls back to RemoteIP",
			trustedProxy: &TrustedProxy{
				Headers:       []string{HeaderXRealIP},
				UseLeftmostIP: false,
			},
			headers:    map[string]string{},
			remoteAddr: "192.168.1.100:12345",
			want:       "192.168.1.100",
		},
		{
			name:          "error getting trusted proxy falls back to RemoteIP",
			getTrustedErr: context.Canceled,
			headers: map[string]string{
				HeaderXForwardedFor: "192.168.1.100",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "10.0.0.1",
		},
		{
			name: "empty trusted headers falls back to RemoteIP",
			trustedProxy: &TrustedProxy{
				Headers:       []string{},
				UseLeftmostIP: false,
			},
			headers: map[string]string{
				HeaderXForwardedFor: "192.168.1.100",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "10.0.0.1",
		},
		{
			name: "empty header value is skipped",
			trustedProxy: &TrustedProxy{
				Headers:       []string{HeaderXForwardedFor},
				UseLeftmostIP: false,
			},
			headers: map[string]string{
				HeaderXForwardedFor: "",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "10.0.0.1",
		},
		{
			name: "invalid IP in header is skipped - rightmost",
			trustedProxy: &TrustedProxy{
				Headers:       []string{HeaderXForwardedFor},
				UseLeftmostIP: false,
			},
			headers: map[string]string{
				HeaderXForwardedFor: "invalid-ip, 192.168.1.100",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "192.168.1.100",
		},
		{
			name: "invalid IP in header is skipped - leftmost",
			trustedProxy: &TrustedProxy{
				Headers:       []string{HeaderXForwardedFor},
				UseLeftmostIP: true,
			},
			headers: map[string]string{
				HeaderXForwardedFor: "invalid-ip, 192.168.1.100",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "192.168.1.100",
		},
		{
			name: "IPv6 address in header",
			trustedProxy: &TrustedProxy{
				Headers:       []string{HeaderXRealIP},
				UseLeftmostIP: false,
			},
			headers: map[string]string{
				HeaderXRealIP: "2001:db8::1",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "2001:0db8:0000:0000:0000:0000:0000:0001",
		},
		{
			name: "multiple header values uses last one",
			trustedProxy: &TrustedProxy{
				Headers:       []string{HeaderXForwardedFor},
				UseLeftmostIP: false,
			},
			headers: map[string]string{
				HeaderXForwardedFor: "203.0.113.1",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "203.0.113.1",
		},
		{
			name: "whitespace in IP list is trimmed",
			trustedProxy: &TrustedProxy{
				Headers:       []string{HeaderXForwardedFor},
				UseLeftmostIP: false,
			},
			headers: map[string]string{
				HeaderXForwardedFor: "  192.168.1.100  ,  10.0.0.1  ",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "10.0.0.1",
		},
		{
			name: "only invalid IPs falls back to RemoteIP",
			trustedProxy: &TrustedProxy{
				Headers:       []string{HeaderXForwardedFor},
				UseLeftmostIP: false,
			},
			headers: map[string]string{
				HeaderXForwardedFor: "invalid1, invalid2, invalid3",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "10.0.0.1",
		},
		{
			name: "mixed valid and invalid IPs - rightmost",
			trustedProxy: &TrustedProxy{
				Headers:       []string{HeaderXForwardedFor},
				UseLeftmostIP: false,
			},
			headers: map[string]string{
				HeaderXForwardedFor: "invalid, 192.168.1.100, invalid2",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "192.168.1.100",
		},
		{
			name: "mixed valid and invalid IPs - leftmost",
			trustedProxy: &TrustedProxy{
				Headers:       []string{HeaderXForwardedFor},
				UseLeftmostIP: true,
			},
			headers: map[string]string{
				HeaderXForwardedFor: "invalid, 192.168.1.100, 10.0.0.1",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "192.168.1.100",
		},
		{
			name: "nil trusted proxy falls back to RemoteIP",
			trustedProxy: &TrustedProxy{
				Headers:       nil,
				UseLeftmostIP: false,
			},
			headers: map[string]string{
				HeaderXForwardedFor: "192.168.1.100",
			},
			remoteAddr: "10.0.0.1:12345",
			want:       "10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getTrusted := func(ctx context.Context) (*TrustedProxy, error) {
				if tt.getTrustedErr != nil {
					return nil, tt.getTrustedErr
				}
				return tt.trustedProxy, nil
			}

			req := &http.Request{
				RemoteAddr: tt.remoteAddr,
				Header:     make(http.Header),
			}

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			extractor := RealIP(getTrusted)
			got := extractor(req)

			require.Equal(t, tt.want, got)
		})
	}
}

func TestRealIP_VerifyExpanding(t *testing.T) {
	tests := []struct {
		name         string
		trustedProxy *TrustedProxy
		headers      map[string]string
		remoteAddr   string
		wantExpanded bool
	}{
		{
			name: "IPv6 from header is expanded",
			trustedProxy: &TrustedProxy{
				Headers:       []string{HeaderXRealIP},
				UseLeftmostIP: false,
			},
			headers: map[string]string{
				HeaderXRealIP: "2001:db8::1",
			},
			remoteAddr:   "10.0.0.1:12345",
			wantExpanded: true,
		},
		{
			name: "IPv4 from header is not expanded",
			trustedProxy: &TrustedProxy{
				Headers:       []string{HeaderXRealIP},
				UseLeftmostIP: false,
			},
			headers: map[string]string{
				HeaderXRealIP: "192.168.1.100",
			},
			remoteAddr:   "10.0.0.1:12345",
			wantExpanded: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getTrusted := func(ctx context.Context) (*TrustedProxy, error) {
				return tt.trustedProxy, nil
			}

			req := &http.Request{
				RemoteAddr: tt.remoteAddr,
				Header:     make(http.Header),
			}

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			extractor := RealIP(getTrusted)
			got := extractor(req)

			if tt.wantExpanded {
				parsed, err := netip.ParseAddr(got)
				require.NoError(t, err)
				require.True(t, parsed.Is6())
			}
		})
	}
}

func TestRemoteIP_WithPort(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		wantIP     string
		wantPort   string
	}{
		{
			name:       "IPv4 with port",
			remoteAddr: "192.168.1.1:8080",
			wantIP:     "192.168.1.1",
			wantPort:   "8080",
		},
		{
			name:       "IPv6 with port",
			remoteAddr: "[2001:db8::1]:443",
			wantIP:     "2001:0db8:0000:0000:0000:0000:0000:0001",
			wantPort:   "443",
		},
		{
			name:       "IPv4 without port returns invalid IP",
			remoteAddr: "192.168.1.1",
			wantIP:     "invalid IP",
			wantPort:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, port, err := net.SplitHostPort(tt.remoteAddr)
			if tt.wantPort != "" {
				require.NoError(t, err)
				require.Equal(t, tt.wantPort, port)
			} else {
				require.Error(t, err)
			}

			req := &http.Request{
				RemoteAddr: tt.remoteAddr,
			}
			got := RemoteIP(req)
			require.Equal(t, tt.wantIP, got)
		})
	}
}
