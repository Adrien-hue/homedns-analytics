package benchmark

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
)

type recordedSuiteRunner struct {
	configs []SuiteConfig
	results []ScenarioResult
	err     error
}

func (r *recordedSuiteRunner) Run(
	_ context.Context,
	config SuiteConfig,
) ([]ScenarioResult, error) {
	r.configs = append(r.configs, config)

	return r.results, r.err
}

type recordedResourceCollector struct {
	called bool
	config ResourceCollectionConfig
	result ResourceCollectionResult
	err    error
}

func (r *recordedResourceCollector) Collect(
	ctx context.Context,
	config ResourceCollectionConfig,
	run func(context.Context) error,
) (ResourceCollectionResult, error) {
	r.called = true
	r.config = config

	runErr := run(ctx)
	if runErr != nil {
		return r.result, runErr
	}

	return r.result, r.err
}

func validReportRunConfig() ReportRunConfig {
	dirty := true

	return ReportRunConfig{
		ID: "benchmark-20260728-200000",

		Profile:           DefaultProfile,
		ProfileCustomized: false,

		Suite: SuiteConfig{
			DirectAddress:    "1.1.1.1:53",
			ForwardedAddress: "127.0.0.1:5300",

			QueryNames: []string{
				"example.com.",
				"cloudflare.com.",
			},

			QueryType:         dns.TypeA,
			WarmupQueries:     5,
			QueryCount:        20,
			ConcurrentWorkers: 5,
			Timeout:           1500 * time.Millisecond,
		},

		BenchmarkBinary: BenchmarkBinaryMetadata{
			Project:        "homedns-analytics",
			Component:      "homedns-dns",
			Version:        "v0.2.0",
			GitCommit:      "abc123",
			GitBranch:      "feat/benchmark-report-v2",
			GitDirty:       &dirty,
			GoVersion:      "go1.26.5",
			BuildTimestamp: "2026-07-28T17:55:00Z",
		},

		TargetService: TargetServiceMetadata{
			Address:       "127.0.0.1:5300",
			HealthAddress: "127.0.0.1:8081",
			Version:       "v0.2.0-rc1",
			GitCommit:     "def456",
			BuildTime:     "2026-07-28T17:30:00Z",
		},

		Environment: Environment{
			Hostname:        "homedns",
			OperatingSystem: "linux",
			Kernel:          "6.12.0",
			Architecture:    "arm64",
			CPUModel:        "ARM Cortex-A53",
			LogicalCPUs:     4,
		},

		ResourceCollection: &ResourceCollectionConfig{
			BenchmarkPID: 100,
			TargetPID:    200,
			Interval:     time.Second,
		},

		HealthAddress: "127.0.0.1:8081",
	}
}

func successfulReportScenario(
	name string,
	protocol string,
	concurrency int,
) ScenarioResult {
	return ScenarioResult{
		Name:              name,
		Protocol:          protocol,
		Concurrency:       concurrency,
		QueryCountPerPath: 10,

		Direct: successfulPathResult(
			10,
			2,
			100,
			100,
		),

		Forwarded: successfulPathResult(
			10,
			3,
			90,
			90,
		),
	}
}

func degradedReportScenario(
	name string,
) ScenarioResult {
	scenario := successfulReportScenario(
		name,
		ProtocolTCP,
		10,
	)

	scenario.Forwarded.Requests = RequestCounts{
		Attempted:          100,
		Successful:         98,
		Failed:             2,
		Timeouts:           1,
		NonTimeoutFailures: 1,
	}

	scenario.Forwarded.Rates = calculateRequestRates(
		scenario.Forwarded.Requests,
	)

	return scenario
}

func failedReportScenario(
	name string,
) ScenarioResult {
	scenario := successfulReportScenario(
		name,
		ProtocolTCP,
		10,
	)

	scenario.Forwarded.Requests = RequestCounts{
		Attempted:  100,
		Successful: 90,
		Failed:     10,
		Timeouts:   8,

		NonTimeoutFailures: 2,
	}

	scenario.Forwarded.Rates = calculateRequestRates(
		scenario.Forwarded.Requests,
	)

	return scenario
}

