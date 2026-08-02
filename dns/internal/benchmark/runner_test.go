package benchmark

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestRunnerRunPath(t *testing.T) {
	t.Parallel()

	var mutex sync.Mutex
	receivedNames := make([]string, 0, 4)

	runner := &Runner{
		exchange: func(
			_ context.Context,
			message *dns.Msg,
			_ string,
		) (*dns.Msg, time.Duration, error) {
			mutex.Lock()
			receivedNames = append(
				receivedNames,
				message.Question[0].Name,
			)
			mutex.Unlock()

			response := new(dns.Msg)
			response.SetReply(message)

			return response, 2 * time.Millisecond, nil
		},
	}

	config := validPathConfig()
	config.QueryNames = []string{
		"example.com.",
		"cloudflare.com.",
	}
	config.QueryCount = 4
	config.Concurrency = 2

	result, err := runner.RunPath(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf("run benchmark path: %v", err)
	}

	assertRequestCounts(
		t,
		result.Requests,
		RequestCounts{
			Attempted:  4,
			Successful: 4,
		},
	)

	assertRequestRates(
		t,
		result.Rates,
		RequestRates{
			SuccessPercent: 100,
		},
	)

	assertFailureBreakdown(
		t,
		result.Failures,
		FailureBreakdown{},
	)

	if result.DurationMilliseconds <= 0 {
		t.Fatalf(
			"expected positive duration, got %f",
			result.DurationMilliseconds,
		)
	}

	if result.Throughput.AttemptedQueriesPerSecond <= 0 {
		t.Fatalf(
			"expected positive attempted QPS, got %f",
			result.Throughput.AttemptedQueriesPerSecond,
		)
	}

	if result.Throughput.SuccessfulQueriesPerSecond <= 0 {
		t.Fatalf(
			"expected positive successful QPS, got %f",
			result.Throughput.SuccessfulQueriesPerSecond,
		)
	}

	if result.Throughput.AttemptedQueriesPerSecond !=
		result.Throughput.SuccessfulQueriesPerSecond {
		t.Fatalf(
			"expected equal attempted and successful QPS: attempted=%f successful=%f",
			result.Throughput.AttemptedQueriesPerSecond,
			result.Throughput.SuccessfulQueriesPerSecond,
		)
	}

	latency := result.SuccessfulRequestLatencyMilliseconds

	if latency.SampleCount != 4 {
		t.Fatalf(
			"unexpected latency sample count: got %d, want %d",
			latency.SampleCount,
			4,
		)
	}

	if latency.Minimum != 2 ||
		latency.Mean != 2 ||
		latency.Median != 2 ||
		latency.P95 != 2 ||
		latency.P99 != 2 ||
		latency.Maximum != 2 {
		t.Fatalf(
			"unexpected latency statistics: %+v",
			latency,
		)
	}

	mutex.Lock()
	defer mutex.Unlock()

	if len(receivedNames) != 4 {
		t.Fatalf(
			"unexpected received query count: got %d, want %d",
			len(receivedNames),
			4,
		)
	}

	nameCounts := make(map[string]int)

	for _, name := range receivedNames {
		nameCounts[name]++
	}

	if nameCounts["example.com."] != 2 {
		t.Fatalf(
			"unexpected example.com query count: got %d, want %d",
			nameCounts["example.com."],
			2,
		)
	}

	if nameCounts["cloudflare.com."] != 2 {
		t.Fatalf(
			"unexpected cloudflare.com query count: got %d, want %d",
			nameCounts["cloudflare.com."],
			2,
		)
	}
}

