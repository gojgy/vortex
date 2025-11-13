package vortex

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"
	"vortex/internal/balancer"
	"vortex/internal/core"
	"vortex/internal/runtime"
	"vortex/internal/utils"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLocation(path string) *core.InternalLocation {
	normalizedPath := utils.NormalizeRequestPath(path)
	return &core.InternalLocation{
		NormalizedPath:    normalizedPath,
		NormalizedPathLen: utf8.RuneCountInString(normalizedPath),
	}
}

func TestResolveLocation(t *testing.T) {
	testConfig := &core.InternalConfig{
		Locations: []*core.InternalLocation{
			newTestLocation("/"),
			newTestLocation("/api/"),
			newTestLocation("/api/v2/"),
			newTestLocation("/images/"),
		},
	}

	vtx := NewVortex(testConfig, nil, nil, zerolog.Nop())

	testCases := []struct {
		name         string
		requestPath  string
		wantLocation *core.InternalLocation
	}{
		{
			name:         "ExactMatchRoot",
			requestPath:  "/",
			wantLocation: testConfig.Locations[0],
		},
		{
			name:         "SimplePrefixMatch",
			requestPath:  "/api/users",
			wantLocation: testConfig.Locations[1],
		},
		{
			name:         "LongestPrefixWins",
			requestPath:  "/api/v2/users",
			wantLocation: testConfig.Locations[2],
		},
		{
			name:         "DifferentBranch",
			requestPath:  "/images/jpeg/cat.jpg",
			wantLocation: testConfig.Locations[3],
		},
		{
			name:         "RequestPathNeedsNormalization",
			requestPath:  "api/v2/users",
			wantLocation: testConfig.Locations[2],
		},
		{
			name:         "RootFallback",
			requestPath:  "/something/else",
			wantLocation: testConfig.Locations[0],
		},
		{
			name:         "NoMatchingLocationButRootExists",
			requestPath:  "/unmatched",
			wantLocation: testConfig.Locations[0],
		},
	}

	t.Run("EmptyConfig", func(t *testing.T) {
		emptyCfg := &core.InternalConfig{}
		reqURL, _ := url.Parse("/any/path")
		got := vtx.resolveLocation(emptyCfg, reqURL)
		assert.Nil(t, got, "Expected nil for empty config")
	})

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqURL, err := url.Parse(tc.requestPath)
			require.NoError(t, err)

			got := vtx.resolveLocation(testConfig, reqURL)

			assert.Equal(t, tc.wantLocation, got)
		})
	}
}

func TestResolveProxyURL(t *testing.T) {
	locationWithSlash := &core.InternalLocation{
		NormalizedPath: "/api/v1/",
	}
	locationWithoutSlash := &core.InternalLocation{
		NormalizedPath: "/app/",
	}

	backendWithSlash, _ := url.Parse("http://service.local/proxy/")
	backendWithoutSlash, _ := url.Parse("http://service.local/proxy")
	backendRoot, _ := url.Parse("http://service.local/")
	backendNoPath, _ := url.Parse("http://service.local")

	testCases := []struct {
		name       string
		location   *core.InternalLocation
		requestURL string
		backendURL *url.URL
		wantURL    string
	}{
		{
			name:       "BackendWithSlash_RequestWithSubpath",
			location:   locationWithSlash,
			requestURL: "http://localhost/api/v1/users/123",
			backendURL: backendWithSlash,
			wantURL:    "http://service.local/proxy/users/123/",
		},
		{
			name:       "BackendWithoutSlash_RequestWithSubpath",
			location:   locationWithSlash,
			requestURL: "http://localhost/api/v1/users/123",
			backendURL: backendWithoutSlash,
			wantURL:    "http://service.local/proxy/users/123/",
		},
		{
			name:       "BackendIsRoot_RequestWithSubpath",
			location:   locationWithSlash,
			requestURL: "http://localhost/api/v1/users/123",
			backendURL: backendRoot,
			wantURL:    "http://service.local/users/123/",
		},
		{
			name:       "BackendHasNoPath_RequestWithSubpath",
			location:   locationWithoutSlash,
			requestURL: "http://localhost/app/settings",
			backendURL: backendNoPath,
			wantURL: "http://service.local/app/settings/",
		},
		{
			name:       "RequestIsSameAsLocation",
			location:   locationWithSlash,
			requestURL: "http://localhost/api/v1/",
			backendURL: backendWithSlash,
			wantURL:    "http://service.local/proxy/",
		},
	}

	vtx := NewVortex(nil, nil, nil, zerolog.Nop())

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqURL, err := url.Parse(tc.requestURL)
			require.NoError(t, err)
			gotURL, err := vtx.resolveProxyURL(tc.location, reqURL, tc.backendURL)
			require.NoError(t, err)
			assert.Equal(t, tc.wantURL, gotURL)
		})
	}
}

