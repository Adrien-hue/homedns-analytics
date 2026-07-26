package upstream

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/Adrien-hue/homedns-analytics/dns/internal/testutil/dnstest"
	"github.com/miekg/dns"
)

func TestClientExchangesARecordOverUDP(t *testing.T) {
	// Arrange
	upstreamServer := dnstest.Start(t)

	client := NewClient(
		upstreamServer.Address(),
		time.Second,
	)

	request := new(dns.Msg)
	request.SetQuestion(
		dns.Fqdn("example.test"),
		dns.TypeA,
	)

	// Act
	response, duration, err := client.Exchange(
		context.Background(),
		request,
		NetworkUDP,
	)

	// Assert
	if err != nil {
		t.Fatalf("Exchange() returned an unexpected error: %v", err)
	}

	if response == nil {
		t.Fatal("Exchange() returned a nil response")
	}

	if duration <= 0 {
		t.Errorf("Exchange() duration = %s, want a positive duration", duration)
	}

	if response.Rcode != dns.RcodeSuccess {
		t.Errorf(
			"Exchange() response code = %s, want %s",
			dns.RcodeToString[response.Rcode],
			dns.RcodeToString[dns.RcodeSuccess],
		)
	}

	if len(response.Answer) != 1 {
		t.Fatalf(
			"Exchange() returned %d answers, want 1",
			len(response.Answer),
		)
	}

	answer, ok := response.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf(
			"Exchange() answer type = %T, want *dns.A",
			response.Answer[0],
		)
	}

	expectedIP := net.ParseIP("192.0.2.10")

	if !answer.A.Equal(expectedIP) {
		t.Errorf(
			"Exchange() answer IP = %s, want %s",
			answer.A,
			expectedIP,
		)
	}
}

func TestClientRejectsNilRequest(t *testing.T) {
	client := NewClient(
		"127.0.0.1:53",
		time.Second,
	)

	response, duration, err := client.Exchange(
		context.Background(),
		nil,
		NetworkUDP,
	)

	if !errors.Is(err, ErrNilRequest) {
		t.Fatalf(
			"Exchange() error = %v, want %v",
			err,
			ErrNilRequest,
		)
	}

	if response != nil {
		t.Errorf(
			"Exchange() response = %v, want nil",
			response,
		)
	}

	if duration < 0 {
		t.Errorf(
			"Exchange() duration = %s, want a non-negative duration",
			duration,
		)
	}
}

func TestClientRejectsUnsupportedNetwork(t *testing.T) {
	client := NewClient(
		"127.0.0.1:53",
		time.Second,
	)

	const unsupportedNetwork = "tls"

	response, duration, err := client.Exchange(
		context.Background(),
		new(dns.Msg),
		unsupportedNetwork,
	)

	if !errors.Is(err, ErrUnsupportedNetwork) {
		t.Fatalf(
			"Exchange() error = %v, want %v",
			err,
			ErrUnsupportedNetwork,
		)
	}

	if response != nil {
		t.Errorf(
			"Exchange() response = %v, want nil",
			response,
		)
	}

	if duration < 0 {
		t.Errorf(
			"Exchange() duration = %s, want a non-negative duration",
			duration,
		)
	}

	if !strings.Contains(err.Error(), unsupportedNetwork) {
		t.Errorf(
			"Exchange() error = %q, want it to contain %q",
			err,
			unsupportedNetwork,
		)
	}
}

