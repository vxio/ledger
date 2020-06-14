package ledger

// TransactionOptions represent options that can be used to configure Find()
type TransactionOptions struct {
	IDs []string
	// FromIDs []string
	// ToIds   []string
	// AmountRange
	// Time
}

func NewTransactionOptions() *TransactionOptions {
	return &TransactionOptions{}
}

func (this *TransactionOptions) SetIDs(v ...string) *TransactionOptions {
	this.IDs = v
	return this
}
