package core

import (
	"fmt"
	"net/url"
	"time"
	"unicode/utf8"
	"vortex/internal/config"
	"vortex/internal/utils"
)

func BuildInternalConfig(userCfg *config.Config) (*InternalConfig, error) {
	internalCfg := &InternalConfig{
		Upstreams: make(map[string]*InternalUpstream),
		Locations: make([]*InternalLocation, 0, len(userCfg.Locations)),
	}

	for name, upstreamCfg := range userCfg.Upstreams {
		servers := make([]string, len(upstreamCfg.Servers))
		copy(servers, upstreamCfg.Servers)

		internalCfg.Upstreams[name] = &InternalUpstream{
			Name:      name,
			Servers:   servers,
			Algorithm: string(upstreamCfg.Algorithm),
		}
	}

	for _, locCfg := range userCfg.Locations {
		normalizedPath := utils.NormalizeRequestPath(locCfg.Path)
		parsedProxyPass, err := url.Parse(locCfg.ProxyPass)
		if err != nil {
			return nil, fmt.Errorf("internal error: failed to parse upstream server URL '%s': %w", locCfg.ProxyPass, err)
		}
		hostname := parsedProxyPass.Host
		upstream, isUpstream := internalCfg.Upstreams[hostname]

		internalLoc := &InternalLocation{
			NormalizedPath:      normalizedPath,
			NormalizedPathLen:   utf8.RuneCountInString(normalizedPath),
			IsProxyPassUpstream: isUpstream,
			ProxyPass:           parsedProxyPass,
			Timeout:             locCfg.Timeout,
			Root:                locCfg.Root,
			WebSocketEnabled:    locCfg.WebSocketEnabled,
		}

		if isUpstream {
			internalLoc.Upstream = upstream
		}

		if locCfg.Cache != nil {
			internalLoc.Cache = &InternalCacheConfig{
				TTL:  locCfg.Cache.TTL,
				Size: locCfg.Cache.Size,
				Key:  locCfg.Cache.Key,
			}
		}

		if locCfg.RateLimit != nil {
			internalLoc.RateLimit = &InternalRateLimitConfig{
				Limit:  int64(locCfg.RateLimit.RPM),
				Window: time.Minute, // TODO: изменить при использовании более гибкого конфига с настройками для рейт лимитинга
				Key:    locCfg.RateLimit.Key,
				Size:   locCfg.RateLimit.Size,
			}
		}

		internalCfg.Locations = append(internalCfg.Locations, internalLoc)
	}

	// TODO: можно добавить сортировку локейшенов для более эффективного поиска

	return internalCfg, nil
}
