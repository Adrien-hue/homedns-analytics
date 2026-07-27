package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Adrien-hue/homedns-analytics/dns/internal/config"
)

func TestNewRejectsPublicHealthAddress(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.Health.ListenAddress = "0.0.0.0"

	application, err := New(cfg)
	if err == nil {
		t.Fatal("expected application creation to fail")
	}

	if application != nil {
		t.Fatal("expected nil application after creation failure")
	}

	if !strings.Contains(
		err.Error(),
		"health server must listen on a loopback address",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestRunStartsAndStopsServices(t *testing.T) {
	t.Parallel()

	cfg := testConfig()

	application, err := New(cfg)
	if err != nil {
		t.Fatalf("create application: %v", err)
	}

	runContext, cancelRun := context.WithCancel(
		context.Background(),
	)

	runDone := make(chan error, 1)

	go func() {
		runDone <- application.Run(runContext)
	}()

	eventually(t, time.Second, func() bool {
		return application.dnsServer.Ready() &&
			application.healthServer.Ready()
	})

	cancelRun()

	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("run application: %v", err)
		}

	case <-time.After(time.Second):
		t.Fatal("application did not stop before timeout")
	}

	if application.dnsServer.Ready() {
		t.Fatal("expected DNS server to be stopped")
	}

	if application.healthServer.Ready() {
		t.Fatal("expected health server to be stopped")
	}
}

func TestHealthStateUsesRuntimeState(t *testing.T) {
	t.Parallel()

	cfg := testConfig()

	application, err := New(cfg)
	if err != nil {
		t.Fatalf("create application: %v", err)
	}

	application.metrics.RecordRequestUDP()
	application.metrics.RecordRequestTCP()
	application.metrics.RecordUpstreamTimeout()

	state := application.healthState()

	if state.Ready {
		t.Fatal("expected application not to be ready before startup")
	}

	if state.DNS.ListenAddress != cfg.DNSAddress() {
		t.Fatalf(
			"unexpected DNS address: got %q, want %q",
			state.DNS.ListenAddress,
			cfg.DNSAddress(),
		)
	}

	if state.Upstream.Address != cfg.Upstream.Address {
		t.Fatalf(
			"unexpected upstream address: got %q, want %q",
			state.Upstream.Address,
			cfg.Upstream.Address,
		)
	}

	if state.Metrics.RequestsTotal != 2 {
		t.Fatalf(
			"unexpected request count: got %d, want %d",
			state.Metrics.RequestsTotal,
			2,
		)
	}

	if state.Metrics.RequestsUDP != 1 {
		t.Fatalf(
			"unexpected UDP request count: got %d, want %d",
			state.Metrics.RequestsUDP,
			1,
		)
	}

	if state.Metrics.RequestsTCP != 1 {
		t.Fatalf(
			"unexpected TCP request count: got %d, want %d",
			state.Metrics.RequestsTCP,
			1,
		)
	}

	if state.Metrics.UpstreamTimeouts != 1 {
		t.Fatalf(
			"unexpected timeout count: got %d, want %d",
			state.Metrics.UpstreamTimeouts,
			1,
		)
	}
}

func testConfig() config.Config {
	cfg := config.Default()

	cfg.Server.ListenAddress = "127.0.0.1"
	cfg.Server.Port = 0

	cfg.Health.ListenAddress = "127.0.0.1"
	cfg.Health.Port = 0

	cfg.Upstream.Address = "1.1.1.1:53"
	cfg.Logging.Level = "error"
	cfg.Logging.Format = "text"
	cfg.Shutdown.Timeout = time.Second

	return cfg
}

func eventually(
	t *testing.T,
	timeout time.Duration,
	condition func() bool,
) {
	t.Helper()

	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if condition() {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("condition was not satisfied before timeout")
}
