package upstream

import (
	"context"
	"fmt"
	"time"

	"github.com/miekg/dns"
)

const (
	NetworkUDP = "udp"
	NetworkTCP = "tcp"
)

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
	network string,
) (*dns.Msg, time.Duration, error) {
	startedAt := time.Now()

	if request == nil {
		return nil, time.Since(startedAt), ErrNilRequest
	}

	if network != NetworkUDP && network != NetworkTCP {
		return nil, time.Since(startedAt), fmt.Errorf(
			"%w: %q",
			ErrUnsupportedNetwork,
			network,
		)
	}

	response, err := c.exchange(ctx, request, network)

	if err != nil {
		return nil, time.Since(startedAt), err
	}

	if network == NetworkUDP && response.Truncated {
		response, err = c.exchange(ctx, request, NetworkTCP)

		if err != nil {
			return nil, time.Since(startedAt), fmt.Errorf(
				"retry truncated upstream response over TCP : %w",
				err,
			)
		}
	}

	return response, time.Since(startedAt), nil
}

func (c *Client) exchange(
	ctx context.Context,
	request *dns.Msg,
	network string,
) (*dns.Msg, error) {
	client := &dns.Client{
		Net:     network,
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
