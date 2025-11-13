package config

import (
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"vortex/internal/utils"
)

type Validator interface {
	Validate() error
}

func (s *ServerConfig) Validate() error {
	_, _, err := net.SplitHostPort(s.Listen)
	return err
}

var validLogLevels = []LogLevel{LogLevelInfo, LogLevelDebug, LogLevelError}

func (l *LoggingConfig) Validate() error {
	if !utils.Contains(l.Level, validLogLevels) {
		return fmt.Errorf("unknown log level '%s': valid levels are %v", l.Level, validLogLevels)
	}
	return nil
}

var validBalancingAlgorithms = []BalancingAlgorithm{BalancingAlgorithmRoundRobin, BalancingAlgorithmLeastConnections}

func (u *UpstreamsConfig) Validate() error {
	for upstreamName, upstreamCfg := range *u {
		if !utils.Contains(upstreamCfg.Algorithm, validBalancingAlgorithms) {
			return fmt.Errorf("invalid upstream '%s' unknown balancing algorithm '%s': unknown balancing algorithm, valid algorithms are %v", upstreamName, upstreamCfg.Algorithm, validBalancingAlgorithms)
		}

		if len(upstreamCfg.Servers) == 0 {
			return fmt.Errorf("invalid upstream '%s': at least one server must be specified", upstreamName)
		}
		for _, server := range upstreamCfg.Servers {
			if err := validateHostPort(server); err != nil {
				return fmt.Errorf("invalid upstream '%s' server url '%s': %w", upstreamName, server, err)
			}
		}
	}
	return nil
}

func (l *LocationsConfig) Validate() error {
	if len(*l) == 0 {
		return fmt.Errorf("invalid location: at least one location must be specified")
	}

	pathsMp := make(map[string]struct{}, len(*l))

	for _, locationCfg := range *l {
		if locationCfg.Path == "" {
			return fmt.Errorf("invalid location: path must be specified")
		}
		if !isAbsoluteNormalizedPath(locationCfg.Path) {
			return fmt.Errorf("invalid location path '%s': should be an absolute path", locationCfg.Path)
		}
		if locationCfg.ProxyPass != "" && locationCfg.Root != "" {
			return fmt.Errorf("invalid loication: proxy_pass and root are used simultaneously")
		}
		if locationCfg.ProxyPass == "" && locationCfg.Root == "" {
			return fmt.Errorf("invalid location for path '%s': either proxy_pass or root must be set", locationCfg.Path)
		}
		if locationCfg.ProxyPass != "" {
			if !isProxyPassURL(locationCfg.ProxyPass) {
				return fmt.Errorf("invalid proxy_pass '%s': must be a full URL ('http://host[:port][/path]') or upstream URL ('http://upstream[/path]')", locationCfg.ProxyPass)
			}
		}
		if locationCfg.Root != "" {
			if err := isValidDirPath(locationCfg.Root); err != nil {
				return err
			}
		}
		if locationCfg.Cache != nil {
			if err := locationCfg.Cache.Validate(); err != nil {
				return fmt.Errorf("invalid cache config for location '%s': %w", locationCfg.Path, err)
			}
		}
		if locationCfg.RateLimit != nil {
			if err := locationCfg.RateLimit.Validate(); err != nil {
				return fmt.Errorf("invalid rate_limit config for location '%s': %w", locationCfg.Path, err)
			}
		}
		if _, ok := pathsMp[locationCfg.Path]; ok {
			return fmt.Errorf("invalid location path '%s': path should be unique", locationCfg.Path)
		}
		pathsMp[locationCfg.Path] = struct{}{}
	}

	return nil
}

func (cc *CacheConfig) Validate() error {
	if cc != nil {
		if cc.Size <= 0 {
			return fmt.Errorf("cache size must be a positive number, but got %d", cc.Size)
		}
		if cc.TTL <= 0 {
			return fmt.Errorf("cache TTL must be a positive duration, but got %s", cc.TTL)
		}
		if cc.Key == "" {
			return fmt.Errorf("cache key template cannot be empty")
		}
		return validateCacheKeyTemplate(cc.Key)
	}
	return nil
}

