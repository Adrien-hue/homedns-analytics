package benchmark

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/miekg/dns"
)

// ReportRunConfig contains the information required to execute a benchmark
// suite and construct its canonical report.
type ReportRunConfig struct {
	ID string

	Suite SuiteConfig

	Project     ProjectMetadata
	Environment Environment
	Resources   ResourceSummary

	HealthAddress string
}

// Validate verifies the report execution configuration.
func (c ReportRunConfig) Validate() error {
	if c.ID == "" {
		return errors.New("benchmark ID is required")
	}

	if err := c.Suite.Validate(); err != nil {
		return fmt.Errorf(
			"invalid benchmark suite configuration: %w",
			err,
		)
	}

	return nil
}

type suiteExecutor interface {
	Run(
		context.Context,
		SuiteConfig,
	) ([]ScenarioResult, error)
}

// ReportRunner executes a benchmark suite and assembles the canonical report.
type ReportRunner struct {
	suiteRunner suiteExecutor
	now         func() time.Time
}

// NewReportRunner creates a report runner using the default benchmark suite.
func NewReportRunner() *ReportRunner {
	return &ReportRunner{
		suiteRunner: NewSuiteRunner(),
		now:         time.Now,
	}
}

// Run executes the benchmark suite and builds its complete report.
func (r *ReportRunner) Run(
	ctx context.Context,
	config ReportRunConfig,
) (Report, error) {
	if r == nil || r.suiteRunner == nil {
		return Report{}, errors.New(
			"benchmark report runner is not initialized",
		)
	}

	if r.now == nil {
		return Report{}, errors.New(
			"benchmark report clock is not initialized",
		)
	}

	if err := config.Validate(); err != nil {
		return Report{}, fmt.Errorf(
			"validate benchmark report configuration: %w",
			err,
		)
	}

	startedAt := r.now()
	report := NewReport(config.ID, startedAt)

	applyReportMetadata(&report, config)

	scenarios, err := r.suiteRunner.Run(
		ctx,
		config.Suite,
	)
	if err != nil {
		return Report{}, fmt.Errorf(
			"run benchmark suite: %w",
			err,
		)
	}

	report.Scenarios = append(
		report.Scenarios,
		scenarios...,
	)

	report.Summary = summarizeScenarios(
		report.Scenarios,
	)

	report.Complete(r.now())

	return report, nil
}

func applyReportMetadata(
	report *Report,
	config ReportRunConfig,
) {
	report.Project = config.Project
	if report.Project.Name == "" {
		report.Project.Name = "homedns-analytics"
	}

	report.Environment = config.Environment
	report.Resources = config.Resources

	report.Configuration = Configuration{
		UpstreamAddress: config.Suite.DirectAddress,
		HomeDNSAddress:  config.Suite.ForwardedAddress,
		HealthAddress:   config.HealthAddress,
		QueryNames: append(
			[]string(nil),
			config.Suite.QueryNames...,
		),
		QueryType: queryTypeName(
			config.Suite.QueryType,
		),
		WarmupQueries: config.Suite.WarmupQueries,
		TimeoutSeconds: round(
			config.Suite.Timeout.Seconds(),
			6,
		),
	}
}

func summarizeScenarios(
	scenarios []ScenarioResult,
) BenchmarkSummary {
	summary := BenchmarkSummary{
		Status: "success",
		Notes:  make([]string, 0),
	}

	if len(scenarios) == 0 {
		summary.Status = "unknown"
		summary.Notes = append(
			summary.Notes,
			"no benchmark scenarios were executed",
		)

		return summary
	}

	for _, scenario := range scenarios {
		accumulatePathSummary(
			&summary,
			scenario.Direct,
		)

		accumulatePathSummary(
			&summary,
			scenario.Forwarded,
		)
	}

	if summary.FailedTotal > 0 ||
		summary.TimeoutsTotal > 0 {
		summary.Status = "degraded"

		summary.Notes = append(
			summary.Notes,
			"one or more DNS queries failed or timed out",
		)
	}

	return summary
}

func accumulatePathSummary(
	summary *BenchmarkSummary,
	result PathResult,
) {
	summary.RequestsTotal += result.Requests
	summary.SuccessfulTotal += result.Successful
	summary.FailedTotal += result.Failed
	summary.TimeoutsTotal += result.Timeouts
}

func queryTypeName(queryType uint16) string {
	if name, ok := dns.TypeToString[queryType]; ok {
		return name
	}

	return fmt.Sprintf("TYPE%d", queryType)
}
