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
// and overhead calculation.
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
		Name:        config.Name,
		Protocol:    config.Protocol,
		Concurrency: config.Concurrency,
		QueryCount:  config.QueryCount,
		Direct:      directResult,
		Forwarded:   forwardedResult,
		Overhead: calculateOverhead(
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

	if result.Successful != config.QueryCount {
		return fmt.Errorf(
			"%d of %d warmup queries succeeded",
			result.Successful,
			config.QueryCount,
		)
	}

	return nil
}

func calculateOverhead(
	direct PathResult,
	forwarded PathResult,
) OverheadResult {
	return OverheadResult{
		MeanLatencyMilliseconds: round(
			forwarded.LatencyMilliseconds.Mean-
				direct.LatencyMilliseconds.Mean,
			6,
		),
		MedianLatencyMilliseconds: round(
			forwarded.LatencyMilliseconds.Median-
				direct.LatencyMilliseconds.Median,
			6,
		),
		P95LatencyMilliseconds: round(
			forwarded.LatencyMilliseconds.P95-
				direct.LatencyMilliseconds.P95,
			6,
		),
		P99LatencyMilliseconds: round(
			forwarded.LatencyMilliseconds.P99-
				direct.LatencyMilliseconds.P99,
			6,
		),
		MeanLatencyPercent: percentageChange(
			direct.LatencyMilliseconds.Mean,
			forwarded.LatencyMilliseconds.Mean,
		),
		ThroughputPercent: throughputLossPercent(
			direct.QueriesPerSecond,
			forwarded.QueriesPerSecond,
		),
	}
}

// percentageChange returns the relative increase from the baseline.
//
// A positive value means the forwarded path is slower. A negative value means
// it is faster.
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

// throughputLossPercent returns throughput lost compared with the direct path.
//
// A positive value means lower forwarded throughput. A negative value means
// the forwarded path measured higher throughput.
func throughputLossPercent(
	direct float64,
	forwarded float64,
) float64 {
	if direct == 0 {
		return 0
	}

	return round(
		(direct-forwarded)/direct*100,
		6,
	)
}

// Ensure the default benchmark query type remains available to callers of
// this package without introducing another query-type representation.
var _ = dns.TypeA
