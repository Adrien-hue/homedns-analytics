package benchmark

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
)

type recordedScenarioRunner struct {
	configs []ScenarioConfig
	results []ScenarioResult
	errs    []error
}

func (r *recordedScenarioRunner) RunScenario(
	_ context.Context,
	config ScenarioConfig,
) (ScenarioResult, error) {
	r.configs = append(r.configs, config)

	index := len(r.configs) - 1

	var result ScenarioResult
	if index < len(r.results) {
		result = r.results[index]
	}

	var err error
	if index < len(r.errs) {
		err = r.errs[index]
	}

	return result, err
}

func validSuiteConfig() SuiteConfig {
	return SuiteConfig{
		DirectAddress:    "1.1.1.1:53",
		ForwardedAddress: "127.0.0.1:5300",
		QueryNames: []string{
			"example.com.",
			"cloudflare.com.",
		},
		QueryType:         dns.TypeA,
		WarmupQueries:     2,
		QueryCount:        20,
		ConcurrentWorkers: 5,
		Timeout:           3 * time.Second,
	}
}

func TestSuiteConfigScenarios(t *testing.T) {
	t.Parallel()

	config := validSuiteConfig()
	scenarios := config.Scenarios()

	if len(scenarios) != 4 {
		t.Fatalf(
			"unexpected scenario count: got %d, want %d",
			len(scenarios),
			4,
		)
	}

	expected := []struct {
		name        string
		protocol    string
		concurrency int
	}{
		{
			name:        "udp-sequential",
			protocol:    ProtocolUDP,
			concurrency: 1,
		},
		{
			name:        "udp-concurrent",
			protocol:    ProtocolUDP,
			concurrency: 5,
		},
		{
			name:        "tcp-sequential",
			protocol:    ProtocolTCP,
			concurrency: 1,
		},
		{
			name:        "tcp-concurrent",
			protocol:    ProtocolTCP,
			concurrency: 5,
		},
	}

	for index, expectedScenario := range expected {
		scenario := scenarios[index]

		if scenario.Name != expectedScenario.name {
			t.Fatalf(
				"scenario %d has name %q, want %q",
				index,
				scenario.Name,
				expectedScenario.name,
			)
		}

		if scenario.Protocol != expectedScenario.protocol {
			t.Fatalf(
				"scenario %q has protocol %q, want %q",
				scenario.Name,
				scenario.Protocol,
				expectedScenario.protocol,
			)
		}

		if scenario.Concurrency != expectedScenario.concurrency {
			t.Fatalf(
				"scenario %q has concurrency %d, want %d",
				scenario.Name,
				scenario.Concurrency,
				expectedScenario.concurrency,
			)
		}

		if scenario.DirectAddress != config.DirectAddress {
			t.Fatalf(
				"scenario %q has unexpected direct address",
				scenario.Name,
			)
		}

		if scenario.ForwardedAddress != config.ForwardedAddress {
			t.Fatalf(
				"scenario %q has unexpected forwarded address",
				scenario.Name,
			)
		}

		if scenario.QueryCount != config.QueryCount {
			t.Fatalf(
				"scenario %q has query count %d, want %d",
				scenario.Name,
				scenario.QueryCount,
				config.QueryCount,
			)
		}

		if scenario.WarmupQueries != config.WarmupQueries {
			t.Fatalf(
				"scenario %q has warmup count %d, want %d",
				scenario.Name,
				scenario.WarmupQueries,
				config.WarmupQueries,
			)
		}
	}
}

func TestSuiteConfigScenariosCopiesQueryNames(t *testing.T) {
	t.Parallel()

	config := validSuiteConfig()
	scenarios := config.Scenarios()

	scenarios[0].QueryNames[0] = "modified.example."

	if config.QueryNames[0] != "example.com." {
		t.Fatal(
			"scenario query names unexpectedly modified suite configuration",
		)
	}

	if scenarios[1].QueryNames[0] != "example.com." {
		t.Fatal(
			"scenario query names unexpectedly share backing storage",
		)
	}
}

func TestSuiteRunnerRun(t *testing.T) {
	t.Parallel()

	fakeRunner := &recordedScenarioRunner{
		results: []ScenarioResult{
			{Name: "udp-sequential"},
			{Name: "udp-concurrent"},
			{Name: "tcp-sequential"},
			{Name: "tcp-concurrent"},
		},
	}

	runner := &SuiteRunner{
		scenarioRunner: fakeRunner,
	}

	results, err := runner.Run(
		context.Background(),
		validSuiteConfig(),
	)
	if err != nil {
		t.Fatalf("run benchmark suite: %v", err)
	}

	if len(results) != 4 {
		t.Fatalf(
			"unexpected result count: got %d, want %d",
			len(results),
			4,
		)
	}

	if len(fakeRunner.configs) != 4 {
		t.Fatalf(
			"unexpected scenario execution count: got %d, want %d",
			len(fakeRunner.configs),
			4,
		)
	}

	expectedNames := []string{
		"udp-sequential",
		"udp-concurrent",
		"tcp-sequential",
		"tcp-concurrent",
	}

	for index, expectedName := range expectedNames {
		if fakeRunner.configs[index].Name != expectedName {
			t.Fatalf(
				"scenario %d executed as %q, want %q",
				index,
				fakeRunner.configs[index].Name,
				expectedName,
			)
		}

		if results[index].Name != expectedName {
			t.Fatalf(
				"result %d has name %q, want %q",
				index,
				results[index].Name,
				expectedName,
			)
		}
	}
}

