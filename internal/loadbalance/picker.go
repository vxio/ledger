package loadbalance

import (
	"strings"
	"sync"
	"sync/atomic"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
)

// todo: remove init() and call externally
func init() {
	balancer.Register(
		base.NewBalancerBuilderV2(Name, &Picker{}, base.Config{}),
	)
}

var _ base.V2PickerBuilder = (*Picker)(nil)

type Picker struct {
	mu        sync.RWMutex
	leader    balancer.SubConn
	followers []balancer.SubConn
	current   uint64 // index for which follower to pick from
}

func (p *Picker) Build(buildInfo base.PickerBuildInfo) balancer.V2Picker {
	p.mu.Lock()
	defer p.mu.Unlock()
	var followers []balancer.SubConn
	for conn, info := range buildInfo.ReadySCs {
		isLeader := info.Address.Attributes.Value("is_leader").(bool)
		if isLeader {
			p.leader = conn
			continue
		}
		followers = append(p.followers, conn)
	}

	p.followers = followers
	return p
}

// picks the server given the action
func (p *Picker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var result balancer.PickResult

	// inspect the RPC's method name to know whether the call is an produce or consume
	// call
	if strings.Contains(info.FullMethodName, "Produce") ||
		len(p.followers) == 0 {
		result.SubConn = p.leader
	} else if strings.Contains(info.FullMethodName, "Consume") {
		result.SubConn = p.nextFollower()
	}
	return result, nil
}

// round-robin load-balancing method of choosing followers
func (p *Picker) nextFollower() balancer.SubConn {
	cur := atomic.AddUint64(&p.current, uint64(1))
	len := uint64(len(p.followers))
	idx := int(cur % len)
	return p.followers[idx]
}
