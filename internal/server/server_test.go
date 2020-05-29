package server

import (
	"context"
	"io/ioutil"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

	api "proglog/api/v1"
	"proglog/internal/log"
)

func TestServer(t *testing.T) {
	cases := map[string]func(
		t *testing.T,
		client api.LogClient,
		config *Config,
	){
		"success: produce/consume a message to/from the log": testProduceConsume,
		"success: produce/consume stream":                    testProduceConsumeStream,
		"fail: consume past log boundary":                    testConsumePastBoundary,
	}
	for description, fn := range cases {
		t.Run(description, func(t *testing.T) {
			client, config, teardown := testSetup(t, nil)
			defer teardown()
			fn(t, client, config)
		})
	}
}

func testConsumePastBoundary(t *testing.T, client api.LogClient, config *Config) {
	ctx := context.Background()
	record := &api.Record{
		Value: []byte("hello"),
	}

	produce, err := client.Produce(ctx, &api.ProduceRequest{
		Record: record,
	})
	require.NoError(t, err)

	consume, err := client.Consume(ctx, &api.ConsumeRequest{
		Offset: produce.Offset + 1,
	})
	require.Nil(t, consume)
	got := status.Code(err)
	want := status.Code(api.ErrOffsetOutOfRange{}.GRPCStatus().Err())
	require.Equal(t, want, got)
}

func testProduceConsumeStream(t *testing.T, client api.LogClient, config *Config) {
	ctx := context.Background()
	records := []*api.Record{
		{Value: []byte("first msg")},
		{Value: []byte("second msg")},
	}
	{ // append records
		stream, err := client.ProduceStream(ctx)
		require.NoError(t, err)

		// send records to the stream and see that what gets received matches
		for offset, record := range records {
			err = stream.Send(&api.ProduceRequest{Record: record})
			require.NoError(t, err)

			res, err := stream.Recv()
			require.NoError(t, err)
			require.EqualValues(t, offset, res.Offset)
		}
	}

	{
		stream, err := client.ConsumeStream(ctx, &api.ConsumeRequest{Offset: 0})
		require.NoError(t, err)

		for _, record := range records {
			res, err := stream.Recv()
			require.NoError(t, err)
			require.Equal(t, res.Record, record)
		}
	}

}

func testProduceConsume(t *testing.T, client api.LogClient, config *Config) {
	ctx := context.Background()
	record := &api.Record{
		Value: []byte("hello"),
	}

	produce, err := client.Produce(ctx, &api.ProduceRequest{
		Record: record,
	})
	require.NoError(t, err)

	offset := produce.Offset
	consume, err := client.Consume(ctx, &api.ConsumeRequest{Offset: offset})
	require.NoError(t, err)

	require.Equal(t, record, consume.Record)
}

func testSetup(t *testing.T, fn func(*Config)) (client api.LogClient, config *Config, teardown func()) {
	t.Helper()

	l, err := net.Listen("tcp", ":0") // port 0 assigns us an unused port
	require.NoError(t, err)

	dir, err := ioutil.TempDir("", "server-test")
	require.NoError(t, err)

	clog, err := log.NewLog(dir, log.Config{})
	require.NoError(t, err)

	config = &Config{
		CommitLog: clog,
	}
	if fn != nil {
		fn(config)
	}
	server, err := NewGRPCServer(config)
	require.NoError(t, err)

	go func() {
		server.Serve(l)
	}()

	clientOptions := []grpc.DialOption{grpc.WithInsecure()}
	clientConnection, err := grpc.Dial(l.Addr().String(), clientOptions...)
	require.NoError(t, err)
	client = api.NewLogClient(clientConnection)

	teardown = func() {
		server.Stop()
		clientConnection.Close()
		l.Close()
	}

	return client, config, teardown
}
