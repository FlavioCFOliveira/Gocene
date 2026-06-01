// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSortRandom.java
//
// testRandomStringSort indexes documents with a random "stringdv"
// SortedDocValues value (about 10% missing), then for many iterations searches
// with a STRING sort over a random reverse / missing-first-or-last
// configuration and asserts the returned per-hit sort values match the
// independently-computed expected ordering. The Lucene original uses a custom
// RandomQuery to match a random subset; since the assertion is purely about the
// STRING comparator's ordering (including missing-value placement), this port
// uses MatchAllDocsQuery — the full document set, a strict superset that
// exercises the same comparator and missing-value logic. Seeds are fixed so the
// run is deterministic.

package search_test

import (
	"bytes"
	"fmt"
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

// TestSortRandom_RandomStringSort ports TestSortRandom.testRandomStringSort.
func TestSortRandom_RandomStringSort(t *testing.T) {
	const seed = 0xC0FFEE
	rng := rand.New(rand.NewSource(seed))

	const numDocs = 150
	dir := store.NewByteBuffersDirectory()
	defer func() { _ = dir.Close() }()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// docValues[i] is the stringdv value of doc i (nil == missing). seen
	// enforces uniqueness so the expected ordering is unambiguous.
	docValues := make([][]byte, 0, numDocs)
	seen := map[string]bool{}
	for len(docValues) < numDocs {
		doc := document.NewDocument()
		var br []byte
		if rng.Intn(10) != 7 { // ~90% have a value
			s := fmt.Sprintf("v%08d", rng.Intn(numDocs*4))
			if seen[s] {
				continue
			}
			seen[s] = true
			br = []byte(s)
			sdv, err := document.NewSortedDocValuesField("stringdv", br)
			if err != nil {
				t.Fatalf("NewSortedDocValuesField: %v", err)
			}
			doc.Add(sdv)
		}
		docValues = append(docValues, br)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
		if rng.Intn(40) == 17 {
			if err := w.Commit(); err != nil { // force a new segment
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
	searcher := search.NewIndexSearcher(reader)

	const iters = 100
	for iter := 0; iter < iters; iter++ {
		reverse := rng.Intn(2) == 0
		sortMissingLast := rng.Intn(2) == 0

		sf := search.NewSortField("stringdv", search.SortFieldTypeString)
		sf.Reverse = reverse
		if sortMissingLast {
			sf.SetMissingValue(search.STRING_LAST)
		} else {
			sf.SetMissingValue(search.STRING_FIRST)
		}
		sortObj := search.NewSort(sf)

		hitCount := 1 + rng.Intn(reader.MaxDoc()+20)

		hits, err := searcher.SearchWithSort(search.NewMatchAllDocsQuery(), hitCount, sortObj)
		if err != nil {
			t.Fatalf("iter %d SearchWithSort: %v", iter, err)
		}

		// Expected: sort the doc values, missing first/last per config, then
		// reverse if requested.
		expected := make([][]byte, len(docValues))
		copy(expected, docValues)
		sort.SliceStable(expected, func(i, j int) bool {
			a, b := expected[i], expected[j]
			if a == nil {
				if b == nil {
					return false
				}
				return !sortMissingLast // missing first => a before b
			}
			if b == nil {
				return sortMissingLast // missing last => a (present) before b (missing)
			}
			return bytes.Compare(a, b) < 0
		})
		if reverse {
			for i, j := 0, len(expected)-1; i < j; i, j = i+1, j-1 {
				expected[i], expected[j] = expected[j], expected[i]
			}
		}

		want := hitCount
		if want > len(expected) {
			want = len(expected)
		}
		if len(hits.FieldDocs) != want {
			t.Fatalf("iter %d hit count: got %d want %d", iter, len(hits.FieldDocs), want)
		}
		for i := 0; i < want; i++ {
			gotVal, _ := hits.FieldDocs[i].Fields[0].([]byte)
			expVal := expected[i]
			if !bytes.Equal(gotVal, expVal) {
				t.Fatalf("iter %d hit %d (reverse=%v missingLast=%v): got %q want %q",
					iter, i, reverse, sortMissingLast, valStr(gotVal), valStr(expVal))
			}
		}
	}
}

func valStr(b []byte) string {
	if b == nil {
		return "<missing>"
	}
	return string(b)
}
