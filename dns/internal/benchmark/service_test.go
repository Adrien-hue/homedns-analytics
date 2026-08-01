package benchmark

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type recordedReportRunner struct {
	configs []ReportRunConfig
	report  Report
	err     error
}

func (r *recordedReportRunner) Run(
	_ context.Context,
	config ReportRunConfig,
) (Report, error) {
	r.configs = append(
		r.configs,
		config,
	)

	return r.report, r.err
}

type recordedReportWriter struct {
	paths   []string
	reports []Report
	err     error
}

func (w *recordedReportWriter) Write(
	path string,
	report Report,
) error {
	w.paths = append(
		w.paths,
		path,
	)

	w.reports = append(
		w.reports,
		report,
	)

	return w.err
}

func validServiceConfig() ServiceConfig {
	reportConfig := validReportRunConfig()

	return ServiceConfig{
		Profile: DefaultProfile,

		ProfileCustomized: false,

		OutputDirectory: "benchmarks/runs",

		Suite: reportConfig.Suite,

		BenchmarkBinary: reportConfig.BenchmarkBinary,

		TargetService: reportConfig.TargetService,

		Environment: reportConfig.Environment,

		ResourceCollection: copyResourceCollectionConfig(
			reportConfig.ResourceCollection,
		),

		HealthAddress: reportConfig.HealthAddress,

		StatusThresholds: reportConfig.StatusThresholds,
	}
}

func TestServiceRun(
	t *testing.T,
) {
	t.Parallel()

	currentTime := time.Date(
		2026,
		time.July,
		28,
		20,
		34,
		22,
		0,
		time.FixedZone(
			"Europe/Paris",
			2*60*60,
		),
	)

	expectedReport := NewReport(
		"benchmark-20260728-183422",
		currentTime,
	)

	reportRunner := &recordedReportRunner{
		report: expectedReport,
	}

	reportWriter :=
		&recordedReportWriter{}

	service := &Service{
		reportRunner: reportRunner,

		writeReport: reportWriter.Write,

		now: func() time.Time {
			return currentTime
		},
	}

	config := validServiceConfig()

	result, err := service.Run(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf(
			"run benchmark service: %v",
			err,
		)
	}

	if len(reportRunner.configs) != 1 {
		t.Fatalf(
			"unexpected report execution count: got %d, want 1",
			len(reportRunner.configs),
		)
	}

	runConfig :=
		reportRunner.configs[0]

	if runConfig.ID !=
		"benchmark-20260728-183422" {
		t.Fatalf(
			"unexpected benchmark ID: got %q",
			runConfig.ID,
		)
	}

	if runConfig.Profile != config.Profile {
		t.Fatalf(
			"unexpected profile: got %q, want %q",
			runConfig.Profile,
			config.Profile,
		)
	}

	if runConfig.ProfileCustomized !=
		config.ProfileCustomized {
		t.Fatalf(
			"unexpected profile customization value: got %t, want %t",
			runConfig.ProfileCustomized,
			config.ProfileCustomized,
		)
	}

	if runConfig.Suite.DirectAddress !=
		config.Suite.DirectAddress {
		t.Fatalf(
			"unexpected direct address: got %q, want %q",
			runConfig.Suite.DirectAddress,
			config.Suite.DirectAddress,
		)
	}

	if runConfig.Suite.ForwardedAddress !=
		config.Suite.ForwardedAddress {
		t.Fatalf(
			"unexpected forwarded address: got %q, want %q",
			runConfig.Suite.ForwardedAddress,
			config.Suite.ForwardedAddress,
		)
	}

	if runConfig.BenchmarkBinary !=
		config.BenchmarkBinary {
		t.Fatal(
			"benchmark binary metadata was not forwarded",
		)
	}

	if runConfig.TargetService !=
		config.TargetService {
		t.Fatal(
			"target service metadata was not forwarded",
		)
	}

	if runConfig.Environment !=
		config.Environment {
		t.Fatal(
			"environment metadata was not forwarded",
		)
	}

	assertResourceCollectionForwarded(
		t,
		runConfig.ResourceCollection,
		config.ResourceCollection,
	)

	if runConfig.HealthAddress !=
		config.HealthAddress {
		t.Fatalf(
			"unexpected health address: got %q, want %q",
			runConfig.HealthAddress,
			config.HealthAddress,
		)
	}

	if runConfig.StatusThresholds !=
		config.StatusThresholds {
		t.Fatal(
			"status thresholds were not forwarded",
		)
	}

	expectedPath := filepath.Join(
		config.OutputDirectory,
		"benchmark-20260728-183422.json",
	)

	if result.Path != expectedPath {
		t.Fatalf(
			"unexpected result path: got %q, want %q",
			result.Path,
			expectedPath,
		)
	}

	if len(reportWriter.paths) != 1 {
		t.Fatalf(
			"unexpected report write count: got %d, want 1",
			len(reportWriter.paths),
		)
	}

	if reportWriter.paths[0] !=
		expectedPath {
		t.Fatalf(
			"unexpected written path: got %q, want %q",
			reportWriter.paths[0],
			expectedPath,
		)
	}

	if len(reportWriter.reports) != 1 {
		t.Fatalf(
			"unexpected written report count: got %d, want 1",
			len(reportWriter.reports),
		)
	}

	if reportWriter.reports[0].Report.ID !=
		expectedReport.Report.ID {
		t.Fatal(
			"unexpected report passed to writer",
		)
	}

	if result.Report.Report.ID !=
		expectedReport.Report.ID {
		t.Fatal(
			"unexpected report returned by service",
		)
	}
}

