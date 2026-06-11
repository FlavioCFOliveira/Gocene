// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// rmp #4778 acceptance test: an end-to-end field-sorted search over DocValues.
// Documents carrying NumericDocValuesField (int/long) and SortedDocValuesField
// are written through IndexWriter, committed, reopened via OpenDirectoryReader,
// and searched with IndexSearcher.SearchWithSort. The returned doc order must
// match the DocValues order (ascending, descending, missing-value placement,
// secondary sort, and docID tie-break) and the per-hit FieldDoc must carry the
// sort values.
//
// Reference: org.apache.lucene.search.IndexSearcher#search(Query,int,Sort),
// TopFieldCollector, FieldComparator (Int/Long/TermOrdVal).
package search_test

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"

	// Blank-import the codecs so the production Lucene104 codec is registered as
	// the default; without it the writer flushes no DocValues (see rmp #4771 /
	// webapi persistence note).
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

// fsDoc describes one document for the field-sort tests. A field is omitted from
// the document (left missing) when its has-flag is false.
type fsDoc struct {
	hasInt  bool
	intVal  int64
	hasLong bool
	longVal int64
	hasTerm bool
	term    string
}

// buildFieldSortIndex indexes the docs (one segment, force-merged) and returns
// the reader. Numeric values are stored in a NumericDocValuesField; the term in
// a SortedDocValuesField. Docs are added in input order so docID == index.
func buildFieldSortIndex(t *testing.T, docs []fsDoc) (*index.DirectoryReader, store.Directory) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i, d := range docs {
		doc := document.NewDocument()
		if d.hasInt {
			f, err := document.NewNumericDocValuesField("intf", d.intVal)
			if err != nil {
				t.Fatalf("doc %d NewNumericDocValuesField(intf): %v", i, err)
			}
			doc.Add(f)
		}
		if d.hasLong {
			f, err := document.NewNumericDocValuesField("longf", d.longVal)
			if err != nil {
				t.Fatalf("doc %d NewNumericDocValuesField(longf): %v", i, err)
			}
			doc.Add(f)
		}
		if d.hasTerm {
			f, err := document.NewSortedDocValuesField("termf", []byte(d.term))
			if err != nil {
				t.Fatalf("doc %d NewSortedDocValuesField(termf): %v", i, err)
			}
			doc.Add(f)
		}
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("doc %d AddDocument: %v", i, err)
		}
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge(1): %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	return reader, dir
}

func docOrder(td *search.TopFieldDocs) []int {
	out := make([]int, len(td.ScoreDocs))
	for i, sd := range td.ScoreDocs {
		out[i] = sd.Doc
	}
	return out
}

// equalInts is provided by multi_term_runnable_test.go in this test package.

// TestFieldSortedSearch_IntAscending sorts by an int DocValues field ascending.
func TestFieldSortedSearch_IntAscending(t *testing.T) {
	// docID: 0->50, 1->10, 2->30, 3->20, 4->40
	reader, _ := buildFieldSortIndex(t, []fsDoc{
		{hasInt: true, intVal: 50},
		{hasInt: true, intVal: 10},
		{hasInt: true, intVal: 30},
		{hasInt: true, intVal: 20},
		{hasInt: true, intVal: 40},
	})
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	sort := search.NewSort(search.NewSortField("intf", search.SortFieldTypeInt))
	td, err := searcher.SearchWithSort(search.NewMatchAllDocsQuery(), 10, sort)
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}

	want := []int{1, 3, 2, 4, 0} // values 10,20,30,40,50
	if got := docOrder(td); !equalInts(got, want) {
		t.Fatalf("ascending order = %v, want %v", got, want)
	}
	if td.TotalHits.Value != 5 {
		t.Fatalf("TotalHits = %d, want 5", td.TotalHits.Value)
	}
	// FieldDoc sort values: first hit (doc 1) must carry int32(10).
	fd := td.FieldDocs[0]
	if fd.Doc != 1 {
		t.Fatalf("FieldDocs[0].Doc = %d, want 1", fd.Doc)
	}
	if len(fd.Fields) != 1 {
		t.Fatalf("FieldDoc.Fields len = %d, want 1", len(fd.Fields))
	}
	if v, ok := fd.Fields[0].(int32); !ok || v != 10 {
		t.Fatalf("FieldDoc.Fields[0] = %v (%T), want int32(10)", fd.Fields[0], fd.Fields[0])
	}
}