func TestReportRunnerRun(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(
		2026,
		time.July,
		28,
		18,
		0,
		0,
		0,
		time.UTC,
	)

	endedAt := startedAt.Add(
		2500 * time.Millisecond,
	)

	times := []time.Time{
		startedAt,
		endedAt,
	}

	clockCalls := 0

	fakeSuite := &recordedSuiteRunner{
		results: []ScenarioResult{
			successfulReportScenario(
				"udp-sequential",
				ProtocolUDP,
				1,
			),
			successfulReportScenario(
				"udp-concurrent",
				ProtocolUDP,
				5,
			),
		},
	}

	runner := &ReportRunner{
		suiteRunner: fakeSuite,
		now: func() time.Time {
			value := times[clockCalls]
			clockCalls++

			return value
		},
		resourceCollector: NoopResourceCollector{},
	}

	config := validReportRunConfig()

	report, err := runner.Run(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf(
			"run benchmark report: %v",
			err,
		)
	}

	if report.SchemaVersion != SchemaVersion {
		t.Fatalf(
			"unexpected schema version: got %q, want %q",
			report.SchemaVersion,
			SchemaVersion,
		)
	}

	if report.Report.ID != config.ID {
		t.Fatalf(
			"unexpected report ID: got %q, want %q",
			report.Report.ID,
			config.ID,
		)
	}

	if !report.Execution.StartedAt.Equal(startedAt) {
		t.Fatalf(
			"unexpected start time: got %s, want %s",
			report.Execution.StartedAt,
			startedAt,
		)
	}

	if !report.Execution.EndedAt.Equal(endedAt) {
		t.Fatalf(
			"unexpected end time: got %s, want %s",
			report.Execution.EndedAt,
			endedAt,
		)
	}

	if report.Execution.DurationMilliseconds != 2500 {
		t.Fatalf(
			"unexpected duration: got %f, want %f",
			report.Execution.DurationMilliseconds,
			2500.0,
		)
	}

	if report.BenchmarkBinary !=
		config.BenchmarkBinary {
		t.Fatal(
			"benchmark binary metadata was not preserved",
		)
	}

	if report.TargetService !=
		config.TargetService {
		t.Fatal(
			"target service metadata was not preserved",
		)
	}

	if report.Environment != config.Environment {
		t.Fatal(
			"environment metadata was not preserved",
		)
	}

	if report.Configuration.UpstreamAddress !=
		config.Suite.DirectAddress {
		t.Fatalf(
			"unexpected upstream address: got %q, want %q",
			report.Configuration.UpstreamAddress,
			config.Suite.DirectAddress,
		)
	}

	if report.Configuration.HomeDNSAddress !=
		config.Suite.ForwardedAddress {
		t.Fatalf(
			"unexpected HomeDNS address: got %q, want %q",
			report.Configuration.HomeDNSAddress,
			config.Suite.ForwardedAddress,
		)
	}

	if report.Configuration.HealthAddress !=
		config.HealthAddress {
		t.Fatalf(
			"unexpected health address: got %q, want %q",
			report.Configuration.HealthAddress,
			config.HealthAddress,
		)
	}

	if report.Configuration.QueryType != "A" {
		t.Fatalf(
			"unexpected query type: got %q, want %q",
			report.Configuration.QueryType,
			"A",
		)
	}

	if report.Configuration.WarmupQueries != 5 {
		t.Fatalf(
			"unexpected warmup count: got %d, want %d",
			report.Configuration.WarmupQueries,
			5,
		)
	}

	if report.Configuration.QueriesPerPath != 20 {
		t.Fatalf(
			"unexpected queries per path: got %d, want %d",
			report.Configuration.QueriesPerPath,
			20,
		)
	}

	if report.Configuration.ConcurrentWorkers != 5 {
		t.Fatalf(
			"unexpected worker count: got %d, want %d",
			report.Configuration.ConcurrentWorkers,
			5,
		)
	}

	if report.Configuration.TimeoutSeconds != 1.5 {
		t.Fatalf(
			"unexpected timeout: got %f, want %f",
			report.Configuration.TimeoutSeconds,
			1.5,
		)
	}

	if report.Configuration.ResourceIntervalMS != 1000 {
		t.Fatalf(
			"unexpected resource interval: got %d, want %d",
			report.Configuration.ResourceIntervalMS,
			1000,
		)
	}

	expectedOrder := []string{
		"udp-sequential",
		"udp-concurrent",
		"tcp-sequential",
		"tcp-concurrent",
	}

	assertStringSlicesEqual(
		t,
		report.Configuration.ScenarioOrder,
		expectedOrder,
	)

	expectedThresholds := StatusThresholds{
		DegradedFailurePercent: 1,
		FailedFailurePercent:   5,
		DegradedTimeoutPercent: 1,
		FailedTimeoutPercent:   5,
	}

	if report.Configuration.StatusThresholds !=
		expectedThresholds {
		t.Fatalf(
			"unexpected status thresholds: got %+v, want %+v",
			report.Configuration.StatusThresholds,
			expectedThresholds,
		)
	}

	if len(report.Scenarios) != 2 {
		t.Fatalf(
			"unexpected scenario count: got %d, want %d",
			len(report.Scenarios),
			2,
		)
	}

	for _, scenario := range report.Scenarios {
		if scenario.Status != StatusPassed {
			t.Fatalf(
				"unexpected scenario status for %q: got %q, want %q",
				scenario.Name,
				scenario.Status,
				StatusPassed,
			)
		}
	}

	if report.Summary.Status != StatusPassed {
		t.Fatalf(
			"unexpected summary status: got %q, want %q",
			report.Summary.Status,
			StatusPassed,
		)
	}

	if report.Report.Status != StatusPassed {
		t.Fatalf(
			"unexpected report status: got %q, want %q",
			report.Report.Status,
			StatusPassed,
		)
	}

	assertRequestCounts(
		t,
		report.Summary.Requests,
		RequestCounts{
			Attempted:  40,
			Successful: 40,
		},
	)

	assertRequestRates(
		t,
		report.Summary.Rates,
		RequestRates{
			SuccessPercent: 100,
		},
	)

	if report.Summary.Throughput.
		AttemptedQueriesPerSecond != 16 {
		t.Fatalf(
			"unexpected attempted summary QPS: got %f, want %f",
			report.Summary.Throughput.
				AttemptedQueriesPerSecond,
			16.0,
		)
	}

	if report.Summary.Throughput.
		SuccessfulQueriesPerSecond != 16 {
		t.Fatalf(
			"unexpected successful summary QPS: got %f, want %f",
			report.Summary.Throughput.
				SuccessfulQueriesPerSecond,
			16.0,
		)
	}

	assertRequestCounts(
		t,
		report.Summary.
			Reliability.
			Direct.
			Requests,
		RequestCounts{
			Attempted:  20,
			Successful: 20,
		},
	)

	assertRequestCounts(
		t,
		report.Summary.
			Reliability.
			Forwarded.
			Requests,
		RequestCounts{
			Attempted:  20,
			Successful: 20,
		},
	)

	assertStringSlicesEqual(
		t,
		report.Summary.ScenarioResults.Passed,
		[]string{
			"udp-sequential",
			"udp-concurrent",
		},
	)

	if report.Summary.WorstScenario == nil {
		t.Fatal("expected worst scenario to be populated")
	}

	if len(fakeSuite.configs) != 1 {
		t.Fatalf(
			"unexpected suite execution count: got %d, want %d",
			len(fakeSuite.configs),
			1,
		)
	}

	if fakeSuite.configs[0].DirectAddress !=
		config.Suite.DirectAddress {
		t.Fatal("suite configuration was not preserved")
	}
}

