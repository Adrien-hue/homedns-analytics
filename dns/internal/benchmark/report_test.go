package benchmark

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewReport(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(
		2026,
		time.July,
		28,
		20,
		30,
		0,
		0,
		time.FixedZone("CEST", 2*60*60),
	)

	report := NewReport(
		"benchmark-20260728-183000",
		startedAt,
	)

	if report.SchemaVersion != SchemaVersion {
		t.Fatalf(
			"unexpected schema version: got %q, want %q",
			report.SchemaVersion,
			SchemaVersion,
		)
	}

	if report.SchemaVersion != "2.0" {
		t.Fatalf(
			"unexpected schema version value: got %q, want %q",
			report.SchemaVersion,
			"2.0",
		)
	}

	if report.Report.ID != "benchmark-20260728-183000" {
		t.Fatalf(
			"unexpected report ID: got %q",
			report.Report.ID,
		)
	}

	if report.Report.Status != StatusUnknown {
		t.Fatalf(
			"unexpected report status: got %q, want %q",
			report.Report.Status,
			StatusUnknown,
		)
	}

	if report.Report.CreatedAt.Location() != time.UTC {
		t.Fatal("expected report creation time in UTC")
	}

	expectedStartedAt := startedAt.UTC()

	if !report.Report.CreatedAt.Equal(expectedStartedAt) {
		t.Fatalf(
			"unexpected report creation time: got %s, want %s",
			report.Report.CreatedAt,
			expectedStartedAt,
		)
	}

	if report.Execution.StartedAt.Location() != time.UTC {
		t.Fatal("expected execution start time in UTC")
	}

	if !report.Execution.StartedAt.Equal(expectedStartedAt) {
		t.Fatalf(
			"unexpected execution start time: got %s, want %s",
			report.Execution.StartedAt,
			expectedStartedAt,
		)
	}

	if report.BenchmarkBinary.Project != "homedns-analytics" {
		t.Fatalf(
			"unexpected benchmark project: got %q",
			report.BenchmarkBinary.Project,
		)
	}

	if report.BenchmarkBinary.Component != "homedns-dns" {
		t.Fatalf(
			"unexpected benchmark component: got %q",
			report.BenchmarkBinary.Component,
		)
	}

	if report.Scenarios == nil {
		t.Fatal("expected scenarios to be initialized")
	}

	if report.Report.StatusReasons == nil {
		t.Fatal("expected report status reasons to be initialized")
	}

	if report.Report.Notes == nil {
		t.Fatal("expected report notes to be initialized")
	}

	if report.Configuration.QueryNames == nil {
		t.Fatal("expected configuration query names to be initialized")
	}

	if report.Configuration.ScenarioOrder == nil {
		t.Fatal("expected scenario order to be initialized")
	}

	if report.Resources.Sampling.Errors == nil {
		t.Fatal("expected resource collection errors to be initialized")
	}

	if report.Summary.Status != StatusUnknown {
		t.Fatalf(
			"unexpected summary status: got %q, want %q",
			report.Summary.Status,
			StatusUnknown,
		)
	}

	if report.Summary.StatusReasons == nil {
		t.Fatal("expected summary status reasons to be initialized")
	}

	if report.Summary.Notes == nil {
		t.Fatal("expected summary notes to be initialized")
	}

	assertScenarioStatusSlicesInitialized(
		t,
		report.Summary.ScenarioResults,
	)
}

func TestReportComplete(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(
		2026,
		time.July,
		28,
		18,
		30,
		0,
		0,
		time.UTC,
	)

	report := NewReport("benchmark-id", startedAt)

	endedAt := startedAt.Add(1500 * time.Millisecond)
	report.Complete(endedAt)

	if !report.Execution.EndedAt.Equal(endedAt) {
		t.Fatalf(
			"unexpected end time: got %s, want %s",
			report.Execution.EndedAt,
			endedAt,
		)
	}

	if report.Execution.EndedAt.Location() != time.UTC {
		t.Fatal("expected execution end time in UTC")
	}

	if report.Execution.DurationMilliseconds != 1500 {
		t.Fatalf(
			"unexpected duration: got %f, want %f",
			report.Execution.DurationMilliseconds,
			1500.0,
		)
	}
}

