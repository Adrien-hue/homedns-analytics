package config

import (
	"net"
	"strconv"
	"time"
)

const (
	defaultServerListenAddress = "127.0.0.1"
	defaultServerPort          = 5353

	defaultHealthListenAddress = "127.0.0.1"
	defaultHealthPort          = 8081

	defaultUpstreamTimeout = 3 * time.Second
	defaultShutdownTimeout = 5 * time.Second

	defaultLogLevel  = "info"
	defaultLogFormat = "json"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Upstream UpstreamConfig `yaml:"upstream"`
	Health   HealthConfig   `yaml:"health"`
	Logging  LoggingConfig  `yaml:"logging"`
	Shutdown ShutdownConfig `yaml:"shutdown"`
}

type ServerConfig struct {
	ListenAddress string `yaml:"listen_address"`
	Port          int    `yaml:"port"`
}

type UpstreamConfig struct {
	Address string        `yaml:"address"`
	Timeout time.Duration `yaml:"timeout"`
}

type HealthConfig struct {
	ListenAddress string `yaml:"listen_address"`
	Port          int    `yaml:"port"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type ShutdownConfig struct {
	Timeout time.Duration `yaml:"timeout"`
}

// Default returns a configuration containing safe local-development defaults.
//
// The upstream address intentionally has no default. It must be explicitly
// configured by the operator.
func Default() Config {
	return Config{
		Server: ServerConfig{
			ListenAddress: defaultServerListenAddress,
			Port:          defaultServerPort,
		},
		Upstream: UpstreamConfig{
			Timeout: defaultUpstreamTimeout,
		},
		Health: HealthConfig{
			ListenAddress: defaultHealthListenAddress,
			Port:          defaultHealthPort,
		},
		Logging: LoggingConfig{
			Level:  defaultLogLevel,
			Format: defaultLogFormat,
		},
		Shutdown: ShutdownConfig{
			Timeout: defaultShutdownTimeout,
		},
	}
}

// DNSAddress returns the configured DNS listener in host:port form.
func (c Config) DNSAddress() string {
	return net.JoinHostPort(
		c.Server.ListenAddress,
		strconv.Itoa(c.Server.Port),
	)
}

// HealthAddress returns the configured health listener in host:port form.
func (c Config) HealthAddress() string {
	return net.JoinHostPort(
		c.Health.ListenAddress,
		strconv.Itoa(c.Health.Port),
	)
}
