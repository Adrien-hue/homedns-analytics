package health

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestNewServerRejectsEmptyAddress(t *testing.T) {
	_, err := NewServer(
		"",
		http.HandlerFunc(func(
			http.ResponseWriter,
			*http.Request,
		) {
		}),
		nil,
	)

	if err == nil {
		t.Fatal("expected empty address to be rejected")
	}
}

func TestNewServerRejectsNilHandler(t *testing.T) {
	_, err := NewServer(
		"127.0.0.1:0",
		nil,
		nil,
	)

	if err == nil {
		t.Fatal("expected nil handler to be rejected")
	}
}

func TestNewServerRejectsPublicAddress(t *testing.T) {
	testCases := []struct {
		name    string
		address string
	}{
		{
			name:    "all IPv4 interfaces",
			address: "0.0.0.0:8080",
		},
		{
			name:    "LAN IPv4 address",
			address: "192.168.1.10:8080",
		},
		{
			name:    "all IPv6 interfaces",
			address: "[::]:8080",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := NewServer(
				testCase.address,
				http.HandlerFunc(func(
					http.ResponseWriter,
					*http.Request,
				) {
				}),
				nil,
			)

			if !errors.Is(err, ErrPublicAddress) {
				t.Fatalf(
					"unexpected error: got %v, want %v",
					err,
					ErrPublicAddress,
				)
			}
		})
	}
}

func TestNewServerAcceptsLoopbackAddresses(t *testing.T) {
	testCases := []string{
		"127.0.0.1:0",
		"localhost:0",
		"[::1]:0",
	}

	for _, address := range testCases {
		t.Run(address, func(t *testing.T) {
			server, err := NewServer(
				address,
				http.HandlerFunc(func(
					http.ResponseWriter,
					*http.Request,
				) {
				}),
				nil,
			)
			if err != nil {
				t.Fatalf(
					"create server for %q: %v",
					address,
					err,
				)
			}

			if server == nil {
				t.Fatal("expected server")
			}
		})
	}
}

func TestServerServesHealthEndpointOverHTTP(t *testing.T) {
	startedAt := time.Now().Add(-time.Minute)

	handler := NewHandler(
		func() State {
			return State{
				Ready:     true,
				Version:   "v0.2.0",
				Commit:    "integration-test",
				BuildTime: "2026-07-26T08:00:00Z",
				StartedAt: startedAt,

				DNS: DNSState{
					UDPListening:  true,
					TCPListening:  true,
					ListenAddress: "127.0.0.1:5353",
				},

				Upstream: UpstreamState{
					Address: "127.0.0.1:5300",
				},

				Metrics: MetricsState{
					RequestsTotal: 12,
					RequestsUDP:   10,
					RequestsTCP:   2,
				},
			}
		},
		nil,
	)

	server, err := NewServer(
		"127.0.0.1:0",
		handler,
		nil,
	)
	if err != nil {
		t.Fatalf("create health server: %v", err)
	}

	if err := server.Start(); err != nil {
		t.Fatalf("start health server: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(
			context.Background(),
			time.Second,
		)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			t.Errorf("shutdown health server: %v", err)
		}
	})

	if !server.Ready() {
		t.Fatal("expected health server to be ready")
	}

	response, err := http.Get(
		"http://" + server.Address() + Route,
	)
	if err != nil {
		t.Fatalf("request health endpoint: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)

		t.Fatalf(
			"unexpected HTTP status: got %d, body %q",
			response.StatusCode,
			body,
		)
	}

	var healthResponse Response

	if err := json.NewDecoder(response.Body).Decode(
		&healthResponse,
	); err != nil {
		t.Fatalf("decode health response: %v", err)
	}

	if healthResponse.Status != "ok" {
		t.Fatalf(
			"unexpected health status: got %q, want %q",
			healthResponse.Status,
			"ok",
		)
	}

	if !healthResponse.Ready {
		t.Fatal("expected DNS service to be reported ready")
	}

	if healthResponse.Version != "v0.2.0" {
		t.Fatalf(
			"unexpected version: got %q",
			healthResponse.Version,
		)
	}

	if healthResponse.Metrics.RequestsTotal != 12 {
		t.Fatalf(
			"unexpected request total: got %d, want 12",
			healthResponse.Metrics.RequestsTotal,
		)
	}
}

