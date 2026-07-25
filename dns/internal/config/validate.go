package config

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

var (
	supportedLogLevels = map[string]struct{}{
		"debug": {},
		"info":  {},
		"warn":  {},
		"error": {},
	}

	supportedLogFormats = map[string]struct{}{
		"json": {},
		"text": {},
	}
)

// Validate verifies that the configuration is safe and internally consistent.
func (c Config) Validate() error {
	var validationErrors []error

	if err := validateListener(
		"server",
		c.Server.ListenAddress,
		c.Server.Port,
	); err != nil {
		validationErrors = append(validationErrors, err)
	}

	if err := validateListener(
		"health",
		c.Health.ListenAddress,
		c.Health.Port,
	); err != nil {
		validationErrors = append(validationErrors, err)
	}

	if err := validateUpstream(c.Upstream.Address); err != nil {
		validationErrors = append(validationErrors, err)
	}

	if c.Upstream.Timeout <= 0 {
		validationErrors = append(
			validationErrors,
			errors.New("upstream timeout must be greater than zero"),
		)
	}

	if c.Shutdown.Timeout <= 0 {
		validationErrors = append(
			validationErrors,
			errors.New("shutdown timeout must be greater than zero"),
		)
	}

	if c.Server.ListenAddress == c.Health.ListenAddress &&
		c.Server.Port == c.Health.Port {
		validationErrors = append(
			validationErrors,
			errors.New(
				"DNS and health listeners must not use the same address and port",
			),
		)
	}

	if _, supported := supportedLogLevels[c.Logging.Level]; !supported {
		validationErrors = append(
			validationErrors,
			fmt.Errorf(
				"unsupported logging level %q; supported values: debug, info, warn, error",
				c.Logging.Level,
			),
		)
	}

	if _, supported := supportedLogFormats[c.Logging.Format]; !supported {
		validationErrors = append(
			validationErrors,
			fmt.Errorf(
				"unsupported logging format %q; supported values: json, text",
				c.Logging.Format,
			),
		)
	}

	return errors.Join(validationErrors...)
}

func validateListener(name string, address string, port int) error {
	var validationErrors []error

	if net.ParseIP(address) == nil {
		validationErrors = append(
			validationErrors,
			fmt.Errorf(
				"%s listen address %q is not a valid IP address",
				name,
				address,
			),
		)
	}

	if port < 1 || port > 65535 {
		validationErrors = append(
			validationErrors,
			fmt.Errorf(
				"%s port must be between 1 and 65535, got %d",
				name,
				port,
			),
		)
	}

	return errors.Join(validationErrors...)
}

func validateUpstream(address string) error {
	if strings.TrimSpace(address) == "" {
		return errors.New("upstream address must not be empty")
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf(
			"upstream address %q must use host:port format: %w",
			address,
			err,
		)
	}

	if strings.TrimSpace(host) == "" {
		return fmt.Errorf(
			"upstream address %q must contain a host",
			address,
		)
	}

	if strings.TrimSpace(port) == "" {
		return fmt.Errorf(
			"upstream address %q must contain a port",
			address,
		)
	}

	if _, err := net.LookupPort("udp", port); err != nil {
		return fmt.Errorf(
			"upstream address %q contains an invalid port: %w",
			address,
			err,
		)
	}

	return nil
}
