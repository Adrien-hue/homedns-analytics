package benchmark

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
)

type recordedPathRunner struct {
	configs []PathConfig
	results []PathResult
	errs    []error
}

func (r *recordedPathRunner) RunPath(
	_ context.Context,
	config PathConfig,
) (PathResult, error) {
	r.configs = append(r.configs, config)

	index := len(r.configs) - 1

	var result PathResult
	if index < len(r.results) {
		result = r.results[index]
	}

	var err error
	if index < len(r.errs) {
		err = r.errs[index]
	}

	return result, err
}

func validScenarioConfig() ScenarioConfig {
	return ScenarioConfig{
		Name:             "udp-sequential",
		Protocol:         ProtocolUDP,
		DirectAddress:    "1.1.1.1:53",
		ForwardedAddress: "127.0.0.1:5300",
		QueryNames: []string{
			"example.com.",
			"cloudflare.com.",
		},
		QueryType:     dns.TypeA,
		WarmupQueries: 2,
		QueryCount:    10,
		Concurrency:   1,
		Timeout:       3 * time.Second,
	}
}

func successfulPathResult(
	requests int,
	meanLatency float64,
	queriesPerSecond float64,
) PathResult {
	return PathResult{
		Requests:         requests,
		Successful:       requests,
		QueriesPerSecond: queriesPerSecond,
		LatencyMilliseconds: LatencyStatistics{
			Minimum: meanLatency,
			Mean:    meanLatency,
			Median:  meanLatency,
			P95:     meanLatency,
			P99:     meanLatency,
			Maximum: meanLatency,
		},
	}
}

func TestScenarioRunnerRunScenario(t *testing.T) {
	t.Parallel()

	fakeRunner := &recordedPathRunner{
		results: []PathResult{
			successfulPathResult(2, 1, 100),
			successfulPathResult(2, 2, 100),
			successfulPathResult(10, 2, 100),
			successfulPathResult(10, 3, 80),
		},
	}

	scenarioRunner := &ScenarioRunner{
		newRunner: func(
			protocol string,
			timeout time.Duration,
		) (pathRunner, error) {
			if protocol != ProtocolUDP {
				t.Fatalf(
					"unexpected protocol: %q",
					protocol,
				)
			}

			if timeout != 3*time.Second {
				t.Fatalf(
					"unexpected timeout: %s",
					timeout,
				)
			}

			return fakeRunner, nil
		},
	}

	config := validScenarioConfig()

	result, err := scenarioRunner.RunScenario(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf("run benchmark scenario: %v", err)
	}

	if len(fakeRunner.configs) != 4 {
		t.Fatalf(
			"unexpected path execution count: got %d, want %d",
			len(fakeRunner.configs),
			4,
		)
	}

	assertPathConfig(
		t,
		fakeRunner.configs[0],
		config.DirectAddress,
		config.WarmupQueries,
		1,
	)

	assertPathConfig(
		t,
		fakeRunner.configs[1],
		config.ForwardedAddress,
		config.WarmupQueries,
		1,
	)

	assertPathConfig(
		t,
		fakeRunner.configs[2],
		config.DirectAddress,
		config.QueryCount,
		config.Concurrency,
	)

	assertPathConfig(
		t,
		fakeRunner.configs[3],
		config.ForwardedAddress,
		config.QueryCount,
		config.Concurrency,
	)

	if result.Name != config.Name {
		t.Fatalf(
			"unexpected scenario name: got %q, want %q",
			result.Name,
			config.Name,
		)
	}

	if result.Direct.LatencyMilliseconds.Mean != 2 {
		t.Fatalf(
			"unexpected direct latency: %f",
			result.Direct.LatencyMilliseconds.Mean,
		)
	}

	if result.Forwarded.LatencyMilliseconds.Mean != 3 {
		t.Fatalf(
			"unexpected forwarded latency: %f",
			result.Forwarded.LatencyMilliseconds.Mean,
		)
	}

	if result.Overhead.MeanLatencyMilliseconds != 1 {
		t.Fatalf(
			"unexpected mean latency overhead: %f",
			result.Overhead.MeanLatencyMilliseconds,
		)
	}

	if result.Overhead.MeanLatencyPercent != 50 {
		t.Fatalf(
			"unexpected mean latency percentage: %f",
			result.Overhead.MeanLatencyPercent,
		)
	}

	if result.Overhead.ThroughputPercent != 20 {
		t.Fatalf(
			"unexpected throughput loss percentage: %f",
			result.Overhead.ThroughputPercent,
		)
	}
}

