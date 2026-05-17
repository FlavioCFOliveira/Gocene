// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// LeafFieldComparator is the per-segment sibling of FieldComparator, used by
// TopFieldCollector to compare a freshly seen document against the current
// queue contents.
//
// Mirrors org.apache.lucene.search.LeafFieldComparator.
type LeafFieldComparator interface {
	// SetBottom records the slot of the queue's weakest entry.
	SetBottom(slot int) error

	// CompareBottom compares a new document against the bottom entry, returning
	// a Java-style sign convention: positive if the new doc is better than the
	// bottom, zero if equal, negative if worse.
	CompareBottom(doc int) (int, error)

	// CompareTop compares a new document against the top value (used in deep
	// pagination scenarios).
	CompareTop(doc int) (int, error)

	// Copy transfers the value of the given document into the queue slot.
	Copy(slot, doc int) error

	// SetScorer hands the leaf scorer to the comparator.
	SetScorer(scorer Scorable) error

	// CompetitiveIterator returns an iterator over the documents that are
	// competitive given the queue's current state, or nil if no such
	// optimization is possible.
	CompetitiveIterator() (DocIdSetIterator, error)

	// SetHitsThresholdReached notifies the comparator that the hits threshold
	// has been reached.
	SetHitsThresholdReached()
}
