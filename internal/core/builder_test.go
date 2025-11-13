package core

import (
	"testing"
	"time"
	"vortex/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildInternalConfig(t *testing.T) {
	t.Run("HappyPath_FullConversion", func(t *testing.T) {
		userCfg := &config.Config{
			Upstreams: config.UpstreamsConfig{
				"my_app": {
					Algorithm: config.BalancingAlgorithmLeastConnections,
					Servers:   []string{"app1:8080", "app2:8080"},
				},
			},
			Locations: config.LocationsConfig{
				{
					Path:      "/api/",
					ProxyPass: "http://my_app/v1",
					Timeout:   30 * time.Second,
					Cache: &config.CacheConfig{
						TTL:  10 * time.Minute,
						Size: 1000,
						Key:  "${uri}",
					},
				},
				{
					Path:      "/direct/",
					ProxyPass: "http://google.com",
				},
				{
					Path: "/static/",
					Root: "/var/www/static",
					RateLimit: &config.RateLimitConfig{
						RPM:  60,
						Size: 100,
						Key:  "${remote_addr}",
					},
				},
				{
					Path:      "legacy-path",
					ProxyPass: "http://my_app",
				},
			},
		}

		internalCfg, err := BuildInternalConfig(userCfg)

		require.NoError(t, err)
		require.NotNil(t, internalCfg)

		require.Len(t, internalCfg.Upstreams, 1, "Should be one upstream")
		require.Contains(t, internalCfg.Upstreams, "my_app")
		assert.Equal(t, "my_app", internalCfg.Upstreams["my_app"].Name)
		assert.Equal(t, string(config.BalancingAlgorithmLeastConnections), internalCfg.Upstreams["my_app"].Algorithm)
		assert.Equal(t, []string{"app1:8080", "app2:8080"}, internalCfg.Upstreams["my_app"].Servers)

		require.Len(t, internalCfg.Locations, 4, "Should be four locations")

		loc1 := internalCfg.Locations[0]
		assert.Equal(t, "/api/", loc1.NormalizedPath)
		assert.True(t, loc1.IsProxyPassUpstream, "Location 1 should be identified as an upstream proxy")
		require.NotNil(t, loc1.Upstream, "Location 1 upstream should be linked")
		assert.Equal(t, "my_app", loc1.Upstream.Name, "Location 1 should be linked to 'my_app' upstream")
		assert.Equal(t, "http", loc1.ProxyPass.Scheme)
		assert.Equal(t, "my_app", loc1.ProxyPass.Host)
		assert.Equal(t, "/v1", loc1.ProxyPass.Path)
		require.NotNil(t, loc1.Cache, "Location 1 should have cache config")
		assert.Equal(t, 10*time.Minute, loc1.Cache.TTL)

		loc2 := internalCfg.Locations[1]
		assert.Equal(t, "/direct/", loc2.NormalizedPath)
		assert.False(t, loc2.IsProxyPassUpstream, "Location 2 should NOT be an upstream proxy")
		assert.Nil(t, loc2.Upstream, "Location 2 upstream should be nil")
		assert.Equal(t, "google.com", loc2.ProxyPass.Host)

		loc3 := internalCfg.Locations[2]
		assert.Equal(t, "/static/", loc3.NormalizedPath)
		assert.Equal(t, "/var/www/static", loc3.Root)
		require.NotNil(t, loc3.RateLimit, "Location 3 should have rate limit config")
		assert.Equal(t, int64(60), loc3.RateLimit.Limit)
		assert.Equal(t, time.Minute, loc3.RateLimit.Window)

		loc4 := internalCfg.Locations[3]
		assert.Equal(t, "/legacy-path/", loc4.NormalizedPath)
	})

	t.Run("EmptyConfig", func(t *testing.T) {
		userCfg := &config.Config{}
		internalCfg, err := BuildInternalConfig(userCfg)

		require.NoError(t, err)
		require.NotNil(t, internalCfg)
		assert.Empty(t, internalCfg.Upstreams)
		assert.Empty(t, internalCfg.Locations)
	})

	t.Run("ErrorOnInvalidProxyPassURL", func(t *testing.T) {
		invalidProxyPass := "http://invalid-host\x7f.com"

		userCfg := &config.Config{
			Locations: config.LocationsConfig{
				{Path: "/", ProxyPass: invalidProxyPass},
			},
		}

		_, err := BuildInternalConfig(userCfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse")
	})
}
