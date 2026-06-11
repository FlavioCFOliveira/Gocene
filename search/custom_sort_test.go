// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

// GC-938: Custom Sort Tests
// Validate sort-by-DocValues produces the same ordering as Java Lucene.
//
// IndexSearcher.SearchWithSort drives TopFieldCollector + the FieldComparator
// chain (rmp #11). These tests index NumericDocValues fields and assert the
// returned doc order for ascending, descending, and multi-field (primary +
// tie-break) sorts.

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"

	// Register the production codec so DocValues are actually flushed.
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

// csDoc is one document: a primary "value" numeric DocValues field and an
// optional secondary "group" numeric DocValues field.
type csDoc struct {
	value    int64
	hasGroup bool
	group    int64
}

// buildCustomSortIndex indexes docs (added in order, so docID == index) and
// returns a searcher plus cleanup. ForceMerge is intentionally avoided.
func buildCustomSortIndex(t testing.TB, docs []csDoc) (*search.IndexSearcher, func()) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i, d := range docs {
		doc := document.NewDocument()
		vf, err := document.NewNumericDocValuesField("value", d.value)
		if err != nil {
			t.Fatalf("doc %d value: %v", i, err)
		}
		doc.Add(vf)
		if d.hasGroup {
			gf, err := document.NewNumericDocValuesField("group", d.group)
			if err != nil {
				t.Fatalf("doc %d group: %v", i, err)
			}
			doc.Add(gf)
		}
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("doc %d AddDocument: %v", i, err)
		}
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	return search.NewIndexSearcher(reader), func() {
		reader.Close()
		dir.Close()
	}
}

func csDocOrder(td *search.TopFieldDocs) []int {
	out := make([]int, len(td.ScoreDocs))
	for i, sd := range td.ScoreDocs {
		out[i] = sd.Doc
	}
	return out
}

func csEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestCustomSort_BasicSort(t *testing.T) {
	// docID: 0->50, 1->10, 2->30, 3->20, 4->40
	s, cleanup := buildCustomSortIndex(t, []csDoc{
		{value: 50}, {value: 10}, {value: 30}, {value: 20}, {value: 40},
	})
	defer cleanup()

	sort := search.NewSort(search.NewSortField("value", search.SortFieldTypeLong))
	td, err := s.SearchWithSort(search.NewMatchAllDocsQuery(), 10, sort)
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}
	want := []int{1, 3, 2, 4, 0} // 10,20,30,40,50
	if got := csDocOrder(td); !csEqual(got, want) {
		t.Fatalf("ascending order = %v, want %v", got, want)
	}
}

func TestCustomSort_DescendingSort(t *testing.T) {
	s, cleanup := buildCustomSortIndex(t, []csDoc{
		{value: 50}, {value: 10}, {value: 30}, {value: 20}, {value: 40},
	})
	defer cleanup()

	sort := search.NewSort(search.NewSortFieldReverse("value", search.SortFieldTypeLong))
	td, err := s.SearchWithSort(search.NewMatchAllDocsQuery(), 10, sort)
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}
	want := []int{0, 4, 2, 3, 1} // 50,40,30,20,10
	if got := csDocOrder(td); !csEqual(got, want) {
		t.Fatalf("descending order = %v, want %v", got, want)
	}
}

func TestCustomSort_MultiFieldSort(t *testing.T) {
	// (group, value): primary group asc, secondary value asc.
	//   doc0: g2 v5   doc1: g1 v9   doc2: g2 v1   doc3: g1 v3   doc4: g1 v7
	// group1 by value: 3(3),4(7),1(9); group2 by value: 2(1),0(5)
	s, cleanup := buildCustomSortIndex(t, []csDoc{
		{value: 5, hasGroup: true, group: 2},
		{value: 9, hasGroup: true, group: 1},
		{value: 1, hasGroup: true, group: 2},
		{value: 3, hasGroup: true, group: 1},
		{value: 7, hasGroup: true, group: 1},
	})
	defer cleanup()

	sort := search.NewSort(
		search.NewSortField("group", search.SortFieldTypeLong),
		search.NewSortField("value", search.SortFieldTypeLong),
	)
	td, err := s.SearchWithSort(search.NewMatchAllDocsQuery(), 10, sort)
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}
	want := []int{3, 4, 1, 2, 0}
	if got := csDocOrder(td); !csEqual(got, want) {
		t.Fatalf("multi-field order = %v, want %v", got, want)
	}
}

func BenchmarkCustomSort_Performance(b *testing.B) {
	docs := make([]csDoc, 1000)
	for i := range docs {
		docs[i] = csDoc{value: int64((i * 7919) % 1000)}
	}
	s, cleanup := buildCustomSortIndex(b, docs)
	defer cleanup()
	sort := search.NewSort(search.NewSortField("value", search.SortFieldTypeLong))
	q := search.NewMatchAllDocsQuery()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.SearchWithSort(q, 10, sort); err != nil {
			b.Fatalf("SearchWithSort: %v", err)
		}
}	}
