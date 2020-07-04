package options

// Range describes data that has a lower and upper bound
type Range interface {
	// From returns the lower bound
	// The return value may be nil so From follows the "comma ok" idiom
	From() (interface{}, bool)
	// To returns the upper bound
	// The return value may be nil so To follows the "comma ok" idiom
	To() (interface{}, bool)
}
