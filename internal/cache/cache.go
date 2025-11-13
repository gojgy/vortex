package cache

import (
	"net/http"
	"regexp"
	"strings"
)

// TODO: дублирование константы
var (
	headerVarRegex = regexp.MustCompile(`\${header:([a-zA-Z0-9-]+)}`)
)

func BuildCacheKey(template string, r *http.Request) string {
	key := strings.NewReplacer(
		"${method}", r.Method,
		"${scheme}", scheme(r),
		"${host}", r.Host,
		"${uri}", r.URL.Path,
		"${query}", r.URL.RawQuery,
	).Replace(template)

	return headerVarRegex.ReplaceAllStringFunc(key, func(match string) string {
		parts := headerVarRegex.FindStringSubmatch(match)
		if len(parts) == 2 {
			return r.Header.Get(parts[1])
		}
		return ""
	})
}

func scheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}
