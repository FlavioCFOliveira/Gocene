// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// SorterDocMap maps documents from the source order to the sorted order
// during an index sort. Mirrors
// org.apache.lucene.index.Sorter.DocMap from Apache Lucene 10.4.0.
type SorterDocMap interface {
	// OldToNew returns the new doc ID for the given old doc ID, or -1 if
	// the source document was discarded.
	OldToNew(oldDocID int) int

	// NewToOld returns the old doc ID for the given new doc ID, or -1 if
	// the target slot is empty.
	NewToOld(newDocID int) int

	// Size returns the number of documents in the sorted view.
	Size() int
}

// SorterPolicy is the strategy that produces a SorterDocMap for a given
// CodecReader. Mirrors the algorithm-side of Lucene's Sorter class.
//
// Gocene skeleton: the actual sort implementation (BKD-aware comparator
// stack) is deferred to backlog #2708.
type SorterPolicy interface {
	// Sort returns the SorterDocMap for reader. Returns nil when reader is
	// already in sorted order.
	Sort(reader interface{}) (SorterDocMap, error)

	// String returns a short description of the policy (for diagnostics).
	String() string
}