func TestServeProxy(t *testing.T) {
	t.Run("HappyPath", func(t *testing.T) {
		backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.NotEmpty(t, r.Header.Get("X-Real-IP"), "X-Real-IP should be set")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Hello from backend"))
		}))
		defer backendServer.Close()

		backendURL, err := url.Parse(backendServer.URL)
		require.NoError(t, err)

		upstream := &core.InternalUpstream{
			Name:      "app-upstream",
			Algorithm: balancer.AlgorithmLeastConnections,
			Servers:   []string{backendURL.Host},
		}

		location := &core.InternalLocation{
			NormalizedPath:      "/proxy/",
			NormalizedPathLen:   len("/proxy/"),
			IsProxyPassUpstream: true,
			Timeout:             1 * time.Second,
			ProxyPass: &url.URL{
				Scheme: "http",
				Host:   "app-upstream",
			},
			Upstream: upstream,
		}

		internalCfg := &core.InternalConfig{
			Locations: []*core.InternalLocation{location},
			Upstreams: map[string]*core.InternalUpstream{
				"app-upstream": upstream,
			},
		}

		runtimeState := runtime.NewRuntimeState(internalCfg)
		require.Contains(t, runtimeState.Upstreams, "app-upstream")

		blc := balancer.NewVortexBalancer(runtimeState)
		vtx := NewVortex(internalCfg, nil, blc, zerolog.Nop())

		recorder := httptest.NewRecorder()

		clientRequest := httptest.NewRequest("GET", "http://vortex.test/proxy/path", nil)
		clientRequest.RemoteAddr = "1.2.3.4:12345"

		err = vtx.serveProxy(location, recorder, clientRequest)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, recorder.Code, "Response code should match backend")
		assert.Equal(t, "Hello from backend", recorder.Body.String(), "Response body should match backend")
		assert.Equal(t, "vortex", recorder.Header().Get("Server"), "Server header should be 'vortex'")

		lcCounter := runtimeState.Upstreams["app-upstream"].ServerConnections[backendURL.Host]
		assert.Equal(t, int64(0), lcCounter.Load(), "Least connections counter should be 0 after request completion")

	})
	t.Run("BackendReturnsError", func(t *testing.T) {
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Service is down for maintenance"))
		}))
		defer backend.Close()

		backendURL, _ := url.Parse(backend.URL)

		upstream := &core.InternalUpstream{
			Name:      "error-upstream",
			Algorithm: balancer.AlgorithmRoundRobin,
			Servers:   []string{backendURL.Host},
		}
		location := &core.InternalLocation{
			NormalizedPath:      "/proxy/",
			IsProxyPassUpstream: true,
			Timeout:             1 * time.Second,
			ProxyPass:           &url.URL{Scheme: "http", Host: "error-upstream"},
			Upstream:            upstream,
		}
		internalCfg := &core.InternalConfig{
			Locations: []*core.InternalLocation{location},
			Upstreams: map[string]*core.InternalUpstream{"error-upstream": upstream},
		}
		runtimeState := runtime.NewRuntimeState(internalCfg)
		blc := balancer.NewVortexBalancer(runtimeState)
		vtx := NewVortex(internalCfg, nil, blc, zerolog.Nop())

		recorder := httptest.NewRecorder()
		clientRequest := httptest.NewRequest("GET", "http://vortex.test/proxy/path", nil)

		err := vtx.serveProxy(location, recorder, clientRequest)
		require.NoError(t, err)
		assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
		assert.Equal(t, "Service is down for maintenance", recorder.Body.String())
	})
}

