// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// LongValues provides per-segment, per-document long values that can be
// computed at search time.
//
// Mirrors org.apache.lucene.search.LongValues.
type LongValues interface {
	// LongValue returns the long value associated with the current document.
	// It must be called only after AdvanceExact returned true.
	LongValue() (int64, error)

	// AdvanceExact positions the iterator at the given doc id and returns true
	// if there is a value for this document.
	AdvanceExact(doc int) (bool, error)
}
