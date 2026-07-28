package benchmark

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

const (
	ProtocolUDP = "udp"
	ProtocolTCP = "tcp"
)

// PathConfig defines one benchmark execution against one DNS target.
type PathConfig struct {
	Address     string
	Protocol    string
	QueryNames  []string
	QueryType   uint16
	QueryCount  int
	Concurrency int
	Timeout     time.Duration
}

// Validate verifies that a path benchmark can be executed safely.
func (c PathConfig) Validate() error {
	if strings.TrimSpace(c.Address) == "" {
		return errors.New("DNS target address is required")
	}

	if _, _, err := splitHostPort(c.Address); err != nil {
		return fmt.Errorf("invalid DNS target address: %w", err)
	}

	switch c.Protocol {
	case ProtocolUDP, ProtocolTCP:
	default:
		return fmt.Errorf(
			"unsupported DNS protocol %q",
			c.Protocol,
		)
	}

	if len(c.QueryNames) == 0 {
		return errors.New("at least one query name is required")
	}

	for _, queryName := range c.QueryNames {
		if strings.TrimSpace(queryName) == "" {
			return errors.New("query names cannot be empty")
		}
	}

	switch c.QueryType {
	case dns.TypeA,
		dns.TypeAAAA,
		dns.TypeCNAME,
		dns.TypeMX,
		dns.TypeNS,
		dns.TypePTR,
		dns.TypeSOA,
		dns.TypeTXT:
	default:
		return fmt.Errorf(
			"unsupported DNS query type %d",
			c.QueryType,
		)
	}

	if c.QueryCount <= 0 {
		return errors.New("query count must be greater than zero")
	}

	if c.Concurrency <= 0 {
		return errors.New("concurrency must be greater than zero")
	}

	if c.Concurrency > c.QueryCount {
		return errors.New(
			"concurrency cannot exceed query count",
		)
	}

	if c.Timeout <= 0 {
		return errors.New("timeout must be greater than zero")
	}

	return nil
}

func splitHostPort(address string) (string, string, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", "", err
	}

	if strings.TrimSpace(host) == "" {
		return "", "", errors.New("DNS target host is required")
	}

	if strings.TrimSpace(port) == "" {
		return "", "", errors.New("DNS target port is required")
	}

	return host, port, nil
}
