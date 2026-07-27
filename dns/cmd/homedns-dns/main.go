package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Adrien-hue/homedns-analytics/dns/internal/config"
	"github.com/Adrien-hue/homedns-analytics/dns/internal/dnsserver"
	"github.com/Adrien-hue/homedns-analytics/dns/internal/health"
	"github.com/Adrien-hue/homedns-analytics/dns/internal/metrics"
	"github.com/Adrien-hue/homedns-analytics/dns/internal/upstream"
	"github.com/Adrien-hue/homedns-analytics/dns/internal/version"
)

func main() {
	os.Exit(run())
}

func run() int {
	configPath := flag.String(
		"config",
		"",
		"path to the YAML configuration file",
	)

	showVersion := flag.Bool(
		"version",
		false,
		"print version information and exit",
	)

	flag.Parse()

	if *showVersion {
		fmt.Println(version.String())
		return 0
	}

	if flag.NArg() > 0 {
		fmt.Fprintf(
			os.Stderr,
			"unexpected arguments: %v\n",
			flag.Args(),
		)

		return 2
	}

	if *configPath == "" {
		fmt.Fprintln(
			os.Stderr,
			"configuration path is required; use --config <path>",
		)

		return 2
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"load configuration: %v\n",
			err,
		)

		return 1
	}

	logger := newLogger(cfg.Logging)

	startedAt := time.Now()

	registry := &metrics.Registry{}

	upstreamClient := upstream.NewClient(
		cfg.Upstream.Address,
		cfg.Upstream.Timeout,
	)

	dnsHandler := dnsserver.NewHandlerWithDependencies(
		upstreamClient,
		logger,
		registry,
	)

	dnsService := dnsserver.NewServer(
		cfg.DNSAddress(),
		dnsHandler,
		logger,
	)

	stateProvider := func() health.State {
		snapshot := registry.Snapshot()
		dnsReady := dnsService.Ready()

		return health.State{
			Ready: dnsReady,

			Version:   version.Version,
			Commit:    version.Commit,
			BuildTime: version.BuildTime,

			StartedAt: startedAt,

			DNS: health.DNSState{
				UDPListening:  dnsReady,
				TCPListening:  dnsReady,
				ListenAddress: cfg.DNSAddress(),
			},

			Upstream: health.UpstreamState{
				Address: cfg.Upstream.Address,
			},

			Metrics: health.MetricsState{
				RequestsTotal: snapshot.RequestsTotal,
				RequestsUDP:   snapshot.RequestsUDP,
				RequestsTCP:   snapshot.RequestsTCP,

				ResponsesNoError:  snapshot.ResponsesNoError,
				ResponsesNXDomain: snapshot.ResponsesNXDomain,
				ResponsesSERVFAIL: snapshot.ResponsesSERVFAIL,
				ResponsesOther:    snapshot.ResponsesOther,

				UpstreamTimeouts: snapshot.UpstreamTimeouts,
				UpstreamErrors:   snapshot.UpstreamErrors,
				InvalidRequests:  snapshot.InvalidRequests,

				ResponseWriteErrors: snapshot.ResponseWriteErrors,
				HandlerPanics:       snapshot.HandlerPanics,

				AverageUpstreamLatencyMilliseconds: durationMilliseconds(
					snapshot.AverageUpstreamLatency,
				),
			},
		}
	}

	healthHandler := health.NewHandler(
		stateProvider,
		logger,
	)

	healthService, err := health.NewServer(
		cfg.HealthAddress(),
		healthHandler,
		logger,
	)
	if err != nil {
		logger.Error(
			"create health server",
			"error", err,
		)

		return 1
	}

	if err := dnsService.Start(); err != nil {
		logger.Error(
			"start DNS server",
			"address", cfg.DNSAddress(),
			"error", err,
		)

		return 1
	}

	if err := healthService.Start(); err != nil {
		logger.Error(
			"start health server",
			"address", cfg.HealthAddress(),
			"error", err,
		)

		if rollbackErr := shutdownDNS(
			dnsService,
			cfg.Shutdown.Timeout,
		); rollbackErr != nil {
			logger.Error(
				"roll back DNS server after health startup failure",
				"error", rollbackErr,
			)
		}

		return 1
	}

	logger.Info(
		"HomeDNS DNS started",
		"dns_address", cfg.DNSAddress(),
		"health_address", healthService.Address(),
		"upstream_address", cfg.Upstream.Address,
		"version", version.Version,
		"commit", version.Commit,
	)

	signalContext, stopSignals := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignals()

	<-signalContext.Done()

	logger.Info("shutdown signal received")

	shutdownContext, cancelShutdown := context.WithTimeout(
		context.Background(),
		cfg.Shutdown.Timeout,
	)
	defer cancelShutdown()

	// The health listener is intentionally stopped first. Once it is closed,
	// the service no longer advertises readiness while DNS shutdown proceeds.
	healthErr := healthService.Shutdown(shutdownContext)
	dnsErr := dnsService.Shutdown(shutdownContext)

	if err := errors.Join(healthErr, dnsErr); err != nil {
		logger.Error(
			"HomeDNS DNS shutdown failed",
			"error", err,
		)

		return 1
	}

	logger.Info("HomeDNS DNS stopped")

	return 0
}

func newLogger(loggingConfig config.LoggingConfig) *slog.Logger {
	level := new(slog.LevelVar)

	switch loggingConfig.Level {
	case "debug":
		level.Set(slog.LevelDebug)
	case "warn":
		level.Set(slog.LevelWarn)
	case "error":
		level.Set(slog.LevelError)
	default:
		level.Set(slog.LevelInfo)
	}

	options := &slog.HandlerOptions{
		Level: level,
	}

	if loggingConfig.Format == "text" {
		return slog.New(
			slog.NewTextHandler(os.Stderr, options),
		)
	}

	return slog.New(
		slog.NewJSONHandler(os.Stderr, options),
	)
}

func durationMilliseconds(duration time.Duration) float64 {
	return float64(duration) / float64(time.Millisecond)
}

func shutdownDNS(
	server *dnsserver.Server,
	timeout time.Duration,
) error {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		timeout,
	)
	defer cancel()

	return server.Shutdown(ctx)
}
