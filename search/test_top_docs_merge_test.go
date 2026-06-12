// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestTopDocsMerge.java
//
// This file ports the three TestTopDocsMerge methods that were NOT already
// covered by the merge-logic suite in top_docs_merge_test.go:
//   - testSort_1 / testSort_2  -> TestTopDocsMerge_Sort1 / _Sort2
//   - testInconsistentTopDocsFail -> TestTopDocsMerge_InconsistentTopDocsFail
//
// testPreAssignedShardIndex and testMergeTotalHitsRelation are already ported
// faithfully (and passing) in top_docs_merge_test.go as
// TestTopDocsMerge_PreAssignedShardIndex / _TotalHitsRelation, so the duplicate
// stubs that previously lived here were removed (dead duplicates).
//
// testSort builds a multi-segment index, searches the whole index with a Sort,
// searches each segment ("shard") independently, sort-merges the per-shard
// TopFieldDocs via search.MergeSort, and asserts the merged result is identical
// to the whole-index result (Lucene's TestUtil.assertConsistent). The Lucene
// original randomizes sort fields over atLeast(300) iterations; this port
// drives a deterministic, seed-fixed cross product of every sort-field type
// Lucene exercises (string/int/float asc+desc, score asc+desc, doc asc+desc)
// plus multi-key combinations, so the same machinery is covered without the
// flakiness of unbounded randomness.

package search_test

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestTopDocsMerge_InconsistentTopDocsFail ports
// TestTopDocsMerge.testInconsistentTopDocsFail: merging a mix of set and unset
// shard indices must fail.
func TestTopDocsMerge_InconsistentTopDocsFail(t *testing.T) {
	topDocs := []*search.TopDocs{
		{
			TotalHits: search.NewTotalHits(1, search.EQUAL_TO),
			ScoreDocs: []*search.ScoreDoc{search.NewScoreDoc(1, 1.0, 5)},
		},
		{
			TotalHits: search.NewTotalHits(1, search.EQUAL_TO),
			ScoreDocs: []*search.ScoreDoc{search.NewScoreDoc(1, 1.0, -1)},
		},
	}
	// As in Lucene, randomly swap the two to exercise both orderings.
	rng := rand.New(rand.NewSource(42))
	if rng.Intn(2) == 0 {
		topDocs[0], topDocs[1] = topDocs[1], topDocs[0]
	}
	if _, err := search.MergeWithStart(0, 2, topDocs); err == nil {
		t.Fatal("expected error merging mixed set/unset shard indices, got nil")
	}
}

// TestTopDocsMerge_Sort1 ports TestTopDocsMerge.testSort_1 (useFrom == false).
func TestTopDocsMerge_Sort1(t *testing.T) {
	runTopDocsMergeSort(t, false)
}

// TestTopDocsMerge_Sort2 ports TestTopDocsMerge.testSort_2 (useFrom == true).
func TestTopDocsMerge_Sort2(t *testing.T) {
	runTopDocsMergeSort(t, true)
}