func TestClientExchangesARecordOverTCP(t *testing.T) {
	upstreamServer := dnstest.Start(t)

	client := NewClient(
		upstreamServer.Address(),
		time.Second,
	)

	request := new(dns.Msg)
	request.SetQuestion(
		dns.Fqdn("example.test"),
		dns.TypeA,
	)

	response, duration, err := client.Exchange(
		context.Background(),
		request,
		NetworkTCP,
	)

	if err != nil {
		t.Fatalf(
			"Exchange() returned an unexpected error: %v",
			err,
		)
	}

	if response == nil {
		t.Fatal("Exchange() returned a nil response")
	}

	if duration <= 0 {
		t.Errorf(
			"Exchange() duration = %s, want a positive duration",
			duration,
		)
	}

	if response.Rcode != dns.RcodeSuccess {
		t.Errorf(
			"Exchange() response code = %s, want %s",
			dns.RcodeToString[response.Rcode],
			dns.RcodeToString[dns.RcodeSuccess],
		)
	}

	if len(response.Answer) != 1 {
		t.Fatalf(
			"Exchange() returned %d answers, want 1",
			len(response.Answer),
		)
	}

	answer, ok := response.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf(
			"Exchange() answer type = %T, want *dns.A",
			response.Answer[0],
		)
	}

	expectedIP := net.ParseIP("192.0.2.10")

	if !answer.A.Equal(expectedIP) {
		t.Errorf(
			"Exchange() answer IP = %s, want %s",
			answer.A,
			expectedIP,
		)
	}
}

func TestClientRetriesTruncatedUDPResponseOverTCP(t *testing.T) {
	upstreamServer := dnstest.Start(t)

	client := NewClient(
		upstreamServer.Address(),
		time.Second,
	)

	request := new(dns.Msg)
	request.SetQuestion(
		dns.Fqdn("truncated.test"),
		dns.TypeA,
	)

	response, duration, err := client.Exchange(
		context.Background(),
		request,
		NetworkUDP,
	)

	if err != nil {
		t.Fatalf(
			"Exchange() returned an unexpected error: %v",
			err,
		)
	}

	if response == nil {
		t.Fatal("Exchange() returned a nil response")
	}

	if duration <= 0 {
		t.Errorf(
			"Exchange() duration = %s, want a positive duration",
			duration,
		)
	}

	if response.Truncated {
		t.Error(
			"Exchange() returned a truncated response after TCP fallback",
		)
	}

	if response.Rcode != dns.RcodeSuccess {
		t.Errorf(
			"Exchange() response code = %s, want %s",
			dns.RcodeToString[response.Rcode],
			dns.RcodeToString[dns.RcodeSuccess],
		)
	}

	if len(response.Answer) != 1 {
		t.Fatalf(
			"Exchange() returned %d answers, want 1",
			len(response.Answer),
		)
	}

	answer, ok := response.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf(
			"Exchange() answer type = %T, want *dns.A",
			response.Answer[0],
		)
	}

	expectedIP := net.ParseIP("192.0.2.10")

	if !answer.A.Equal(expectedIP) {
		t.Errorf(
			"Exchange() answer IP = %s, want %s",
			answer.A,
			expectedIP,
		)
	}
}

func TestClientReturnsTimeoutError(t *testing.T) {
	server := dnstest.Start(t)

	const clientTimeout = 50 * time.Millisecond

	client := NewClient(
		server.Address(),
		clientTimeout,
	)

	request := new(dns.Msg)
	request.SetQuestion(
		dnstest.SlowDomain,
		dns.TypeA,
	)

	response, duration, err := client.Exchange(
		context.Background(),
		request,
		NetworkUDP,
	)

	if err == nil {
		t.Fatal("Exchange() error = nil, want timeout error")
	}

	if !IsTimeout(err) {
		t.Errorf(
			"IsTimeout(Exchange() error) = false, want true; error: %v",
			err,
		)
	}

	if response != nil {
		t.Errorf(
			"Exchange() response = %v, want nil",
			response,
		)
	}

	if duration <= 0 {
		t.Errorf(
			"Exchange() duration = %s, want a positive duration",
			duration,
		)
	}

	if duration < clientTimeout {
		t.Errorf(
			"Exchange() duration = %s, want at least %s",
			duration,
			clientTimeout,
		)
	}

	if duration >= dnstest.DefaultDelay {
		t.Errorf(
			"Exchange() duration = %s, want less than server delay %s",
			duration,
			dnstest.DefaultDelay,
		)
	}
}

