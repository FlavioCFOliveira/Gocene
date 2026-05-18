// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

// MultiDoubleValues represents a per-document iterator of multiple double
// values. Mirrors the abstract org.apache.lucene.facet.MultiDoubleValues.
//
// Iteration contract: AdvanceExact positions the cursor at a document and
// returns true if it has any values; subsequent NextValue calls return each
// value until DocValueCount has been exhausted.
type MultiDoubleValues interface {
	// AdvanceExact positions the iterator to docID and returns whether the
	// document has at least one value.
	AdvanceExact(docID int) (bool, error)

	// DocValueCount returns the number of values for the current document.
	DocValueCount() int

	// NextValue returns the next value for the current document.
	NextValue() (float64, error)
}

// EmptyMultiDoubleValues is a MultiDoubleValues that never returns any value.
type EmptyMultiDoubleValues struct{}

// AdvanceExact always returns false.
func (EmptyMultiDoubleValues) AdvanceExact(int) (bool, error) { return false, nil }

// DocValueCount always returns 0.
func (EmptyMultiDoubleValues) DocValueCount() int { return 0 }

// NextValue always returns 0.
func (EmptyMultiDoubleValues) NextValue() (float64, error) { return 0, nil }

// NewEmptyMultiDoubleValues returns the canonical empty iterator.
func NewEmptyMultiDoubleValues() MultiDoubleValues { return EmptyMultiDoubleValues{} }
