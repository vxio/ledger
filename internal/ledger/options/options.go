package options

import "time"

// TransactionOptions represent options that can be used to configure Find()
type TransactionOptions struct {
	IDs []string
	// FromIDs []string
	// ToIds   []string
	Amount    *IntRange
	Timestamp *TimeRange
}

func NewTransactionOptions() *TransactionOptions {
	return &TransactionOptions{}
}

func (this *TransactionOptions) SetIDs(v ...string) *TransactionOptions {
	this.IDs = v
	return this
}

func (this *TransactionOptions) SetAmountRange(v *IntRange) *TransactionOptions {
	this.Amount = v
	return this
}

func (this *TransactionOptions) SetTimeRange(v *TimeRange) *TransactionOptions {
	this.Timestamp = v
	return this
}

type TimeRange struct {
	Low  *time.Time
	High *time.Time
}

func (r *TimeRange) From() (interface{}, bool) {
	if r.Low != nil {
		return r.Low, true
	}
	return nil, false
}

func (r *TimeRange) To() (interface{}, bool) {
	if r.High != nil {
		return r.High, true
	}
	return nil, false
}