func TestNilReportCompleteDoesNotPanic(t *testing.T) {
	t.Parallel()

	var report *Report

	report.Complete(time.Now())
}

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	report := NewReport(
		"benchmark-id",
		time.Date(
			2026,
			time.July,
			28,
			18,
			30,
			0,
			0,
			time.UTC,
		),
	)

	report.TargetService = TargetServiceMetadata{
		Address:       "192.168.1.32:53",
		HealthAddress: "127.0.0.1:8081",
		Version:       "v0.2.0-rc1",
		GitCommit:     "fce8f87",
	}

	report.Configuration = Configuration{
		UpstreamAddress:   "1.1.1.1:53",
		HomeDNSAddress:    "192.168.1.32:53",
		HealthAddress:     "127.0.0.1:8081",
		QueryNames:        []string{"example.com."},
		QueryType:         "A",
		WarmupQueries:     10,
		QueriesPerPath:    100,
		ConcurrentWorkers: 10,
		TimeoutSeconds:    3,
		ScenarioOrder: []string{
			"udp-sequential",
			"udp-concurrent",
			"tcp-sequential",
			"tcp-concurrent",
		},
	}

	report.Scenarios = append(
		report.Scenarios,
		ScenarioResult{
			Name:              "udp-sequential",
			Protocol:          ProtocolUDP,
			Concurrency:       1,
			QueryCountPerPath: 100,
			Status:            StatusPassed,
			StatusReasons:     make([]StatusReason, 0),

			Direct: PathResult{
				Requests: RequestCounts{
					Attempted:  100,
					Successful: 100,
				},
				Rates: RequestRates{
					SuccessPercent: 100,
				},
				DurationMilliseconds: 1000,
				Throughput: ThroughputMetrics{
					AttemptedQueriesPerSecond:  100,
					SuccessfulQueriesPerSecond: 100,
				},
				SuccessfulRequestLatencyMilliseconds: SuccessfulLatencyStatistics{
					SampleCount: 100,
					Minimum:     1,
					Mean:        2,
					Median:      2,
					P95:         3,
					P99:         4,
					Maximum:     5,
				},
			},

			Forwarded: PathResult{
				Requests: RequestCounts{
					Attempted:  100,
					Successful: 100,
				},
				Rates: RequestRates{
					SuccessPercent: 100,
				},
				DurationMilliseconds: 1200,
				Throughput: ThroughputMetrics{
					AttemptedQueriesPerSecond:  83.333333,
					SuccessfulQueriesPerSecond: 83.333333,
				},
				SuccessfulRequestLatencyMilliseconds: SuccessfulLatencyStatistics{
					SampleCount: 100,
					Minimum:     2,
					Mean:        3,
					Median:      3,
					P95:         4,
					P99:         5,
					Maximum:     6,
				},
			},
		},
	)

	var output bytes.Buffer

	if err := WriteJSON(&output, report); err != nil {
		t.Fatalf("write JSON report: %v", err)
	}

	if !strings.Contains(
		output.String(),
		`"schema_version": "2.0"`,
	) {
		t.Fatalf(
			"output does not contain schema version:\n%s",
			output.String(),
		)
	}

	if !strings.Contains(
		output.String(),
		`"benchmark_binary"`,
	) {
		t.Fatalf(
			"output does not contain benchmark binary metadata:\n%s",
			output.String(),
		)
	}

	if !strings.Contains(
		output.String(),
		`"target_service"`,
	) {
		t.Fatalf(
			"output does not contain target service metadata:\n%s",
			output.String(),
		)
	}

	if !strings.Contains(
		output.String(),
		`"attempted_queries_per_second"`,
	) {
		t.Fatalf(
			"output does not contain attempted throughput:\n%s",
			output.String(),
		)
	}

	if !strings.Contains(
		output.String(),
		`"successful_queries_per_second"`,
	) {
		t.Fatalf(
			"output does not contain successful throughput:\n%s",
			output.String(),
		)
	}

	if !strings.Contains(
		output.String(),
		`"successful_request_latency_ms"`,
	) {
		t.Fatalf(
			"output does not contain explicit successful latency field:\n%s",
			output.String(),
		)
	}

	var decoded Report

	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("decode generated report: %v", err)
	}

	if decoded.Report.ID != report.Report.ID {
		t.Fatalf(
			"unexpected decoded report ID: got %q, want %q",
			decoded.Report.ID,
			report.Report.ID,
		)
	}

	if decoded.TargetService.Address !=
		report.TargetService.Address {
		t.Fatalf(
			"unexpected decoded target address: got %q, want %q",
			decoded.TargetService.Address,
			report.TargetService.Address,
		)
	}

	if decoded.Configuration.QueriesPerPath != 100 {
		t.Fatalf(
			"unexpected decoded query count: got %d, want %d",
			decoded.Configuration.QueriesPerPath,
			100,
		)
	}

	if len(decoded.Scenarios) != 1 {
		t.Fatalf(
			"unexpected decoded scenario count: got %d, want %d",
			len(decoded.Scenarios),
			1,
		)
	}

	latency := decoded.
		Scenarios[0].
		Forwarded.
		SuccessfulRequestLatencyMilliseconds

	if latency.SampleCount != 100 {
		t.Fatalf(
			"unexpected decoded latency sample count: got %d, want %d",
			latency.SampleCount,
			100,
		)
	}

	if latency.Mean != 3 {
		t.Fatalf(
			"unexpected decoded mean latency: got %f, want %f",
			latency.Mean,
			3.0,
		)
	}

	if decoded.Configuration.WarmupQueries != 10 {
		t.Fatalf(
			"unexpected decoded warmup count: got %d, want %d",
			decoded.Configuration.WarmupQueries,
			10,
		)
	}

	if decoded.Configuration.ConcurrentWorkers != 10 {
		t.Fatalf(
			"unexpected decoded concurrency: got %d, want %d",
			decoded.Configuration.ConcurrentWorkers,
			10,
		)
	}

	if decoded.Configuration.TimeoutSeconds != 3 {
		t.Fatalf(
			"unexpected decoded timeout: got %f, want %f",
			decoded.Configuration.TimeoutSeconds,
			3.0,
		)
	}

	if decoded.Configuration.QueryType != "A" {
		t.Fatalf(
			"unexpected decoded query type: got %q, want %q",
			decoded.Configuration.QueryType,
			"A",
		)
	}

	if len(decoded.Configuration.QueryNames) != 1 {
		t.Fatalf(
			"unexpected decoded query-name count: got %d, want 1",
			len(decoded.Configuration.QueryNames),
		)
	}

	if decoded.Configuration.QueryNames[0] !=
		"example.com." {
		t.Fatalf(
			"unexpected decoded query name: got %q, want %q",
			decoded.Configuration.QueryNames[0],
			"example.com.",
		)
	}

	if len(decoded.Configuration.ScenarioOrder) != 4 {
		t.Fatalf(
			"unexpected decoded scenario-order count: got %d, want 4",
			len(decoded.Configuration.ScenarioOrder),
		)
	}
}