func TestScenarioRunnerSkipsWarmup(t *testing.T) {
	t.Parallel()

	fakeRunner := &recordedPathRunner{
		results: []PathResult{
			successfulPathResult(10, 2, 100),
			successfulPathResult(10, 3, 80),
		},
	}

	scenarioRunner := &ScenarioRunner{
		newRunner: func(
			string,
			time.Duration,
		) (pathRunner, error) {
			return fakeRunner, nil
		},
	}

	config := validScenarioConfig()
	config.WarmupQueries = 0

	_, err := scenarioRunner.RunScenario(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf("run benchmark scenario: %v", err)
	}

	if len(fakeRunner.configs) != 2 {
		t.Fatalf(
			"unexpected path execution count: got %d, want %d",
			len(fakeRunner.configs),
			2,
		)
	}
}

func TestScenarioRunnerRejectsFailedWarmup(t *testing.T) {
	t.Parallel()

	fakeRunner := &recordedPathRunner{
		results: []PathResult{
			{
				Requests:   2,
				Successful: 1,
				Failed:     1,
			},
		},
	}

	scenarioRunner := &ScenarioRunner{
		newRunner: func(
			string,
			time.Duration,
		) (pathRunner, error) {
			return fakeRunner, nil
		},
	}

	_, err := scenarioRunner.RunScenario(
		context.Background(),
		validScenarioConfig(),
	)
	if err == nil {
		t.Fatal("expected failed warmup to fail")
	}

	if !strings.Contains(
		err.Error(),
		"warm up direct DNS path",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}

	if len(fakeRunner.configs) != 1 {
		t.Fatalf(
			"unexpected path execution count: got %d, want %d",
			len(fakeRunner.configs),
			1,
		)
	}
}

func TestScenarioRunnerReturnsPathError(t *testing.T) {
	t.Parallel()

	fakeRunner := &recordedPathRunner{
		errs: []error{
			nil,
			nil,
			errors.New("direct path unavailable"),
		},
		results: []PathResult{
			successfulPathResult(2, 1, 100),
			successfulPathResult(2, 1, 100),
		},
	}

	scenarioRunner := &ScenarioRunner{
		newRunner: func(
			string,
			time.Duration,
		) (pathRunner, error) {
			return fakeRunner, nil
		},
	}

	_, err := scenarioRunner.RunScenario(
		context.Background(),
		validScenarioConfig(),
	)
	if err == nil {
		t.Fatal("expected direct path error")
	}

	if !strings.Contains(
		err.Error(),
		"benchmark direct DNS path",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestScenarioRunnerRejectsInvalidConfiguration(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*ScenarioConfig)
	}{
		{
			name: "missing name",
			mutate: func(config *ScenarioConfig) {
				config.Name = ""
			},
		},
		{
			name: "negative warmup",
			mutate: func(config *ScenarioConfig) {
				config.WarmupQueries = -1
			},
		},
		{
			name: "missing direct address",
			mutate: func(config *ScenarioConfig) {
				config.DirectAddress = ""
			},
		},
		{
			name: "missing forwarded address",
			mutate: func(config *ScenarioConfig) {
				config.ForwardedAddress = ""
			},
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			config := validScenarioConfig()
			test.mutate(&config)

			if err := config.Validate(); err == nil {
				t.Fatal("expected validation to fail")
			}
		})
	}
}

func TestScenarioRunnerRejectsFactoryError(t *testing.T) {
	t.Parallel()

	scenarioRunner := &ScenarioRunner{
		newRunner: func(
			string,
			time.Duration,
		) (pathRunner, error) {
			return nil, errors.New(
				"create runner failure",
			)
		},
	}

	_, err := scenarioRunner.RunScenario(
		context.Background(),
		validScenarioConfig(),
	)
	if err == nil {
		t.Fatal("expected runner factory error")
	}
}

func TestScenarioRunnerRejectsNilFactoryRunner(t *testing.T) {
	t.Parallel()

	scenarioRunner := &ScenarioRunner{
		newRunner: func(
			string,
			time.Duration,
		) (pathRunner, error) {
			return nil, nil
		},
	}

	_, err := scenarioRunner.RunScenario(
		context.Background(),
		validScenarioConfig(),
	)
	if err == nil {
		t.Fatal("expected nil factory runner to fail")
	}
}