func TestClientRespectsContextCancellation(t *testing.T) {
	server := dnstest.Start(t)

	client := NewClient(
		server.Address(),
		time.Second,
	)

	request := new(dns.Msg)
	request.SetQuestion(
		dnstest.SlowDomain,
		dns.TypeA,
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	response, duration, err := client.Exchange(
		ctx,
		request,
		NetworkUDP,
	)

	if err == nil {
		t.Fatal("Exchange() error = nil, want context cancellation error")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf(
			"Exchange() error = %v, want context.Canceled",
			err,
		)
	}

	if response != nil {
		t.Errorf(
			"Exchange() response = %v, want nil",
			response,
		)
	}

	if duration < 0 {
		t.Errorf(
			"Exchange() duration = %s, want non-negative duration",
			duration,
		)
	}
}

func TestClientReturnsSERVFAILResponse(t *testing.T) {
	server := dnstest.Start(t)

	client := NewClient(
		server.Address(),
		time.Second,
	)

	request := new(dns.Msg)
	request.SetQuestion(
		dnstest.ServfailDomain,
		dns.TypeA,
	)

	response, duration, err := client.Exchange(
		context.Background(),
		request,
		NetworkUDP,
	)

	if err != nil {
		t.Fatalf(
			"Exchange() returned an unexpected error: %v",
			err,
		)
	}

	if response == nil {
		t.Fatal("Exchange() returned a nil response")
	}

	if duration <= 0 {
		t.Errorf(
			"Exchange() duration = %s, want a positive duration",
			duration,
		)
	}

	if response.Rcode != dns.RcodeServerFailure {
		t.Errorf(
			"Exchange() response code = %s, want %s",
			dns.RcodeToString[response.Rcode],
			dns.RcodeToString[dns.RcodeServerFailure],
		)
	}

	if len(response.Answer) != 0 {
		t.Errorf(
			"Exchange() returned %d answers, want 0",
			len(response.Answer),
		)
	}
}

func TestClientReturnsNXDOMAINResponse(t *testing.T) {
	server := dnstest.Start(t)

	client := NewClient(
		server.Address(),
		time.Second,
	)

	request := new(dns.Msg)
	request.SetQuestion(
		dnstest.NXDomain,
		dns.TypeA,
	)

	response, duration, err := client.Exchange(
		context.Background(),
		request,
		NetworkUDP,
	)

	if err != nil {
		t.Fatalf(
			"Exchange() returned an unexpected error: %v",
			err,
		)
	}

	if response == nil {
		t.Fatal("Exchange() returned a nil response")
	}

	if duration <= 0 {
		t.Errorf(
			"Exchange() duration = %s, want a positive duration",
			duration,
		)
	}

	if response.Rcode != dns.RcodeNameError {
		t.Errorf(
			"Exchange() response code = %s, want %s",
			dns.RcodeToString[response.Rcode],
			dns.RcodeToString[dns.RcodeNameError],
		)
	}

	if len(response.Answer) != 0 {
		t.Errorf(
			"Exchange() returned %d answers, want 0",
			len(response.Answer),
		)
	}
}

func TestClientReturnsErrorForUnreachableUpstream(t *testing.T) {
	client := NewClient(
		"127.0.0.1:1",
		100*time.Millisecond,
	)

	request := new(dns.Msg)
	request.SetQuestion(
		dnstest.ExampleDomain,
		dns.TypeA,
	)

	response, duration, err := client.Exchange(
		context.Background(),
		request,
		NetworkUDP,
	)

	if err == nil {
		t.Fatal("Exchange() error = nil, want transport error")
	}

	if response != nil {
		t.Errorf(
			"Exchange() response = %v, want nil",
			response,
		)
	}

	if duration <= 0 {
		t.Errorf(
			"Exchange() duration = %s, want a positive duration",
			duration,
		)
	}
}
