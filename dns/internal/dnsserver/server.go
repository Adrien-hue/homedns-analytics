package dnsserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/miekg/dns"
)

var (
	// ErrServerAlreadyStarted is returned when Start is called while the DNS
	// server is already running.
	ErrServerAlreadyStarted = errors.New("DNS server already started")

	// ErrMissingAddress is returned when no DNS listen address is configured.
	ErrMissingAddress = errors.New("DNS server listen address is required")

	// ErrMissingHandler is returned when no DNS request handler is configured.
	ErrMissingHandler = errors.New("DNS server handler is required")
)

// Server manages the UDP and TCP DNS listener lifecycle.
type Server struct {
	address string
	handler dns.Handler
	logger  *slog.Logger

	ready atomic.Bool

	mu sync.Mutex

	udpServer *dns.Server
	tcpServer *dns.Server

	started      bool
	shuttingDown bool
}

// NewServer creates a DNS server lifecycle manager.
//
// A discard logger is used when logger is nil.
func NewServer(
	address string,
	handler dns.Handler,
	logger *slog.Logger,
) *Server {
	if logger == nil {
		logger = slog.New(
			slog.NewTextHandler(io.Discard, nil),
		)
	}

	return &Server{
		address: address,
		handler: handler,
		logger:  logger,
	}
}

// Ready reports whether both DNS listeners are running and available to accept
// requests.
func (s *Server) Ready() bool {
	if s == nil {
		return false
	}

	return s.ready.Load()
}

// Start creates and starts the UDP and TCP DNS listeners.
//
// Both sockets are acquired before either server is marked as ready. Start
// returns only after both miekg/dns servers confirm that their serving loops
// are active.
//
// If startup fails, all acquired resources are released before Start returns.
func (s *Server) Start() error {
	if s == nil {
		return errors.New("DNS server is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started || s.shuttingDown {
		return ErrServerAlreadyStarted
	}

	if strings.TrimSpace(s.address) == "" {
		return ErrMissingAddress
	}

	if s.handler == nil {
		return ErrMissingHandler
	}

	packetConn, err := net.ListenPacket("udp", s.address)
	if err != nil {
		return fmt.Errorf(
			"open UDP DNS listener on %s: %w",
			s.address,
			err,
		)
	}

	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		if closeErr := packetConn.Close(); closeErr != nil {
			s.logger.Error(
				"failed to close UDP listener after TCP startup failure",
				"address", s.address,
				"error", closeErr,
			)
		}

		return fmt.Errorf(
			"open TCP DNS listener on %s: %w",
			s.address,
			err,
		)
	}

	udpStarted := make(chan struct{})
	tcpStarted := make(chan struct{})

	udpDone := make(chan error, 1)
	tcpDone := make(chan error, 1)

	udpServer := &dns.Server{
		PacketConn: packetConn,
		Handler:    s.handler,
		NotifyStartedFunc: func() {
			close(udpStarted)
		},
	}

	tcpServer := &dns.Server{
		Listener: listener,
		Handler:  s.handler,
		NotifyStartedFunc: func() {
			close(tcpStarted)
		},
	}

	s.udpServer = udpServer
	s.tcpServer = tcpServer

	go func() {
		udpDone <- udpServer.ActivateAndServe()
	}()

	go func() {
		tcpDone <- tcpServer.ActivateAndServe()
	}()

	if err := waitForListenerStartup(
		"UDP",
		udpStarted,
		udpDone,
	); err != nil {
		s.rollbackStartup(packetConn, listener)

		return err
	}

	if err := waitForListenerStartup(
		"TCP",
		tcpStarted,
		tcpDone,
	); err != nil {
		s.rollbackStartup(packetConn, listener)

		return err
	}

	s.started = true
	s.ready.Store(true)

	go s.observeListener("udp", udpDone)
	go s.observeListener("tcp", tcpDone)

	s.logger.Info(
		"DNS listeners started",
		"address", s.address,
	)

	return nil
}

// Shutdown gracefully stops both DNS listeners.
//
// Readiness becomes false immediately when shutdown begins. Shutdown is safe to
// call when the server is not running. Once shutdown completes, the server may
// be started again.
func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}

	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()

	if !s.started {
		s.ready.Store(false)
		s.mu.Unlock()

		return nil
	}

	s.ready.Store(false)
	s.shuttingDown = true

	udpServer := s.udpServer
	tcpServer := s.tcpServer

	s.mu.Unlock()

	var udpErr error
	var tcpErr error

	if udpServer != nil {
		udpErr = udpServer.ShutdownContext(ctx)
	}

	if tcpServer != nil {
		tcpErr = tcpServer.ShutdownContext(ctx)
	}

	s.mu.Lock()

	s.started = false
	s.shuttingDown = false
	s.udpServer = nil
	s.tcpServer = nil

	s.mu.Unlock()

	shutdownErr := errors.Join(
		wrapShutdownError("UDP", udpErr),
		wrapShutdownError("TCP", tcpErr),
	)

	if shutdownErr != nil {
		return shutdownErr
	}

	s.logger.Info(
		"DNS listeners stopped",
		"address", s.address,
	)

	return nil
}

// observeListener records an unexpected listener termination.
//
// A listener stopping during graceful shutdown is expected and is therefore
// not logged as a failure.
func (s *Server) observeListener(
	network string,
	done <-chan error,
) {
	err := <-done

	s.mu.Lock()
	running := s.started
	shuttingDown := s.shuttingDown
	s.mu.Unlock()

	if !running || shuttingDown {
		return
	}

	s.ready.Store(false)

	if err != nil {
		s.logger.Error(
			"DNS listener stopped unexpectedly",
			"network", network,
			"address", s.address,
			"error", err,
		)

		return
	}

	s.logger.Error(
		"DNS listener stopped unexpectedly",
		"network", network,
		"address", s.address,
	)
}

// rollbackStartup releases resources acquired during an unsuccessful startup
// and restores the initial lifecycle state.
//
// The caller must hold s.mu.
func (s *Server) rollbackStartup(
	packetConn net.PacketConn,
	listener net.Listener,
) {
	s.ready.Store(false)
	s.started = false
	s.shuttingDown = false
	s.udpServer = nil
	s.tcpServer = nil

	if packetConn != nil {
		if err := packetConn.Close(); err != nil {
			s.logger.Error(
				"failed to close UDP listener during startup rollback",
				"address", s.address,
				"error", err,
			)
		}
	}

	if listener != nil {
		if err := listener.Close(); err != nil {
			s.logger.Error(
				"failed to close TCP listener during startup rollback",
				"address", s.address,
				"error", err,
			)
		}
	}
}

// waitForListenerStartup waits until a DNS listener confirms that its serving
// loop is active or terminates during startup.
func waitForListenerStartup(
	network string,
	started <-chan struct{},
	done <-chan error,
) error {
	select {
	case <-started:
		return nil

	case err := <-done:
		if err == nil {
			return fmt.Errorf(
				"%s DNS listener stopped during startup",
				network,
			)
		}

		return fmt.Errorf(
			"start %s DNS listener: %w",
			network,
			err,
		)
	}
}

// wrapShutdownError adds listener context to an error returned during graceful
// shutdown.
func wrapShutdownError(
	network string,
	err error,
) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf(
		"shut down %s DNS listener: %w",
		network,
		err,
	)
}
