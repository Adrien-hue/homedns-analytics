package dnsserver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"

	"github.com/Adrien-hue/homedns-analytics/dns/internal/metrics"
	"github.com/Adrien-hue/homedns-analytics/dns/internal/upstream"
)

type fakeForwarder struct {
	response *dns.Msg
	duration time.Duration
	err      error

	receivedRequest *dns.Msg
	receivedNetwork upstream.Network
}

type timeoutError struct{}

func (timeoutError) Error() string {
	return "operation timed out"
}

func (timeoutError) Timeout() bool {
	return true
}

func (timeoutError) Temporary() bool {
	return true
}

func (f *fakeForwarder) Exchange(
	_ context.Context,
	request *dns.Msg,
	network upstream.Network,
) (*dns.Msg, time.Duration, error) {
	f.receivedRequest = request
	f.receivedNetwork = network

	return f.response, f.duration, f.err
}

type fakeResponseWriter struct {
	remoteAddress  net.Addr
	writtenMessage *dns.Msg
	writeError     error
}

func (w *fakeResponseWriter) LocalAddr() net.Addr {
	return nil
}

func (w *fakeResponseWriter) RemoteAddr() net.Addr {
	return w.remoteAddress
}

func (w *fakeResponseWriter) WriteMsg(message *dns.Msg) error {
	w.writtenMessage = message

	return w.writeError
}

func (w *fakeResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}

func (w *fakeResponseWriter) Close() error {
	return nil
}

func (w *fakeResponseWriter) TsigStatus() error {
	return nil
}

func (w *fakeResponseWriter) TsigTimersOnly(bool) {}

func (w *fakeResponseWriter) Hijack() {}

type panicForwarder struct{}

func (panicForwarder) Exchange(
	context.Context,
	*dns.Msg,
	upstream.Network,
) (*dns.Msg, time.Duration, error) {
	panic("unexpected forwarding failure")
}

func TestHandlerForwardsUDPRequest(t *testing.T) {
	request := new(dns.Msg)
	request.SetQuestion("example.org.", dns.TypeA)
	request.Id = 1234

	upstreamResponse := new(dns.Msg)
	upstreamResponse.SetReply(request)

	forwarder := &fakeForwarder{
		response: upstreamResponse,
		duration: time.Millisecond,
	}

	writer := &fakeResponseWriter{
		remoteAddress: &net.UDPAddr{
			IP:   net.ParseIP("192.0.2.10"),
			Port: 53000,
		},
	}

	handler := NewHandler(forwarder)
	handler.ServeDNS(writer, request)

	if forwarder.receivedRequest != request {
		t.Fatal("handler did not forward the original request")
	}

	if forwarder.receivedNetwork != upstream.NetworkUDP {
		t.Fatalf(
			"expected network %q, got %q",
			upstream.NetworkUDP,
			forwarder.receivedNetwork,
		)
	}

	if writer.writtenMessage == nil {
		t.Fatal("handler did not write a response")
	}

	if writer.writtenMessage.Id != request.Id {
		t.Fatalf(
			"expected response ID %d, got %d",
			request.Id,
			writer.writtenMessage.Id,
		)
	}
}

func TestHandlerForwardsTCPRequest(t *testing.T) {
	request := new(dns.Msg)
	request.SetQuestion("example.org.", dns.TypeAAAA)

	upstreamResponse := new(dns.Msg)
	upstreamResponse.SetReply(request)

	forwarder := &fakeForwarder{
		response: upstreamResponse,
	}

	writer := &fakeResponseWriter{
		remoteAddress: &net.TCPAddr{
			IP:   net.ParseIP("192.0.2.10"),
			Port: 53000,
		},
	}

	handler := NewHandler(forwarder)
	handler.ServeDNS(writer, request)

	if forwarder.receivedNetwork != upstream.NetworkTCP {
		t.Fatalf(
			"expected network %q, got %q",
			upstream.NetworkTCP,
			forwarder.receivedNetwork,
		)
	}
}

func TestHandlerPreservesRequestID(t *testing.T) {
	request := new(dns.Msg)
	request.SetQuestion("example.org.", dns.TypeA)
	request.Id = 4321

	upstreamResponse := new(dns.Msg)
	upstreamResponse.SetReply(request)
	upstreamResponse.Id = 9999

	forwarder := &fakeForwarder{
		response: upstreamResponse,
	}

	writer := &fakeResponseWriter{
		remoteAddress: &net.UDPAddr{},
	}

	handler := NewHandler(forwarder)
	handler.ServeDNS(writer, request)

	if writer.writtenMessage == nil {
		t.Fatal("handler did not write a response")
	}

	if writer.writtenMessage.Id != request.Id {
		t.Fatalf(
			"expected response ID %d, got %d",
			request.Id,
			writer.writtenMessage.Id,
		)
	}
}

