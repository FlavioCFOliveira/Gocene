package util

import (
	"math"
)

const NO_MORE_DOCS = math.MaxInt32

// DocIdSetIterator defines methods to iterate over a set of non-decreasing doc IDs.
type DocIdSetIterator interface {
	// DocID returns the current doc ID.
	// -1 if nextDoc() or Advance() were not called yet.
	// NO_MORE_DOCS if the iterator has exhausted.
	DocID() int

	// NextDoc advances to the next document in the set and returns the doc it is currently on.
	NextDoc() (int, error)

	// Advance advances to the first beyond the current whose document number is >= target.
	Advance(target int) (int, error)

	// DocIDRunEnd returns the end of the run of consecutive doc IDs that match
	// this iterator and that contains the current docID.
	// Returns one plus the last doc ID of the run.
	DocIDRunEnd() int

	// Cost returns the estimated cost of this iterator.
	Cost() int64
}

