package ledger_test

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"github.com/peterbourgon/ff"
	"github.com/stretchr/testify/require"
	"github.com/travisjeffery/go-dynaport"
	"google.golang.org/grpc"

	api "proglog/api/v1"
	"proglog/internal/log"
	"proglog/internal/server"
	"proglog/ledger"
)

func TestX(t *testing.T) {
	postgresFlags := flag.NewFlagSet("postgres", flag.ExitOnError)
	var (
		host   = postgresFlags.String("host", "localhost", "host to connect to")
		port   = postgresFlags.Int("port", 5432, "port to bind to")
		user   = postgresFlags.String("user", "", "user to sign in as")
		dbName = postgresFlags.String("db_name", "", "name of the database")
		_      = postgresFlags.String("test.v", "", "") // ignore flag passed by Intellij
	)

	require.NoError(t, godotenv.Load("../.env"))
	require.NoError(t,
		ff.Parse(postgresFlags, os.Args[1:],
			ff.WithIgnoreUndefined(true),
			ff.WithEnvVarPrefix("POSTGRES"),
		))

	// todo: call table creation
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

	db, err := ledger.ConnectPostgresDB(*host, *port, *user, *dbName)
	require.NoError(t, err)
	repo, err := ledger.NewPostgresRepo(db)
	require.NoError(t, err)

	ledgerServer, err := ledger.NewServer(&ledger.Config{
		Repo:      repo,
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
			amount := fmt.Sprintf("%d", 100*i)
			resp, err := ledgerClient.CreateTransaction(ctx, &api.TransactionRequest{
				Amount: &api.BigDecimal{Value: amount},
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