func TestReportRunnerCopiesConfigurationSlices(
	t *testing.T,
) {
	t.Parallel()

	startedAt := time.Date(
		2026,
		time.July,
		28,
		18,
		0,
		0,
		0,
		time.UTC,
	)

	fakeSuite := &recordedSuiteRunner{
		results: []ScenarioResult{
			successfulReportScenario(
				"udp-sequential",
				ProtocolUDP,
				1,
			),
		},
	}

	runner := &ReportRunner{
		suiteRunner: fakeSuite,
		now: func() time.Time {
			return startedAt
		},
		resourceCollector: NoopResourceCollector{},
	}

	config := validReportRunConfig()

	report, err := runner.Run(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf(
			"run benchmark report: %v",
			err,
		)
	}

	report.Configuration.QueryNames[0] =
		"modified.example."

	report.Configuration.ScenarioOrder[0] =
		"modified-scenario"

	if config.Suite.QueryNames[0] != "example.com." {
		t.Fatal(
			"report query names unexpectedly modify configuration",
		)
	}

	if config.Suite.Scenarios()[0].Name !=
		"udp-sequential" {
		t.Fatal(
			"report scenario order unexpectedly modifies suite configuration",
		)
	}
}

func TestReportRunnerUsesDefaultBinaryMetadata(
	t *testing.T,
) {
	t.Parallel()

	fakeSuite := &recordedSuiteRunner{
		results: []ScenarioResult{
			successfulReportScenario(
				"udp-sequential",
				ProtocolUDP,
				1,
			),
		},
	}

	runner := &ReportRunner{
		suiteRunner:       fakeSuite,
		now:               time.Now,
		resourceCollector: NoopResourceCollector{},
	}

	config := validReportRunConfig()
	config.BenchmarkBinary.Project = ""
	config.BenchmarkBinary.Component = ""

	report, err := runner.Run(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf(
			"run benchmark report: %v",
			err,
		)
	}

	if report.BenchmarkBinary.Project !=
		"homedns-analytics" {
		t.Fatalf(
			"unexpected project name: got %q",
			report.BenchmarkBinary.Project,
		)
	}

	if report.BenchmarkBinary.Component !=
		"homedns-dns" {
		t.Fatalf(
			"unexpected component name: got %q",
			report.BenchmarkBinary.Component,
		)
	}
}

func TestReportRunnerUsesTargetAddressDefaults(
	t *testing.T,
) {
	t.Parallel()

	fakeSuite := &recordedSuiteRunner{
		results: []ScenarioResult{
			successfulReportScenario(
				"udp-sequential",
				ProtocolUDP,
				1,
			),
		},
	}

	runner := &ReportRunner{
		suiteRunner:       fakeSuite,
		now:               time.Now,
		resourceCollector: NoopResourceCollector{},
	}

	config := validReportRunConfig()
	config.TargetService.Address = ""
	config.TargetService.HealthAddress = ""

	report, err := runner.Run(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf(
			"run benchmark report: %v",
			err,
		)
	}

	if report.TargetService.Address !=
		config.Suite.ForwardedAddress {
		t.Fatalf(
			"unexpected target address: got %q, want %q",
			report.TargetService.Address,
			config.Suite.ForwardedAddress,
		)
	}

	if report.TargetService.HealthAddress !=
		config.HealthAddress {
		t.Fatalf(
			"unexpected target health address: got %q, want %q",
			report.TargetService.HealthAddress,
			config.HealthAddress,
		)
	}
}

func TestReportRunnerUsesCustomThresholds(
	t *testing.T,
) {
	t.Parallel()

	fakeSuite := &recordedSuiteRunner{
		results: []ScenarioResult{
			degradedReportScenario(
				"tcp-concurrent",
			),
		},
	}

	runner := &ReportRunner{
		suiteRunner:       fakeSuite,
		now:               time.Now,
		resourceCollector: NoopResourceCollector{},
	}

	config := validReportRunConfig()
	config.StatusThresholds = StatusThresholds{
		DegradedFailurePercent: 3,
		FailedFailurePercent:   10,
		DegradedTimeoutPercent: 3,
		FailedTimeoutPercent:   10,
	}

	report, err := runner.Run(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf(
			"run benchmark report: %v",
			err,
		)
	}

	if report.Scenarios[0].Status != StatusPassed {
		t.Fatalf(
			"unexpected scenario status: got %q, want %q",
			report.Scenarios[0].Status,
			StatusPassed,
		)
	}
}