func TestRunnerRunPathRecordsOtherFailures(t *testing.T) {
	t.Parallel()

	var requestNumber int
	var mutex sync.Mutex

	runner := &Runner{
		exchange: func(
			_ context.Context,
			message *dns.Msg,
			_ string,
		) (*dns.Msg, time.Duration, error) {
			mutex.Lock()
			requestNumber++
			currentRequest := requestNumber
			mutex.Unlock()

			if currentRequest == 1 {
				return nil, 0, errors.New(
					"temporary DNS failure",
				)
			}

			response := new(dns.Msg)
			response.SetReply(message)

			return response, time.Millisecond, nil
		},
	}

	config := validPathConfig()
	config.QueryCount = 2
	config.Concurrency = 1

	result, err := runner.RunPath(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf("run benchmark path: %v", err)
	}

	assertRequestCounts(
		t,
		result.Requests,
		RequestCounts{
			Attempted:          2,
			Successful:         1,
			Failed:             1,
			NonTimeoutFailures: 1,
		},
	)

	assertRequestRates(
		t,
		result.Rates,
		RequestRates{
			SuccessPercent: 50,
			FailurePercent: 50,
		},
	)

	assertFailureBreakdown(
		t,
		result.Failures,
		FailureBreakdown{
			Other: 1,
		},
	)

	if result.SuccessfulRequestLatencyMilliseconds.SampleCount != 1 {
		t.Fatalf(
			"unexpected latency sample count: got %d, want %d",
			result.SuccessfulRequestLatencyMilliseconds.SampleCount,
			1,
		)
	}

	if result.Throughput.AttemptedQueriesPerSecond <=
		result.Throughput.SuccessfulQueriesPerSecond {
		t.Fatalf(
			"expected attempted QPS to exceed successful QPS: attempted=%f successful=%f",
			result.Throughput.AttemptedQueriesPerSecond,
			result.Throughput.SuccessfulQueriesPerSecond,
		)
	}
}

func TestRunnerRunPathRecordsTimeouts(t *testing.T) {
	t.Parallel()

	runner := &Runner{
		exchange: func(
			_ context.Context,
			_ *dns.Msg,
			_ string,
		) (*dns.Msg, time.Duration, error) {
			return nil, 0, context.DeadlineExceeded
		},
	}

	config := validPathConfig()
	config.QueryCount = 3
	config.Concurrency = 1

	result, err := runner.RunPath(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf("run benchmark path: %v", err)
	}

	assertRequestCounts(
		t,
		result.Requests,
		RequestCounts{
			Attempted: 3,
			Failed:    3,
			Timeouts:  3,
		},
	)

	assertRequestRates(
		t,
		result.Rates,
		RequestRates{
			FailurePercent: 100,
			TimeoutPercent: 100,
		},
	)

	assertFailureBreakdown(
		t,
		result.Failures,
		FailureBreakdown{
			Timeout: 3,
		},
	)

	if result.SuccessfulRequestLatencyMilliseconds.SampleCount != 0 {
		t.Fatalf(
			"expected no latency samples, got %d",
			result.SuccessfulRequestLatencyMilliseconds.SampleCount,
		)
	}

	if result.Throughput.AttemptedQueriesPerSecond <= 0 {
		t.Fatalf(
			"expected positive attempted QPS, got %f",
			result.Throughput.AttemptedQueriesPerSecond,
		)
	}

	if result.Throughput.SuccessfulQueriesPerSecond != 0 {
		t.Fatalf(
			"expected zero successful QPS, got %f",
			result.Throughput.SuccessfulQueriesPerSecond,
		)
	}
}

func TestRunnerRunPathRecordsNetworkFailures(t *testing.T) {
	t.Parallel()

	runner := &Runner{
		exchange: func(
			_ context.Context,
			_ *dns.Msg,
			_ string,
		) (*dns.Msg, time.Duration, error) {
			return nil, 0, testNetworkError{
				message: "network unavailable",
			}
		},
	}

	config := validPathConfig()
	config.QueryCount = 2
	config.Concurrency = 1

	result, err := runner.RunPath(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf("run benchmark path: %v", err)
	}

	assertRequestCounts(
		t,
		result.Requests,
		RequestCounts{
			Attempted:          2,
			Failed:             2,
			NonTimeoutFailures: 2,
		},
	)

	assertFailureBreakdown(
		t,
		result.Failures,
		FailureBreakdown{
			Network: 2,
		},
	)
}

