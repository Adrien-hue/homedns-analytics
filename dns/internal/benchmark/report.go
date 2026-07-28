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

const SchemaVersion = "1.0"

// Report is the canonical machine-readable result of one benchmark execution.
type Report struct {
	SchemaVersion string `json:"schema_version"`

	Benchmark     BenchmarkMetadata `json:"benchmark"`
	Project       ProjectMetadata   `json:"project"`
	Environment   Environment       `json:"environment"`
	Configuration Configuration     `json:"configuration"`

	Scenarios []ScenarioResult `json:"scenarios"`
	Resources ResourceSummary  `json:"resources"`
	Summary   BenchmarkSummary `json:"summary"`
}

type BenchmarkMetadata struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`

	DurationMilliseconds float64 `json:"duration_ms"`
}

type ProjectMetadata struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	GitBranch string `json:"git_branch"`
	Dirty     bool   `json:"dirty"`
}

type Environment struct {
	Hostname        string `json:"hostname"`
	OperatingSystem string `json:"operating_system"`
	Kernel          string `json:"kernel"`
	Architecture    string `json:"architecture"`
	CPUModel        string `json:"cpu_model"`
	LogicalCPUs     int    `json:"logical_cpus"`

	MemoryTotalBytes *uint64 `json:"memory_total_bytes"`
	GoVersion        string  `json:"go_version"`
}

type Configuration struct {
	UpstreamAddress string `json:"upstream_address"`
	HomeDNSAddress  string `json:"homedns_address"`
	HealthAddress   string `json:"health_address"`

	QueryNames []string `json:"query_names"`
	QueryType  string   `json:"query_type"`

	WarmupQueries  int     `json:"warmup_queries"`
	TimeoutSeconds float64 `json:"timeout_seconds"`
}

type ScenarioResult struct {
	Name        string `json:"name"`
	Protocol    string `json:"protocol"`
	Concurrency int    `json:"concurrency"`
	QueryCount  int    `json:"query_count"`

	Direct    PathResult     `json:"direct"`
	Forwarded PathResult     `json:"forwarded"`
	Overhead  OverheadResult `json:"overhead"`
}

type PathResult struct {
	Requests   int `json:"requests"`
	Successful int `json:"successful"`
	Failed     int `json:"failed"`
	Timeouts   int `json:"timeouts"`

	DurationMilliseconds float64           `json:"duration_ms"`
	QueriesPerSecond     float64           `json:"queries_per_second"`
	LatencyMilliseconds  LatencyStatistics `json:"latency_ms"`
}

type OverheadResult struct {
	MeanLatencyMilliseconds   float64 `json:"mean_latency_ms"`
	MedianLatencyMilliseconds float64 `json:"median_latency_ms"`
	P95LatencyMilliseconds    float64 `json:"p95_latency_ms"`
	P99LatencyMilliseconds    float64 `json:"p99_latency_ms"`

	MeanLatencyPercent float64 `json:"mean_latency_percent"`
	ThroughputPercent  float64 `json:"throughput_percent"`
}

type ResourceSummary struct {
	Process ProcessResources `json:"process"`
	System  SystemResources  `json:"system"`
}

type ProcessResources struct {
	CPUPercentMean *float64 `json:"cpu_percent_mean"`
	CPUPercentPeak *float64 `json:"cpu_percent_peak"`

	RSSBytesMean *uint64 `json:"rss_bytes_mean"`
	RSSBytesPeak *uint64 `json:"rss_bytes_peak"`
}

type SystemResources struct {
	LoadAverageOneMinuteBefore *float64 `json:"load_average_1m_before"`
	LoadAverageOneMinutePeak   *float64 `json:"load_average_1m_peak"`

	TemperatureCelsiusBefore *float64 `json:"temperature_celsius_before"`
	TemperatureCelsiusPeak   *float64 `json:"temperature_celsius_peak"`

	ThrottledStateBefore *string `json:"throttled_state_before"`
	ThrottledStateAfter  *string `json:"throttled_state_after"`
}

type BenchmarkSummary struct {
	Status string   `json:"status"`
	Notes  []string `json:"notes"`

	RequestsTotal   int `json:"requests_total"`
	SuccessfulTotal int `json:"successful_total"`
	FailedTotal     int `json:"failed_total"`
	TimeoutsTotal   int `json:"timeouts_total"`
}

// NewReport creates an empty report with the current schema version.
func NewReport(
	id string,
	startedAt time.Time,
) Report {
	return Report{
		SchemaVersion: SchemaVersion,
		Benchmark: BenchmarkMetadata{
			ID:        id,
			CreatedAt: startedAt.UTC(),
			StartedAt: startedAt.UTC(),
		},
		Project: ProjectMetadata{
			Name: "homedns-analytics",
		},
		Scenarios: make([]ScenarioResult, 0),
		Summary: BenchmarkSummary{
			Status: "unknown",
			Notes:  make([]string, 0),
		},
	}
}

// Complete records the benchmark completion time and total duration.
func (r *Report) Complete(endedAt time.Time) {
	if r == nil {
		return
	}

	r.Benchmark.EndedAt = endedAt.UTC()
	r.Benchmark.DurationMilliseconds = round(
		durationMilliseconds(
			endedAt.Sub(r.Benchmark.StartedAt),
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