// TestFieldSortedSearch_IntDescending sorts by the same int field descending.
func TestFieldSortedSearch_IntDescending(t *testing.T) {
	reader, _ := buildFieldSortIndex(t, []fsDoc{
		{hasInt: true, intVal: 50},
		{hasInt: true, intVal: 10},
		{hasInt: true, intVal: 30},
		{hasInt: true, intVal: 20},
		{hasInt: true, intVal: 40},
	})
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	sort := search.NewSort(search.NewSortFieldReverse("intf", search.SortFieldTypeInt))
	td, err := searcher.SearchWithSort(search.NewMatchAllDocsQuery(), 10, sort)
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}

	want := []int{0, 4, 2, 3, 1} // values 50,40,30,20,10
	if got := docOrder(td); !equalInts(got, want) {
		t.Fatalf("descending order = %v, want %v", got, want)
	}
}

// TestFieldSortedSearch_TopN keeps only the top-N when N < total, exercising the
// competitive priority-queue eviction path.
func TestFieldSortedSearch_TopN(t *testing.T) {
	reader, _ := buildFieldSortIndex(t, []fsDoc{
		{hasInt: true, intVal: 50},
		{hasInt: true, intVal: 10},
		{hasInt: true, intVal: 30},
		{hasInt: true, intVal: 20},
		{hasInt: true, intVal: 40},
	})
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	sort := search.NewSort(search.NewSortField("intf", search.SortFieldTypeInt))
	td, err := searcher.SearchWithSort(search.NewMatchAllDocsQuery(), 3, sort)
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}
	want := []int{1, 3, 2} // smallest three: 10,20,30
	if got := docOrder(td); !equalInts(got, want) {
		t.Fatalf("top-3 ascending order = %v, want %v", got, want)
	}
	if td.TotalHits.Value != 5 {
		t.Fatalf("TotalHits = %d, want 5 (all matches counted)", td.TotalHits.Value)
	}
}

// TestFieldSortedSearch_LongMissingLast leaves some docs without a value; with
// the default missing strategy they sort after present values (ascending).
func TestFieldSortedSearch_LongMissingLast(t *testing.T) {
	// docID: 0->100, 1->(missing), 2->5, 3->(missing), 4->50
	reader, _ := buildFieldSortIndex(t, []fsDoc{
		{hasLong: true, longVal: 100},
		{},
		{hasLong: true, longVal: 5},
		{},
		{hasLong: true, longVal: 50},
	})
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	sf := search.NewSortField("longf", search.SortFieldTypeLong)
	// Missing defaults to MissingValueLast, but for a numeric field the
	// substituted missing value is 0; with 0 < every present value the missing
	// docs would actually sort first. Set an explicit large missing value so the
	// missing docs sort last, as the test name asserts.
	sf.SetMissingValue(int64(1 << 62))
	td, err := searcher.SearchWithSort(search.NewMatchAllDocsQuery(), 10, search.NewSort(sf))
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}
	// Present values ascending: 5(doc2),50(doc4),100(doc0); then missing docs
	// 1 and 3 in docID order (tie-break).
	want := []int{2, 4, 0, 1, 3}
	if got := docOrder(td); !equalInts(got, want) {
		t.Fatalf("missing-last order = %v, want %v", got, want)
	}
}

// TestFieldSortedSearch_NumericMissingDefaultZero documents the Lucene default:
// a missing numeric value is substituted with 0, so missing docs sort with the
// zeros (here, before any positive value, ascending).
func TestFieldSortedSearch_NumericMissingDefaultZero(t *testing.T) {
	// docID: 0->100, 1->(missing=0), 2->5, 3->(missing=0), 4->50
	reader, _ := buildFieldSortIndex(t, []fsDoc{
		{hasLong: true, longVal: 100},
		{},
		{hasLong: true, longVal: 5},
		{},
		{hasLong: true, longVal: 50},
	})
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	sf := search.NewSortField("longf", search.SortFieldTypeLong) // missing -> 0
	td, err := searcher.SearchWithSort(search.NewMatchAllDocsQuery(), 10, search.NewSort(sf))
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}
	// 0(doc1),0(doc3) tie-break by docID, then 5(doc2),50(doc4),100(doc0).
	want := []int{1, 3, 2, 4, 0}
	if got := docOrder(td); !equalInts(got, want) {
		t.Fatalf("missing-default-zero order = %v, want %v", got, want)
	}
}