func TestRunnerRunPathRecordsNetworkTimeouts(t *testing.T) {
	t.Parallel()

	runner := &Runner{
		exchange: func(
			_ context.Context,
			_ *dns.Msg,
			_ string,
		) (*dns.Msg, time.Duration, error) {
			return nil, 0, testNetworkError{
				message: "network timeout",
				timeout: true,
			}
		},
	}

	config := validPathConfig()
	config.QueryCount = 2
	config.Concurrency = 1

	result, err := runner.RunPath(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf("run benchmark path: %v", err)
	}

	assertRequestCounts(
		t,
		result.Requests,
		RequestCounts{
			Attempted: 2,
			Failed:    2,
			Timeouts:  2,
		},
	)

	assertFailureBreakdown(
		t,
		result.Failures,
		FailureBreakdown{
			Timeout: 2,
		},
	)
}

func TestRunnerRunPathRecordsDNSResponseFailures(t *testing.T) {
	t.Parallel()

	runner := &Runner{
		exchange: func(
			_ context.Context,
			message *dns.Msg,
			_ string,
		) (*dns.Msg, time.Duration, error) {
			response := new(dns.Msg)
			response.SetReply(message)
			response.Rcode = dns.RcodeServerFailure

			return response, time.Millisecond, nil
		},
	}

	config := validPathConfig()
	config.QueryCount = 2
	config.Concurrency = 1

	result, err := runner.RunPath(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf("run benchmark path: %v", err)
	}

	assertRequestCounts(
		t,
		result.Requests,
		RequestCounts{
			Attempted:          2,
			Failed:             2,
			NonTimeoutFailures: 2,
		},
	)

	assertFailureBreakdown(
		t,
		result.Failures,
		FailureBreakdown{
			DNSResponse: 2,
		},
	)
}

func TestRunnerRunPathRecordsNilResponseAsInvalid(t *testing.T) {
	t.Parallel()

	runner := &Runner{
		exchange: func(
			_ context.Context,
			_ *dns.Msg,
			_ string,
		) (*dns.Msg, time.Duration, error) {
			return nil, time.Millisecond, nil
		},
	}

	config := validPathConfig()
	config.QueryCount = 1
	config.Concurrency = 1

	result, err := runner.RunPath(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf("run benchmark path: %v", err)
	}

	assertRequestCounts(
		t,
		result.Requests,
		RequestCounts{
			Attempted:          1,
			Failed:             1,
			NonTimeoutFailures: 1,
		},
	)

	assertFailureBreakdown(
		t,
		result.Failures,
		FailureBreakdown{
			InvalidResponse: 1,
		},
	)
}

func TestRunnerRunPathRecordsMismatchedResponseIDAsInvalid(
	t *testing.T,
) {
	t.Parallel()

	runner := &Runner{
		exchange: func(
			_ context.Context,
			message *dns.Msg,
			_ string,
		) (*dns.Msg, time.Duration, error) {
			response := new(dns.Msg)
			response.SetReply(message)
			response.Id++

			return response, time.Millisecond, nil
		},
	}

	config := validPathConfig()
	config.QueryCount = 1
	config.Concurrency = 1

	result, err := runner.RunPath(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf("run benchmark path: %v", err)
	}

	assertRequestCounts(
		t,
		result.Requests,
		RequestCounts{
			Attempted:          1,
			Failed:             1,
			NonTimeoutFailures: 1,
		},
	)

	assertFailureBreakdown(
		t,
		result.Failures,
		FailureBreakdown{
			InvalidResponse: 1,
		},
	)
}

