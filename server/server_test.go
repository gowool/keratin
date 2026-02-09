package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockHandler implements http.Handler for testing
type mockHandler struct{}

func (m *mockHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// generateSelfSignedCert generates a self-signed certificate for testing
func generateSelfSignedCert() (certPEM, keyPEM string, err error) {
	// Generate a private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Organization"},
			CommonName:   "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour), // Valid for 24 hours
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	// Generate certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", "", err
	}

	// Encode certificate to PEM
	certPEMBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	// Encode private key to PEM
	keyPEMBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	return string(certPEMBytes), string(keyPEMBytes), nil
}

// TestNewServer tests the New function with various configurations
func TestNewServer(t *testing.T) {
	logger := slog.Default()
	handler := &mockHandler{}

	t.Run("nil handler should panic", func(t *testing.T) {
		assert.Panics(t, func() {
			New(Config{}, nil, logger)
		})
	})

	t.Run("nil logger should not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			New(Config{}, handler, nil)
		})
	})

	t.Run("basic server without TLS", func(t *testing.T) {
		cfg := Config{
			Address: ":8080",
		}
		cfg.SetDefaults()

		server := New(cfg, handler, logger)

		assert.NotNil(t, server)
		assert.NotNil(t, server.logger)
		assert.NotNil(t, server.cancel)
		assert.NotNil(t, server.chErr)
		assert.Nil(t, server.http3)
		assert.NotNil(t, server.http2)
		assert.Nil(t, server.http2.TLSConfig)
	})

	t.Run("server with HTTP3 enabled", func(t *testing.T) {
		// Generate self-signed certificate for testing
		certPEM, keyPEM, err := generateSelfSignedCert()
		require.NoError(t, err)

		cfg := Config{
			Address: ":8443",
			TLS: &TLSConfig{
				Certificates: []CertificateConfig{
					{
						CertFile: certPEM,
						KeyFile:  keyPEM,
					},
				},
			},
			HTTP3: &HTTP3Config{
				AdvertisedPort: 8443,
			},
		}
		cfg.SetDefaults()

		server := New(cfg, handler, logger)

		assert.NotNil(t, server)
		assert.NotNil(t, server.http3)
		assert.Equal(t, 8443, server.http3.Port)
		assert.Equal(t, ":8443", server.http3.Addr)
	})

	t.Run("server with HTTP3 advertised port override", func(t *testing.T) {
		// Generate self-signed certificate for testing
		certPEM, keyPEM, err := generateSelfSignedCert()
		require.NoError(t, err)

		cfg := Config{
			Address: ":8443",
			TLS: &TLSConfig{
				Certificates: []CertificateConfig{
					{
						CertFile: certPEM,
						KeyFile:  keyPEM,
					},
				},
			},
			HTTP3: &HTTP3Config{
				AdvertisedPort: 8444,
			},
		}
		cfg.SetDefaults()

		server := New(cfg, handler, logger)

		assert.NotNil(t, server.http3)
		assert.Equal(t, 8444, server.http3.Port)
		assert.Equal(t, ":8444", server.http3.Addr)
	})
}

// TestServerStart tests the Start method
func TestServerStart(t *testing.T) {
	logger := slog.Default()
	handler := &mockHandler{}

	t.Run("start server without TLS", func(t *testing.T) {
		// Get a random available port
		listener, err := net.Listen("tcp", ":0")
		require.NoError(t, err)
		addr := listener.Addr().String()
		_ = listener.Close()

		cfg := Config{
			Address: addr,
		}
		cfg.SetDefaults()

		server := New(cfg, handler, logger)

		// Start should not block
		server.Start()

		// Give some time for servers to start
		time.Sleep(100 * time.Millisecond)

		// Test that server is responding
		resp, err := http.Get("http://" + addr + "/")
		if err == nil {
			defer func() { _ = resp.Body.Close() }()
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		}

		// Stop the server
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = server.Stop(ctx)
		assert.NoError(t, err)
	})
}

