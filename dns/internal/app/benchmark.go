package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/Adrien-hue/homedns-analytics/dns/internal/benchmark"
	"github.com/Adrien-hue/homedns-analytics/dns/internal/config"
	"github.com/Adrien-hue/homedns-analytics/dns/internal/version"
)

const defaultBenchmarkOutputDirectory = "benchmarks/runs"

// RunBenchmark executes the default HomeDNS benchmark suite and writes its
// machine-readable JSON report.
func RunBenchmark(
	ctx context.Context,
	cfg config.Config,
	outputDirectory string,
) (benchmark.ServiceResult, error) {
	if ctx == nil {
		return benchmark.ServiceResult{},
			errors.New("benchmark context is required")
	}

	if outputDirectory == "" {
		outputDirectory = defaultBenchmarkOutputDirectory
	}

	suite := benchmark.DefaultSuiteConfig(
		cfg.Upstream.Address,
		cfg.DNSAddress(),
	)

	suite.Timeout = cfg.Upstream.Timeout

	hostname, err := os.Hostname()
	if err != nil {
		return benchmark.ServiceResult{},
			fmt.Errorf("read benchmark hostname: %w", err)
	}

	service := benchmark.NewService()

	result, err := service.Run(
		ctx,
		benchmark.ServiceConfig{
			OutputDirectory: outputDirectory,
			Suite:           suite,
			Project: benchmark.ProjectMetadata{
				Name:      "homedns-analytics",
				Version:   version.Version,
				GitCommit: version.Commit,
			},
			Environment: benchmark.Environment{
				Hostname:        hostname,
				OperatingSystem: runtime.GOOS,
				Architecture:    runtime.GOARCH,
				LogicalCPUs:     runtime.NumCPU(),
				GoVersion:       runtime.Version(),
			},
			HealthAddress: cfg.HealthAddress(),
		},
	)
	if err != nil {
		return benchmark.ServiceResult{},
			fmt.Errorf("run DNS benchmark: %w", err)
	}

	return result, nil
}
