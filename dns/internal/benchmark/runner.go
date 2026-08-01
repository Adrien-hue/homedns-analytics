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
	if protocol != ProtocolUDP &&
		protocol != ProtocolTCP {
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

	if r.protocol != "" &&
		r.protocol != config.Protocol {
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

	reporter := resolvedProgressReporter(
		config.ProgressReporter,
	)

	pathStartedAt := time.Now()

	benchmarkStartedAt :=
		config.BenchmarkStartedAt

	if benchmarkStartedAt.IsZero() {
		benchmarkStartedAt = pathStartedAt
	}

	reportProgress(
		reporter,
		config,
		ProgressEventPhaseStarted,
		0,
		0,
		0,
		0,
		benchmarkStartedAt,
		pathStartedAt,
		nil,
	)

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
		Requests: RequestCounts{
			Attempted: config.QueryCount,
		},
	}

	latencies := make(
		[]time.Duration,
		0,
		config.QueryCount,
	)

	completed := 0
	lastProgressAt := time.Time{}

	for observation := range observations {
		completed++

		if observation.err != nil {
			recordFailure(
				&result,
				observation.failure,
			)
		} else {
			result.Requests.Successful++

			latencies = append(
				latencies,
				observation.latency,
			)
		}

		now := time.Now()

		if shouldReportProgress(
			completed,
			config.QueryCount,
			lastProgressAt,
			now,
		) {
			reportProgress(
				reporter,
				config,
				ProgressEventPhaseProgress,
				completed,
				result.Requests.Successful,
				result.Requests.Failed,
				result.Requests.Timeouts,
				benchmarkStartedAt,
				pathStartedAt,
				nil,
			)

			lastProgressAt = now
		}
	}

	unscheduled :=
		config.QueryCount -
			int(scheduled.Load())

	if unscheduled > 0 {
		recordUnscheduledFailures(
			&result,
			unscheduled,
			ctx.Err(),
		)

		completed += unscheduled
	}

	elapsed := time.Since(
		pathStartedAt,
	)

	result.DurationMilliseconds = round(
		durationMilliseconds(elapsed),
		6,
	)

	result.Requests.NonTimeoutFailures =
		result.Requests.Failed -
			result.Requests.Timeouts

	result.Rates = calculateRequestRates(
		result.Requests,
	)

	result.Throughput = calculateThroughput(
		result.Requests,
		elapsed,
	)

	if len(latencies) > 0 {
		statistics, err :=
			CalculateLatencyStatistics(
				latencies,
			)
		if err != nil {
			return PathResult{}, fmt.Errorf(
				"calculate path latency statistics: %w",
				err,
			)
		}

		result.
			SuccessfulRequestLatencyMilliseconds =
			successfulLatencyStatistics(
				len(latencies),
				statistics,
			)
	}

	reportProgress(
		reporter,
		config,
		ProgressEventPhaseCompleted,
		completed,
		result.Requests.Successful,
		result.Requests.Failed,
		result.Requests.Timeouts,
		benchmarkStartedAt,
		pathStartedAt,
		&result,
	)

	return result, nil
}

type queryObservation struct {
	latency time.Duration
	failure failureKind
	err     error
}

type failureKind string

const (
	failureNone            failureKind = ""
	failureTimeout         failureKind = "timeout"
	failureNetwork         failureKind = "network"
	failureDNSResponse     failureKind = "dns_response"
	failureInvalidResponse failureKind = "invalid_response"
	failureInternal        failureKind = "internal"
	failureOther           failureKind = "other"
)

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
			failure: classifyExchangeError(
				err,
			),
			err: err,
		}
	}

	if response == nil {
		return queryObservation{
			failure: failureInvalidResponse,
			err: errors.New(
				"DNS server returned no response",
			),
		}
	}

	if response.Id != message.Id {
		return queryObservation{
			failure: failureInvalidResponse,
			err: errors.New(
				"DNS response ID does not match query",
			),
		}
	}

	if response.Rcode != dns.RcodeSuccess {
		return queryObservation{
			failure: failureDNSResponse,
			err: fmt.Errorf(
				"DNS server returned response code %s",
				dns.RcodeToString[response.Rcode],
			),
		}
	}

	return queryObservation{
		latency: latency,
		failure: failureNone,
	}
}

