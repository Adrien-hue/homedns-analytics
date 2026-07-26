package dnstest

import (
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestServerReturnsARecordOverUDP(t *testing.T) {
	server := Start(t)

	response := exchange(t, server.Address(), "udp", ExampleDomain, dns.TypeA)

	if response.Rcode != dns.RcodeSuccess {
		t.Fatalf(
			"response code = %s, want %s",
			dns.RcodeToString[response.Rcode],
			dns.RcodeToString[dns.RcodeSuccess],
		)
	}

	if len(response.Answer) != 1 {
		t.Fatalf("answer count = %d, want 1", len(response.Answer))
	}

	record, ok := response.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("answer type = %T, want *dns.A", response.Answer[0])
	}

	if got := record.A.String(); got != ExampleIPv4 {
		t.Errorf("A address = %q, want %q", got, ExampleIPv4)
	}
}

func TestServerReturnsAAAARecordOverTCP(t *testing.T) {
	server := Start(t)

	response := exchange(t, server.Address(), "tcp", ExampleDomain, dns.TypeAAAA)

	if response.Rcode != dns.RcodeSuccess {
		t.Fatalf(
			"response code = %s, want %s",
			dns.RcodeToString[response.Rcode],
			dns.RcodeToString[dns.RcodeSuccess],
		)
	}

	if len(response.Answer) != 1 {
		t.Fatalf("answer count = %d, want 1", len(response.Answer))
	}

	record, ok := response.Answer[0].(*dns.AAAA)
	if !ok {
		t.Fatalf("answer type = %T, want *dns.AAAA", response.Answer[0])
	}

	if got := record.AAAA.String(); got != ExampleIPv6 {
		t.Errorf("AAAA address = %q, want %q", got, ExampleIPv6)
	}
}

func TestServerReturnsCNAMEChain(t *testing.T) {
	server := Start(t)

	response := exchange(t, server.Address(), "udp", AliasDomain, dns.TypeA)

	if len(response.Answer) != 2 {
		t.Fatalf("answer count = %d, want 2", len(response.Answer))
	}

	if _, ok := response.Answer[0].(*dns.CNAME); !ok {
		t.Fatalf(
			"first answer type = %T, want *dns.CNAME",
			response.Answer[0],
		)
	}

	record, ok := response.Answer[1].(*dns.A)
	if !ok {
		t.Fatalf(
			"second answer type = %T, want *dns.A",
			response.Answer[1],
		)
	}

	if got := record.A.String(); got != ExampleIPv4 {
		t.Errorf("A address = %q, want %q", got, ExampleIPv4)
	}
}

func TestServerReturnsMXRecord(t *testing.T) {
	server := Start(t)

	response := exchange(t, server.Address(), "udp", MailDomain, dns.TypeMX)

	if len(response.Answer) != 1 {
		t.Fatalf("answer count = %d, want 1", len(response.Answer))
	}

	record, ok := response.Answer[0].(*dns.MX)
	if !ok {
		t.Fatalf("answer type = %T, want *dns.MX", response.Answer[0])
	}

	if record.Preference != 10 {
		t.Errorf("MX preference = %d, want 10", record.Preference)
	}

	if record.Mx != MailHost {
		t.Errorf("MX host = %q, want %q", record.Mx, MailHost)
	}
}

func TestServerReturnsNXDOMAIN(t *testing.T) {
	server := Start(t)

	response := exchange(t, server.Address(), "udp", NXDomain, dns.TypeA)

	if response.Rcode != dns.RcodeNameError {
		t.Fatalf(
			"response code = %s, want %s",
			dns.RcodeToString[response.Rcode],
			dns.RcodeToString[dns.RcodeNameError],
		)
	}
}

func TestServerReturnsSERVFAIL(t *testing.T) {
	server := Start(t)

	response := exchange(
		t,
		server.Address(),
		"udp",
		ServfailDomain,
		dns.TypeA,
	)

	if response.Rcode != dns.RcodeServerFailure {
		t.Fatalf(
			"response code = %s, want %s",
			dns.RcodeToString[response.Rcode],
			dns.RcodeToString[dns.RcodeServerFailure],
		)
	}
}

func TestServerSupportsDelayedResponses(t *testing.T) {
	server := Start(t)

	startedAt := time.Now()

	response := exchange(t, server.Address(), "udp", SlowDomain, dns.TypeA)

	elapsed := time.Since(startedAt)

	if response.Rcode != dns.RcodeSuccess {
		t.Fatalf(
			"response code = %s, want %s",
			dns.RcodeToString[response.Rcode],
			dns.RcodeToString[dns.RcodeSuccess],
		)
	}

	if elapsed < DefaultDelay {
		t.Errorf(
			"response delay = %s, want at least %s",
			elapsed,
			DefaultDelay,
		)
	}
}

func TestServerUsesSamePortForUDPAndTCP(t *testing.T) {
	server := Start(t)

	udpResponse := exchange(
		t,
		server.Address(),
		"udp",
		ExampleDomain,
		dns.TypeA,
	)

	tcpResponse := exchange(
		t,
		server.Address(),
		"tcp",
		ExampleDomain,
		dns.TypeA,
	)

	if udpResponse.Rcode != dns.RcodeSuccess {
		t.Errorf("UDP response code = %d, want success", udpResponse.Rcode)
	}

	if tcpResponse.Rcode != dns.RcodeSuccess {
		t.Errorf("TCP response code = %d, want success", tcpResponse.Rcode)
	}
}

func exchange(
	t testing.TB,
	address string,
	network string,
	name string,
	queryType uint16,
) *dns.Msg {
	t.Helper()

	request := new(dns.Msg)
	request.SetQuestion(name, queryType)

	client := &dns.Client{
		Net:     network,
		Timeout: time.Second,
	}

	response, _, err := client.Exchange(request, address)
	if err != nil {
		t.Fatalf(
			"exchange %s query for %q with %s: %v",
			network,
			name,
			address,
			err,
		)
	}

	return response
}

func TestServerAddressIsLoopback(t *testing.T) {
	server := Start(t)

	host, _, err := net.SplitHostPort(server.Address())
	if err != nil {
		t.Fatalf("parse server address: %v", err)
	}

	if !net.ParseIP(host).IsLoopback() {
		t.Errorf("server host = %q, want loopback address", host)
	}
}
