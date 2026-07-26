package dnsserver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"

	"github.com/miekg/dns"

	"github.com/Adrien-hue/homedns-analytics/dns/internal/metrics"
	"github.com/Adrien-hue/homedns-analytics/dns/internal/upstream"
)

// Handler processes incoming DNS requests and forwards them to an upstream
// resolver.
type Handler struct {
	forwarder upstream.Forwarder
	logger    *slog.Logger
	metrics   *metrics.Registry
}

// Compile-time verification that Handler implements dns.Handler.
var _ dns.Handler = (*Handler)(nil)

// NewHandler creates a DNS handler with default logging and metrics
// dependencies.
func NewHandler(forwarder upstream.Forwarder) *Handler {
	return NewHandlerWithDependencies(
		forwarder,
		nil,
		nil,
	)
}

// NewHandlerWithLogger creates a DNS handler with the provided logger and a
// default metrics registry.
func NewHandlerWithLogger(
	forwarder upstream.Forwarder,
	logger *slog.Logger,
) *Handler {
	return NewHandlerWithDependencies(
		forwarder,
		logger,
		nil,
	)
}

// NewHandlerWithDependencies creates a DNS handler with explicitly provided
// dependencies.
//
// A discard logger and an empty metrics registry are created when the
// corresponding dependencies are nil.
func NewHandlerWithDependencies(
	forwarder upstream.Forwarder,
	logger *slog.Logger,
	registry *metrics.Registry,
) *Handler {
	if logger == nil {
		logger = slog.New(
			slog.NewTextHandler(io.Discard, nil),
		)
	}

	if registry == nil {
		registry = &metrics.Registry{}
	}

	return &Handler{
		forwarder: forwarder,
		logger:    logger,
		metrics:   registry,
	}
}

// ServeDNS handles one DNS request and protects the DNS server from unexpected
// panics in the request-processing path.
func (h *Handler) ServeDNS(
	writer dns.ResponseWriter,
	request *dns.Msg,
) {
	defer h.recoverPanic(writer, request)

	h.serveDNS(writer, request)
}

// serveDNS contains the normal DNS request-processing flow.
func (h *Handler) serveDNS(
	writer dns.ResponseWriter,
	request *dns.Msg,
) {
	if request == nil {
		h.metrics.RecordInvalidRequest()
		return
	}

	if len(request.Question) == 0 {
		h.metrics.RecordInvalidRequest()

		h.writeErrorResponse(
			writer,
			request,
			dns.RcodeFormatError,
		)
		return
	}

	network := requestNetwork(writer)

	switch network {
	case upstream.NetworkTCP:
		h.metrics.RecordRequestTCP()
	default:
		h.metrics.RecordRequestUDP()
	}

	if h.forwarder == nil {
		h.metrics.RecordUpstreamError()

		h.writeErrorResponse(
			writer,
			request,
			dns.RcodeServerFailure,
		)
		return
	}

	response, latency, err := h.forwarder.Exchange(
		context.Background(),
		request,
		network,
	)

	h.metrics.RecordUpstreamLatency(latency)

	if err != nil {
		timedOut := isTimeoutError(err)

		if timedOut {
			h.metrics.RecordUpstreamTimeout()
		} else {
			h.metrics.RecordUpstreamError()
		}

		h.logger.Error(
			"failed to forward DNS request",
			"network", network,
			"latency", latency,
			"timeout", timedOut,
			"error", err,
		)

		h.writeErrorResponse(
			writer,
			request,
			dns.RcodeServerFailure,
		)
		return
	}

	if response == nil {
		h.metrics.RecordUpstreamError()

		h.logger.Error(
			"upstream returned nil DNS response",
			"network", network,
			"latency", latency,
		)

		h.writeErrorResponse(
			writer,
			request,
			dns.RcodeServerFailure,
		)
		return
	}

	response.Id = request.Id

	h.metrics.RecordResponse(response.Rcode)

	if err := writer.WriteMsg(response); err != nil {
		h.metrics.RecordResponseWriteError()

		h.logger.Error(
			"failed to write DNS response",
			"network", network,
			"rcode", response.Rcode,
			"error", err,
		)
	}
}

// recoverPanic prevents an unexpected panic while handling one request from
// terminating the DNS server process.
func (h *Handler) recoverPanic(
	writer dns.ResponseWriter,
	request *dns.Msg,
) {
	recovered := recover()
	if recovered == nil {
		return
	}

	h.metrics.RecordHandlerPanic()

	h.logger.Error(
		"panic while handling DNS request",
		"panic", recovered,
	)

	if writer == nil || request == nil {
		return
	}

	h.writeErrorResponse(
		writer,
		request,
		dns.RcodeServerFailure,
	)
}

// requestNetwork determines whether a request was received through TCP or UDP.
func requestNetwork(writer dns.ResponseWriter) upstream.Network {
	if writer == nil {
		return upstream.NetworkUDP
	}

	switch writer.RemoteAddr().(type) {
	case *net.TCPAddr:
		return upstream.NetworkTCP
	default:
		return upstream.NetworkUDP
	}
}

// isTimeoutError identifies context deadline failures and network errors that
// explicitly report timeout behavior.
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var networkError net.Error

	return errors.As(err, &networkError) &&
		networkError.Timeout()
}

// writeErrorResponse creates and writes a DNS error response.
func (h *Handler) writeErrorResponse(
	writer dns.ResponseWriter,
	request *dns.Msg,
	responseCode int,
) {
	if writer == nil || request == nil {
		return
	}

	response := new(dns.Msg)
	response.SetRcode(request, responseCode)

	h.metrics.RecordResponse(responseCode)

	if err := writer.WriteMsg(response); err != nil {
		h.metrics.RecordResponseWriteError()

		h.logger.Error(
			"failed to write DNS error response",
			"rcode", responseCode,
			"error", err,
		)
	}
}