// runTopDocsMergeSort is the Go port of TestTopDocsMerge.testSort(useFrom).
func runTopDocsMergeSort(t *testing.T, useFrom bool) {
	t.Helper()
	const numDocs = 120
	const seed = 987654321

	dir := store.NewByteBuffersDirectory()
	defer func() { _ = dir.Close() }()

	rng := rand.New(rand.NewSource(seed))
	tokens := []string{"a", "b", "c", "d", "e"}

	// Build content strings, each a few random tokens (so TermQuery hits vary).
	content := make([]string, 24)
	for i := range content {
		n := 1 + rng.Intn(10)
		s := ""
		for j := 0; j < n; j++ {
			s += tokens[rng.Intn(len(tokens))] + " "
		}
		content[i] = s
	}

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		// "string" SortedDocValues sort key (realistic ASCII string).
		sv := fmt.Sprintf("s%05d", rng.Intn(numDocs))
		sdv, err := document.NewSortedDocValuesField("string", []byte(sv))
		if err != nil {
			t.Fatalf("NewSortedDocValuesField: %v", err)
		}
		doc.Add(sdv)
		// "text" indexed field for TermQuery.
		tf, err := document.NewTextField("text", content[rng.Intn(len(content))], false)
		if err != nil {
			t.Fatalf("NewTextField: %v", err)
		}
		doc.Add(tf)
		// "float" FloatDocValues sort key.
		fdv, err := document.NewFloatDocValuesField("float", rng.Float32())
		if err != nil {
			t.Fatalf("NewFloatDocValuesField: %v", err)
		}
		doc.Add(fdv)
		// "int" NumericDocValues sort key (with occasional extremes).
		var iv int64
		switch rng.Intn(50) {
		case 17:
			iv = -2147483648
		case 23:
			iv = 2147483647
		default:
			iv = int64(rng.Int31())
			if rng.Intn(2) == 0 {
				iv = -iv
			}
		}
		ndv, err := document.NewNumericDocValuesField("int", iv)
		if err != nil {
			t.Fatalf("NewNumericDocValuesField: %v", err)
		}
		doc.Add(ndv)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
		// Commit periodically to force multiple segments ("shards").
		if i > 0 && i%30 == 0 {
			if err := w.Commit(); err != nil {
				t.Fatalf("Commit: %v", err)
			}
		}
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	defer func() { _ = w.Close() }()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer func() { _ = reader.Close() }()

	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}
	if len(leaves) < 2 {
		t.Fatalf("expected a multi-segment index for the shard-merge test, got %d leaves", len(leaves))
	}
	segReaders := reader.GetSegmentReaders()
	docStarts := make([]int, len(segReaders))
	docBase := 0
	for i, sr := range segReaders {
		docStarts[i] = docBase
		docBase += sr.MaxDoc()
	}

	searcher := search.NewIndexSearcher(reader)

	// Every SortField the Lucene test draws from.
	sortFields := []*search.SortField{
		search.NewSortFieldReverse("string", search.SortFieldTypeString),
		search.NewSortField("string", search.SortFieldTypeString),
		search.NewSortFieldReverse("int", search.SortFieldTypeInt),
		search.NewSortField("int", search.SortFieldTypeInt),
		search.NewSortFieldReverse("float", search.SortFieldTypeFloat),
		search.NewSortField("float", search.SortFieldTypeFloat),
		search.NewSortFieldReverse("", search.SortFieldTypeScore),
		search.NewSortField("", search.SortFieldTypeScore),
		search.NewSortFieldReverse("", search.SortFieldTypeDoc),
		search.NewSortField("", search.SortFieldTypeDoc),
	}

	// Deterministic set of sort configurations: each single field, plus a set
	// of multi-key combinations (string then int, int then float, etc.).
	var sorts []*search.Sort
	for _, sf := range sortFields {
		sorts = append(sorts, search.NewSort(sf))
	}
	multiKey := [][]int{
		{0, 3}, {2, 5}, {1, 6}, {4, 8}, {0, 3, 5}, {1, 2, 9},
	}
	for _, combo := range multiKey {
		fields := make([]*search.SortField, len(combo))
		for k, idx := range combo {
			fields[k] = sortFields[idx]
		}
		sorts = append(sorts, search.NewSort(fields...))
	}

	iter := 0
	for _, tok := range tokens {
		for _, sort := range sorts {
			iter++
			query := search.NewTermQuery(index.NewTerm("text", tok))
			numHits := 1 + (iter % numDocs)

			// Whole-index sorted search.
			whole, err := searcher.SearchWithSort(query, numHits, sort)
			if err != nil {
				t.Fatalf("iter %d SearchWithSort(whole): %v", iter, err)
			}

			// Per-shard search: each segment as an independent shard.
			shardHits := make([]*search.TopFieldDocs, len(segReaders))
			for s, sr := range segReaders {
				ss := search.NewIndexSearcher(sr)
				sub, err := ss.SearchWithSort(query, numHits, sort)
				if err != nil {
					t.Fatalf("iter %d shard %d SearchWithSort: %v", iter, s, err)
				}
				// Rebase shard-local docIDs to the global doc space and stamp
				// the shard index (Lucene sets sd.shardIndex = shardIDX). A
				// single-segment shard searcher uses docBase=0, so DOC-type sort
				// values are shard-local; rebase those to global too, matching
				// the whole-index DOC sort (which sees global doc ids). In
				// Lucene the ShardSearcher's DOC comparator is bound to the
				// parent composite context, so its DOC values are already global.
				for _, fd := range sub.FieldDocs {
					for k, sfld := range sort.Fields {
						if sfld.Type == search.SortFieldTypeDoc {
							if dv, ok := fd.Fields[k].(int); ok {
								fd.Fields[k] = dv + docStarts[s]
							}
						}
					}
					fd.Doc += docStarts[s]
					fd.ShardIndex = s
				}
				shardHits[s] = sub
			}

			var merged *search.TopFieldDocs
			if useFrom {
				from := iter % numHits
				size := numHits - from
				merged, err = search.MergeSort(sort, from, size, shardHits)
				if err != nil {
					t.Fatalf("iter %d MergeSort(from=%d,size=%d): %v", iter, from, size, err)
				}
				whole = topFieldDocsWindow(whole, from, size)
			} else {
				merged, err = search.MergeSort(sort, 0, numHits, shardHits)
				if err != nil {
					t.Fatalf("iter %d MergeSort: %v", iter, err)
				}
			}

			// Every merged hit must be attributed to the shard owning its docID.
			for _, fd := range merged.FieldDocs {
				wantShard := subIndex(fd.Doc, docStarts)
				if fd.ShardIndex != wantShard {
					t.Fatalf("iter %d doc=%d wrong shard: got %d want %d", iter, fd.Doc, fd.ShardIndex, wantShard)
				}
			}

			assertTopFieldDocsConsistent(t, iter, sort, whole, merged)
		}
	}
}

