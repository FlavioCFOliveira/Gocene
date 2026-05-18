// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sortedset

// SortedSetDocValuesReaderState exposes the ordinal/term metadata required by
// SortedSetDocValuesFacetCounts. Mirrors the abstract
// org.apache.lucene.facet.sortedset.SortedSetDocValuesReaderState.
type SortedSetDocValuesReaderState interface {
	// GetField returns the indexed SortedSetDocValues field.
	GetField() string

	// GetSize returns the total number of unique ordinals.
	GetSize() int

	// GetOrdRange returns the [start, end) ordinal range owned by dim, or
	// (-1, -1) when dim is unknown.
	GetOrdRange(dim string) (int, int)

	// GetDims returns the list of dimensions known to the state.
	GetDims() []string
}