func TestServiceCopiesResourceCollectionConfig(
	t *testing.T,
) {
	t.Parallel()

	reportRunner := &recordedReportRunner{
		report: NewReport(
			"benchmark-copy",
			time.Now(),
		),
	}

	service := &Service{
		reportRunner: reportRunner,

		writeReport: func(
			string,
			Report,
		) error {
			return nil
		},

		now: time.Now,
	}

	config := validServiceConfig()

	originalInterval :=
		config.ResourceCollection.Interval

	_, err := service.Run(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf(
			"run benchmark service: %v",
			err,
		)
	}

	if len(reportRunner.configs) != 1 {
		t.Fatalf(
			"unexpected report execution count: got %d, want 1",
			len(reportRunner.configs),
		)
	}

	forwarded :=
		reportRunner.
			configs[0].
			ResourceCollection

	if forwarded == nil {
		t.Fatal(
			"resource collection configuration was not forwarded",
		)
	}

	forwarded.Interval =
		2 * time.Second

	if config.ResourceCollection.Interval !=
		originalInterval {
		t.Fatal(
			"service shared resource collection storage with report config",
		)
	}
}

func TestServiceSupportsDisabledResourceCollection(
	t *testing.T,
) {
	t.Parallel()

	reportRunner := &recordedReportRunner{
		report: NewReport(
			"benchmark-no-resources",
			time.Now(),
		),
	}

	service := &Service{
		reportRunner: reportRunner,

		writeReport: func(
			string,
			Report,
		) error {
			return nil
		},

		now: time.Now,
	}

	config := validServiceConfig()
	config.ResourceCollection = nil

	_, err := service.Run(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf(
			"run benchmark service: %v",
			err,
		)
	}

	if len(reportRunner.configs) != 1 {
		t.Fatalf(
			"unexpected report execution count: got %d, want 1",
			len(reportRunner.configs),
		)
	}

	if reportRunner.
		configs[0].
		ResourceCollection != nil {
		t.Fatal(
			"disabled resource collection was unexpectedly enabled",
		)
	}
}

