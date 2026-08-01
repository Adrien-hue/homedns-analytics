package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/miekg/dns"
)

func TestRunPrintsVersion(
	t *testing.T,
) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(
		[]string{"--version"},
		&stdout,
		&stderr,
	)

	if exitCode != exitSuccess {
		t.Fatalf(
			"unexpected exit code: got %d, want %d\nstderr:\n%s",
			exitCode,
			exitSuccess,
			stderr.String(),
		)
	}

	if stdout.Len() == 0 {
		t.Fatal(
			"expected version output",
		)
	}

	if stderr.Len() != 0 {
		t.Fatalf(
			"unexpected stderr output:\n%s",
			stderr.String(),
		)
	}
}

func TestRunServerRequiresConfiguration(
	t *testing.T,
) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(
		nil,
		&stdout,
		&stderr,
	)

	if exitCode != exitInvalidArgs {
		t.Fatalf(
			"unexpected exit code: got %d, want %d",
			exitCode,
			exitInvalidArgs,
		)
	}

	assertCLIOutputContains(
		t,
		stderr.String(),
		"configuration path is required",
	)
}

func TestRunServerRejectsUnexpectedArguments(
	t *testing.T,
) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(
		[]string{"unexpected"},
		&stdout,
		&stderr,
	)

	if exitCode != exitInvalidArgs {
		t.Fatalf(
			"unexpected exit code: got %d, want %d",
			exitCode,
			exitInvalidArgs,
		)
	}

	assertCLIOutputContains(
		t,
		stderr.String(),
		"unexpected arguments",
	)
}

func TestRunBenchmarkRequiresConfiguration(
	t *testing.T,
) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(
		[]string{"benchmark"},
		&stdout,
		&stderr,
	)

	if exitCode != exitInvalidArgs {
		t.Fatalf(
			"unexpected exit code: got %d, want %d",
			exitCode,
			exitInvalidArgs,
		)
	}

	assertCLIOutputContains(
		t,
		stderr.String(),
		"configuration path is required",
	)
}

func TestRunBenchmarkRejectsUnknownProfile(
	t *testing.T,
) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(
		[]string{
			"benchmark",
			"--config",
			"config.example.yaml",
			"--profile",
			"unknown",
		},
		&stdout,
		&stderr,
	)

	if exitCode != exitInvalidArgs {
		t.Fatalf(
			"unexpected exit code: got %d, want %d",
			exitCode,
			exitInvalidArgs,
		)
	}

	assertCLIOutputContains(
		t,
		stderr.String(),
		"invalid benchmark profile",
	)

	assertCLIOutputContains(
		t,
		stderr.String(),
		"unsupported benchmark profile",
	)
}

func TestRunBenchmarkRejectsZeroQueries(
	t *testing.T,
) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(
		[]string{
			"benchmark",
			"--config",
			"config.example.yaml",
			"--queries",
			"0",
		},
		&stdout,
		&stderr,
	)

	if exitCode != exitInvalidArgs {
		t.Fatalf(
			"unexpected exit code: got %d, want %d",
			exitCode,
			exitInvalidArgs,
		)
	}

	assertCLIOutputContains(
		t,
		stderr.String(),
		"benchmark query count must be greater than zero",
	)
}

func TestRunBenchmarkRejectsNegativeWarmup(
	t *testing.T,
) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(
		[]string{
			"benchmark",
			"--config",
			"config.example.yaml",
			"--warmup",
			"-1",
		},
		&stdout,
		&stderr,
	)

	if exitCode != exitInvalidArgs {
		t.Fatalf(
			"unexpected exit code: got %d, want %d",
			exitCode,
			exitInvalidArgs,
		)
	}

	assertCLIOutputContains(
		t,
		stderr.String(),
		"benchmark warmup query count cannot be negative",
	)
}

func TestRunBenchmarkRejectsZeroConcurrency(
	t *testing.T,
) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(
		[]string{
			"benchmark",
			"--config",
			"config.example.yaml",
			"--concurrency",
			"0",
		},
		&stdout,
		&stderr,
	)

	if exitCode != exitInvalidArgs {
		t.Fatalf(
			"unexpected exit code: got %d, want %d",
			exitCode,
			exitInvalidArgs,
		)
	}

	assertCLIOutputContains(
		t,
		stderr.String(),
		"benchmark concurrency must be greater than zero",
	)
}

func TestRunBenchmarkRejectsConcurrencyAboveQueries(
	t *testing.T,
) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(
		[]string{
			"benchmark",
			"--config",
			"config.example.yaml",
			"--queries",
			"5",
			"--concurrency",
			"6",
		},
		&stdout,
		&stderr,
	)

	if exitCode != exitInvalidArgs {
		t.Fatalf(
			"unexpected exit code: got %d, want %d",
			exitCode,
			exitInvalidArgs,
		)
	}

	assertCLIOutputContains(
		t,
		stderr.String(),
		"benchmark concurrency 6 cannot exceed query count 5",
	)
}

func TestRunBenchmarkRejectsZeroTimeout(
	t *testing.T,
) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(
		[]string{
			"benchmark",
			"--config",
			"config.example.yaml",
			"--timeout",
			"0s",
		},
		&stdout,
		&stderr,
	)

	if exitCode != exitInvalidArgs {
		t.Fatalf(
			"unexpected exit code: got %d, want %d",
			exitCode,
			exitInvalidArgs,
		)
	}

	assertCLIOutputContains(
		t,
		stderr.String(),
		"benchmark timeout must be greater than zero",
	)
}

