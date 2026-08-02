package upstream

import (
	"context"
	"time"

	"github.com/miekg/dns"
)

// Forwarder forwards DNS requests to an upstream resolver.
type Forwarder interface {
	Exchange(
		ctx context.Context,
		request *dns.Msg,
		network Network,
	) (*dns.Msg, time.Duration, error)
}

// Compile-time verification that Client implements Forwarder.
var _ Forwarder = (*Client)(nil)
