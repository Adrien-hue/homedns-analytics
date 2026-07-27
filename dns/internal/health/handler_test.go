package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandlerReturnsHealthResponse(t *testing.T) {
	startedAt := time.Date(
		2026,
		time.July,
		26,
		10,
		0,
		0,
		0,
		time.UTC,
	)

	now := startedAt.Add(
		2*time.Minute + 22*time.Second,
	)

	handler := NewHandler(
		func() State {
			return State{
				Ready:     true,
				Version:   "v0.2.0",
				Commit:    "a3f8d21",
				BuildTime: "2026-07-24T18:00:00Z",
				StartedAt: startedAt,

				DNS: DNSState{
					UDPListening:  true,
					TCPListening:  true,
					ListenAddress: "0.0.0.0:5353",
				},

				Upstream: UpstreamState{
					Address: "1.1.1.1:53",
				},

				Metrics: MetricsState{
					RequestsTotal: 1457,
					RequestsUDP:   1431,
					RequestsTCP:   26,

					ResponsesNoError:  1392,
					ResponsesNXDomain: 41,
					ResponsesSERVFAIL: 24,
					ResponsesOther:    0,

					UpstreamTimeouts: 3,
					UpstreamErrors:   2,
					InvalidRequests:  4,

					ResponseWriteErrors: 1,
					HandlerPanics:       0,

					AverageUpstreamLatencyMilliseconds: 16.4,
				},
			}
		},
		nil,
	)

	handler.now = func() time.Time {
		return now
	}

	request := httptest.NewRequest(
		http.MethodGet,
		Route,
		nil,
	)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"unexpected status code: got %d, want %d",
			recorder.Code,
			http.StatusOK,
		)
	}

	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json; charset=utf-8" {
		t.Fatalf(
			"unexpected content type: got %q",
			contentType,
		)
	}

	if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf(
			"unexpected cache control: got %q",
			cacheControl,
		)
	}

	var response Response

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode health response: %v", err)
	}

	if response.Status != "ok" {
		t.Fatalf(
			"unexpected health status: got %q, want %q",
			response.Status,
			"ok",
		)
	}

	if !response.Ready {
		t.Fatal("expected response to report ready")
	}

	if response.Version != "v0.2.0" {
		t.Fatalf(
			"unexpected version: got %q",
			response.Version,
		)
	}

	if response.Commit != "a3f8d21" {
		t.Fatalf(
			"unexpected commit: got %q",
			response.Commit,
		)
	}

	if response.UptimeSeconds != 142 {
		t.Fatalf(
			"unexpected uptime: got %d, want 142",
			response.UptimeSeconds,
		)
	}

	if !response.DNS.UDPListening {
		t.Fatal("expected UDP listener to be reported active")
	}

	if !response.DNS.TCPListening {
		t.Fatal("expected TCP listener to be reported active")
	}

	if response.DNS.ListenAddress != "0.0.0.0:5353" {
		t.Fatalf(
			"unexpected DNS address: got %q",
			response.DNS.ListenAddress,
		)
	}

	if response.Upstream.Address != "1.1.1.1:53" {
		t.Fatalf(
			"unexpected upstream address: got %q",
			response.Upstream.Address,
		)
	}

	if response.Metrics.RequestsTotal != 1457 {
		t.Fatalf(
			"unexpected total requests: got %d",
			response.Metrics.RequestsTotal,
		)
	}

	if response.Metrics.AverageUpstreamLatencyMilliseconds != 16.4 {
		t.Fatalf(
			"unexpected average latency: got %f",
			response.Metrics.AverageUpstreamLatencyMilliseconds,
		)
	}

	if response.Metrics.InvalidRequests != 4 {
		t.Fatalf(
			"unexpected invalid request count: got %d, want %d",
			response.Metrics.InvalidRequests,
			4,
		)
	}
}

func TestHandlerReportsNotReady(t *testing.T) {
	handler := NewHandler(
		func() State {
			return State{
				Ready: false,
			}
		},
		nil,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		Route,
		nil,
	)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"unexpected status code: got %d, want %d",
			recorder.Code,
			http.StatusOK,
		)
	}

	var response Response

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode health response: %v", err)
	}

	if response.Status != "not_ready" {
		t.Fatalf(
			"unexpected health status: got %q, want %q",
			response.Status,
			"not_ready",
		)
	}

	if response.Ready {
		t.Fatal("expected response not to report ready")
	}
}

func TestHandlerRejectsUnsupportedMethod(t *testing.T) {
	handler := NewHandler(
		func() State {
			return State{}
		},
		nil,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		Route,
		nil,
	)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf(
			"unexpected status code: got %d, want %d",
			recorder.Code,
			http.StatusMethodNotAllowed,
		)
	}

	if allow := recorder.Header().Get("Allow"); allow != http.MethodGet {
		t.Fatalf(
			"unexpected Allow header: got %q, want %q",
			allow,
			http.MethodGet,
		)
	}
}

func TestHandlerReturnsNotFoundForUnknownRoute(t *testing.T) {
	handler := NewHandler(
		func() State {
			return State{}
		},
		nil,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/unknown",
		nil,
	)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf(
			"unexpected status code: got %d, want %d",
			recorder.Code,
			http.StatusNotFound,
		)
	}
}

func TestHandlerWithoutStateProviderIsSafe(t *testing.T) {
	handler := NewHandler(nil, nil)

	request := httptest.NewRequest(
		http.MethodGet,
		Route,
		nil,
	)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"unexpected status code: got %d, want %d",
			recorder.Code,
			http.StatusOK,
		)
	}

	var response Response

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode health response: %v", err)
	}

	if response.Ready {
		t.Fatal("expected missing state provider not to report ready")
	}

	if response.Status != "not_ready" {
		t.Fatalf(
			"unexpected status: got %q, want %q",
			response.Status,
			"not_ready",
		)
	}
}

func TestBuildResponseDoesNotReturnNegativeUptime(t *testing.T) {
	now := time.Date(
		2026,
		time.July,
		26,
		10,
		0,
		0,
		0,
		time.UTC,
	)

	response := buildResponse(
		State{
			StartedAt: now.Add(time.Minute),
		},
		now,
	)

	if response.UptimeSeconds != 0 {
		t.Fatalf(
			"unexpected uptime: got %d, want 0",
			response.UptimeSeconds,
		)
	}
}
