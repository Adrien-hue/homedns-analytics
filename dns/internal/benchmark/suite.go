package benchmark

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/miekg/dns"
)

const (
	DefaultSequentialConcurrency = 1
	DefaultConcurrentConcurrency = 10
)

// SuiteConfig contains the shared configuration used to build the default
// DNS benchmark scenarios.
type SuiteConfig struct {
	DirectAddress    string
	ForwardedAddress string

	QueryNames []string
	QueryType  uint16

	WarmupQueries int
	QueryCount    int

	ConcurrentWorkers int
	Timeout           time.Duration
}

// Validate verifies that the suite can generate valid benchmark scenarios.
func (c SuiteConfig) Validate() error {
	if c.DirectAddress == "" {
		return errors.New("direct DNS address is required")
	}

	if c.ForwardedAddress == "" {
		return errors.New("forwarded DNS address is required")
	}

	if len(c.QueryNames) == 0 {
		return errors.New("at least one DNS query name is required")
	}

	if c.QueryType == 0 {
		return errors.New("DNS query type is required")
	}

	if c.WarmupQueries < 0 {
		return errors.New(
			"warmup query count cannot be negative",
		)
	}

	if c.QueryCount <= 0 {
		return errors.New(
			"query count must be greater than zero",
		)
	}

	if c.ConcurrentWorkers <= 0 {
		return errors.New(
			"concurrent worker count must be greater than zero",
		)
	}

	if c.ConcurrentWorkers > c.QueryCount {
		return errors.New(
			"concurrent worker count cannot exceed query count",
		)
	}

	if c.Timeout <= 0 {
		return errors.New(
			"DNS timeout must be greater than zero",
		)
	}

	for _, scenario := range c.Scenarios() {
		if err := scenario.Validate(); err != nil {
			return fmt.Errorf(
				"invalid scenario %q: %w",
				scenario.Name,
				err,
			)
		}
	}

	return nil
}

// Scenarios returns the four standard benchmark scenarios in stable order.
func (c SuiteConfig) Scenarios() []ScenarioConfig {
	return []ScenarioConfig{
		c.scenario(
			"udp-sequential",
			ProtocolUDP,
			DefaultSequentialConcurrency,
		),
		c.scenario(
			"udp-concurrent",
			ProtocolUDP,
			c.ConcurrentWorkers,
		),
		c.scenario(
			"tcp-sequential",
			ProtocolTCP,
			DefaultSequentialConcurrency,
		),
		c.scenario(
			"tcp-concurrent",
			ProtocolTCP,
			c.ConcurrentWorkers,
		),
	}
}

func (c SuiteConfig) scenario(
	name string,
	protocol string,
	concurrency int,
) ScenarioConfig {
	return ScenarioConfig{
		Name:             name,
		Protocol:         protocol,
		DirectAddress:    c.DirectAddress,
		ForwardedAddress: c.ForwardedAddress,
		QueryNames:       append([]string(nil), c.QueryNames...),
		QueryType:        c.QueryType,
		WarmupQueries:    c.WarmupQueries,
		QueryCount:       c.QueryCount,
		Concurrency:      concurrency,
		Timeout:          c.Timeout,
	}
}

// DefaultSuiteConfig returns conservative defaults suitable for the first
// Raspberry Pi benchmark release.
func DefaultSuiteConfig(
	directAddress string,
	forwardedAddress string,
) SuiteConfig {
	return SuiteConfig{
		DirectAddress:    directAddress,
		ForwardedAddress: forwardedAddress,
		QueryNames: []string{
			"example.com.",
			"cloudflare.com.",
			"google.com.",
			"github.com.",
		},
		QueryType:         dns.TypeA,
		WarmupQueries:     10,
		QueryCount:        100,
		ConcurrentWorkers: DefaultConcurrentConcurrency,
		Timeout:           2 * time.Second,
	}
}

type scenarioExecutor interface {
	RunScenario(
		context.Context,
		ScenarioConfig,
	) (ScenarioResult, error)
}

// SuiteRunner executes a complete collection of benchmark scenarios.
type SuiteRunner struct {
	scenarioRunner scenarioExecutor
}

// NewSuiteRunner creates a benchmark suite runner.
func NewSuiteRunner() *SuiteRunner {
	return &SuiteRunner{
		scenarioRunner: NewScenarioRunner(),
	}
}

// Run executes every scenario sequentially in its stable declared order.
func (r *SuiteRunner) Run(
	ctx context.Context,
	config SuiteConfig,
) ([]ScenarioResult, error) {
	if r == nil || r.scenarioRunner == nil {
		return nil, errors.New(
			"benchmark suite runner is not initialized",
		)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf(
			"validate benchmark suite: %w",
			err,
		)
	}

	scenarios := config.Scenarios()
	results := make([]ScenarioResult, 0, len(scenarios))

	for _, scenario := range scenarios {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf(
				"benchmark suite interrupted before scenario %q: %w",
				scenario.Name,
				err,
			)
		}

		result, err := r.scenarioRunner.RunScenario(
			ctx,
			scenario,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"run benchmark scenario %q: %w",
				scenario.Name,
				err,
			)
		}

		results = append(results, result)
	}

	return results, nil
}
