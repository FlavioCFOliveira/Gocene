// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Shared helpers for the *RangeFieldQueries testBasics integration ports
// (Int/Long/Float/Double). They build the per-dimension sortable-byte query
// payloads that the document-side range factories (DoubleRange.newXxxQuery in
// Lucene) would otherwise produce, and route them through the BKD-backed
// search.RangeFieldQuery (NewRangeFieldQueryFull), which is the exact Weight
// Lucene's RangeFieldQuery builds.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// encodeLongRangeQueryBounds packs N-dimensional long bounds into the
// numDims*8 sortable-byte queryMin / queryMax slices RangeFieldQueryFull wants.
func encodeLongRangeQueryBounds(values []int64) []byte {
	out := make([]byte, len(values)*8)
	for i, v := range values {
		util.LongToSortableBytes(v, out, i*8)
	}
	return out
}

// encodeIntRangeQueryBounds packs N-dimensional int bounds (4 bytes/dim).
func encodeIntRangeQueryBounds(values []int32) []byte {
	out := make([]byte, len(values)*4)
	for i, v := range values {
		util.IntToSortableBytes(v, out, i*4)
	}
	return out
}

// encodeDoubleRangeQueryBounds packs N-dimensional double bounds (8 bytes/dim).
func encodeDoubleRangeQueryBounds(values []float64) []byte {
	out := make([]byte, len(values)*8)
	for i, v := range values {
		util.LongToSortableBytes(util.DoubleToSortableLong(v), out, i*8)
	}
	return out
}

// encodeFloatRangeQueryBounds packs N-dimensional float bounds (4 bytes/dim).
func encodeFloatRangeQueryBounds(values []float32) []byte {
	out := make([]byte, len(values)*4)
	for i, v := range values {
		util.IntToSortableBytes(util.FloatToSortableInt(v), out, i*4)
	}
	return out
}

// rangeQueryCount runs query over reader and returns the hit count.
func rangeQueryCount(t *testing.T, reader index.IndexReaderInterface, q search.Query) int {
	t.Helper()
	searcher := search.NewIndexSearcher(reader)
	// n is bounded by the index size; the testBasics fixtures have 8 docs, so a
	// generous cap returns every hit.
	top, err := searcher.Search(q, 1000)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	return len(top.ScoreDocs)
}
