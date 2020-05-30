package discovery

import (
	"log"
	"net"

	"github.com/hashicorp/serf/serf"
)

// Membership wraps Serf to provide discovery and cluster membership to our services
type Membership struct {
	Config
	handler Handler
	serf    *serf.Serf
	// events when a node joins or leaves the cluster
	events chan serf.Event
}

func New(handler Handler, config Config) (*Membership, error) {
	c := &Membership{
		Config:  config,
		handler: handler,
	}
	err := c.setupSerf()
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (this *Membership) setupSerf() error {
	var err error
	config := serf.DefaultConfig()
	config.Init()
	config.MemberlistConfig.BindAddr = this.BindAddr.IP.String()
	config.MemberlistConfig.BindPort = this.BindAddr.Port
	this.events = make(chan serf.Event)

	config.EventCh = this.events
	config.Tags = this.Tags
	config.NodeName = this.Config.NodeName
	this.serf, err = serf.Create(config)
	if err != nil {
		return nil
	}

	go this.eventHandler()
	if this.StartJoinAddrs != nil {
		_, err = this.serf.Join(this.StartJoinAddrs, true)
		if err != nil {
			return nil
		}
	}

	return nil
}

type Config struct {
	// node's unique identifier across the Serf cluster
	NodeName string
	// Serf listens on this address and port for gossipping
	BindAddr *net.TCPAddr
	// shares these tags to other nodes in the cluster and should e used for simple data on how to handle this node
	Tags map[string]string
	// used to introduce a new node to an existing cluster
	StartJoinAddrs []string
}

type Handler interface {
	Join(name, addr string) error
	Leave(name, addr string) error
}

func (this *Membership) eventHandler() {
	for e := range this.events {
		switch e.EventType() {
		case serf.EventMemberJoin:
			for _, m := range e.(serf.MemberEvent).Members {
				if this.isLocal(m) {
					continue
				}
				this.handleJoin(m)
			}
		case serf.EventMemberLeave, serf.EventMemberFailed:
			for _, m := range e.(serf.MemberEvent).Members {
				if this.isLocal(m) {
					// we return here since if the member leaving is itself,
					// it no longer cares about the state of the cluster
					return
				}
				this.handleLeave(m)
			}
		}
	}
}

func (this *Membership) handleJoin(m serf.Member) {
	err := this.handler.Join(m.Name, m.Tags["rpc_addr"])
	if err != nil {
		log.Printf("[ERROR] proglog: failed to join: %s, %s", m.Name, m.Tags["rpc_addr"])
	}
}

func (this *Membership) handleLeave(m serf.Member) {
	err := this.handler.Leave(m.Name, m.Tags["rpc_addr"])
	if err != nil {
		log.Printf("[ERROR] proglog: failed to leave: %s, %s", m.Name, m.Tags["rpc_addr"])
	}
}

func (this *Membership) isLocal(member serf.Member) bool {
	return this.serf.LocalMember().Name == member.Name
}

func (this *Membership) Members() []serf.Member {
	return this.serf.Members()
}

func (this *Membership) Leave() error {
	return this.serf.Leave()
}