func TestHandlerReturnsSERVFAILWhenForwardingFails(t *testing.T) {
	request := new(dns.Msg)
	request.SetQuestion("example.org.", dns.TypeA)

	forwarder := &fakeForwarder{
		err: errors.New("upstream unavailable"),
	}

	writer := &fakeResponseWriter{
		remoteAddress: &net.UDPAddr{},
	}

	handler := NewHandler(forwarder)
	handler.ServeDNS(writer, request)

	if writer.writtenMessage == nil {
		t.Fatal("handler did not write a response")
	}

	if writer.writtenMessage.Rcode != dns.RcodeServerFailure {
		t.Fatalf(
			"expected rcode %s, got %s",
			dns.RcodeToString[dns.RcodeServerFailure],
			dns.RcodeToString[writer.writtenMessage.Rcode],
		)
	}
}

func TestHandlerReturnsFORMERRForRequestWithoutQuestion(t *testing.T) {
	request := new(dns.Msg)
	request.Id = 1234

	forwarder := &fakeForwarder{}

	writer := &fakeResponseWriter{
		remoteAddress: &net.UDPAddr{},
	}

	handler := NewHandler(forwarder)
	handler.ServeDNS(writer, request)

	if forwarder.receivedRequest != nil {
		t.Fatal("invalid request should not be forwarded")
	}

	if writer.writtenMessage == nil {
		t.Fatal("handler did not write a response")
	}

	if writer.writtenMessage.Rcode != dns.RcodeFormatError {
		t.Fatalf(
			"expected rcode %s, got %s",
			dns.RcodeToString[dns.RcodeFormatError],
			dns.RcodeToString[writer.writtenMessage.Rcode],
		)
	}

	if writer.writtenMessage.Id != request.Id {
		t.Fatalf(
			"expected response ID %d, got %d",
			request.Id,
			writer.writtenMessage.Id,
		)
	}
}

func TestHandlerReturnsSERVFAILWithoutForwarder(t *testing.T) {
	request := new(dns.Msg)
	request.SetQuestion("example.org.", dns.TypeA)

	writer := &fakeResponseWriter{
		remoteAddress: &net.UDPAddr{},
	}

	handler := NewHandler(nil)
	handler.ServeDNS(writer, request)

	if writer.writtenMessage == nil {
		t.Fatal("handler did not write a response")
	}

	if writer.writtenMessage.Rcode != dns.RcodeServerFailure {
		t.Fatalf(
			"expected rcode %s, got %s",
			dns.RcodeToString[dns.RcodeServerFailure],
			dns.RcodeToString[writer.writtenMessage.Rcode],
		)
	}
}

func TestHandlerReturnsSERVFAILForNilUpstreamResponse(t *testing.T) {
	request := new(dns.Msg)
	request.SetQuestion("example.org.", dns.TypeA)

	forwarder := &fakeForwarder{
		response: nil,
		err:      nil,
	}

	writer := &fakeResponseWriter{
		remoteAddress: &net.UDPAddr{},
	}

	handler := NewHandler(forwarder)
	handler.ServeDNS(writer, request)

	if writer.writtenMessage == nil {
		t.Fatal("handler did not write a response")
	}

	if writer.writtenMessage.Rcode != dns.RcodeServerFailure {
		t.Fatalf(
			"expected rcode %s, got %s",
			dns.RcodeToString[dns.RcodeServerFailure],
			dns.RcodeToString[writer.writtenMessage.Rcode],
		)
	}
}

func TestHandlerLogsNormalResponseWriteFailure(t *testing.T) {
	request := new(dns.Msg)
	request.SetQuestion("example.org.", dns.TypeA)

	response := new(dns.Msg)
	response.SetReply(request)

	forwarder := &fakeForwarder{
		response: response,
	}

	writer := &fakeResponseWriter{
		remoteAddress: &net.UDPAddr{},
		writeError:    errors.New("write failed"),
	}

	var logs bytes.Buffer

	logger := slog.New(
		slog.NewTextHandler(&logs, nil),
	)

	handler := NewHandlerWithLogger(forwarder, logger)
	handler.ServeDNS(writer, request)

	if !strings.Contains(
		logs.String(),
		"failed to write DNS response",
	) {
		t.Fatalf(
			"expected response write failure to be logged, got %q",
			logs.String(),
		)
	}
}

