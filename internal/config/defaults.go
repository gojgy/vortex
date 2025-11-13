package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

const (
	defaultServerListen                        = ":8080"
	defaultServerShutdownTimeout               = 10 * time.Second
	defaultLoggingLevel                        = LogLevelInfo
	defaultUpstreamsAlgorithm                  = BalancingAlgorithmRoundRobin
	defaultUpstreamsHealthCheckInterval        = 5 * time.Second
	defaultUpstreamsHealthCheckDisableDuration = 30 * time.Second
	defaultLocationsCacheTTL                   = 10 * time.Minute
	defaultLocationsCacheSize                  = 1000
	defaultLocationsCacheKey                   = "${method}:${host}${uri}${query}"
	defaultLocationsTimeout                    = 1 * time.Minute
	defaultLocationsRateLimitKey               = "${remote_addr}"
	defaultLocationsRateLimitSize              = 10000
	defaultAdminListen                         = ":8081"
)

func (c *Config) setDefaults() {
	if c.Server.Listen == "" {
		c.Server.Listen = defaultServerListen
	}
	if c.Server.ShutdownTimeout == 0 {
		c.Server.ShutdownTimeout = defaultServerShutdownTimeout
	}

	if c.Admin.Listen == "" {
		c.Admin.Listen = defaultAdminListen
	}

	if c.Logging.Level == "" {
		c.Logging.Level = defaultLoggingLevel
	}

	for name, upstream := range c.Upstreams {
		if upstream.Algorithm == "" {
			upstream.Algorithm = defaultUpstreamsAlgorithm
		}
		healthCheckKey := fmt.Sprintf("upstreams.%s.health_check", name)
		if viper.IsSet(healthCheckKey) && upstream.HealthCheck == nil {
			upstream.HealthCheck = &HealthCheck{}
		}
		if upstream.HealthCheck != nil {
			if upstream.HealthCheck.Interval == 0 {
				upstream.HealthCheck.Interval = defaultUpstreamsHealthCheckInterval
			}
			if upstream.HealthCheck.DisableDuration == 0 {
				upstream.HealthCheck.DisableDuration = defaultUpstreamsHealthCheckDisableDuration
			}
		}
		c.Upstreams[name] = upstream
	}

	for i := range c.Locations {
		if c.Locations[i].Cache != nil {
			if c.Locations[i].Cache.TTL == 0 {
				c.Locations[i].Cache.TTL = defaultLocationsCacheTTL
			}
			if c.Locations[i].Cache.Size == 0 {
				c.Locations[i].Cache.Size = defaultLocationsCacheSize
			}
			if c.Locations[i].Cache.Key == "" {
				c.Locations[i].Cache.Key = defaultLocationsCacheKey
			}
		}

		if c.Locations[i].RateLimit != nil {
			if c.Locations[i].RateLimit.Key == "" {
				c.Locations[i].RateLimit.Key = defaultLocationsRateLimitKey
			}
			if c.Locations[i].RateLimit.Size == 0 {
				c.Locations[i].RateLimit.Size = defaultLocationsRateLimitSize
			}
		}

		if c.Locations[i].Timeout == 0 {
			c.Locations[i].Timeout = defaultLocationsTimeout
		}
	}
}
