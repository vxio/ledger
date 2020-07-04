package discovery

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"
	"github.com/travisjeffery/go-dynaport"
)

func TestMembership(t *testing.T) {
	n := 3

	members, h := setupMembers(t, n)

	require.Eventually(t, func() bool {
		return (n-1) == len(h.joins) &&
			// all members get notified of all other members
			n == len(members[0].Members()) &&
			n == len(members[1].Members()) &&
			n == len(members[2].Members()) &&
			0 == len(h.leaves)
	}, 3*time.Second, 250*time.Millisecond)

	// set last member to leave the cluster
	nodeLeaveIdx := n - 1
	require.NoError(t, members[nodeLeaveIdx].Leave())
	require.Eventually(t, func() bool {
		return (n-1) == len(h.joins) &&
			n == len(members[0].Members()) &&
			serf.StatusLeft == members[0].Members()[nodeLeaveIdx].Status &&
			1 == len(h.leaves)
	}, 3*time.Second, 250*time.Millisecond)

	require.Equal(t,
		fmt.Sprintf("%d", nodeLeaveIdx), // member's id
		<-h.leaves)
}

func setupMembers(t *testing.T, numMembers int) ([]*Membership, *handler) {
	ports := dynaport.Get(numMembers)

	var members []*Membership
	var leaderHandler *handler
	for i := 0; i < numMembers; i++ {
		addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("127.0.0.1:%d", ports[i]))
		require.NoError(t, err)

		tags := map[string]string{
			"rpc_addr": addr.String(),
		}
		c := Config{
			NodeName: fmt.Sprintf("%d", i),
			BindAddr: addr,
			Tags:     tags,
		}

		// node at 0th index is the leader
		h := &handler{}
		if i != 0 {
			c.StartJoinAddrs = []string{members[0].Config.BindAddr.String()}
		} else {
			// only the leader uses the handler to tracks joins and leaves
			h = &handler{
				joins:  make(chan map[string]string, numMembers),
				leaves: make(chan string, numMembers),
			}
			leaderHandler = h
		}

		membership, err := New(h, c)
		require.NoError(t, err)
		members = append(members, membership)
	}

	return members, leaderHandler
}

// handler mock
type handler struct {
	joins  chan map[string]string
	leaves chan string
}

func (this *handler) Join(id, addr string) error {
	if this.joins != nil {
		this.joins <- map[string]string{"id": id, "addr": addr}
	}
	return nil
}

func (this *handler) Leave(id, addr string) error {
	if this.leaves != nil {
		this.leaves <- id
	}
	return nil
}