func TestHandlerLogsErrorResponseWriteFailure(t *testing.T) {
	request := new(dns.Msg)
	request.SetQuestion("example.org.", dns.TypeA)

	forwarder := &fakeForwarder{
		err: errors.New("upstream unavailable"),
	}

	writer := &fakeResponseWriter{
		remoteAddress: &net.UDPAddr{},
		writeError:    errors.New("write failed"),
	}

	var logs bytes.Buffer

	logger := slog.New(
		slog.NewTextHandler(&logs, nil),
	)

	handler := NewHandlerWithLogger(forwarder, logger)
	handler.ServeDNS(writer, request)

	if !strings.Contains(
		logs.String(),
		"failed to write DNS error response",
	) {
		t.Fatalf(
			"expected error response write failure to be logged, got %q",
			logs.String(),
		)
	}
}

func TestHandlerRecordsSuccessfulUDPRequestMetrics(t *testing.T) {
	request := new(dns.Msg)
	request.SetQuestion("example.org.", dns.TypeA)

	response := new(dns.Msg)
	response.SetReply(request)
	response.Rcode = dns.RcodeSuccess

	forwarder := &fakeForwarder{
		response: response,
		duration: 25 * time.Millisecond,
	}

	writer := &fakeResponseWriter{
		remoteAddress: &net.UDPAddr{},
	}

	registry := &metrics.Registry{}

	handler := NewHandlerWithDependencies(
		forwarder,
		nil,
		registry,
	)

	handler.ServeDNS(writer, request)

	snapshot := registry.Snapshot()

	if snapshot.RequestsTotal != 1 {
		t.Fatalf(
			"expected 1 total request, got %d",
			snapshot.RequestsTotal,
		)
	}

	if snapshot.RequestsUDP != 1 {
		t.Fatalf(
			"expected 1 UDP request, got %d",
			snapshot.RequestsUDP,
		)
	}

	if snapshot.RequestsTCP != 0 {
		t.Fatalf(
			"expected no TCP requests, got %d",
			snapshot.RequestsTCP,
		)
	}

	if snapshot.ResponsesNoError != 1 {
		t.Fatalf(
			"expected 1 NOERROR response, got %d",
			snapshot.ResponsesNoError,
		)
	}

	if snapshot.UpstreamLatencySamples != 1 {
		t.Fatalf(
			"expected 1 latency sample, got %d",
			snapshot.UpstreamLatencySamples,
		)
	}

	if snapshot.TotalUpstreamLatency != 25*time.Millisecond {
		t.Fatalf(
			"expected latency %s, got %s",
			25*time.Millisecond,
			snapshot.TotalUpstreamLatency,
		)
	}
}

func TestHandlerRecordsSuccessfulTCPRequestMetrics(t *testing.T) {
	request := new(dns.Msg)
	request.SetQuestion("example.org.", dns.TypeA)

	response := new(dns.Msg)
	response.SetReply(request)
	response.Rcode = dns.RcodeNameError

	forwarder := &fakeForwarder{
		response: response,
		duration: 10 * time.Millisecond,
	}

	writer := &fakeResponseWriter{
		remoteAddress: &net.TCPAddr{},
	}

	registry := &metrics.Registry{}

	handler := NewHandlerWithDependencies(
		forwarder,
		nil,
		registry,
	)

	handler.ServeDNS(writer, request)

	snapshot := registry.Snapshot()

	if snapshot.RequestsTotal != 1 {
		t.Fatalf(
			"expected 1 total request, got %d",
			snapshot.RequestsTotal,
		)
	}

	if snapshot.RequestsTCP != 1 {
		t.Fatalf(
			"expected 1 TCP request, got %d",
			snapshot.RequestsTCP,
		)
	}

	if snapshot.ResponsesNXDomain != 1 {
		t.Fatalf(
			"expected 1 NXDOMAIN response, got %d",
			snapshot.ResponsesNXDomain,
		)
	}
}