func TestServerRejectsSecondStartWhileRunning(t *testing.T) {
	server := newTestHealthServer(t)

	if err := server.Start(); err != nil {
		t.Fatalf("start health server: %v", err)
	}

	t.Cleanup(func() {
		shutdownTestHealthServer(t, server)
	})

	err := server.Start()

	if !errors.Is(err, ErrServerRunning) {
		t.Fatalf(
			"unexpected error: got %v, want %v",
			err,
			ErrServerRunning,
		)
	}
}

func TestServerShutdownIsIdempotent(t *testing.T) {
	server := newTestHealthServer(t)

	if err := server.Start(); err != nil {
		t.Fatalf("start health server: %v", err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Second,
	)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("first shutdown: %v", err)
	}

	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("second shutdown: %v", err)
	}

	if server.Ready() {
		t.Fatal("expected server not to be ready after shutdown")
	}
}

func TestServerCanRestartAfterShutdown(t *testing.T) {
	server := newTestHealthServer(t)

	if err := server.Start(); err != nil {
		t.Fatalf("first start: %v", err)
	}

	firstAddress := server.Address()

	shutdownTestHealthServer(t, server)

	if server.Ready() {
		t.Fatal("expected server not to be ready after shutdown")
	}

	if err := server.Start(); err != nil {
		t.Fatalf("second start: %v", err)
	}

	t.Cleanup(func() {
		shutdownTestHealthServer(t, server)
	})

	if !server.Ready() {
		t.Fatal("expected restarted server to be ready")
	}

	secondAddress := server.Address()

	if firstAddress == "" {
		t.Fatal("expected first bound address")
	}

	if secondAddress == "" {
		t.Fatal("expected second bound address")
	}

	response, err := http.Get(
		"http://" + secondAddress + Route,
	)
	if err != nil {
		t.Fatalf(
			"request restarted health server: %v",
			err,
		)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf(
			"unexpected status after restart: got %d",
			response.StatusCode,
		)
	}
}

func TestServerReturnsConfiguredAddressBeforeStart(t *testing.T) {
	const address = "127.0.0.1:0"

	server, err := NewServer(
		address,
		http.HandlerFunc(func(
			http.ResponseWriter,
			*http.Request,
		) {
		}),
		nil,
	)
	if err != nil {
		t.Fatalf("create health server: %v", err)
	}

	if actual := server.Address(); actual != address {
		t.Fatalf(
			"unexpected address: got %q, want %q",
			actual,
			address,
		)
	}
}

func TestServerReleasesPortAfterShutdown(t *testing.T) {
	server := newTestHealthServer(t)

	if err := server.Start(); err != nil {
		t.Fatalf("start health server: %v", err)
	}

	address := server.Address()

	shutdownTestHealthServer(t, server)

	deadline := time.Now().Add(time.Second)

	for {
		listener, err := net.Listen("tcp", address)
		if err == nil {
			if err := listener.Close(); err != nil {
				t.Fatalf(
					"close replacement listener: %v",
					err,
				)
			}

			return
		}

		if time.Now().After(deadline) {
			t.Fatalf(
				"health address was not released before deadline: %v",
				err,
			)
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func newTestHealthServer(t *testing.T) *Server {
	t.Helper()

	server, err := NewServer(
		"127.0.0.1:0",
		http.HandlerFunc(func(
			writer http.ResponseWriter,
			_ *http.Request,
		) {
			writer.WriteHeader(http.StatusOK)
		}),
		nil,
	)
	if err != nil {
		t.Fatalf("create test health server: %v", err)
	}

	return server
}

func shutdownTestHealthServer(
	t *testing.T,
	server *Server,
) {
	t.Helper()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Second,
	)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown test health server: %v", err)
	}
}
