package benchmark

import (
	"testing"
	"time"
)

func TestProgressReporterFuncReportsEvent(
	t *testing.T,
) {
	t.Parallel()

	expected := ProgressEvent{
		Type: ProgressEventPhaseProgress,

		ScenarioName:  "udp-sequential",
		ScenarioIndex: 1,
		ScenarioCount: 4,

		Phase: ProgressPhaseDirect,

		Current: 50,
		Total:   100,
	}

	var received ProgressEvent

	reporter := ProgressReporterFunc(
		func(event ProgressEvent) {
			received = event
		},
	)

	reporter.ReportProgress(expected)

	if received != expected {
		t.Fatalf(
			"unexpected progress event: got %+v, want %+v",
			received,
			expected,
		)
	}
}

func TestNilProgressReporterFuncDoesNothing(
	t *testing.T,
) {
	t.Parallel()

	var reporter ProgressReporterFunc

	reporter.ReportProgress(
		ProgressEvent{
			Type: ProgressEventBenchmarkStarted,
		},
	)
}

func TestProgressEventIsWarmup(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name     string
		phase    string
		expected bool
	}{
		{
			name:     "direct warmup",
			phase:    ProgressPhaseWarmupDirect,
			expected: true,
		},
		{
			name:     "forwarded warmup",
			phase:    ProgressPhaseWarmupForwarded,
			expected: true,
		},
		{
			name:     "direct measurement",
			phase:    ProgressPhaseDirect,
			expected: false,
		},
		{
			name:     "forwarded measurement",
			phase:    ProgressPhaseForwarded,
			expected: false,
		},
		{
			name:     "empty phase",
			phase:    "",
			expected: false,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				event := ProgressEvent{
					Phase: test.phase,
				}

				actual := event.IsWarmup()

				if actual != test.expected {
					t.Fatalf(
						"unexpected warmup result: got %t, want %t",
						actual,
						test.expected,
					)
				}
			},
		)
	}
}

func TestProgressEventPercent(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name     string
		current  int
		total    int
		expected float64
	}{
		{
			name:     "zero total",
			current:  0,
			total:    0,
			expected: 0,
		},
		{
			name:     "half complete",
			current:  50,
			total:    100,
			expected: 50,
		},
		{
			name:     "complete",
			current:  100,
			total:    100,
			expected: 100,
		},
		{
			name:     "clamps negative",
			current:  -10,
			total:    100,
			expected: 0,
		},
		{
			name:     "clamps above total",
			current:  150,
			total:    100,
			expected: 100,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				event := ProgressEvent{
					Current: test.current,
					Total:   test.total,
				}

				actual := event.Percent()

				if actual != test.expected {
					t.Fatalf(
						"unexpected percent: got %f, want %f",
						actual,
						test.expected,
					)
				}
			},
		)
	}
}

func TestProgressEventOverallCompleted(
	t *testing.T,
) {
	t.Parallel()

	const queriesPerPath = 100

	tests := []struct {
		name          string
		scenarioIndex int
		phase         string
		current       int
		expected      int
	}{
		{
			name:          "invalid scenario index",
			scenarioIndex: 0,
			phase:         ProgressPhaseDirect,
			current:       10,
			expected:      0,
		},
		{
			name:          "first scenario warmup",
			scenarioIndex: 1,
			phase:         ProgressPhaseWarmupDirect,
			current:       10,
			expected:      0,
		},
		{
			name:          "first scenario direct",
			scenarioIndex: 1,
			phase:         ProgressPhaseDirect,
			current:       25,
			expected:      25,
		},
		{
			name:          "first scenario forwarded",
			scenarioIndex: 1,
			phase:         ProgressPhaseForwarded,
			current:       25,
			expected:      125,
		},
		{
			name:          "second scenario direct",
			scenarioIndex: 2,
			phase:         ProgressPhaseDirect,
			current:       50,
			expected:      250,
		},
		{
			name:          "fourth scenario forwarded complete",
			scenarioIndex: 4,
			phase:         ProgressPhaseForwarded,
			current:       100,
			expected:      800,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				event := ProgressEvent{
					ScenarioIndex: test.scenarioIndex,
					Phase:         test.phase,
					Current:       test.current,
				}

				actual := event.OverallCompleted(
					queriesPerPath,
				)

				if actual != test.expected {
					t.Fatalf(
						"unexpected completed count: got %d, want %d",
						actual,
						test.expected,
					)
				}
			},
		)
	}
}

