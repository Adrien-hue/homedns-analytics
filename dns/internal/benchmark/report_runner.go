package benchmark

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/miekg/dns"
)

const (
	defaultDegradedFailurePercent = 1.0
	defaultFailedFailurePercent   = 5.0

	defaultDegradedTimeoutPercent = 1.0
	defaultFailedTimeoutPercent   = 5.0
)

// ReportRunConfig contains the information required to execute a benchmark
// suite and construct its canonical report.
type ReportRunConfig struct {
	ID string

	Suite SuiteConfig

	BenchmarkBinary BenchmarkBinaryMetadata
	TargetService   TargetServiceMetadata
	Environment     Environment
	Resources       ResourceSummary

	HealthAddress string

	StatusThresholds StatusThresholds
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

	if err := validateStatusThresholds(
		resolvedStatusThresholds(c.StatusThresholds),
	); err != nil {
		return fmt.Errorf(
			"invalid benchmark status thresholds: %w",
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

	if ctx == nil {
		return Report{}, errors.New(
			"benchmark context is required",
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

	thresholds := report.Configuration.StatusThresholds

	report.Scenarios = classifyScenarios(
		scenarios,
		thresholds,
	)

	report.Complete(r.now())

	report.Summary = summarizeScenarios(
		report.Scenarios,
		report.Execution.DurationMilliseconds,
	)

	report.Report.Status = report.Summary.Status
	report.Report.StatusReasons = append(
		report.Report.StatusReasons,
		report.Summary.StatusReasons...,
	)
	report.Report.Notes = append(
		report.Report.Notes,
		report.Summary.Notes...,
	)

	return report, nil
}

func applyReportMetadata(
	report *Report,
	config ReportRunConfig,
) {
	if report == nil {
		return
	}

	report.BenchmarkBinary = config.BenchmarkBinary

	if report.BenchmarkBinary.Project == "" {
		report.BenchmarkBinary.Project =
			"homedns-analytics"
	}

	if report.BenchmarkBinary.Component == "" {
		report.BenchmarkBinary.Component =
			"homedns-dns"
	}

	report.TargetService = config.TargetService

	if report.TargetService.Address == "" {
		report.TargetService.Address =
			config.Suite.ForwardedAddress
	}

	if report.TargetService.HealthAddress == "" {
		report.TargetService.HealthAddress =
			config.HealthAddress
	}

	report.Environment = config.Environment
	report.Resources = config.Resources

	if report.Resources.Sampling.Errors == nil {
		report.Resources.Sampling.Errors =
			make([]string, 0)
	}

	thresholds := resolvedStatusThresholds(
		config.StatusThresholds,
	)

	scenarios := config.Suite.Scenarios()
	scenarioOrder := make(
		[]string,
		0,
		len(scenarios),
	)

	for _, scenario := range scenarios {
		scenarioOrder = append(
			scenarioOrder,
			scenario.Name,
		)
	}

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

		WarmupQueries:     config.Suite.WarmupQueries,
		QueriesPerPath:    config.Suite.QueryCount,
		ConcurrentWorkers: config.Suite.ConcurrentWorkers,

		TimeoutSeconds: round(
			config.Suite.Timeout.Seconds(),
			6,
		),

		ResourceIntervalMS: config.Resources.
			Sampling.
			IntervalMilliseconds,

		ScenarioOrder:    scenarioOrder,
		StatusThresholds: thresholds,
	}
}

func resolvedStatusThresholds(
	thresholds StatusThresholds,
) StatusThresholds {
	if thresholds ==
		(StatusThresholds{}) {
		return StatusThresholds{
			DegradedFailurePercent: defaultDegradedFailurePercent,
			FailedFailurePercent:   defaultFailedFailurePercent,

			DegradedTimeoutPercent: defaultDegradedTimeoutPercent,
			FailedTimeoutPercent:   defaultFailedTimeoutPercent,
		}
	}

	return thresholds
}

func validateStatusThresholds(
	thresholds StatusThresholds,
) error {
	if thresholds.DegradedFailurePercent < 0 {
		return errors.New(
			"degraded failure percentage cannot be negative",
		)
	}

	if thresholds.FailedFailurePercent < 0 {
		return errors.New(
			"failed failure percentage cannot be negative",
		)
	}

	if thresholds.DegradedTimeoutPercent < 0 {
		return errors.New(
			"degraded timeout percentage cannot be negative",
		)
	}

	if thresholds.FailedTimeoutPercent < 0 {
		return errors.New(
			"failed timeout percentage cannot be negative",
		)
	}

	if thresholds.DegradedFailurePercent >
		thresholds.FailedFailurePercent {
		return errors.New(
			"degraded failure percentage cannot exceed failed failure percentage",
		)
	}

	if thresholds.DegradedTimeoutPercent >
		thresholds.FailedTimeoutPercent {
		return errors.New(
			"degraded timeout percentage cannot exceed failed timeout percentage",
		)
	}

	return nil
}

func classifyScenarios(
	scenarios []ScenarioResult,
	thresholds StatusThresholds,
) []ScenarioResult {
	results := make(
		[]ScenarioResult,
		len(scenarios),
	)

	for index := range scenarios {
		results[index] = scenarios[index]
		classifyScenario(
			&results[index],
			thresholds,
		)
	}

	return results
}

func classifyScenario(
	scenario *ScenarioResult,
	thresholds StatusThresholds,
) {
	if scenario == nil {
		return
	}

	scenario.Status = StatusPassed
	scenario.StatusReasons = make(
		[]StatusReason,
		0,
	)

	classifyPath(
		scenario,
		"direct",
		scenario.Direct,
		thresholds,
	)

	classifyPath(
		scenario,
		"forwarded",
		scenario.Forwarded,
		thresholds,
	)
}

func classifyPath(
	scenario *ScenarioResult,
	pathName string,
	path PathResult,
	thresholds StatusThresholds,
) {
	if path.Requests.Attempted <= 0 {
		scenario.Status = higherStatus(
			scenario.Status,
			StatusInvalid,
		)

		scenario.StatusReasons = append(
			scenario.StatusReasons,
			StatusReason{
				Code:     "NO_REQUESTS_ATTEMPTED",
				Scenario: scenario.Name,
				Path:     pathName,
				Message:  "path attempted no DNS queries",
			},
		)

		return
	}

	failureStatus, failureReason :=
		classifyRate(
			"QUERY_FAILURE_RATE_EXCEEDED",
			scenario.Name,
			pathName,
			path.Rates.FailurePercent,
			thresholds.DegradedFailurePercent,
			thresholds.FailedFailurePercent,
			"query failure rate exceeded its acceptance threshold",
		)

	if failureReason != nil {
		scenario.Status = higherStatus(
			scenario.Status,
			failureStatus,
		)

		scenario.StatusReasons = append(
			scenario.StatusReasons,
			*failureReason,
		)
	}

	timeoutStatus, timeoutReason :=
		classifyRate(
			"QUERY_TIMEOUT_RATE_EXCEEDED",
			scenario.Name,
			pathName,
			path.Rates.TimeoutPercent,
			thresholds.DegradedTimeoutPercent,
			thresholds.FailedTimeoutPercent,
			"query timeout rate exceeded its acceptance threshold",
		)

	if timeoutReason != nil {
		scenario.Status = higherStatus(
			scenario.Status,
			timeoutStatus,
		)

		scenario.StatusReasons = append(
			scenario.StatusReasons,
			*timeoutReason,
		)
	}
}

func classifyRate(
	code string,
	scenarioName string,
	pathName string,
	observed float64,
	degradedThreshold float64,
	failedThreshold float64,
	message string,
) (string, *StatusReason) {
	if observed >= failedThreshold {
		return StatusFailed, &StatusReason{
			Code:      code,
			Scenario:  scenarioName,
			Path:      pathName,
			Observed:  float64Pointer(observed),
			Threshold: float64Pointer(failedThreshold),
			Message:   message,
		}
	}

	if observed >= degradedThreshold {
		return StatusDegraded, &StatusReason{
			Code:      code,
			Scenario:  scenarioName,
			Path:      pathName,
			Observed:  float64Pointer(observed),
			Threshold: float64Pointer(degradedThreshold),
			Message:   message,
		}
	}

	return StatusPassed, nil
}

func summarizeScenarios(
	scenarios []ScenarioResult,
	durationMilliseconds float64,
) BenchmarkSummary {
	summary := newBenchmarkSummary()

	if len(scenarios) == 0 {
		summary.Status = StatusInvalid

		summary.StatusReasons = append(
			summary.StatusReasons,
			StatusReason{
				Code:    "NO_SCENARIOS_EXECUTED",
				Message: "no benchmark scenarios were executed",
			},
		)

		summary.Notes = append(
			summary.Notes,
			"no benchmark scenarios were executed",
		)

		return summary
	}

	for _, scenario := range scenarios {
		accumulatePathSummary(
			&summary.Requests,
			scenario.Direct.Requests,
		)

		accumulatePathSummary(
			&summary.Requests,
			scenario.Forwarded.Requests,
		)

		accumulatePathSummary(
			&summary.
				Reliability.
				Direct.
				Requests,
			scenario.Direct.Requests,
		)

		accumulatePathSummary(
			&summary.
				Reliability.
				Forwarded.
				Requests,
			scenario.Forwarded.Requests,
		)

		accumulateScenarioStatus(
			&summary,
			scenario,
		)

		updateWorstScenario(
			&summary,
			scenario.Name,
			"direct",
			scenario.Direct,
		)

		updateWorstScenario(
			&summary,
			scenario.Name,
			"forwarded",
			scenario.Forwarded,
		)
	}

	summary.Rates = calculateRequestRates(
		summary.Requests,
	)

	summary.
		Reliability.
		Direct.
		Rates = calculateRequestRates(
		summary.
			Reliability.
			Direct.
			Requests,
	)

	summary.
		Reliability.
		Forwarded.
		Rates = calculateRequestRates(
		summary.
			Reliability.
			Forwarded.
			Requests,
	)

	summary.Throughput = calculateSummaryThroughput(
		summary.Requests,
		durationMilliseconds,
	)

	summary.Status = summarizeScenarioStatuses(
		summary.ScenarioResults,
	)

	if summary.Requests.Failed > 0 {
		summary.Notes = append(
			summary.Notes,
			"one or more DNS queries failed",
		)
	}

	if summary.Requests.Timeouts > 0 {
		summary.Notes = append(
			summary.Notes,
			"one or more DNS queries timed out",
		)
	}

	return summary
}

func newBenchmarkSummary() BenchmarkSummary {
	return BenchmarkSummary{
		Status:        StatusUnknown,
		StatusReasons: make([]StatusReason, 0),
		Notes:         make([]string, 0),

		ScenarioResults: ScenarioStatusSummary{
			Passed:   make([]string, 0),
			Degraded: make([]string, 0),
			Failed:   make([]string, 0),
			Invalid:  make([]string, 0),
		},
	}
}

func accumulatePathSummary(
	total *RequestCounts,
	path RequestCounts,
) {
	if total == nil {
		return
	}

	total.Attempted += path.Attempted
	total.Successful += path.Successful
	total.Failed += path.Failed
	total.Timeouts += path.Timeouts
	total.NonTimeoutFailures +=
		path.NonTimeoutFailures
}

func accumulateScenarioStatus(
	summary *BenchmarkSummary,
	scenario ScenarioResult,
) {
	if summary == nil {
		return
	}

	summary.StatusReasons = append(
		summary.StatusReasons,
		scenario.StatusReasons...,
	)

	switch scenario.Status {
	case StatusPassed:
		summary.ScenarioResults.Passed = append(
			summary.ScenarioResults.Passed,
			scenario.Name,
		)

	case StatusDegraded:
		summary.ScenarioResults.Degraded = append(
			summary.ScenarioResults.Degraded,
			scenario.Name,
		)

	case StatusFailed:
		summary.ScenarioResults.Failed = append(
			summary.ScenarioResults.Failed,
			scenario.Name,
		)

	default:
		summary.ScenarioResults.Invalid = append(
			summary.ScenarioResults.Invalid,
			scenario.Name,
		)
	}
}

func summarizeScenarioStatuses(
	statuses ScenarioStatusSummary,
) string {
	if len(statuses.Invalid) > 0 {
		return StatusInvalid
	}

	if len(statuses.Failed) > 0 {
		return StatusFailed
	}

	if len(statuses.Degraded) > 0 {
		return StatusDegraded
	}

	if len(statuses.Passed) > 0 {
		return StatusPassed
	}

	return StatusUnknown
}

func calculateSummaryThroughput(
	requests RequestCounts,
	durationMilliseconds float64,
) ThroughputMetrics {
	if durationMilliseconds <= 0 {
		return ThroughputMetrics{}
	}

	return calculateThroughput(
		requests,
		time.Duration(
			durationMilliseconds*
				float64(time.Millisecond),
		),
	)
}

func updateWorstScenario(
	summary *BenchmarkSummary,
	scenarioName string,
	pathName string,
	path PathResult,
) {
	if summary == nil {
		return
	}

	candidate := WorstScenarioSummary{
		Name: scenarioName,
		Path: pathName,

		FailurePercent: path.Rates.FailurePercent,

		TimeoutPercent: path.Rates.TimeoutPercent,

		P95SuccessfulLatencyMilliseconds: path.
			SuccessfulRequestLatencyMilliseconds.
			P95,
	}

	if summary.WorstScenario == nil ||
		isWorseScenario(
			candidate,
			*summary.WorstScenario,
		) {
		summary.WorstScenario = &candidate
	}
}

func isWorseScenario(
	candidate WorstScenarioSummary,
	current WorstScenarioSummary,
) bool {
	if candidate.FailurePercent !=
		current.FailurePercent {
		return candidate.FailurePercent >
			current.FailurePercent
	}

	if candidate.TimeoutPercent !=
		current.TimeoutPercent {
		return candidate.TimeoutPercent >
			current.TimeoutPercent
	}

	return candidate.
		P95SuccessfulLatencyMilliseconds >
		current.
			P95SuccessfulLatencyMilliseconds
}

func higherStatus(
	current string,
	candidate string,
) string {
	if statusSeverity(candidate) >
		statusSeverity(current) {
		return candidate
	}

	return current
}

func statusSeverity(status string) int {
	switch status {
	case StatusPassed:
		return 1

	case StatusDegraded:
		return 2

	case StatusFailed:
		return 3

	case StatusInvalid:
		return 4

	default:
		return 0
	}
}

func float64Pointer(value float64) *float64 {
	return &value
}

func queryTypeName(queryType uint16) string {
	if name, ok := dns.TypeToString[queryType]; ok {
		return name
	}

	return fmt.Sprintf("TYPE%d", queryType)
}
