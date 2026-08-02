package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/miekg/dns"

	"github.com/Adrien-hue/homedns-analytics/dns/internal/benchmark"
	"github.com/Adrien-hue/homedns-analytics/dns/internal/config"
	"github.com/Adrien-hue/homedns-analytics/dns/internal/version"
)

const (
	DefaultBenchmarkOutputDirectory = "benchmarks/runs"

	BenchmarkProfileQuick = benchmark.ProfileQuick

	BenchmarkProfileValidation = benchmark.ProfileValidation

	BenchmarkProfileEndurance = benchmark.ProfileEndurance

	DefaultBenchmarkProfile = benchmark.DefaultProfile
)

// BenchmarkOptions contains workload settings that can be overridden through
// the benchmark CLI.
type BenchmarkOptions struct {
	Profile           string
	ProfileCustomized bool

	ConfigPath      string
	OutputDirectory string

	QueryCount        int
	WarmupQueries     int
	ConcurrentWorkers int

	// A zero timeout means the value should be inherited from the DNS service
	// configuration.
	Timeout time.Duration

	ResourceSamplingInterval time.Duration

	QueryNames []string
	QueryType  uint16

	ProgressReporter benchmark.ProgressReporter
}

type BenchmarkProfile = benchmark.Profile

// DefaultBenchmarkOptions returns the standard quick benchmark workload.
func DefaultBenchmarkOptions() BenchmarkOptions {
	profile, err := ResolveBenchmarkProfile(
		DefaultBenchmarkProfile,
	)
	if err != nil {
		panic(
			fmt.Sprintf(
				"resolve default benchmark profile: %v",
				err,
			),
		)
	}

	return BenchmarkOptions{
		Profile: profile.Name,

		OutputDirectory: DefaultBenchmarkOutputDirectory,

		QueryCount: profile.QueryCount,

		WarmupQueries: profile.WarmupQueries,

		ConcurrentWorkers: profile.ConcurrentWorkers,

		ResourceSamplingInterval: benchmark.DefaultResourceSamplingInterval,

		QueryNames: []string{
			"example.com.",
			"cloudflare.com.",
			"google.com.",
			"github.com.",
		},

		QueryType: dns.TypeA,
	}
}

// ResolveBenchmarkProfile delegates profile resolution to the benchmark
// domain package.
func ResolveBenchmarkProfile(
	name string,
) (BenchmarkProfile, error) {
	return benchmark.ResolveProfile(name)
}

// Resolve fills values that depend on the DNS service configuration.
func (o BenchmarkOptions) Resolve(
	cfg config.Config,
) BenchmarkOptions {
	if o.Timeout == 0 {
		o.Timeout = cfg.Upstream.Timeout
	}

	if o.ResourceSamplingInterval == 0 {
		o.ResourceSamplingInterval =
			benchmark.DefaultResourceSamplingInterval
	}

	return o
}

// Validate verifies that the benchmark workload can be executed.
func (o BenchmarkOptions) Validate() error {
	if err := benchmark.ValidateProfile(
		o.Profile,
	); err != nil {
		return err
	}

	if strings.TrimSpace(o.ConfigPath) == "" {
		return errors.New(
			"benchmark configuration path is required",
		)
	}

	if o.OutputDirectory == "" {
		return errors.New(
			"benchmark output directory is required",
		)
	}

	if o.QueryCount <= 0 {
		return errors.New(
			"benchmark query count must be greater than zero",
		)
	}

	if o.WarmupQueries < 0 {
		return errors.New(
			"benchmark warmup query count cannot be negative",
		)
	}

	if o.ConcurrentWorkers <= 0 {
		return errors.New(
			"benchmark concurrency must be greater than zero",
		)
	}

	if o.ConcurrentWorkers > o.QueryCount {
		return fmt.Errorf(
			"benchmark concurrency %d cannot exceed query count %d",
			o.ConcurrentWorkers,
			o.QueryCount,
		)
	}

	if o.Timeout <= 0 {
		return errors.New(
			"benchmark timeout must be greater than zero",
		)
	}

	if o.ResourceSamplingInterval <= 0 {
		return errors.New(
			"benchmark resource sampling interval must be greater than zero",
		)
	}

	if len(o.QueryNames) == 0 {
		return errors.New(
			"at least one benchmark query name is required",
		)
	}

	for index, queryName := range o.QueryNames {
		if strings.TrimSpace(queryName) == "" {
			return fmt.Errorf(
				"benchmark query name at index %d is empty",
				index,
			)
		}
	}

	switch o.QueryType {
	case dns.TypeA, dns.TypeAAAA:
		return nil

	default:
		queryType := dns.TypeToString[o.QueryType]
		if queryType == "" {
			queryType = fmt.Sprintf(
				"TYPE%d",
				o.QueryType,
			)
		}

		return fmt.Errorf(
			"unsupported benchmark query type %q; supported types are A and AAAA",
			queryType,
		)
	}
}

