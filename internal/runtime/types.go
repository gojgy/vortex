package runtime

import (
	"sync/atomic"
	"vortex/internal/cache"
	"vortex/internal/ratelimit"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

type UpstreamState struct {
	RoundRobinCounter atomic.Uint64
	ServerConnections map[string]*atomic.Int64
	// serverHealth      map[string]*ServerHealth
}

// TODO: на реализацию Healthcheck
// type ServerHealth struct {
// 	mu        sync.Mutex
// 	isDown    bool
// 	downSince time.Time
// }

type RuntimeState struct {
	Upstreams    map[string]*UpstreamState
	Caches       map[string]*expirable.LRU[string, *cache.CachedResponse]
	RateLimiters map[string]ratelimit.Limiter
}
