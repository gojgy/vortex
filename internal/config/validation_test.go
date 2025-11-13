package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestValidateHostPort(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "ValidHostnameAndPort",
			input:       "localhost:8080",
			expectError: false,
		},
		{
			name:        "ValidIPAndPort",
			input:       "127.0.0.1:80",
			expectError: false,
		},
		{
			name:        "ValidHostnameOnly",
			input:       "example.com",
			expectError: false,
		},
		{
			name:        "ValidHostnameWithHyphen",
			input:       "my-service.internal",
			expectError: false,
		},
		{
			name:        "ValidPort1",
			input:       "host:1",
			expectError: false,
		},
		{
			name:        "ValidPort65535",
			input:       "host:65535",
			expectError: false,
		},
		{
			name:        "ValidHostnameWithUnderscore",
			input:       "my_host:8080",
			expectError: false,
		},
		{
			name:        "InvalidPortZero",
			input:       "host:0",
			expectError: true,
		},
		{
			name:        "InvalidPortTooLarge",
			input:       "host:65536",
			expectError: true,
		},
		{
			name:        "InvalidPortNotANumber",
			input:       "host:abc",
			expectError: true,
		},
		{
			name:        "InvalidFormatMissingHost",
			input:       ":8080",
			expectError: true,
		},
		{
			name:        "InvalidFormatMultipleColons",
			input:       "host:80:80",
			expectError: true,
		},
		{
			name:        "EmptyString",
			input:       "",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateHostPort(tc.input)
			if tc.expectError {
				assert.Error(t, err, "Expected an error for input: %s", tc.input)
			} else {
				assert.NoError(t, err, "Did not expect an error for input: %s", tc.input)
			}
		})
	}
}

func TestIsProxyPassURL(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "ValidHttpURL",
			input: "http://example.com",
			want:  true,
		},
		{
			name:  "ValidHttpsURLWithPort",
			input: "https://localhost:8080",
			want:  true,
		},
		{
			name:  "ValidURLWithIpAndPath",
			input: "http://127.0.0.1/api/v1",
			want:  true,
		},
		{
			name:  "ValidUpstreamStyleURL",
			input: "http://my_backend",
			want:  true,
		},
		{
			name:  "ValidURLWithPathSuffix",
			input: "http://my_backend/api/",
			want:  true,
		},
		{
			name:  "InvalidMissingScheme",
			input: "example.com",
			want:  false,
		},
		{
			name:  "InvalidURLWithQuery",
			input: "http://example.com?id=123",
			want:  false,
		},
		{
			name:  "InvalidURLWithFragment",
			input: "http://example.com#section",
			want:  false,
		},
		{
			name:  "InvalidURLWithoutHost",
			input: "http:///path",
			want:  false,
		},
		{
			name:  "InvalidJustAPath",
			input: "/some/path",
			want:  false,
		},
		{
			name:  "InvalidEmptyString",
			input: "",
			want:  false,
		},
		{
			name:  "InvalidJustScheme",
			input: "http://",
			want:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := isProxyPassURL(tc.input)
			assert.Equal(t, tc.want, got, "For input: %s", tc.input)
		})
	}
}

func TestIsValidDirPath(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "ValidAbsolutePath",
			input:       "/var/www/static",
			expectError: false,
		},
		{
			name:        "ValidRootPath",
			input:       "/",
			expectError: false,
		},
		{
			name:        "InvalidRelativePath",
			input:       "var/www/static",
			expectError: true,
		},
		{
			name:        "InvalidRelativePathWithDots",
			input:       "../www",
			expectError: true,
		},
		{
			name:        "InvalidEmptyString",
			input:       "",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := isValidDirPath(tc.input)
			if tc.expectError {
				assert.Error(t, err, "Expected an error for input: %s", tc.input)
			} else {
				assert.NoError(t, err, "Did not expect an error for input: %s", tc.input)
			}
		})
	}
}

func TestIsAbsoluteNormalizedPath(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "ValidRootPath",
			input: "/",
			want:  true,
		},
		{
			name:  "ValidPathWithTrailingSlash",
			input: "/api/",
			want:  true,
		},
		{
			name:  "ValidLongPathWithTrailingSlash",
			input: "/api/v1/users/",
			want:  true,
		},
		{
			name:  "MissingLeadingSlash",
			input: "api/",
			want:  false,
		},
		{
			name:  "MissingTrailingSlash",
			input: "/api",
			want:  false,
		},
		{
			name:  "EmptyString",
			input: "",
			want:  false,
		},
		{
			name:  "PathWithQuery",
			input: "/api/?user=123",
			want:  false,
		},
		{
			name:  "PathWithFragment",
			input: "/api/#top",
			want:  false,
		},
		{
			name:  "FullURL",
			input: "http://example.com/path/",
			want:  false,
		},
		{
			name:  "PathWithDoubleSlashInMiddle",
			input: "/api//v1/",
			want:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := isAbsoluteNormalizedPath(tc.input)
			assert.Equal(t, tc.want, got, "For input: %s", tc.input)
		})
	}
}