func TestHandlerRecordsUpstreamFailureMetrics(t *testing.T) {
	request := new(dns.Msg)
	request.SetQuestion("example.org.", dns.TypeA)

	forwarder := &fakeForwarder{
		duration: 50 * time.Millisecond,
		err:      errors.New("upstream unavailable"),
	}

	writer := &fakeResponseWriter{
		remoteAddress: &net.UDPAddr{},
	}

	registry := &metrics.Registry{}

	handler := NewHandlerWithDependencies(
		forwarder,
		nil,
		registry,
	)

	handler.ServeDNS(writer, request)

	snapshot := registry.Snapshot()

	if snapshot.RequestsTotal != 1 {
		t.Fatalf(
			"expected 1 total request, got %d",
			snapshot.RequestsTotal,
		)
	}

	if snapshot.UpstreamErrors != 1 {
		t.Fatalf(
			"expected 1 upstream error, got %d",
			snapshot.UpstreamErrors,
		)
	}

	if snapshot.ResponsesSERVFAIL != 1 {
		t.Fatalf(
			"expected 1 SERVFAIL response, got %d",
			snapshot.ResponsesSERVFAIL,
		)
	}

	if snapshot.UpstreamLatencySamples != 1 {
		t.Fatalf(
			"expected 1 latency sample, got %d",
			snapshot.UpstreamLatencySamples,
		)
	}

	if snapshot.TotalUpstreamLatency != 50*time.Millisecond {
		t.Fatalf(
			"expected latency %s, got %s",
			50*time.Millisecond,
			snapshot.TotalUpstreamLatency,
		)
	}
}

func TestHandlerRecordsInvalidRequestMetrics(t *testing.T) {
	request := new(dns.Msg)

	writer := &fakeResponseWriter{
		remoteAddress: &net.UDPAddr{},
	}

	registry := &metrics.Registry{}

	handler := NewHandlerWithDependencies(
		&fakeForwarder{},
		nil,
		registry,
	)

	handler.ServeDNS(writer, request)

	snapshot := registry.Snapshot()

	if snapshot.InvalidRequests != 1 {
		t.Fatalf(
			"expected 1 invalid request, got %d",
			snapshot.InvalidRequests,
		)
	}

	if snapshot.ResponsesOther != 1 {
		t.Fatalf(
			"expected 1 other response for FORMERR, got %d",
			snapshot.ResponsesOther,
		)
	}

	if snapshot.RequestsTotal != 0 {
		t.Fatalf(
			"expected no classified requests, got %d",
			snapshot.RequestsTotal,
		)
	}
}

func TestHandlerRecordsNilUpstreamResponseMetrics(t *testing.T) {
	request := new(dns.Msg)
	request.SetQuestion("example.org.", dns.TypeA)

	forwarder := &fakeForwarder{
		response: nil,
		duration: 15 * time.Millisecond,
		err:      nil,
	}

	writer := &fakeResponseWriter{
		remoteAddress: &net.UDPAddr{},
	}

	registry := &metrics.Registry{}

	handler := NewHandlerWithDependencies(
		forwarder,
		nil,
		registry,
	)

	handler.ServeDNS(writer, request)

	snapshot := registry.Snapshot()

	if snapshot.UpstreamErrors != 1 {
		t.Fatalf(
			"expected 1 upstream error, got %d",
			snapshot.UpstreamErrors,
		)
	}

	if snapshot.ResponsesSERVFAIL != 1 {
		t.Fatalf(
			"expected 1 SERVFAIL response, got %d",
			snapshot.ResponsesSERVFAIL,
		)
	}

	if snapshot.UpstreamLatencySamples != 1 {
		t.Fatalf(
			"expected 1 latency sample, got %d",
			snapshot.UpstreamLatencySamples,
		)
	}
}

func TestHandlerRecordsResponseWriteFailureMetrics(t *testing.T) {
	request := new(dns.Msg)
	request.SetQuestion("example.org.", dns.TypeA)

	response := new(dns.Msg)
	response.SetReply(request)

	forwarder := &fakeForwarder{
		response: response,
	}

	writer := &fakeResponseWriter{
		remoteAddress: &net.UDPAddr{},
		writeError:    errors.New("write failed"),
	}

	registry := &metrics.Registry{}

	handler := NewHandlerWithDependencies(
		forwarder,
		nil,
		registry,
	)

	handler.ServeDNS(writer, request)

	snapshot := registry.Snapshot()

	if snapshot.ResponseWriteErrors != 1 {
		t.Fatalf(
			"expected 1 response write error, got %d",
			snapshot.ResponseWriteErrors,
		)
	}

	if snapshot.ResponsesNoError != 1 {
		t.Fatalf(
			"expected 1 produced NOERROR response, got %d",
			snapshot.ResponsesNoError,
		)
	}
}