// TestFieldSortedSearch_StringSorted sorts by a SortedDocValues (term) field.
func TestFieldSortedSearch_StringSorted(t *testing.T) {
	// docID: 0->"delta", 1->"alpha", 2->"charlie", 3->"bravo", 4->"echo"
	reader, _ := buildFieldSortIndex(t, []fsDoc{
		{hasTerm: true, term: "delta"},
		{hasTerm: true, term: "alpha"},
		{hasTerm: true, term: "charlie"},
		{hasTerm: true, term: "bravo"},
		{hasTerm: true, term: "echo"},
	})
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	td, err := searcher.SearchWithSort(
		search.NewMatchAllDocsQuery(), 10,
		search.NewSort(search.NewSortField("termf", search.SortFieldTypeString)),
	)
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}
	// Lexicographic: alpha(1),bravo(3),charlie(2),delta(0),echo(4).
	want := []int{1, 3, 2, 0, 4}
	if got := docOrder(td); !equalInts(got, want) {
		t.Fatalf("string-sort order = %v, want %v", got, want)
	}
	// FieldDoc must carry the term bytes for the first hit (doc 1 -> "alpha").
	fd := td.FieldDocs[0]
	b, ok := fd.Fields[0].([]byte)
	if !ok || !bytes.Equal(b, []byte("alpha")) {
		t.Fatalf("FieldDoc.Fields[0] = %v (%T), want []byte(\"alpha\")", fd.Fields[0], fd.Fields[0])
	}
}

// TestFieldSortedSearch_StringMissingFirst places docs without a term value
// before all present terms.
func TestFieldSortedSearch_StringMissingFirst(t *testing.T) {
	// docID: 0->"delta", 1->(missing), 2->"alpha", 3->(missing), 4->"bravo"
	reader, _ := buildFieldSortIndex(t, []fsDoc{
		{hasTerm: true, term: "delta"},
		{},
		{hasTerm: true, term: "alpha"},
		{},
		{hasTerm: true, term: "bravo"},
	})
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	sf := search.NewSortField("termf", search.SortFieldTypeString)
	sf.SetMissingValue(search.STRING_FIRST)
	td, err := searcher.SearchWithSort(search.NewMatchAllDocsQuery(), 10, search.NewSort(sf))
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}
	// Missing first (docs 1,3 by docID), then alpha(2),bravo(4),delta(0).
	want := []int{1, 3, 2, 4, 0}
	if got := docOrder(td); !equalInts(got, want) {
		t.Fatalf("string missing-first order = %v, want %v", got, want)
	}

// TestFieldSortedSearch_SecondaryAndTieBreak exercises a two-key sort (primary
// int ascending, secondary term ascending) plus the implicit docID tie-break.
func TestFieldSortedSearch_SecondaryAndTieBreak(t *testing.T) {
	// Primary intf, secondary termf:
	//   doc0: int=1 term="b"
	//   doc1: int=1 term="a"
	//   doc2: int=2 term="z"
	//   doc3: int=1 term="a"   (full tie with doc1 -> docID tie-break)
	//   doc4: int=2 term="a"
	reader, _ := buildFieldSortIndex(t, []fsDoc{
		{hasInt: true, intVal: 1, hasTerm: true, term: "b"},
		{hasInt: true, intVal: 1, hasTerm: true, term: "a"},
		{hasInt: true, intVal: 2, hasTerm: true, term: "z"},
		{hasInt: true, intVal: 1, hasTerm: true, term: "a"},
		{hasInt: true, intVal: 2, hasTerm: true, term: "a"},
	})
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	sort := search.NewSort(
		search.NewSortField("intf", search.SortFieldTypeInt),
		search.NewSortField("termf", search.SortFieldTypeString),
	)
	td, err := searcher.SearchWithSort(search.NewMatchAllDocsQuery(), 10, sort)
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}
	// Ordering:
	//   int=1: a(doc1), a(doc3) [docID tie], b(doc0)
	//   int=2: a(doc4), z(doc2)
	want := []int{1, 3, 0, 4, 2}
	if got := docOrder(td); !equalInts(got, want) {
		t.Fatalf("secondary+tiebreak order = %v, want %v", got, want)
	}
	// First hit's FieldDoc carries both sort values: int32(1) then []byte("a").
	fd := td.FieldDocs[0]
	if len(fd.Fields) != 2 {
		t.Fatalf("FieldDoc.Fields len = %d, want 2", len(fd.Fields))
	}
	if v, ok := fd.Fields[0].(int32); !ok || v != 1 {
		t.Fatalf("FieldDoc.Fields[0] = %v, want int32(1)", fd.Fields[0])
	}
	if b, ok := fd.Fields[1].([]byte); !ok || !bytes.Equal(b, []byte("a")) {
		t.Fatalf("FieldDoc.Fields[1] = %v, want []byte(\"a\")", fd.Fields[1])
	}
}