func TestReportRunnerCollectsResources(
	t *testing.T,
) {
	t.Parallel()

	startedAt := time.Date(
		2026,
		time.August,
		1,
		10,
		0,
		0,
		0,
		time.UTC,
	)

	endedAt := startedAt.Add(
		2 * time.Second,
	)

	collector := &recordedResourceCollector{
		result: ResourceCollectionResult{
			StartedAt: startedAt,
			EndedAt:   endedAt,

			BenchmarkPIDBefore: 100,
			BenchmarkPIDAfter:  100,
			TargetPIDBefore:    200,
			TargetPIDAfter:     200,

			Samples: []ResourceSample{
				{
					BenchmarkProcess: ProcessResourceSample{
						PID:        100,
						CPUPercent: float64Pointer(2),
						RSSBytes:   uint64Pointer(10_000),
						Threads:    uint64Pointer(4),
					},
					TargetService: ProcessResourceSample{
						PID:        200,
						CPUPercent: float64Pointer(3),
						RSSBytes:   uint64Pointer(20_000),
						Threads:    uint64Pointer(7),
					},
					System: SystemResourceSample{
						LoadAverageOneMinute: float64Pointer(0.25),
						MemoryAvailableBytes: uint64Pointer(300_000),
						TemperatureCelsius:   float64Pointer(52),
						ThrottledState:       stringPointer("throttled=0x0"),
					},
				},
			},
			Errors: make([]string, 0),
		},
	}

	fakeSuite := &recordedSuiteRunner{
		results: []ScenarioResult{
			successfulReportScenario(
				"udp-sequential",
				ProtocolUDP,
				1,
			),
		},
	}

	runner := &ReportRunner{
		suiteRunner:       fakeSuite,
		resourceCollector: collector,
		now:               time.Now,
	}

	report, err := runner.Run(
		context.Background(),
		validReportRunConfig(),
	)
	if err != nil {
		t.Fatalf(
			"run benchmark report: %v",
			err,
		)
	}

	if !collector.called {
		t.Fatal(
			"resource collector was not called",
		)
	}

	if report.Resources.Sampling.SamplesCollected != 1 {
		t.Fatalf(
			"unexpected sample count: got %d, want 1",
			report.Resources.Sampling.SamplesCollected,
		)
	}

	if report.Resources.BenchmarkProcess.CPUPercent.Mean == nil ||
		*report.Resources.BenchmarkProcess.CPUPercent.Mean != 2 {
		t.Fatalf(
			"unexpected benchmark CPU mean: %v",
			report.Resources.BenchmarkProcess.CPUPercent.Mean,
		)
	}

	if report.Resources.TargetService.RSSBytes.Peak == nil ||
		*report.Resources.TargetService.RSSBytes.Peak != 20_000 {
		t.Fatalf(
			"unexpected target RSS peak: %v",
			report.Resources.TargetService.RSSBytes.Peak,
		)
	}

	if report.Summary.Stability.ServiceRestartDetected == nil ||
		*report.Summary.Stability.ServiceRestartDetected {
		t.Fatal(
			"unexpected service restart detection",
		)
	}

	if report.Summary.Stability.ThermalThrottlingDetected == nil ||
		*report.Summary.Stability.ThermalThrottlingDetected {
		t.Fatal(
			"unexpected thermal throttling detection",
		)
	}
}

func TestReportRunnerPassesResourceCollectionConfig(
	t *testing.T,
) {
	t.Parallel()

	collector := &recordedResourceCollector{
		result: ResourceCollectionResult{
			Samples: make([]ResourceSample, 0),
			Errors:  make([]string, 0),
		},
	}

	config := validReportRunConfig()

	runner := &ReportRunner{
		suiteRunner: &recordedSuiteRunner{
			results: []ScenarioResult{
				successfulReportScenario(
					"udp-sequential",
					ProtocolUDP,
					1,
				),
			},
		},
		resourceCollector: collector,
		now:               time.Now,
	}

	_, err := runner.Run(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf(
			"run benchmark report: %v",
			err,
		)
	}

	if collector.config !=
		*config.ResourceCollection {
		t.Fatalf(
			"unexpected resource collection config: got %+v, want %+v",
			collector.config,
			*config.ResourceCollection,
		)
	}
}

