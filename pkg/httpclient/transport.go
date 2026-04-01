package httpclient

import (
	"net"
	"net/http"
	"time"
)

// defaults for SharedTransport.
const (
	defaultMaxIdleConns        = 100
	defaultMaxConnsPerHost     = 10
	defaultIdleConnTimeout     = 90 * time.Second
	defaultTLSHandshakeTimeout = 10 * time.Second
	defaultDialTimeout         = 30 * time.Second
	defaultKeepAlive           = 30 * time.Second
	defaultTimeout             = 15 * time.Second
)

// SharedTransport is a pre-configured HTTP transport with connection pooling
// tuned for concurrent service usage. All services should share this transport
// to benefit from connection reuse across hosts.
var SharedTransport = &http.Transport{
	MaxIdleConns:        defaultMaxIdleConns,
	MaxIdleConnsPerHost: defaultMaxConnsPerHost,
	MaxConnsPerHost:     defaultMaxConnsPerHost,
	IdleConnTimeout:     defaultIdleConnTimeout,
	TLSHandshakeTimeout: defaultTLSHandshakeTimeout,
	DialContext: (&net.Dialer{
		Timeout:   defaultDialTimeout,
		KeepAlive: defaultKeepAlive,
	}).DialContext,
}

// Option configures an *http.Client returned by New.
type Option func(*http.Client)

// WithTimeout sets the request-level timeout on the client.
func WithTimeout(d time.Duration) Option {
	return func(c *http.Client) {
		c.Timeout = d
	}
}

// New returns an *http.Client backed by the SharedTransport.
// Without options it uses a 15 s default timeout.
func New(opts ...Option) *http.Client {
	c := &http.Client{
		Transport: SharedTransport,
		Timeout:   defaultTimeout,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// NewClient returns an *http.Client backed by the SharedTransport with the
// given request-level timeout. It is a convenience wrapper kept for backward
// compatibility; prefer New(WithTimeout(d)) in new code.
func NewClient(timeout time.Duration) *http.Client {
	return New(WithTimeout(timeout))
}
