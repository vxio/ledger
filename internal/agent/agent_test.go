package agent_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/travisjeffery/go-dynaport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	api "ledger/api/v1"
	"ledger/config"
	"ledger/internal/agent"
	"ledger/internal/loadbalance"
	"ledger/internal/network"
)

func TestAgent(t *testing.T) {
	var agents []*agent.Agent

	serverTLSConfig, err := network.SetupTLSConfig(network.TLSConfig{
		CertFile:      config.ServerCertFile,
		KeyFile:       config.ServerKeyFile,
		CAFile:        config.CAFile,
		Server:        true,
		ServerAddress: "127.0.0.1",
	})
	require.NoError(t, err)

	peerTLSConfig, err := network.SetupTLSConfig(network.TLSConfig{
		CertFile:      config.RootClientCertFile,
		KeyFile:       config.RootClientKeyFile,
		CAFile:        config.CAFile,
		Server:        false,
		ServerAddress: "127.0.0.1",
	})
	require.NoError(t, err)

	getLeader := func() *agent.Agent { return agents[0] }
	// setup agents
	for i := 0; i < 3; i++ {
		// Elect node 0 as the leader
		isLeader := i == 0

		ports := dynaport.Get(2)
		bindAddr := &net.TCPAddr{
			IP:   []byte{127, 0, 0, 1},
			Port: ports[0],
		}
		rpcPort := ports[1]

		dataDir, err := ioutil.TempDir("", "test-distributed-log")
		require.NoError(t, err)

		var startJoinAddrs []string

		// for new nodes joining an existing cluster, make sure we give them a reference to the leader in the cluster
		if !isLeader {
			startJoinAddrs = append(startJoinAddrs, getLeader().Config.BindAddr.String())
		}

		agent, err := agent.New(agent.Config{
			NodeName:        fmt.Sprintf("%d", i),
			Bootstrap:       isLeader,
			StartJoinAddrs:  startJoinAddrs,
			BindAddr:        bindAddr,
			RPCPort:         rpcPort,
			DataDir:         dataDir,
			ACLModelFile:    config.ACLModelFile,
			ACLPolicyFile:   config.ACLPolicyFile,
			ServerTLSConfig: serverTLSConfig,
			PeerTLSConfig:   peerTLSConfig,
		})
		require.NoError(t, err)

		agents = append(agents, agent)
	}
	defer func() {
		for _, agent := range agents {
			_ = agent.Shutdown()
			require.NoError(t, os.RemoveAll(agent.Config.DataDir))
		}
	}()

	// wait until agents have joined the cluster
	time.Sleep(3 * time.Second)

	// write once to the log
	logMessage := []byte("lorem ipsum")
	leaderClient := createClient(t, getLeader(), peerTLSConfig)
	produceResponse, err := leaderClient.Produce(
		context.Background(),
		&api.ProduceRequest{
			Record: &api.Record{
				Value: logMessage,
			},
		},
	)
	require.NoError(t, err)

	// wait til replication has finished
	time.Sleep(3 * time.Second)

	consumeResponse, err := leaderClient.Consume(
		context.Background(),
		&api.ConsumeRequest{
			Offset: produceResponse.Offset,
		},
	)
	require.NoError(t, err)
	require.Equal(t, consumeResponse.Record.Value, logMessage)

	followerClient := createClient(t, agents[1], peerTLSConfig)
	consumeResponse, err = followerClient.Consume(
		context.Background(),
		&api.ConsumeRequest{
			Offset: produceResponse.Offset,
		},
	)
	require.NoError(t, err)
	require.Equal(t, consumeResponse.Record.Value, logMessage)

	// expect an error when we try to consume an out of range offset
	consumeResponse, err = leaderClient.Consume(
		context.Background(),
		&api.ConsumeRequest{
			Offset: produceResponse.Offset + 1,
		},
	)
	require.Nil(t, consumeResponse)
	require.Error(t, err)
	got := status.Code(err)
	want := status.Code(api.ErrOffsetOutOfRange{})
	require.Equal(t, got, want)
}

func createClient(t *testing.T, agent *agent.Agent, tlsConfig *tls.Config) api.LogClient {
	tlsCreds := credentials.NewTLS(tlsConfig)
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(tlsCreds),
	}
	conn, err := grpc.Dial(fmt.Sprintf(
		"%s:///%s:%d",
		loadbalance.Name,
		agent.Config.BindAddr.IP.String(),
		agent.Config.RPCPort,
	), opts...)

	require.NoError(t, err)
	client := api.NewLogClient(conn)
	return client
}