func TestWriteJSONDoesNotEscapeHTML(t *testing.T) {
	t.Parallel()

	report := NewReport(
		"benchmark-id",
		time.Date(
			2026,
			time.July,
			28,
			18,
			30,
			0,
			0,
			time.UTC,
		),
	)

	report.Report.Notes = append(
		report.Report.Notes,
		"direct < forwarded && target > baseline",
	)

	var output bytes.Buffer

	if err := WriteJSON(&output, report); err != nil {
		t.Fatalf("write JSON report: %v", err)
	}

	if strings.Contains(output.String(), `\u003c`) ||
		strings.Contains(output.String(), `\u003e`) ||
		strings.Contains(output.String(), `\u0026`) {
		t.Fatalf(
			"expected HTML characters not to be escaped:\n%s",
			output.String(),
		)
	}
}

func TestWriteJSONRejectsNilWriter(t *testing.T) {
	t.Parallel()

	err := WriteJSON(nil, Report{})
	if err == nil {
		t.Fatal("expected nil writer to fail")
	}
}

func TestWriteJSONFile(t *testing.T) {
	t.Parallel()

	report := NewReport(
		"benchmark-id",
		time.Date(
			2026,
			time.July,
			28,
			18,
			30,
			0,
			0,
			time.UTC,
		),
	)

	report.Report.Status = StatusPassed
	report.Summary.Status = StatusPassed

	path := filepath.Join(
		t.TempDir(),
		"nested",
		"benchmark.json",
	)

	if err := WriteJSONFile(path, report); err != nil {
		t.Fatalf("write JSON report file: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read JSON report file: %v", err)
	}

	var decoded Report

	if err := json.Unmarshal(content, &decoded); err != nil {
		t.Fatalf("decode JSON report file: %v", err)
	}

	if decoded.SchemaVersion != SchemaVersion {
		t.Fatalf(
			"unexpected schema version: got %q, want %q",
			decoded.SchemaVersion,
			SchemaVersion,
		)
	}

	if decoded.Report.ID != report.Report.ID {
		t.Fatalf(
			"unexpected report ID: got %q, want %q",
			decoded.Report.ID,
			report.Report.ID,
		)
	}

	if decoded.Report.Status != StatusPassed {
		t.Fatalf(
			"unexpected report status: got %q, want %q",
			decoded.Report.Status,
			StatusPassed,
		)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat JSON report file: %v", err)
	}

	if !fileInfo.Mode().IsRegular() {
		t.Fatalf(
			"expected regular report file, got mode %s",
			fileInfo.Mode(),
		)
	}
}

func TestWriteJSONFileReplacesExistingFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(
		t.TempDir(),
		"benchmark.json",
	)

	if err := os.WriteFile(
		path,
		[]byte(`{"schema_version":"old"}`),
		0o600,
	); err != nil {
		t.Fatalf("create existing report file: %v", err)
	}

	report := NewReport(
		"replacement-report",
		time.Date(
			2026,
			time.July,
			28,
			18,
			30,
			0,
			0,
			time.UTC,
		),
	)

	if err := WriteJSONFile(path, report); err != nil {
		t.Fatalf("replace JSON report file: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read replaced JSON report file: %v", err)
	}

	var decoded Report

	if err := json.Unmarshal(content, &decoded); err != nil {
		t.Fatalf("decode replaced JSON report file: %v", err)
	}

	if decoded.Report.ID != "replacement-report" {
		t.Fatalf(
			"unexpected replacement report ID: got %q",
			decoded.Report.ID,
		)
	}
}

func TestWriteJSONFileRejectsEmptyPath(t *testing.T) {
	t.Parallel()

	err := WriteJSONFile("", Report{})
	if err == nil {
		t.Fatal("expected empty path to fail")
	}
}

func assertScenarioStatusSlicesInitialized(
	t *testing.T,
	summary ScenarioStatusSummary,
) {
	t.Helper()

	if summary.Passed == nil {
		t.Fatal("expected passed scenarios to be initialized")
	}

	if summary.Degraded == nil {
		t.Fatal("expected degraded scenarios to be initialized")
	}

	if summary.Failed == nil {
		t.Fatal("expected failed scenarios to be initialized")
	}

	if summary.Invalid == nil {
		t.Fatal("expected invalid scenarios to be initialized")
	}
}

func TestWriteJSONPreservesBenchmarkProfile(
	t *testing.T,
) {
	t.Parallel()

	report := NewReport(
		"benchmark-profile",
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
	)

	report.Configuration.Profile =
		"validation"

	report.Configuration.ProfileCustomized =
		true

	report.Configuration.QueriesPerPath =
		250

	report.Configuration.WarmupQueries =
		5

	report.Configuration.ConcurrentWorkers =
		4

	var output bytes.Buffer

	if err := WriteJSON(
		&output,
		report,
	); err != nil {
		t.Fatalf(
			"write JSON report: %v",
			err,
		)
	}

	var decoded Report

	if err := json.Unmarshal(
		output.Bytes(),
		&decoded,
	); err != nil {
		t.Fatalf(
			"decode generated report: %v",
			err,
		)
	}

	if decoded.Configuration.Profile !=
		"validation" {
		t.Fatalf(
			"unexpected decoded profile: got %q, want %q",
			decoded.Configuration.Profile,
			"validation",
		)
	}

	if !decoded.
		Configuration.
		ProfileCustomized {
		t.Fatal(
			"expected decoded profile to be customized",
		)
	}

	if decoded.Configuration.QueriesPerPath !=
		250 {
		t.Fatalf(
			"unexpected query count: got %d, want 250",
			decoded.Configuration.QueriesPerPath,
		)
	}

	if decoded.Configuration.WarmupQueries !=
		5 {
		t.Fatalf(
			"unexpected warmup count: got %d, want 5",
			decoded.Configuration.WarmupQueries,
		)
	}

	if decoded.
		Configuration.
		ConcurrentWorkers != 4 {
		t.Fatalf(
			"unexpected concurrency: got %d, want 4",
			decoded.
				Configuration.
				ConcurrentWorkers,
		)
	}
}

