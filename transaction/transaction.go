package transaction

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Transaction represents a cash transaction between a sender and receiver
type Transaction struct {
	ID         string          `db:"id"`
	SenderID   uuid.UUID       `db:"sender_id"`
	ReceiverID uuid.UUID       `db:"receiver_id"`
	Amount     decimal.Decimal `db:"amount"`
	CreatedAt  time.Time       `db:"created_at"`
}
