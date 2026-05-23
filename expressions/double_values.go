// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package expressions

// DoubleValues provides per-document double-precision floating-point values.
// It is the Go equivalent of org.apache.lucene.search.DoubleValues.
//
// The typical usage pattern is:
//  1. Call AdvanceExact(docID) to position on a document.
//  2. Call DoubleValue() to read the value; it may return 0 when the source
//     has no value for the current document (depending on the implementation).
type DoubleValues interface {
	// AdvanceExact positions the source on docID and returns true if a value
	// is available for that document.
	AdvanceExact(docID int) (bool, error)

	// DoubleValue returns the numeric value for the current document.
	DoubleValue() (float64, error)
}

// DoubleValuesSource creates per-segment DoubleValues instances.
// It is the Go equivalent of org.apache.lucene.search.DoubleValuesSource.
type DoubleValuesSource interface {
	// GetValues returns the DoubleValues for a segment given an optional scores
	// source. scores may be nil.
	GetValues(scores DoubleValues) (DoubleValues, error)

	// NeedsScores returns true if this source depends on document scores.
	NeedsScores() bool

	// IsCacheable returns true if results can be cached across readers.
	IsCacheable() bool
}

// ConstantDoubleValues is a DoubleValues that always returns a fixed value for
// every document. Useful as a stand-in for scores when calling GetValues.
type ConstantDoubleValues struct {
	value float64
}

// NewConstantDoubleValues creates a DoubleValues that always returns v.
func NewConstantDoubleValues(v float64) *ConstantDoubleValues {
	return &ConstantDoubleValues{value: v}
}

// AdvanceExact always returns true.
func (c *ConstantDoubleValues) AdvanceExact(_ int) (bool, error) { return true, nil }

// DoubleValue returns the constant.
func (c *ConstantDoubleValues) DoubleValue() (float64, error) { return c.value, nil }

var _ DoubleValues = (*ConstantDoubleValues)(nil)
