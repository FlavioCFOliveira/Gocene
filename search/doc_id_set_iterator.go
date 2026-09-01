package search

import "math"

// NO_MORE_DOCS is the sentinel value indicating no more docs in the iterator.
const NO_MORE_DOCS = math.MaxInt32

// Empty returns an empty DocIdSetIterator.
func Empty() DocIdSetIterator {
	return NewRangeDocIdSetIterator(0, 0)
}

// All returns a DocIdSetIterator that matches all documents up to maxDoc - 1.
func All(maxDoc int) DocIdSetIterator {
	if maxDoc < 0 {
		panic("maxDoc must be >= 0")
	}
	return NewRangeDocIdSetIterator(0, maxDoc)
}

// Range returns a DocIdSetIterator that matches a range of documents from minDoc (inclusive) to maxDoc (exclusive).
func Range(minDoc, maxDoc int) DocIdSetIterator {
	if minDoc >= maxDoc {
		panic("minDoc must be < maxDoc")
	}
	if minDoc < 0 {
		panic("minDoc must be >= 0")
	}
	return NewRangeDocIdSetIterator(minDoc, maxDoc)
}

// DocIdSetIterator defines methods to iterate over a set of non-decreasing doc ids.
type DocIdSetIterator interface {
	// DocID returns the current doc ID.
	DocID() int

	// NextDoc advances to the next document in the set and returns it.
	NextDoc() (int, error)

	// Advance advances to the first document greater than or equal to target.
	Advance(target int) (int, error)

	// Cost returns the estimated cost of this iterator.
	Cost() int64

	// IntoBitSet loads doc IDs into a FixedBitSet.
	IntoBitSet(upTo int, bitSet BitSet, offset int) error

	// DocIDRunEnd returns the end of the run of consecutive doc IDs.
	DocIDRunEnd() (int, error)
}

// SlowAdvance is a linear implementation of Advance relying on NextDoc.
func SlowAdvance(it DocIdSetIterator, target int) (int, error) {
	doc := it.DocID()
	for doc < target {
		var err error
		doc, err = it.NextDoc()
		if err != nil {
			return 0, err
		}
	}
	return doc, nil
}

// BitSet is a simple interface for bitset operations.
type BitSet interface {
	Set(doc, upTo int)
}

// RangeDocIdSetIterator implements DocIdSetIterator for a range of docs.
type RangeDocIdSetIterator struct {
	minDoc, maxDoc int
	doc            int
}

func NewRangeDocIdSetIterator(minDoc, maxDoc int) *RangeDocIdSetIterator {
	return &RangeDocIdSetIterator{
		minDoc: minDoc,
		maxDoc: maxDoc,
		doc:    -1,
	}
}

func (r *RangeDocIdSetIterator) DocID() int {
	return r.doc
}

func (r *RangeDocIdSetIterator) NextDoc() (int, error) {
	return r.Advance(r.doc + 1)
}

func (r *RangeDocIdSetIterator) Advance(target int) (int, error) {
	if target >= r.maxDoc {
		r.doc = NO_MORE_DOCS
	} else if target < r.minDoc {
		r.doc = r.minDoc
	} else {
		r.doc = target
	}
	return r.doc, nil
}

func (r *RangeDocIdSetIterator) Cost() int64 {
	return int64(r.maxDoc - r.minDoc)
}

func (r *RangeDocIdSetIterator) IntoBitSet(upTo int, bitSet BitSet, offset int) error {
	if upTo > r.doc {
		limit := upTo
		if r.maxDoc < limit {
			limit = r.maxDoc
		}
		bitSet.Set(r.doc-offset, limit-offset)
		r.Advance(upTo)
	}
	return nil
}

func (r *RangeDocIdSetIterator) DocIDRunEnd() (int, error) {
	return r.maxDoc, nil
}