func TestCalculateRequestRates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		requests RequestCounts
		expected RequestRates
	}{
		{
			name: "all successful",
			requests: RequestCounts{
				Attempted:  100,
				Successful: 100,
			},
			expected: RequestRates{
				SuccessPercent: 100,
			},
		},
		{
			name: "mixed result",
			requests: RequestCounts{
				Attempted:  200,
				Successful: 190,
				Failed:     10,
				Timeouts:   4,
			},
			expected: RequestRates{
				SuccessPercent: 95,
				FailurePercent: 5,
				TimeoutPercent: 2,
			},
		},
		{
			name:     "no attempts",
			requests: RequestCounts{},
			expected: RequestRates{},
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual := calculateRequestRates(test.requests)

			assertRequestRates(
				t,
				actual,
				test.expected,
			)
		})
	}
}

func TestCalculateThroughput(t *testing.T) {
	t.Parallel()

	actual := calculateThroughput(
		RequestCounts{
			Attempted:  100,
			Successful: 80,
		},
		2*time.Second,
	)

	expected := ThroughputMetrics{
		AttemptedQueriesPerSecond:  50,
		SuccessfulQueriesPerSecond: 40,
	}

	if actual != expected {
		t.Fatalf(
			"unexpected throughput: got %+v, want %+v",
			actual,
			expected,
		)
	}
}

func TestCalculateThroughputRejectsNonPositiveDuration(
	t *testing.T,
) {
	t.Parallel()

	actual := calculateThroughput(
		RequestCounts{
			Attempted:  100,
			Successful: 100,
		},
		0,
	)

	if actual != (ThroughputMetrics{}) {
		t.Fatalf(
			"expected empty throughput, got %+v",
			actual,
		)
	}
}

func TestClassifyExchangeError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected failureKind
	}{
		{
			name:     "nil",
			err:      nil,
			expected: failureNone,
		},
		{
			name:     "context deadline",
			err:      context.DeadlineExceeded,
			expected: failureTimeout,
		},
		{
			name: "network timeout",
			err: testNetworkError{
				message: "timeout",
				timeout: true,
			},
			expected: failureTimeout,
		},
		{
			name: "network error",
			err: testNetworkError{
				message: "network failure",
			},
			expected: failureNetwork,
		},
		{
			name:     "context canceled",
			err:      context.Canceled,
			expected: failureInternal,
		},
		{
			name:     "other",
			err:      errors.New("unexpected failure"),
			expected: failureOther,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual := classifyExchangeError(test.err)

			if actual != test.expected {
				t.Fatalf(
					"unexpected failure kind: got %q, want %q",
					actual,
					test.expected,
				)
			}
		})
	}
}

func TestRecordUnscheduledFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		contextErr error
		expected   PathResult
	}{
		{
			name:       "deadline exceeded",
			contextErr: context.DeadlineExceeded,
			expected: PathResult{
				Requests: RequestCounts{
					Failed:   3,
					Timeouts: 3,
				},
				Failures: FailureBreakdown{
					Timeout: 3,
				},
			},
		},
		{
			name:       "context canceled",
			contextErr: context.Canceled,
			expected: PathResult{
				Requests: RequestCounts{
					Failed: 3,
				},
				Failures: FailureBreakdown{
					Internal: 3,
				},
			},
		},
		{
			name:       "other",
			contextErr: errors.New("scheduler stopped"),
			expected: PathResult{
				Requests: RequestCounts{
					Failed: 3,
				},
				Failures: FailureBreakdown{
					Other: 3,
				},
			},
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var result PathResult

			recordUnscheduledFailures(
				&result,
				3,
				test.contextErr,
			)

			assertRequestCounts(
				t,
				result.Requests,
				test.expected.Requests,
			)

			assertFailureBreakdown(
				t,
				result.Failures,
				test.expected.Failures,
			)
		})
	}
}

func TestRunnerRunPathRejectsInvalidRunner(t *testing.T) {
	t.Parallel()

	var runner *Runner

	_, err := runner.RunPath(
		context.Background(),
		validPathConfig(),
	)
	if err == nil {
		t.Fatal("expected nil runner to fail")
	}
}