func TestServerConfig_Validate(t *testing.T) {
	testCases := []struct {
		name    string
		config  ServerConfig
		wantErr bool
	}{
		{name: "ValidListenAddress", config: ServerConfig{Listen: ":8080"}, wantErr: false},
		{name: "ValidListenAddressWithHost", config: ServerConfig{Listen: "127.0.0.1:8080"}, wantErr: false},
		{name: "InvalidListenAddressMissingPort", config: ServerConfig{Listen: "localhost"}, wantErr: true},
		{name: "InvalidListenAddressMissingColon", config: ServerConfig{Listen: "localhost8080"}, wantErr: true},
		{name: "EmptyListenAddress", config: ServerConfig{Listen: ""}, wantErr: true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLoggingConfig_Validate(t *testing.T) {
	testCases := []struct {
		name    string
		config  LoggingConfig
		wantErr bool
	}{
		{name: "ValidLevelInfo", config: LoggingConfig{Level: LogLevelInfo}, wantErr: false},
		{name: "ValidLevelDebug", config: LoggingConfig{Level: LogLevelDebug}, wantErr: false},
		{name: "ValidLevelError", config: LoggingConfig{Level: LogLevelError}, wantErr: false},
		{name: "InvalidLevel", config: LoggingConfig{Level: "verbose"}, wantErr: true},
		{name: "EmptyLevel", config: LoggingConfig{Level: ""}, wantErr: true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpstreamsConfig_Validate(t *testing.T) {
	testCases := []struct {
		name        string
		config      UpstreamsConfig
		wantErr     bool
		errContains string
	}{
		{
			name: "ValidUpstream",
			config: UpstreamsConfig{
				"my_app": {
					Algorithm: BalancingAlgorithmRoundRobin,
					Servers:   []string{"server1:80", "server2"},
				},
			},
			wantErr: false,
		},
		{
			name: "InvalidBalancingAlgorithm",
			config: UpstreamsConfig{
				"my_app": {Algorithm: "random", Servers: []string{"server1:80", "server2:80"}},
			},
			wantErr:     true,
			errContains: "unknown balancing algorithm",
		},
		{
			name: "InvalidNotEnoughServers",
			config: UpstreamsConfig{
				"my_app": {Algorithm: BalancingAlgorithmRoundRobin},
			},
			wantErr:     true,
			errContains: "at least one server must be specified",
		},
		{
			name: "InvalidServerURLPath",
			config: UpstreamsConfig{
				"my_app": {Algorithm: BalancingAlgorithmRoundRobin, Servers: []string{"server1:80/api", "invalid-url"}},
			},
			wantErr:     true,
			errContains: "invalid host:port format",
		},
		{
			name: "InvalidServerURLScheme",
			config: UpstreamsConfig{
				"my_app": {Algorithm: BalancingAlgorithmRoundRobin, Servers: []string{"server1:80", "http://invalid-url"}},
			},
			wantErr:     true,
			errContains: "invalid host:port format",
		},
		{
			name: "ValidHealthCheck",
			config: UpstreamsConfig{
				"my_app": {
					Algorithm:   BalancingAlgorithmRoundRobin,
					Servers:     []string{"server1:80", "server2:80"},
					HealthCheck: &HealthCheck{Interval: 10 * time.Second, DisableDuration: 5 * time.Second},
				},
			},
			wantErr: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.wantErr {
				assert.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLocationsConfig_Validate(t *testing.T) {
	testCases := []struct {
		name        string
		config      LocationsConfig
		wantErr     bool
		errContains string
	}{
		{
			name: "ValidProxyPassLocation",
			config: LocationsConfig{
				{Path: "/api/", ProxyPass: "http://my_app"},
			},
			wantErr: false,
		},
		{
			name: "ValidRootLocation",
			config: LocationsConfig{
				{Path: "/static/", Root: "/var/www"},
			},
			wantErr: false,
		},
		{
			name:        "InvalidNoLocations",
			config:      LocationsConfig{},
			wantErr:     true,
			errContains: "at least one location must be specified",
		},
		{
			name: "InvalidMissingPath",
			config: LocationsConfig{
				{ProxyPass: "http://my_app"},
			},
			wantErr:     true,
			errContains: "path must be specified",
		},
		{
			name: "InvalidPathFormat",
			config: LocationsConfig{
				{Path: "api", ProxyPass: "http://my_app"},
			},
			wantErr:     true,
			errContains: "should be an absolute path",
		},
		{
			name: "InvalidProxyPassAndRoot",
			config: LocationsConfig{
				{Path: "/", ProxyPass: "http://my_app", Root: "/var/www"},
			},
			wantErr:     true,
			errContains: "proxy_pass and root are used simultaneously",
		},
		{
			name: "InvalidNeitherProxyPassNorRoot",
			config: LocationsConfig{
				{Path: "/"},
			},
			wantErr:     true,
			errContains: "either proxy_pass or root must be set",
		},
		{
			name: "InvalidProxyPassURLFormat",
			config: LocationsConfig{
				{Path: "/", ProxyPass: "my_app"},
			},
			wantErr:     true,
			errContains: "invalid proxy_pass",
		},
		{
			name: "InvalidRootPathFormat",
			config: LocationsConfig{
				{Path: "/", Root: "var/www"},
			},
			wantErr:     true,
			errContains: "is not an absolute path",
		},
		{
			name: "InvalidDuplicatePath",
			config: LocationsConfig{
				{Path: "/api/", ProxyPass: "http://app1"},
				{Path: "/api/", ProxyPass: "http://app2"},
			},
			wantErr:     true,
			errContains: "path should be unique",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.wantErr {
				assert.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCacheKeyTemplate(t *testing.T) {
	testCases := []struct {
		name        string
		template    string
		expectError bool
		errContains string
	}{
		{name: "ValidFullKey", template: "${method}:${scheme}:${host}${uri}${query}", expectError: false},
		{name: "ValidHeaderKey", template: "${uri}${header:X-User-ID}", expectError: false},
		{name: "ValidHeaderKeyWithHyphens", template: "${uri}${header:Content-Type}", expectError: false},
		{name: "StaticTextOnly", template: "my-static-key", expectError: false},
		{name: "CombinedStaticAndVars", template: "cache:${host}${uri}", expectError: false},
		{name: "InvalidVariable", template: "${host}${unknown_var}", expectError: true, errContains: "unknown variable"},
		{name: "InvalidHeaderSyntax", template: "${header:}", expectError: true, errContains: "unknown variable"},
		{name: "MalformedVariable", template: "${host", expectError: false},
		{name: "VariableNotAllowedInCache", template: "${remote_addr}", expectError: true, errContains: "unknown variable"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCacheKeyTemplate(tc.template)
			if tc.expectError {
				assert.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateRateLimitKeyTemplate(t *testing.T) {
	testCases := []struct {
		name        string
		template    string
		expectError bool
		errContains string
	}{
		{name: "ValidRemoteAddr", template: "${remote_addr}", expectError: false},
		{name: "ValidCombinedKey", template: "${host}${uri}", expectError: false},
		{name: "StaticTextIsAllowed", template: "global", expectError: false},
		{name: "InvalidVariable", template: "${unknown}", expectError: true, errContains: "unknown or unsupported variable"},
		{name: "HeaderVariableNotAllowed", template: "${header:X-Real-IP}", expectError: true, errContains: "unknown or unsupported variable"},
		{name: "MethodVariableNotAllowed", template: "${method}", expectError: true, errContains: "unknown or unsupported variable"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRateLimitKeyTemplate(tc.template)
			if tc.expectError {
				assert.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCacheConfig_Validate(t *testing.T) {
	testCases := []struct {
		name    string
		config  CacheConfig
		wantErr bool
	}{
		{name: "ValidConfig", config: CacheConfig{Size: 100, TTL: time.Minute, Key: "${host}${uri}"}, wantErr: false},
		{name: "InvalidSizeZero", config: CacheConfig{Size: 0, TTL: time.Minute, Key: "${host}${uri}"}, wantErr: true},
		{name: "InvalidSizeNegative", config: CacheConfig{Size: -1, TTL: time.Minute, Key: "${host}${uri}"}, wantErr: true},
		{name: "InvalidTTLZero", config: CacheConfig{Size: 100, TTL: 0, Key: "${host}${uri}"}, wantErr: true},
		{name: "InvalidTTLNegative", config: CacheConfig{Size: 100, TTL: -time.Second, Key: "${host}${uri}"}, wantErr: true},
		{name: "InvalidEmptyKey", config: CacheConfig{Size: 100, TTL: time.Minute, Key: ""}, wantErr: true},
		{name: "InvalidKeyTemplate", config: CacheConfig{Size: 100, TTL: time.Minute, Key: "${bad_var}"}, wantErr: true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			assert.Equal(t, tc.wantErr, err != nil)
		})
	}
}

func TestRateLimitConfig_Validate(t *testing.T) {
	testCases := []struct {
		name    string
		config  RateLimitConfig
		wantErr bool
	}{
		{name: "ValidConfig", config: RateLimitConfig{RPM: 100, Size: 1000, Key: "${remote_addr}"}, wantErr: false},
		{name: "InvalidRPMZero", config: RateLimitConfig{RPM: 0, Size: 1000, Key: "${remote_addr}"}, wantErr: true},
		{name: "InvalidRPMNegative", config: RateLimitConfig{RPM: -1, Size: 1000, Key: "${remote_addr}"}, wantErr: true},
		{name: "InvalidSizeZero", config: RateLimitConfig{RPM: 100, Size: 0, Key: "${remote_addr}"}, wantErr: true},
		{name: "InvalidSizeNegative", config: RateLimitConfig{RPM: 100, Size: -1, Key: "${remote_addr}"}, wantErr: true},
		{name: "InvalidEmptyKey", config: RateLimitConfig{RPM: 100, Size: 1000, Key: ""}, wantErr: true},
		{name: "InvalidKeyTemplate", config: RateLimitConfig{RPM: 100, Size: 1000, Key: "${method}"}, wantErr: true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			assert.Equal(t, tc.wantErr, err != nil)
		})
	}
}