// RunBenchmark executes the configured HomeDNS benchmark suite and writes its
// machine-readable JSON report.
func RunBenchmark(
	ctx context.Context,
	cfg config.Config,
	options BenchmarkOptions,
) (benchmark.ServiceResult, error) {
	if ctx == nil {
		return benchmark.ServiceResult{},
			errors.New(
				"benchmark context is required",
			)
	}

	options = options.Resolve(cfg)

	queryNames, err := normalizeBenchmarkQueryNames(
		options.QueryNames,
	)
	if err != nil {
		return benchmark.ServiceResult{},
			fmt.Errorf(
				"normalize benchmark query names: %w",
				err,
			)
	}

	options.QueryNames = queryNames

	if err := options.Validate(); err != nil {
		return benchmark.ServiceResult{},
			fmt.Errorf(
				"validate benchmark options: %w",
				err,
			)
	}

	suite := benchmark.DefaultSuiteConfig(
		cfg.Upstream.Address,
		cfg.DNSAddress(),
	)

	suite.QueryCount = options.QueryCount
	suite.WarmupQueries = options.WarmupQueries
	suite.ConcurrentWorkers =
		options.ConcurrentWorkers
	suite.Timeout = options.Timeout
	suite.QueryType = options.QueryType
	suite.QueryNames = append(
		[]string(nil),
		options.QueryNames...,
	)

	suite.ProgressReporter =
		options.ProgressReporter

	hostname, err := os.Hostname()
	if err != nil {
		return benchmark.ServiceResult{},
			fmt.Errorf(
				"read benchmark hostname: %w",
				err,
			)
	}

	targetPID, err := resolveBenchmarkTargetPID(
		options.ConfigPath,
	)
	if err != nil {
		return benchmark.ServiceResult{},
			fmt.Errorf(
				"resolve benchmark target process: %w",
				err,
			)
	}

	var resourceCollection *benchmark.ResourceCollectionConfig

	if targetPID > 0 {
		resourceCollection =
			&benchmark.ResourceCollectionConfig{
				BenchmarkPID: os.Getpid(),

				TargetPID: targetPID,

				Interval: options.ResourceSamplingInterval,
			}
	}

	service := benchmark.NewService()

	result, err := service.Run(
		ctx,
		benchmark.ServiceConfig{
			Profile: options.Profile,

			ProfileCustomized: options.ProfileCustomized,

			OutputDirectory: options.OutputDirectory,

			Suite: suite,

			BenchmarkBinary: benchmark.BenchmarkBinaryMetadata{
				Project: "homedns-analytics",

				Component: "homedns-dns",

				Version: version.Version,

				GitCommit: version.Commit,

				GoVersion: runtime.Version(),

				BuildTimestamp: version.BuildTime,
			},

			TargetService: benchmark.TargetServiceMetadata{
				Address: cfg.DNSAddress(),

				HealthAddress: cfg.HealthAddress(),
			},

			Environment: benchmark.Environment{
				Hostname: hostname,

				OperatingSystem: runtime.GOOS,

				Architecture: runtime.GOARCH,

				LogicalCPUs: runtime.NumCPU(),
			},

			ResourceCollection: resourceCollection,

			HealthAddress: cfg.HealthAddress(),
		},
	)
	if err != nil {
		return benchmark.ServiceResult{},
			fmt.Errorf(
				"run DNS benchmark: %w",
				err,
			)
	}

	return result, nil
}

func normalizeBenchmarkQueryNames(
	queryNames []string,
) ([]string, error) {
	if len(queryNames) == 0 {
		return nil, errors.New(
			"at least one benchmark query name is required",
		)
	}

	normalized := make(
		[]string,
		0,
		len(queryNames),
	)

	for index, queryName := range queryNames {
		queryName = strings.TrimSpace(
			queryName,
		)

		if queryName == "" {
			return nil, fmt.Errorf(
				"benchmark query name at index %d is empty",
				index,
			)
		}

		normalized = append(
			normalized,
			dns.Fqdn(queryName),
		)
	}

	return normalized, nil
}
