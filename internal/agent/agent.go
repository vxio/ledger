package agent

import (
	"crypto/tls"
	"fmt"
	"net"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	api "proglog/api/v1"
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

	return a, nil
}

func (this *Agent) setupLog() error {
	var err error
	this.log, err = log.NewLog(
		this.Config.DataDir,
		log.Config{}, // using defaults here
	)
	return err
}

func (this *Agent) setupServer() error {
	serverConfig := &server.Config{
		CommitLog: this.log,
		Authorizer: auth.New(
			this.Config.ACLModelFile,
			this.Config.ACLPolicyFile,
		),
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

	ln, err := net.Listen("tcp", this.RPCAddr())
	if err != nil {
		return err
	}

	// launch the server
	go func() {
		err := this.server.Serve(ln)
		if err != nil {
			_ = this.Shutdown()
		}
	}()

	return nil
}

func (this *Agent) setupMembership() error {
	var err error
	var opts []grpc.DialOption
	if this.Config.PeerTLSConfig != nil {
		opts = append(opts,
			grpc.WithTransportCredentials(
				credentials.NewTLS(this.PeerTLSConfig),
			),
		)
	}
	conn, err := grpc.Dial(this.Config.RPCAddr(), opts...)
	if err != nil {
		return err
	}

	client := api.NewLogClient(conn)
	this.replicator = &log.Replicator{
		DialOptions: opts,
		LocalServer: client,
	}

	this.membership, err = discovery.New(this.replicator, discovery.Config{
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

	log        *log.Log
	server     *grpc.Server
	membership *discovery.Membership
	replicator *log.Replicator

	shutdown     bool
	shutdowns    chan struct{}
	shutdownLock sync.Mutex
}

type Config struct {
	ServerTLSConfig *tls.Config
	PeerTLSConfig   *tls.Config
	DataDir         string
	BindAddr        *net.TCPAddr
	RPCPort         int
	NodeName        string
	StartJoinAddrs  []string
	ACLModelFile    string
	ACLPolicyFile   string
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
		this.replicator.Close,
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
