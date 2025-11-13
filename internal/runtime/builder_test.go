package runtime

import (
	"testing"
	"time"
	"vortex/internal/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRuntimeState(t *testing.T) {
	t.Run("HappyPath_FullStateCreation", func(t *testing.T) {
		internalCfg := &core.InternalConfig{
			Upstreams: map[string]*core.InternalUpstream{
				"app1": {
					Name:    "app1",
					Servers: []string{"s1.app1:80", "s2.app1:80"},
				},
				"app2": {
					Name:    "app2",
					Servers: []string{"s1.app2:80"},
				},
			},
			Locations: []*core.InternalLocation{
				{
					NormalizedPath: "/with-cache/",
					Cache: &core.InternalCacheConfig{
						Size: 100,
						TTL:  10 * time.Minute,
					},
				},
				{
					NormalizedPath: "/with-ratelimit/",
					RateLimit: &core.InternalRateLimitConfig{
						Size:  1000,
						Limit: 60,
					},
				},
				{
					NormalizedPath: "/plain/",
				},
			},
		}

		state := NewRuntimeState(internalCfg)
		require.NotNil(t, state)

		require.NotNil(t, state.Upstreams)
		require.Len(t, state.Upstreams, 2, "Should have state for 2 upstreams")
		require.Contains(t, state.Upstreams, "app1")
		require.Contains(t, state.Upstreams, "app2")

		app1State := state.Upstreams["app1"]
		require.NotNil(t, app1State.ServerConnections)
		require.Len(t, app1State.ServerConnections, 2, "Upstream 'app1' should have 2 server connection counters")
		require.Contains(t, app1State.ServerConnections, "s1.app1:80")
		require.Contains(t, app1State.ServerConnections, "s2.app1:80")
		assert.Equal(t, int64(0), app1State.ServerConnections["s1.app1:80"].Load(), "Initial connection count should be 0")

		require.NotNil(t, state.Caches)
		require.Len(t, state.Caches, 1, "Should have 1 cache instance created")
		assert.Contains(t, state.Caches, "/with-cache/", "Cache for '/with-cache/' should exist")
		assert.NotContains(t, state.Caches, "/with-ratelimit/", "Cache for '/with-ratelimit/' should NOT exist")
		assert.NotNil(t, state.Caches["/with-cache/"], "Cache instance should not be nil")

		require.NotNil(t, state.RateLimiters)
		require.Len(t, state.RateLimiters, 1, "Should have 1 rate limiter instance created")
		assert.Contains(t, state.RateLimiters, "/with-ratelimit/", "Rate limiter for '/with-ratelimit/' should exist")
		assert.NotContains(t, state.RateLimiters, "/with-cache/", "Rate limiter for '/with-cache/' should NOT exist")
		assert.NotNil(t, state.RateLimiters["/with-ratelimit/"], "Rate limiter instance should not be nil")
	})

	t.Run("ConfigWithNoSpecialFeatures", func(t *testing.T) {
		internalCfg := &core.InternalConfig{
			Upstreams: map[string]*core.InternalUpstream{
				"app1": {Name: "app1", Servers: []string{"s1:80"}},
			},
			Locations: []*core.InternalLocation{
				{NormalizedPath: "/path1/"},
				{NormalizedPath: "/path2/"},
			},
		}

		state := NewRuntimeState(internalCfg)

		require.NotNil(t, state)
		assert.Len(t, state.Upstreams, 1, "Upstream state should still be created")
		assert.Empty(t, state.Caches, "Caches map should be empty")
		assert.Empty(t, state.RateLimiters, "RateLimiters map should be empty")
	})

	t.Run("EmptyConfig", func(t *testing.T) {
		internalCfg := &core.InternalConfig{}
		state := NewRuntimeState(internalCfg)

		require.NotNil(t, state)
		assert.NotNil(t, state.Upstreams)
		assert.Empty(t, state.Upstreams)
		assert.NotNil(t, state.Caches)
		assert.Empty(t, state.Caches)
		assert.NotNil(t, state.RateLimiters)
		assert.Empty(t, state.RateLimiters)
	})
}