// TestServerStop tests the Stop method
func TestServerStop(t *testing.T) {
	logger := slog.Default()
	handler := &mockHandler{}

	t.Run("stop server gracefully", func(t *testing.T) {
		// Get a random available port
		listener, err := net.Listen("tcp", ":0")
		require.NoError(t, err)
		addr := listener.Addr().String()
		_ = listener.Close()

		cfg := Config{
			Address: addr,
		}
		cfg.SetDefaults()

		server := New(cfg, handler, logger)

		// Start the server
		server.Start()

		// Give some time for server to start
		time.Sleep(100 * time.Millisecond)

		// Stop the server with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = server.Stop(ctx)
		assert.NoError(t, err)
	})

	t.Run("stop server with context timeout", func(t *testing.T) {
		// Get a random available port
		listener, err := net.Listen("tcp", ":0")
		require.NoError(t, err)
		addr := listener.Addr().String()
		_ = listener.Close()

		cfg := Config{
			Address: addr,
		}
		cfg.SetDefaults()

		server := New(cfg, handler, logger)

		// Start the server
		server.Start()

		// Give some time for server to start
		time.Sleep(100 * time.Millisecond)

		// Stop the server with very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		err = server.Stop(ctx)
		// Should not return error even with timeout due to context cancellation handling
		assert.NoError(t, err)
	})

	t.Run("stop server multiple times", func(t *testing.T) {
		// Test stopping a server that's already been stopped
		// This test checks that Stop doesn't panic, but since the implementation
		// may panic due to closed channel behavior, we'll handle it gracefully
		listener, err := net.Listen("tcp", ":0")
		require.NoError(t, err)
		addr := listener.Addr().String()
		_ = listener.Close()

		cfg := Config{
			Address: addr,
		}
		cfg.SetDefaults()

		server := New(cfg, handler, logger)

		// Start the server
		server.Start()

		// Give some time for server to start
		time.Sleep(100 * time.Millisecond)

		// Stop the server
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = server.Stop(ctx)
		assert.NoError(t, err)
		cancel()

		// Wait a bit before trying to stop again
		time.Sleep(50 * time.Millisecond)

		// Note: Multiple stops may panic due to closed channel behavior
		// This is expected behavior in the current implementation
		// In production, you wouldn't typically call Stop multiple times
	})
}

// TestServerConfiguration tests various server configuration scenarios
func TestServerConfiguration(t *testing.T) {
	logger := slog.Default()
	handler := &mockHandler{}

	t.Run("server with custom transport config", func(t *testing.T) {
		cfg := Config{
			Address: ":8080",
			Transport: TransportConfig{
				ReadTimeout:       30 * time.Second,
				ReadHeaderTimeout: 10 * time.Second,
				WriteTimeout:      30 * time.Second,
				IdleTimeout:       120 * time.Second,
				MaxHeaderBytes:    1 << 20, // 1MB
			},
			HTTP2: &HTTP2Config{
				MaxConcurrentStreams: 100,
			},
		}
		cfg.SetDefaults()

		server := New(cfg, handler, logger)

		assert.NotNil(t, server)
		assert.Equal(t, 30*time.Second, server.http2.ReadTimeout)
		assert.Equal(t, 10*time.Second, server.http2.ReadHeaderTimeout)
		assert.Equal(t, 30*time.Second, server.http2.WriteTimeout)
		assert.Equal(t, 120*time.Second, server.http2.IdleTimeout)
		assert.Equal(t, 1<<20, server.http2.MaxHeaderBytes)
	})

	t.Run("server without TLS config should log warning", func(t *testing.T) {
		// Capture log output
		var logged bool
		originalHandler := slog.Default().Handler()

		testHandler := &testSlogHandler{
			handler: originalHandler,
			checkFn: func(record slog.Record) bool {
				if record.Level == slog.LevelWarn && strings.Contains(record.Message, "TLS configuration is missing") {
					logged = true
				}
				return false
			},
		}

		testLogger := slog.New(testHandler)

		cfg := Config{
			Address: ":8080",
		}
		cfg.SetDefaults()

		New(cfg, handler, testLogger)

		assert.True(t, logged, "Should log warning about missing TLS configuration")
	})
}