func TestWriteJSONPreservesPathPerformanceMetrics(
	t *testing.T,
) {
	t.Parallel()

	report := NewReport(
		"benchmark-performance",
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
	)

	report.Scenarios = append(
		report.Scenarios,
		ScenarioResult{
			Name: "tcp-concurrent",

			Protocol: ProtocolTCP,

			Concurrency: 10,

			QueryCountPerPath: 100,

			Status: StatusDegraded,

			StatusReasons: make([]StatusReason, 0),

			Forwarded: PathResult{
				Requests: RequestCounts{
					Attempted:  100,
					Successful: 97,
					Failed:     3,
					Timeouts:   2,

					NonTimeoutFailures: 1,
				},

				Rates: RequestRates{
					SuccessPercent: 97,
					FailurePercent: 3,
					TimeoutPercent: 2,
				},

				DurationMilliseconds: 2_000,

				Throughput: ThroughputMetrics{
					AttemptedQueriesPerSecond: 50,

					SuccessfulQueriesPerSecond: 48.5,
				},

				SuccessfulRequestLatencyMilliseconds: SuccessfulLatencyStatistics{
					SampleCount: 97,
					Minimum:     5,
					Mean:        15,
					Median:      12,
					P95:         30,
					P99:         45,
					Maximum:     60,
				},
			},
		},
	)

	var output bytes.Buffer

	if err := WriteJSON(
		&output,
		report,
	); err != nil {
		t.Fatalf(
			"write JSON report: %v",
			err,
		)
	}

	var decoded Report

	if err := json.Unmarshal(
		output.Bytes(),
		&decoded,
	); err != nil {
		t.Fatalf(
			"decode generated report: %v",
			err,
		)
	}

	if len(decoded.Scenarios) != 1 {
		t.Fatalf(
			"unexpected scenario count: got %d, want 1",
			len(decoded.Scenarios),
		)
	}

	path := decoded.Scenarios[0].Forwarded

	expectedRequests := RequestCounts{
		Attempted:          100,
		Successful:         97,
		Failed:             3,
		Timeouts:           2,
		NonTimeoutFailures: 1,
	}

	if path.Requests != expectedRequests {
		t.Fatalf(
			"unexpected request counts: got %+v, want %+v",
			path.Requests,
			expectedRequests,
		)
	}

	if path.
		Throughput.
		AttemptedQueriesPerSecond != 50 {
		t.Fatalf(
			"unexpected attempted QPS: got %f, want 50",
			path.
				Throughput.
				AttemptedQueriesPerSecond,
		)
	}

	if path.
		Throughput.
		SuccessfulQueriesPerSecond != 48.5 {
		t.Fatalf(
			"unexpected successful QPS: got %f, want 48.5",
			path.
				Throughput.
				SuccessfulQueriesPerSecond,
		)
	}

	latency :=
		path.
			SuccessfulRequestLatencyMilliseconds

	if latency.SampleCount != 97 {
		t.Fatalf(
			"unexpected latency sample count: got %d, want 97",
			latency.SampleCount,
		)
	}

	if latency.P95 != 30 {
		t.Fatalf(
			"unexpected p95 latency: got %f, want 30",
			latency.P95,
		)
	}
}

func TestWriteJSONDecodedSlicesAreIndependent(
	t *testing.T,
) {
	t.Parallel()

	report := NewReport(
		"benchmark-slices",
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
	)

	report.Configuration.QueryNames =
		[]string{
			"example.com.",
		}

	report.Configuration.ScenarioOrder =
		[]string{
			"udp-sequential",
		}

	var output bytes.Buffer

	if err := WriteJSON(
		&output,
		report,
	); err != nil {
		t.Fatalf(
			"write JSON report: %v",
			err,
		)
	}

	var decoded Report

	if err := json.Unmarshal(
		output.Bytes(),
		&decoded,
	); err != nil {
		t.Fatalf(
			"decode generated report: %v",
			err,
		)
	}

	decoded.Configuration.QueryNames[0] =
		"modified.example."

	decoded.Configuration.ScenarioOrder[0] =
		"tcp-concurrent"

	if report.Configuration.QueryNames[0] !=
		"example.com." {
		t.Fatal(
			"decoded query names share storage with original report",
		)
	}

	if report.Configuration.ScenarioOrder[0] !=
		"udp-sequential" {
		t.Fatal(
			"decoded scenario order shares storage with original report",
		)
	}
}
