package transaction_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/travisjeffery/go-dynaport"
	"google.golang.org/grpc"

	api "ledger/api/v1"
	"ledger/internal/log"
	"ledger/internal/server"
	"ledger/transaction"
	"ledger/transaction/postgres"
)

func TestServer(t *testing.T) {
	s := NewSuite(t)
	suite.Run(t, s)
}

func NewSuite(t *testing.T) *Suite {
	return &Suite{
		Assertions: require.New(t),
	}
}

type Suite struct {
	suite.Suite
	*require.Assertions // default to require behavior

	db *sqlx.DB

	// TransactionRepo                ledger.TransactionRepo
	logServerAddr string
	logServer     *grpc.Server
	logClient     api.LogClient

	ledgerServerAddr string
	ledgerServer     *grpc.Server
	ledgerClient     api.LedgerClient
}

func (s *Suite) SetupSuite() {
	// load environment
	err := godotenv.Load("../.env")
	s.NoError(err)

	// setup postgres
	config, err := postgres.Parse()
	s.NoError(err)

	db, err := postgres.Connect(config)
	s.NoError(err)
	s.db = db

	// setup address ports
	ports := dynaport.Get(2)
	s.ledgerServerAddr = fmt.Sprintf(":%d", ports[0])
	s.logServerAddr = fmt.Sprintf(":%d", ports[1])

	dir, err := ioutil.TempDir("", "app-test")
	s.NoError(err)

	commitLog, err := log.NewLog(dir, log.Config{})
	s.NoError(err)

	// create server and client for `log`
	logServer, err := server.NewGRPCServer(&server.Config{
		CommitLog:    commitLog,
		ServerGetter: nil,
	})
	s.NoError(err)
	s.logServer = logServer

	// serve logServer
	go func() {
		ln, err := net.Listen("tcp", s.logServerAddr)
		s.NoError(err)
		s.NoError(s.logServer.Serve(ln))
	}()

	conn, err := grpc.Dial(s.logServerAddr, grpc.WithInsecure())
	s.NoError(err)
	s.logClient = api.NewLogClient(conn)

	repo, err := transaction.NewPostgresRepo(s.db)
	s.NoError(err)
	// create server and client for `ledger`
	ledgerServer, err := transaction.NewServer(&transaction.Config{
		Repo:      repo,
		LogClient: s.logClient,
	})
	s.NoError(err)
	s.ledgerServer = ledgerServer

	// serve ledgerServer
	go func() {
		ln, err := net.Listen("tcp", s.ledgerServerAddr)
		s.NoError(err)

		s.NoError(s.ledgerServer.Serve(ln))
	}()

	// create ledger client
	opts := []grpc.DialOption{grpc.WithInsecure()}
	conn, err = grpc.Dial(s.ledgerServerAddr, opts...)
	s.NoError(err)

	s.ledgerClient = api.NewLedgerClient(conn)
}

func (s *Suite) SetupTest() {
	s.db.MustExec("DElETE FROM transaction")
}

func (s *Suite) TestCreateTransactions() {
	numTransactions := 5
	for i := 0; i < numTransactions; i++ {
		ctx := context.Background()
		{
			amount := fmt.Sprintf("%d", 100*i)
			resp, err := s.ledgerClient.CreateTransaction(ctx, &api.TransactionRequest{
				Amount: &api.BigDecimal{Value: amount},
			})
			s.NoError(err)
			s.NotNil(resp)
		}

		{ // Check log
			resp, err := s.logClient.Consume(ctx, &api.ConsumeRequest{Offset: uint64(i)})
			s.NoError(err)
			s.EqualValues(i, resp.Record.Offset)
		}
	}
}
