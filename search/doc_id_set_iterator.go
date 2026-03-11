// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// DocIdSetIterator iterates over document IDs.
type DocIdSetIterator interface {
	// DocID returns the current document ID.
	// Returns -1 if not positioned, NO_MORE_DOCS if past last document.
	DocID() int

	// NextDoc advances to the next document.
	// Returns the document ID or NO_MORE_DOCS if no more documents.
	NextDoc() (int, error)

	// Advance advances to the document at or beyond the target.
	// Returns the document ID or NO_MORE_DOCS if no more documents.
	Advance(target int) (int, error)

	// Cost returns the estimated cost of iterating through all documents.
	Cost() int64
}

// NO_MORE_DOCS indicates the end of the document iterator.
const NO_MORE_DOCS = 2147483647

// BaseDocIdSetIterator provides common functionality.
type BaseDocIdSetIterator struct {
	doc int
}

// NewBaseDocIdSetIterator creates a new BaseDocIdSetIterator.
func NewBaseDocIdSetIterator() *BaseDocIdSetIterator {
	return &BaseDocIdSetIterator{doc: -1}
}

// DocID returns the current document ID.
func (it *BaseDocIdSetIterator) DocID() int {
	return it.doc
}

// NextDoc advances to the next document.
func (it *BaseDocIdSetIterator) NextDoc() (int, error) {
	return NO_MORE_DOCS, nil
}

// Advance advances to the target document.
func (it *BaseDocIdSetIterator) Advance(target int) (int, error) {
	return NO_MORE_DOCS, nil
}

// Cost returns the cost.
func (it *BaseDocIdSetIterator) Cost() int64 {
	return 0
}
