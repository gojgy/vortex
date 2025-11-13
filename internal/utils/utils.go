package utils

import (
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
)

func Contains[T comparable](target T, list []T) bool {
	for _, v := range list {
		if target == v {
			return true
		}
	}
	return false
}

func GetHostFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return parsed.Host
}

func NormalizeRequestPath(requestPath string) string {
	if requestPath == "" {
		return "/"
	}
	if requestPath[0] != '/' {
		requestPath = "/" + requestPath
	}

	if !strings.HasSuffix(requestPath, "/") && strings.Contains(path.Base(requestPath), ".") {
		return requestPath
	}

	if !strings.HasSuffix(requestPath, "/") {
		requestPath += "/"
	}

	return requestPath
}

func GetClientIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return strings.Trim(r.RemoteAddr, "[]")
	}
	return ip
}
