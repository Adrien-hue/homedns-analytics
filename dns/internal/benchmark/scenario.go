package benchmark

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/miekg/dns"
)

// ScenarioConfig defines one direct-versus-forwarded DNS benchmark.
type ScenarioConfig struct {
	Name             string
	Protocol         string
	DirectAddress    string
	ForwardedAddress string

	QueryNames []string
	QueryType  uint16

	WarmupQueries int
	QueryCount    int
	Concurrency   int
	Timeout       time.Duration
}

// Validate verifies that a benchmark scenario can be executed.
func (c ScenarioConfig) Validate() error {
	if c.Name == "" {
		return errors.New("scenario name is required")
	}

	if c.WarmupQueries < 0 {
		return errors.New(
			"warmup query count cannot be negative",
		)
	}

	directConfig := c.pathConfig(
		c.DirectAddress,
		c.QueryCount,
		c.Concurrency,
	)

	if err := directConfig.Validate(); err != nil {
		return fmt.Errorf(
			"invalid direct path configuration: %w",
			err,
		)
	}

	forwardedConfig := c.pathConfig(
		c.ForwardedAddress,
		c.QueryCount,
		c.Concurrency,
	)

	if err := forwardedConfig.Validate(); err != nil {
		return fmt.Errorf(
			"invalid forwarded path configuration: %w",
			err,
		)
	}

	return nil
}

func (c ScenarioConfig) pathConfig(
	address string,
	queryCount int,
	concurrency int,
) PathConfig {
	return PathConfig{
		Address:     address,
		Protocol:    c.Protocol,
		QueryNames:  append([]string(nil), c.QueryNames...),
		QueryType:   c.QueryType,
		QueryCount:  queryCount,
		Concurrency: concurrency,
		Timeout:     c.Timeout,
	}
}

type pathRunner interface {
	RunPath(
		context.Context,
		PathConfig,
	) (PathResult, error)
}

type runnerFactory func(
	string,
	time.Duration,
) (pathRunner, error)

// ScenarioRunner executes complete direct-versus-forwarded scenarios.
type ScenarioRunner struct {
	newRunner runnerFactory
}

// NewScenarioRunner creates a scenario runner backed by miekg/dns.
func NewScenarioRunner() *ScenarioRunner {
	return &ScenarioRunner{
		newRunner: func(
			protocol string,
			timeout time.Duration,
		) (pathRunner, error) {
			return NewRunner(protocol, timeout)
		},
	}
}

// RunScenario executes warmup, direct measurement, forwarded measurement,
// and comparison calculation.
func (r *ScenarioRunner) RunScenario(
	ctx context.Context,
	config ScenarioConfig,
) (ScenarioResult, error) {
	if r == nil || r.newRunner == nil {
		return ScenarioResult{}, errors.New(
			"scenario runner is not initialized",
		)
	}

	if ctx == nil {
		return ScenarioResult{}, errors.New(
			"benchmark context is required",
		)
	}

	if err := config.Validate(); err != nil {
		return ScenarioResult{}, fmt.Errorf(
			"validate benchmark scenario: %w",
			err,
		)
	}

	runner, err := r.newRunner(
		config.Protocol,
		config.Timeout,
	)
	if err != nil {
		return ScenarioResult{}, fmt.Errorf(
			"create DNS benchmark runner: %w",
			err,
		)
	}

	if runner == nil {
		return ScenarioResult{}, errors.New(
			"DNS benchmark runner factory returned nil",
		)
	}

	if config.WarmupQueries > 0 {
		if err := warmupPath(
			ctx,
			runner,
			config.pathConfig(
				config.DirectAddress,
				config.WarmupQueries,
				1,
			),
		); err != nil {
			return ScenarioResult{}, fmt.Errorf(
				"warm up direct DNS path: %w",
				err,
			)
		}

		if err := warmupPath(
			ctx,
			runner,
			config.pathConfig(
				config.ForwardedAddress,
				config.WarmupQueries,
				1,
			),
		); err != nil {
			return ScenarioResult{}, fmt.Errorf(
				"warm up forwarded DNS path: %w",
				err,
			)
		}
	}

	directResult, err := runner.RunPath(
		ctx,
		config.pathConfig(
			config.DirectAddress,
			config.QueryCount,
			config.Concurrency,
		),
	)
	if err != nil {
		return ScenarioResult{}, fmt.Errorf(
			"benchmark direct DNS path: %w",
			err,
		)
	}

	forwardedResult, err := runner.RunPath(
		ctx,
		config.pathConfig(
			config.ForwardedAddress,
			config.QueryCount,
			config.Concurrency,
		),
	)
	if err != nil {
		return ScenarioResult{}, fmt.Errorf(
			"benchmark forwarded DNS path: %w",
			err,
		)
	}

	return ScenarioResult{
		Name:              config.Name,
		Protocol:          config.Protocol,
		Concurrency:       config.Concurrency,
		QueryCountPerPath: config.QueryCount,

		Status:        StatusUnknown,
		StatusReasons: make([]StatusReason, 0),

		Direct:    directResult,
		Forwarded: forwardedResult,

		Comparison: calculateScenarioComparison(
			directResult,
			forwardedResult,
		),
	}, nil
}

