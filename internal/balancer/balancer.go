package balancer

import (
	"vortex/internal/core"
	"vortex/internal/runtime"
)

type Balancer interface {
	Balance(upstream *core.InternalUpstream) (string, func())
}

// TODO: убрать дублирование констант
const (
	AlgorithmRoundRobin       string = "round_robin"
	AlgorithmLeastConnections string = "least_connections"
)

type BalancingAlgorithm string

type VortexBalancer struct {
	state *runtime.RuntimeState
}

func NewVortexBalancer(state *runtime.RuntimeState) *VortexBalancer {
	return &VortexBalancer{
		state: state,
	}
}

func (blc *VortexBalancer) Balance(upstream *core.InternalUpstream) (string, func()) {
	upstreamState := blc.state.Upstreams[upstream.Name]

	switch upstream.Algorithm {
	case AlgorithmRoundRobin:
		return blc.roundRobin(upstreamState, upstream), func() {}
	case AlgorithmLeastConnections:
		return blc.leastConnections(upstreamState, upstream)
	}

	return "", func() {}
}

func (blc *VortexBalancer) roundRobin(state *runtime.UpstreamState, upstream *core.InternalUpstream) string {
	cnt := state.RoundRobinCounter.Add(1)
	return upstream.Servers[(cnt-1)%uint64(len(upstream.Servers))]
}

func (blc *VortexBalancer) leastConnections(state *runtime.UpstreamState, upstream *core.InternalUpstream) (string, func()) {
	var minConns int64 = -1
	chosenServer := upstream.Servers[0]

	for _, serverAddr := range upstream.Servers {
		cnt := state.ServerConnections[serverAddr].Load()
		if minConns == -1 || cnt < minConns {
			minConns = cnt
			chosenServer = serverAddr
		}
	}

	connCounter := state.ServerConnections[chosenServer]
	connCounter.Add(1)
	return chosenServer, func() {
		connCounter.Add(-1)
	}
}
