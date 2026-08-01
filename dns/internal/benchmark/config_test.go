package benchmark

import (
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func validPathConfig() PathConfig {
	return PathConfig{
		Address:     "127.0.0.1:5300",
		Protocol:    ProtocolUDP,
		QueryNames:  []string{"example.com."},
		QueryType:   dns.TypeA,
		QueryCount:  100,
		Concurrency: 10,
		Timeout:     3 * time.Second,

		BenchmarkStartedAt: time.Date(
			2026,
			time.August,
			1,
			10,
			0,
			0,
			0,
			time.UTC,
		),

		ScenarioName:  "udp-sequential",
		ScenarioIndex: 1,
		ScenarioCount: 4,

		Phase: ProgressPhaseDirect,
	}
}

func TestPathConfigValidate(
	t *testing.T,
) {
	t.Parallel()

	if err := validPathConfig().Validate(); err != nil {
		t.Fatalf(
			"validate path config: %v",
			err,
		)
	}
}

func TestPathConfigValidateAcceptsProgressReporter(
	t *testing.T,
) {
	t.Parallel()

	called := false

	config := validPathConfig()

	config.ProgressReporter =
		ProgressReporterFunc(
			func(event ProgressEvent) {
				called = true

				if event.Type !=
					ProgressEventPhaseStarted {
					t.Fatalf(
						"unexpected event type: got %q, want %q",
						event.Type,
						ProgressEventPhaseStarted,
					)
				}
			},
		)

	if err := config.Validate(); err != nil {
		t.Fatalf(
			"validate path config with reporter: %v",
			err,
		)
	}

	config.ProgressReporter.ReportProgress(
		ProgressEvent{
			Type: ProgressEventPhaseStarted,
		},
	)

	if !called {
		t.Fatal(
			"progress reporter was not called",
		)
	}
}

func TestPathConfigValidateAcceptsNilProgressReporter(
	t *testing.T,
) {
	t.Parallel()

	config := validPathConfig()
	config.ProgressReporter = nil

	if err := config.Validate(); err != nil {
		t.Fatalf(
			"validate path config without reporter: %v",
			err,
		)
	}
}

func TestPathConfigValidateAcceptsEmptyProgressMetadata(
	t *testing.T,
) {
	t.Parallel()

	config := validPathConfig()

	config.BenchmarkStartedAt =
		time.Time{}

	config.ScenarioName = ""
	config.ScenarioIndex = 0
	config.ScenarioCount = 0
	config.Phase = ""

	if err := config.Validate(); err != nil {
		t.Fatalf(
			"progress metadata must remain optional: %v",
			err,
		)
	}
}

func TestPathConfigProgressMetadata(
	t *testing.T,
) {
	t.Parallel()

	config := validPathConfig()

	if config.ScenarioName !=
		"udp-sequential" {
		t.Fatalf(
			"unexpected scenario name: got %q, want %q",
			config.ScenarioName,
			"udp-sequential",
		)
	}

	if config.ScenarioIndex != 1 {
		t.Fatalf(
			"unexpected scenario index: got %d, want 1",
			config.ScenarioIndex,
		)
	}

	if config.ScenarioCount != 4 {
		t.Fatalf(
			"unexpected scenario count: got %d, want 4",
			config.ScenarioCount,
		)
	}

	if config.Phase !=
		ProgressPhaseDirect {
		t.Fatalf(
			"unexpected phase: got %q, want %q",
			config.Phase,
			ProgressPhaseDirect,
		)
	}

	if config.BenchmarkStartedAt.IsZero() {
		t.Fatal(
			"benchmark start time is zero",
		)
	}
}

func TestPathConfigValidateRejectsInvalidValues(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name          string
		mutate        func(*PathConfig)
		expectedError string
	}{
		{
			name: "missing address",
			mutate: func(config *PathConfig) {
				config.Address = ""
			},
			expectedError: "DNS target address is required",
		},
		{
			name: "address without port",
			mutate: func(config *PathConfig) {
				config.Address = "127.0.0.1"
			},
			expectedError: "invalid DNS target address",
		},
		{
			name: "unsupported protocol",
			mutate: func(config *PathConfig) {
				config.Protocol = "https"
			},
			expectedError: "unsupported DNS protocol",
		},
		{
			name: "missing query names",
			mutate: func(config *PathConfig) {
				config.QueryNames = nil
			},
			expectedError: "at least one query name is required",
		},
		{
			name: "empty query name",
			mutate: func(config *PathConfig) {
				config.QueryNames =
					[]string{""}
			},
			expectedError: "query names cannot be empty",
		},
		{
			name: "unsupported query type",
			mutate: func(config *PathConfig) {
				config.QueryType = 65535
			},
			expectedError: "unsupported DNS query type",
		},
		{
			name: "zero query count",
			mutate: func(config *PathConfig) {
				config.QueryCount = 0
			},
			expectedError: "query count must be greater than zero",
		},
		{
			name: "zero concurrency",
			mutate: func(config *PathConfig) {
				config.Concurrency = 0
			},
			expectedError: "concurrency must be greater than zero",
		},
		{
			name: "concurrency exceeds query count",
			mutate: func(config *PathConfig) {
				config.QueryCount = 5
				config.Concurrency = 10
			},
			expectedError: "concurrency cannot exceed query count",
		},
		{
			name: "zero timeout",
			mutate: func(config *PathConfig) {
				config.Timeout = 0
			},
			expectedError: "timeout must be greater than zero",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				config := validPathConfig()
				test.mutate(&config)

				err := config.Validate()
				if err == nil {
					t.Fatal(
						"expected validation to fail",
					)
				}

				if test.expectedError != "" &&
					!containsError(
						err,
						test.expectedError,
					) {
					t.Fatalf(
						"unexpected validation error: got %q, want it to contain %q",
						err.Error(),
						test.expectedError,
					)
				}
			},
		)
	}
}

func TestPathConfigValidateDoesNotMutateQueryNames(
	t *testing.T,
) {
	t.Parallel()

	config := validPathConfig()

	original := append(
		[]string(nil),
		config.QueryNames...,
	)

	if err := config.Validate(); err != nil {
		t.Fatalf(
			"validate path config: %v",
			err,
		)
	}

	if len(config.QueryNames) != len(original) {
		t.Fatalf(
			"validation changed query-name count: got %d, want %d",
			len(config.QueryNames),
			len(original),
		)
	}

	for index := range original {
		if config.QueryNames[index] !=
			original[index] {
			t.Fatalf(
				"validation changed query name at index %d: got %q, want %q",
				index,
				config.QueryNames[index],
				original[index],
			)
		}
	}
}

func containsError(
	err error,
	expected string,
) bool {
	if err == nil {
		return false
	}

	return strings.Contains(
		err.Error(),
		expected,
	)
}