func TestScenarioRunnerRejectsNilReceiver(t *testing.T) {
	t.Parallel()

	var scenarioRunner *ScenarioRunner

	_, err := scenarioRunner.RunScenario(
		context.Background(),
		validScenarioConfig(),
	)
	if err == nil {
		t.Fatal("expected nil scenario runner to fail")
	}
}

func TestScenarioRunnerRejectsNilContext(t *testing.T) {
	t.Parallel()

	scenarioRunner := &ScenarioRunner{
		newRunner: func(
			string,
			time.Duration,
		) (pathRunner, error) {
			t.Fatal("runner factory must not be called")

			return nil, nil
		},
	}

	_, err := scenarioRunner.RunScenario(
		nil,
		validScenarioConfig(),
	)
	if err == nil {
		t.Fatal("expected nil context to fail")
	}
}

func TestCalculateOverhead(t *testing.T) {
	t.Parallel()

	direct := successfulPathResult(100, 4, 200)
	direct.LatencyMilliseconds.Median = 3
	direct.LatencyMilliseconds.P95 = 6
	direct.LatencyMilliseconds.P99 = 8

	forwarded := successfulPathResult(100, 5, 180)
	forwarded.LatencyMilliseconds.Median = 4
	forwarded.LatencyMilliseconds.P95 = 8
	forwarded.LatencyMilliseconds.P99 = 11

	overhead := calculateOverhead(
		direct,
		forwarded,
	)

	if overhead.MeanLatencyMilliseconds != 1 {
		t.Fatalf(
			"unexpected mean overhead: %f",
			overhead.MeanLatencyMilliseconds,
		)
	}

	if overhead.MedianLatencyMilliseconds != 1 {
		t.Fatalf(
			"unexpected median overhead: %f",
			overhead.MedianLatencyMilliseconds,
		)
	}

	if overhead.P95LatencyMilliseconds != 2 {
		t.Fatalf(
			"unexpected p95 overhead: %f",
			overhead.P95LatencyMilliseconds,
		)
	}

	if overhead.P99LatencyMilliseconds != 3 {
		t.Fatalf(
			"unexpected p99 overhead: %f",
			overhead.P99LatencyMilliseconds,
		)
	}

	if overhead.MeanLatencyPercent != 25 {
		t.Fatalf(
			"unexpected latency percentage: %f",
			overhead.MeanLatencyPercent,
		)
	}

	if overhead.ThroughputPercent != 10 {
		t.Fatalf(
			"unexpected throughput percentage: %f",
			overhead.ThroughputPercent,
		)
	}
}

func TestCalculateOverheadHandlesZeroBaselines(
	t *testing.T,
) {
	t.Parallel()

	overhead := calculateOverhead(
		PathResult{},
		PathResult{},
	)

	if overhead.MeanLatencyPercent != 0 {
		t.Fatalf(
			"unexpected latency percentage: %f",
			overhead.MeanLatencyPercent,
		)
	}

	if overhead.ThroughputPercent != 0 {
		t.Fatalf(
			"unexpected throughput percentage: %f",
			overhead.ThroughputPercent,
		)
	}
}

func TestRunnerRejectsProtocolMismatch(t *testing.T) {
	t.Parallel()

	runner := &Runner{
		protocol: ProtocolUDP,
		exchange: func(
			context.Context,
			*dns.Msg,
			string,
		) (*dns.Msg, time.Duration, error) {
			t.Fatal("exchange must not be called")

			return nil, 0, nil
		},
	}

	config := validPathConfig()
	config.Protocol = ProtocolTCP

	_, err := runner.RunPath(
		context.Background(),
		config,
	)
	if err == nil {
		t.Fatal("expected protocol mismatch to fail")
	}
}

func assertPathConfig(
	t *testing.T,
	config PathConfig,
	expectedAddress string,
	expectedQueryCount int,
	expectedConcurrency int,
) {
	t.Helper()

	if config.Address != expectedAddress {
		t.Fatalf(
			"unexpected address: got %q, want %q",
			config.Address,
			expectedAddress,
		)
	}

	if config.QueryCount != expectedQueryCount {
		t.Fatalf(
			"unexpected query count: got %d, want %d",
			config.QueryCount,
			expectedQueryCount,
		)
	}

	if config.Concurrency != expectedConcurrency {
		t.Fatalf(
			"unexpected concurrency: got %d, want %d",
			config.Concurrency,
			expectedConcurrency,
		)
	}
}
