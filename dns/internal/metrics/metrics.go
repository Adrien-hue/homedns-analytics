package metrics

import (
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
)

// Registry stores lightweight process-local DNS runtime metrics.
//
// All fields use atomic operations because DNS requests may be handled
// concurrently by multiple goroutines.
type Registry struct {
	requestsTotal atomic.Uint64
	requestsUDP   atomic.Uint64
	requestsTCP   atomic.Uint64

	responsesNoError  atomic.Uint64
	responsesNXDomain atomic.Uint64
	responsesSERVFAIL atomic.Uint64
	responsesOther    atomic.Uint64

	upstreamTimeouts    atomic.Uint64
	upstreamErrors      atomic.Uint64
	invalidRequests     atomic.Uint64
	responseWriteErrors atomic.Uint64

	upstreamLatencyNanoseconds atomic.Uint64
	upstreamLatencySamples     atomic.Uint64

	handlerPanics atomic.Uint64
}

// Snapshot is an immutable copy of the current runtime metrics.
type Snapshot struct {
	RequestsTotal uint64
	RequestsUDP   uint64
	RequestsTCP   uint64

	ResponsesNoError  uint64
	ResponsesNXDomain uint64
	ResponsesSERVFAIL uint64
	ResponsesOther    uint64

	UpstreamTimeouts    uint64
	UpstreamErrors      uint64
	InvalidRequests     uint64
	ResponseWriteErrors uint64

	TotalUpstreamLatency   time.Duration
	UpstreamLatencySamples uint64
	AverageUpstreamLatency time.Duration

	HandlerPanics uint64
}

func (r *Registry) RecordRequestUDP() {
	if r == nil {
		return
	}

	r.requestsTotal.Add(1)
	r.requestsUDP.Add(1)
}

func (r *Registry) RecordRequestTCP() {
	if r == nil {
		return
	}

	r.requestsTotal.Add(1)
	r.requestsTCP.Add(1)
}

func (r *Registry) RecordResponse(responseCode int) {
	if r == nil {
		return
	}

	switch responseCode {
	case dns.RcodeSuccess:
		r.responsesNoError.Add(1)
	case dns.RcodeNameError:
		r.responsesNXDomain.Add(1)
	case dns.RcodeServerFailure:
		r.responsesSERVFAIL.Add(1)
	default:
		r.responsesOther.Add(1)
	}
}

func (r *Registry) RecordUpstreamTimeout() {
	if r == nil {
		return
	}

	r.upstreamTimeouts.Add(1)
}

func (r *Registry) RecordUpstreamError() {
	if r == nil {
		return
	}

	r.upstreamErrors.Add(1)
}

func (r *Registry) RecordInvalidRequest() {
	if r == nil {
		return
	}

	r.invalidRequests.Add(1)
}

func (r *Registry) RecordResponseWriteError() {
	if r == nil {
		return
	}

	r.responseWriteErrors.Add(1)
}

func (r *Registry) RecordUpstreamLatency(latency time.Duration) {
	if r == nil || latency < 0 {
		return
	}

	r.upstreamLatencyNanoseconds.Add(uint64(latency))
	r.upstreamLatencySamples.Add(1)
}

func (r *Registry) RecordHandlerPanic() {
	if r == nil {
		return
	}

	r.handlerPanics.Add(1)
}

func (r *Registry) Snapshot() Snapshot {
	if r == nil {
		return Snapshot{}
	}

	totalLatency := time.Duration(
		r.upstreamLatencyNanoseconds.Load(),
	)
	samples := r.upstreamLatencySamples.Load()

	var averageLatency time.Duration
	if samples > 0 {
		averageLatency = totalLatency / time.Duration(samples)
	}

	return Snapshot{
		RequestsTotal: r.requestsTotal.Load(),
		RequestsUDP:   r.requestsUDP.Load(),
		RequestsTCP:   r.requestsTCP.Load(),

		ResponsesNoError:  r.responsesNoError.Load(),
		ResponsesNXDomain: r.responsesNXDomain.Load(),
		ResponsesSERVFAIL: r.responsesSERVFAIL.Load(),
		ResponsesOther:    r.responsesOther.Load(),

		UpstreamTimeouts:    r.upstreamTimeouts.Load(),
		UpstreamErrors:      r.upstreamErrors.Load(),
		InvalidRequests:     r.invalidRequests.Load(),
		ResponseWriteErrors: r.responseWriteErrors.Load(),

		TotalUpstreamLatency:   totalLatency,
		UpstreamLatencySamples: samples,
		AverageUpstreamLatency: averageLatency,

		HandlerPanics: r.handlerPanics.Load(),
	}
}