func TestProgressEventOverallTotal(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name           string
		scenarioCount  int
		queriesPerPath int
		expected       int
	}{
		{
			name:           "invalid scenario count",
			scenarioCount:  0,
			queriesPerPath: 100,
			expected:       0,
		},
		{
			name:           "invalid query count",
			scenarioCount:  4,
			queriesPerPath: 0,
			expected:       0,
		},
		{
			name:           "four scenarios",
			scenarioCount:  4,
			queriesPerPath: 100,
			expected:       800,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				event := ProgressEvent{
					ScenarioCount: test.scenarioCount,
				}

				actual := event.OverallTotal(
					test.queriesPerPath,
				)

				if actual != test.expected {
					t.Fatalf(
						"unexpected total: got %d, want %d",
						actual,
						test.expected,
					)
				}
			},
		)
	}
}

func TestProgressEventOverallPercent(
	t *testing.T,
) {
	t.Parallel()

	event := ProgressEvent{
		ScenarioIndex: 2,
		ScenarioCount: 4,
		Phase:         ProgressPhaseDirect,
		Current:       100,
	}

	actual := event.OverallPercent(100)

	// One complete scenario = 200 queries, plus 100 direct queries from the
	// second scenario. The complete benchmark contains 800 queries.
	const expected = 37.5

	if actual != expected {
		t.Fatalf(
			"unexpected overall percent: got %f, want %f",
			actual,
			expected,
		)
	}
}

func TestProgressEventEstimatedRemainingWaitsForMinimumSamples(
	t *testing.T,
) {
	t.Parallel()

	event := ProgressEvent{
		ScenarioIndex: 1,
		ScenarioCount: 4,
		Phase:         ProgressPhaseDirect,
		Current:       5,
		Elapsed:       time.Second,
	}

	actual := event.EstimatedRemaining(100)

	if actual != 0 {
		t.Fatalf(
			"expected no estimate, got %s",
			actual,
		)
	}
}

func TestProgressEventEstimatedRemaining(
	t *testing.T,
) {
	t.Parallel()

	event := ProgressEvent{
		ScenarioIndex: 1,
		ScenarioCount: 4,
		Phase:         ProgressPhaseDirect,
		Current:       100,
		Elapsed:       10 * time.Second,
	}

	actual := event.EstimatedRemaining(100)

	// The benchmark contains 800 measured queries. At 100 completed queries in
	// 10 seconds, 700 remain at an average of 100 ms per query.
	expected := 70 * time.Second

	if actual != expected {
		t.Fatalf(
			"unexpected estimate: got %s, want %s",
			actual,
			expected,
		)
	}
}

func TestProgressEventEstimatedRemainingReturnsZeroWhenComplete(
	t *testing.T,
) {
	t.Parallel()

	event := ProgressEvent{
		ScenarioIndex: 4,
		ScenarioCount: 4,
		Phase:         ProgressPhaseForwarded,
		Current:       100,
		Elapsed:       time.Minute,
	}

	actual := event.EstimatedRemaining(100)

	if actual != 0 {
		t.Fatalf(
			"expected no remaining duration, got %s",
			actual,
		)
	}
}

func TestResolvedProgressReporterReturnsNoopForNil(
	t *testing.T,
) {
	t.Parallel()

	reporter := resolvedProgressReporter(nil)

	if reporter == nil {
		t.Fatal(
			"expected non-nil no-op progress reporter",
		)
	}

	reporter.ReportProgress(
		ProgressEvent{
			Type: ProgressEventBenchmarkStarted,
		},
	)
}

func TestResolvedProgressReporterKeepsProvidedReporter(
	t *testing.T,
) {
	t.Parallel()

	called := false

	provided := ProgressReporterFunc(
		func(ProgressEvent) {
			called = true
		},
	)

	reporter := resolvedProgressReporter(
		provided,
	)

	reporter.ReportProgress(
		ProgressEvent{
			Type: ProgressEventBenchmarkStarted,
		},
	)

	if !called {
		t.Fatal(
			"provided progress reporter was not called",
		)
	}
}
