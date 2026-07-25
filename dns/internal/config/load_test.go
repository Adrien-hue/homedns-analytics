package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	path := writeTestConfig(t, `
server:
  listen_address: "0.0.0.0"
  port: 5300

upstream:
  address: "9.9.9.9:53"
  timeout: 2s

health:
  listen_address: "127.0.0.1"
  port: 8090

logging:
  level: "debug"
  format: "text"

shutdown:
  timeout: 4s
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.ListenAddress != "0.0.0.0" {
		t.Errorf(
			"Server.ListenAddress = %q, want %q",
			cfg.Server.ListenAddress,
			"0.0.0.0",
		)
	}

	if cfg.Server.Port != 5300 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 5300)
	}

	if cfg.Upstream.Address != "9.9.9.9:53" {
		t.Errorf(
			"Upstream.Address = %q, want %q",
			cfg.Upstream.Address,
			"9.9.9.9:53",
		)
	}

	if cfg.Upstream.Timeout != 2*time.Second {
		t.Errorf(
			"Upstream.Timeout = %s, want %s",
			cfg.Upstream.Timeout,
			2*time.Second,
		)
	}

	if cfg.Health.Port != 8090 {
		t.Errorf("Health.Port = %d, want %d", cfg.Health.Port, 8090)
	}

	if cfg.Logging.Level != "debug" {
		t.Errorf(
			"Logging.Level = %q, want %q",
			cfg.Logging.Level,
			"debug",
		)
	}

	if cfg.Logging.Format != "text" {
		t.Errorf(
			"Logging.Format = %q, want %q",
			cfg.Logging.Format,
			"text",
		)
	}

	if cfg.Shutdown.Timeout != 4*time.Second {
		t.Errorf(
			"Shutdown.Timeout = %s, want %s",
			cfg.Shutdown.Timeout,
			4*time.Second,
		)
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	path := writeTestConfig(t, `
upstream:
  address: "1.1.1.1:53"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.ListenAddress != "127.0.0.1" {
		t.Errorf(
			"Server.ListenAddress = %q, want %q",
			cfg.Server.ListenAddress,
			"127.0.0.1",
		)
	}

	if cfg.Server.Port != 5353 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 5353)
	}

	if cfg.Upstream.Timeout != 3*time.Second {
		t.Errorf(
			"Upstream.Timeout = %s, want %s",
			cfg.Upstream.Timeout,
			3*time.Second,
		)
	}

	if cfg.Health.Port != 8081 {
		t.Errorf("Health.Port = %d, want %d", cfg.Health.Port, 8081)
	}

	if cfg.Logging.Level != "info" {
		t.Errorf(
			"Logging.Level = %q, want %q",
			cfg.Logging.Level,
			"info",
		)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	path := writeTestConfig(t, `
server:
  listen_adress: "127.0.0.1"

upstream:
  address: "1.1.1.1:53"
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want an unknown-field error")
	}

	if !strings.Contains(err.Error(), "listen_adress") {
		t.Fatalf("Load() error = %q, want unknown field name", err)
	}
}

func TestLoadRejectsMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want a missing-file error")
	}
}

func TestLoadRejectsEmptyPath(t *testing.T) {
	_, err := Load("")
	if err == nil {
		t.Fatal("Load() error = nil, want an empty-path error")
	}
}

func TestLoadRejectsMultipleDocuments(t *testing.T) {
	path := writeTestConfig(t, `
upstream:
  address: "1.1.1.1:53"
---
upstream:
  address: "9.9.9.9:53"
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want a multiple-document error")
	}
}

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "dns.yaml")

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write configuration file: %v", err)
	}

	return path
}