func TestServeStatic(t *testing.T) {
	rootDir, err := os.MkdirTemp("", "vortex-static-test-")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "index.html"), []byte("root index"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(rootDir, "css"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "css", "style.css"), []byte("body { color: blue; }"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(rootDir, "empty_dir"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(rootDir, "dir_with_dir_index", "index.html"), 0755))

	secretDir := t.TempDir()
	secretFile := filepath.Join(secretDir, "secret.txt")
	require.NoError(t, os.WriteFile(secretFile, []byte("this is a secret"), 0644))

	symlinkPath := filepath.Join(rootDir, "link_to_secret")
	require.NoError(t, os.Symlink(secretFile, symlinkPath))

	testCases := []struct {
		name                 string
		requestPath          string
		requestMethod        string
		expectedStatus       int
		expectedBodyContains string
		expectErrorReturn    bool
	}{
		{
			name:                 "ServeRootIndex",
			requestPath:          "/static/",
			requestMethod:        http.MethodGet,
			expectedStatus:       http.StatusOK,
			expectedBodyContains: "root index",
			expectErrorReturn:    false,
		},
		{
			name:                 "ServeFileInSubdir",
			requestPath:          "/static/css/style.css",
			requestMethod:        http.MethodGet,
			expectedStatus:       http.StatusOK,
			expectedBodyContains: "body { color: blue; }",
			expectErrorReturn:    false,
		},
		{
			name:                 "ServeWithHeadMethod",
			requestPath:          "/static/css/style.css",
			requestMethod:        http.MethodHead,
			expectedStatus:       http.StatusOK,
			expectedBodyContains: "",
			expectErrorReturn:    false,
		},
		{
			name:                 "FileNotFound",
			requestPath:          "/static/nonexistent.file",
			requestMethod:        http.MethodGet,
			expectedStatus:       http.StatusNotFound,
			expectedBodyContains: "404 page not found",
			expectErrorReturn:    false,
		},
		{
			name:                 "DirectoryListingForbidden",
			requestPath:          "/static/empty_dir/",
			requestMethod:        http.MethodGet,
			expectedStatus:       http.StatusForbidden,
			expectedBodyContains: "Forbidden",
			expectErrorReturn:    true,
		},
		{
			name:                 "IndexIsADirectory",
			requestPath:          "/static/dir_with_dir_index/",
			requestMethod:        http.MethodGet,
			expectedStatus:       http.StatusForbidden,
			expectedBodyContains: "Forbidden",
			expectErrorReturn:    true,
		},
		{
			name:                 "PathTraversalSimple",
			requestPath:          "/static/../secret.txt",
			requestMethod:        http.MethodGet,
			expectedStatus:       http.StatusNotFound,
			expectedBodyContains: "Not Found",
			expectErrorReturn:    true,
		},
		{
			name:                 "SymlinkAttackToOutsideRoot",
			requestPath:          "/static/link_to_secret",
			requestMethod:        http.MethodGet,
			expectedStatus:       http.StatusNotFound,
			expectedBodyContains: "Not Found",
			expectErrorReturn:    true,
		},
		{
			name:                 "MethodNotAllowed",
			requestPath:          "/static/index.html",
			requestMethod:        http.MethodPost,
			expectedStatus:       http.StatusMethodNotAllowed,
			expectedBodyContains: "Method Not Allowed",
			expectErrorReturn:    true,
		},
	}

	location := &core.InternalLocation{
		Root:           rootDir,
		NormalizedPath: "/static/",
	}
	vtx := NewVortex(nil, nil, nil, zerolog.Nop())

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tc.requestMethod, tc.requestPath, nil)

			err := vtx.serveStatic(location, recorder, request)

			assert.Equal(t, tc.expectedStatus, recorder.Code, "unexpected status code")

			if tc.expectedBodyContains != "" {
				assert.Contains(t, recorder.Body.String(), tc.expectedBodyContains, "response body mismatch")
			} else {
				assert.Empty(t, recorder.Body.String(), "response body should be empty")
			}

			if tc.expectErrorReturn {
				assert.Error(t, err, "expected an error to be returned for logging")
			} else {
				assert.NoError(t, err, "did not expect an error to be returned")
			}
		})
	}
}

