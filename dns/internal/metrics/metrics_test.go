package metrics

import (
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestRegistryRecordsRequests(t *testing.T) {
	registry := &Registry{}

	registry.RecordRequestUDP()
	registry.RecordRequestUDP()
	registry.RecordRequestTCP()

	snapshot := registry.Snapshot()

	if snapshot.RequestsTotal != 3 {
		t.Fatalf(
			"expected 3 total requests, got %d",
			snapshot.RequestsTotal,
		)
	}

	if snapshot.RequestsUDP != 2 {
		t.Fatalf(
			"expected 2 UDP requests, got %d",
			snapshot.RequestsUDP,
		)
	}

	if snapshot.RequestsTCP != 1 {
		t.Fatalf(
			"expected 1 TCP request, got %d",
			snapshot.RequestsTCP,
		)
	}
}

func TestRegistryRecordsResponses(t *testing.T) {
	registry := &Registry{}

	registry.RecordResponse(dns.RcodeSuccess)
	registry.RecordResponse(dns.RcodeNameError)
	registry.RecordResponse(dns.RcodeServerFailure)
	registry.RecordResponse(dns.RcodeRefused)

	snapshot := registry.Snapshot()

	if snapshot.ResponsesNoError != 1 {
		t.Fatalf(
			"expected 1 NOERROR response, got %d",
			snapshot.ResponsesNoError,
		)
	}

	if snapshot.ResponsesNXDomain != 1 {
		t.Fatalf(
			"expected 1 NXDOMAIN response, got %d",
			snapshot.ResponsesNXDomain,
		)
	}

	if snapshot.ResponsesSERVFAIL != 1 {
		t.Fatalf(
			"expected 1 SERVFAIL response, got %d",
			snapshot.ResponsesSERVFAIL,
		)
	}

	if snapshot.ResponsesOther != 1 {
		t.Fatalf(
			"expected 1 other response, got %d",
			snapshot.ResponsesOther,
		)
	}
}

func TestRegistryRecordsErrors(t *testing.T) {
	registry := &Registry{}

	registry.RecordUpstreamTimeout()
	registry.RecordUpstreamError()
	registry.RecordInvalidRequest()
	registry.RecordResponseWriteError()

	snapshot := registry.Snapshot()

	if snapshot.UpstreamTimeouts != 1 {
		t.Fatalf(
			"expected 1 upstream timeout, got %d",
			snapshot.UpstreamTimeouts,
		)
	}

	if snapshot.UpstreamErrors != 1 {
		t.Fatalf(
			"expected 1 upstream error, got %d",
			snapshot.UpstreamErrors,
		)
	}

	if snapshot.InvalidRequests != 1 {
		t.Fatalf(
			"expected 1 invalid request, got %d",
			snapshot.InvalidRequests,
		)
	}

	if snapshot.ResponseWriteErrors != 1 {
		t.Fatalf(
			"expected 1 response write error, got %d",
			snapshot.ResponseWriteErrors,
		)
	}
}

func TestRegistryCalculatesAverageUpstreamLatency(t *testing.T) {
	registry := &Registry{}

	registry.RecordUpstreamLatency(10 * time.Millisecond)
	registry.RecordUpstreamLatency(20 * time.Millisecond)
	registry.RecordUpstreamLatency(30 * time.Millisecond)

	snapshot := registry.Snapshot()

	if snapshot.UpstreamLatencySamples != 3 {
		t.Fatalf(
			"expected 3 latency samples, got %d",
			snapshot.UpstreamLatencySamples,
		)
	}

	expectedTotal := 60 * time.Millisecond
	if snapshot.TotalUpstreamLatency != expectedTotal {
		t.Fatalf(
			"expected total latency %s, got %s",
			expectedTotal,
			snapshot.TotalUpstreamLatency,
		)
	}

	expectedAverage := 20 * time.Millisecond
	if snapshot.AverageUpstreamLatency != expectedAverage {
		t.Fatalf(
			"expected average latency %s, got %s",
			expectedAverage,
			snapshot.AverageUpstreamLatency,
		)
	}
}

func TestRegistryIgnoresNegativeLatency(t *testing.T) {
	registry := &Registry{}

	registry.RecordUpstreamLatency(-time.Millisecond)

	snapshot := registry.Snapshot()

	if snapshot.UpstreamLatencySamples != 0 {
		t.Fatalf(
			"expected no latency samples, got %d",
			snapshot.UpstreamLatencySamples,
		)
	}

	if snapshot.TotalUpstreamLatency != 0 {
		t.Fatalf(
			"expected zero total latency, got %s",
			snapshot.TotalUpstreamLatency,
		)
	}
}

func TestNilRegistryIsSafe(t *testing.T) {
	var registry *Registry

	registry.RecordRequestUDP()
	registry.RecordRequestTCP()
	registry.RecordResponse(dns.RcodeSuccess)
	registry.RecordUpstreamTimeout()
	registry.RecordUpstreamError()
	registry.RecordInvalidRequest()
	registry.RecordResponseWriteError()
	registry.RecordUpstreamLatency(time.Millisecond)

	snapshot := registry.Snapshot()

	if snapshot != (Snapshot{}) {
		t.Fatalf(
			"expected empty snapshot, got %+v",
			snapshot,
		)
	}
}

func TestRegistrySupportsConcurrentUpdates(t *testing.T) {
	registry := &Registry{}

	const goroutines = 100
	const incrementsPerGoroutine = 100

	var waitGroup sync.WaitGroup
	waitGroup.Add(goroutines)

	for range goroutines {
		go func() {
			defer waitGroup.Done()

			for range incrementsPerGoroutine {
				registry.RecordRequestUDP()
				registry.RecordResponse(dns.RcodeSuccess)
				registry.RecordUpstreamLatency(time.Millisecond)
			}
		}()
	}

	waitGroup.Wait()

	snapshot := registry.Snapshot()
	expected := uint64(goroutines * incrementsPerGoroutine)

	if snapshot.RequestsTotal != expected {
		t.Fatalf(
			"expected %d total requests, got %d",
			expected,
			snapshot.RequestsTotal,
		)
	}

	if snapshot.ResponsesNoError != expected {
		t.Fatalf(
			"expected %d NOERROR responses, got %d",
			expected,
			snapshot.ResponsesNoError,
		)
	}

	if snapshot.UpstreamLatencySamples != expected {
		t.Fatalf(
			"expected %d latency samples, got %d",
			expected,
			snapshot.UpstreamLatencySamples,
		)
	}
}
