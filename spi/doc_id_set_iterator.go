// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

// DocIdSetIterator provides an iterator over a set of document IDs.
// This is the Go port of Lucene's org.apache.lucene.index.DocIdSetIterator.
type DocIdSetIterator interface {
	// NextDoc advances to the next document ID and returns it.
	// Returns NO_MORE_DOCS when no more documents are available.
	NextDoc() (int, error)

	// Advance positions the iterator on the first document with doc ID >= target.
	// Returns the doc ID or NO_MORE_DOCS.
	Advance(target int) (int, error)

	// DocID returns the current document ID.
	DocID() int

	// DocIDRunEnd returns the end of the current run of documents.
	// A run is a contiguous sequence of document IDs.
	DocIDRunEnd() int

	// Cost returns an estimate of the cost of iterating over the set.
	Cost() int64
}
