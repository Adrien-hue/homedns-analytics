package upstream

import (
	"context"
	"fmt"
	"time"

	"github.com/miekg/dns"
)

const (
	NetworkUDP Network = "udp"
	NetworkTCP Network = "tcp"
)

type Network string

type Client struct {
	address string
	timeout time.Duration
}

func NewClient(address string, timeout time.Duration) *Client {
	return &Client{
		address: address,
		timeout: timeout,
	}
}

func (c *Client) Exchange(
	ctx context.Context,
	request *dns.Msg,
	network Network,
) (*dns.Msg, time.Duration, error) {
	if request == nil {
		return nil, 0, ErrNilRequest
	}

	switch network {
	case NetworkUDP, NetworkTCP:
		// Supported network.
	default:
		return nil, 0, fmt.Errorf(
			"%w: %s",
			ErrUnsupportedNetwork,
			network,
		)
	}

	start := time.Now()

	response, err := c.exchange(ctx, request, network)
	duration := time.Since(start)

	if err != nil {
		return nil, duration, err
	}

	if network == NetworkUDP && response.Truncated {
		start = time.Now()

		response, err = c.exchange(ctx, request, NetworkTCP)
		duration += time.Since(start)

		if err != nil {
			return nil, duration, err
		}
	}

	return response, duration, nil
}

func (c *Client) exchange(
	ctx context.Context,
	request *dns.Msg,
	network Network,
) (*dns.Msg, error) {
	client := &dns.Client{
		Net:     string(network),
		Timeout: c.timeout,
	}

	response, _, err := client.ExchangeContext(
		ctx,
		request,
		c.address,
	)

	if err != nil {
		return nil, fmt.Errorf(
			"exchange DNS request with upstream %s over %s: %w",
			c.address,
			network,
			err,
		)
	}

	if response == nil {
		return nil, ErrNilResponse
	}

	return response, nil
}
