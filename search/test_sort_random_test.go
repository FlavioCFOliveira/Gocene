// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSortRandom.java
//
// TestSortRandom_RandomNumericSort indexes documents with random long
// DocValues, then for several iterations searches with a LONG sort over
// random reverse / missing-value configurations and verifies the returned
// order matches the independently-computed expected ordering. Seeds are
// fixed so the run is deterministic.
//
// The original testRandomStringSort is deferred until the codec supports
// SORTED doc values (currently rejects SORTED type at flush time).

package search_test

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestSortRandom_RandomStringSort exercises random LONG sorts over a fixed
// set of present-only doc values. STRING sorts deferred until SORTED doc
// values are supported by the codec.
func TestSortRandom_RandomStringSort(t *testing.T) {
	const seed = 0xC0FFEE
	rng := rand.New(rand.NewSource(seed))

	const numDocs = 50
	dir := store.NewByteBuffersDirectory()
	defer func() { _ = dir.Close() }()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	values := make([]int64, numDocs)
	for i := 0; i < numDocs; i++ {
		v := rng.Int63n(10000)
		values[i] = v
		doc := document.NewDocument()
		f, err := document.NewNumericDocValuesField("longdv", v)
		if err != nil {
			t.Fatalf("NewNumericDocValuesField: %v", err)
		}
		doc.Add(f)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
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
	searcher := search.NewIndexSearcher(reader)

	const iters = 20
	for iter := 0; iter < iters; iter++ {
		reverse := rng.Intn(2) == 0

		sf := search.NewSortField("longdv", search.SortFieldTypeLong)
		sf.Reverse = reverse
		sortObj := search.NewSort(sf)

		hitCount := 1 + rng.Intn(numDocs)

		hits, err := searcher.SearchWithSort(search.NewMatchAllDocsQuery(), hitCount, sortObj)
		if err != nil {
			t.Fatalf("iter %d SearchWithSort: %v", iter, err)
		}

		expected := make([]int64, numDocs)
		copy(expected, values)
		if reverse {
			sort.Slice(expected, func(i, j int) bool { return expected[i] > expected[j] })
		} else {
			sort.Slice(expected, func(i, j int) bool { return expected[i] < expected[j] })
		}

		want := hitCount
		if want > len(expected) {
			want = len(expected)
		}
		if len(hits.FieldDocs) != want {
			t.Fatalf("iter %d hit count: got %d want %d", iter, len(hits.FieldDocs), want)
		}
		for i := 0; i < want; i++ {
			gotVal := toInt64Any(hits.FieldDocs[i].Fields[0])
			expVal := expected[i]
			if gotVal != expVal {
				t.Fatalf("iter %d hit %d (reverse=%v): got %d want %d",
					iter, i, reverse, gotVal, expVal)
			}
		}
	}
}