func TestServiceReturnsReportRunnerError(
	t *testing.T,
) {
	t.Parallel()

	service := &Service{
		reportRunner: &recordedReportRunner{
			err: errors.New(
				"report failed",
			),
		},

		writeReport: func(
			string,
			Report,
		) error {
			t.Fatal(
				"report writer must not be called",
			)

			return nil
		},

		now: time.Now,
	}

	_, err := service.Run(
		context.Background(),
		validServiceConfig(),
	)
	if err == nil {
		t.Fatal(
			"expected report execution to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"execute benchmark report",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestServiceReturnsReportWriterError(
	t *testing.T,
) {
	t.Parallel()

	service := &Service{
		reportRunner: &recordedReportRunner{
			report: NewReport(
				"benchmark-test",
				time.Now(),
			),
		},

		writeReport: func(
			string,
			Report,
		) error {
			return errors.New(
				"write failed",
			)
		},

		now: time.Now,
	}

	_, err := service.Run(
		context.Background(),
		validServiceConfig(),
	)
	if err == nil {
		t.Fatal(
			"expected report writing to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"write benchmark report",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestServiceRejectsInvalidConfiguration(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name          string
		mutate        func(*ServiceConfig)
		expectedError string
	}{
		{
			name: "missing output directory",

			mutate: func(
				config *ServiceConfig,
			) {
				config.OutputDirectory = ""
			},

			expectedError: "benchmark output directory is required",
		},
		{
			name: "invalid suite",

			mutate: func(
				config *ServiceConfig,
			) {
				config.Suite.QueryCount = 0
			},

			expectedError: "invalid benchmark suite configuration",
		},
		{
			name: "invalid resource benchmark PID",

			mutate: func(
				config *ServiceConfig,
			) {
				config.
					ResourceCollection.
					BenchmarkPID = 0
			},

			expectedError: "invalid resource collection configuration",
		},
		{
			name: "invalid resource target PID",

			mutate: func(
				config *ServiceConfig,
			) {
				config.
					ResourceCollection.
					TargetPID = 0
			},

			expectedError: "invalid resource collection configuration",
		},
		{
			name: "invalid resource interval",

			mutate: func(
				config *ServiceConfig,
			) {
				config.
					ResourceCollection.
					Interval = 0
			},

			expectedError: "invalid resource collection configuration",
		},
		{
			name: "invalid status thresholds",

			mutate: func(
				config *ServiceConfig,
			) {
				config.StatusThresholds =
					StatusThresholds{
						DegradedFailurePercent: 10,

						FailedFailurePercent: 5,

						DegradedTimeoutPercent: 1,

						FailedTimeoutPercent: 5,
					}
			},

			expectedError: "invalid benchmark status thresholds",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				config :=
					validServiceConfig()

				test.mutate(
					&config,
				)

				service := &Service{
					reportRunner: &recordedReportRunner{},

					writeReport: func(
						string,
						Report,
					) error {
						return nil
					},

					now: time.Now,
				}

				_, err := service.Run(
					context.Background(),
					config,
				)
				if err == nil {
					t.Fatal(
						"expected invalid configuration to fail",
					)
				}

				if !strings.Contains(
					err.Error(),
					test.expectedError,
				) {
					t.Fatalf(
						"unexpected error: got %q, want it to contain %q",
						err.Error(),
						test.expectedError,
					)
				}
			},
		)
	}
}

func TestServiceRejectsNilContext(
	t *testing.T,
) {
	t.Parallel()

	service := &Service{
		reportRunner: &recordedReportRunner{},

		writeReport: func(
			string,
			Report,
		) error {
			return nil
		},

		now: time.Now,
	}

	_, err := service.Run(
		nil,
		validServiceConfig(),
	)
	if err == nil {
		t.Fatal(
			"expected nil context to fail",
		)
	}
}

func TestServiceRejectsNilReceiver(
	t *testing.T,
) {
	t.Parallel()

	var service *Service

	_, err := service.Run(
		context.Background(),
		validServiceConfig(),
	)
	if err == nil {
		t.Fatal(
			"expected nil service to fail",
		)
	}
}

func TestServiceRejectsMissingReportRunner(
	t *testing.T,
) {
	t.Parallel()

	service := &Service{
		writeReport: WriteJSONFile,

		now: time.Now,
	}

	_, err := service.Run(
		context.Background(),
		validServiceConfig(),
	)
	if err == nil {
		t.Fatal(
			"expected missing report runner to fail",
		)
	}
}

func TestServiceRejectsMissingReportWriter(
	t *testing.T,
) {
	t.Parallel()

	service := &Service{
		reportRunner: &recordedReportRunner{},

		now: time.Now,
	}

	_, err := service.Run(
		context.Background(),
		validServiceConfig(),
	)
	if err == nil {
		t.Fatal(
			"expected missing report writer to fail",
		)
	}
}

func TestServiceRejectsMissingClock(
	t *testing.T,
) {
	t.Parallel()

	service := &Service{
		reportRunner: &recordedReportRunner{},

		writeReport: WriteJSONFile,
	}

	_, err := service.Run(
		context.Background(),
		validServiceConfig(),
	)
	if err == nil {
		t.Fatal(
			"expected missing service clock to fail",
		)
	}
}

func TestBenchmarkIDFromTime(
	t *testing.T,
) {
	t.Parallel()

	value := time.Date(
		2026,
		time.July,
		28,
		22,
		15,
		30,
		0,
		time.FixedZone(
			"Europe/Paris",
			2*60*60,
		),
	)

	got := benchmarkIDFromTime(
		value,
	)

	want :=
		"benchmark-20260728-201530"

	if got != want {
		t.Fatalf(
			"unexpected benchmark ID: got %q, want %q",
			got,
			want,
		)
	}
}

func TestNewService(
	t *testing.T,
) {
	t.Parallel()

	service := NewService()

	if service == nil {
		t.Fatal(
			"expected benchmark service",
		)
	}

	if service.reportRunner == nil {
		t.Fatal(
			"expected report runner",
		)
	}

	if service.writeReport == nil {
		t.Fatal(
			"expected report writer",
		)
	}

	if service.now == nil {
		t.Fatal(
			"expected service clock",
		)
	}
}

func TestServiceRunForwardsProgressReporter(
	t *testing.T,
) {
	t.Parallel()

	reporterCalled := false

	reporter := ProgressReporterFunc(
		func(event ProgressEvent) {
			reporterCalled = true

			if event.Type !=
				ProgressEventBenchmarkStarted {
				t.Fatalf(
					"unexpected event type: got %q, want %q",
					event.Type,
					ProgressEventBenchmarkStarted,
				)
			}
		},
	)

	reportRunner := &recordedReportRunner{
		report: NewReport(
			"benchmark-20260801-103000",
			time.Date(
				2026,
				time.August,
				1,
				10,
				30,
				0,
				0,
				time.UTC,
			),
		),
	}

	reportWriter :=
		&recordedReportWriter{}

	currentTime := time.Date(
		2026,
		time.August,
		1,
		10,
		30,
		0,
		0,
		time.UTC,
	)

	service := &Service{
		reportRunner: reportRunner,

		writeReport: reportWriter.Write,

		now: func() time.Time {
			return currentTime
		},
	}

	config := validServiceConfig()

	config.Suite.ProgressReporter =
		reporter

	_, err := service.Run(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf(
			"run benchmark service: %v",
			err,
		)
	}

	if len(reportRunner.configs) != 1 {
		t.Fatalf(
			"unexpected report execution count: got %d, want 1",
			len(reportRunner.configs),
		)
	}

	forwardedReporter :=
		reportRunner.
			configs[0].
			Suite.
			ProgressReporter

	if forwardedReporter == nil {
		t.Fatal(
			"progress reporter was not forwarded to report execution",
		)
	}

	forwardedReporter.ReportProgress(
		ProgressEvent{
			Type: ProgressEventBenchmarkStarted,
		},
	)

	if !reporterCalled {
		t.Fatal(
			"forwarded progress reporter was not called",
		)
	}
}

func TestCopyResourceCollectionConfig(
	t *testing.T,
) {
	t.Parallel()

	original := &ResourceCollectionConfig{
		BenchmarkPID: 100,

		TargetPID: 200,

		Interval: time.Second,
	}

	copied :=
		copyResourceCollectionConfig(
			original,
		)

	if copied == nil {
		t.Fatal(
			"expected copied resource configuration",
		)
	}

	if copied == original {
		t.Fatal(
			"resource configuration pointer was not copied",
		)
	}

	if *copied != *original {
		t.Fatalf(
			"unexpected copied configuration: got %+v, want %+v",
			*copied,
			*original,
		)
	}

	copied.Interval =
		2 * time.Second

	if original.Interval !=
		time.Second {
		t.Fatal(
			"copy mutation affected original configuration",
		)
	}

	if copyResourceCollectionConfig(nil) != nil {
		t.Fatal(
			"nil resource configuration should remain nil",
		)
	}
}

func assertResourceCollectionForwarded(
	t *testing.T,
	actual *ResourceCollectionConfig,
	expected *ResourceCollectionConfig,
) {
	t.Helper()

	if expected == nil {
		if actual != nil {
			t.Fatal(
				"unexpected resource collection configuration",
			)
		}

		return
	}

	if actual == nil {
		t.Fatal(
			"resource collection configuration was not forwarded",
		)
	}

	if actual.BenchmarkPID !=
		expected.BenchmarkPID {
		t.Fatalf(
			"unexpected benchmark PID: got %d, want %d",
			actual.BenchmarkPID,
			expected.BenchmarkPID,
		)
	}

	if actual.TargetPID !=
		expected.TargetPID {
		t.Fatalf(
			"unexpected target PID: got %d, want %d",
			actual.TargetPID,
			expected.TargetPID,
		)
	}

	if actual.Interval !=
		expected.Interval {
		t.Fatalf(
			"unexpected collection interval: got %s, want %s",
			actual.Interval,
			expected.Interval,
		)
	}

	if actual == expected {
		t.Fatal(
			"resource collection config shares caller storage",
		)
	}
}
