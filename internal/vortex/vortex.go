package vortex

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"vortex/internal/balancer"
	"vortex/internal/cache"
	"vortex/internal/core"
	"vortex/internal/metrics"
	"vortex/internal/ratelimit"
	"vortex/internal/runtime"
	"vortex/internal/utils"

	"github.com/rs/zerolog"
)

type Vortex struct {
	cfg      *core.InternalConfig
	log      zerolog.Logger
	client   *http.Client
	state    *runtime.RuntimeState
	balancer balancer.Balancer
}

func NewVortex(cfg *core.InternalConfig, state *runtime.RuntimeState, balancer balancer.Balancer, logger zerolog.Logger) *Vortex {
	vtx := &Vortex{
		cfg:      cfg,
		client:   &http.Client{},
		state:    state,
		balancer: balancer,
		log:      logger,
	}
	return vtx
}

type responseWriterInterceptor struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriterInterceptor) WriteHeader(code int) {
	if w.statusCode != 0 && w.statusCode != http.StatusOK {
		return
	}
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (vtx *Vortex) ServeRequest(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var err error
	interceptor := &responseWriterInterceptor{ResponseWriter: w, statusCode: http.StatusOK}

	targetLocation := vtx.resolveLocation(vtx.cfg, r.URL)

	defer func() {
		duration := time.Since(start)

		locationLabel := "unknown"
		if targetLocation != nil {
			locationLabel = targetLocation.NormalizedPath
		}

		metrics.HTTPRequestDuration.WithLabelValues(locationLabel).Observe(duration.Seconds())
		metrics.HTTPRequestsTotal.WithLabelValues(locationLabel, r.Method, strconv.Itoa(interceptor.statusCode)).Inc()

		if err != nil {
			vtx.log.Error().
				Err(err).
				Str("method", r.Method).
				Str("url", r.URL.String()).
				Str("remote_addr", r.RemoteAddr).
				Int("status_code", interceptor.statusCode).
				Str("location", locationLabel).
				Dur("duration", duration).
				Msg("request processed with error")
			if _, ok := interceptor.Header()["Content-Type"]; !ok {
				http.Error(interceptor, "Bad Gateway", http.StatusBadGateway)
			}
		} else {
			vtx.log.Info().
				Str("method", r.Method).
				Str("url", r.URL.String()).
				Str("remote_addr", r.RemoteAddr).
				Int("status_code", interceptor.statusCode).
				Str("location", locationLabel).
				Dur("duration", duration).
				Msg("request processed succesfully")
		}
	}()

	if targetLocation == nil {
		err = fmt.Errorf("location not found")
		interceptor.WriteHeader(http.StatusNotFound)
		return
	}

	if targetLocation.RateLimit != nil {
		limiter := vtx.state.RateLimiters[targetLocation.NormalizedPath]
		limiterKey := ratelimit.BuildKey(targetLocation.RateLimit.Key, r)

		if !limiter.Allow(limiterKey) {
			metrics.RateLimitRequestsLimitedTotal.WithLabelValues(targetLocation.NormalizedPath).Inc()
			err = fmt.Errorf("rate limit exceeded")
			http.Error(interceptor, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
	}

	if targetLocation.Root != "" {
		if err = vtx.serveStatic(targetLocation, interceptor, r); err != nil {
			err = fmt.Errorf("failed to serve static request: %w", err)
		}
		return
	}
	if targetLocation.Cache != nil && r.Method == http.MethodGet {
		if err = vtx.serveCache(targetLocation, interceptor, r); err != nil {
			err = fmt.Errorf("failed to serve proxy with cache request: %w", err)
		}
		return
	}
	if err = vtx.serveProxy(targetLocation, interceptor, r); err != nil {
		err = fmt.Errorf("failed to serve proxy request: %w", err)
	}
}

func (vtx *Vortex) resolveLocation(cfg *core.InternalConfig, requestURL *url.URL) *core.InternalLocation {
	maxLength := -1
	var resLocation *core.InternalLocation
	requestPath := utils.NormalizeRequestPath(requestURL.Path)
	for _, location := range cfg.Locations {
		if strings.HasPrefix(requestPath, location.NormalizedPath) {
			if location.NormalizedPathLen > maxLength {
				maxLength = location.NormalizedPathLen
				resLocation = location
			}
		}
	}

	return resLocation
}

func (vtx *Vortex) serveCache(location *core.InternalLocation, w http.ResponseWriter, clientReq *http.Request) error {
	locationCache, found := vtx.state.Caches[location.NormalizedPath]
	if !found {
		vtx.log.Error().Str("location", location.NormalizedPath).Msg("cache not found for location")
		return vtx.serveProxy(location, w, clientReq)
	}
	cacheKey := cache.BuildCacheKey(location.Cache.Key, clientReq)

	if cachedResp, ok := locationCache.Get(cacheKey); ok {
		vtx.log.Debug().Str("key", cacheKey).Msg("cache hit")
		for key, values := range cachedResp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.Header().Set("X-Vortex-Cache", "HIT")
		w.WriteHeader(cachedResp.StatusCode)
		w.Write(cachedResp.Body)
		return nil
	}

	vtx.log.Debug().Str("key", cacheKey).Msg("cache miss")
	recorder := httptest.NewRecorder()
	err := vtx.serveProxy(location, recorder, clientReq)
	if err != nil {
		return err
	}

	result := recorder.Result()
	bodyBytes, _ := io.ReadAll(result.Body)
	result.Body.Close()

	if result.StatusCode >= 200 && result.StatusCode < 400 {
		locationCache.Add(cacheKey, &cache.CachedResponse{
			StatusCode: result.StatusCode,
			Header:     result.Header.Clone(),
			Body:       bodyBytes,
		})
	}

	for key, values := range result.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.Header().Set("X-Vortex-Cache", "MISS")
	w.WriteHeader(recorder.Code)
	w.Write(bodyBytes)
	return nil
}

func (vtx *Vortex) serveProxy(location *core.InternalLocation, w http.ResponseWriter, clientReq *http.Request) error {
	backendURL := location.ProxyPass
	if location.IsProxyPassUpstream {
		backend, callback := vtx.balancer.Balance(location.Upstream)
		if callback != nil {
			defer callback()
		}
		if backend == "" {
			return fmt.Errorf("balancer returned no available server for upstream '%s'", location.Upstream.Name)
		}

		metrics.UpstreamRequestsTotal.WithLabelValues(location.Upstream.Name, backend).Inc()

		backendURL = &url.URL{
			Scheme: location.ProxyPass.Scheme,
			Host:   backend,
			Path:   location.ProxyPass.Path,
		}
	}
	targetURL, err := vtx.resolveProxyURL(location, clientReq.URL, backendURL)
	if err != nil {
		return fmt.Errorf("failed to resolve target URL: %w", err)
	}

	resp, err := vtx.proxyRequest(targetURL, location.Timeout, clientReq)
	if err != nil {
		return fmt.Errorf("failed proxy request to %q: %w", targetURL, err)
	}
	defer resp.Body.Close()

	if err := vtx.proxyResponse(w, resp); err != nil {
		vtx.log.Warn().
			Err(err).
			Str("method", clientReq.Method).
			Str("path", clientReq.URL.Path).
			Msg("error after response headers were sent")
	}
	return nil
}

func (vtx *Vortex) proxyRequest(requestURL string, requestTimeout time.Duration, clientReq *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(clientReq.Context(), requestTimeout)
	defer cancel()

	req, err := vtx.buildRequest(ctx, requestURL, clientReq)
	if err != nil {
		return nil, fmt.Errorf("unable to proxy request: %w", err)
	}

	resp, err := vtx.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to proxy request %w", err)
	}
	return resp, nil
}

// TODO: отдавать 504 при timeout

func (vtx *Vortex) proxyResponse(w http.ResponseWriter, resp *http.Response) error {
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	if headers, ok := w.Header()["Connection"]; ok {
		for _, header := range headers {
			for _, headerName := range strings.Split(header, ",") {
				headerName = strings.TrimSpace(headerName)
				w.Header().Del(headerName)
			}
		}
	}

	for _, header := range HopByHopHeaders {
		w.Header().Del(header)
	}

	w.Header().Set("Server", "vortex")

	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("failed to copy response body to client: %w", err)
	}

	return nil
}

var HopByHopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"TE",
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

func (vtx *Vortex) buildRequest(ctx context.Context, requestURL string, clientReq *http.Request) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, clientReq.Method, requestURL, clientReq.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to build proxy request: %w", err)
	}

	// копирование оригинальных заголовков
	req.Header = clientReq.Header.Clone()

	// удаление заголовков Connection
	if headers, ok := req.Header["Connection"]; ok {
		for _, header := range headers {
			for _, headerName := range strings.Split(header, ",") {
				headerName = strings.TrimSpace(headerName)
				req.Header.Del(headerName)
			}
		}
	}

	// удаление hop-by-hop заголовков
	for _, header := range HopByHopHeaders {
		req.Header.Del(header)
	}

	// установка заголовка Host. Go делает это автоматически, однако для читаемости сделал явно
	req.Host = req.URL.Host

	// установка заголовка X-Forwarded-Host
	req.Header.Set("X-Forwarded-Host", clientReq.Host)

	// установка заголовка X-Real-IP
	clientIP := utils.GetClientIP(clientReq)
	req.Header.Set("X-Real-IP", clientIP)

	// обработка заголовка X-Forwarded-For
	existingFwdFor := clientReq.Header.Get("X-Forwarded-For")
	if existingFwdFor != "" {
		req.Header.Set("X-Forwarded-For", existingFwdFor+", "+clientIP)
	} else {
		req.Header.Set("X-Forwarded-For", clientIP)
	}

	// установка заголовка X-Forwarded-Proto
	if clientReq.TLS != nil {
		req.Header.Set("X-Forwarded-Proto", "https")
	} else {
		req.Header.Set("X-Forwarded-Proto", "http")
	}

	return req, nil
}

