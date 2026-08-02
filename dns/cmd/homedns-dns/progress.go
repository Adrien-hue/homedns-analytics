package main

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/Adrien-hue/homedns-analytics/dns/internal/benchmark"
)

const (
	progressBarWidth  = 30
	progressLineWidth = 72
)

// terminalProgressReporter renders benchmark progress as line-oriented output.
//
// It intentionally avoids terminal cursor control so output remains readable
// over SSH, in CI logs, and when redirected to a file.
type terminalProgressReporter struct {
	writer io.Writer

	queryCountPerPath int

	mutex sync.Mutex

	lastEventByPhase map[string]time.Time
}

// newTerminalProgressReporter creates a terminal benchmark reporter.
func newTerminalProgressReporter(
	writer io.Writer,
	queryCountPerPath int,
) benchmark.ProgressReporter {
	if writer == nil {
		return nil
	}

	return &terminalProgressReporter{
		writer: writer,

		queryCountPerPath: queryCountPerPath,

		lastEventByPhase: make(map[string]time.Time),
	}
}

// ReportProgress implements benchmark.ProgressReporter.
func (r *terminalProgressReporter) ReportProgress(
	event benchmark.ProgressEvent,
) {
	if r == nil ||
		r.writer == nil {
		return
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	switch event.Type {
	case benchmark.ProgressEventBenchmarkStarted:
		r.reportBenchmarkStarted(event)

	case benchmark.ProgressEventScenarioStarted:
		r.reportScenarioStarted(event)

	case benchmark.ProgressEventPhaseStarted:
		r.reportPhaseStarted(event)

	case benchmark.ProgressEventPhaseProgress:
		r.reportPhaseProgress(event)

	case benchmark.ProgressEventPhaseCompleted:
		r.reportPhaseCompleted(event)

	case benchmark.ProgressEventScenarioCompleted:
		r.reportScenarioCompleted(event)

	case benchmark.ProgressEventBenchmarkCompleted:
		r.reportBenchmarkCompleted(event)
	}
}

func (r *terminalProgressReporter) reportBenchmarkStarted(
	event benchmark.ProgressEvent,
) {
	fmt.Fprintln(
		r.writer,
		"Benchmark progress",
	)

	fmt.Fprintln(
		r.writer,
		strings.Repeat(
			"-",
			progressLineWidth,
		),
	)

	fmt.Fprintf(
		r.writer,
		"Scenarios: %d\n",
		event.ScenarioCount,
	)

	fmt.Fprintf(
		r.writer,
		"Measured queries: %d\n",
		event.OverallTotal(
			r.queryCountPerPath,
		),
	)

	fmt.Fprintln(r.writer)
}

func (r *terminalProgressReporter) reportScenarioStarted(
	event benchmark.ProgressEvent,
) {
	fmt.Fprintf(
		r.writer,
		"[%d/%d] %s\n",
		event.ScenarioIndex,
		event.ScenarioCount,
		displayScenarioName(
			event.ScenarioName,
		),
	)
}

func (r *terminalProgressReporter) reportPhaseStarted(
	event benchmark.ProgressEvent,
) {
	if event.IsWarmup() {
		return
	}

	fmt.Fprintf(
		r.writer,
		"  %-20s started\n",
		displayPhaseName(
			event.Phase,
		),
	)
}

func (r *terminalProgressReporter) reportPhaseProgress(
	event benchmark.ProgressEvent,
) {
	if event.IsWarmup() {
		return
	}

	phaseKey := fmt.Sprintf(
		"%d:%s",
		event.ScenarioIndex,
		event.Phase,
	)

	now := event.OccurredAt
	if now.IsZero() {
		now = time.Now()
	}

	lastEventAt :=
		r.lastEventByPhase[phaseKey]

	if event.Current < event.Total &&
		!lastEventAt.IsZero() &&
		now.Sub(lastEventAt) < time.Second {
		return
	}

	r.lastEventByPhase[phaseKey] = now

	fmt.Fprintf(
		r.writer,
		"  %-12s %s %6.2f%%  %d/%d",
		displayPhaseName(
			event.Phase,
		),
		progressBar(
			event.Percent(),
			progressBarWidth,
		),
		event.Percent(),
		event.Current,
		event.Total,
	)

	fmt.Fprintf(
		r.writer,
		"  qps=%.1f/%.1f",
		event.AttemptedQueriesPerSecond,
		event.SuccessfulQueriesPerSecond,
	)

	fmt.Fprintf(
		r.writer,
		"  success=%d failed=%d timeouts=%d",
		event.Successful,
		event.Failed,
		event.Timeouts,
	)

	fmt.Fprintf(
		r.writer,
		"  phase=%s",
		formatDuration(
			event.PhaseElapsed,
		),
	)

	remaining := event.EstimatedRemaining(
		r.queryCountPerPath,
	)

	if remaining > 0 {
		fmt.Fprintf(
			r.writer,
			"  benchmark_eta=%s",
			formatDuration(
				remaining,
			),
		)
	}

	fmt.Fprintln(r.writer)
}

func (r *terminalProgressReporter) reportPhaseCompleted(
	event benchmark.ProgressEvent,
) {
	if event.IsWarmup() {
		fmt.Fprintf(
			r.writer,
			"  %-20s completed — %d/%d successful\n",
			displayPhaseName(
				event.Phase,
			),
			event.Successful,
			event.Total,
		)

		return
	}

	if event.PathResult == nil {
		fmt.Fprintf(
			r.writer,
			"  %-20s completed — %d/%d successful, %d failed, %d timeouts\n",
			displayPhaseName(
				event.Phase,
			),
			event.Successful,
			event.Total,
			event.Failed,
			event.Timeouts,
		)

		return
	}

	result := event.PathResult
	latency :=
		result.SuccessfulRequestLatencyMilliseconds

	fmt.Fprintf(
		r.writer,
		"  %-20s completed — successful=%d/%d failed=%d timeouts=%d\n",
		displayPhaseName(
			event.Phase,
		),
		result.Requests.Successful,
		result.Requests.Attempted,
		result.Requests.Failed,
		result.Requests.Timeouts,
	)

	fmt.Fprintf(
		r.writer,
		"  %-20s qps attempted=%.2f successful=%.2f | mean=%.3fms p95=%.3fms\n",
		"",
		result.Throughput.
			AttemptedQueriesPerSecond,
		result.Throughput.
			SuccessfulQueriesPerSecond,
		latency.Mean,
		latency.P95,
	)
}

func (r *terminalProgressReporter) reportScenarioCompleted(
	event benchmark.ProgressEvent,
) {
	if event.ScenarioResult == nil {
		fmt.Fprintf(
			r.writer,
			"  Scenario completed in %s\n\n",
			formatDuration(
				event.Elapsed,
			),
		)

		return
	}

	result := event.ScenarioResult

	fmt.Fprintln(
		r.writer,
		"  Scenario summary",
	)

	reportScenarioPathSummary(
		r.writer,
		"Direct",
		result.Direct,
	)

	reportScenarioPathSummary(
		r.writer,
		"Forwarded",
		result.Forwarded,
	)

	measuredDuration :=
		time.Duration(
			(result.Direct.DurationMilliseconds +
				result.Forwarded.DurationMilliseconds) *
				float64(time.Millisecond),
		)

	fmt.Fprintf(
		r.writer,
		"  %-12s latency overhead mean=%+.3fms p95=%+.3fms\n",
		"Comparison",
		result.Comparison.
			LatencyOverheadMilliseconds.
			Mean,
		result.Comparison.
			LatencyOverheadMilliseconds.
			P95,
	)

	fmt.Fprintf(
		r.writer,
		"  %-12s throughput change successful=%+.2f%%\n",
		"",
		result.Comparison.
			ThroughputChangePercent.
			Successful,
	)

	fmt.Fprintf(
		r.writer,
		"  Measured duration: %s\n",
		formatDuration(
			measuredDuration,
		),
	)

	fmt.Fprintln(r.writer)
}

func reportScenarioPathSummary(
	writer io.Writer,
	name string,
	result benchmark.PathResult,
) {
	latency :=
		result.SuccessfulRequestLatencyMilliseconds

	fmt.Fprintf(
		writer,
		"  %-12s successful=%d/%d (%.2f%%) failed=%d timeouts=%d\n",
		name,
		result.Requests.Successful,
		result.Requests.Attempted,
		result.Rates.SuccessPercent,
		result.Requests.Failed,
		result.Requests.Timeouts,
	)

	fmt.Fprintf(
		writer,
		"  %-12s qps=%.2f | mean=%.3fms median=%.3fms p95=%.3fms p99=%.3fms\n",
		"",
		result.Throughput.
			SuccessfulQueriesPerSecond,
		latency.Mean,
		latency.Median,
		latency.P95,
		latency.P99,
	)
}

func (r *terminalProgressReporter) reportBenchmarkCompleted(
	event benchmark.ProgressEvent,
) {
	fmt.Fprintln(
		r.writer,
		strings.Repeat(
			"-",
			progressLineWidth,
		),
	)

	fmt.Fprintf(
		r.writer,
		"Benchmark execution finished in %s\n",
		formatDuration(
			event.Elapsed,
		),
	)

	fmt.Fprintln(r.writer)
}

func progressBar(
	percent float64,
	width int,
) string {
	if width <= 0 {
		return "[]"
	}

	if percent < 0 {
		percent = 0
	}

	if percent > 100 {
		percent = 100
	}

	completed := int(
		percent /
			100 *
			float64(width),
	)

	if completed > width {
		completed = width
	}

	return "[" +
		strings.Repeat(
			"#",
			completed,
		) +
		strings.Repeat(
			"-",
			width-completed,
		) +
		"]"
}

func displayScenarioName(
	value string,
) string {
	switch value {
	case "udp-sequential":
		return "UDP sequential"

	case "udp-concurrent":
		return "UDP concurrent"

	case "tcp-sequential":
		return "TCP sequential"

	case "tcp-concurrent":
		return "TCP concurrent"

	default:
		return value
	}
}

func displayPhaseName(
	value string,
) string {
	switch value {
	case benchmark.ProgressPhaseWarmupDirect:
		return "Direct warmup"

	case benchmark.ProgressPhaseWarmupForwarded:
		return "Forwarded warmup"

	case benchmark.ProgressPhaseDirect:
		return "Direct"

	case benchmark.ProgressPhaseForwarded:
		return "Forwarded"

	default:
		return value
	}
}

func formatDuration(
	duration time.Duration,
) string {
	if duration <= 0 {
		return "0s"
	}

	if duration < time.Second {
		milliseconds :=
			duration.Round(
				time.Millisecond,
			) / time.Millisecond

		if milliseconds < 1 {
			milliseconds = 1
		}

		return fmt.Sprintf(
			"%dms",
			milliseconds,
		)
	}

	duration = duration.Round(
		time.Second,
	)

	hours := int(
		duration / time.Hour,
	)

	duration -=
		time.Duration(hours) *
			time.Hour

	minutes := int(
		duration / time.Minute,
	)

	duration -=
		time.Duration(minutes) *
			time.Minute

	seconds := int(
		duration / time.Second,
	)

	switch {
	case hours > 0:
		return fmt.Sprintf(
			"%dh%02dm%02ds",
			hours,
			minutes,
			seconds,
		)

	case minutes > 0:
		return fmt.Sprintf(
			"%dm%02ds",
			minutes,
			seconds,
		)

	default:
		return fmt.Sprintf(
			"%ds",
			seconds,
		)
	}
}
