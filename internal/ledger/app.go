package ledger

import (
	"context"

	"github.com/gogo/protobuf/proto"
	"google.golang.org/grpc"

	api "proglog/api/v1"
)

func NewServer(config *Config, opts ...grpc.ServerOption) (*grpc.Server, error) {
	grpcServer := grpc.NewServer(opts...)

	server := &LedgerServer{
		logClient: config.LogClient,
		repo:      config.Repo,
	}

	api.RegisterLedgerServer(grpcServer, server)
	return grpcServer, nil
}

type Config struct {
	Repo          Repo
	LogClientAddr string
	LogClient     api.LogClient
}

var _ api.LedgerServer = (*LedgerServer)(nil)

// Ledge server
type LedgerServer struct {
	logClient api.LogClient
	repo      Repo
}

func (l *LedgerServer) CreateTransaction(ctx context.Context, req *api.TransactionRequest, ) (*api.TransactionResponse, error) {
	t := api.Transaction{
		Value: req.Value,
	}

	b, _ := proto.Marshal(&t)

	// store in our log
	request := &api.ProduceRequest{
		Record: &api.Record{Value: b},
	}

	_, err := l.logClient.Produce(ctx, request)
	if err != nil {
		return nil, err
	}

	transaction := Transaction{Amount: int(req.Value)}

	// save to database
	_ = l.repo.Create(&transaction)

	// return the transaction
	res := &api.TransactionResponse{
		Transaction: &api.Transaction{
			Value: t.GetValue(),
		},
	}

	return res, nil
}
