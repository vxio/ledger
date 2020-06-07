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

	"proglog/internal/auth"
	"proglog/internal/discovery"
	"proglog/internal/log"
	"proglog/internal/server"
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
	var err error

	// launch the server
	go a.serve()

	return a, err
}

func (this *Agent) serve() error {
	err := this.mux.Serve()
	if err != nil {
		_ = this.Shutdown()
		return err
	}

	return nil
}

func (this *Agent) setupLog() error {
	raftLn := this.mux.Match(func(reader io.Reader) bool {
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
		this.Config.ServerTLSConfig,
		this.Config.PeerTLSConfig,
	)
	logConfig.Raft.LocalID = raft.ServerID(this.Config.NodeName)
	logConfig.Raft.Bootstrap = this.Config.Bootstrap

	var err error
	this.log, err = log.NewDistributedLog(
		this.Config.DataDir,
		logConfig,
	)
	if err != nil {
		return err
	}

	if this.Config.Bootstrap {
		return this.log.WaitForLeader(3 * time.Second)
	}

	return nil
}

func (this *Agent) setupServer() error {
	serverConfig := &server.Config{
		CommitLog: this.log,
		Authorizer: auth.New(
			this.Config.ACLModelFile,
			this.Config.ACLPolicyFile,
		),
		GetSeverer: this.log,
	}

	var opts []grpc.ServerOption
	if this.Config.ServerTLSConfig != nil {
		creds := credentials.NewTLS(this.Config.ServerTLSConfig)
		opts = append(opts, grpc.Creds(creds))
	}

	var err error
	this.server, err = server.NewGRPCServer(serverConfig, opts...)
	if err != nil {
		return err
	}

	// we've mutiplexed two connection types (Raft and gRPC)
	// we've already added a matcher for the Raft connections
	// so we know all other connections are gRPC
	// thus, we can use the Any matcher to get the right listener
	grpcLn := this.mux.Match(cmux.Any())
	go func() {
		err := this.server.Serve(grpcLn)
		if err != nil {
			_ = this.Shutdown()
		}
	}()

	return nil
}

func (this *Agent) setupMembership() error {
	var err error
	this.membership, err = discovery.New(this.log, discovery.Config{
		NodeName: this.Config.NodeName,
		BindAddr: this.Config.BindAddr,
		Tags: map[string]string{
			"rpc_addr": this.Config.RPCAddr(),
		},
		StartJoinAddrs: this.Config.StartJoinAddrs,
	})

	return err
}

type Agent struct {
	Config

	mux        cmux.CMux
	log        *log.DistributedLog
	server     *grpc.Server
	membership *discovery.Membership

	shutdown  bool
	shutdowns chan struct {
	}
	shutdownLock sync.Mutex
}

type Config struct {
	ServerTLSConfig *tls.Config
	PeerTLSConfig   *tls.Config
	DataDir         string
	// BindAddr.IP is base address for both RPC and Serf
	// BindAddr.Port is used by Serf
	BindAddr *net.TCPAddr
	// RPCPort used for our server address
	RPCPort        int
	NodeName       string
	StartJoinAddrs []string
	ACLModelFile   string
	ACLPolicyFile  string
	Bootstrap      bool
}

func (this *Config) RPCAddr() string {
	return fmt.Sprintf("%s:%d", this.BindAddr.IP.String(), this.RPCPort)
}

func (this *Agent) Shutdown() error {
	// ensures that Shutdown is only called once even if users call Shutdown() multiple times
	this.shutdownLock.Lock()
	defer this.shutdownLock.Unlock()

	if this.shutdown {
		return nil
	}

	this.shutdown = true
	close(this.shutdowns) // todo: currently not used

	serverCloseFn := func() error {
		this.server.GracefulStop()
		return nil
	}
	shutdown := []func() error{
		this.membership.Leave,
		serverCloseFn,
		this.log.Close,
	}
	for _, fn := range shutdown {
		err := fn()
		if err != nil {
			return err
		}
	}

	return nil
}

func (this *Agent) setupMux() error {
	// creates a listener on our RPC address that'll accept both Raft and gRPC connections
	// todo: use helper method
	rpcAddr := fmt.Sprintf(
		"%s:%d",
		this.Config.BindAddr.IP.String(),
		this.Config.RPCPort,
	)
	ln, err := net.Listen("tcp", rpcAddr)
	if err != nil {
		return err
	}

	this.mux = cmux.New(ln)
	return nil
}