func warmupPath(
	ctx context.Context,
	runner pathRunner,
	config PathConfig,
) error {
	result, err := runner.RunPath(ctx, config)
	if err != nil {
		return err
	}

	if result.Requests.Successful != config.QueryCount {
		return fmt.Errorf(
			"%d of %d warmup queries succeeded",
			result.Requests.Successful,
			config.QueryCount,
		)
	}

	return nil
}

func calculateScenarioComparison(
	direct PathResult,
	forwarded PathResult,
) ScenarioComparison {
	directLatency :=
		direct.SuccessfulRequestLatencyMilliseconds

	forwardedLatency :=
		forwarded.SuccessfulRequestLatencyMilliseconds

	return ScenarioComparison{
		LatencyOverheadMilliseconds: LatencyComparison{
			Mean: difference(
				directLatency.Mean,
				forwardedLatency.Mean,
			),
			Median: difference(
				directLatency.Median,
				forwardedLatency.Median,
			),
			P95: difference(
				directLatency.P95,
				forwardedLatency.P95,
			),
			P99: difference(
				directLatency.P99,
				forwardedLatency.P99,
			),
		},

		LatencyOverheadPercent: LatencyComparison{
			Mean: percentageChange(
				directLatency.Mean,
				forwardedLatency.Mean,
			),
			Median: percentageChange(
				directLatency.Median,
				forwardedLatency.Median,
			),
			P95: percentageChange(
				directLatency.P95,
				forwardedLatency.P95,
			),
			P99: percentageChange(
				directLatency.P99,
				forwardedLatency.P99,
			),
		},

		ThroughputChangePercent: ThroughputComparison{
			Attempted: percentageChange(
				direct.Throughput.
					AttemptedQueriesPerSecond,
				forwarded.Throughput.
					AttemptedQueriesPerSecond,
			),
			Successful: percentageChange(
				direct.Throughput.
					SuccessfulQueriesPerSecond,
				forwarded.Throughput.
					SuccessfulQueriesPerSecond,
			),
		},

		ReliabilityDeltaPercentagePoints: ReliabilityComparison{
			Success: percentagePointDifference(
				direct.Rates.SuccessPercent,
				forwarded.Rates.SuccessPercent,
			),
			Failure: percentagePointDifference(
				direct.Rates.FailurePercent,
				forwarded.Rates.FailurePercent,
			),
			Timeout: percentagePointDifference(
				direct.Rates.TimeoutPercent,
				forwarded.Rates.TimeoutPercent,
			),
		},
	}
}

func difference(
	baseline float64,
	value float64,
) float64 {
	return round(
		value-baseline,
		6,
	)
}

// percentageChange returns the relative change from the baseline.
//
// A positive value means the forwarded value is greater than the direct value.
// A negative value means it is lower.
//
// For latency, a positive value means slower forwarding. For throughput, a
// negative value means lower forwarded throughput.
func percentageChange(
	baseline float64,
	value float64,
) float64 {
	if baseline == 0 {
		return 0
	}

	return round(
		(value-baseline)/baseline*100,
		6,
	)
}

// percentagePointDifference returns the forwarded rate minus the direct rate.
//
// This intentionally uses percentage points rather than relative percentage
// change because it compares success, failure, and timeout rates.
func percentagePointDifference(
	direct float64,
	forwarded float64,
) float64 {
	return round(
		forwarded-direct,
		6,
	)
}

// Ensure the default benchmark query type remains available to callers of
// this package without introducing another query-type representation.
var _ = dns.TypeA
