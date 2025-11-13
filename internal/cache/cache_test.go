package cache

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildCacheKey(t *testing.T) {
	reqURL, err := url.Parse("https://example.com/api/users?id=123&sort=asc")
	require.NoError(t, err)

	req := &http.Request{
		Method: http.MethodGet,
		Host:   "example.com",
		URL:    reqURL,
		Header: http.Header{
			"X-User-Id":      []string{"user-abc-123"},
			"Accept-Charset": []string{"utf-8"},
			"X-Empty":        []string{""},
		},
	}

	reqTLS := req.Clone(req.Context())
	reqTLS.TLS = &tls.ConnectionState{}

	testCases := []struct {
		name     string
		template string
		request  *http.Request
		wantKey  string
	}{
		{
			name:     "KeyByMethodAndURI",
			template: "${method}:${uri}",
			request:  req,
			wantKey:  "GET:/api/users",
		},
		{
			name:     "KeyByHostAndQuery",
			template: "${host}?${query}",
			request:  req,
			wantKey:  "example.com?id=123&sort=asc",
		},
		{
			name:     "FullDefaultKey_HTTP",
			template: "${method}:${scheme}:${host}${uri}${query}",
			request:  req,
			wantKey:  "GET:http:example.com/api/usersid=123&sort=asc",
		},
		{
			name:     "FullDefaultKey_HTTPS",
			template: "${method}:${scheme}:${host}${uri}${query}",
			request:  reqTLS,
			wantKey:  "GET:https:example.com/api/usersid=123&sort=asc",
		},
		{
			name:     "KeyWithHeader",
			template: "${uri}:${header:X-User-Id}",
			request:  req,
			wantKey:  "/api/users:user-abc-123",
		},
		{
			name:     "KeyWithNonExistentHeader",
			template: "${uri}:${header:X-Auth-Token}",
			request:  req,
			wantKey:  "/api/users:",
		},
		{
			name:     "KeyWithEmptyHeaderValue",
			template: "${uri}:${header:X-Empty}",
			request:  req,
			wantKey:  "/api/users:",
		},
		{
			name:     "KeyWithMultipleHeaders",
			template: "${header:Accept-Charset}|${header:X-User-Id}",
			request:  req,
			wantKey:  "utf-8|user-abc-123",
		},
		{
			name:     "StaticTextOnly",
			template: "my-global-cache-key",
			request:  req,
			wantKey:  "my-global-cache-key",
		},
		{
			name:     "TemplateWithUnknownVariable",
			template: "${method}:${unknown_var}",
			request:  req,
			wantKey:  "GET:${unknown_var}",
		},
		{
			name:     "RequestWithoutQuery",
			template: "${host}${uri}?${query}",
			request:  &http.Request{URL: &url.URL{Path: "/path"}, Host: "host.com"},
			wantKey:  "host.com/path?",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotKey := BuildCacheKey(tc.template, tc.request)
			assert.Equal(t, tc.wantKey, gotKey)
		})
	}
}
