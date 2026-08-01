package app

import (
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"

	"github.com/Adrien-hue/homedns-analytics/dns/internal/benchmark"
	"github.com/Adrien-hue/homedns-analytics/dns/internal/config"
)

func TestResolveBenchmarkProfile(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name     string
		profile  string
		expected BenchmarkProfile
	}{
		{
			name:    "quick",
			profile: BenchmarkProfileQuick,
			expected: BenchmarkProfile{
				Name:              BenchmarkProfileQuick,
				QueryCount:        100,
				WarmupQueries:     10,
				ConcurrentWorkers: 10,
			},
		},
		{
			name:    "validation",
			profile: BenchmarkProfileValidation,
			expected: BenchmarkProfile{
				Name:              BenchmarkProfileValidation,
				QueryCount:        1_000,
				WarmupQueries:     25,
				ConcurrentWorkers: 10,
			},
		},
		{
			name:    "endurance",
			profile: BenchmarkProfileEndurance,
			expected: BenchmarkProfile{
				Name:              BenchmarkProfileEndurance,
				QueryCount:        17_000,
				WarmupQueries:     50,
				ConcurrentWorkers: 10,
			},
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				actual, err := ResolveBenchmarkProfile(
					test.profile,
				)
				if err != nil {
					t.Fatalf(
						"resolve benchmark profile: %v",
						err,
					)
				}

				if actual != test.expected {
					t.Fatalf(
						"unexpected profile: got %+v, want %+v",
						actual,
						test.expected,
					)
				}
			},
		)
	}
}

func TestResolveBenchmarkProfileRejectsUnknownProfile(
	t *testing.T,
) {
	t.Parallel()

	_, err := ResolveBenchmarkProfile(
		"unknown",
	)
	if err == nil {
		t.Fatal(
			"expected unknown benchmark profile to fail",
		)
	}

	for _, expected := range []string{
		`unsupported benchmark profile "unknown"`,
		BenchmarkProfileQuick,
		BenchmarkProfileValidation,
		BenchmarkProfileEndurance,
	} {
		if !strings.Contains(
			err.Error(),
			expected,
		) {
			t.Fatalf(
				"error does not contain %q: %v",
				expected,
				err,
			)
		}
	}
}

func TestDefaultBenchmarkOptions(
	t *testing.T,
) {
	t.Parallel()

	options := DefaultBenchmarkOptions()

	if options.ConfigPath != "" {
		t.Fatalf(
			"default config path should be empty, got %q",
			options.ConfigPath,
		)
	}

	if options.Profile !=
		DefaultBenchmarkProfile {
		t.Fatalf(
			"unexpected default profile: got %q, want %q",
			options.Profile,
			DefaultBenchmarkProfile,
		)
	}

	if options.ProfileCustomized {
		t.Fatal(
			"default profile must not be customized",
		)
	}

	if options.OutputDirectory !=
		DefaultBenchmarkOutputDirectory {
		t.Fatalf(
			"unexpected output directory: got %q, want %q",
			options.OutputDirectory,
			DefaultBenchmarkOutputDirectory,
		)
	}

	if options.QueryCount != 100 {
		t.Fatalf(
			"unexpected query count: got %d, want 100",
			options.QueryCount,
		)
	}

	if options.WarmupQueries != 10 {
		t.Fatalf(
			"unexpected warmup count: got %d, want 10",
			options.WarmupQueries,
		)
	}

	if options.ConcurrentWorkers != 10 {
		t.Fatalf(
			"unexpected concurrency: got %d, want 10",
			options.ConcurrentWorkers,
		)
	}

	if options.Timeout != 0 {
		t.Fatalf(
			"expected unresolved timeout, got %s",
			options.Timeout,
		)
	}

	if options.QueryType != dns.TypeA {
		t.Fatalf(
			"unexpected query type: got %d, want %d",
			options.QueryType,
			dns.TypeA,
		)
	}

	expectedNames := []string{
		"example.com.",
		"cloudflare.com.",
		"google.com.",
		"github.com.",
	}

	if len(options.QueryNames) !=
		len(expectedNames) {
		t.Fatalf(
			"unexpected query-name count: got %d, want %d",
			len(options.QueryNames),
			len(expectedNames),
		)
	}

	for index, expected := range expectedNames {
		if options.QueryNames[index] != expected {
			t.Fatalf(
				"unexpected query name at index %d: got %q, want %q",
				index,
				options.QueryNames[index],
				expected,
			)
		}
	}
}

