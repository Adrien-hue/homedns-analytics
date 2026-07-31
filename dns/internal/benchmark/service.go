package benchmark

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"
)

const defaultReportFileExtension = ".json"

// ServiceConfig contains the inputs required for one benchmark execution.
type ServiceConfig struct {
	OutputDirectory string

	Suite SuiteConfig

	BenchmarkBinary BenchmarkBinaryMetadata
	TargetService   TargetServiceMetadata
	Environment     Environment
	Resources       ResourceSummary

	HealthAddress string

	StatusThresholds StatusThresholds
}

// Validate verifies the benchmark service configuration.
func (c ServiceConfig) Validate() error {
	if c.OutputDirectory == "" {
		return errors.New(
			"benchmark output directory is required",
		)
	}

	if err := c.Suite.Validate(); err != nil {
		return fmt.Errorf(
			"invalid benchmark suite configuration: %w",
			err,
		)
	}

	if err := validateStatusThresholds(
		resolvedStatusThresholds(
			c.StatusThresholds,
		),
	); err != nil {
		return fmt.Errorf(
			"invalid benchmark status thresholds: %w",
			err,
		)
	}

	return nil
}

// ServiceResult contains the completed report and its persisted location.
type ServiceResult struct {
	Report Report
	Path   string
}

type reportExecutor interface {
	Run(
		context.Context,
		ReportRunConfig,
	) (Report, error)
}

type reportFileWriter func(
	string,
	Report,
) error

// Service orchestrates complete benchmark executions.
type Service struct {
	reportRunner reportExecutor
	writeReport  reportFileWriter
	now          func() time.Time
}

// NewService creates a benchmark service using the production dependencies.
func NewService() *Service {
	return &Service{
		reportRunner: NewReportRunner(),
		writeReport:  WriteJSONFile,
		now:          time.Now,
	}
}

// Run executes a complete benchmark and persists its JSON report.
func (s *Service) Run(
	ctx context.Context,
	config ServiceConfig,
) (ServiceResult, error) {
	if s == nil {
		return ServiceResult{}, errors.New(
			"benchmark service is not initialized",
		)
	}

	if s.reportRunner == nil {
		return ServiceResult{}, errors.New(
			"benchmark report runner is not initialized",
		)
	}

	if s.writeReport == nil {
		return ServiceResult{}, errors.New(
			"benchmark report writer is not initialized",
		)
	}

	if s.now == nil {
		return ServiceResult{}, errors.New(
			"benchmark service clock is not initialized",
		)
	}

	if ctx == nil {
		return ServiceResult{}, errors.New(
			"benchmark context is required",
		)
	}

	if err := config.Validate(); err != nil {
		return ServiceResult{}, fmt.Errorf(
			"validate benchmark service configuration: %w",
			err,
		)
	}

	benchmarkID := benchmarkIDFromTime(
		s.now(),
	)

	report, err := s.reportRunner.Run(
		ctx,
		ReportRunConfig{
			ID: benchmarkID,

			Suite: config.Suite,

			BenchmarkBinary: config.BenchmarkBinary,

			TargetService: config.TargetService,

			Environment: config.Environment,

			Resources: config.Resources,

			HealthAddress: config.HealthAddress,

			StatusThresholds: config.StatusThresholds,
		},
	)
	if err != nil {
		return ServiceResult{}, fmt.Errorf(
			"execute benchmark report: %w",
			err,
		)
	}

	reportPath := filepath.Join(
		config.OutputDirectory,
		benchmarkID+defaultReportFileExtension,
	)

	if err := s.writeReport(
		reportPath,
		report,
	); err != nil {
		return ServiceResult{}, fmt.Errorf(
			"write benchmark report: %w",
			err,
		)
	}

	return ServiceResult{
		Report: report,
		Path:   reportPath,
	}, nil
}

func benchmarkIDFromTime(
	value time.Time,
) string {
	return "benchmark-" +
		value.UTC().Format(
			"20060102-150405",
		)
}
