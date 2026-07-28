package benchmark

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
)

type exchangeFunc func(
	context.Context,
	*dns.Msg,
	string,
) (*dns.Msg, time.Duration, error)

// Runner executes DNS benchmark paths.
type Runner struct {
	protocol string
	exchange exchangeFunc
}

// NewRunner creates a benchmark runner using a miekg/dns client.
func NewRunner(
	protocol string,
	timeout time.Duration,
) (*Runner, error) {
	if protocol != ProtocolUDP && protocol != ProtocolTCP {
		return nil, fmt.Errorf(
			"unsupported DNS protocol %q",
			protocol,
		)
	}

	if timeout <= 0 {
		return nil, errors.New(
			"DNS client timeout must be greater than zero",
		)
	}

	client := &dns.Client{
		Net:     protocol,
		Timeout: timeout,
	}

	return &Runner{
		protocol: protocol,
		exchange: client.ExchangeContext,
	}, nil
}

// RunPath benchmarks one DNS target using the supplied configuration.
func (r *Runner) RunPath(
	ctx context.Context,
	config PathConfig,
) (PathResult, error) {
	if r == nil || r.exchange == nil {
		return PathResult{}, errors.New(
			"benchmark runner is not initialized",
		)
	}

	if ctx == nil {
		return PathResult{}, errors.New(
			"benchmark context is required",
		)
	}

	if r.protocol != "" && r.protocol != config.Protocol {
		return PathResult{}, fmt.Errorf(
			"runner protocol %q does not match path protocol %q",
			r.protocol,
			config.Protocol,
		)
	}

	if err := config.Validate(); err != nil {
		return PathResult{}, fmt.Errorf(
			"validate benchmark path: %w",
			err,
		)
	}

	startedAt := time.Now()

	observations := make(
		chan queryObservation,
		config.QueryCount,
	)

	jobs := make(chan int)

	var workers sync.WaitGroup

	for worker := 0; worker < config.Concurrency; worker++ {
		workers.Add(1)

		go func() {
			defer workers.Done()

			for queryIndex := range jobs {
				observations <- r.executeQuery(
					ctx,
					config,
					queryIndex,
				)
			}
		}()
	}

	var scheduled atomic.Int64

	go func() {
		defer close(jobs)

		for queryIndex := 0; queryIndex < config.QueryCount; queryIndex++ {
			select {
			case <-ctx.Done():
				return
			case jobs <- queryIndex:
				scheduled.Add(1)
			}
		}
	}()

	go func() {
		workers.Wait()
		close(observations)
	}()

	result := PathResult{
		Requests: config.QueryCount,
	}

	latencies := make(
		[]time.Duration,
		0,
		config.QueryCount,
	)

	for observation := range observations {
		if observation.timeout {
			result.Timeouts++
		}

		if observation.err != nil {
			result.Failed++

			continue
		}

		result.Successful++
		latencies = append(
			latencies,
			observation.latency,
		)
	}

	unscheduled := config.QueryCount - int(scheduled.Load())
	if unscheduled > 0 {
		result.Failed += unscheduled

		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			result.Timeouts += unscheduled
		}
	}

	elapsed := time.Since(startedAt)

	result.DurationMilliseconds = round(
		durationMilliseconds(elapsed),
		6,
	)

	if elapsed > 0 {
		result.QueriesPerSecond = round(
			float64(result.Successful)/elapsed.Seconds(),
			6,
		)
	}

	if len(latencies) > 0 {
		statistics, err := CalculateLatencyStatistics(
			latencies,
		)
		if err != nil {
			return PathResult{}, fmt.Errorf(
				"calculate path latency statistics: %w",
				err,
			)
		}

		result.LatencyMilliseconds = statistics
	}

	return result, nil
}

type queryObservation struct {
	latency time.Duration
	timeout bool
	err     error
}

func (r *Runner) executeQuery(
	ctx context.Context,
	config PathConfig,
	queryIndex int,
) queryObservation {
	queryName := config.QueryNames[queryIndex%len(config.QueryNames)]

	message := new(dns.Msg)
	message.SetQuestion(
		dns.Fqdn(queryName),
		config.QueryType,
	)
	message.RecursionDesired = true

	response, latency, err := r.exchange(
		ctx,
		message,
		config.Address,
	)
	if err != nil {
		return queryObservation{
			timeout: isTimeout(err),
			err:     err,
		}
	}

	if response == nil {
		return queryObservation{
			err: errors.New("DNS server returned no response"),
		}
	}

	if response.Id != message.Id {
		return queryObservation{
			err: errors.New("DNS response ID does not match query"),
		}
	}

	if response.Rcode != dns.RcodeSuccess {
		return queryObservation{
			err: fmt.Errorf(
				"DNS server returned response code %s",
				dns.RcodeToString[response.Rcode],
			),
		}
	}

	return queryObservation{
		latency: latency,
	}
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var networkError net.Error

	return errors.As(err, &networkError) &&
		networkError.Timeout()
}
