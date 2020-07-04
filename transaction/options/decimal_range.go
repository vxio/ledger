package options

import "github.com/shopspring/decimal"

// Creates a new DecimalRange instance
func NewDecimalRange() *DecimalRange {
	return &DecimalRange{}
}

var _ Range = (*TimeRange)(nil)

// DecimalRange describes a lower and upper bound for Decimal values
// Either bound is optional
type DecimalRange struct {
	Low  *decimal.Decimal
	High *decimal.Decimal
}

func (r *DecimalRange) From() (interface{}, bool) {
	if r.Low != nil {
		return r.Low.String(), true
	}
	return nil, false
}

func (r *DecimalRange) To() (interface{}, bool) {
	if r.High != nil {
		return r.High.String(), true
	}
	return nil, false
}
