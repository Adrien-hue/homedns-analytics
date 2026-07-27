package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Adrien-hue/homedns-analytics/dns/internal/config"
	"github.com/Adrien-hue/homedns-analytics/dns/internal/dnsserver"
	"github.com/Adrien-hue/homedns-analytics/dns/internal/health"
	"github.com/Adrien-hue/homedns-analytics/dns/internal/metrics"
	"github.com/Adrien-hue/homedns-analytics/dns/internal/upstream"
	"github.com/Adrien-hue/homedns-analytics/dns/internal/version"
)

// App composes and manages the HomeDNS DNS service.
type App struct {
	config config.Config
	logger *slog.Logger

	startedAt time.Time
	metrics   *metrics.Registry

	dnsServer    *dnsserver.Server
	healthServer *health.Server
}

// New creates the HomeDNS application and its runtime dependencies.
func New(cfg config.Config) (*App, error) {
	logger := newLogger(cfg.Logging)
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

	application := &App{
		config:    cfg,
		logger:    logger,
		startedAt: time.Now(),
		metrics:   registry,
		dnsServer: dnsService,
	}

	healthHandler := health.NewHandler(
		application.healthState,
		logger,
	)

	healthService, err := health.NewServer(
		cfg.HealthAddress(),
		healthHandler,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("create health server: %w", err)
	}

	application.healthServer = healthService

	return application, nil
}

// Run starts the application, waits for cancellation, and gracefully shuts
// down all running services.
func (a *App) Run(ctx context.Context) error {
	if a == nil {
		return errors.New("application is nil")
	}

	if ctx == nil {
		return errors.New("application context is required")
	}

	if err := a.dnsServer.Start(); err != nil {
		a.logger.Error(
			"start DNS server",
			"address", a.config.DNSAddress(),
			"error", err,
		)

		return fmt.Errorf("start DNS server: %w", err)
	}

	if err := a.healthServer.Start(); err != nil {
		a.logger.Error(
			"start health server",
			"address", a.config.HealthAddress(),
			"error", err,
		)

		if rollbackErr := a.shutdownDNS(); rollbackErr != nil {
			a.logger.Error(
				"roll back DNS server after health startup failure",
				"error", rollbackErr,
			)

			return errors.Join(
				fmt.Errorf("start health server: %w", err),
				fmt.Errorf(
					"roll back DNS server: %w",
					rollbackErr,
				),
			)
		}

		return fmt.Errorf("start health server: %w", err)
	}

	a.logger.Info(
		"HomeDNS DNS started",
		"dns_address", a.config.DNSAddress(),
		"health_address", a.healthServer.Address(),
		"upstream_address", a.config.Upstream.Address,
		"version", version.Version,
		"commit", version.Commit,
	)

	<-ctx.Done()

	a.logger.Info("shutdown signal received")

	if err := a.shutdown(); err != nil {
		a.logger.Error(
			"HomeDNS DNS shutdown failed",
			"error", err,
		)

		return err
	}

	a.logger.Info("HomeDNS DNS stopped")

	return nil
}

func (a *App) shutdown() error {
	shutdownContext, cancelShutdown := context.WithTimeout(
		context.Background(),
		a.config.Shutdown.Timeout,
	)
	defer cancelShutdown()

	// Stop health first so the process stops advertising readiness before the
	// DNS listeners begin shutting down.
	healthErr := a.healthServer.Shutdown(shutdownContext)
	dnsErr := a.dnsServer.Shutdown(shutdownContext)

	return errors.Join(healthErr, dnsErr)
}

func (a *App) shutdownDNS() error {
	shutdownContext, cancelShutdown := context.WithTimeout(
		context.Background(),
		a.config.Shutdown.Timeout,
	)
	defer cancelShutdown()

	return a.dnsServer.Shutdown(shutdownContext)
}

func (a *App) healthState() health.State {
	snapshot := a.metrics.Snapshot()
	dnsReady := a.dnsServer.Ready()

	return health.State{
		Ready: dnsReady,

		Version:   version.Version,
		Commit:    version.Commit,
		BuildTime: version.BuildTime,

		StartedAt: a.startedAt,

		DNS: health.DNSState{
			UDPListening:  dnsReady,
			TCPListening:  dnsReady,
			ListenAddress: a.config.DNSAddress(),
		},

		Upstream: health.UpstreamState{
			Address: a.config.Upstream.Address,
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