func TestBenchmarkOptionsResolveInheritsTimeout(
	t *testing.T,
) {
	t.Parallel()

	options := DefaultBenchmarkOptions()

	cfg := config.Config{}
	cfg.Upstream.Timeout = 5 * time.Second

	resolved := options.Resolve(cfg)

	if resolved.Timeout !=
		5*time.Second {
		t.Fatalf(
			"unexpected resolved timeout: got %s, want %s",
			resolved.Timeout,
			5*time.Second,
		)
	}

	if options.Timeout != 0 {
		t.Fatalf(
			"Resolve modified the original options: %s",
			options.Timeout,
		)
	}
}

func TestBenchmarkOptionsResolveKeepsExplicitTimeout(
	t *testing.T,
) {
	t.Parallel()

	options := DefaultBenchmarkOptions()
	options.Timeout = 2 * time.Second

	cfg := config.Config{}
	cfg.Upstream.Timeout = 5 * time.Second

	resolved := options.Resolve(cfg)

	if resolved.Timeout !=
		2*time.Second {
		t.Fatalf(
			"unexpected resolved timeout: got %s, want %s",
			resolved.Timeout,
			2*time.Second,
		)
	}
}

func TestBenchmarkOptionsResolveKeepsProgressReporter(
	t *testing.T,
) {
	t.Parallel()

	called := false

	reporter := benchmark.ProgressReporterFunc(
		func(benchmark.ProgressEvent) {
			called = true
		},
	)

	options := DefaultBenchmarkOptions()
	options.ProgressReporter = reporter

	cfg := config.Config{}
	cfg.Upstream.Timeout = 3 * time.Second

	resolved := options.Resolve(cfg)

	if resolved.ProgressReporter == nil {
		t.Fatal(
			"resolved options lost the progress reporter",
		)
	}

	resolved.ProgressReporter.ReportProgress(
		benchmark.ProgressEvent{
			Type: benchmark.ProgressEventBenchmarkStarted,
		},
	)

	if !called {
		t.Fatal(
			"resolved progress reporter was not called",
		)
	}
}

func TestBenchmarkOptionsValidate(
	t *testing.T,
) {
	t.Parallel()

	options := DefaultBenchmarkOptions()
	options.ConfigPath = "config.example.yaml"
	options.Timeout = 3 * time.Second

	if err := options.Validate(); err != nil {
		t.Fatalf(
			"valid benchmark options rejected: %v",
			err,
		)
	}
}

func TestBenchmarkOptionsValidateAcceptsAAAA(
	t *testing.T,
) {
	t.Parallel()

	options := DefaultBenchmarkOptions()
	options.ConfigPath = "config.example.yaml"
	options.Timeout = 3 * time.Second
	options.QueryType = dns.TypeAAAA

	if err := options.Validate(); err != nil {
		t.Fatalf(
			"valid AAAA benchmark options rejected: %v",
			err,
		)
	}
}

