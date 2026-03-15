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

	// DocIDRunEnd returns the end of the run of consecutive doc IDs that match
	// this iterator and that contains the current docID.
	// Returns one plus the last doc ID of the run.
	DocIDRunEnd() int
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

// DocIDRunEnd returns the end of the current run of consecutive doc IDs.
// Default implementation assumes runs of a single doc ID.
func (it *BaseDocIdSetIterator) DocIDRunEnd() int {
	return it.doc + 1
}

// RangeDocIdSetIterator iterates over a range of document IDs [minDoc, maxDoc).
type RangeDocIdSetIterator struct {
	BaseDocIdSetIterator
	minDoc int
	maxDoc int
}

// NewRangeDocIdSetIterator creates a new iterator over the range [minDoc, maxDoc).
func NewRangeDocIdSetIterator(minDoc, maxDoc int) *RangeDocIdSetIterator {
	return &RangeDocIdSetIterator{
		BaseDocIdSetIterator: BaseDocIdSetIterator{doc: -1},
		minDoc:               minDoc,
		maxDoc:               maxDoc,
	}
}

// NextDoc advances to the next document.
func (it *RangeDocIdSetIterator) NextDoc() (int, error) {
	if it.doc == -1 {
		it.doc = it.minDoc
	} else {
		it.doc++
	}
	if it.doc >= it.maxDoc {
		it.doc = NO_MORE_DOCS
	}
	return it.doc, nil
}

// Advance advances to the target document.
func (it *RangeDocIdSetIterator) Advance(target int) (int, error) {
	if target >= it.maxDoc {
		it.doc = NO_MORE_DOCS
		return it.doc, nil
	}
	if target <= it.minDoc {
		it.doc = it.minDoc
	} else {
		it.doc = target
	}
	return it.doc, nil
}

// Cost returns the cost.
func (it *RangeDocIdSetIterator) Cost() int64 {
	return int64(it.maxDoc - it.minDoc)
}

// DocIDRunEnd returns the end of the current run.
func (it *RangeDocIdSetIterator) DocIDRunEnd() int {
	return it.maxDoc
}

// Ensure RangeDocIdSetIterator implements DocIdSetIterator
var _ DocIdSetIterator = (*RangeDocIdSetIterator)(nil)

// EmptyDocIdSetIterator is an empty iterator.
type EmptyDocIdSetIterator struct {
	BaseDocIdSetIterator
}

// NewEmptyDocIdSetIterator returns an empty DocIdSetIterator.
func NewEmptyDocIdSetIterator() DocIdSetIterator {
	return &EmptyDocIdSetIterator{
		BaseDocIdSetIterator: BaseDocIdSetIterator{doc: NO_MORE_DOCS},
	}
}

// NextDoc returns NO_MORE_DOCS.
func (it *EmptyDocIdSetIterator) NextDoc() (int, error) {
	return NO_MORE_DOCS, nil
}

// Advance returns NO_MORE_DOCS.
func (it *EmptyDocIdSetIterator) Advance(target int) (int, error) {
	return NO_MORE_DOCS, nil
}

// Cost returns 0.
func (it *EmptyDocIdSetIterator) Cost() int64 {
	return 0
}

// Ensure EmptyDocIdSetIterator implements DocIdSetIterator
var _ DocIdSetIterator = (*EmptyDocIdSetIterator)(nil)
