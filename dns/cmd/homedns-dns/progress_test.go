package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Adrien-hue/homedns-analytics/dns/internal/benchmark"
)

func TestNewTerminalProgressReporterReturnsNilForNilWriter(
	t *testing.T,
) {
	t.Parallel()

	reporter := newTerminalProgressReporter(
		nil,
		100,
	)

	if reporter != nil {
		t.Fatal(
			"expected nil progress reporter for nil writer",
		)
	}
}

func TestTerminalProgressReporterSuppressesWarmupProgress(
	t *testing.T,
) {
	t.Parallel()

	var output bytes.Buffer

	reporter := newTerminalProgressReporter(
		&output,
		100,
	)

	reporter.ReportProgress(
		benchmark.ProgressEvent{
			Type: benchmark.ProgressEventPhaseStarted,

			Phase: benchmark.ProgressPhaseWarmupDirect,

			ScenarioIndex: 1,
			ScenarioCount: 4,
		},
	)

	reporter.ReportProgress(
		benchmark.ProgressEvent{
			Type: benchmark.ProgressEventPhaseProgress,

			Phase: benchmark.ProgressPhaseWarmupDirect,

			ScenarioIndex: 1,
			ScenarioCount: 4,

			Current:    1,
			Total:      5,
			Successful: 1,
		},
	)

	if output.Len() != 0 {
		t.Fatalf(
			"unexpected warmup progress output:\n%s",
			output.String(),
		)
	}
}

func TestTerminalProgressReporterPrintsCompactWarmupCompletion(
	t *testing.T,
) {
	t.Parallel()

	var output bytes.Buffer

	reporter := newTerminalProgressReporter(
		&output,
		100,
	)

	reporter.ReportProgress(
		benchmark.ProgressEvent{
			Type: benchmark.ProgressEventPhaseCompleted,

			Phase: benchmark.ProgressPhaseWarmupDirect,

			Current:    5,
			Total:      5,
			Successful: 5,
		},
	)

	assertOutputContains(
		t,
		output.String(),
		"Direct warmup",
	)

	assertOutputContains(
		t,
		output.String(),
		"5/5 successful",
	)

	if strings.Contains(
		output.String(),
		"[",
	) {
		t.Fatalf(
			"warmup completion unexpectedly contains a progress bar:\n%s",
			output.String(),
		)
	}
}

func TestTerminalProgressReporterPrintsLiveThroughput(
	t *testing.T,
) {
	t.Parallel()

	var output bytes.Buffer

	reporter := newTerminalProgressReporter(
		&output,
		100,
	)

	reporter.ReportProgress(
		benchmark.ProgressEvent{
			Type: benchmark.ProgressEventPhaseProgress,

			ScenarioName: "udp-sequential",

			ScenarioIndex: 1,
			ScenarioCount: 4,

			Phase: benchmark.ProgressPhaseDirect,

			OccurredAt: time.Date(
				2026,
				time.August,
				1,
				10,
				0,
				0,
				0,
				time.UTC,
			),

			Current:    50,
			Total:      100,
			Successful: 49,
			Failed:     1,
			Timeouts:   1,

			PhaseElapsed: 500 * time.Millisecond,

			Elapsed: 2 * time.Second,

			AttemptedQueriesPerSecond: 100,

			SuccessfulQueriesPerSecond: 98,
		},
	)

	result := output.String()

	assertOutputContains(
		t,
		result,
		"50.00%",
	)

	assertOutputContains(
		t,
		result,
		"50/100",
	)

	assertOutputContains(
		t,
		result,
		"qps=100.0/98.0",
	)

	assertOutputContains(
		t,
		result,
		"success=49 failed=1 timeouts=1",
	)

	assertOutputContains(
		t,
		result,
		"phase=500ms",
	)
}

func TestTerminalProgressReporterPrintsCompletedPathMetrics(
	t *testing.T,
) {
	t.Parallel()

	var output bytes.Buffer

	reporter := newTerminalProgressReporter(
		&output,
		100,
	)

	pathResult := benchmark.PathResult{
		Requests: benchmark.RequestCounts{
			Attempted:  100,
			Successful: 99,
			Failed:     1,
			Timeouts:   1,
		},

		Throughput: benchmark.ThroughputMetrics{
			AttemptedQueriesPerSecond:  120.5,
			SuccessfulQueriesPerSecond: 119.3,
		},

		SuccessfulRequestLatencyMilliseconds: benchmark.SuccessfulLatencyStatistics{
			SampleCount: 99,
			Mean:        8.25,
			P95:         12.5,
		},
	}

	reporter.ReportProgress(
		benchmark.ProgressEvent{
			Type: benchmark.ProgressEventPhaseCompleted,

			Phase: benchmark.ProgressPhaseForwarded,

			Total:      100,
			Successful: 99,
			Failed:     1,
			Timeouts:   1,

			PathResult: &pathResult,
		},
	)

	result := output.String()

	assertOutputContains(
		t,
		result,
		"successful=99/100",
	)

	assertOutputContains(
		t,
		result,
		"failed=1",
	)

	assertOutputContains(
		t,
		result,
		"timeouts=1",
	)

	assertOutputContains(
		t,
		result,
		"attempted=120.50",
	)

	assertOutputContains(
		t,
		result,
		"successful=119.30",
	)

	assertOutputContains(
		t,
		result,
		"mean=8.250ms",
	)

	assertOutputContains(
		t,
		result,
		"p95=12.500ms",
	)
}

