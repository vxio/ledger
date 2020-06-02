package loadbalance_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/resolver"

	"proglog/internal/loadbalance"
)

func TestPickerProducesToLeader(t *testing.T) {
	picker, subConns := setupTest()
	info := balancer.PickInfo{
		FullMethodName: "/log.vX.Log/Produce",
	}

	for i := 0; i < 5; i++ {
		gotPickResult, err := picker.Pick(info)
		require.NoError(t, err)
		require.Equal(t, subConns[0], gotPickResult.SubConn)
	}
}

func TestPickerConsumesFromFollowers(t *testing.T) {
	picker, subConns := setupTest()
	info := balancer.PickInfo{
		FullMethodName: "/log.vX.Log/Consume",
	}

	for i := 0; i < 5; i++ {
		gotPickResult, err := picker.Pick(info)
		require.NoError(t, err)
		// cycle between and index 1 and 2 (follower subConns)
		// ensures we're cycling through the subConns using the
		// round-robin load-balancing strategy
		idx := i%2 + 1
		require.Equal(t, subConns[idx], gotPickResult.SubConn)
	}
}

func setupTest() (*loadbalance.Picker, []*subConn) {
	var subConns []*subConn
	buildInfo := base.PickerBuildInfo{
		ReadySCs: make(map[balancer.SubConn]base.SubConnInfo),
	}
	// create 3 subconnections with one leader
	for i := 0; i < 3; i++ {
		sc := &subConn{}
		addr := resolver.Address{
			// make 0th sub conn is the leader
			Attributes: attributes.New("is_leader", i == 0),
		}
		sc.UpdateAddresses([]resolver.Address{addr})
		buildInfo.ReadySCs[sc] = base.SubConnInfo{Address: addr}
		subConns = append(subConns, sc)
	}
	picker := &loadbalance.Picker{}
	picker.Build(buildInfo)
	return picker, subConns
}

// subConn implements balancer.SubConn
type subConn struct {
	addrs []resolver.Address
}

func (s *subConn) UpdateAddresses(addrs []resolver.Address) {
	s.addrs = addrs
}

func (s *subConn) Connect() {}
