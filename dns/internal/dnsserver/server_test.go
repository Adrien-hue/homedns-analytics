package dnsserver

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestNewServerUsesProvidedDependencies(t *testing.T) {
	handler := dns.HandlerFunc(func(
		dns.ResponseWriter,
		*dns.Msg,
	) {
	})

	server := NewServer(
		"127.0.0.1:5353",
		handler,
		nil,
	)

	if server.address != "127.0.0.1:5353" {
		t.Fatalf(
			"expected address %q, got %q",
			"127.0.0.1:5353",
			server.address,
		)
	}

	if server.handler == nil {
		t.Fatal("expected handler to be configured")
	}

	if server.logger == nil {
		t.Fatal("expected default logger to be configured")
	}
}

func TestServerIsNotReadyInitially(t *testing.T) {
	server := NewServer(
		"127.0.0.1:5353",
		nil,
		nil,
	)

	if server.Ready() {
		t.Fatal("expected new server not to be ready")
	}
}

func TestNilServerIsNotReady(t *testing.T) {
	var server *Server

	if server.Ready() {
		t.Fatal("expected nil server not to be ready")
	}
}

func TestServerStartRejectsMissingAddress(t *testing.T) {
	handler := dns.HandlerFunc(func(
		dns.ResponseWriter,
		*dns.Msg,
	) {
	})

	server := NewServer("", handler, nil)

	err := server.Start()

	if !errors.Is(err, ErrMissingAddress) {
		t.Fatalf(
			"expected ErrMissingAddress, got %v",
			err,
		)
	}

	if server.Ready() {
		t.Fatal("expected server not to be ready")
	}
}

func TestServerStartRejectsMissingHandler(t *testing.T) {
	server := NewServer(
		"127.0.0.1:5353",
		nil,
		nil,
	)

	err := server.Start()

	if !errors.Is(err, ErrMissingHandler) {
		t.Fatalf(
			"expected ErrMissingHandler, got %v",
			err,
		)
	}

	if server.Ready() {
		t.Fatal("expected server not to be ready")
	}
}

func TestServerStartRejectsSecondStart(t *testing.T) {
	address := availableDualProtocolAddress(t)

	handler := dns.HandlerFunc(func(
		dns.ResponseWriter,
		*dns.Msg,
	) {
	})

	server := NewServer(address, handler, nil)

	if err := server.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}

	t.Cleanup(func() {
		if err := server.Shutdown(context.Background()); err != nil {
			t.Errorf("shut down server: %v", err)
		}
	})

	err := server.Start()

	if !errors.Is(err, ErrServerAlreadyStarted) {
		t.Fatalf(
			"expected ErrServerAlreadyStarted, got %v",
			err,
		)
	}
}

func TestServerStartsUDPAndTCPListeners(t *testing.T) {
	address := availableDualProtocolAddress(t)

	handler := dns.HandlerFunc(func(
		dns.ResponseWriter,
		*dns.Msg,
	) {
	})

	server := NewServer(address, handler, nil)

	if err := server.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}

	t.Cleanup(func() {
		if err := server.Shutdown(context.Background()); err != nil {
			t.Errorf("shut down server: %v", err)
		}
	})

	if !server.Ready() {
		t.Fatal("expected server to be ready")
	}

	if server.udpServer == nil {
		t.Fatal("expected UDP server to be configured")
	}

	if server.tcpServer == nil {
		t.Fatal("expected TCP server to be configured")
	}
}

func TestServerRollsBackUDPListenerWhenTCPStartupFails(
	t *testing.T,
) {
	tcpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve TCP address: %v", err)
	}

	t.Cleanup(func() {
		if err := tcpListener.Close(); err != nil {
			t.Fatalf("close reserved TCP listener: %v", err)
		}
	})

	address := tcpListener.Addr().String()

	handler := dns.HandlerFunc(func(
		dns.ResponseWriter,
		*dns.Msg,
	) {
	})

	server := NewServer(address, handler, nil)

	err = server.Start()
	if err == nil {
		t.Fatal("expected TCP listener startup failure")
	}

	if server.Ready() {
		t.Fatal("expected server not to be ready")
	}

	if server.started {
		t.Fatal("expected server not to be marked as started")
	}

	if server.udpServer != nil {
		t.Fatal("expected UDP server not to be published")
	}

	if server.tcpServer != nil {
		t.Fatal("expected TCP server not to be published")
	}

	packetConn, listenErr := net.ListenPacket("udp", address)
	if listenErr != nil {
		t.Fatalf(
			"expected rolled-back UDP address to be reusable: %v",
			listenErr,
		)
	}

	if err := packetConn.Close(); err != nil {
		t.Fatalf("close verification UDP listener: %v", err)
	}
}

