package config

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultRequiresExplicitUpstream(t *testing.T) {
	cfg := Default()

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Default().Validate() error = nil, want missing-upstream error")
	}

	if !strings.Contains(err.Error(), "upstream address must not be empty") {
		t.Fatalf(
			"Default().Validate() error = %q, want missing-upstream error",
			err,
		)
	}
}

func TestValidateAcceptsValidConfiguration(t *testing.T) {
	cfg := validTestConfig()

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name          string
		mutate        func(*Config)
		expectedError string
	}{
		{
			name: "empty upstream",
			mutate: func(cfg *Config) {
				cfg.Upstream.Address = ""
			},
			expectedError: "upstream address must not be empty",
		},
		{
			name: "upstream without port",
			mutate: func(cfg *Config) {
				cfg.Upstream.Address = "1.1.1.1"
			},
			expectedError: "must use host:port format",
		},
		{
			name: "invalid server address",
			mutate: func(cfg *Config) {
				cfg.Server.ListenAddress = "not-an-ip"
			},
			expectedError: "server listen address",
		},
		{
			name: "server port too low",
			mutate: func(cfg *Config) {
				cfg.Server.Port = 0
			},
			expectedError: "server port must be between 1 and 65535",
		},
		{
			name: "server port too high",
			mutate: func(cfg *Config) {
				cfg.Server.Port = 65536
			},
			expectedError: "server port must be between 1 and 65535",
		},
		{
			name: "invalid health address",
			mutate: func(cfg *Config) {
				cfg.Health.ListenAddress = "localhost"
			},
			expectedError: "health listen address",
		},
		{
			name: "invalid health port",
			mutate: func(cfg *Config) {
				cfg.Health.Port = -1
			},
			expectedError: "health port must be between 1 and 65535",
		},
		{
			name: "listener collision",
			mutate: func(cfg *Config) {
				cfg.Health.ListenAddress = cfg.Server.ListenAddress
				cfg.Health.Port = cfg.Server.Port
			},
			expectedError: "must not use the same address and port",
		},
		{
			name: "zero upstream timeout",
			mutate: func(cfg *Config) {
				cfg.Upstream.Timeout = 0
			},
			expectedError: "upstream timeout must be greater than zero",
		},
		{
			name: "negative shutdown timeout",
			mutate: func(cfg *Config) {
				cfg.Shutdown.Timeout = -time.Second
			},
			expectedError: "shutdown timeout must be greater than zero",
		},
		{
			name: "unsupported log level",
			mutate: func(cfg *Config) {
				cfg.Logging.Level = "verbose"
			},
			expectedError: "unsupported logging level",
		},
		{
			name: "unsupported log format",
			mutate: func(cfg *Config) {
				cfg.Logging.Format = "xml"
			},
			expectedError: "unsupported logging format",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := validTestConfig()
			test.mutate(&cfg)

			err := cfg.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want validation error")
			}

			if !strings.Contains(err.Error(), test.expectedError) {
				t.Fatalf(
					"Validate() error = %q, want error containing %q",
					err,
					test.expectedError,
				)
			}
		})
	}
}

func TestAddresses(t *testing.T) {
	cfg := validTestConfig()

	if got := cfg.DNSAddress(); got != "127.0.0.1:5353" {
		t.Errorf("DNSAddress() = %q, want %q", got, "127.0.0.1:5353")
	}

	if got := cfg.HealthAddress(); got != "127.0.0.1:8081" {
		t.Errorf(
			"HealthAddress() = %q, want %q",
			got,
			"127.0.0.1:8081",
		)
	}
}

func validTestConfig() Config {
	cfg := Default()
	cfg.Upstream.Address = "1.1.1.1:53"

	return cfg
}