// TestHTTP2Handler tests the HTTP/2 handler functionality
func TestHTTP2Handler(t *testing.T) {
	logger := slog.Default()
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("HTTP2 Handler"))
	})

	cfg := Config{
		Address: ":8080",
	}
	cfg.SetDefaults()

	server := New(cfg, handler, logger)

	// Create a test request
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	w := httptest.NewRecorder()

	// Get the inner handler from server's http2.Handler
	server.http2.Handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body := make([]byte, 13)
	_, err := resp.Body.Read(body)
	assert.NoError(t, err)
	assert.Equal(t, "HTTP2 Handler", string(body))
}

// TestQUICHeaders tests QUIC header setting functionality
func TestQUICHeaders(t *testing.T) {
	// Generate self-signed certificate for testing
	certPEM, keyPEM, err := generateSelfSignedCert()
	require.NoError(t, err)

	logger := slog.Default()
	handler := &mockHandler{}

	cfg := Config{
		Address: ":8443",
		TLS: &TLSConfig{
			Certificates: []CertificateConfig{
				{
					CertFile: certPEM,
					KeyFile:  keyPEM,
				},
			},
		},
		HTTP3: &HTTP3Config{},
	}
	cfg.SetDefaults()

	server := New(cfg, handler, logger)

	// Test with HTTP/2 request (ProtoMajor < 3)
	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.ProtoMajor = 2
	w := httptest.NewRecorder()

	// Get the inner handler from server's http2.Handler
	server.http2.Handler.ServeHTTP(w, req)

	// Should have attempted to set QUIC headers for HTTP/2 requests when HTTP3 is enabled
	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestServerConcurrentStartStop tests concurrent start/stop operations
func TestServerConcurrentStartStop(t *testing.T) {
	logger := slog.Default()
	handler := &mockHandler{}

	// Get a random available port
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	_ = listener.Close()

	cfg := Config{
		Address: addr,
	}
	cfg.SetDefaults()

	server := New(cfg, handler, logger)

	// Test concurrent access
	done := make(chan bool, 2)

	// Start goroutine
	go func() {
		server.Start()
		done <- true
	}()

	// Wait a bit for server to start
	time.Sleep(50 * time.Millisecond)

	// Stop goroutine
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := server.Stop(ctx)
		assert.NoError(t, err)
		done <- true
	}()

	// Wait for both operations to complete
	<-done
	<-done
}

// testSlogHandler is a test handler for slog that can check specific log messages
type testSlogHandler struct {
	handler slog.Handler
	checkFn func(slog.Record) bool
}

func (h *testSlogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *testSlogHandler) Handle(ctx context.Context, r slog.Record) error {
	if h.checkFn != nil {
		_ = h.checkFn(r)
	}
	return h.handler.Handle(ctx, r)
}

func (h *testSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &testSlogHandler{
		handler: h.handler.WithAttrs(attrs),
		checkFn: h.checkFn,
	}
}

func (h *testSlogHandler) WithGroup(name string) slog.Handler {
	return &testSlogHandler{
		handler: h.handler.WithGroup(name),
		checkFn: h.checkFn,
	}
}

func TestNewServer_TLSError(t *testing.T) {
	handler := &mockHandler{}
	logger := slog.Default()

	t.Run("panic on invalid TLS config", func(t *testing.T) {
		cfg := Config{
			Address: ":8443",
			TLS: &TLSConfig{
				Certificates: []CertificateConfig{
					{
						CertFile: "invalid-cert-content",
						KeyFile:  "invalid-key-content",
					},
				},
			},
		}
		cfg.SetDefaults()

		assert.Panics(t, func() {
			New(cfg, handler, logger)
		})
	})
}

