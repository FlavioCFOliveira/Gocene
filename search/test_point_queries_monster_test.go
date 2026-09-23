// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_monsters

// The @Nightly test port of
// lucene/core/src/test/org/apache/lucene/search/TestPointQueries.java
// (Apache Lucene 10.5.0); the gocene_monsters build tag renders the
// tests.nightly switch.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/util/bkd"
)

func TestPointQueriesInversePointRange(t *testing.T) {
	pointQueriesBeforeClass()
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	numDims := nextInt(1, 3)
	// we need multiple leaves to enable this optimization
	numDocs := atLeast(10 * bkd.DefaultMaxPointsInLeafNode)
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		values := make([]int32, numDims)
		for d := range values {
			values[d] = int32(i)
		}
		doc.Add(document.NewIntPoint("f", values...))
		mustAddDocument(t, w, doc)
	}
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	r := mustOpenDirectoryReaderFromWriter(t, w)
	mustClose(t, w)

	searcher := newSearcher(t, r)
	low := make([]int32, numDims)
	high := make([]int32, numDims)
	fill := func(a []int32, v int) {
		for i := range a {
			a[i] = int32(v)
		}
	}
	fill(high, numDocs-2)
	assertCount(t, searcher, intPointNewRangeQueryDims(t, "f", low, high), int(high[0]-low[0]+1))
	fill(low, 1)
	assertCount(t, searcher, intPointNewRangeQueryDims(t, "f", low, high), int(high[0]-low[0]+1))
	fill(high, numDocs-1)
	assertCount(t, searcher, intPointNewRangeQueryDims(t, "f", low, high), int(high[0]-low[0]+1))
	fill(low, bkd.DefaultMaxPointsInLeafNode+1)
	assertCount(t, searcher, intPointNewRangeQueryDims(t, "f", low, high), int(high[0]-low[0]+1))
	fill(high, numDocs-bkd.DefaultMaxPointsInLeafNode)
	assertCount(t, searcher, intPointNewRangeQueryDims(t, "f", low, high), int(high[0]-low[0]+1))

	mustClose(t, r, dir)
}
