package search

// DocIdStream is a stream of doc IDs.
type DocIdStream interface {
	// ForEach iterates over doc IDs contained in this stream in order.
	ForEach(consumer func(int) error) error

	// ForEachUpTo iterates over doc IDs up to the given upTo exclusive.
	ForEachUpTo(upTo int, consumer func(int) error) error

	// Count returns the number of entries in this stream.
	Count() (int, error)

	// CountUpTo returns the number of doc IDs in this stream that are below the given upTo.
	CountUpTo(upTo int) (int, error)

	// IntoArray copies some matching doc IDs into the provided array and returns the count.
	IntoArray(array []int) int

	// IntoArrayUpTo copies some matching doc IDs under upTo (exclusive) into the provided array.
	IntoArrayUpTo(upTo int, array []int) int

	// MayHaveRemaining returns true if this stream may have remaining doc IDs.
	MayHaveRemaining() bool
}
