// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

// MultiLongValues represents a per-document iterator over multiple long
// (int64) values. Mirrors the abstract org.apache.lucene.facet.MultiLongValues.
type MultiLongValues interface {
	AdvanceExact(docID int) (bool, error)
	DocValueCount() int
	NextValue() (int64, error)
}

// EmptyMultiLongValues is the canonical empty MultiLongValues.
type EmptyMultiLongValues struct{}

func (EmptyMultiLongValues) AdvanceExact(int) (bool, error) { return false, nil }
func (EmptyMultiLongValues) DocValueCount() int             { return 0 }
func (EmptyMultiLongValues) NextValue() (int64, error)      { return 0, nil }

// NewEmptyMultiLongValues returns the canonical empty iterator.
func NewEmptyMultiLongValues() MultiLongValues { return EmptyMultiLongValues{} }