func availableDualProtocolAddress(t *testing.T) string {
	t.Helper()

	tcpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find available TCP address: %v", err)
	}

	address := tcpListener.Addr().String()

	if err := tcpListener.Close(); err != nil {
		t.Fatalf("release available TCP address: %v", err)
	}

	packetConn, err := net.ListenPacket("udp", address)
	if err != nil {
		t.Fatalf("verify available UDP address: %v", err)
	}

	if err := packetConn.Close(); err != nil {
		t.Fatalf("release available UDP address: %v", err)
	}

	return address
}

func closeServerSockets(
	t *testing.T,
	server *Server,
) {
	t.Helper()

	if server == nil {
		return
	}

	server.ready.Store(false)

	if server.udpServer != nil &&
		server.udpServer.PacketConn != nil {
		if err := server.udpServer.PacketConn.Close(); err != nil {
			t.Errorf("close UDP socket: %v", err)
		}
	}

	if server.tcpServer != nil &&
		server.tcpServer.Listener != nil {
		if err := server.tcpServer.Listener.Close(); err != nil {
			t.Errorf("close TCP socket: %v", err)
		}
	}
}

func TestServerShutdownBeforeStartIsSafe(t *testing.T) {
	server := NewServer(
		"127.0.0.1:5353",
		nil,
		nil,
	)

	err := server.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("shut down inactive server: %v", err)
	}

	if server.Ready() {
		t.Fatal("expected inactive server not to be ready")
	}
}

func TestServerShutdownClearsLifecycleState(t *testing.T) {
	address := availableDualProtocolAddress(t)

	handler := dns.HandlerFunc(func(
		writer dns.ResponseWriter,
		request *dns.Msg,
	) {
		response := new(dns.Msg)
		response.SetReply(request)

		_ = writer.WriteMsg(response)
	})

	server := NewServer(address, handler, nil)

	if err := server.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}

	if !server.Ready() {
		t.Fatal("expected server to be ready")
	}

	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("shut down server: %v", err)
	}

	if server.Ready() {
		t.Fatal("expected server not to be ready after shutdown")
	}

	server.mu.Lock()
	defer server.mu.Unlock()

	if server.started {
		t.Fatal("expected server not to be marked as started")
	}

	if server.udpServer != nil {
		t.Fatal("expected UDP server reference to be cleared")
	}

	if server.tcpServer != nil {
		t.Fatal("expected TCP server reference to be cleared")
	}
}

func TestServerShutdownIsIdempotent(t *testing.T) {
	address := availableDualProtocolAddress(t)

	handler := dns.HandlerFunc(func(
		dns.ResponseWriter,
		*dns.Msg,
	) {
	})

	server := NewServer(address, handler, nil)

	if err := server.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}

	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("first shutdown: %v", err)
	}

	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("second shutdown: %v", err)
	}
}

func TestServerCanRestartAfterShutdown(t *testing.T) {
	address := availableDualProtocolAddress(t)

	handler := dns.HandlerFunc(func(
		dns.ResponseWriter,
		*dns.Msg,
	) {
	})

	server := NewServer(address, handler, nil)

	if err := server.Start(); err != nil {
		t.Fatalf("first start: %v", err)
	}

	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("first shutdown: %v", err)
	}

	if err := server.Start(); err != nil {
		t.Fatalf("restart server: %v", err)
	}

	t.Cleanup(func() {
		if err := server.Shutdown(context.Background()); err != nil {
			t.Errorf("final shutdown: %v", err)
		}
	})

	if !server.Ready() {
		t.Fatal("expected restarted server to be ready")
	}
}