func TestTerminalProgressReporterPrintsScenarioSummary(
	t *testing.T,
) {
	t.Parallel()

	var output bytes.Buffer

	reporter := newTerminalProgressReporter(
		&output,
		100,
	)

	scenarioResult := benchmark.ScenarioResult{
		Name:              "udp-sequential",
		Protocol:          benchmark.ProtocolUDP,
		Concurrency:       1,
		QueryCountPerPath: 100,

		Direct: benchmark.PathResult{
			Requests: benchmark.RequestCounts{
				Attempted:  100,
				Successful: 100,
			},

			Rates: benchmark.RequestRates{
				SuccessPercent: 100,
			},

			DurationMilliseconds: 800,

			Throughput: benchmark.ThroughputMetrics{
				SuccessfulQueriesPerSecond: 125,
			},

			SuccessfulRequestLatencyMilliseconds: benchmark.SuccessfulLatencyStatistics{
				SampleCount: 100,
				Mean:        8,
				Median:      7.5,
				P95:         10,
				P99:         12,
			},
		},

		Forwarded: benchmark.PathResult{
			Requests: benchmark.RequestCounts{
				Attempted:  100,
				Successful: 99,
				Failed:     1,
				Timeouts:   1,
			},

			Rates: benchmark.RequestRates{
				SuccessPercent: 99,
				FailurePercent: 1,
				TimeoutPercent: 1,
			},

			DurationMilliseconds: 1_000,

			Throughput: benchmark.ThroughputMetrics{
				SuccessfulQueriesPerSecond: 99,
			},

			SuccessfulRequestLatencyMilliseconds: benchmark.SuccessfulLatencyStatistics{
				SampleCount: 99,
				Mean:        9,
				Median:      8.5,
				P95:         12,
				P99:         15,
			},
		},

		Comparison: benchmark.ScenarioComparison{
			LatencyOverheadMilliseconds: benchmark.LatencyComparison{
				Mean: 1,
				P95:  2,
			},

			ThroughputChangePercent: benchmark.ThroughputComparison{
				Successful: -20.8,
			},
		},
	}

	reporter.ReportProgress(
		benchmark.ProgressEvent{
			Type: benchmark.ProgressEventScenarioCompleted,

			ScenarioName: scenarioResult.Name,

			ScenarioResult: &scenarioResult,
		},
	)

	result := output.String()

	assertOutputContains(
		t,
		result,
		"Scenario summary",
	)

	assertOutputContains(
		t,
		result,
		"Direct",
	)

	assertOutputContains(
		t,
		result,
		"Forwarded",
	)

	assertOutputContains(
		t,
		result,
		"100/100",
	)

	assertOutputContains(
		t,
		result,
		"99/100",
	)

	assertOutputContains(
		t,
		result,
		"latency overhead mean=+1.000ms",
	)

	assertOutputContains(
		t,
		result,
		"throughput change successful=-20.80%",
	)

	assertOutputContains(
		t,
		result,
		"Measured duration: 2s",
	)
}

func TestFormatDuration(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "zero",
			duration: 0,
			expected: "0s",
		},
		{
			name:     "sub millisecond",
			duration: 400 * time.Microsecond,
			expected: "1ms",
		},
		{
			name:     "milliseconds",
			duration: 382 * time.Millisecond,
			expected: "382ms",
		},
		{
			name:     "seconds",
			duration: 7 * time.Second,
			expected: "7s",
		},
		{
			name:     "minutes",
			duration: 69 * time.Second,
			expected: "1m09s",
		},
		{
			name: "hours",
			duration: time.Hour +
				2*time.Minute +
				3*time.Second,
			expected: "1h02m03s",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				actual := formatDuration(
					test.duration,
				)

				if actual != test.expected {
					t.Fatalf(
						"unexpected duration: got %q, want %q",
						actual,
						test.expected,
					)
				}
			},
		)
	}
}

func TestProgressBar(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name     string
		percent  float64
		width    int
		expected string
	}{
		{
			name:     "zero",
			percent:  0,
			width:    5,
			expected: "[-----]",
		},
		{
			name:     "half",
			percent:  50,
			width:    6,
			expected: "[###---]",
		},
		{
			name:     "complete",
			percent:  100,
			width:    5,
			expected: "[#####]",
		},
		{
			name:     "clamps negative",
			percent:  -25,
			width:    5,
			expected: "[-----]",
		},
		{
			name:     "clamps above one hundred",
			percent:  125,
			width:    5,
			expected: "[#####]",
		},
		{
			name:     "invalid width",
			percent:  50,
			width:    0,
			expected: "[]",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				actual := progressBar(
					test.percent,
					test.width,
				)

				if actual != test.expected {
					t.Fatalf(
						"unexpected progress bar: got %q, want %q",
						actual,
						test.expected,
					)
				}
			},
		)
	}
}

func assertOutputContains(
	t *testing.T,
	output string,
	expected string,
) {
	t.Helper()

	if !strings.Contains(
		output,
		expected,
	) {
		t.Fatalf(
			"output does not contain %q:\n%s",
			expected,
			output,
		)
	}
}