func reportProgress(
	reporter ProgressReporter,
	config PathConfig,
	eventType string,
	current int,
	successful int,
	failed int,
	timeouts int,
	benchmarkStartedAt time.Time,
	phaseStartedAt time.Time,
	pathResult *PathResult,
) {
	if reporter == nil {
		return
	}

	now := time.Now()

	path := ""

	switch config.Phase {
	case ProgressPhaseWarmupDirect,
		ProgressPhaseDirect:
		path = "direct"

	case ProgressPhaseWarmupForwarded,
		ProgressPhaseForwarded:
		path = "forwarded"
	}

	phaseElapsed := now.Sub(
		phaseStartedAt,
	)

	reporter.ReportProgress(
		ProgressEvent{
			Type: eventType,

			BenchmarkStartedAt: benchmarkStartedAt,

			OccurredAt: now,

			ScenarioName: config.ScenarioName,

			ScenarioIndex: config.ScenarioIndex,

			ScenarioCount: config.ScenarioCount,

			Phase: config.Phase,

			Path: path,

			Current: current,

			Total: config.QueryCount,

			Successful: successful,

			Failed: failed,

			Timeouts: timeouts,

			Elapsed: now.Sub(
				benchmarkStartedAt,
			),

			PhaseElapsed: phaseElapsed,

			AttemptedQueriesPerSecond: liveQueriesPerSecond(
				current,
				phaseElapsed,
			),

			SuccessfulQueriesPerSecond: liveQueriesPerSecond(
				successful,
				phaseElapsed,
			),

			PathResult: pathResult,
		},
	)
}

func shouldReportProgress(
	current int,
	total int,
	lastProgressAt time.Time,
	now time.Time,
) bool {
	if current <= 0 {
		return false
	}

	if current >= total {
		return true
	}

	if lastProgressAt.IsZero() {
		return true
	}

	return now.Sub(lastProgressAt) >=
		time.Second
}

func liveQueriesPerSecond(
	queryCount int,
	elapsed time.Duration,
) float64 {
	if queryCount <= 0 ||
		elapsed <= 0 {
		return 0
	}

	return round(
		float64(queryCount)/
			elapsed.Seconds(),
		6,
	)
}

func classifyExchangeError(
	err error,
) failureKind {
	if err == nil {
		return failureNone
	}

	if isTimeout(err) {
		return failureTimeout
	}

	var networkError net.Error

	if errors.As(
		err,
		&networkError,
	) {
		return failureNetwork
	}

	if errors.Is(
		err,
		context.Canceled,
	) ||
		errors.Is(
			err,
			context.DeadlineExceeded,
		) {
		return failureInternal
	}

	return failureOther
}

func recordFailure(
	result *PathResult,
	kind failureKind,
) {
	if result == nil {
		return
	}

	result.Requests.Failed++

	switch kind {
	case failureTimeout:
		result.Requests.Timeouts++
		result.Failures.Timeout++

	case failureNetwork:
		result.Failures.Network++

	case failureDNSResponse:
		result.Failures.DNSResponse++

	case failureInvalidResponse:
		result.Failures.InvalidResponse++

	case failureInternal:
		result.Failures.Internal++

	default:
		result.Failures.Other++
	}
}

func recordUnscheduledFailures(
	result *PathResult,
	count int,
	contextErr error,
) {
	if result == nil ||
		count <= 0 {
		return
	}

	result.Requests.Failed += count

	if errors.Is(
		contextErr,
		context.DeadlineExceeded,
	) {
		result.Requests.Timeouts += count
		result.Failures.Timeout += count

		return
	}

	if errors.Is(
		contextErr,
		context.Canceled,
	) {
		result.Failures.Internal += count

		return
	}

	result.Failures.Other += count
}

func calculateRequestRates(
	requests RequestCounts,
) RequestRates {
	if requests.Attempted <= 0 {
		return RequestRates{}
	}

	attempted := float64(
		requests.Attempted,
	)

	return RequestRates{
		SuccessPercent: round(
			float64(
				requests.Successful,
			)/attempted*100,
			6,
		),

		FailurePercent: round(
			float64(
				requests.Failed,
			)/attempted*100,
			6,
		),

		TimeoutPercent: round(
			float64(
				requests.Timeouts,
			)/attempted*100,
			6,
		),
	}
}

func calculateThroughput(
	requests RequestCounts,
	elapsed time.Duration,
) ThroughputMetrics {
	if elapsed <= 0 {
		return ThroughputMetrics{}
	}

	elapsedSeconds :=
		elapsed.Seconds()

	return ThroughputMetrics{
		AttemptedQueriesPerSecond: round(
			float64(
				requests.Attempted,
			)/elapsedSeconds,
			6,
		),

		SuccessfulQueriesPerSecond: round(
			float64(
				requests.Successful,
			)/elapsedSeconds,
			6,
		),
	}
}

func successfulLatencyStatistics(
	sampleCount int,
	statistics LatencyStatistics,
) SuccessfulLatencyStatistics {
	return SuccessfulLatencyStatistics{
		SampleCount: sampleCount,
		Minimum:     statistics.Minimum,
		Mean:        statistics.Mean,
		Median:      statistics.Median,
		P95:         statistics.P95,
		P99:         statistics.P99,
		Maximum:     statistics.Maximum,
	}
}

func isTimeout(
	err error,
) bool {
	if errors.Is(
		err,
		context.DeadlineExceeded,
	) {
		return true
	}

	var networkError net.Error

	return errors.As(
		err,
		&networkError,
	) && networkError.Timeout()
}
