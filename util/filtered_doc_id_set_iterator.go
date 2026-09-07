package util

import (
	"fmt"
	"unsafe"
)

// FilteredDocIdSetIterator is a DocIdSetIterator that returns documents matching a predicate
// by scanning all document positions.
//
// This generic iterator can be used to iterate either live or deleted documents based on a
// predicate function. Iteration has O(maxDoc) complexity since it must scan all document positions.
type FilteredDocIdSetIterator struct {
	maxDoc    int
	cost      int
	predicate func(int) bool
	doc       int
}

// NewFilteredDocIdSetIterator creates a filtered iterator over documents.
func NewFilteredDocIdSetIterator(maxDoc, cost int, predicate func(int) bool) *FilteredDocIdSetIterator {
	return &FilteredDocIdSetIterator{
		maxDoc:    maxDoc,
		cost:      cost,
		predicate: predicate,
		doc:       -1,
	}
}

// DocID returns the current doc ID.
func (f *FilteredDocIdSetIterator) DocID() int {
	return f.doc
}

// NextDoc advances to the next document in the set and returns the doc it is currently on.
func (f *FilteredDocIdSetIterator) NextDoc() (int, error) {
	return f.Advance(f.doc + 1)
}

// Advance advances to the first document whose document number is >= target and matches the predicate.
func (f *FilteredDocIdSetIterator) Advance(target int) (int, error) {
	if target >= f.maxDoc {
		f.doc = NO_MORE_DOCS
		return f.doc, nil
	}

	f.doc = target
	for f.doc < f.maxDoc {
		if f.predicate(f.doc) {
			return f.doc, nil
		}
		f.doc++
	}
	f.doc = NO_MORE_DOCS
	return f.doc, nil
}

// DocIDRunEnd returns the end of the run of consecutive doc IDs that match
// this iterator and contains the current docID.
func (f *FilteredDocIdSetIterator) DocIDRunEnd() int {
	return f.doc + 1
}

// Cost returns the estimated cost of this iterator.
func (f *FilteredDocIdSetIterator) Cost() int64 {
	return int64(f.cost)
}

// Equals checks if this iterator is the same instance as the other.
func (f *FilteredDocIdSetIterator) Equals(other any) bool {
	return f == other
}

// HashCode returns a hash code based on the iterator's memory address.
func (f *FilteredDocIdSetIterator) HashCode() int {
	return int(uintptr(unsafe.Pointer(f)))
}

// String returns a string representation of the iterator.
func (f *FilteredDocIdSetIterator) String() string {
	return fmt.Sprintf("FilteredDocIdSetIterator@%p", f)
}
