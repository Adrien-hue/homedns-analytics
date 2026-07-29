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

	if result.Requests != 4 {
		t.Fatalf(
			"unexpected request count: got %d, want %d",
			result.Requests,
			4,
		)
	}

	if result.Successful != 4 {
		t.Fatalf(
			"unexpected successful count: got %d, want %d",
			result.Successful,
			4,
		)
	}

	if result.Failed != 0 {
		t.Fatalf(
			"unexpected failed count: got %d, want %d",
			result.Failed,
			0,
		)
	}

	if result.Timeouts != 0 {
		t.Fatalf(
			"unexpected timeout count: got %d, want %d",
			result.Timeouts,
			0,
		)
	}

	if result.LatencyMilliseconds.Mean != 2 {
		t.Fatalf(
			"unexpected mean latency: got %f, want %f",
			result.LatencyMilliseconds.Mean,
			2.0,
		)
	}

	if result.QueriesPerSecond <= 0 {
		t.Fatalf(
			"expected positive QPS, got %f",
			result.QueriesPerSecond,
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
}

func TestRunnerRunPathRecordsFailures(t *testing.T) {
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

	if result.Successful != 1 {
		t.Fatalf(
			"unexpected successful count: got %d, want %d",
			result.Successful,
			1,
		)
	}

	if result.Failed != 1 {
		t.Fatalf(
			"unexpected failed count: got %d, want %d",
			result.Failed,
			1,
		)
	}

	if result.Timeouts != 0 {
		t.Fatalf(
			"unexpected timeout count: got %d, want %d",
			result.Timeouts,
			0,
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

	if result.Successful != 0 {
		t.Fatalf(
			"unexpected successful count: got %d, want %d",
			result.Successful,
			0,
		)
	}

	if result.Failed != 3 {
		t.Fatalf(
			"unexpected failed count: got %d, want %d",
			result.Failed,
			3,
		)
	}

	if result.Timeouts != 3 {
		t.Fatalf(
			"unexpected timeout count: got %d, want %d",
			result.Timeouts,
			3,
		)
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
