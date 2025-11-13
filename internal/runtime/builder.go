package runtime

import (
	"sync/atomic"
	"vortex/internal/cache"
	"vortex/internal/core"
	"vortex/internal/ratelimit"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

func NewRuntimeState(cfg *core.InternalConfig) *RuntimeState {
	state := &RuntimeState{
		Upstreams:    make(map[string]*UpstreamState, len(cfg.Upstreams)),
		Caches:       make(map[string]*expirable.LRU[string, *cache.CachedResponse]),
		RateLimiters: make(map[string]ratelimit.Limiter),
	}

	for name, upstreamCfg := range cfg.Upstreams {
		upState := &UpstreamState{
			ServerConnections: make(map[string]*atomic.Int64, len(upstreamCfg.Servers)),
		}
		for _, serverAddr := range upstreamCfg.Servers {
			upState.ServerConnections[serverAddr] = &atomic.Int64{}
		}
		state.Upstreams[name] = upState
	}

	for _, loc := range cfg.Locations {
		if loc.Cache != nil {
			state.Caches[loc.NormalizedPath] = expirable.NewLRU[string, *cache.CachedResponse](
				loc.Cache.Size,
				nil,
				loc.Cache.TTL,
			)
		}

		if loc.RateLimit != nil {
			limiter, err := ratelimit.NewLRUFixedWindowLimiter(
				loc.RateLimit.Size,
				loc.RateLimit.Limit,
				loc.RateLimit.Window,
			)
			if err == nil {
				state.RateLimiters[loc.NormalizedPath] = limiter
			}
		}
	}

	return state
}

// TODO: переименовать файл