func (rlc *RateLimitConfig) Validate() error {
	if rlc.RPM <= 0 {
		return fmt.Errorf("rate limit rpm must be a positive number, but got %d", rlc.RPM)
	}
	if rlc.Size <= 0 {
		return fmt.Errorf("rate limit 'size' must be a positive number, but got %d", rlc.Size)
	}
	if rlc.Key == "" {
		return fmt.Errorf("rate limit key cannot be empty")
	}
	return validateRateLimitKeyTemplate(rlc.Key)
}

func (a *AdminConfig) Validate() error {
	_, _, err := net.SplitHostPort(a.Listen)
	return err
}

func (c *Config) validateConfig() error {
	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("invalid 'server' section: %w", err)
	}
	if err := c.Logging.Validate(); err != nil {
		return fmt.Errorf("invalid 'logging' section: %w", err)
	}

	if err := c.Upstreams.Validate(); err != nil {
		return fmt.Errorf("invalid 'upstreams' section: %w", err)
	}

	if err := c.Locations.Validate(); err != nil {
		return fmt.Errorf("invalid 'locations' section: %w", err)
	}

	if err := c.Admin.Validate(); err != nil {
		return fmt.Errorf("invalid 'admin' section: %w", err)
	}

	return nil
}

var regexpHostPort = regexp.MustCompile(`^([a-zA-Z0-9_.-]+)(:([1-9][0-9]{0,4}))?$`)

func validateHostPort(s string) error {
	matches := regexpHostPort.FindStringSubmatch(s)
	if matches == nil {
		return fmt.Errorf("invalid host:port format '%s'", s)
	}

	if port := matches[3]; port != "" {
		i, _ := strconv.Atoi(port)
		if i > 65535 {
			return fmt.Errorf("port out of range: %s", port)
		}
	}

	return nil
}

func isAbsoluteNormalizedPath(rawURL string) bool {
	if !strings.HasPrefix(rawURL, "/") || !strings.HasSuffix(rawURL, "/") {
		return false
	}

	if strings.Contains(rawURL, "//") {
		return false
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	return u.Scheme == "" && u.Host == "" && u.RawQuery == "" && u.Fragment == ""
}

func isProxyPassURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	if u.Scheme == "" || u.Host == "" {
		return false
	}

	if u.RawQuery != "" || u.Fragment != "" {
		return false
	}

	return true
}

func isValidDirPath(path string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	if !filepath.IsAbs(path) {
		return fmt.Errorf("path '%s' is not an absolute path", path)
	}

	return nil
}

var (
	validCacheKeyVars   = []string{"${method}", "${scheme}", "${host}", "${uri}", "${query}"}
	cacheKeyVarRegex    = regexp.MustCompile(`\$\{[^}]+\}`)
	headerCacheKeyRegex = regexp.MustCompile(`^\${header:([a-zA-Z0-9-]+)}$`)
)

func validateCacheKeyTemplate(template string) error {
	vars := cacheKeyVarRegex.FindAllString(template, -1)
	for _, v := range vars {
		isStaticVar := utils.Contains(v, validCacheKeyVars)
		isHeaderVar := headerCacheKeyRegex.MatchString(v)

		if !isStaticVar && !isHeaderVar {
			return fmt.Errorf("unknown variable '%s' in cache key template, valids are %v", v, validCacheKeyVars)
		}
	}
	return nil
}

var (
	validRateLimitKeyVars = []string{"${remote_addr}", "${host}", "${uri}"}
	rateLimitKeyVarRegex  = regexp.MustCompile(`\$\{[^}]+\}`)
)

func validateRateLimitKeyTemplate(template string) error {
	vars := rateLimitKeyVarRegex.FindAllString(template, -1)
	if len(vars) == 0 && template != "" {
		return nil
	}

	for _, v := range vars {
		if !utils.Contains(v, validRateLimitKeyVars) {
			return fmt.Errorf("unknown or unsupported variable '%s' in rate limit key template, valid are %v", v, validRateLimitKeyVars)
		}
	}
	return nil
}
