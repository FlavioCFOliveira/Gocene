// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// SegmentOrder is the comparator-friendly enum that determines how segments
// are visited during merging. Mirrors org.apache.lucene.index.SegmentOrder
// from Apache Lucene 10.4.0.
//
// Ordinal ordering matches Lucene's enum:
//
//	NATURAL = 0, REVERSE = 1
type SegmentOrder int

const (
	// SegmentOrderNatural visits segments in their natural index order.
	SegmentOrderNatural SegmentOrder = iota
	// SegmentOrderReverse visits segments in reverse index order.
	SegmentOrderReverse
)

// String returns the Lucene-canonical name for diagnostics.
func (s SegmentOrder) String() string {
	switch s {
	case SegmentOrderNatural:
		return "NATURAL"
	case SegmentOrderReverse:
		return "REVERSE"
	default:
		return "UNKNOWN"
	}
}
