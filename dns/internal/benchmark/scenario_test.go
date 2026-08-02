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
	attemptedQueriesPerSecond float64,
	successfulQueriesPerSecond float64,
) PathResult {
	return PathResult{
		Requests: RequestCounts{
			Attempted:  requests,
			Successful: requests,
		},
		Rates: RequestRates{
			SuccessPercent: 100,
		},
		Throughput: ThroughputMetrics{
			AttemptedQueriesPerSecond:  attemptedQueriesPerSecond,
			SuccessfulQueriesPerSecond: successfulQueriesPerSecond,
		},
		SuccessfulRequestLatencyMilliseconds: SuccessfulLatencyStatistics{
			SampleCount: requests,
			Minimum:     meanLatency,
			Mean:        meanLatency,
			Median:      meanLatency,
			P95:         meanLatency,
			P99:         meanLatency,
			Maximum:     meanLatency,
		},
	}
}

func TestScenarioRunnerRunScenario(t *testing.T) {
	t.Parallel()

	fakeRunner := &recordedPathRunner{
		results: []PathResult{
			successfulPathResult(2, 1, 100, 100),
			successfulPathResult(2, 2, 100, 100),
			successfulPathResult(10, 2, 100, 100),
			successfulPathResult(10, 3, 80, 80),
		},
	}

	scenarioRunner := &ScenarioRunner{
		newRunner: func(
			protocol string,
			timeout time.Duration,
		) (pathRunner, error) {
			if protocol != ProtocolUDP {
				t.Fatalf(
					"unexpected protocol: got %q, want %q",
					protocol,
					ProtocolUDP,
				)
			}

			if timeout != 3*time.Second {
				t.Fatalf(
					"unexpected timeout: got %s, want %s",
					timeout,
					3*time.Second,
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

	if result.Protocol != config.Protocol {
		t.Fatalf(
			"unexpected scenario protocol: got %q, want %q",
			result.Protocol,
			config.Protocol,
		)
	}

	if result.Concurrency != config.Concurrency {
		t.Fatalf(
			"unexpected concurrency: got %d, want %d",
			result.Concurrency,
			config.Concurrency,
		)
	}

	if result.QueryCountPerPath != config.QueryCount {
		t.Fatalf(
			"unexpected query count: got %d, want %d",
			result.QueryCountPerPath,
			config.QueryCount,
		)
	}

	if result.Status != StatusUnknown {
		t.Fatalf(
			"unexpected initial status: got %q, want %q",
			result.Status,
			StatusUnknown,
		)
	}

	if result.StatusReasons == nil {
		t.Fatal("expected status reasons to be initialized")
	}

	if result.Direct.
		SuccessfulRequestLatencyMilliseconds.
		Mean != 2 {
		t.Fatalf(
			"unexpected direct latency: %f",
			result.Direct.
				SuccessfulRequestLatencyMilliseconds.
				Mean,
		)
	}

	if result.Forwarded.
		SuccessfulRequestLatencyMilliseconds.
		Mean != 3 {
		t.Fatalf(
			"unexpected forwarded latency: %f",
			result.Forwarded.
				SuccessfulRequestLatencyMilliseconds.
				Mean,
		)
	}

	comparison := result.Comparison

	if comparison.LatencyOverheadMilliseconds.Mean != 1 {
		t.Fatalf(
			"unexpected mean latency overhead: %f",
			comparison.LatencyOverheadMilliseconds.Mean,
		)
	}

	if comparison.LatencyOverheadPercent.Mean != 50 {
		t.Fatalf(
			"unexpected mean latency percentage: %f",
			comparison.LatencyOverheadPercent.Mean,
		)
	}

	if comparison.ThroughputChangePercent.Attempted != -20 {
		t.Fatalf(
			"unexpected attempted throughput change: %f",
			comparison.ThroughputChangePercent.Attempted,
		)
	}

	if comparison.ThroughputChangePercent.Successful != -20 {
		t.Fatalf(
			"unexpected successful throughput change: %f",
			comparison.ThroughputChangePercent.Successful,
		)
	}

	if comparison.
		ReliabilityDeltaPercentagePoints.
		Success != 0 {
		t.Fatalf(
			"unexpected success-rate delta: %f",
			comparison.
				ReliabilityDeltaPercentagePoints.
				Success,
		)
	}
}

func TestScenarioRunnerSkipsWarmup(t *testing.T) {
	t.Parallel()

	fakeRunner := &recordedPathRunner{
		results: []PathResult{
			successfulPathResult(10, 2, 100, 100),
			successfulPathResult(10, 3, 80, 80),
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

func TestScenarioRunnerRejectsFailedDirectWarmup(
	t *testing.T,
) {
	t.Parallel()

	fakeRunner := &recordedPathRunner{
		results: []PathResult{
			{
				Requests: RequestCounts{
					Attempted:  2,
					Successful: 1,
					Failed:     1,
				},
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
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fakeRunner.configs) != 1 {
		t.Fatalf(
			"unexpected path execution count: got %d, want %d",
			len(fakeRunner.configs),
			1,
		)
	}
}

func TestScenarioRunnerRejectsFailedForwardedWarmup(
	t *testing.T,
) {
	t.Parallel()

	fakeRunner := &recordedPathRunner{
		results: []PathResult{
			successfulPathResult(2, 1, 100, 100),
			{
				Requests: RequestCounts{
					Attempted:  2,
					Successful: 1,
					Failed:     1,
				},
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
		t.Fatal("expected failed forwarded warmup to fail")
	}

	if !strings.Contains(
		err.Error(),
		"warm up forwarded DNS path",
	) {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fakeRunner.configs) != 2 {
		t.Fatalf(
			"unexpected path execution count: got %d, want %d",
			len(fakeRunner.configs),
			2,
		)
	}
}

func TestScenarioRunnerReturnsDirectPathError(
	t *testing.T,
) {
	t.Parallel()

	fakeRunner := &recordedPathRunner{
		errs: []error{
			nil,
			nil,
			errors.New("direct path unavailable"),
		},
		results: []PathResult{
			successfulPathResult(2, 1, 100, 100),
			successfulPathResult(2, 1, 100, 100),
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
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScenarioRunnerReturnsForwardedPathError(
	t *testing.T,
) {
	t.Parallel()

	fakeRunner := &recordedPathRunner{
		errs: []error{
			nil,
			nil,
			nil,
			errors.New("forwarded path unavailable"),
		},
		results: []PathResult{
			successfulPathResult(2, 1, 100, 100),
			successfulPathResult(2, 1, 100, 100),
			successfulPathResult(10, 2, 100, 100),
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
		t.Fatal("expected forwarded path error")
	}

	if !strings.Contains(
		err.Error(),
		"benchmark forwarded DNS path",
	) {
		t.Fatalf("unexpected error: %v", err)
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

func TestScenarioRunnerRejectsFactoryError(
	t *testing.T,
) {
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

func TestScenarioRunnerRejectsNilFactoryRunner(
	t *testing.T,
) {
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

func TestScenarioRunnerRejectsNilReceiver(
	t *testing.T,
) {
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

func TestScenarioRunnerRejectsNilContext(
	t *testing.T,
) {
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

func TestCalculateScenarioComparison(t *testing.T) {
	t.Parallel()

	direct := successfulPathResult(
		100,
		4,
		200,
		190,
	)

	direct.SuccessfulRequestLatencyMilliseconds.Median = 3
	direct.SuccessfulRequestLatencyMilliseconds.P95 = 6
	direct.SuccessfulRequestLatencyMilliseconds.P99 = 8

	direct.Rates = RequestRates{
		SuccessPercent: 95,
		FailurePercent: 5,
		TimeoutPercent: 2,
	}

	forwarded := successfulPathResult(
		100,
		5,
		180,
		170,
	)

	forwarded.SuccessfulRequestLatencyMilliseconds.Median = 4
	forwarded.SuccessfulRequestLatencyMilliseconds.P95 = 8
	forwarded.SuccessfulRequestLatencyMilliseconds.P99 = 11

	forwarded.Rates = RequestRates{
		SuccessPercent: 90,
		FailurePercent: 10,
		TimeoutPercent: 6,
	}

	comparison := calculateScenarioComparison(
		direct,
		forwarded,
	)

	assertLatencyComparison(
		t,
		comparison.LatencyOverheadMilliseconds,
		LatencyComparison{
			Mean:   1,
			Median: 1,
			P95:    2,
			P99:    3,
		},
	)

	assertLatencyComparison(
		t,
		comparison.LatencyOverheadPercent,
		LatencyComparison{
			Mean:   25,
			Median: 33.333333,
			P95:    33.333333,
			P99:    37.5,
		},
	)

	if comparison.ThroughputChangePercent.Attempted != -10 {
		t.Fatalf(
			"unexpected attempted throughput change: got %f, want %f",
			comparison.ThroughputChangePercent.Attempted,
			-10.0,
		)
	}

	expectedSuccessfulChange := round(
		(170.0-190.0)/190.0*100,
		6,
	)

	if comparison.
		ThroughputChangePercent.
		Successful != expectedSuccessfulChange {
		t.Fatalf(
			"unexpected successful throughput change: got %f, want %f",
			comparison.
				ThroughputChangePercent.
				Successful,
			expectedSuccessfulChange,
		)
	}

	reliability :=
		comparison.
			ReliabilityDeltaPercentagePoints

	if reliability.Success != -5 {
		t.Fatalf(
			"unexpected success delta: got %f, want %f",
			reliability.Success,
			-5.0,
		)
	}

	if reliability.Failure != 5 {
		t.Fatalf(
			"unexpected failure delta: got %f, want %f",
			reliability.Failure,
			5.0,
		)
	}

	if reliability.Timeout != 4 {
		t.Fatalf(
			"unexpected timeout delta: got %f, want %f",
			reliability.Timeout,
			4.0,
		)
	}
}

func TestCalculateScenarioComparisonHandlesZeroBaselines(
	t *testing.T,
) {
	t.Parallel()

	comparison := calculateScenarioComparison(
		PathResult{},
		PathResult{},
	)

	if comparison.LatencyOverheadPercent !=
		(LatencyComparison{}) {
		t.Fatalf(
			"unexpected latency percentage comparison: %+v",
			comparison.LatencyOverheadPercent,
		)
	}

	if comparison.ThroughputChangePercent !=
		(ThroughputComparison{}) {
		t.Fatalf(
			"unexpected throughput comparison: %+v",
			comparison.ThroughputChangePercent,
		)
	}
}

func TestDifference(t *testing.T) {
	t.Parallel()

	if actual := difference(4, 5.25); actual != 1.25 {
		t.Fatalf(
			"unexpected difference: got %f, want %f",
			actual,
			1.25,
		)
	}
}

func TestPercentageChange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		baseline float64
		value    float64
		expected float64
	}{
		{
			name:     "increase",
			baseline: 100,
			value:    125,
			expected: 25,
		},
		{
			name:     "decrease",
			baseline: 100,
			value:    80,
			expected: -20,
		},
		{
			name:     "zero baseline",
			baseline: 0,
			value:    100,
			expected: 0,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual := percentageChange(
				test.baseline,
				test.value,
			)

			if actual != test.expected {
				t.Fatalf(
					"unexpected percentage change: got %f, want %f",
					actual,
					test.expected,
				)
			}
		})
	}
}

func TestPercentagePointDifference(t *testing.T) {
	t.Parallel()

	if actual := percentagePointDifference(
		97.5,
		95,
	); actual != -2.5 {
		t.Fatalf(
			"unexpected percentage-point difference: got %f, want %f",
			actual,
			-2.5,
		)
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

func assertLatencyComparison(
	t *testing.T,
	actual LatencyComparison,
	expected LatencyComparison,
) {
	t.Helper()

	if actual != expected {
		t.Fatalf(
			"unexpected latency comparison: got %+v, want %+v",
			actual,
			expected,
		)
	}
}

func TestScenarioRunnerReportsLifecycleEvents(
	t *testing.T,
) {
	t.Parallel()

	benchmarkStartedAt := time.Date(
		2026,
		time.August,
		1,
		10,
		0,
		0,
		0,
		time.UTC,
	)

	events := make(
		[]ProgressEvent,
		0,
	)

	reporter := ProgressReporterFunc(
		func(event ProgressEvent) {
			events = append(
				events,
				event,
			)
		},
	)

	directResult := successfulPathResult(
		10,
		2,
		100,
		100,
	)

	forwardedResult := successfulPathResult(
		10,
		3,
		80,
		80,
	)

	fakeRunner := &recordedPathRunner{
		results: []PathResult{
			successfulPathResult(
				2,
				1,
				100,
				100,
			),
			successfulPathResult(
				2,
				1,
				100,
				100,
			),
			directResult,
			forwardedResult,
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

	config.ProgressReporter =
		reporter

	config.BenchmarkStartedAt =
		benchmarkStartedAt

	config.ScenarioIndex = 2
	config.ScenarioCount = 4

	result, err := scenarioRunner.RunScenario(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf(
			"run benchmark scenario: %v",
			err,
		)
	}

	if len(events) != 2 {
		t.Fatalf(
			"unexpected scenario event count: got %d, want 2: %+v",
			len(events),
			events,
		)
	}

	started := events[0]

	if started.Type !=
		ProgressEventScenarioStarted {
		t.Fatalf(
			"unexpected first event type: got %q, want %q",
			started.Type,
			ProgressEventScenarioStarted,
		)
	}

	assertScenarioProgressMetadata(
		t,
		started,
		config,
	)

	if started.ScenarioResult != nil {
		t.Fatal(
			"scenario-started event unexpectedly contains a result",
		)
	}

	if started.Current != 0 {
		t.Fatalf(
			"unexpected scenario-started progress: got %d, want 0",
			started.Current,
		)
	}

	completed := events[1]

	if completed.Type !=
		ProgressEventScenarioCompleted {
		t.Fatalf(
			"unexpected final event type: got %q, want %q",
			completed.Type,
			ProgressEventScenarioCompleted,
		)
	}

	assertScenarioProgressMetadata(
		t,
		completed,
		config,
	)

	if completed.Phase !=
		ProgressPhaseForwarded {
		t.Fatalf(
			"unexpected completion phase: got %q, want %q",
			completed.Phase,
			ProgressPhaseForwarded,
		)
	}

	if completed.Current !=
		config.QueryCount {
		t.Fatalf(
			"unexpected completion progress: got %d, want %d",
			completed.Current,
			config.QueryCount,
		)
	}

	if completed.Successful !=
		forwardedResult.Requests.Successful {
		t.Fatalf(
			"unexpected completion success count: got %d, want %d",
			completed.Successful,
			forwardedResult.Requests.Successful,
		)
	}

	if completed.ScenarioResult == nil {
		t.Fatal(
			"scenario-completed event does not contain the scenario result",
		)
	}

	reported := completed.ScenarioResult

	if reported.Name != result.Name {
		t.Fatalf(
			"unexpected reported scenario name: got %q, want %q",
			reported.Name,
			result.Name,
		)
	}

	if reported.Direct.Requests !=
		result.Direct.Requests {
		t.Fatalf(
			"unexpected reported direct requests: got %+v, want %+v",
			reported.Direct.Requests,
			result.Direct.Requests,
		)
	}

	if reported.Forwarded.Requests !=
		result.Forwarded.Requests {
		t.Fatalf(
			"unexpected reported forwarded requests: got %+v, want %+v",
			reported.Forwarded.Requests,
			result.Forwarded.Requests,
		)
	}

	if reported.
		Comparison.
		LatencyOverheadMilliseconds.
		Mean != 1 {
		t.Fatalf(
			"unexpected reported mean overhead: got %f, want 1",
			reported.
				Comparison.
				LatencyOverheadMilliseconds.
				Mean,
		)
	}
}

func TestScenarioRunnerPassesProgressMetadataToPaths(
	t *testing.T,
) {
	t.Parallel()

	benchmarkStartedAt := time.Date(
		2026,
		time.August,
		1,
		10,
		30,
		0,
		0,
		time.UTC,
	)

	reporter := ProgressReporterFunc(
		func(ProgressEvent) {},
	)

	fakeRunner := &recordedPathRunner{
		results: []PathResult{
			successfulPathResult(
				2,
				1,
				100,
				100,
			),
			successfulPathResult(
				2,
				1,
				100,
				100,
			),
			successfulPathResult(
				10,
				2,
				100,
				100,
			),
			successfulPathResult(
				10,
				3,
				80,
				80,
			),
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

	config.ProgressReporter =
		reporter

	config.BenchmarkStartedAt =
		benchmarkStartedAt

	config.ScenarioIndex = 3
	config.ScenarioCount = 4

	_, err := scenarioRunner.RunScenario(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf(
			"run benchmark scenario: %v",
			err,
		)
	}

	if len(fakeRunner.configs) != 4 {
		t.Fatalf(
			"unexpected path count: got %d, want 4",
			len(fakeRunner.configs),
		)
	}

	expectedPhases := []string{
		ProgressPhaseWarmupDirect,
		ProgressPhaseWarmupForwarded,
		ProgressPhaseDirect,
		ProgressPhaseForwarded,
	}

	for index, pathConfig := range fakeRunner.configs {
		if pathConfig.ProgressReporter == nil {
			t.Fatalf(
				"path %d does not contain a progress reporter",
				index,
			)
		}

		if !pathConfig.
			BenchmarkStartedAt.
			Equal(
				benchmarkStartedAt,
			) {
			t.Fatalf(
				"unexpected path %d benchmark start: got %s, want %s",
				index,
				pathConfig.BenchmarkStartedAt,
				benchmarkStartedAt,
			)
		}

		if pathConfig.ScenarioName !=
			config.Name {
			t.Fatalf(
				"unexpected path %d scenario name: got %q, want %q",
				index,
				pathConfig.ScenarioName,
				config.Name,
			)
		}

		if pathConfig.ScenarioIndex != 3 {
			t.Fatalf(
				"unexpected path %d scenario index: got %d, want 3",
				index,
				pathConfig.ScenarioIndex,
			)
		}

		if pathConfig.ScenarioCount != 4 {
			t.Fatalf(
				"unexpected path %d scenario count: got %d, want 4",
				index,
				pathConfig.ScenarioCount,
			)
		}

		if pathConfig.Phase !=
			expectedPhases[index] {
			t.Fatalf(
				"unexpected path %d phase: got %q, want %q",
				index,
				pathConfig.Phase,
				expectedPhases[index],
			)
		}
	}
}

func TestScenarioRunnerReportsLifecycleWithoutWarmup(
	t *testing.T,
) {
	t.Parallel()

	events := make(
		[]ProgressEvent,
		0,
	)

	reporter := ProgressReporterFunc(
		func(event ProgressEvent) {
			events = append(
				events,
				event,
			)
		},
	)

	fakeRunner := &recordedPathRunner{
		results: []PathResult{
			successfulPathResult(
				10,
				2,
				100,
				100,
			),
			successfulPathResult(
				10,
				3,
				80,
				80,
			),
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
	config.ProgressReporter = reporter
	config.ScenarioIndex = 1
	config.ScenarioCount = 4

	_, err := scenarioRunner.RunScenario(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf(
			"run benchmark scenario: %v",
			err,
		)
	}

	if len(events) != 2 {
		t.Fatalf(
			"unexpected event count without warmup: got %d, want 2",
			len(events),
		)
	}

	if events[0].Type !=
		ProgressEventScenarioStarted {
		t.Fatalf(
			"unexpected first event: %q",
			events[0].Type,
		)
	}

	if events[1].Type !=
		ProgressEventScenarioCompleted {
		t.Fatalf(
			"unexpected final event: %q",
			events[1].Type,
		)
	}

	if events[1].ScenarioResult == nil {
		t.Fatal(
			"completed event does not contain a scenario result",
		)
	}
}

func TestScenarioRunnerWorksWithoutProgressReporter(
	t *testing.T,
) {
	t.Parallel()

	fakeRunner := &recordedPathRunner{
		results: []PathResult{
			successfulPathResult(
				2,
				1,
				100,
				100,
			),
			successfulPathResult(
				2,
				1,
				100,
				100,
			),
			successfulPathResult(
				10,
				2,
				100,
				100,
			),
			successfulPathResult(
				10,
				3,
				80,
				80,
			),
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
	config.ProgressReporter = nil

	result, err := scenarioRunner.RunScenario(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf(
			"run benchmark scenario: %v",
			err,
		)
	}

	if result.Name != config.Name {
		t.Fatalf(
			"unexpected scenario name: got %q, want %q",
			result.Name,
			config.Name,
		)
	}
}

func assertScenarioProgressMetadata(
	t *testing.T,
	event ProgressEvent,
	config ScenarioConfig,
) {
	t.Helper()

	if event.ScenarioName !=
		config.Name {
		t.Fatalf(
			"unexpected scenario name: got %q, want %q",
			event.ScenarioName,
			config.Name,
		)
	}

	if event.ScenarioIndex !=
		config.ScenarioIndex {
		t.Fatalf(
			"unexpected scenario index: got %d, want %d",
			event.ScenarioIndex,
			config.ScenarioIndex,
		)
	}

	if event.ScenarioCount !=
		config.ScenarioCount {
		t.Fatalf(
			"unexpected scenario count: got %d, want %d",
			event.ScenarioCount,
			config.ScenarioCount,
		)
	}

	if !event.BenchmarkStartedAt.Equal(
		config.BenchmarkStartedAt,
	) {
		t.Fatalf(
			"unexpected benchmark start: got %s, want %s",
			event.BenchmarkStartedAt,
			config.BenchmarkStartedAt,
		)
	}

	if event.OccurredAt.IsZero() {
		t.Fatal(
			"progress event occurrence time is zero",
		)
	}

	if event.Elapsed < 0 {
		t.Fatalf(
			"progress event has negative elapsed time: %s",
			event.Elapsed,
		)
	}
}
