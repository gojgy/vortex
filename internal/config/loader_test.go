package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestConfig(t *testing.T, content string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	configName := "vortex"
	configPath := filepath.Join(dir, configName+".yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err, "Failed to write temp config file")
	return dir, configName
}

func TestLoadConfig(t *testing.T) {
	t.Cleanup(viper.Reset)

	t.Run("HappyPath_FullConfig", func(t *testing.T) {
		yamlContent := `
server:
  listen: ":8000"
  shutdown_timeout: 15s
logging:
  log_level: "debug"
admin:
  listen: ":9091"
upstreams:
  my_app:
    algorithm: "least_connections"
    servers: ["app1.local:8081", "app2.local:8082"]
    health_check:
      interval: 30s
      disable_duration: 5s
locations:
  - path: "/api/"
    proxy_pass: "http://my_app/v1/"
    websocket: true
    timeout: 30s
  - path: "/static/"
    root: "/var/www/"
    cache:
      ttl: 10m
      size: 500
      key: "${uri}"
  - path: "/ratelimit/"
    proxy_pass: "http://my_app"
    rate_limit:
      rpm: 100
      size: 200
      key: "${remote_addr}"
`
		configDir, configName := createTestConfig(t, yamlContent)
		cfg, err := LoadConfig(configDir, configName)

		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, ":8000", cfg.Server.Listen)
		assert.Equal(t, 15*time.Second, cfg.Server.ShutdownTimeout)
		assert.Equal(t, LogLevelDebug, cfg.Logging.Level)
		assert.Equal(t, ":9091", cfg.Admin.Listen)
		require.Contains(t, cfg.Upstreams, "my_app")
		assert.Equal(t, BalancingAlgorithmLeastConnections, cfg.Upstreams["my_app"].Algorithm)
		assert.Equal(t, 30*time.Second, cfg.Upstreams["my_app"].HealthCheck.Interval)
		assert.Equal(t, 10*time.Minute, cfg.Locations[1].Cache.TTL)
		assert.Equal(t, 500, cfg.Locations[1].Cache.Size)
		assert.Equal(t, 100, cfg.Locations[2].RateLimit.RPM)
		assert.Equal(t, 200, cfg.Locations[2].RateLimit.Size)
	})

	t.Run("Defaults_And_IsSet_Logic", func(t *testing.T) {
		yamlContent := `
upstreams:
  app1:
    servers: ["host1:9000"]
    health_check: {}
locations:
  - path: "/default-proxy/"
    proxy_pass: "http://app1"
  - path: "/default-cache/"
    proxy_pass: "http://app1"
    cache: {}
  - path: "/default-ratelimit/"
    proxy_pass: "http://app1"
    rate_limit:
      rpm: 50
`
		configDir, configName := createTestConfig(t, yamlContent)
		cfg, err := LoadConfig(configDir, configName)

		require.NoError(t, err)
		require.NotNil(t, cfg)

		assert.Equal(t, defaultServerListen, cfg.Server.Listen, "Server listen default")
		assert.Equal(t, defaultServerShutdownTimeout, cfg.Server.ShutdownTimeout, "Server shutdown default")
		assert.Equal(t, defaultAdminListen, cfg.Admin.Listen, "Admin listen default")
		assert.Equal(t, defaultLoggingLevel, cfg.Logging.Level, "Logging level default")
		assert.Equal(t, defaultUpstreamsAlgorithm, cfg.Upstreams["app1"].Algorithm, "Upstream algorithm default")

		require.NotNil(t, cfg.Upstreams["app1"].HealthCheck, "HealthCheck should be created for empty key")
		assert.Equal(t, defaultUpstreamsHealthCheckInterval, cfg.Upstreams["app1"].HealthCheck.Interval, "HealthCheck interval default")
		assert.Equal(t, defaultUpstreamsHealthCheckDisableDuration, cfg.Upstreams["app1"].HealthCheck.DisableDuration, "HealthCheck disable duration default")

		assert.Nil(t, cfg.Locations[0].Cache, "Cache should be nil when key is absent")
		assert.Nil(t, cfg.Locations[0].RateLimit, "RateLimit should be nil when key is absent")

		require.NotNil(t, cfg.Locations[1].Cache, "Cache should be created for empty key")
		assert.Equal(t, defaultLocationsCacheTTL, cfg.Locations[1].Cache.TTL, "Cache TTL default")
		assert.Equal(t, defaultLocationsCacheSize, cfg.Locations[1].Cache.Size, "Cache Size default")
		assert.Equal(t, defaultLocationsCacheKey, cfg.Locations[1].Cache.Key, "Cache Key default")

		require.NotNil(t, cfg.Locations[2].RateLimit, "RateLimit should be created for empty key")
		assert.Equal(t, defaultLocationsRateLimitKey, cfg.Locations[2].RateLimit.Key, "RateLimit Key default")
		assert.Equal(t, defaultLocationsRateLimitSize, cfg.Locations[2].RateLimit.Size, "RateLimit Size default")

		assert.Equal(t, defaultLocationsTimeout, cfg.Locations[0].Timeout, "Location timeout default")
	})

	t.Run("EnvironmentVariableExpansion", func(t *testing.T) {
		t.Setenv("TEST_STATIC_PATH", "/tmp/www")
		yamlContent := `
locations:
  - path: "/"
    root: "${TEST_STATIC_PATH}/main"
`
		configDir, configName := createTestConfig(t, yamlContent)
		cfg, err := LoadConfig(configDir, configName)

		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.Len(t, cfg.Locations, 1)
		assert.Equal(t, "/tmp/www/main", cfg.Locations[0].Root)
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		testCases := []struct {
			name          string
			yamlContent   string
			errorContains string
		}{
			{
				name:          "InvalidYAML",
				yamlContent:   `server: { listen: ":8080" } logging: -`,
				errorContains: "unable to read config",
			},
			{
				name:          "UnmarshalError",
				yamlContent:   `server: ":8080"`,
				errorContains: "unable to unmarshal config",
			},
			{
				name:          "ValidationError",
				yamlContent:   `logging: { log_level: "verbose" }`,
				errorContains: "invalid 'logging' section: unknown log level 'verbose'",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				configDir, configName := createTestConfig(t, tc.yamlContent)
				_, err := LoadConfig(configDir, configName)
				require.Error(t, err)
				assert.True(t, strings.Contains(err.Error(), tc.errorContains), "Error message should contain '%s', but was '%s'", tc.errorContains, err.Error())
			})
		}

		t.Run("FileDoesNotExist", func(t *testing.T) {
			_, err := LoadConfig("/non/existent/path", "vortex")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unable to read config file")
		})
	})
}