func TestServeCache(t *testing.T) {
	var backendHits int64

	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&backendHits, 1)
		w.Header().Set("X-Backend-Time", time.Now().String())
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("live data"))
	}))
	defer backendServer.Close()
	backendURL, _ := url.Parse(backendServer.URL)

	upstream := &core.InternalUpstream{
		Name:      "cached-upstream",
		Algorithm: balancer.AlgorithmRoundRobin,
		Servers:   []string{backendURL.Host},
	}
	location := &core.InternalLocation{
		NormalizedPath:      "/cached/",
		IsProxyPassUpstream: true,
		ProxyPass:           &url.URL{Scheme: "http", Host: "cached-upstream"},
		Upstream:            upstream,
		Timeout:             1 * time.Second,
		Cache: &core.InternalCacheConfig{
			TTL:  100 * time.Millisecond,
			Size: 10,
			Key:  "${uri}",
		},
	}
	internalCfg := &core.InternalConfig{
		Locations: []*core.InternalLocation{location},
		Upstreams: map[string]*core.InternalUpstream{"cached-upstream": upstream},
	}

	runtimeState := runtime.NewRuntimeState(internalCfg)
	blc := balancer.NewVortexBalancer(runtimeState)
	vtx := NewVortex(internalCfg, runtimeState, blc, zerolog.Nop())

	t.Run("FirstRequest_CacheMISS", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest("GET", "/cached/resource1", nil)

		err := vtx.serveCache(location, recorder, request)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "live data", recorder.Body.String())
		assert.Equal(t, "MISS", recorder.Header().Get("X-Vortex-Cache"))

		assert.Equal(t, int64(1), atomic.LoadInt64(&backendHits), "Backend should be hit once")
	})

	t.Run("SecondRequest_CacheHIT", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest("GET", "/cached/resource1", nil)

		err := vtx.serveCache(location, recorder, request)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "live data", recorder.Body.String())
		assert.Equal(t, "HIT", recorder.Header().Get("X-Vortex-Cache"))

		assert.Equal(t, int64(1), atomic.LoadInt64(&backendHits), "Backend should NOT be hit on second request")
	})

	t.Run("ThirdRequest_AfterTTLExpiration_CacheMISS", func(t *testing.T) {
		time.Sleep(120 * time.Millisecond)

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest("GET", "/cached/resource1", nil)

		err := vtx.serveCache(location, recorder, request)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "live data", recorder.Body.String())
		assert.Equal(t, "MISS", recorder.Header().Get("X-Vortex-Cache"))

		assert.Equal(t, int64(2), atomic.LoadInt64(&backendHits), "Backend should be hit again after TTL expires")
	})

	t.Run("FourthRequest_DifferentURL_CacheMISS", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest("GET", "/cached/resource2", nil)

		err := vtx.serveCache(location, recorder, request)
		require.NoError(t, err)

		assert.Equal(t, "MISS", recorder.Header().Get("X-Vortex-Cache"))
		assert.Equal(t, int64(3), atomic.LoadInt64(&backendHits), "Backend should be hit for a different resource")
	})
}

