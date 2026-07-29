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

func validReportRunConfig() ReportRunConfig {
	return ReportRunConfig{
		ID: "benchmark-20260728-200000",
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
		Project: ProjectMetadata{
			Name:      "homedns-analytics",
			Version:   "v0.2.0",
			GitCommit: "abc123",
			GitBranch: "feature/dns-benchmark",
			Dirty:     boolPointer(true),
		},
		Environment: Environment{
			Hostname:        "homedns",
			OperatingSystem: "linux",
			Kernel:          "6.12.0",
			Architecture:    "arm64",
			CPUModel:        "Raspberry Pi 3 Model B",
			LogicalCPUs:     4,
			GoVersion:       "go1.25.0",
		},
		HealthAddress: "127.0.0.1:8081",
	}
}

func successfulScenarioResult(
	name string,
) ScenarioResult {
	return ScenarioResult{
		Name:        name,
		Protocol:    ProtocolUDP,
		Concurrency: 1,
		QueryCount:  10,
		Direct: PathResult{
			Requests:   10,
			Successful: 10,
		},
		Forwarded: PathResult{
			Requests:   10,
			Successful: 10,
		},
	}
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
			successfulScenarioResult(
				"udp-sequential",
			),
			successfulScenarioResult(
				"udp-concurrent",
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

	if report.Benchmark.ID != config.ID {
		t.Fatalf(
			"unexpected benchmark ID: got %q, want %q",
			report.Benchmark.ID,
			config.ID,
		)
	}

	if !report.Benchmark.StartedAt.Equal(startedAt) {
		t.Fatalf(
			"unexpected start time: %v",
			report.Benchmark.StartedAt,
		)
	}

	if !report.Benchmark.EndedAt.Equal(endedAt) {
		t.Fatalf(
			"unexpected end time: %v",
			report.Benchmark.EndedAt,
		)
	}

	if report.Benchmark.DurationMilliseconds != 2500 {
		t.Fatalf(
			"unexpected duration: got %f, want %f",
			report.Benchmark.DurationMilliseconds,
			2500.0,
		)
	}

	if report.Project != config.Project {
		t.Fatal(
			"project metadata was not preserved",
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
			"unexpected upstream address: %q",
			report.Configuration.UpstreamAddress,
		)
	}

	if report.Configuration.HomeDNSAddress !=
		config.Suite.ForwardedAddress {
		t.Fatalf(
			"unexpected HomeDNS address: %q",
			report.Configuration.HomeDNSAddress,
		)
	}

	if report.Configuration.HealthAddress !=
		config.HealthAddress {
		t.Fatalf(
			"unexpected health address: %q",
			report.Configuration.HealthAddress,
		)
	}

	if report.Configuration.QueryType != "A" {
		t.Fatalf(
			"unexpected query type: %q",
			report.Configuration.QueryType,
		)
	}

	if report.Configuration.TimeoutSeconds != 1.5 {
		t.Fatalf(
			"unexpected timeout: got %f, want %f",
			report.Configuration.TimeoutSeconds,
			1.5,
		)
	}

	if len(report.Scenarios) != 2 {
		t.Fatalf(
			"unexpected scenario count: got %d, want %d",
			len(report.Scenarios),
			2,
		)
	}

	if report.Summary.Status != "success" {
		t.Fatalf(
			"unexpected summary status: %q",
			report.Summary.Status,
		)
	}

	if report.Summary.RequestsTotal != 40 {
		t.Fatalf(
			"unexpected request total: got %d, want %d",
			report.Summary.RequestsTotal,
			40,
		)
	}

	if report.Summary.SuccessfulTotal != 40 {
		t.Fatalf(
			"unexpected successful total: got %d, want %d",
			report.Summary.SuccessfulTotal,
			40,
		)
	}

	if len(fakeSuite.configs) != 1 {
		t.Fatalf(
			"unexpected suite execution count: got %d, want %d",
			len(fakeSuite.configs),
			1,
		)
	}
}

func TestReportRunnerCopiesQueryNames(t *testing.T) {
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
			successfulScenarioResult(
				"udp-sequential",
			),
		},
	}

	runner := &ReportRunner{
		suiteRunner: fakeSuite,
		now: func() time.Time {
			return startedAt
		},
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

	if config.Suite.QueryNames[0] != "example.com." {
		t.Fatal(
			"report query names unexpectedly modify configuration",
		)
	}
}

