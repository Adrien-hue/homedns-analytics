package benchmark

import (
	"context"
	"errors"
	"time"
)

const DefaultResourceSamplingInterval = time.Second

// ResourceCollectionConfig identifies the benchmark process, the target
// HomeDNS service, and the interval used for periodic measurements.
type ResourceCollectionConfig struct {
	BenchmarkPID int
	TargetPID    int

	Interval time.Duration
}

// Validate verifies that resource collection can be started safely.
func (c ResourceCollectionConfig) Validate() error {
	if c.BenchmarkPID <= 0 {
		return errors.New(
			"benchmark process PID must be greater than zero",
		)
	}

	if c.TargetPID <= 0 {
		return errors.New(
			"target service PID must be greater than zero",
		)
	}

	if c.Interval <= 0 {
		return errors.New(
			"resource sampling interval must be greater than zero",
		)
	}

	return nil
}

// ResourceSample is one point-in-time measurement collected during a
// benchmark.
//
// Individual values are pointers because a platform may support only part of
// the resource contract. Unsupported or temporarily unavailable values remain
// nil instead of being reported as zero.
type ResourceSample struct {
	CollectedAt time.Time

	BenchmarkProcess ProcessResourceSample
	TargetService    ProcessResourceSample
	System           SystemResourceSample

	Errors []string
}

// ProcessResourceSample contains one process measurement.
type ProcessResourceSample struct {
	PID int

	CPUPercent *float64
	RSSBytes   *uint64
	Threads    *uint64
}

// SystemResourceSample contains one machine-level measurement.
type SystemResourceSample struct {
	LoadAverageOneMinute *float64
	MemoryAvailableBytes *uint64
	TemperatureCelsius   *float64
	ThrottledState       *string
}

// ResourceCollectionResult contains all samples and boundary measurements
// collected around one benchmark execution.
type ResourceCollectionResult struct {
	StartedAt time.Time
	EndedAt   time.Time

	BenchmarkPIDBefore int
	BenchmarkPIDAfter  int

	TargetPIDBefore int
	TargetPIDAfter  int

	Before ResourceSample
	After  ResourceSample

	Samples []ResourceSample
	Errors  []string
}

// ResourceCollector collects process and host measurements while a benchmark
// function executes.
type ResourceCollector interface {
	Collect(
		context.Context,
		ResourceCollectionConfig,
		func(context.Context) error,
	) (ResourceCollectionResult, error)
}

// ResourceCollectorFunc adapts a function to ResourceCollector.
type ResourceCollectorFunc func(
	context.Context,
	ResourceCollectionConfig,
	func(context.Context) error,
) (ResourceCollectionResult, error)

// Collect executes the adapted resource collector function.
func (f ResourceCollectorFunc) Collect(
	ctx context.Context,
	config ResourceCollectionConfig,
	run func(context.Context) error,
) (ResourceCollectionResult, error) {
	if f == nil {
		return ResourceCollectionResult{},
			errors.New(
				"resource collector function is required",
			)
	}

	return f(
		ctx,
		config,
		run,
	)
}

// NoopResourceCollector executes the benchmark without collecting resources.
//
// It is useful on unsupported operating systems and in tests that do not need
// resource measurements.
type NoopResourceCollector struct{}

// Collect executes the supplied benchmark function without collecting
// resource samples.
func (NoopResourceCollector) Collect(
	ctx context.Context,
	config ResourceCollectionConfig,
	run func(context.Context) error,
) (ResourceCollectionResult, error) {
	if ctx == nil {
		return ResourceCollectionResult{},
			errors.New(
				"resource collection context is required",
			)
	}

	if run == nil {
		return ResourceCollectionResult{},
			errors.New(
				"resource collection run function is required",
			)
	}

	if err := config.Validate(); err != nil {
		return ResourceCollectionResult{},
			err
	}

	startedAt := time.Now().UTC()

	runErr := run(ctx)

	endedAt := time.Now().UTC()

	return ResourceCollectionResult{
		StartedAt: startedAt,
		EndedAt:   endedAt,

		BenchmarkPIDBefore: config.BenchmarkPID,

		BenchmarkPIDAfter: config.BenchmarkPID,

		TargetPIDBefore: config.TargetPID,

		TargetPIDAfter: config.TargetPID,

		Samples: make([]ResourceSample, 0),

		Errors: make([]string, 0),
	}, runErr
}
