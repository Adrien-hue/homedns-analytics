package upstream

import (
	"context"
	"errors"
	"net"
)

var (
	ErrNilRequest         = errors.New("DNS request must not be nil")
	ErrUnsupportedNetwork = errors.New("unsupported DNS network")
	ErrNilResponse        = errors.New("upstream returned a nil DNS response")
)

func IsTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var networkErr net.Error

	return errors.As(err, &networkErr) && networkErr.Timeout()
}
