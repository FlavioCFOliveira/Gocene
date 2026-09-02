// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// SorterDocMap maps documents from the source order to the sorted order
// during an index sort. Mirrors org.apache.lucene.index.Sorter.DocMap from
// Apache Lucene 10.4.0.
//
// Lifted onto the SPI by rmp #4707 because the wide
// spi.KnnVectorsWriter.Flush signature takes the same interface; this
// index-package identifier is preserved as a type alias so existing
// callers keep compiling without churn.
type SorterDocMap = spi.SorterDocMap

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
