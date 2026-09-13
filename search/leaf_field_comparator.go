// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Ported from Apache Lucene 10.5.0:
//   lucene/core/src/java/org/apache/lucene/search/LeafFieldComparator.java

// LeafFieldComparator is the comparator that gets instantiated on each leaf
// from a top-level [FieldComparator] instance.
//
// A leaf comparator must define these functions:
//
//   - SetBottom: called by the field-value hit queue to notify the comparator
//     of the current weakest ("bottom") slot. Note that this slot may not hold
//     the weakest value according to this comparator, in cases where it is not
//     the primary one (ie, is only used to break ties from the comparators
//     before it).
//   - CompareBottom: compare a new hit (docID) against the "weakest" (bottom)
//     entry in the queue.
//   - CompareTop: compare a new hit (docID) against the top value previously
//     set by a call to [FieldComparator].SetTopValue.
//   - Copy: installs a new hit into the priority queue. The field-value hit
//     queue calls this method when a new hit is competitive.
//
// Mirrors org.apache.lucene.search.LeafFieldComparator. Every member that Java
// declares `throws IOException` returns an error here.
type LeafFieldComparator interface {
	// SetBottom sets the bottom slot, ie the "weakest" (sorted last) entry in
	// the queue. When CompareBottom is called, you should compare against this
	// slot. This will always be called before CompareBottom.
	//
	// Mirrors void setBottom(int slot) throws IOException.
	SetBottom(slot int) error

	// CompareBottom compares the bottom of the queue with this doc. This will
	// only be invoked after SetBottom has been called. It returns the same
	// result as FieldComparator.Compare would as if bottom were slot1 and the
	// new document were slot 2: any N < 0 if the doc's value is sorted after
	// the bottom entry (not competitive), any N > 0 if the doc's value is
	// sorted before the bottom entry and 0 if they are equal.
	//
	// Mirrors int compareBottom(int doc) throws IOException.
	CompareBottom(doc int) (int, error)

	// CompareTop compares the top value with this doc. This will only be
	// invoked after FieldComparator.SetTopValue has been called, and only for
	// searches that use searchAfter (deep paging).
	//
	// Mirrors int compareTop(int doc) throws IOException.
	CompareTop(doc int) (int, error)

	// Copy is called when a new hit is competitive: it copies any state
	// associated with this document that will be required for future
	// comparisons into the specified slot.
	//
	// Mirrors void copy(int slot, int doc) throws IOException.
	Copy(slot, doc int) error

	// SetScorer sets the Scorable to use in case a document's score is needed.
	//
	// Mirrors void setScorer(Scorable scorer) throws IOException.
	SetScorer(scorer Scorable) error

	// CompetitiveIterator returns an iterator over competitive docs that are
	// stronger than already collected docs, or nil if such an iterator is not
	// available for the current comparator or segment.
	//
	// Mirrors the default method
	// DocIdSetIterator competitiveIterator() throws IOException.
	CompetitiveIterator() (DocIdSetIterator, error)

	// SetHitsThresholdReached informs this leaf comparator that the hits
	// threshold is reached. This method is called from a collector when the
	// hits threshold is reached.
	//
	// Mirrors the default method
	// void setHitsThresholdReached() throws IOException.
	SetHitsThresholdReached() error
}
