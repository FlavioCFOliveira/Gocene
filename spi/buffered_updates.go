// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

// BufferedUpdatesRef is a marker interface that lets SegmentWriteState
// carry a pointer to the concrete BufferedUpdates structure defined in
// package index without spi/ taking a dependency on index/.
//
// The concrete *index.BufferedUpdates value satisfies this interface via
// the IsBufferedUpdates() sentinel; consumers inside index/ that need
// the structured data type-assert back to *BufferedUpdates.
//
// This is a deliberate Gocene deviation from the Apache Lucene 10.4.0
// shape: Lucene's SegmentWriteState carries a concrete BufferedUpdates
// field, but doing the same in Go would force spi/ to import index/,
// which is the very cycle the SPI unification (rmp #4669) breaks. The
// marker interface preserves type safety (the field cannot be set to an
// unrelated type) without re-introducing the cycle.
type BufferedUpdatesRef interface {
	// IsBufferedUpdates is a sentinel method that no other type in the
	// tree implements, so the interface is effectively closed to
	// *index.BufferedUpdates.
	IsBufferedUpdates()
}
