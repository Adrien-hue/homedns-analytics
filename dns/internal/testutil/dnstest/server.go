package dnstest

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"
)

const (
	ExampleDomain  = "example.test."
	AliasDomain    = "alias.test."
	MailDomain     = "mail.test."
	MailHost       = "mailhost.test."
	SlowDomain     = "slow.test."
	ServfailDomain = "servfail.test."
	NXDomain       = "nxdomain.test."

	ExampleIPv4 = "192.0.2.10"
	ExampleIPv6 = "2001:db8::10"
	MailIPv4    = "192.0.2.20"

	DefaultDelay   = 250 * time.Millisecond
	startupTimeout = time.Second
)

// Server is a deterministic local DNS server intended for automated tests.
type Server struct {
	address string

	udpServer *dns.Server
	tcpServer *dns.Server

	errCh chan error

	shutdownOnce sync.Once
}

// Start starts UDP and TCP DNS listeners on the same temporary local port.
//
// The server is automatically stopped when the test completes.
func Start(t testing.TB) *Server {
	t.Helper()

	tcpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("start test upstream TCP listener: %v", err)
	}

	address := tcpListener.Addr().String()

	udpConnection, err := net.ListenPacket("udp", address)
	if err != nil {
		_ = tcpListener.Close()
		t.Fatalf("start test upstream UDP listener: %v", err)
	}

	handler := dns.HandlerFunc(handleRequest)

	server := &Server{
		address: address,
		udpServer: &dns.Server{
			PacketConn: udpConnection,
			Net:        "udp",
			Handler:    handler,
		},
		tcpServer: &dns.Server{
			Listener: tcpListener,
			Net:      "tcp",
			Handler:  handler,
		},
		errCh: make(chan error, 2),
	}

	go server.serve(server.udpServer)
	go server.serve(server.tcpServer)

	server.waitUntilReady(t)

	t.Cleanup(func() {
		if err := server.Shutdown(); err != nil {
			t.Errorf("shut down test upstream: %v", err)
		}
	})

	return server
}

// Address returns the shared UDP and TCP listener address.
func (s *Server) Address() string {
	return s.address
}

// Shutdown stops both DNS listeners.
func (s *Server) Shutdown() error {
	var shutdownErr error

	s.shutdownOnce.Do(func() {
		udpErr := s.udpServer.Shutdown()
		tcpErr := s.tcpServer.Shutdown()

		switch {
		case udpErr != nil && tcpErr != nil:
			shutdownErr = fmt.Errorf(
				"shutdown UDP server: %v; shutdown TCP server: %v",
				udpErr,
				tcpErr,
			)
		case udpErr != nil:
			shutdownErr = fmt.Errorf("shutdown UDP server: %w", udpErr)
		case tcpErr != nil:
			shutdownErr = fmt.Errorf("shutdown TCP server: %w", tcpErr)
		}
	})

	return shutdownErr
}

func (s *Server) serve(server *dns.Server) {
	if err := server.ActivateAndServe(); err != nil {
		s.errCh <- err
	}
}

func handleRequest(writer dns.ResponseWriter, request *dns.Msg) {
	response := new(dns.Msg)
	response.SetReply(request)
	response.Authoritative = true

	if len(request.Question) == 0 {
		response.Rcode = dns.RcodeFormatError
		writeResponse(writer, response)

		return
	}

	question := request.Question[0]
	name := dns.CanonicalName(question.Name)

	if name == SlowDomain {
		time.Sleep(DefaultDelay)
	}

	switch name {
	case ExampleDomain:
		addExampleAnswer(response, question)

	case AliasDomain:
		addAliasAnswer(response, question)

	case MailDomain:
		addMailAnswer(response, question)

	case MailHost:
		addMailHostAnswer(response, question)

	case SlowDomain:
		addSlowAnswer(response, question)

	case ServfailDomain:
		response.Rcode = dns.RcodeServerFailure

	case NXDomain:
		response.Rcode = dns.RcodeNameError

	default:
		response.Rcode = dns.RcodeNameError
	}

	writeResponse(writer, response)
}

func addExampleAnswer(response *dns.Msg, question dns.Question) {
	switch question.Qtype {
	case dns.TypeA:
		response.Answer = append(response.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   ExampleDomain,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    60,
			},
			A: net.ParseIP(ExampleIPv4),
		})

	case dns.TypeAAAA:
		response.Answer = append(response.Answer, &dns.AAAA{
			Hdr: dns.RR_Header{
				Name:   ExampleDomain,
				Rrtype: dns.TypeAAAA,
				Class:  dns.ClassINET,
				Ttl:    60,
			},
			AAAA: net.ParseIP(ExampleIPv6),
		})
	}
}

func addAliasAnswer(response *dns.Msg, question dns.Question) {
	if question.Qtype != dns.TypeA && question.Qtype != dns.TypeCNAME {
		return
	}

	response.Answer = append(response.Answer, &dns.CNAME{
		Hdr: dns.RR_Header{
			Name:   AliasDomain,
			Rrtype: dns.TypeCNAME,
			Class:  dns.ClassINET,
			Ttl:    60,
		},
		Target: ExampleDomain,
	})

	if question.Qtype == dns.TypeA {
		response.Answer = append(response.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   ExampleDomain,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    60,
			},
			A: net.ParseIP(ExampleIPv4),
		})
	}
}

func addMailAnswer(response *dns.Msg, question dns.Question) {
	if question.Qtype != dns.TypeMX {
		return
	}

	response.Answer = append(response.Answer, &dns.MX{
		Hdr: dns.RR_Header{
			Name:   MailDomain,
			Rrtype: dns.TypeMX,
			Class:  dns.ClassINET,
			Ttl:    60,
		},
		Preference: 10,
		Mx:         MailHost,
	})
}

func addMailHostAnswer(response *dns.Msg, question dns.Question) {
	if question.Qtype != dns.TypeA {
		return
	}

	response.Answer = append(response.Answer, &dns.A{
		Hdr: dns.RR_Header{
			Name:   MailHost,
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    60,
		},
		A: net.ParseIP(MailIPv4),
	})
}

func addSlowAnswer(response *dns.Msg, question dns.Question) {
	if question.Qtype != dns.TypeA {
		return
	}

	response.Answer = append(response.Answer, &dns.A{
		Hdr: dns.RR_Header{
			Name:   SlowDomain,
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    60,
		},
		A: net.ParseIP(ExampleIPv4),
	})
}

func writeResponse(writer dns.ResponseWriter, response *dns.Msg) {
	// There is no useful recovery action inside the test helper. Tests using
	// the server will fail through a missing or invalid response.
	_ = writer.WriteMsg(response)
}

func (s *Server) waitUntilReady(t testing.TB) {
	t.Helper()

	deadline := time.Now().Add(startupTimeout)

	for {
		udpReady := s.isReady("udp")
		tcpReady := s.isReady("tcp")

		if udpReady && tcpReady {
			return
		}

		select {
		case err := <-s.errCh:
			t.Fatalf("start test upstream: %v", err)
		default:
		}

		if time.Now().After(deadline) {
			t.Fatalf(
				"test upstream did not become ready within %s",
				startupTimeout,
			)
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func (s *Server) isReady(network string) bool {
	request := new(dns.Msg)
	request.SetQuestion(ExampleDomain, dns.TypeA)

	client := &dns.Client{
		Net:     network,
		Timeout: 50 * time.Millisecond,
	}

	response, _, err := client.Exchange(request, s.address)

	return err == nil &&
		response != nil &&
		response.Rcode == dns.RcodeSuccess
}
