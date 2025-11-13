package balancer

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"vortex/internal/core"
	"vortex/internal/runtime"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVortexBalancer_RoundRobin(t *testing.T) {
	upstreamCfg := &core.InternalUpstream{
		Name:      "test-rr",
		Algorithm: AlgorithmRoundRobin,
		Servers:   []string{"server1", "server2", "server3"},
	}
	t.Run("StrictSequence", func(t *testing.T) {
		runtimeState := &runtime.RuntimeState{
			Upstreams: map[string]*runtime.UpstreamState{"test-rr": {}},
		}
		balancer := NewVortexBalancer(runtimeState)

		numServers := len(upstreamCfg.Servers)
		for i := 0; i < numServers*2; i++ {
			expectedServer := upstreamCfg.Servers[i%numServers]
			msg := fmt.Sprintf("Call #%d: expected %s", i+1, expectedServer)
			backend, cb := balancer.Balance(upstreamCfg)

			assert.Equal(t, expectedServer, backend, msg)
			require.NotNil(t, cb, msg)
		}
	})

	t.Run("ConcurrentCalls", func(t *testing.T) {
		runtimeState := &runtime.RuntimeState{
			Upstreams: map[string]*runtime.UpstreamState{"test-rr": {}},
		}
		balancer := NewVortexBalancer(runtimeState)

		var wg sync.WaitGroup
		numGoroutines := 100
		callsPerGoroutine := 10
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < callsPerGoroutine; j++ {
					balancer.Balance(upstreamCfg)
				}
			}()
		}
		wg.Wait()

		finalCount := runtimeState.Upstreams["test-rr"].RoundRobinCounter.Load()
		assert.Equal(t, uint64(numGoroutines*callsPerGoroutine), finalCount)
	})
}

func TestVortexBalancer_LeastConnections(t *testing.T) {
	upstreamCfg := &core.InternalUpstream{
		Name:      "test-lc",
		Algorithm: AlgorithmLeastConnections,
		Servers:   []string{"server1", "server2", "server3"},
	}

	s1Counter, s2Counter, s3Counter := &atomic.Int64{}, &atomic.Int64{}, &atomic.Int64{}
	runtimeState := &runtime.RuntimeState{
		Upstreams: map[string]*runtime.UpstreamState{
			"test-lc": {
				ServerConnections: map[string]*atomic.Int64{
					"server1": s1Counter,
					"server2": s2Counter,
					"server3": s3Counter,
				},
			},
		},
	}
	balancer := NewVortexBalancer(runtimeState)

	t.Run("SelectsLeastBusyWithThreeServers", func(t *testing.T) {
		b1, cb1 := balancer.Balance(upstreamCfg)
		assert.Equal(t, "server1", b1)
		assert.Equal(t, int64(1), s1Counter.Load(), "s1 should be 1")

		b2, cb2 := balancer.Balance(upstreamCfg)
		assert.Equal(t, "server2", b2)
		assert.Equal(t, int64(1), s2Counter.Load(), "s2 should be 1")

		b3, cb3 := balancer.Balance(upstreamCfg)
		assert.Equal(t, "server3", b3)
		assert.Equal(t, int64(1), s3Counter.Load(), "s3 should be 1")

		cb2()
		assert.Equal(t, int64(0), s2Counter.Load(), "s2 should be 0 after callback")

		b4, cb4 := balancer.Balance(upstreamCfg)
		assert.Equal(t, "server2", b4)
		assert.Equal(t, int64(1), s2Counter.Load(), "s2 should be 1 again")

		cb1()
		cb3()
		cb4()
		assert.Equal(t, int64(0), s1Counter.Load())
		assert.Equal(t, int64(0), s2Counter.Load())
		assert.Equal(t, int64(0), s3Counter.Load())
	})

	t.Run("ConcurrentLeastConnections", func(t *testing.T) {
		s1Counter.Store(0)
		s2Counter.Store(0)
		s3Counter.Store(0)

		var wg sync.WaitGroup
		numGoroutines := 300
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				_, cb := balancer.Balance(upstreamCfg)
				cb()
			}()
		}
		wg.Wait()

		assert.Equal(t, int64(0), s1Counter.Load(), "Server1 connections should be zero after all goroutines finish")
		assert.Equal(t, int64(0), s2Counter.Load(), "Server2 connections should be zero after all goroutines finish")
		assert.Equal(t, int64(0), s3Counter.Load(), "Server3 connections should be zero after all goroutines finish")
	})
}
