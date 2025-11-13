package core

import (
	"net/url"
	"time"
)

type InternalConfig struct {
	Locations []*InternalLocation
	Upstreams map[string]*InternalUpstream
}

type InternalUpstream struct {
	// balancer
	// healthcheck
	Name      string
	Algorithm string
	Servers   []string
}

type InternalCacheConfig struct {
	TTL  time.Duration
	Size int
	Key  string
}

type InternalRateLimitConfig struct {
	Limit  int64
	Window time.Duration
	Key    string
	Size   int
}

type InternalLocation struct {
	// PathRegex        
	NormalizedPath      string
	NormalizedPathLen   int
	IsProxyPassUpstream bool

	Cache     *InternalCacheConfig
	RateLimit *InternalRateLimitConfig
	Upstream  *InternalUpstream

	ProxyPass        *url.URL
	Timeout          time.Duration
	Root             string
	WebSocketEnabled bool
}
