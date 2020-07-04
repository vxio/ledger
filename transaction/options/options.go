package options

// TransactionOptions represent options that can be used to configure a Find operation
type TransactionOptions struct {
	// filters transactions that match any id in this slice
	IDs []string
	// filters transactions that have an amount in this range (inclusive)
	Amount *DecimalRange
	// filters transactions that were created in this range (inclusive)
	Timestamp *TimeRange
}

func NewTransactionOptions() *TransactionOptions {
	return &TransactionOptions{}
}

func (this *TransactionOptions) SetIDs(v ...string) *TransactionOptions {
	this.IDs = v
	return this
}

func (this *TransactionOptions) SetAmountRange(v *DecimalRange) *TransactionOptions {
	this.Amount = v
	return this
}

func (this *TransactionOptions) SetTimeRange(v *TimeRange) *TransactionOptions {
	this.Timestamp = v
	return this
}