func (vtx *Vortex) resolveProxyURL(location *core.InternalLocation, requestURL, backendURL *url.URL) (string, error) {
	normReqPath := utils.NormalizeRequestPath(requestURL.Path)

	if backendURL.Path == "" {
		return backendURL.String() + normReqPath, nil
	}

	targetSuffix := strings.TrimPrefix(normReqPath, location.NormalizedPath)
	if strings.HasSuffix(backendURL.Path, "/") {
		return backendURL.String() + targetSuffix, nil
	}
	return backendURL.String() + "/" + targetSuffix, nil
}

func (vtx *Vortex) serveStatic(location *core.InternalLocation, w http.ResponseWriter, clientReq *http.Request) error {
	if clientReq.Method != http.MethodGet && clientReq.Method != http.MethodHead {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return fmt.Errorf("method not allowed: %s", clientReq.Method)
	}

	subPath := strings.TrimPrefix(clientReq.URL.Path, location.NormalizedPath)
	if strings.Contains(subPath, "..") {
		http.Error(w, "Not Found", http.StatusNotFound)
		return fmt.Errorf("path traversal attempt detected: %s", subPath)
	}

	fullPath := filepath.Join(location.Root, subPath)

	realPath, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, clientReq)
			return nil
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return fmt.Errorf("failed to evaluate symlinks for %s: %w", fullPath, err)
	}

	cleanRoot, _ := filepath.Abs(location.Root)
	if !strings.HasPrefix(realPath, cleanRoot) {
		http.Error(w, "Not Found", http.StatusNotFound)
		return fmt.Errorf("security violation: real path %s is outside of root %s", realPath, cleanRoot)
	}

	stat, err := os.Stat(realPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, clientReq)
			return nil
		}
		if errors.Is(err, os.ErrPermission) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return fmt.Errorf("permission denied for path: %s", realPath)
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return fmt.Errorf("failed to stat path %s: %w", realPath, err)
	}

	finalPath := realPath
	if stat.IsDir() {
		indexPath := filepath.Join(realPath, "index.html")
		indexStat, err := os.Stat(indexPath)
		if err == nil && !indexStat.IsDir() {
			finalPath = indexPath
		} else {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return fmt.Errorf("directory listing is forbidden for: %s", realPath)
		}
	}

	http.ServeFile(w, clientReq, finalPath)
	return nil
}
