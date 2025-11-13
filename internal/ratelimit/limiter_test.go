package ratelimit

import (
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildKey(t *testing.T) {
	req := &http.Request{
		RemoteAddr: "192.168.1.1:12345",
		Host:       "example.com",
		URL: &url.URL{
			Path: "/api/v1/users",
		},
	}

	testCases := []struct {
		name     string
		template string
		wantKey  string
	}{
		{name: "KeyByRemoteAddress", template: "${remote_addr}", wantKey: "192.168.1.1"},
		{name: "KeyByHost", template: "${host}", wantKey: "example.com"},
		{name: "KeyByURI", template: "${uri}", wantKey: "/api/v1/users"},
		{name: "CombinedKey", template: "${host}${uri}", wantKey: "example.com/api/v1/users"},
		{name: "CombinedKeyWithStaticText", template: "ratelimit:${remote_addr}:${host}", wantKey: "ratelimit:192.168.1.1:example.com"},
		{name: "StaticKey", template: "global_limit", wantKey: "global_limit"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotKey := BuildKey(tc.template, req)
			assert.Equal(t, tc.wantKey, gotKey)
		})
	}
}

func TestNewLRUFixedWindowLimiter_Error(t *testing.T) {
	t.Run("InvalidSize", func(t *testing.T) {
		_, err := NewLRUFixedWindowLimiter(0, 10, time.Minute)
		require.Error(t, err, "Should return an error for size 0")
	})
}

func TestLRUFixedWindowLimiter(t *testing.T) {
	t.Run("BasicLimitAndWindowReset", func(t *testing.T) {
		limiter, err := NewLRUFixedWindowLimiter(10, 2, 100*time.Millisecond)
		require.NoError(t, err)

		clientKey := "client-A"

		assert.True(t, limiter.Allow(clientKey), "1st call should be allowed")
		assert.True(t, limiter.Allow(clientKey), "2nd call should be allowed")
		assert.False(t, limiter.Allow(clientKey), "3rd call should be denied")
		assert.False(t, limiter.Allow(clientKey), "4th call should still be denied")

		time.Sleep(110 * time.Millisecond)

		assert.True(t, limiter.Allow(clientKey), "Call after window expiration should be allowed")
	})

	t.Run("IndependentKeys", func(t *testing.T) {
		limiter, err := NewLRUFixedWindowLimiter(10, 1, time.Minute)
		require.NoError(t, err)

		clientA := "client-A"
		clientB := "client-B"

		assert.True(t, limiter.Allow(clientA), "Client A's 1st call should be allowed")
		assert.True(t, limiter.Allow(clientB), "Client B's 1st call should be allowed")
		assert.False(t, limiter.Allow(clientA), "Client A's 2nd call should be denied")
		assert.False(t, limiter.Allow(clientB), "Client B's 2nd call should be denied")
	})

	t.Run("LRUEviction", func(t *testing.T) {
		limiter, err := NewLRUFixedWindowLimiter(2, 5, time.Minute)
		require.NoError(t, err)

		clientA := "client-A"
		clientB := "client-B"
		clientC := "client-C"

		limiter.Allow(clientA)
		limiter.Allow(clientB)
		limiter.Allow(clientC)

		assert.True(t, limiter.Allow(clientA), "Client A should be allowed after being evicted and re-added")
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		limit := int64(100)
		limiter, err := NewLRUFixedWindowLimiter(10, limit, time.Minute)
		require.NoError(t, err)

		clientKey := "concurrent-client"
		numGoroutines := 500
		var successfulHits int32

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				if limiter.Allow(clientKey) {
					atomic.AddInt32(&successfulHits, 1)
				}
			}()
		}
		wg.Wait()

		assert.Equal(t, int32(limit), atomic.LoadInt32(&successfulHits), "Number of concurrent successful hits should match the limit exactly")
	})
}