// topFieldDocsWindow mirrors the from/size slicing the Lucene test applies to
// the whole-index hits before comparing against the merged hits.
func topFieldDocsWindow(td *search.TopFieldDocs, from, size int) *search.TopFieldDocs {
	if from >= len(td.FieldDocs) {
		return search.NewTopFieldDocsWithFieldDocs(td.TotalHits, nil, td.Fields)
	}
	end := from + size
	if end > len(td.FieldDocs) {
		end = len(td.FieldDocs)
	}
	return search.NewTopFieldDocsWithFieldDocs(td.TotalHits, td.FieldDocs[from:end], td.Fields)
}

// subIndex mirrors org.apache.lucene.index.ReaderUtil.subIndex: the segment
// owning the given global docID, given the per-segment doc starts.
func subIndex(doc int, docStarts []int) int {
	idx := 0
	for i := 1; i < len(docStarts); i++ {
		if doc < docStarts[i] {
			break
		}
		idx = i
	}
	return idx
}

// assertTopFieldDocsConsistent mirrors TestUtil.assertConsistent for the
// field-sorted case: same total-hit relation/value, same hit count, and for
// every hit the same docID and sort field values.
func assertTopFieldDocsConsistent(t *testing.T, iter int, sort *search.Sort, expected, actual *search.TopFieldDocs) {
	t.Helper()
	if (expected.TotalHits.Value == 0) != (actual.TotalHits.Value == 0) {
		t.Fatalf("iter %d wrong total hits zero-ness: expected %d actual %d", iter, expected.TotalHits.Value, actual.TotalHits.Value)
	}
	if expected.TotalHits.Relation == search.EQUAL_TO && actual.TotalHits.Relation == search.EQUAL_TO {
		if expected.TotalHits.Value != actual.TotalHits.Value {
			t.Fatalf("iter %d wrong total hits: expected %d actual %d", iter, expected.TotalHits.Value, actual.TotalHits.Value)
		}
	}
	if len(expected.FieldDocs) != len(actual.FieldDocs) {
		t.Fatalf("iter %d wrong hit count: expected %d actual %d", iter, len(expected.FieldDocs), len(actual.FieldDocs))
	}
	for i := range expected.FieldDocs {
		e := expected.FieldDocs[i]
		a := actual.FieldDocs[i]
		if e.Doc != a.Doc {
			t.Fatalf("iter %d hit %d wrong docID: expected %d actual %d", iter, i, e.Doc, a.Doc)
		}
		if e.Score != a.Score {
			t.Fatalf("iter %d hit %d wrong score: expected %v actual %v", iter, i, e.Score, a.Score)
		}
		if !reflect.DeepEqual(e.Fields, a.Fields) {
			t.Fatalf("iter %d hit %d wrong sort field values: expected %v actual %v", iter, i, e.Fields, a.Fields)
		}
	}
}
