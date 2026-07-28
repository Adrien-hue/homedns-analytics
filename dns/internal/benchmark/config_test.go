package benchmark

import (
	"testing"
	"time"

	"github.com/miekg/dns"
)

func validPathConfig() PathConfig {
	return PathConfig{
		Address:     "127.0.0.1:5300",
		Protocol:    ProtocolUDP,
		QueryNames:  []string{"example.com."},
		QueryType:   dns.TypeA,
		QueryCount:  100,
		Concurrency: 10,
		Timeout:     3 * time.Second,
	}
}

func TestPathConfigValidate(t *testing.T) {
	t.Parallel()

	if err := validPathConfig().Validate(); err != nil {
		t.Fatalf("validate path config: %v", err)
	}
}

func TestPathConfigValidateRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*PathConfig)
	}{
		{
			name: "missing address",
			mutate: func(config *PathConfig) {
				config.Address = ""
			},
		},
		{
			name: "address without port",
			mutate: func(config *PathConfig) {
				config.Address = "127.0.0.1"
			},
		},
		{
			name: "unsupported protocol",
			mutate: func(config *PathConfig) {
				config.Protocol = "https"
			},
		},
		{
			name: "missing query names",
			mutate: func(config *PathConfig) {
				config.QueryNames = nil
			},
		},
		{
			name: "empty query name",
			mutate: func(config *PathConfig) {
				config.QueryNames = []string{""}
			},
		},
		{
			name: "unsupported query type",
			mutate: func(config *PathConfig) {
				config.QueryType = 65535
			},
		},
		{
			name: "zero query count",
			mutate: func(config *PathConfig) {
				config.QueryCount = 0
			},
		},
		{
			name: "zero concurrency",
			mutate: func(config *PathConfig) {
				config.Concurrency = 0
			},
		},
		{
			name: "concurrency exceeds query count",
			mutate: func(config *PathConfig) {
				config.QueryCount = 5
				config.Concurrency = 10
			},
		},
		{
			name: "zero timeout",
			mutate: func(config *PathConfig) {
				config.Timeout = 0
			},
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			config := validPathConfig()
			test.mutate(&config)

			if err := config.Validate(); err == nil {
				t.Fatal("expected validation to fail")
			}
		})
	}
}