func TestReportRunnerUsesDefaultProjectName(
	t *testing.T,
) {
	t.Parallel()

	fakeSuite := &recordedSuiteRunner{
		results: []ScenarioResult{
			successfulScenarioResult(
				"udp-sequential",
			),
		},
	}

	runner := &ReportRunner{
		suiteRunner: fakeSuite,
		now:         time.Now,
	}

	config := validReportRunConfig()
	config.Project.Name = ""

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

	if report.Project.Name != "homedns-analytics" {
		t.Fatalf(
			"unexpected project name: %q",
			report.Project.Name,
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
		suiteRunner: fakeSuite,
		now:         time.Now,
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
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			config := validReportRunConfig()
			test.mutate(&config)

			runner := &ReportRunner{
				suiteRunner: &recordedSuiteRunner{},
				now:         time.Now,
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
		now: time.Now,
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
		suiteRunner: &recordedSuiteRunner{},
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

func TestSummarizeScenariosSuccess(t *testing.T) {
	t.Parallel()

	summary := summarizeScenarios(
		[]ScenarioResult{
			successfulScenarioResult(
				"udp-sequential",
			),
			successfulScenarioResult(
				"tcp-sequential",
			),
		},
	)

	if summary.Status != "success" {
		t.Fatalf(
			"unexpected status: %q",
			summary.Status,
		)
	}

	if summary.RequestsTotal != 40 {
		t.Fatalf(
			"unexpected request total: %d",
			summary.RequestsTotal,
		)
	}

	if summary.SuccessfulTotal != 40 {
		t.Fatalf(
			"unexpected successful total: %d",
			summary.SuccessfulTotal,
		)
	}

	if summary.FailedTotal != 0 {
		t.Fatalf(
			"unexpected failed total: %d",
			summary.FailedTotal,
		)
	}

	if summary.TimeoutsTotal != 0 {
		t.Fatalf(
			"unexpected timeout total: %d",
			summary.TimeoutsTotal,
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

	scenario := successfulScenarioResult(
		"udp-sequential",
	)

	scenario.Forwarded.Successful = 7
	scenario.Forwarded.Failed = 3
	scenario.Forwarded.Timeouts = 2

	summary := summarizeScenarios(
		[]ScenarioResult{scenario},
	)

	if summary.Status != "degraded" {
		t.Fatalf(
			"unexpected status: %q",
			summary.Status,
		)
	}

	if summary.RequestsTotal != 20 {
		t.Fatalf(
			"unexpected request total: %d",
			summary.RequestsTotal,
		)
	}

	if summary.SuccessfulTotal != 17 {
		t.Fatalf(
			"unexpected successful total: %d",
			summary.SuccessfulTotal,
		)
	}

	if summary.FailedTotal != 3 {
		t.Fatalf(
			"unexpected failed total: %d",
			summary.FailedTotal,
		)
	}

	if summary.TimeoutsTotal != 2 {
		t.Fatalf(
			"unexpected timeout total: %d",
			summary.TimeoutsTotal,
		)
	}

	if len(summary.Notes) != 1 {
		t.Fatalf(
			"unexpected notes: %v",
			summary.Notes,
		)
	}
}

func TestSummarizeScenariosEmpty(t *testing.T) {
	t.Parallel()

	summary := summarizeScenarios(nil)

	if summary.Status != "unknown" {
		t.Fatalf(
			"unexpected status: %q",
			summary.Status,
		)
	}

	if len(summary.Notes) != 1 {
		t.Fatalf(
			"unexpected notes: %v",
			summary.Notes,
		)
	}
}

func TestQueryTypeName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		queryType uint16
		want      string
	}{
		{
			name:      "known query type",
			queryType: dns.TypeAAAA,
			want:      "AAAA",
		},
		{
			name:      "unknown query type",
			queryType: 65000,
			want:      "TYPE65000",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := queryTypeName(
				test.queryType,
			)

			if got != test.want {
				t.Fatalf(
					"unexpected query type name: got %q, want %q",
					got,
					test.want,
				)
			}
		})
	}
}

func boolPointer(value bool) *bool {
	return &value
}
