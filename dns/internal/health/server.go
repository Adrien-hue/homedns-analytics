package health

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	// ErrServerRunning is returned when Start is called while the health server
	// is already running.
	ErrServerRunning = errors.New("health server is already running")

	// ErrPublicAddress is returned when the health server is configured to bind
	// to a non-loopback address.
	ErrPublicAddress = errors.New(
		"health server must listen on a loopback address",
	)
)

const (
	defaultReadHeaderTimeout = 5 * time.Second
)

// Server manages the lifecycle of the internal health HTTP listener.
type Server struct {
	address string
	handler http.Handler
	logger  *slog.Logger

	mu         sync.RWMutex
	httpServer *http.Server
	listener   net.Listener
	ready      bool
}

// NewServer creates an internal health HTTP server.
//
// The address must resolve to a loopback interface. Port 0 is supported and is
// useful in automated tests because the operating system selects an available
// port.
func NewServer(
	address string,
	handler http.Handler,
	logger *slog.Logger,
) (*Server, error) {
	if strings.TrimSpace(address) == "" {
		return nil, errors.New("health server address is required")
	}

	if handler == nil {
		return nil, errors.New("health server handler is required")
	}

	if err := validateLoopbackAddress(address); err != nil {
		return nil, err
	}

	return &Server{
		address: address,
		handler: handler,
		logger:  logger,
	}, nil
}

// Start binds the configured address and begins serving HTTP requests.
//
// Start returns only after the TCP listener has been created successfully.
// Once Start returns nil, Ready reports true and Address returns the actual
// bound address.
func (s *Server) Start() error {
	if s == nil {
		return errors.New("health server is nil")
	}

	s.mu.Lock()

	if s.httpServer != nil || s.listener != nil || s.ready {
		s.mu.Unlock()

		return ErrServerRunning
	}

	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		s.mu.Unlock()

		return fmt.Errorf(
			"listen on health address %q: %w",
			s.address,
			err,
		)
	}

	httpServer := &http.Server{
		Handler:           s.handler,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
	}

	s.listener = listener
	s.httpServer = httpServer
	s.ready = true

	actualAddress := listener.Addr().String()

	s.mu.Unlock()

	if s.logger != nil {
		s.logger.Info(
			"health server started",
			"address", actualAddress,
		)
	}

	go s.serve(httpServer, listener)

	return nil
}

// Shutdown gracefully stops accepting health requests and waits for active
// requests to finish until the supplied context expires.
//
// Calling Shutdown when the server is already stopped is safe.
func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}

	if ctx == nil {
		return errors.New("shutdown context is required")
	}

	s.mu.Lock()

	httpServer := s.httpServer

	if httpServer == nil {
		s.ready = false
		s.listener = nil
		s.mu.Unlock()

		return nil
	}

	s.ready = false

	s.mu.Unlock()

	err := httpServer.Shutdown(ctx)

	s.mu.Lock()

	if s.httpServer == httpServer {
		s.httpServer = nil
		s.listener = nil
	}

	s.mu.Unlock()

	if err != nil {
		return fmt.Errorf("shutdown health server: %w", err)
	}

	if s.logger != nil {
		s.logger.Info("health server stopped")
	}

	return nil
}

// Ready reports whether the health listener is currently accepting requests.
func (s *Server) Ready() bool {
	if s == nil {
		return false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.ready
}

// Address returns the actual bound listener address while the server is
// running. Before startup and after shutdown, it returns the configured
// address.
func (s *Server) Address() string {
	if s == nil {
		return ""
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.listener != nil {
		return s.listener.Addr().String()
	}

	return s.address
}

func (s *Server) serve(
	httpServer *http.Server,
	listener net.Listener,
) {
	err := httpServer.Serve(listener)

	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		if s.logger != nil {
			s.logger.Error(
				"health server stopped unexpectedly",
				"error", err,
			)
		}
	}

	s.mu.Lock()

	if s.httpServer == httpServer {
		s.ready = false
		s.httpServer = nil
		s.listener = nil
	}

	s.mu.Unlock()
}

func validateLoopbackAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf(
			"invalid health server address %q: %w",
			address,
			err,
		)
	}

	host = strings.TrimSpace(host)

	if strings.EqualFold(host, "localhost") {
		return nil
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf(
			"%w: host %q is not localhost or a loopback IP",
			ErrPublicAddress,
			host,
		)
	}

	if !ip.IsLoopback() {
		return fmt.Errorf(
			"%w: %q",
			ErrPublicAddress,
			address,
		)
	}

	return nil
}