func TestReportRunnerReturnsResourceCollectorError(
	t *testing.T,
) {
	t.Parallel()

	expectedErr := errors.New(
		"collector failed",
	)

	runner := &ReportRunner{
		suiteRunner: &recordedSuiteRunner{
			results: []ScenarioResult{
				successfulReportScenario(
					"udp-sequential",
					ProtocolUDP,
					1,
				),
			},
		},
		resourceCollector: &recordedResourceCollector{
			err: expectedErr,
		},
		now: time.Now,
	}

	_, err := runner.Run(
		context.Background(),
		validReportRunConfig(),
	)
	if err == nil {
		t.Fatal(
			"expected resource collection to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"collect benchmark resources",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}

	if !errors.Is(err, expectedErr) {
		t.Fatalf(
			"resource collector error was not preserved: %v",
			err,
		)
	}
}

func TestReportRunnerRejectsMissingResourceCollector(
	t *testing.T,
) {
	t.Parallel()

	runner := &ReportRunner{
		suiteRunner: &recordedSuiteRunner{},
		now:         time.Now,
	}

	_, err := runner.Run(
		context.Background(),
		validReportRunConfig(),
	)
	if err == nil {
		t.Fatal(
			"expected missing resource collector to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"benchmark resource collector is not initialized",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestReportRunnerReturnsSuiteError(
	t *testing.T,
) {
	t.Parallel()

	fakeSuite := &recordedSuiteRunner{
		err: errors.New("suite failed"),
	}

	runner := &ReportRunner{
		suiteRunner:       fakeSuite,
		now:               time.Now,
		resourceCollector: NoopResourceCollector{},
	}

	_, err := runner.Run(
		context.Background(),
		validReportRunConfig(),
	)
	if err == nil {
		t.Fatal(
			"expected report runner to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"run benchmark suite",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestReportRunnerRejectsInvalidConfiguration(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*ReportRunConfig)
	}{
		{
			name: "missing benchmark ID",
			mutate: func(config *ReportRunConfig) {
				config.ID = ""
			},
		},
		{
			name: "invalid suite configuration",
			mutate: func(config *ReportRunConfig) {
				config.Suite.DirectAddress = ""
			},
		},
		{
			name: "negative degraded failure threshold",
			mutate: func(config *ReportRunConfig) {
				config.StatusThresholds =
					StatusThresholds{
						DegradedFailurePercent: -1,
						FailedFailurePercent:   5,
						DegradedTimeoutPercent: 1,
						FailedTimeoutPercent:   5,
					}
			},
		},
		{
			name: "degraded failure exceeds failed threshold",
			mutate: func(config *ReportRunConfig) {
				config.StatusThresholds =
					StatusThresholds{
						DegradedFailurePercent: 10,
						FailedFailurePercent:   5,
						DegradedTimeoutPercent: 1,
						FailedTimeoutPercent:   5,
					}
			},
		},
		{
			name: "degraded timeout exceeds failed threshold",
			mutate: func(config *ReportRunConfig) {
				config.StatusThresholds =
					StatusThresholds{
						DegradedFailurePercent: 1,
						FailedFailurePercent:   5,
						DegradedTimeoutPercent: 10,
						FailedTimeoutPercent:   5,
					}
			},
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			config := validReportRunConfig()
			test.mutate(&config)

			runner := &ReportRunner{
				suiteRunner:       &recordedSuiteRunner{},
				now:               time.Now,
				resourceCollector: NoopResourceCollector{},
			}

			_, err := runner.Run(
				context.Background(),
				config,
			)
			if err == nil {
				t.Fatal(
					"expected invalid configuration to fail",
				)
			}
		})
	}
}

func TestReportRunnerRejectsNilReceiver(
	t *testing.T,
) {
	t.Parallel()

	var runner *ReportRunner

	_, err := runner.Run(
		context.Background(),
		validReportRunConfig(),
	)
	if err == nil {
		t.Fatal(
			"expected nil report runner to fail",
		)
	}
}

func TestReportRunnerRejectsMissingSuiteRunner(
	t *testing.T,
) {
	t.Parallel()

	runner := &ReportRunner{
		now:               time.Now,
		resourceCollector: NoopResourceCollector{},
	}

	_, err := runner.Run(
		context.Background(),
		validReportRunConfig(),
	)
	if err == nil {
		t.Fatal(
			"expected missing suite runner to fail",
		)
	}
}

func TestReportRunnerRejectsMissingClock(
	t *testing.T,
) {
	t.Parallel()

	runner := &ReportRunner{
		suiteRunner:       &recordedSuiteRunner{},
		resourceCollector: NoopResourceCollector{},
	}

	_, err := runner.Run(
		context.Background(),
		validReportRunConfig(),
	)
	if err == nil {
		t.Fatal(
			"expected missing report clock to fail",
		)
	}
}

func TestReportRunnerRejectsNilContext(
	t *testing.T,
) {
	t.Parallel()

	runner := &ReportRunner{
		suiteRunner:       &recordedSuiteRunner{},
		now:               time.Now,
		resourceCollector: NoopResourceCollector{},
	}

	_, err := runner.Run(
		nil,
		validReportRunConfig(),
	)
	if err == nil {
		t.Fatal(
			"expected nil benchmark context to fail",
		)
	}
}

func TestClassifyScenarioPassed(t *testing.T) {
	t.Parallel()

	scenario := successfulReportScenario(
		"udp-sequential",
		ProtocolUDP,
		1,
	)

	classifyScenario(
		&scenario,
		resolvedStatusThresholds(
			StatusThresholds{},
		),
	)

	if scenario.Status != StatusPassed {
		t.Fatalf(
			"unexpected status: got %q, want %q",
			scenario.Status,
			StatusPassed,
		)
	}

	if len(scenario.StatusReasons) != 0 {
		t.Fatalf(
			"unexpected status reasons: %v",
			scenario.StatusReasons,
		)
	}
}

func TestClassifyScenarioDegraded(t *testing.T) {
	t.Parallel()

	scenario := degradedReportScenario(
		"tcp-concurrent",
	)

	classifyScenario(
		&scenario,
		resolvedStatusThresholds(
			StatusThresholds{},
		),
	)

	if scenario.Status != StatusDegraded {
		t.Fatalf(
			"unexpected status: got %q, want %q",
			scenario.Status,
			StatusDegraded,
		)
	}

	if len(scenario.StatusReasons) == 1 {
		t.Fatalf(
			"unexpected status reason count: got %d, want %d",
			len(scenario.StatusReasons),
			1,
		)
	}

	reason := scenario.StatusReasons[0]

	if reason.Code !=
		"QUERY_FAILURE_RATE_EXCEEDED" {
		t.Fatalf(
			"unexpected reason code: got %q",
			reason.Code,
		)
	}

	if reason.Path != "forwarded" {
		t.Fatalf(
			"unexpected reason path: got %q",
			reason.Path,
		)
	}

	if reason.Observed == nil ||
		*reason.Observed != 2 {
		t.Fatalf(
			"unexpected observed rate: %v",
			reason.Observed,
		)
	}

	if reason.Threshold == nil ||
		*reason.Threshold != 1 {
		t.Fatalf(
			"unexpected threshold: %v",
			reason.Threshold,
		)
	}
}

func TestClassifyScenarioFailed(t *testing.T) {
	t.Parallel()

	scenario := failedReportScenario(
		"tcp-concurrent",
	)

	classifyScenario(
		&scenario,
		resolvedStatusThresholds(
			StatusThresholds{},
		),
	)

	if scenario.Status != StatusFailed {
		t.Fatalf(
			"unexpected status: got %q, want %q",
			scenario.Status,
			StatusFailed,
		)
	}

	if len(scenario.StatusReasons) != 2 {
		t.Fatalf(
			"unexpected status reason count: got %d, want %d",
			len(scenario.StatusReasons),
			2,
		)
	}
}

func TestClassifyScenarioInvalidWithoutRequests(
	t *testing.T,
) {
	t.Parallel()

	scenario := ScenarioResult{
		Name: "udp-sequential",
	}

	classifyScenario(
		&scenario,
		resolvedStatusThresholds(
			StatusThresholds{},
		),
	)

	if scenario.Status != StatusInvalid {
		t.Fatalf(
			"unexpected status: got %q, want %q",
			scenario.Status,
			StatusInvalid,
		)
	}

	if len(scenario.StatusReasons) != 2 {
		t.Fatalf(
			"unexpected status reason count: got %d, want %d",
			len(scenario.StatusReasons),
			2,
		)
	}
}

func TestSummarizeScenariosPassed(t *testing.T) {
	t.Parallel()

	scenarios := classifyScenarios(
		[]ScenarioResult{
			successfulReportScenario(
				"udp-sequential",
				ProtocolUDP,
				1,
			),
			successfulReportScenario(
				"tcp-sequential",
				ProtocolTCP,
				1,
			),
		},
		resolvedStatusThresholds(
			StatusThresholds{},
		),
	)

	summary := summarizeScenarios(
		scenarios,
		2_000,
	)

	if summary.Status != StatusPassed {
		t.Fatalf(
			"unexpected status: got %q, want %q",
			summary.Status,
			StatusPassed,
		)
	}

	assertRequestCounts(
		t,
		summary.Requests,
		RequestCounts{
			Attempted:  40,
			Successful: 40,
		},
	)

	assertRequestRates(
		t,
		summary.Rates,
		RequestRates{
			SuccessPercent: 100,
		},
	)

	if summary.Throughput.
		AttemptedQueriesPerSecond != 20 {
		t.Fatalf(
			"unexpected attempted QPS: got %f, want %f",
			summary.Throughput.
				AttemptedQueriesPerSecond,
			20.0,
		)
	}

	if len(summary.Notes) != 0 {
		t.Fatalf(
			"unexpected notes: %v",
			summary.Notes,
		)
	}
}

func TestSummarizeScenariosDegraded(t *testing.T) {
	t.Parallel()

	scenarios := classifyScenarios(
		[]ScenarioResult{
			degradedReportScenario(
				"tcp-concurrent",
			),
		},
		resolvedStatusThresholds(
			StatusThresholds{},
		),
	)

	summary := summarizeScenarios(
		scenarios,
		10_000,
	)

	if summary.Status != StatusDegraded {
		t.Fatalf(
			"unexpected status: got %q, want %q",
			summary.Status,
			StatusDegraded,
		)
	}

	assertRequestCounts(
		t,
		summary.Requests,
		RequestCounts{
			Attempted:          110,
			Successful:         108,
			Failed:             2,
			Timeouts:           1,
			NonTimeoutFailures: 1,
		},
	)

	assertStringSlicesEqual(
		t,
		summary.ScenarioResults.Degraded,
		[]string{"tcp-concurrent"},
	)

	if len(summary.Notes) != 2 {
		t.Fatalf(
			"unexpected notes: %v",
			summary.Notes,
		)
	}

	if summary.WorstScenario == nil {
		t.Fatal("expected worst scenario")
	}

	if summary.WorstScenario.Name !=
		"tcp-concurrent" {
		t.Fatalf(
			"unexpected worst scenario: got %q",
			summary.WorstScenario.Name,
		)
	}

	if summary.WorstScenario.Path !=
		"forwarded" {
		t.Fatalf(
			"unexpected worst path: got %q",
			summary.WorstScenario.Path,
		)
	}
}

func TestSummarizeScenariosInvalidWhenEmpty(
	t *testing.T,
) {
	t.Parallel()

	summary := summarizeScenarios(nil, 0)

	if summary.Status != StatusInvalid {
		t.Fatalf(
			"unexpected status: got %q, want %q",
			summary.Status,
			StatusInvalid,
		)
	}

	if len(summary.StatusReasons) != 1 {
		t.Fatalf(
			"unexpected reason count: got %d, want %d",
			len(summary.StatusReasons),
			1,
		)
	}

	if summary.StatusReasons[0].Code !=
		"NO_SCENARIOS_EXECUTED" {
		t.Fatalf(
			"unexpected reason code: got %q",
			summary.StatusReasons[0].Code,
		)
	}
}

func TestQueryTypeName(t *testing.T) {
	t.Parallel()

	if actual := queryTypeName(dns.TypeA); actual != "A" {
		t.Fatalf(
			"unexpected A query type: got %q",
			actual,
		)
	}

	if actual := queryTypeName(65000); actual !=
		"TYPE65000" {
		t.Fatalf(
			"unexpected unknown query type: got %q",
			actual,
		)
	}
}

func TestHigherStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		current   string
		candidate string
		expected  string
	}{
		{
			current:   StatusUnknown,
			candidate: StatusPassed,
			expected:  StatusPassed,
		},
		{
			current:   StatusPassed,
			candidate: StatusDegraded,
			expected:  StatusDegraded,
		},
		{
			current:   StatusFailed,
			candidate: StatusDegraded,
			expected:  StatusFailed,
		},
		{
			current:   StatusFailed,
			candidate: StatusInvalid,
			expected:  StatusInvalid,
		},
	}

	for _, test := range tests {
		actual := higherStatus(
			test.current,
			test.candidate,
		)

		if actual != test.expected {
			t.Fatalf(
				"unexpected higher status: got %q, want %q",
				actual,
				test.expected,
			)
		}
	}
}

func assertStringSlicesEqual(
	t *testing.T,
	actual []string,
	expected []string,
) {
	t.Helper()

	if len(actual) != len(expected) {
		t.Fatalf(
			"unexpected slice length: got %d, want %d; actual=%v expected=%v",
			len(actual),
			len(expected),
			actual,
			expected,
		)
	}

	for index := range expected {
		if actual[index] != expected[index] {
			t.Fatalf(
				"unexpected value at index %d: got %q, want %q",
				index,
				actual[index],
				expected[index],
			)
		}
	}
}

func TestReportRunnerForwardsProgressReporter(
	t *testing.T,
) {
	t.Parallel()

	reporterCalled := false

	reporter := ProgressReporterFunc(
		func(event ProgressEvent) {
			reporterCalled = true

			if event.Type !=
				ProgressEventBenchmarkStarted {
				t.Fatalf(
					"unexpected progress event type: got %q, want %q",
					event.Type,
					ProgressEventBenchmarkStarted,
				)
			}
		},
	)

	fakeSuite := &recordedSuiteRunner{
		results: []ScenarioResult{
			successfulReportScenario(
				"udp-sequential",
				ProtocolUDP,
				1,
			),
		},
	}

	times := []time.Time{
		time.Date(
			2026,
			time.August,
			1,
			10,
			0,
			0,
			0,
			time.UTC,
		),
		time.Date(
			2026,
			time.August,
			1,
			10,
			0,
			1,
			0,
			time.UTC,
		),
	}

	clockIndex := 0

	runner := &ReportRunner{
		suiteRunner: fakeSuite,

		now: func() time.Time {
			value := times[clockIndex]
			clockIndex++

			return value
		},
		resourceCollector: NoopResourceCollector{},
	}

	config := validReportRunConfig()

	config.Suite.ProgressReporter =
		reporter

	_, err := runner.Run(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf(
			"run benchmark report: %v",
			err,
		)
	}

	if len(fakeSuite.configs) != 1 {
		t.Fatalf(
			"unexpected suite execution count: got %d, want 1",
			len(fakeSuite.configs),
		)
	}

	forwardedReporter :=
		fakeSuite.configs[0].
			ProgressReporter

	if forwardedReporter == nil {
		t.Fatal(
			"progress reporter was not forwarded to the benchmark suite",
		)
	}

	forwardedReporter.ReportProgress(
		ProgressEvent{
			Type: ProgressEventBenchmarkStarted,
		},
	)

	if !reporterCalled {
		t.Fatal(
			"forwarded progress reporter was not called",
		)
	}
}

func TestReportRunnerWorksWithoutProgressReporter(
	t *testing.T,
) {
	t.Parallel()

	fakeSuite := &recordedSuiteRunner{
		results: []ScenarioResult{
			successfulReportScenario(
				"udp-sequential",
				ProtocolUDP,
				1,
			),
		},
	}

	times := []time.Time{
		time.Date(
			2026,
			time.August,
			1,
			10,
			0,
			0,
			0,
			time.UTC,
		),
		time.Date(
			2026,
			time.August,
			1,
			10,
			0,
			1,
			0,
			time.UTC,
		),
	}

	clockIndex := 0

	runner := &ReportRunner{
		suiteRunner: fakeSuite,

		now: func() time.Time {
			value := times[clockIndex]
			clockIndex++

			return value
		},
		resourceCollector: NoopResourceCollector{},
	}

	config := validReportRunConfig()
	config.Suite.ProgressReporter = nil

	report, err := runner.Run(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf(
			"run benchmark report: %v",
			err,
		)
	}

	if report.Report.ID != config.ID {
		t.Fatalf(
			"unexpected report ID: got %q, want %q",
			report.Report.ID,
			config.ID,
		)
	}

	if len(fakeSuite.configs) != 1 {
		t.Fatalf(
			"unexpected suite execution count: got %d, want 1",
			len(fakeSuite.configs),
		)
	}
}

func TestReportRunnerCopiesWorkloadConfiguration(
	t *testing.T,
) {
	t.Parallel()

	fakeSuite := &recordedSuiteRunner{
		results: []ScenarioResult{
			successfulReportScenario(
				"udp-sequential",
				ProtocolUDP,
				1,
			),
		},
	}

	startedAt := time.Date(
		2026,
		time.August,
		1,
		10,
		0,
		0,
		0,
		time.UTC,
	)

	endedAt := startedAt.Add(
		time.Second,
	)

	times := []time.Time{
		startedAt,
		endedAt,
	}

	clockIndex := 0

	runner := &ReportRunner{
		suiteRunner: fakeSuite,

		now: func() time.Time {
			value := times[clockIndex]
			clockIndex++

			return value
		},
		resourceCollector: NoopResourceCollector{},
	}

	config := validReportRunConfig()

	report, err := runner.Run(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf(
			"run benchmark report: %v",
			err,
		)
	}

	if report.Configuration.UpstreamAddress !=
		config.Suite.DirectAddress {
		t.Fatalf(
			"unexpected upstream address: got %q, want %q",
			report.Configuration.UpstreamAddress,
			config.Suite.DirectAddress,
		)
	}

	if report.Configuration.HomeDNSAddress !=
		config.Suite.ForwardedAddress {
		t.Fatalf(
			"unexpected HomeDNS address: got %q, want %q",
			report.Configuration.HomeDNSAddress,
			config.Suite.ForwardedAddress,
		)
	}

	if report.Configuration.HealthAddress !=
		config.HealthAddress {
		t.Fatalf(
			"unexpected health address: got %q, want %q",
			report.Configuration.HealthAddress,
			config.HealthAddress,
		)
	}

	if report.Configuration.QueryType != "A" {
		t.Fatalf(
			"unexpected query type: got %q, want %q",
			report.Configuration.QueryType,
			"A",
		)
	}

	if report.Configuration.WarmupQueries !=
		config.Suite.WarmupQueries {
		t.Fatalf(
			"unexpected warmup query count: got %d, want %d",
			report.Configuration.WarmupQueries,
			config.Suite.WarmupQueries,
		)
	}

	if report.Configuration.QueriesPerPath !=
		config.Suite.QueryCount {
		t.Fatalf(
			"unexpected queries-per-path value: got %d, want %d",
			report.Configuration.QueriesPerPath,
			config.Suite.QueryCount,
		)
	}

	if report.Configuration.ConcurrentWorkers !=
		config.Suite.ConcurrentWorkers {
		t.Fatalf(
			"unexpected worker count: got %d, want %d",
			report.Configuration.ConcurrentWorkers,
			config.Suite.ConcurrentWorkers,
		)
	}

	if report.Configuration.TimeoutSeconds !=
		config.Suite.Timeout.Seconds() {
		t.Fatalf(
			"unexpected timeout: got %f, want %f",
			report.Configuration.TimeoutSeconds,
			config.Suite.Timeout.Seconds(),
		)
	}

	if len(report.Configuration.QueryNames) !=
		len(config.Suite.QueryNames) {
		t.Fatalf(
			"unexpected query-name count: got %d, want %d",
			len(report.Configuration.QueryNames),
			len(config.Suite.QueryNames),
		)
	}

	for index := range config.Suite.QueryNames {
		if report.Configuration.QueryNames[index] !=
			config.Suite.QueryNames[index] {
			t.Fatalf(
				"unexpected query name at index %d: got %q, want %q",
				index,
				report.Configuration.QueryNames[index],
				config.Suite.QueryNames[index],
			)
		}
	}
}

func TestReportRunnerReportQueryNamesDoNotShareSuiteStorage(
	t *testing.T,
) {
	t.Parallel()

	fakeSuite := &recordedSuiteRunner{
		results: []ScenarioResult{
			successfulReportScenario(
				"udp-sequential",
				ProtocolUDP,
				1,
			),
		},
	}

	times := []time.Time{
		time.Date(
			2026,
			time.August,
			1,
			10,
			0,
			0,
			0,
			time.UTC,
		),
		time.Date(
			2026,
			time.August,
			1,
			10,
			0,
			1,
			0,
			time.UTC,
		),
	}

	clockIndex := 0

	runner := &ReportRunner{
		suiteRunner: fakeSuite,

		now: func() time.Time {
			value := times[clockIndex]
			clockIndex++

			return value
		},
		resourceCollector: NoopResourceCollector{},
	}

	config := validReportRunConfig()

	report, err := runner.Run(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf(
			"run benchmark report: %v",
			err,
		)
	}

	report.Configuration.QueryNames[0] =
		"modified.example."

	if config.Suite.QueryNames[0] !=
		"example.com." {
		t.Fatalf(
			"report query names share suite storage: got %q",
			config.Suite.QueryNames[0],
		)
	}
}