func TestRunnerRunPathRejectsNilContext(t *testing.T) {
	t.Parallel()

	runner := &Runner{
		exchange: func(
			context.Context,
			*dns.Msg,
			string,
		) (*dns.Msg, time.Duration, error) {
			t.Fatal("exchange must not be called")

			return nil, 0, nil
		},
	}

	_, err := runner.RunPath(
		nil,
		validPathConfig(),
	)
	if err == nil {
		t.Fatal("expected nil context to fail")
	}
}

func TestRunnerRunPathRejectsProtocolMismatch(t *testing.T) {
	t.Parallel()

	runner := &Runner{
		protocol: ProtocolTCP,
		exchange: func(
			context.Context,
			*dns.Msg,
			string,
		) (*dns.Msg, time.Duration, error) {
			t.Fatal("exchange must not be called")

			return nil, 0, nil
		},
	}

	config := validPathConfig()
	config.Protocol = ProtocolUDP

	_, err := runner.RunPath(
		context.Background(),
		config,
	)
	if err == nil {
		t.Fatal("expected protocol mismatch to fail")
	}
}

func TestRunnerRunPathRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	runner := &Runner{
		exchange: func(
			context.Context,
			*dns.Msg,
			string,
		) (*dns.Msg, time.Duration, error) {
			t.Fatal("exchange must not be called")

			return nil, 0, nil
		},
	}

	config := validPathConfig()
	config.QueryCount = 0

	_, err := runner.RunPath(
		context.Background(),
		config,
	)
	if err == nil {
		t.Fatal("expected invalid config to fail")
	}
}

func TestRunnerRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	_, err := NewRunner("invalid", time.Second)
	if err == nil {
		t.Fatal("expected invalid protocol to fail")
	}

	_, err = NewRunner(ProtocolUDP, 0)
	if err == nil {
		t.Fatal("expected invalid timeout to fail")
	}
}

type testNetworkError struct {
	message string
	timeout bool
}

func (e testNetworkError) Error() string {
	return e.message
}

func (e testNetworkError) Timeout() bool {
	return e.timeout
}

func (e testNetworkError) Temporary() bool {
	return false
}

func assertRequestCounts(
	t *testing.T,
	actual RequestCounts,
	expected RequestCounts,
) {
	t.Helper()

	if actual != expected {
		t.Fatalf(
			"unexpected request counts: got %+v, want %+v",
			actual,
			expected,
		)
	}
}

func assertRequestRates(
	t *testing.T,
	actual RequestRates,
	expected RequestRates,
) {
	t.Helper()

	if actual != expected {
		t.Fatalf(
			"unexpected request rates: got %+v, want %+v",
			actual,
			expected,
		)
	}
}

func assertFailureBreakdown(
	t *testing.T,
	actual FailureBreakdown,
	expected FailureBreakdown,
) {
	t.Helper()

	if actual != expected {
		t.Fatalf(
			"unexpected failure breakdown: got %+v, want %+v",
			actual,
			expected,
		)
	}
}