func TestSuiteRunnerStopsOnScenarioError(t *testing.T) {
	t.Parallel()

	fakeRunner := &recordedScenarioRunner{
		results: []ScenarioResult{
			{Name: "udp-sequential"},
		},
		errs: []error{
			nil,
			errors.New("UDP concurrent benchmark failed"),
		},
	}

	runner := &SuiteRunner{
		scenarioRunner: fakeRunner,
	}

	_, err := runner.Run(
		context.Background(),
		validSuiteConfig(),
	)
	if err == nil {
		t.Fatal("expected benchmark suite to fail")
	}

	if !strings.Contains(
		err.Error(),
		`run benchmark scenario "udp-concurrent"`,
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}

	if len(fakeRunner.configs) != 2 {
		t.Fatalf(
			"unexpected execution count: got %d, want %d",
			len(fakeRunner.configs),
			2,
		)
	}
}

func TestSuiteRunnerStopsWhenContextCancelled(
	t *testing.T,
) {
	t.Parallel()

	ctx, cancel := context.WithCancel(
		context.Background(),
	)

	fakeRunner := &recordedScenarioRunner{}

	fakeRunner.results = []ScenarioResult{
		{Name: "udp-sequential"},
	}

	runner := &SuiteRunner{
		scenarioRunner: &cancellingScenarioRunner{
			delegate: fakeRunner,
			cancel:   cancel,
		},
	}

	_, err := runner.Run(
		ctx,
		validSuiteConfig(),
	)
	if err == nil {
		t.Fatal("expected cancelled suite to fail")
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context cancellation, got: %v",
			err,
		)
	}

	if len(fakeRunner.configs) != 1 {
		t.Fatalf(
			"unexpected execution count: got %d, want %d",
			len(fakeRunner.configs),
			1,
		)
	}
}

type cancellingScenarioRunner struct {
	delegate scenarioExecutor
	cancel   context.CancelFunc
	calls    int
}

func (r *cancellingScenarioRunner) RunScenario(
	ctx context.Context,
	config ScenarioConfig,
) (ScenarioResult, error) {
	result, err := r.delegate.RunScenario(
		ctx,
		config,
	)

	r.calls++
	if r.calls == 1 {
		r.cancel()
	}

	return result, err
}

func TestSuiteRunnerRejectsInvalidConfiguration(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*SuiteConfig)
	}{
		{
			name: "missing direct address",
			mutate: func(config *SuiteConfig) {
				config.DirectAddress = ""
			},
		},
		{
			name: "missing forwarded address",
			mutate: func(config *SuiteConfig) {
				config.ForwardedAddress = ""
			},
		},
		{
			name: "empty query names",
			mutate: func(config *SuiteConfig) {
				config.QueryNames = nil
			},
		},
		{
			name: "missing query type",
			mutate: func(config *SuiteConfig) {
				config.QueryType = 0
			},
		},
		{
			name: "negative warmup count",
			mutate: func(config *SuiteConfig) {
				config.WarmupQueries = -1
			},
		},
		{
			name: "zero query count",
			mutate: func(config *SuiteConfig) {
				config.QueryCount = 0
			},
		},
		{
			name: "zero concurrent workers",
			mutate: func(config *SuiteConfig) {
				config.ConcurrentWorkers = 0
			},
		},
		{
			name: "workers exceed query count",
			mutate: func(config *SuiteConfig) {
				config.ConcurrentWorkers =
					config.QueryCount + 1
			},
		},
		{
			name: "zero timeout",
			mutate: func(config *SuiteConfig) {
				config.Timeout = 0
			},
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			config := validSuiteConfig()
			test.mutate(&config)

			if err := config.Validate(); err == nil {
				t.Fatal(
					"expected suite validation to fail",
				)
			}
		})
	}
}

func TestSuiteRunnerRejectsNilReceiver(t *testing.T) {
	t.Parallel()

	var runner *SuiteRunner

	_, err := runner.Run(
		context.Background(),
		validSuiteConfig(),
	)
	if err == nil {
		t.Fatal(
			"expected nil suite runner to fail",
		)
	}
}

func TestSuiteRunnerRejectsMissingScenarioRunner(
	t *testing.T,
) {
	t.Parallel()

	runner := &SuiteRunner{}

	_, err := runner.Run(
		context.Background(),
		validSuiteConfig(),
	)
	if err == nil {
		t.Fatal(
			"expected missing scenario runner to fail",
		)
	}
}

func TestDefaultSuiteConfig(t *testing.T) {
	t.Parallel()

	config := DefaultSuiteConfig(
		"1.1.1.1:53",
		"127.0.0.1:5300",
	)

	if err := config.Validate(); err != nil {
		t.Fatalf(
			"default suite configuration is invalid: %v",
			err,
		)
	}

	if config.QueryType != dns.TypeA {
		t.Fatalf(
			"unexpected default query type: %d",
			config.QueryType,
		)
	}

	if config.ConcurrentWorkers !=
		DefaultConcurrentConcurrency {
		t.Fatalf(
			"unexpected default concurrency: %d",
			config.ConcurrentWorkers,
		)
	}

	if len(config.QueryNames) == 0 {
		t.Fatal(
			"default query names must not be empty",
		)
	}
}
