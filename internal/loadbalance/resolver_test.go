package loadbalance_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"

	api "proglog/api/v1"
	"proglog/internal/loadbalance"
	"proglog/internal/network"
	"proglog/internal/server"
	"proglog/test/testutil"
)

func TestResolver(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	tlsConfig, err := network.SetupTLSConfig(network.TLSConfig{
		CertFile:      testutil.ServerCertFile,
		KeyFile:       testutil.ServerKeyFile,
		CAFile:        testutil.CAFile,
		ServerAddress: l.Addr().String(),
		Server:        true,
	})
	require.NoError(t, err)

	serverCreds := credentials.NewTLS(tlsConfig)
	srv, err := server.NewGRPCServer(
		&server.Config{
			GetSeverer: &getServers{},
		},
		grpc.Creds(serverCreds,
		))

	go srv.Serve(l)

	clientConn := &clientConn{}
	tlsConfig, err = network.SetupTLSConfig(network.TLSConfig{
		CertFile:      testutil.RootClientCertFile,
		KeyFile:       testutil.RootClientKeyFile,
		CAFile:        testutil.CAFile,
		ServerAddress: "127.0.0.1",
		Server:        false,
	})
	require.NoError(t, err)

	clientCreds := credentials.NewTLS(

		tlsConfig)
	opts := resolver.BuildOptions{
		DialCreds: clientCreds,
	}

	// the resolver will call GetServers() to resolve the servers and
	// update the client connection with the servers' addresses
	r := &loadbalance.Resolver{}
	_, err = r.Build(
		resolver.Target{Endpoint: l.Addr().String()},
		clientConn,
		opts,
	)
	require.NoError(t, err)

	wantState := resolver.State{
		Addresses: []resolver.Address{
			{
				Addr:       "localhost:9001",
				Attributes: attributes.New("is_leader", true),
			},
			{
				Addr:       "localhost:9002",
				Attributes: attributes.New("is_leader", false),
			},
		},
	}
	require.Equal(t, wantState, clientConn.state)

	// reset state and check that it works again
	clientConn.state.Addresses = nil
	r.ResolveNow(resolver.ResolveNowOptions{})
	require.Equal(t, wantState, clientConn.state)
}

type getServers struct{}

func (s *getServers) GetServers() ([]*api.Server, error) {
	return []*api.Server{{
		Id:       "leader",
		RpcAddr:  "localhost:9001",
		IsLeader: true,
	}, {
		Id:      "follower",
		RpcAddr: "localhost:9002",
	}}, nil
}

type clientConn struct {
	resolver.ClientConn
	state resolver.State
}

func (c *clientConn) UpdateState(state resolver.State) {
	c.state = state
}

func (c *clientConn) ReportError(err error) {}

func (c *clientConn) NewAddress(addrs []resolver.Address) {}

func (c *clientConn) NewServiceConfig(config string) {}

func (c *clientConn) ParseServiceConfig(
	config string,
) *serviceconfig.ParseResult {
	return nil
}