func TestBenchmarkOptionsValidateRejectsInvalidOptions(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name          string
		mutate        func(*BenchmarkOptions)
		expectedError string
	}{
		{
			name: "unknown profile",
			mutate: func(
				options *BenchmarkOptions,
			) {
				options.Profile = "unknown"
			},
			expectedError: "unsupported benchmark profile",
		},
		{
			name: "missing configuration path",
			mutate: func(
				options *BenchmarkOptions,
			) {
				options.ConfigPath = ""
			},
			expectedError: "benchmark configuration path is required",
		},
		{
			name: "missing output directory",
			mutate: func(
				options *BenchmarkOptions,
			) {
				options.OutputDirectory = ""
			},
			expectedError: "benchmark output directory is required",
		},
		{
			name: "zero query count",
			mutate: func(
				options *BenchmarkOptions,
			) {
				options.QueryCount = 0
			},
			expectedError: "benchmark query count must be greater than zero",
		},
		{
			name: "negative warmup",
			mutate: func(
				options *BenchmarkOptions,
			) {
				options.WarmupQueries = -1
			},
			expectedError: "benchmark warmup query count cannot be negative",
		},
		{
			name: "zero concurrency",
			mutate: func(
				options *BenchmarkOptions,
			) {
				options.ConcurrentWorkers = 0
			},
			expectedError: "benchmark concurrency must be greater than zero",
		},
		{
			name: "concurrency exceeds queries",
			mutate: func(
				options *BenchmarkOptions,
			) {
				options.QueryCount = 5
				options.ConcurrentWorkers = 6
			},
			expectedError: "cannot exceed query count",
		},
		{
			name: "zero timeout",
			mutate: func(
				options *BenchmarkOptions,
			) {
				options.Timeout = 0
			},
			expectedError: "benchmark timeout must be greater than zero",
		},
		{
			name: "missing query names",
			mutate: func(
				options *BenchmarkOptions,
			) {
				options.QueryNames = nil
			},
			expectedError: "at least one benchmark query name is required",
		},
		{
			name: "blank query name",
			mutate: func(
				options *BenchmarkOptions,
			) {
				options.QueryNames =
					[]string{"example.com", " "}
			},
			expectedError: "benchmark query name at index 1 is empty",
		},
		{
			name: "unsupported query type",
			mutate: func(
				options *BenchmarkOptions,
			) {
				options.QueryType =
					dns.TypeMX
			},
			expectedError: "unsupported benchmark query type",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				options := DefaultBenchmarkOptions()

				options.ConfigPath =
					"config.example.yaml"

				options.Timeout =
					3 * time.Second

				test.mutate(&options)

				err := options.Validate()
				if err == nil {
					t.Fatal(
						"expected benchmark validation to fail",
					)
				}

				if !strings.Contains(
					err.Error(),
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

func TestNormalizeBenchmarkQueryNames(
	t *testing.T,
) {
	t.Parallel()

	actual, err :=
		normalizeBenchmarkQueryNames(
			[]string{
				" example.com ",
				"cloudflare.com.",
				"GOOGLE.com",
			},
		)
	if err != nil {
		t.Fatalf(
			"normalize benchmark query names: %v",
			err,
		)
	}

	expected := []string{
		"example.com.",
		"cloudflare.com.",
		"GOOGLE.com.",
	}

	if len(actual) != len(expected) {
		t.Fatalf(
			"unexpected normalized name count: got %d, want %d",
			len(actual),
			len(expected),
		)
	}

	for index := range expected {
		if actual[index] != expected[index] {
			t.Fatalf(
				"unexpected normalized name at index %d: got %q, want %q",
				index,
				actual[index],
				expected[index],
			)
		}
	}
}

func TestNormalizeBenchmarkQueryNamesRejectsEmptyInput(
	t *testing.T,
) {
	t.Parallel()

	_, err := normalizeBenchmarkQueryNames(nil)
	if err == nil {
		t.Fatal(
			"expected empty query-name input to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"at least one benchmark query name is required",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestNormalizeBenchmarkQueryNamesRejectsBlankName(
	t *testing.T,
) {
	t.Parallel()

	_, err := normalizeBenchmarkQueryNames(
		[]string{
			"example.com",
			" ",
		},
	)
	if err == nil {
		t.Fatal(
			"expected blank query name to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"benchmark query name at index 1 is empty",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestNormalizeBenchmarkQueryNamesReturnsIndependentSlice(
	t *testing.T,
) {
	t.Parallel()

	input := []string{
		"example.com",
	}

	normalized, err :=
		normalizeBenchmarkQueryNames(input)
	if err != nil {
		t.Fatalf(
			"normalize benchmark query names: %v",
			err,
		)
	}

	normalized[0] = "modified.example."

	if input[0] != "example.com" {
		t.Fatal(
			"normalization modified the input slice",
		)
	}
}