func TestServerShutdownWithExpiredContext(t *testing.T) {
	address := availableDualProtocolAddress(t)

	handler := dns.HandlerFunc(func(
		dns.ResponseWriter,
		*dns.Msg,
	) {
	})

	server := NewServer(address, handler, nil)

	if err := server.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := server.Shutdown(ctx)
	if err == nil {
		t.Fatal("expected shutdown error with cancelled context")
	}

	if server.Ready() {
		t.Fatal("expected server not to be ready")
	}
}

func TestServerHandlesDNSQueriesOverUDPAndTCP(t *testing.T) {
	address := availableDualProtocolAddress(t)

	const (
		queryName = "example.test."
		answerIP  = "192.0.2.10"
	)

	handler := dns.HandlerFunc(func(
		writer dns.ResponseWriter,
		request *dns.Msg,
	) {
		response := new(dns.Msg)
		response.SetReply(request)

		response.Answer = []dns.RR{
			&dns.A{
				Hdr: dns.RR_Header{
					Name:   queryName,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    60,
				},
				A: net.ParseIP(answerIP),
			},
		}

		if err := writer.WriteMsg(response); err != nil {
			t.Errorf("write DNS response: %v", err)
		}
	})

	server := NewServer(address, handler, nil)

	if err := server.Start(); err != nil {
		t.Fatalf("start DNS server: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(
			context.Background(),
			time.Second,
		)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			t.Errorf("shut down DNS server: %v", err)
		}
	})

	if !server.Ready() {
		t.Fatal("expected DNS server to be ready")
	}

	tests := []struct {
		name    string
		network string
	}{
		{
			name:    "UDP",
			network: "udp",
		},
		{
			name:    "TCP",
			network: "tcp",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := new(dns.Msg)
			request.SetQuestion(
				queryName,
				dns.TypeA,
			)

			client := &dns.Client{
				Net:     test.network,
				Timeout: time.Second,
			}

			response, _, err := client.Exchange(
				request,
				address,
			)
			if err != nil {
				t.Fatalf(
					"exchange DNS request over %s: %v",
					test.network,
					err,
				)
			}

			assertDNSAnswer(
				t,
				request,
				response,
				queryName,
				answerIP,
			)
		})
	}
}

func assertDNSAnswer(
	t *testing.T,
	request *dns.Msg,
	response *dns.Msg,
	expectedName string,
	expectedIP string,
) {
	t.Helper()

	if response == nil {
		t.Fatal("expected DNS response")
	}

	if response.Id != request.Id {
		t.Fatalf(
			"unexpected response ID: got %d, want %d",
			response.Id,
			request.Id,
		)
	}

	if response.Rcode != dns.RcodeSuccess {
		t.Fatalf(
			"unexpected response code: got %s, want %s",
			dns.RcodeToString[response.Rcode],
			dns.RcodeToString[dns.RcodeSuccess],
		)
	}

	if len(response.Question) != 1 {
		t.Fatalf(
			"unexpected question count: got %d, want 1",
			len(response.Question),
		)
	}

	question := response.Question[0]

	if question.Name != expectedName {
		t.Fatalf(
			"unexpected question name: got %q, want %q",
			question.Name,
			expectedName,
		)
	}

	if question.Qtype != dns.TypeA {
		t.Fatalf(
			"unexpected question type: got %d, want %d",
			question.Qtype,
			dns.TypeA,
		)
	}

	if len(response.Answer) != 1 {
		t.Fatalf(
			"unexpected answer count: got %d, want 1",
			len(response.Answer),
		)
	}

	answer, ok := response.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf(
			"unexpected answer type: got %T, want *dns.A",
			response.Answer[0],
		)
	}

	if answer.Hdr.Name != expectedName {
		t.Fatalf(
			"unexpected answer name: got %q, want %q",
			answer.Hdr.Name,
			expectedName,
		)
	}

	if !answer.A.Equal(net.ParseIP(expectedIP)) {
		t.Fatalf(
			"unexpected answer IP: got %s, want %s",
			answer.A,
			expectedIP,
		)
	}
}
