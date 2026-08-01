package benchmark

import "time"

const (
	ProgressPhaseWarmupDirect    = "warmup-direct"
	ProgressPhaseWarmupForwarded = "warmup-forwarded"
	ProgressPhaseDirect          = "direct"
	ProgressPhaseForwarded       = "forwarded"
)

const (
	ProgressEventBenchmarkStarted   = "benchmark-started"
	ProgressEventScenarioStarted    = "scenario-started"
	ProgressEventPhaseStarted       = "phase-started"
	ProgressEventPhaseProgress      = "phase-progress"
	ProgressEventPhaseCompleted     = "phase-completed"
	ProgressEventScenarioCompleted  = "scenario-completed"
	ProgressEventBenchmarkCompleted = "benchmark-completed"
)

// ProgressReporter receives benchmark lifecycle and progress events.
//
// Implementations must return quickly because reporting happens on the
// benchmark execution path. Reporters performing slow I/O should buffer or
// dispatch events asynchronously.
type ProgressReporter interface {
	ReportProgress(ProgressEvent)
}

// ProgressReporterFunc adapts a function into a ProgressReporter.
type ProgressReporterFunc func(ProgressEvent)

// ReportProgress implements ProgressReporter.
func (f ProgressReporterFunc) ReportProgress(
	event ProgressEvent,
) {
	if f == nil {
		return
	}

	f(event)
}

// ProgressEvent describes one benchmark lifecycle or progress update.
type ProgressEvent struct {
	Type string

	BenchmarkStartedAt time.Time
	OccurredAt         time.Time

	ScenarioName  string
	ScenarioIndex int
	ScenarioCount int

	Phase string
	Path  string

	Current int
	Total   int

	Successful int
	Failed     int
	Timeouts   int

	// Elapsed is the duration since the complete benchmark started.
	Elapsed time.Duration

	// PhaseElapsed is the duration since the current path or warmup phase
	// started.
	PhaseElapsed time.Duration

	// AttemptedQueriesPerSecond and SuccessfulQueriesPerSecond represent the
	// live throughput of the current phase.
	AttemptedQueriesPerSecond  float64
	SuccessfulQueriesPerSecond float64

	// PathResult is populated when a measured path completes.
	PathResult *PathResult

	// ScenarioResult is populated when a complete direct-versus-forwarded
	// scenario finishes.
	ScenarioResult *ScenarioResult
}

// IsWarmup reports whether the event belongs to a warmup phase.
func (e ProgressEvent) IsWarmup() bool {
	switch e.Phase {
	case ProgressPhaseWarmupDirect,
		ProgressPhaseWarmupForwarded:
		return true

	default:
		return false
	}
}

// Percent returns the completion percentage for the current phase.
func (e ProgressEvent) Percent() float64 {
	if e.Total <= 0 {
		return 0
	}

	percent := float64(e.Current) /
		float64(e.Total) *
		100

	if percent < 0 {
		return 0
	}

	if percent > 100 {
		return 100
	}

	return percent
}

// OverallCompleted returns the number of completed measured queries across all
// scenarios, excluding warmup queries.
//
// ScenarioIndex is one-based. The current measured phase contributes its
// Current value.
func (e ProgressEvent) OverallCompleted(
	queriesPerPath int,
) int {
	if queriesPerPath <= 0 ||
		e.ScenarioIndex <= 0 {
		return 0
	}

	completedScenarios :=
		e.ScenarioIndex - 1

	completed := completedScenarios *
		2 *
		queriesPerPath

	switch e.Phase {
	case ProgressPhaseDirect:
		completed += e.Current

	case ProgressPhaseForwarded:
		completed += queriesPerPath
		completed += e.Current
	}

	return completed
}

// OverallTotal returns the complete number of measured DNS queries.
func (e ProgressEvent) OverallTotal(
	queriesPerPath int,
) int {
	if queriesPerPath <= 0 ||
		e.ScenarioCount <= 0 {
		return 0
	}

	return e.ScenarioCount *
		2 *
		queriesPerPath
}

// OverallPercent returns completion across every measured path.
func (e ProgressEvent) OverallPercent(
	queriesPerPath int,
) float64 {
	total := e.OverallTotal(
		queriesPerPath,
	)
	if total <= 0 {
		return 0
	}

	percent := float64(
		e.OverallCompleted(
			queriesPerPath,
		),
	) / float64(total) * 100

	if percent < 0 {
		return 0
	}

	if percent > 100 {
		return 100
	}

	return percent
}

// EstimatedRemaining estimates the complete benchmark time remaining from
// measured overall progress.
//
// The estimate is hidden until at least 1% of the measured workload, with a
// minimum of 10 queries, has completed. This prevents misleading estimates
// based on the first few DNS requests.
func (e ProgressEvent) EstimatedRemaining(
	queriesPerPath int,
) time.Duration {
	completed := e.OverallCompleted(
		queriesPerPath,
	)

	total := e.OverallTotal(
		queriesPerPath,
	)

	minimumSamples := total / 100
	if minimumSamples < 10 {
		minimumSamples = 10
	}

	if completed < minimumSamples ||
		total <= completed ||
		e.Elapsed <= 0 {
		return 0
	}

	averagePerQuery :=
		e.Elapsed /
			time.Duration(completed)

	return averagePerQuery *
		time.Duration(total-completed)
}

type noopProgressReporter struct{}

func (noopProgressReporter) ReportProgress(
	ProgressEvent,
) {
}

func resolvedProgressReporter(
	reporter ProgressReporter,
) ProgressReporter {
	if reporter == nil {
		return noopProgressReporter{}
	}

	return reporter
}
