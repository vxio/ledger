package options

import "time"

var _ Range = (*TimeRange)(nil)

// TimeRange describes a lower and upper bound for Time values
// Either bound is optional
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