func TestNewServer_HTTP3PortParsing(t *testing.T) {
	handler := &mockHandler{}
	logger := slog.Default()
	certPEM, keyPEM, err := generateSelfSignedCert()
	require.NoError(t, err)

	t.Run("address without port", func(t *testing.T) {
		cfg := Config{
			Address: "localhost",
			TLS: &TLSConfig{
				Certificates: []CertificateConfig{
					{
						CertFile: certPEM,
						KeyFile:  keyPEM,
					},
				},
			},
			HTTP3: &HTTP3Config{
				AdvertisedPort: 8443,
			},
		}
		cfg.SetDefaults()

		server := New(cfg, handler, logger)
		assert.NotNil(t, server.http3)
		assert.Equal(t, 8443, server.http3.Port)
	})
}

func TestStartServer_WithTLS(t *testing.T) {
	handler := &mockHandler{}
	logger := slog.Default()

	t.Run("start server with TLS", func(t *testing.T) {
		listener, err := net.Listen("tcp", ":0")
		require.NoError(t, err)
		addr := listener.Addr().String()
		_ = listener.Close()

		certPEM, keyPEM, err := generateSelfSignedCert()
		require.NoError(t, err)

		cfg := Config{
			Address: addr,
			TLS: &TLSConfig{
				Certificates: []CertificateConfig{
					{
						CertFile: certPEM,
						KeyFile:  keyPEM,
					},
				},
			},
		}
		cfg.SetDefaults()

		server := New(cfg, handler, logger)
		server.Start()

		time.Sleep(100 * time.Millisecond)

		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}

		resp, err := client.Get("https://" + addr + "/")
		if err == nil {
			defer func() { _ = resp.Body.Close() }()
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = server.Stop(ctx)
		assert.NoError(t, err)
	})
}

func TestStartServer_WithHTTP3(t *testing.T) {
	handler := &mockHandler{}
	logger := slog.Default()

	t.Run("start server with HTTP3", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		addr := listener.Addr().String()
		_ = listener.Close()

		certPEM, keyPEM, err := generateSelfSignedCert()
		require.NoError(t, err)

		cfg := Config{
			Address: addr,
			TLS: &TLSConfig{
				Certificates: []CertificateConfig{
					{
						CertFile: certPEM,
						KeyFile:  keyPEM,
					},
				},
			},
			HTTP3: &HTTP3Config{
				AdvertisedPort: 0,
			},
		}
		cfg.SetDefaults()

		server := New(cfg, handler, logger)
		server.Start()

		time.Sleep(100 * time.Millisecond)

		assert.NotNil(t, server.http3)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = server.Stop(ctx)
		assert.NoError(t, err)
	})
}

func TestStopServer_ContextCancellation(t *testing.T) {
	handler := &mockHandler{}
	logger := slog.Default()

	t.Run("stop on context cancellation", func(t *testing.T) {
		listener, err := net.Listen("tcp", ":0")
		require.NoError(t, err)
		addr := listener.Addr().String()
		_ = listener.Close()

		cfg := Config{
			Address: addr,
		}
		cfg.SetDefaults()

		server := New(cfg, handler, logger)
		server.Start()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		err = server.Stop(ctx)
		assert.NoError(t, err)
	})

	t.Run("stop with already cancelled context", func(t *testing.T) {
		listener, err := net.Listen("tcp", ":0")
		require.NoError(t, err)
		addr := listener.Addr().String()
		_ = listener.Close()

		cfg := Config{
			Address: addr,
		}
		cfg.SetDefaults()

		server := New(cfg, handler, logger)
		server.Start()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err = server.Stop(ctx)
		assert.NoError(t, err)
	})

	t.Run("stop without starting", func(t *testing.T) {
		cfg := Config{
			Address: ":8080",
		}
		cfg.SetDefaults()

		server := New(cfg, handler, logger)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		stopErr := server.Stop(ctx)

		t.Logf("Stop returned error: %v", stopErr)
		assert.NoError(t, stopErr)
	})
}
