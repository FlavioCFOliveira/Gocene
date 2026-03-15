// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"fmt"
	"sort"
)

// IntArrayDocIdSet is a DocIdSet implementation backed by a sorted int array.
// This is the Go port of Lucene's org.apache.lucene.util.IntArrayDocIdSet.
type IntArrayDocIdSet struct {
	docs   []int
	length int
}

// NewIntArrayDocIdSet creates a new IntArrayDocIdSet.
// The docs array must be sorted and the element at position 'length' must be NO_MORE_DOCS.
func NewIntArrayDocIdSet(docs []int, length int) (*IntArrayDocIdSet, error) {
	if length >= len(docs) {
		return nil, fmt.Errorf("length %d exceeds array length %d", length, len(docs))
	}
	if docs[length] != NO_MORE_DOCS {
		return nil, fmt.Errorf("docs[length] must be NO_MORE_DOCS (%d), got %d", NO_MORE_DOCS, docs[length])
	}
	// Verify array is sorted
	for i := 1; i < length; i++ {
		if docs[i] < docs[i-1] {
			return nil, fmt.Errorf("docs array must be sorted")
		}
	}
	return &IntArrayDocIdSet{
		docs:   docs,
		length: length,
	}, nil
}

// Iterator returns a DocIdSetIterator over the int array.
func (i *IntArrayDocIdSet) Iterator() DocIdSetIterator {
	return NewIntArrayDocIdSetIterator(i.docs, i.length)
}

// Length returns the number of documents in this set.
func (i *IntArrayDocIdSet) Length() int {
	return i.length
}

// Docs returns the underlying docs array (for testing).
func (i *IntArrayDocIdSet) Docs() []int {
	return i.docs
}

// IntArrayDocIdSetIterator iterates over an int array.
type IntArrayDocIdSetIterator struct {
	docs   []int
	length int
	i      int
	doc    int
}

// NewIntArrayDocIdSetIterator creates a new iterator over the int array.
func NewIntArrayDocIdSetIterator(docs []int, length int) *IntArrayDocIdSetIterator {
	return &IntArrayDocIdSetIterator{
		docs:   docs,
		length: length,
		i:      0,
		doc:    -1,
	}
}

// DocID returns the current document ID.
func (it *IntArrayDocIdSetIterator) DocID() int {
	return it.doc
}

// NextDoc advances to the next document.
func (it *IntArrayDocIdSetIterator) NextDoc() (int, error) {
	if it.i >= it.length {
		it.doc = NO_MORE_DOCS
		return it.doc, nil
	}
	it.doc = it.docs[it.i]
	it.i++
	return it.doc, nil
}

// Advance advances to the target document using exponential search + binary search.
func (it *IntArrayDocIdSetIterator) Advance(target int) (int, error) {
	if it.i >= it.length {
		it.doc = NO_MORE_DOCS
		return it.doc, nil
	}

	// Exponential search to find range
	bound := 1
	for it.i+bound < it.length && it.docs[it.i+bound] < target {
		bound *= 2
	}

	// Binary search within the range
	low := it.i + bound/2
	high := it.i + bound + 1
	if high > it.length {
		high = it.length
	}

	// Use sort.Search to find the first element >= target
	idx := sort.Search(high-low, func(i int) bool {
		return it.docs[low+i] >= target
	})

	it.i = low + idx
	if it.i >= it.length {
		it.doc = NO_MORE_DOCS
		return it.doc, nil
	}
	it.doc = it.docs[it.i]
	it.i++
	return it.doc, nil
}

// Cost returns the estimated cost.
func (it *IntArrayDocIdSetIterator) Cost() int64 {
	return int64(it.length)
}

// DocIDRunEnd returns the end of the current run of consecutive doc IDs.
func (it *IntArrayDocIdSetIterator) DocIDRunEnd() int {
	return it.doc + 1
}

// Ensure IntArrayDocIdSet implements DocIdSet
var _ DocIdSet = (*IntArrayDocIdSet)(nil)

// Ensure IntArrayDocIdSetIterator implements DocIdSetIterator
var _ DocIdSetIterator = (*IntArrayDocIdSetIterator)(nil)
