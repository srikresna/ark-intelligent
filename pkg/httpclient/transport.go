package httpclient

import (
	"net"
	"net/http"
	"time"
)

// SharedTransport is a pre-configured HTTP transport with connection pooling
// tuned for concurrent service usage. All services should share this transport
// to benefit from connection reuse across hosts.
var SharedTransport = &http.Transport{
	MaxIdleConns:        50,
	MaxIdleConnsPerHost: 10,
	MaxConnsPerHost:     20,
	IdleConnTimeout:     90 * time.Second,
	TLSHandshakeTimeout: 10 * time.Second,
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
}

// NewClient returns an *http.Client backed by the SharedTransport with the
// given request-level timeout.
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: SharedTransport,
		Timeout:   timeout,
	}
}