func TestVortex_ServeRequest_Integration(t *testing.T) {
	var backendHits atomic.Int64
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendHits.Add(1)
		w.Header().Set("X-Backend-Path", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response from backend"))
	}))
	defer backendServer.Close()

	backendURL, err := url.Parse(backendServer.URL)
	require.NoError(t, err)

	staticDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("static index page"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(staticDir, "style.css"), []byte("body { color: red; }"), 0644))

	appUpstream := &core.InternalUpstream{
		Name:      "my_app",
		Algorithm: "round_robin",
		Servers:   []string{backendURL.Host},
	}

	internalCfg := &core.InternalConfig{
		Upstreams: map[string]*core.InternalUpstream{
			"my_app": appUpstream,
		},
		Locations: []*core.InternalLocation{
			{
				NormalizedPath:      "/api/",
				NormalizedPathLen:   len("/api/"),
				IsProxyPassUpstream: true,
				Upstream:            appUpstream,
				ProxyPass:           &url.URL{Scheme: "http", Host: "my_app", Path: "/"},
				Timeout:             1 * time.Second,
			},
			{
				NormalizedPath:    "/static/",
				NormalizedPathLen: len("/static/"),
				Root:              staticDir,
			},
			{
				NormalizedPath:      "/limited/",
				NormalizedPathLen:   len("/limited/"),
				IsProxyPassUpstream: true,
				Upstream:            appUpstream,
				ProxyPass:           &url.URL{Scheme: "http", Host: "my_app", Path: "/"},
				Timeout:             1 * time.Second,
				RateLimit:           &core.InternalRateLimitConfig{Size: 10, Limit: 2, Window: time.Minute, Key: "${remote_addr}"},
			},
			{
				NormalizedPath:      "/cached/",
				NormalizedPathLen:   len("/cached/"),
				IsProxyPassUpstream: true,
				Upstream:            appUpstream,
				ProxyPass:           &url.URL{Scheme: "http", Host: "my_app", Path: "/"},
				Timeout:             1 * time.Second,
				Cache:               &core.InternalCacheConfig{Size: 10, TTL: time.Minute, Key: "${uri}"},
			},
		},
	}

	runtimeState := runtime.NewRuntimeState(internalCfg)
	balancer := balancer.NewVortexBalancer(runtimeState)
	vtx := NewVortex(internalCfg, runtimeState, balancer, zerolog.Nop())

	t.Run("ProxyRequest_HappyPath", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/users", nil)
		recorder := httptest.NewRecorder()
		vtx.ServeRequest(recorder, req)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "response from backend", recorder.Body.String())
		assert.Equal(t, "/users/", recorder.Header().Get("X-Backend-Path"), "Path should be correctly passed to backend")
	})

	t.Run("StaticRequest_DirectoryIndex", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/static/", nil)
		recorder := httptest.NewRecorder()
		vtx.ServeRequest(recorder, req)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "static index page", recorder.Body.String())
	})

	t.Run("StaticRequest_SpecificFile", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/static/style.css", nil)
		recorder := httptest.NewRecorder()
		vtx.ServeRequest(recorder, req)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "body { color: red; }", recorder.Body.String())
	})

	t.Run("StaticRequest_IndexHtmlRedirect", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/static/index.html", nil)
		recorder := httptest.NewRecorder()
		vtx.ServeRequest(recorder, req)

		assert.Equal(t, http.StatusMovedPermanently, recorder.Code)
		assert.Equal(t, "./", recorder.Header().Get("Location"))
	})

	t.Run("LocationNotFound", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/unmatched/path", nil)
		recorder := httptest.NewRecorder()
		vtx.ServeRequest(recorder, req)

		assert.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("RateLimiter_BlocksRequest", func(t *testing.T) {
		req1 := httptest.NewRequest("GET", "/limited/data", nil)
		req1.RemoteAddr = "1.2.3.4:1234"
		rec1 := httptest.NewRecorder()
		vtx.ServeRequest(rec1, req1)
		assert.Equal(t, http.StatusOK, rec1.Code)

		req2 := httptest.NewRequest("GET", "/limited/data", nil)
		req2.RemoteAddr = "1.2.3.4:1234"
		rec2 := httptest.NewRecorder()
		vtx.ServeRequest(rec2, req2)
		assert.Equal(t, http.StatusOK, rec2.Code)

		req3 := httptest.NewRequest("GET", "/limited/data", nil)
		req3.RemoteAddr = "1.2.3.4:1234"
		rec3 := httptest.NewRecorder()
		vtx.ServeRequest(rec3, req3)
		assert.Equal(t, http.StatusTooManyRequests, rec3.Code)
	})

	t.Run("Cache_HitAndMiss", func(t *testing.T) {
		backendHits.Store(0)

		req1 := httptest.NewRequest("GET", "/cached/resource", nil)
		rec1 := httptest.NewRecorder()
		vtx.ServeRequest(rec1, req1)
		assert.Equal(t, http.StatusOK, rec1.Code)
		assert.Equal(t, "MISS", rec1.Header().Get("X-Vortex-Cache"))
		assert.Equal(t, int64(1), backendHits.Load(), "Backend should be hit on first request")

		req2 := httptest.NewRequest("GET", "/cached/resource", nil)
		rec2 := httptest.NewRecorder()
		vtx.ServeRequest(rec2, req2)
		assert.Equal(t, http.StatusOK, rec2.Code)
		assert.Equal(t, "HIT", rec2.Header().Get("X-Vortex-Cache"))
		assert.Equal(t, int64(1), backendHits.Load(), "Backend should NOT be hit on second request")
	})
}
