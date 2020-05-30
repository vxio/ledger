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
	m, handler := setupMember(t, nil)
	m, _ = setupMember(t, m)
	m, _ = setupMember(t, m)

	require.Eventually(t, func() bool {
		return 2 == len(handler.joins) &&
			3 == len(m[0].Members()) &&
			3 == len(m[1].Members()) &&
			3 == len(m[2].Members()) &&
			0 == len(handler.leaves)
	}, 3*time.Second, 250*time.Millisecond)

	require.NoError(t, m[2].Leave())
	require.Eventually(t, func() bool {
		return 2 == len(handler.joins) &&
			3 == len(m[0].Members()) &&
			serf.StatusLeft == m[0].Members()[2].Status &&
			1 == len(handler.leaves)
	}, 3*time.Second, 250*time.Millisecond)

	require.Equal(t, fmt.Sprintf("%d", 2), <-handler.leaves)
}

func setupMember(t *testing.T, members []*Membership) ([]*Membership, *handler) {
	id := len(members)
	ports := dynaport.Get(1)
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("127.0.0.1:%d", ports[0]))
	require.NoError(t, err)

	tags := map[string]string{
		"rpc_addr": addr.String(),
	}
	c := Config{
		NodeName: fmt.Sprintf("%d", id),
		BindAddr: addr,
		Tags:     tags,
	}

	h := &handler{}
	if len(members) == 0 {
		h.joins = make(chan map[string]string, 3)
		h.leaves = make(chan string, 3)
	} else {
		// bind nodes [2,3] to the node [1]
		c.StartJoinAddrs = []string{
			members[0].BindAddr.String(),
		}
	}

	m, err := New(h, c)
	require.NoError(t, err)
	members = append(members, m)
	return members, h
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
