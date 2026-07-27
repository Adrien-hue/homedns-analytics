package health

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

const Route = "/health"

// StateProvider returns the current health state.
//
// The provider is called for every request so readiness and metrics remain
// current without requiring the HTTP package to own runtime state.
type StateProvider func() State

// State contains the runtime information exposed by the health endpoint.
type State struct {
	Ready     bool
	Version   string
	Commit    string
	BuildTime string

	StartedAt time.Time

	DNS      DNSState
	Upstream UpstreamState
	Metrics  MetricsState
}

// DNSState describes the current DNS listener state.
type DNSState struct {
	UDPListening  bool
	TCPListening  bool
	ListenAddress string
}

// UpstreamState describes the configured upstream resolver.
type UpstreamState struct {
	Address string
}

// MetricsState contains the lightweight process-local DNS metrics exposed by
// the health endpoint.
type MetricsState struct {
	RequestsTotal uint64
	RequestsUDP   uint64
	RequestsTCP   uint64

	ResponsesNoError  uint64
	ResponsesNXDomain uint64
	ResponsesSERVFAIL uint64
	ResponsesOther    uint64

	UpstreamTimeouts uint64
	UpstreamErrors   uint64
	InvalidRequests  uint64

	ResponseWriteErrors uint64
	HandlerPanics       uint64

	AverageUpstreamLatencyMilliseconds float64
}

// Response is the JSON representation returned by GET /health.
type Response struct {
	Status string `json:"status"`
	Ready  bool   `json:"ready"`

	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`

	UptimeSeconds uint64 `json:"uptime_seconds"`

	DNS      DNSResponse      `json:"dns"`
	Upstream UpstreamResponse `json:"upstream"`
	Metrics  MetricsResponse  `json:"metrics"`
}

// DNSResponse is the DNS listener section of the health response.
type DNSResponse struct {
	UDPListening  bool   `json:"udp_listening"`
	TCPListening  bool   `json:"tcp_listening"`
	ListenAddress string `json:"listen_address"`
}

// UpstreamResponse is the upstream resolver section of the health response.
type UpstreamResponse struct {
	Address string `json:"address"`
}

// MetricsResponse is the metrics section of the health response.
type MetricsResponse struct {
	RequestsTotal uint64 `json:"requests_total"`
	RequestsUDP   uint64 `json:"requests_udp"`
	RequestsTCP   uint64 `json:"requests_tcp"`

	ResponsesNoError  uint64 `json:"responses_noerror"`
	ResponsesNXDomain uint64 `json:"responses_nxdomain"`
	ResponsesSERVFAIL uint64 `json:"responses_servfail"`
	ResponsesOther    uint64 `json:"responses_other"`

	UpstreamTimeouts uint64 `json:"upstream_timeouts"`
	UpstreamErrors   uint64 `json:"upstream_errors"`
	InvalidRequests  uint64 `json:"invalid_requests"`

	ResponseWriteErrors uint64 `json:"response_write_errors"`
	HandlerPanics       uint64 `json:"handler_panics"`

	AverageUpstreamLatencyMilliseconds float64 `json:"average_upstream_latency_ms"`
}

// Handler serves the HomeDNS health endpoint.
type Handler struct {
	stateProvider StateProvider
	logger        *slog.Logger
	now           func() time.Time
}

// NewHandler creates a health HTTP handler.
func NewHandler(
	stateProvider StateProvider,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		stateProvider: stateProvider,
		logger:        logger,
		now:           time.Now,
	}
}

// ServeHTTP handles health endpoint requests.
func (h *Handler) ServeHTTP(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if request.URL.Path != Route {
		http.NotFound(writer, request)

		return
	}

	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		http.Error(
			writer,
			http.StatusText(http.StatusMethodNotAllowed),
			http.StatusMethodNotAllowed,
		)

		return
	}

	state := State{}
	if h != nil && h.stateProvider != nil {
		state = h.stateProvider()
	}

	response := buildResponse(state, h.currentTime())

	writer.Header().Set(
		"Content-Type",
		"application/json; charset=utf-8",
	)
	writer.Header().Set(
		"Cache-Control",
		"no-store",
	)
	writer.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(writer).Encode(response); err != nil {
		if h != nil && h.logger != nil {
			h.logger.Error(
				"failed to encode health response",
				"error", err,
			)
		}
	}
}

func (h *Handler) currentTime() time.Time {
	if h != nil && h.now != nil {
		return h.now()
	}

	return time.Now()
}

func buildResponse(
	state State,
	now time.Time,
) Response {
	status := "not_ready"
	if state.Ready {
		status = "ok"
	}

	uptimeSeconds := uint64(0)

	if !state.StartedAt.IsZero() && now.After(state.StartedAt) {
		uptimeSeconds = uint64(
			now.Sub(state.StartedAt) / time.Second,
		)
	}

	return Response{
		Status: status,
		Ready:  state.Ready,

		Version:   state.Version,
		Commit:    state.Commit,
		BuildTime: state.BuildTime,

		UptimeSeconds: uptimeSeconds,

		DNS: DNSResponse{
			UDPListening:  state.DNS.UDPListening,
			TCPListening:  state.DNS.TCPListening,
			ListenAddress: state.DNS.ListenAddress,
		},

		Upstream: UpstreamResponse{
			Address: state.Upstream.Address,
		},

		Metrics: MetricsResponse{
			RequestsTotal: state.Metrics.RequestsTotal,
			RequestsUDP:   state.Metrics.RequestsUDP,
			RequestsTCP:   state.Metrics.RequestsTCP,

			ResponsesNoError:  state.Metrics.ResponsesNoError,
			ResponsesNXDomain: state.Metrics.ResponsesNXDomain,
			ResponsesSERVFAIL: state.Metrics.ResponsesSERVFAIL,
			ResponsesOther:    state.Metrics.ResponsesOther,

			UpstreamTimeouts: state.Metrics.UpstreamTimeouts,
			UpstreamErrors:   state.Metrics.UpstreamErrors,
			InvalidRequests:  state.Metrics.InvalidRequests,

			ResponseWriteErrors: state.Metrics.ResponseWriteErrors,
			HandlerPanics:       state.Metrics.HandlerPanics,

			AverageUpstreamLatencyMilliseconds: state.Metrics.
				AverageUpstreamLatencyMilliseconds,
		},
	}
}