func TestRunnerRunPathReportsProgressEvents(
	t *testing.T,
) {
	t.Parallel()

	startedAt := time.Date(
		2026,
		time.August,
		1,
		10,
		0,
		0,
		0,
		time.UTC,
	)

	events := make(
		[]ProgressEvent,
		0,
	)

	reporter := ProgressReporterFunc(
		func(event ProgressEvent) {
			events = append(
				events,
				event,
			)
		},
	)

	runner := &Runner{
		exchange: func(
			_ context.Context,
			message *dns.Msg,
			_ string,
		) (*dns.Msg, time.Duration, error) {
			response := new(dns.Msg)
			response.SetReply(message)

			return response,
				2 * time.Millisecond,
				nil
		},
	}

	config := validPathConfig()
	config.QueryCount = 4
	config.Concurrency = 1

	config.ProgressReporter =
		reporter

	config.BenchmarkStartedAt =
		startedAt

	config.ScenarioName =
		"udp-sequential"

	config.ScenarioIndex = 1
	config.ScenarioCount = 4
	config.Phase = ProgressPhaseDirect

	result, err := runner.RunPath(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf(
			"run benchmark path: %v",
			err,
		)
	}

	if len(events) < 3 {
		t.Fatalf(
			"expected at least three progress events, got %d: %+v",
			len(events),
			events,
		)
	}

	started := events[0]

	if started.Type !=
		ProgressEventPhaseStarted {
		t.Fatalf(
			"unexpected first event type: got %q, want %q",
			started.Type,
			ProgressEventPhaseStarted,
		)
	}

	assertRunnerProgressMetadata(
		t,
		started,
		config,
	)

	if started.Current != 0 {
		t.Fatalf(
			"unexpected started progress: got %d, want 0",
			started.Current,
		)
	}

	if started.PathResult != nil {
		t.Fatal(
			"phase-started event unexpectedly contains a path result",
		)
	}

	var progress *ProgressEvent

	for index := range events {
		if events[index].Type ==
			ProgressEventPhaseProgress {
			progress = &events[index]
			break
		}
	}

	if progress == nil {
		t.Fatalf(
			"no phase-progress event received: %+v",
			events,
		)
	}

	assertRunnerProgressMetadata(
		t,
		*progress,
		config,
	)

	if progress.Current <= 0 ||
		progress.Current >
			config.QueryCount {
		t.Fatalf(
			"unexpected live progress count: got %d",
			progress.Current,
		)
	}

	if progress.Successful !=
		progress.Current {
		t.Fatalf(
			"unexpected live success count: current=%d successful=%d",
			progress.Current,
			progress.Successful,
		)
	}

	if progress.AttemptedQueriesPerSecond <= 0 {
		t.Fatalf(
			"expected positive live attempted QPS, got %f",
			progress.AttemptedQueriesPerSecond,
		)
	}

	if progress.SuccessfulQueriesPerSecond <= 0 {
		t.Fatalf(
			"expected positive live successful QPS, got %f",
			progress.SuccessfulQueriesPerSecond,
		)
	}

	if progress.PhaseElapsed <= 0 {
		t.Fatalf(
			"expected positive phase elapsed time, got %s",
			progress.PhaseElapsed,
		)
	}

	completed := events[len(events)-1]

	if completed.Type !=
		ProgressEventPhaseCompleted {
		t.Fatalf(
			"unexpected final event type: got %q, want %q",
			completed.Type,
			ProgressEventPhaseCompleted,
		)
	}

	assertRunnerProgressMetadata(
		t,
		completed,
		config,
	)

	if completed.Current !=
		config.QueryCount {
		t.Fatalf(
			"unexpected completion count: got %d, want %d",
			completed.Current,
			config.QueryCount,
		)
	}

	if completed.Successful !=
		config.QueryCount {
		t.Fatalf(
			"unexpected completion success count: got %d, want %d",
			completed.Successful,
			config.QueryCount,
		)
	}

	if completed.Failed != 0 ||
		completed.Timeouts != 0 {
		t.Fatalf(
			"unexpected completion failures: failed=%d timeouts=%d",
			completed.Failed,
			completed.Timeouts,
		)
	}

	if completed.PathResult == nil {
		t.Fatal(
			"phase-completed event does not contain the final path result",
		)
	}

	if completed.PathResult.Requests !=
		result.Requests {
		t.Fatalf(
			"unexpected reported request counts: got %+v, want %+v",
			completed.PathResult.Requests,
			result.Requests,
		)
	}

	if completed.
		PathResult.
		Throughput.
		SuccessfulQueriesPerSecond <= 0 {
		t.Fatalf(
			"expected final path result to contain successful QPS: %+v",
			completed.PathResult.Throughput,
		)
	}

	if completed.
		PathResult.
		SuccessfulRequestLatencyMilliseconds.
		SampleCount != config.QueryCount {
		t.Fatalf(
			"unexpected final latency sample count: got %d, want %d",
			completed.
				PathResult.
				SuccessfulRequestLatencyMilliseconds.
				SampleCount,
			config.QueryCount,
		)
	}
}

