// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/search/TestMultiSliceMerge.java
//
// This test builds two independent indexes carrying identical logical content,
// searches each with MatchAllDocsQuery, and asserts the two hit lists are equal
// (testMultipleSlicesOfSameIndexSearcher). The second case stamps the two hit
// lists with distinct shard indices, merges them twice via TopDocs.merge, and
// asserts the two merges agree (testMultipleSlicesOfMultipleIndexSearchers).
//
// Deviation: Gocene's IndexSearcher has no Executor/slice concept, so the
// upstream "multiple slices of the same searcher" is reproduced as two searchers
// over two equal-content indexes — the property under test (slice-merge
// determinism and equality) is identical. The upstream Integer.MAX_VALUE hit cap
// is replaced with 200 (> the 100-document corpus) so every hit is still
// returned, while avoiding Gocene's TopDocs priority queue pre-allocating its
// full nominal capacity.
package search_test

import (
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/search/testutil"
)

// buildMultiSliceIndex builds a 100-document index where each document carries a
// "field" StringField holding its ordinal and a "field2" StringField holding the
// parity of its ordinal, matching the upstream corpus (minus the doc-values copy
// of field2, which the queried MatchAllDocsQuery does not consult).
func buildMultiSliceIndex(t *testing.T) (*search.IndexSearcher, func()) {
	t.Helper()
	ix := newIntegrationIndex(t)
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		f, err := document.NewStringField("field", strconv.Itoa(i), false)
		if err != nil {
			t.Fatalf("NewStringField(field): %v", err)
		}
		doc.Add(f)
		f2, err := document.NewStringField("field2", strconv.FormatBool(i%2 == 0), false)
		if err != nil {
			t.Fatalf("NewStringField(field2): %v", err)
		}
		doc.Add(f2)
		ix.addDoc(doc)
		if i%7 == 0 {
			ix.commit() // multiple segments, as the upstream LogMergePolicy produces
		}
	}
	return ix.searcher()
}

// TestMultiSliceMerge_MultipleSlicesOfSameIndexSearcher mirrors
// testMultipleSlicesOfSameIndexSearcher.
func TestMultiSliceMerge_MultipleSlicesOfSameIndexSearcher(t *testing.T) {
	searcher1, cleanup1 := buildMultiSliceIndex(t)
	defer cleanup1()
	searcher2, cleanup2 := buildMultiSliceIndex(t)
	defer cleanup2()

	query := search.NewMatchAllDocsQuery()

	topDocs1, err := searcher1.Search(query, 200)
	if err != nil {
		t.Fatalf("searcher1.Search: %v", err)
	}
	topDocs2, err := searcher2.Search(query, 200)
	if err != nil {
		t.Fatalf("searcher2.Search: %v", err)
	}

	testutil.CheckEqual(t, query, topDocs1.ScoreDocs, topDocs2.ScoreDocs)
}

// TestMultiSliceMerge_MultipleSlicesOfMultipleIndexSearchers mirrors
// testMultipleSlicesOfMultipleIndexSearchers.
func TestMultiSliceMerge_MultipleSlicesOfMultipleIndexSearchers(t *testing.T) {
	searcher1, cleanup1 := buildMultiSliceIndex(t)
	defer cleanup1()
	searcher2, cleanup2 := buildMultiSliceIndex(t)
	defer cleanup2()

	query := search.NewMatchAllDocsQuery()

	topDocs1, err := searcher1.Search(query, 200)
	if err != nil {
		t.Fatalf("searcher1.Search: %v", err)
	}
	topDocs2, err := searcher2.Search(query, 200)
	if err != nil {
		t.Fatalf("searcher2.Search: %v", err)
	}

	if len(topDocs1.ScoreDocs) != len(topDocs2.ScoreDocs) {
		t.Fatalf("scoreDocs length %d != %d", len(topDocs1.ScoreDocs), len(topDocs2.ScoreDocs))
	}

	for i := range topDocs1.ScoreDocs {
		topDocs1.ScoreDocs[i].ShardIndex = 0
		topDocs2.ScoreDocs[i].ShardIndex = 1
	}

	shardHits := []*search.TopDocs{topDocs1, topDocs2}

	mergedHits1, err := search.MergeWithStart(0, len(topDocs1.ScoreDocs), shardHits)
	if err != nil {
		t.Fatalf("MergeWithStart(1): %v", err)
	}
	mergedHits2, err := search.MergeWithStart(0, len(topDocs1.ScoreDocs), shardHits)
	if err != nil {
		t.Fatalf("MergeWithStart(2): %v", err)
	}

	testutil.CheckEqual(t, query, mergedHits1.ScoreDocs, mergedHits2.ScoreDocs)
}