func TestRunBenchmarkRejectsUnsupportedQueryType(
	t *testing.T,
) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(
		[]string{
			"benchmark",
			"--config",
			"config.example.yaml",
			"--query-type",
			"MX",
		},
		&stdout,
		&stderr,
	)

	if exitCode != exitInvalidArgs {
		t.Fatalf(
			"unexpected exit code: got %d, want %d",
			exitCode,
			exitInvalidArgs,
		)
	}

	assertCLIOutputContains(
		t,
		stderr.String(),
		"invalid benchmark query type",
	)
}

func TestRunBenchmarkRejectsEmptyDomain(
	t *testing.T,
) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(
		[]string{
			"benchmark",
			"--config",
			"config.example.yaml",
			"--domains",
			"example.com,,cloudflare.com",
		},
		&stdout,
		&stderr,
	)

	if exitCode != exitInvalidArgs {
		t.Fatalf(
			"unexpected exit code: got %d, want %d",
			exitCode,
			exitInvalidArgs,
		)
	}

	assertCLIOutputContains(
		t,
		stderr.String(),
		"invalid benchmark domains",
	)

	assertCLIOutputContains(
		t,
		stderr.String(),
		"domain at index 1 is empty",
	)
}

func TestRunBenchmarkRejectsUnexpectedArguments(
	t *testing.T,
) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(
		[]string{
			"benchmark",
			"--config",
			"config.example.yaml",
			"unexpected",
		},
		&stdout,
		&stderr,
	)

	if exitCode != exitInvalidArgs {
		t.Fatalf(
			"unexpected exit code: got %d, want %d",
			exitCode,
			exitInvalidArgs,
		)
	}

	assertCLIOutputContains(
		t,
		stderr.String(),
		"unexpected benchmark arguments",
	)
}

func TestParseBenchmarkQueryType(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected uint16
	}{
		{
			name:     "A uppercase",
			input:    "A",
			expected: dns.TypeA,
		},
		{
			name:     "A lowercase",
			input:    "a",
			expected: dns.TypeA,
		},
		{
			name:     "AAAA uppercase",
			input:    "AAAA",
			expected: dns.TypeAAAA,
		},
		{
			name:     "AAAA lowercase",
			input:    "aaaa",
			expected: dns.TypeAAAA,
		},
		{
			name:     "surrounding whitespace",
			input:    "  AAAA  ",
			expected: dns.TypeAAAA,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				actual, err :=
					parseBenchmarkQueryType(
						test.input,
					)
				if err != nil {
					t.Fatalf(
						"parse benchmark query type: %v",
						err,
					)
				}

				if actual != test.expected {
					t.Fatalf(
						"unexpected query type: got %d, want %d",
						actual,
						test.expected,
					)
				}
			},
		)
	}
}

func TestParseBenchmarkQueryTypeRejectsUnsupportedType(
	t *testing.T,
) {
	t.Parallel()

	_, err := parseBenchmarkQueryType(
		"MX",
	)
	if err == nil {
		t.Fatal(
			"expected unsupported query type to fail",
		)
	}

	assertCLIOutputContains(
		t,
		err.Error(),
		"supported types are A and AAAA",
	)
}

func TestParseBenchmarkDomains(
	t *testing.T,
) {
	t.Parallel()

	actual, err := parseBenchmarkDomains(
		" example.com,cloudflare.com , github.com. ",
	)
	if err != nil {
		t.Fatalf(
			"parse benchmark domains: %v",
			err,
		)
	}

	expected := []string{
		"example.com",
		"cloudflare.com",
		"github.com.",
	}

	if len(actual) != len(expected) {
		t.Fatalf(
			"unexpected domain count: got %d, want %d",
			len(actual),
			len(expected),
		)
	}

	for index := range expected {
		if actual[index] != expected[index] {
			t.Fatalf(
				"unexpected domain at index %d: got %q, want %q",
				index,
				actual[index],
				expected[index],
			)
		}
	}
}

func TestParseBenchmarkDomainsRejectsBlankDomain(
	t *testing.T,
) {
	t.Parallel()

	_, err := parseBenchmarkDomains(
		"example.com, ,cloudflare.com",
	)
	if err == nil {
		t.Fatal(
			"expected blank domain to fail",
		)
	}

	assertCLIOutputContains(
		t,
		err.Error(),
		"domain at index 1 is empty",
	)
}

func TestRunRejectsInvalidFlag(
	t *testing.T,
) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(
		[]string{"--unknown"},
		&stdout,
		&stderr,
	)

	if exitCode != exitInvalidArgs {
		t.Fatalf(
			"unexpected exit code: got %d, want %d",
			exitCode,
			exitInvalidArgs,
		)
	}

	assertCLIOutputContains(
		t,
		stderr.String(),
		"flag provided but not defined",
	)
}

func TestRunBenchmarkRejectsInvalidFlag(
	t *testing.T,
) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(
		[]string{
			"benchmark",
			"--unknown",
		},
		&stdout,
		&stderr,
	)

	if exitCode != exitInvalidArgs {
		t.Fatalf(
			"unexpected exit code: got %d, want %d",
			exitCode,
			exitInvalidArgs,
		)
	}

	assertCLIOutputContains(
		t,
		stderr.String(),
		"flag provided but not defined",
	)
}

func assertCLIOutputContains(
	t *testing.T,
	output string,
	expected string,
) {
	t.Helper()

	if !strings.Contains(
		output,
		expected,
	) {
		t.Fatalf(
			"output does not contain %q:\n%s",
			expected,
			output,
		)
	}
}