func TestRunnerRunPathProgressReportsFailures(
	t *testing.T,
) {
	t.Parallel()

	events := make(
		[]ProgressEvent,
		0,
	)

	reporter := ProgressReporterFunc(
		func(event ProgressEvent) {
			events = append(
				events,
				event,
			)
		},
	)

	requestNumber := 0

	runner := &Runner{
		exchange: func(
			_ context.Context,
			message *dns.Msg,
			_ string,
		) (*dns.Msg, time.Duration, error) {
			requestNumber++

			switch requestNumber {
			case 1:
				return nil,
					0,
					context.DeadlineExceeded

			case 2:
				return nil,
					0,
					errors.New(
						"temporary failure",
					)

			default:
				response := new(dns.Msg)
				response.SetReply(message)

				return response,
					time.Millisecond,
					nil
			}
		},
	}

	config := validPathConfig()
	config.QueryCount = 3
	config.Concurrency = 1

	config.ProgressReporter =
		reporter

	config.ScenarioName =
		"udp-sequential"

	config.ScenarioIndex = 1
	config.ScenarioCount = 4
	config.Phase = ProgressPhaseForwarded

	result, err := runner.RunPath(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf(
			"run benchmark path: %v",
			err,
		)
	}

	if len(events) == 0 {
		t.Fatal(
			"expected progress events",
		)
	}

	completed := events[len(events)-1]

	if completed.Type !=
		ProgressEventPhaseCompleted {
		t.Fatalf(
			"unexpected final event type: got %q, want %q",
			completed.Type,
			ProgressEventPhaseCompleted,
		)
	}

	if completed.Current != 3 {
		t.Fatalf(
			"unexpected completed count: got %d, want 3",
			completed.Current,
		)
	}

	if completed.Successful != 1 {
		t.Fatalf(
			"unexpected successful count: got %d, want 1",
			completed.Successful,
		)
	}

	if completed.Failed != 2 {
		t.Fatalf(
			"unexpected failed count: got %d, want 2",
			completed.Failed,
		)
	}

	if completed.Timeouts != 1 {
		t.Fatalf(
			"unexpected timeout count: got %d, want 1",
			completed.Timeouts,
		)
	}

	if completed.Path != "forwarded" {
		t.Fatalf(
			"unexpected progress path: got %q, want %q",
			completed.Path,
			"forwarded",
		)
	}

	if completed.PathResult == nil {
		t.Fatal(
			"completed event does not contain a path result",
		)
	}

	if completed.PathResult.Requests !=
		result.Requests {
		t.Fatalf(
			"unexpected completed path requests: got %+v, want %+v",
			completed.PathResult.Requests,
			result.Requests,
		)
	}

	if completed.
		SuccessfulQueriesPerSecond >=
		completed.
			AttemptedQueriesPerSecond {
		t.Fatalf(
			"expected successful live QPS below attempted QPS: attempted=%f successful=%f",
			completed.
				AttemptedQueriesPerSecond,
			completed.
				SuccessfulQueriesPerSecond,
		)
	}
}

func TestRunnerRunPathWorksWithoutProgressReporter(
	t *testing.T,
) {
	t.Parallel()

	runner := &Runner{
		exchange: func(
			_ context.Context,
			message *dns.Msg,
			_ string,
		) (*dns.Msg, time.Duration, error) {
			response := new(dns.Msg)
			response.SetReply(message)

			return response,
				time.Millisecond,
				nil
		},
	}

	config := validPathConfig()
	config.QueryCount = 2
	config.Concurrency = 1
	config.ProgressReporter = nil

	result, err := runner.RunPath(
		context.Background(),
		config,
	)
	if err != nil {
		t.Fatalf(
			"run benchmark path: %v",
			err,
		)
	}

	if result.Requests.Successful != 2 {
		t.Fatalf(
			"unexpected successful count: got %d, want 2",
			result.Requests.Successful,
		)
	}
}

