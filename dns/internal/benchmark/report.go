package benchmark

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const SchemaVersion = "2.0"

const (
	StatusUnknown  = "unknown"
	StatusPassed   = "passed"
	StatusDegraded = "degraded"
	StatusFailed   = "failed"
	StatusInvalid  = "invalid"
)

// Report is the canonical machine-readable result of one benchmark execution.
type Report struct {
	SchemaVersion string `json:"schema_version"`

	Report    ReportMetadata    `json:"report"`
	Execution ExecutionMetadata `json:"execution"`

	BenchmarkBinary BenchmarkBinaryMetadata `json:"benchmark_binary"`
	TargetService   TargetServiceMetadata   `json:"target_service"`
	Environment     Environment             `json:"environment"`
	Configuration   Configuration           `json:"configuration"`

	Scenarios []ScenarioResult `json:"scenarios"`
	Resources ResourceSummary  `json:"resources"`
	Summary   BenchmarkSummary `json:"summary"`
}

// ReportMetadata describes the benchmark report itself.
type ReportMetadata struct {
	ID string `json:"id"`

	CreatedAt time.Time `json:"created_at"`

	Status        string         `json:"status"`
	StatusReasons []StatusReason `json:"status_reasons"`
	Notes         []string       `json:"notes"`
}

// ExecutionMetadata describes the complete benchmark execution window.
type ExecutionMetadata struct {
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`

	DurationMilliseconds float64 `json:"duration_ms"`
}

// BenchmarkBinaryMetadata identifies the binary executing the benchmark.
type BenchmarkBinaryMetadata struct {
	Project   string `json:"project"`
	Component string `json:"component"`

	Version        string `json:"version"`
	GitCommit      string `json:"git_commit"`
	GitBranch      string `json:"git_branch"`
	GitDirty       *bool  `json:"git_dirty"`
	GoVersion      string `json:"go_version"`
	BuildTimestamp string `json:"build_timestamp"`
}

// TargetServiceMetadata identifies the HomeDNS service under test.
//
// Values collected before and after execution allow the report to prove that
// the same service remained available throughout the benchmark.
type TargetServiceMetadata struct {
	Address       string `json:"address"`
	HealthAddress string `json:"health_address"`

	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	BuildTime string `json:"build_time"`

	ReadyBefore *bool `json:"ready_before"`
	ReadyAfter  *bool `json:"ready_after"`

	UptimeSecondsBefore *uint64 `json:"uptime_seconds_before"`
	UptimeSecondsAfter  *uint64 `json:"uptime_seconds_after"`

	PIDBefore *int `json:"pid_before"`
	PIDAfter  *int `json:"pid_after"`

	RestartCountBefore *uint64 `json:"restart_count_before"`
	RestartCountAfter  *uint64 `json:"restart_count_after"`

	RestartDetected *bool `json:"restart_detected"`
}

// Environment describes the machine executing the benchmark.
type Environment struct {
	Hostname        string `json:"hostname"`
	OperatingSystem string `json:"operating_system"`
	Kernel          string `json:"kernel"`
	Architecture    string `json:"architecture"`
	CPUModel        string `json:"cpu_model"`
	LogicalCPUs     int    `json:"logical_cpus"`

	MemoryTotalBytes *uint64 `json:"memory_total_bytes"`
}

// Configuration contains every workload parameter needed to reproduce the
// benchmark.
type Configuration struct {
	UpstreamAddress string `json:"upstream_address"`
	HomeDNSAddress  string `json:"homedns_address"`
	HealthAddress   string `json:"health_address"`

	QueryNames []string `json:"query_names"`
	QueryType  string   `json:"query_type"`

	WarmupQueries      int              `json:"warmup_queries"`
	QueriesPerPath     int              `json:"queries_per_path"`
	ConcurrentWorkers  int              `json:"concurrent_workers"`
	TimeoutSeconds     float64          `json:"timeout_seconds"`
	ResourceIntervalMS int64            `json:"resource_sampling_interval_ms"`
	ScenarioOrder      []string         `json:"scenario_order"`
	StatusThresholds   StatusThresholds `json:"status_thresholds"`
}

// StatusThresholds contains the acceptance thresholds used to classify paths,
// scenarios, and the complete benchmark.
type StatusThresholds struct {
	DegradedFailurePercent float64 `json:"degraded_failure_percent"`
	FailedFailurePercent   float64 `json:"failed_failure_percent"`

	DegradedTimeoutPercent float64 `json:"degraded_timeout_percent"`
	FailedTimeoutPercent   float64 `json:"failed_timeout_percent"`
}

// ScenarioResult contains one direct-versus-forwarded DNS comparison.
type ScenarioResult struct {
	Name        string `json:"name"`
	Protocol    string `json:"protocol"`
	Concurrency int    `json:"concurrency"`

	QueryCountPerPath int `json:"query_count_per_path"`

	Status        string         `json:"status"`
	StatusReasons []StatusReason `json:"status_reasons"`

	Direct     PathResult         `json:"direct"`
	Forwarded  PathResult         `json:"forwarded"`
	Comparison ScenarioComparison `json:"comparison"`
}

// PathResult contains the measured result for one DNS target.
type PathResult struct {
	Requests RequestCounts `json:"requests"`
	Rates    RequestRates  `json:"rates"`

	DurationMilliseconds float64 `json:"duration_ms"`

	Throughput ThroughputMetrics `json:"throughput"`

	// SuccessfulRequestLatencyMilliseconds intentionally excludes failed and
	// timed-out requests. SampleCount makes the measured population explicit.
	SuccessfulRequestLatencyMilliseconds SuccessfulLatencyStatistics `json:"successful_request_latency_ms"`

	Failures FailureBreakdown `json:"failures"`
}

// RequestCounts contains attempted and completed query totals.
type RequestCounts struct {
	Attempted          int `json:"attempted"`
	Successful         int `json:"successful"`
	Failed             int `json:"failed"`
	Timeouts           int `json:"timeouts"`
	NonTimeoutFailures int `json:"non_timeout_failures"`
}

// RequestRates contains path reliability percentages.
type RequestRates struct {
	SuccessPercent float64 `json:"success_percent"`
	FailurePercent float64 `json:"failure_percent"`
	TimeoutPercent float64 `json:"timeout_percent"`
}

// ThroughputMetrics separates attempted throughput from successful throughput.
type ThroughputMetrics struct {
	AttemptedQueriesPerSecond  float64 `json:"attempted_queries_per_second"`
	SuccessfulQueriesPerSecond float64 `json:"successful_queries_per_second"`
}

// SuccessfulLatencyStatistics contains latency statistics for successful
// requests only.
type SuccessfulLatencyStatistics struct {
	SampleCount int `json:"sample_count"`

	Minimum float64 `json:"minimum"`
	Mean    float64 `json:"mean"`
	Median  float64 `json:"median"`
	P95     float64 `json:"p95"`
	P99     float64 `json:"p99"`
	Maximum float64 `json:"maximum"`
}

// FailureBreakdown classifies failed DNS requests.
//
// Timeout is also represented in RequestCounts for quick summary access. The
// detailed breakdown exists to support investigation and future dashboards.
type FailureBreakdown struct {
	Timeout         int `json:"timeout"`
	Network         int `json:"network"`
	DNSResponse     int `json:"dns_response"`
	InvalidResponse int `json:"invalid_response"`
	Internal        int `json:"internal"`
	Other           int `json:"other"`
}

// ScenarioComparison describes the cost and reliability difference introduced
// by forwarding through HomeDNS.
type ScenarioComparison struct {
	LatencyOverheadMilliseconds LatencyComparison `json:"latency_overhead_ms"`
	LatencyOverheadPercent      LatencyComparison `json:"latency_overhead_percent"`

	ThroughputChangePercent ThroughputComparison `json:"throughput_change_percent"`

	ReliabilityDeltaPercentagePoints ReliabilityComparison `json:"reliability_delta_percentage_points"`
}

// LatencyComparison compares successful-request latency statistics.
type LatencyComparison struct {
	Mean   float64 `json:"mean"`
	Median float64 `json:"median"`
	P95    float64 `json:"p95"`
	P99    float64 `json:"p99"`
}

// ThroughputComparison compares direct and forwarded throughput.
//
// A negative value means the forwarded path measured lower throughput than the
// direct path. A positive value means it measured higher throughput.
type ThroughputComparison struct {
	Attempted  float64 `json:"attempted"`
	Successful float64 `json:"successful"`
}

// ReliabilityComparison compares forwarded path rates against direct path
// rates in percentage points.
type ReliabilityComparison struct {
	Success float64 `json:"success"`
	Failure float64 `json:"failure"`
	Timeout float64 `json:"timeout"`
}

// StatusReason explains a machine-readable benchmark classification.
type StatusReason struct {
	Code string `json:"code"`

	Scenario string `json:"scenario,omitempty"`
	Path     string `json:"path,omitempty"`

	Observed  *float64 `json:"observed,omitempty"`
	Threshold *float64 `json:"threshold,omitempty"`

	Message string `json:"message"`
}

// ResourceSummary contains measurements collected during the benchmark.
type ResourceSummary struct {
	Sampling ResourceSampling `json:"sampling"`

	BenchmarkProcess ProcessResources `json:"benchmark_process"`
	TargetService    ProcessResources `json:"target_service"`
	System           SystemResources  `json:"system"`
}

// ResourceSampling describes how resource metrics were collected.
type ResourceSampling struct {
	IntervalMilliseconds int64 `json:"interval_ms"`
	SamplesCollected     int   `json:"samples_collected"`

	CollectionStartedAt *time.Time `json:"collection_started_at"`
	CollectionEndedAt   *time.Time `json:"collection_ended_at"`

	Errors []string `json:"errors"`
}

// ProcessResources contains resource measurements for one process.
type ProcessResources struct {
	PIDBefore *int `json:"pid_before"`
	PIDAfter  *int `json:"pid_after"`

	RestartDetected *bool `json:"restart_detected"`

	CPUPercent NumericStatistics `json:"cpu_percent"`
	RSSBytes   IntegerStatistics `json:"rss_bytes"`
	Threads    IntegerStatistics `json:"threads"`
}

// NumericStatistics contains floating-point sample statistics.
type NumericStatistics struct {
	Minimum *float64 `json:"minimum"`
	Mean    *float64 `json:"mean"`
	Peak    *float64 `json:"peak"`
}

// IntegerStatistics contains integer sample statistics.
type IntegerStatistics struct {
	Minimum *uint64  `json:"minimum"`
	Mean    *float64 `json:"mean"`
	Peak    *uint64  `json:"peak"`
}

// SystemResources contains host-level resource measurements.
type SystemResources struct {
	LoadAverageOneMinute NumericBoundaryStatistics `json:"load_average_1m"`

	MemoryAvailableBytes IntegerBoundaryStatistics `json:"memory_available_bytes"`

	TemperatureCelsius NumericBoundaryStatistics `json:"temperature_celsius"`

	Throttling ThrottlingSummary `json:"throttling"`
}

// NumericBoundaryStatistics contains before, sampled, and after values.
type NumericBoundaryStatistics struct {
	Before *float64 `json:"before"`
	Mean   *float64 `json:"mean"`
	Peak   *float64 `json:"peak"`
	After  *float64 `json:"after"`
}

// IntegerBoundaryStatistics contains before, sampled, and after integer values.
type IntegerBoundaryStatistics struct {
	Before  *uint64 `json:"before"`
	Minimum *uint64 `json:"minimum"`
	After   *uint64 `json:"after"`
}

// ThrottlingSummary describes Raspberry Pi throttling observations.
type ThrottlingSummary struct {
	Before *string `json:"before"`

	ObservedDuringRun *bool `json:"observed_during_run"`

	After *string `json:"after"`
}

// BenchmarkSummary provides a concise view of the entire report.
type BenchmarkSummary struct {
	Status string `json:"status"`

	StatusReasons []StatusReason `json:"status_reasons"`
	Notes         []string       `json:"notes"`

	Requests RequestCounts `json:"requests"`
	Rates    RequestRates  `json:"rates"`

	Throughput ThroughputMetrics `json:"throughput"`

	Reliability ReliabilitySummary `json:"reliability"`

	ScenarioResults ScenarioStatusSummary `json:"scenario_results"`

	WorstScenario *WorstScenarioSummary `json:"worst_scenario"`

	Stability StabilitySummary `json:"stability"`
}

// ReliabilitySummary separates direct and forwarded reliability.
type ReliabilitySummary struct {
	Direct    ReliabilityPathSummary `json:"direct"`
	Forwarded ReliabilityPathSummary `json:"forwarded"`
}

// ReliabilityPathSummary contains aggregate counts and reliability for one
// benchmark path category.
type ReliabilityPathSummary struct {
	Requests RequestCounts `json:"requests"`
	Rates    RequestRates  `json:"rates"`
}

// ScenarioStatusSummary groups scenarios by their final classification.
type ScenarioStatusSummary struct {
	Passed   []string `json:"passed"`
	Degraded []string `json:"degraded"`
	Failed   []string `json:"failed"`
	Invalid  []string `json:"invalid"`
}

// WorstScenarioSummary identifies the least reliable measured path.
type WorstScenarioSummary struct {
	Name string `json:"name"`
	Path string `json:"path"`

	FailurePercent float64 `json:"failure_percent"`
	TimeoutPercent float64 `json:"timeout_percent"`

	P95SuccessfulLatencyMilliseconds float64 `json:"p95_successful_latency_ms"`
}

// StabilitySummary describes whether the target remained operational.
type StabilitySummary struct {
	ServiceReadyAfter *bool `json:"service_ready_after"`

	ServiceRestartDetected *bool `json:"service_restart_detected"`
	MemoryGrowthDetected   *bool `json:"memory_growth_detected"`

	ThermalThrottlingDetected *bool `json:"thermal_throttling_detected"`
}

// NewReport creates an empty report with the current schema version.
func NewReport(
	id string,
	startedAt time.Time,
) Report {
	startedAt = startedAt.UTC()

	return Report{
		SchemaVersion: SchemaVersion,

		Report: ReportMetadata{
			ID:            id,
			CreatedAt:     startedAt,
			Status:        StatusUnknown,
			StatusReasons: make([]StatusReason, 0),
			Notes:         make([]string, 0),
		},

		Execution: ExecutionMetadata{
			StartedAt: startedAt,
		},

		BenchmarkBinary: BenchmarkBinaryMetadata{
			Project:   "homedns-analytics",
			Component: "homedns-dns",
		},

		Configuration: Configuration{
			QueryNames:    make([]string, 0),
			ScenarioOrder: make([]string, 0),
		},

		Scenarios: make([]ScenarioResult, 0),

		Resources: ResourceSummary{
			Sampling: ResourceSampling{
				Errors: make([]string, 0),
			},
		},

		Summary: BenchmarkSummary{
			Status:        StatusUnknown,
			StatusReasons: make([]StatusReason, 0),
			Notes:         make([]string, 0),

			ScenarioResults: ScenarioStatusSummary{
				Passed:   make([]string, 0),
				Degraded: make([]string, 0),
				Failed:   make([]string, 0),
				Invalid:  make([]string, 0),
			},
		},
	}
}

// Complete records the benchmark completion time and total duration.
func (r *Report) Complete(endedAt time.Time) {
	if r == nil {
		return
	}

	endedAt = endedAt.UTC()

	r.Execution.EndedAt = endedAt
	r.Execution.DurationMilliseconds = round(
		durationMilliseconds(
			endedAt.Sub(r.Execution.StartedAt),
		),
		6,
	)
}

// WriteJSON writes an indented JSON report to the supplied writer.
func WriteJSON(
	writer io.Writer,
	report Report,
) error {
	if writer == nil {
		return errors.New("report writer is required")
	}

	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("encode benchmark report: %w", err)
	}

	return nil
}

// WriteJSONFile writes the report atomically to a JSON file.
func WriteJSONFile(
	path string,
	report Report,
) error {
	if path == "" {
		return errors.New("report path is required")
	}

	directory := filepath.Dir(path)

	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf(
			"create report directory: %w",
			err,
		)
	}

	temporaryFile, err := os.CreateTemp(
		directory,
		".benchmark-*.json",
	)
	if err != nil {
		return fmt.Errorf(
			"create temporary report file: %w",
			err,
		)
	}

	temporaryPath := temporaryFile.Name()
	removeTemporary := true

	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()

	if err := WriteJSON(temporaryFile, report); err != nil {
		_ = temporaryFile.Close()

		return err
	}

	if err := temporaryFile.Sync(); err != nil {
		_ = temporaryFile.Close()

		return fmt.Errorf(
			"sync temporary report file: %w",
			err,
		)
	}

	if err := temporaryFile.Close(); err != nil {
		return fmt.Errorf(
			"close temporary report file: %w",
			err,
		)
	}

	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf(
			"replace benchmark report: %w",
			err,
		)
	}

	removeTemporary = false

	return nil
}