func TestIsTimeoutError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "deadline exceeded",
			err:      context.DeadlineExceeded,
			expected: true,
		},
		{
			name: "wrapped deadline exceeded",
			err: fmt.Errorf(
				"upstream request failed: %w",
				context.DeadlineExceeded,
			),
			expected: true,
		},
		{
			name:     "network timeout",
			err:      timeoutError{},
			expected: true,
		},
		{
			name: "wrapped network timeout",
			err: fmt.Errorf(
				"DNS exchange failed: %w",
				timeoutError{},
			),
			expected: true,
		},
		{
			name:     "ordinary error",
			err:      errors.New("upstream unavailable"),
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := isTimeoutError(test.err)

			if actual != test.expected {
				t.Fatalf(
					"expected timeout classification %t, got %t",
					test.expected,
					actual,
				)
			}
		})
	}
}

func TestHandlerRecordsUpstreamTimeoutMetrics(t *testing.T) {
	request := new(dns.Msg)
	request.SetQuestion("example.org.", dns.TypeA)

	forwarder := &fakeForwarder{
		duration: 75 * time.Millisecond,
		err:      context.DeadlineExceeded,
	}

	writer := &fakeResponseWriter{
		remoteAddress: &net.UDPAddr{},
	}

	registry := &metrics.Registry{}

	handler := NewHandlerWithDependencies(
		forwarder,
		nil,
		registry,
	)

	handler.ServeDNS(writer, request)

	snapshot := registry.Snapshot()

	if snapshot.RequestsTotal != 1 {
		t.Fatalf(
			"expected 1 total request, got %d",
			snapshot.RequestsTotal,
		)
	}

	if snapshot.UpstreamTimeouts != 1 {
		t.Fatalf(
			"expected 1 upstream timeout, got %d",
			snapshot.UpstreamTimeouts,
		)
	}

	if snapshot.UpstreamErrors != 0 {
		t.Fatalf(
			"expected no generic upstream errors, got %d",
			snapshot.UpstreamErrors,
		)
	}

	if snapshot.ResponsesSERVFAIL != 1 {
		t.Fatalf(
			"expected 1 SERVFAIL response, got %d",
			snapshot.ResponsesSERVFAIL,
		)
	}

	if snapshot.UpstreamLatencySamples != 1 {
		t.Fatalf(
			"expected 1 latency sample, got %d",
			snapshot.UpstreamLatencySamples,
		)
	}

	if snapshot.TotalUpstreamLatency != 75*time.Millisecond {
		t.Fatalf(
			"expected latency %s, got %s",
			75*time.Millisecond,
			snapshot.TotalUpstreamLatency,
		)
	}
}

func TestHandlerRecoversFromPanic(t *testing.T) {
	request := new(dns.Msg)
	request.SetQuestion("example.org.", dns.TypeA)

	writer := &fakeResponseWriter{
		remoteAddress: &net.UDPAddr{},
	}

	registry := &metrics.Registry{}

	handler := NewHandlerWithDependencies(
		panicForwarder{},
		nil,
		registry,
	)

	handler.ServeDNS(writer, request)

	if writer.writtenMessage == nil {
		t.Fatal("expected SERVFAIL response")
	}

	if writer.writtenMessage.Rcode != dns.RcodeServerFailure {
		t.Fatalf(
			"expected SERVFAIL, got %s",
			dns.RcodeToString[writer.writtenMessage.Rcode],
		)
	}

	snapshot := registry.Snapshot()

	if snapshot.HandlerPanics != 1 {
		t.Fatalf(
			"expected 1 handler panic, got %d",
			snapshot.HandlerPanics,
		)
	}
}

func TestHandlerLogsRecoveredPanic(t *testing.T) {
	request := new(dns.Msg)
	request.SetQuestion("example.org.", dns.TypeA)

	writer := &fakeResponseWriter{
		remoteAddress: &net.UDPAddr{},
	}

	var logs bytes.Buffer

	logger := slog.New(
		slog.NewTextHandler(&logs, nil),
	)

	handler := NewHandlerWithDependencies(
		panicForwarder{},
		logger,
		&metrics.Registry{},
	)

	handler.ServeDNS(writer, request)

	if !strings.Contains(
		logs.String(),
		"panic while handling DNS request",
	) {
		t.Fatalf(
			"expected panic log, got %q",
			logs.String(),
		)
	}
}
