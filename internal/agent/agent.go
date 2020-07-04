package agent

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/hashicorp/raft"
	"github.com/soheilhy/cmux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"ledger/internal/auth"
	"ledger/internal/discovery"
	"ledger/internal/log"
	"ledger/internal/server"
)

func New(config Config) (*Agent, error) {
	a := &Agent{
		Config:    config,
		shutdowns: make(chan struct{}),
	}
	setup := []func() error{
		// order matters here
		a.setupMux,
		a.setupLog,
		a.setupServer,
		a.setupMembership,
	}
	for _, fn := range setup {
		err := fn()
		if err != nil {
			return nil, err
		}
	}

	// launch the server
	go a.serve()

	return a, nil
}

type Agent struct {
	Config Config

	// multiplexer to service different services on the same port
	// e.g. on the same port we can serve our log server with our Raft servers
	mux cmux.CMux
	// distributed log service
	log *log.DistributedLog
	// server for our log service that clients can make requests to
	server *grpc.Server
	// service discovery
	membership *discovery.Membership

	// indicates that this agent has already shutdown
	shutdown bool
	// stream used to signal a shutdown
	shutdowns    chan struct{}
	shutdownLock sync.Mutex
}

func (a *Agent) serve() error {
	err := a.mux.Serve()
	if err != nil {
		_ = a.Shutdown()
		return err
	}

	return nil
}

func (a *Agent) setupLog() error {
	raftLn := a.mux.Match(func(reader io.Reader) bool {
		// read one byte to identify the raft connection
		b := make([]byte, 1)
		_, err := reader.Read(b)
		if err != nil {
			return false
		}
		return bytes.Compare(
			b,
			[]byte{byte(log.RaftRPC)},
		) == 0
	})

	logConfig := log.Config{}
	logConfig.Raft.StreamLayer = log.NewStreamLayer(
		raftLn,
		a.Config.ServerTLSConfig,
		a.Config.PeerTLSConfig,
	)
	logConfig.Raft.LocalID = raft.ServerID(a.Config.NodeName)
	logConfig.Raft.Bootstrap = a.Config.Bootstrap

	var err error
	a.log, err = log.NewDistributedLog(
		a.Config.DataDir,
		logConfig,
	)
	if err != nil {
		return err
	}

	if a.Config.Bootstrap {
		return a.log.WaitForLeader(3 * time.Second)
	}

	return nil
}

func (a *Agent) setupServer() error {
	serverConfig := &server.Config{
		CommitLog: a.log,
		Authorizer: auth.New(
			a.Config.ACLModelFile,
			a.Config.ACLPolicyFile,
		),
		ServerGetter: a.log,
	}

	var opts []grpc.ServerOption
	if a.Config.ServerTLSConfig != nil {
		creds := credentials.NewTLS(a.Config.ServerTLSConfig)
		opts = append(opts, grpc.Creds(creds))
	}

	var err error
	a.server, err = server.NewGRPCServer(serverConfig, opts...)
	if err != nil {
		return err
	}

	// we've mutiplexed two connection types (Raft and gRPC)
	// we've already added a matcher for the Raft connections
	// so we know all other connections are gRPC
	// thus, we can use the Any matcher to get the right listener
	grpcLn := a.mux.Match(cmux.Any())
	go func() {
		err := a.server.Serve(grpcLn)
		if err != nil {
			_ = a.Shutdown()
		}
	}()

	return nil
}

func (a *Agent) setupMembership() error {
	var err error
	a.membership, err = discovery.New(a.log, discovery.Config{
		NodeName: a.Config.NodeName,
		BindAddr: a.Config.BindAddr,
		Tags: map[string]string{
			"rpc_addr": a.Config.RPCAddr(),
		},
		StartJoinAddrs: a.Config.StartJoinAddrs,
	})

	return err
}

type Config struct {
	ServerTLSConfig *tls.Config
	// the createClient's tls config
	PeerTLSConfig *tls.Config
	// directory that will store our logs
	DataDir string
	// BindAddr's
	// - IP is the base address for both RPC and Serf
	// - Port is used by Serf
	BindAddr *net.TCPAddr
	// RPCPort used for our server address
	RPCPort int
	// the node's name in the cluster
	NodeName string
	// If you want to add a new node to an existing cluster,
	// we must point the new node to at least one of the nodes in the cluster
	// When it connects to one of the nodes, it'll learn about the other nodes thanks to Serf
	// StartJoinAddrs indicates the addresses of member nodes in the cluster
	// In a production, specify atleast 3 address to avoid 1-2 node failures
	StartJoinAddrs []string
	// authorization config files
	ACLModelFile  string
	ACLPolicyFile string
	// Indicate this server to bootstrap the cluster
	// Should be set to true when starting the first node of the cluster to elect it as the leader
	Bootstrap bool
}

// Returns the full gRPC address, e.g. "127.0.0.1:8080"
func (this *Config) RPCAddr() string {
	return fmt.Sprintf("%s:%d", this.BindAddr.IP.String(), this.RPCPort)
}

func (a *Agent) Shutdown() error {
	// ensures that Shutdown is only called once even if users call Shutdown() multiple times
	a.shutdownLock.Lock()
	defer a.shutdownLock.Unlock()

	if a.shutdown {
		return nil
	}

	a.shutdown = true
	close(a.shutdowns) // todo: currently not used

	serverCloseFn := func() error {
		a.server.GracefulStop()
		return nil
	}
	shutdown := []func() error{
		a.membership.Leave,
		serverCloseFn,
		a.log.Close,
	}
	for _, fn := range shutdown {
		err := fn()
		if err != nil {
			return err
		}
	}

	return nil
}

// Setup our multiplexer to accept connections
func (a *Agent) setupMux() error {
	// creates a listener on our RPC address that'll accept both Raft and gRPC connections
	rpcAddr := a.Config.RPCAddr()

	ln, err := net.Listen("tcp", rpcAddr)
	if err != nil {
		return err
	}

	a.mux = cmux.New(ln)
	return nil
}