func TestLiveQueriesPerSecond(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name       string
		queryCount int
		elapsed    time.Duration
		expected   float64
	}{
		{
			name:       "zero queries",
			queryCount: 0,
			elapsed:    time.Second,
			expected:   0,
		},
		{
			name:       "zero elapsed",
			queryCount: 10,
			elapsed:    0,
			expected:   0,
		},
		{
			name:       "one hundred QPS",
			queryCount: 50,
			elapsed:    500 * time.Millisecond,
			expected:   100,
		},
		{
			name:       "fractional QPS",
			queryCount: 1,
			elapsed:    3 * time.Second,
			expected:   0.333333,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				actual := liveQueriesPerSecond(
					test.queryCount,
					test.elapsed,
				)

				if actual != test.expected {
					t.Fatalf(
						"unexpected live QPS: got %f, want %f",
						actual,
						test.expected,
					)
				}
			},
		)
	}
}

func TestShouldReportProgress(
	t *testing.T,
) {
	t.Parallel()

	now := time.Date(
		2026,
		time.August,
		1,
		10,
		0,
		0,
		0,
		time.UTC,
	)

	tests := []struct {
		name           string
		current        int
		total          int
		lastProgressAt time.Time
		expected       bool
	}{
		{
			name:     "no completed query",
			current:  0,
			total:    100,
			expected: false,
		},
		{
			name:     "first progress event",
			current:  1,
			total:    100,
			expected: true,
		},
		{
			name:    "completed path",
			current: 100,
			total:   100,
			lastProgressAt: now.Add(
				-100 * time.Millisecond,
			),
			expected: true,
		},
		{
			name:    "less than one second",
			current: 50,
			total:   100,
			lastProgressAt: now.Add(
				-500 * time.Millisecond,
			),
			expected: false,
		},
		{
			name:    "one second elapsed",
			current: 50,
			total:   100,
			lastProgressAt: now.Add(
				-time.Second,
			),
			expected: true,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(
			test.name,
			func(t *testing.T) {
				t.Parallel()

				actual := shouldReportProgress(
					test.current,
					test.total,
					test.lastProgressAt,
					now,
				)

				if actual != test.expected {
					t.Fatalf(
						"unexpected reporting decision: got %t, want %t",
						actual,
						test.expected,
					)
				}
			},
		)
	}
}

func assertRunnerProgressMetadata(
	t *testing.T,
	event ProgressEvent,
	config PathConfig,
) {
	t.Helper()

	if event.ScenarioName !=
		config.ScenarioName {
		t.Fatalf(
			"unexpected scenario name: got %q, want %q",
			event.ScenarioName,
			config.ScenarioName,
		)
	}

	if event.ScenarioIndex !=
		config.ScenarioIndex {
		t.Fatalf(
			"unexpected scenario index: got %d, want %d",
			event.ScenarioIndex,
			config.ScenarioIndex,
		)
	}

	if event.ScenarioCount !=
		config.ScenarioCount {
		t.Fatalf(
			"unexpected scenario count: got %d, want %d",
			event.ScenarioCount,
			config.ScenarioCount,
		)
	}

	if event.Phase != config.Phase {
		t.Fatalf(
			"unexpected phase: got %q, want %q",
			event.Phase,
			config.Phase,
		)
	}

	if event.Path != "direct" {
		t.Fatalf(
			"unexpected path: got %q, want %q",
			event.Path,
			"direct",
		)
	}

	if !event.BenchmarkStartedAt.Equal(
		config.BenchmarkStartedAt,
	) {
		t.Fatalf(
			"unexpected benchmark start: got %s, want %s",
			event.BenchmarkStartedAt,
			config.BenchmarkStartedAt,
		)
	}
}
