package search

// CheckedIntConsumer is like IntConsumer, but may return an error.
type CheckedIntConsumer func(int) error
