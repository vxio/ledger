package ledger_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/travisjeffery/go-dynaport"
	"google.golang.org/grpc"

	api "proglog/api/v1"
	"proglog/internal/ledger"
	"proglog/internal/log"
	"proglog/internal/server"
)

func TestX(t *testing.T) {
	ports := dynaport.Get(2)
	ledgerServerAddr := fmt.Sprintf(":%d", ports[0])
	logServerAddr := fmt.Sprintf(":%d", ports[1])

	dir, err := ioutil.TempDir("", "app-test")
	require.NoError(t, err)

	commitLog, err := log.NewLog(dir, log.Config{})
	require.NoError(t, err)

	logServer, err := server.NewGRPCServer(&server.Config{
		CommitLog:  commitLog,
		GetSeverer: nil,
	})
	require.NoError(t, err)

	go func() {
		ln, err := net.Listen("tcp", logServerAddr)
		require.NoError(t, err)
		require.NoError(t, logServer.Serve(ln))
	}()

	conn, err := grpc.Dial(logServerAddr, grpc.WithInsecure())
	require.NoError(t, err)
	logClient := api.NewLogClient(conn)

	ledgerServer, err := ledger.NewServer(&ledger.Config{
		Repo:      &repo{},
		LogClient: logClient,
	})
	require.NoError(t, err)

	go func() {
		ln, err := net.Listen("tcp", ledgerServerAddr)
		require.NoError(t, err)

		require.NoError(t,
			ledgerServer.Serve(ln))
	}()

	// create ledger client
	opts := []grpc.DialOption{grpc.WithInsecure()}
	conn, err = grpc.Dial(ledgerServerAddr, opts...)
	require.NoError(t, err)

	ledgerClient := api.NewLedgerClient(conn)
	numTransactions := 5
	for i := 0; i < numTransactions; i++ {
		ctx := context.Background()
		{
			resp, err := ledgerClient.CreateTransaction(ctx, &api.TransactionRequest{
				Value: float64(i),
			})
			require.NoError(t, err)
			require.NotNil(t, resp)
		}

		{
			// check log
			resp, err := logClient.Consume(ctx, &api.ConsumeRequest{Offset: uint64(i)})
			require.NoError(t, err)
			require.EqualValues(t, i, resp.Record.Offset)
		}
	}
}

type repo struct {
	transactions []ledger.Transaction
}

func (r *repo) Create(transaction *ledger.Transaction) error {
	r.transactions = append(r.transactions, *transaction)
	return nil
}
