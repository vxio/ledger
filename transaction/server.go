package transaction

import (
	"context"
	"fmt"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc"

	api "ledger/api/v1"
)

// Creates a gRPC server and registers our Server with it
// Give the gRPC server a listener to accept incoming connections
func NewServer(config *Config, opts ...grpc.ServerOption) (*grpc.Server, error) {
	grpcServer := grpc.NewServer(opts...)

	server := &Server{
		logClient: config.LogClient,
		repo:      config.Repo,
	}

	api.RegisterLedgerServer(grpcServer, server)
	return grpcServer, nil
}

// config used to create a new Server
type Config struct {
	Repo      TransactionRepo
	LogClient api.LogClient
}

// guarantee Server satisfies the api.LedgerServer interface
var _ api.LedgerServer = (*Server)(nil)

type Server struct {
	// the write-ahead-log used to record transactions
	logClient api.LogClient
	// our data-access layer used to store transactions
	repo TransactionRepo
}

func (l *Server) CreateTransaction(ctx context.Context, req *api.TransactionRequest) (*api.TransactionResponse, error) {
	senderId := uuid.New()
	receiverId := uuid.New()
	createdAt := time.Now().UTC()

	t := api.Transaction{
		SenderId:   &api.UUID{Value: senderId.String()},
		ReceiverId: &api.UUID{Value: receiverId.String()},
		Amount:     &api.BigDecimal{Value: req.Amount.Value},
		CreatedAt:  &createdAt,
	}

	b, _ := proto.Marshal(&t)

	// store in our write-ahead-log
	request := &api.ProduceRequest{
		Record: &api.Record{Value: b},
	}

	_, err := l.logClient.Produce(ctx, request)
	if err != nil {
		return nil, err
	}

	amount, err := decimal.NewFromString(req.Amount.Value)
	if err != nil {
		return nil, fmt.Errorf("parsing decimal from string: %v", err)
	}

	transaction := Transaction{
		SenderID:   senderId,
		ReceiverID: receiverId,
		Amount:     amount,
		CreatedAt:  createdAt,
	}

	// save to database
	err = l.repo.Create(&transaction)
	if err != nil {
		return nil, err
	}

	res := &api.TransactionResponse{
		Transaction: &api.Transaction{
			SenderId:   t.GetSenderId(),
			ReceiverId: t.GetReceiverId(),
			Amount:     t.GetAmount(),
			CreatedAt:  t.GetCreatedAt(),
		},
	}

	return res, nil
}
