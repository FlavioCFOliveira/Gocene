// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Port of
// lucene/join/src/test/org/apache/lucene/search/join/TestBlockJoinSorting.java
// (Apache Lucene 10.5.0).

// sortingChild renders the child document of testNestedSorting: field2 as a
// StringField and a SortedDocValuesField, and filter_1.
func sortingChild(t testing.TB, field2, filter1 string) *document.Document {
	t.Helper()
	return newTestDocument(
		newStringField(t, "field2", field2, false),
		mustSortedDVField(t, "field2", field2),
		newStringField(t, "filter_1", filter1, false))
}

// sortingParent renders the parent document of testNestedSorting.
func sortingParent(t testing.TB, field1 string) *document.Document {
	t.Helper()
	return newTestDocument(
		newStringField(t, "__type", "parent", false),
		newStringField(t, "field1", field1, false))
}

func mustSortField(t testing.TB) func(*ToParentBlockJoinSortField, error) *ToParentBlockJoinSortField {
	return func(sf *ToParentBlockJoinSortField, err error) *ToParentBlockJoinSortField {
		t.Helper()
		if err != nil {
			t.Fatalf("new ToParentBlockJoinSortField: %v", err)
		}
		return sf
	}
}

func mustSearchSorted(t testing.TB, s *search.IndexSearcher, q search.Query, n int, sortField *ToParentBlockJoinSortField) *search.TopFieldDocs {
	t.Helper()
	td, err := s.SearchWithSortNoScores(q, n, search.NewSort(sortField.SortField()))
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	return td
}

// assertSortedHit asserts topDocs.scoreDocs[i].doc and the FieldDoc's first
// sort value; want is a string (BytesRef.utf8ToString()), an int ((int)
// Integer) or nil (assertNull).
func assertSortedHit(t testing.TB, topDocs *search.TopFieldDocs, i, doc int, want any) {
	t.Helper()
	if topDocs.ScoreDocs[i].Doc != doc {
		t.Fatalf("scoreDocs[%d].doc: expected %d, got %d", i, doc, topDocs.ScoreDocs[i].Doc)
	}
	got := topDocs.FieldDocs[i].Fields[0]
	switch w := want.(type) {
	case nil:
		if got != nil {
			if b, ok := got.(*util.BytesRef); !ok || b != nil {
				t.Fatalf("fields[0] of hit %d: expected null, got %v", i, got)
			}
		}
	case string:
		b, ok := got.(*util.BytesRef)
		if !ok || b == nil || b.Utf8ToString() != w {
			t.Fatalf("fields[0] of hit %d: expected %q, got %v", i, w, got)
		}
	case int:
		var v int
		switch n := got.(type) {
		case int32:
			v = int(n)
		case int:
			v = n
		case int64:
			v = int(n)
		default:
			t.Fatalf("fields[0] of hit %d: expected an Integer, got %T", i, got)
		}
		if v != w {
			t.Fatalf("fields[0] of hit %d: expected %d, got %d", i, w, v)
		}
	}
}

func assertSortedTotals(t testing.TB, topDocs *search.TopFieldDocs, totalHits int64, length int) {
	t.Helper()
	if topDocs.TotalHits.Value != totalHits {
		t.Fatalf("totalHits: expected %d, got %d", totalHits, topDocs.TotalHits.Value)
	}
	if len(topDocs.ScoreDocs) != length {
		t.Fatalf("scoreDocs.length: expected %d, got %d", length, len(topDocs.ScoreDocs))
	}
}

func TestBlockJoinSortingNestedSorting(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newLogMergePolicy())
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	addBlock := func(docs ...*document.Document) {
		t.Helper()
		if _, err := w.AddDocuments(docs); err != nil {
			t.Fatalf("addDocuments: %v", err)
		}
	}

	addBlock(sortingChild(t, "a", "T"), sortingChild(t, "b", "T"), sortingChild(t, "c", "T"), sortingParent(t, "a"))
	mustCommit(t, w)

	addBlock(sortingChild(t, "c", "T"), sortingChild(t, "d", "T"), sortingChild(t, "e", "T"), sortingParent(t, "b"))

	addBlock(sortingChild(t, "e", "T"), sortingChild(t, "f", "T"), sortingChild(t, "g", "T"), sortingParent(t, "c"))

	addBlock(sortingChild(t, "g", "T"), sortingChild(t, "h", "F"), sortingChild(t, "i", "F"), sortingParent(t, "d"))
	mustCommit(t, w)

	addBlock(sortingChild(t, "i", "F"), sortingChild(t, "j", "F"), sortingChild(t, "k", "F"), sortingParent(t, "f"))

	addBlock(sortingChild(t, "k", "T"), sortingChild(t, "l", "T"), sortingChild(t, "m", "T"), sortingParent(t, "g"))

	addBlock(sortingChild(t, "m", "T"), sortingChild(t, "n", "F"), sortingChild(t, "o", "F"), sortingParent(t, "i"))
	mustCommit(t, w)

	searcher := search.NewIndexSearcher(mustOpenDirectoryReaderFromWriter(t, w.W))
	mustClose(t, w)
	parentFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("__type", "parent")))
	if err := Check(searcher.GetIndexReader(), parentFilter); err != nil {
		t.Fatalf("CheckJoinIndex.check: %v", err)
	}
	childFilter := NewQueryBitSetProducer(search.NewPrefixQuery(index.NewTerm("field2", "")))
	query := NewToParentBlockJoinQuery(search.NewPrefixQuery(index.NewTerm("field2", "")), parentFilter, None)

	must := mustSortField(t)

	// Sort by field ascending, order first
	sortField := must(NewToParentBlockJoinSortField("field2", spi.SortFieldTypeString, false, parentFilter, childFilter))
	topDocs := mustSearchSorted(t, searcher, query, 5, sortField)
	assertSortedTotals(t, topDocs, 7, 5)
	assertSortedHit(t, topDocs, 0, 3, "a")
	assertSortedHit(t, topDocs, 1, 7, "c")
	assertSortedHit(t, topDocs, 2, 11, "e")
	assertSortedHit(t, topDocs, 3, 15, "g")
	assertSortedHit(t, topDocs, 4, 19, "i")

	// Sort by field ascending, order last
	sortField = notEqualSortField(t, sortField, func() *ToParentBlockJoinSortField {
		return must(NewToParentBlockJoinSortFieldOrder("field2", spi.SortFieldTypeString, false, true, parentFilter, childFilter))
	})

	topDocs = mustSearchSorted(t, searcher, query, 5, sortField)
	assertSortedTotals(t, topDocs, 7, 5)
	assertSortedHit(t, topDocs, 0, 3, "c")
	assertSortedHit(t, topDocs, 1, 7, "e")
	assertSortedHit(t, topDocs, 2, 11, "g")
	assertSortedHit(t, topDocs, 3, 15, "i")
	assertSortedHit(t, topDocs, 4, 19, "k")

	// Sort by field descending, order last
	sortField = notEqualSortField(t, sortField, func() *ToParentBlockJoinSortField {
		return must(NewToParentBlockJoinSortField("field2", spi.SortFieldTypeString, true, parentFilter, childFilter))
	})
	topDocs = mustSearchSorted(t, searcher, query, 5, sortField)
	assertSortedTotals(t, topDocs, 7, 5)
	assertSortedHit(t, topDocs, 0, 27, "o")
	assertSortedHit(t, topDocs, 1, 23, "m")
	assertSortedHit(t, topDocs, 2, 19, "k")
	assertSortedHit(t, topDocs, 3, 15, "i")
	assertSortedHit(t, topDocs, 4, 11, "g")

	// Sort by field descending, order last, sort filter (filter_1:T)
	childFilter1T := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("filter_1", "T")))
	query = NewToParentBlockJoinQuery(search.NewTermQuery(index.NewTerm("filter_1", "T")), parentFilter, None)

	sortField = notEqualSortField(t, sortField, func() *ToParentBlockJoinSortField {
		return must(NewToParentBlockJoinSortField("field2", spi.SortFieldTypeString, true, parentFilter, childFilter1T))
	})

	topDocs = mustSearchSorted(t, searcher, query, 5, sortField)
	assertSortedTotals(t, topDocs, 6, 5)
	assertSortedHit(t, topDocs, 0, 23, "m")
	assertSortedHit(t, topDocs, 1, 27, "m")
	assertSortedHit(t, topDocs, 2, 11, "g")
	assertSortedHit(t, topDocs, 3, 15, "g")
	assertSortedHit(t, topDocs, 4, 7, "e")

	notEqualSortField(t, sortField, func() *ToParentBlockJoinSortField {
		return must(NewToParentBlockJoinSortField("field2", spi.SortFieldTypeString, true,
			NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("__type", "another"))), childFilter1T))
	})

	mustClose(t, searcher.GetIndexReader(), dir)
}

func TestBlockJoinSortingParentMissingValueNestedSorting(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newLogMergePolicy())
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	child := func(sortVal *int64) *document.Document {
		d := newTestDocument(newStringField(t, "child", "true", false))
		if sortVal != nil {
			d.Add(mustNumericDVField(t, "sort_val", *sortVal))
		}
		return d
	}
	parent := func() *document.Document {
		return newTestDocument(newStringField(t, "__type", "parent", false))
	}
	v := func(x int64) *int64 { return &x }
	addBlock := func(docs ...*document.Document) {
		t.Helper()
		if _, err := w.AddDocuments(docs); err != nil {
			t.Fatalf("addDocuments: %v", err)
		}
	}

	// Parent A (doc 2): children with values
	addBlock(child(v(20)), child(v(40)), parent())
	mustCommit(t, w)

	// Parent B (doc 5): children with values
	addBlock(child(v(10)), child(v(30)), parent())

	// Parent C (doc 8): children without values
	addBlock(child(nil), child(nil), parent())
	mustCommit(t, w)

	searcher := search.NewIndexSearcher(mustOpenDirectoryReaderFromWriter(t, w.W))
	mustClose(t, w)
	parentFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("__type", "parent")))
	if err := Check(searcher.GetIndexReader(), parentFilter); err != nil {
		t.Fatalf("CheckJoinIndex.check: %v", err)
	}
	childFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("child", "true")))
	query := NewToParentBlockJoinQuery(search.NewTermQuery(index.NewTerm("child", "true")), parentFilter, None)

	must := mustSortField(t)

	// Sort by ascending, with smaller missing value
	sortField := must(NewToParentBlockJoinSortFieldMissing("sort_val", spi.SortFieldTypeInt, false, int32(5), nil, parentFilter, childFilter))
	topDocs := mustSearchSorted(t, searcher, query, 10, sortField)
	assertSortedTotals(t, topDocs, 3, 3)
	assertSortedHit(t, topDocs, 0, 8, 5)
	assertSortedHit(t, topDocs, 1, 5, 10)
	assertSortedHit(t, topDocs, 2, 2, 20)

	// Sort by descending, with smaller missing value
	sortField = must(NewToParentBlockJoinSortFieldMissing("sort_val", spi.SortFieldTypeInt, true, int32(5), nil, parentFilter, childFilter))
	topDocs = mustSearchSorted(t, searcher, query, 10, sortField)
	assertSortedTotals(t, topDocs, 3, 3)
	assertSortedHit(t, topDocs, 0, 2, 40)
	assertSortedHit(t, topDocs, 1, 5, 30)
	assertSortedHit(t, topDocs, 2, 8, 5)

	// Sort by ascending, with greater missing value
	sortField = must(NewToParentBlockJoinSortFieldMissing("sort_val", spi.SortFieldTypeInt, false, int32(100), nil, parentFilter, childFilter))
	topDocs = mustSearchSorted(t, searcher, query, 10, sortField)
	assertSortedTotals(t, topDocs, 3, 3)
	assertSortedHit(t, topDocs, 0, 5, 10)
	assertSortedHit(t, topDocs, 1, 2, 20)
	assertSortedHit(t, topDocs, 2, 8, 100)

	// Sort descending with greater missing value
	sortField = must(NewToParentBlockJoinSortFieldMissing("sort_val", spi.SortFieldTypeInt, true, int32(100), nil, parentFilter, childFilter))
	topDocs = mustSearchSorted(t, searcher, query, 10, sortField)
	assertSortedTotals(t, topDocs, 3, 3)
	assertSortedHit(t, topDocs, 0, 8, 100)
	assertSortedHit(t, topDocs, 1, 2, 40)
	assertSortedHit(t, topDocs, 2, 5, 30)

	mustClose(t, searcher.GetIndexReader(), dir)
}

func TestBlockJoinSortingChildMissingValueNestedSorting(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newLogMergePolicy())
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	child := func(val *int64) *document.Document {
		d := newTestDocument(newStringField(t, "child", "true", false))
		if val != nil {
			d.Add(mustNumericDVField(t, "sort_numeric_val", *val))
			d.Add(mustSortedDVField(t, "sort_string_val", fmtInt(*val)))
		}
		return d
	}
	parent := func() *document.Document {
		return newTestDocument(newStringField(t, "__type", "parent", false))
	}
	v := func(x int64) *int64 { return &x }
	addBlock := func(docs ...*document.Document) {
		t.Helper()
		if _, err := w.AddDocuments(docs); err != nil {
			t.Fatalf("addDocuments: %v", err)
		}
	}

	// Parent A (doc 2): one child with value, one child without
	addBlock(child(v(30)), child(nil), parent())
	mustCommit(t, w)

	// Parent B (doc 5): all children with values
	addBlock(child(v(20)), child(v(40)), parent())
	mustCommit(t, w)

	searcher := search.NewIndexSearcher(mustOpenDirectoryReaderFromWriter(t, w.W))
	mustClose(t, w)
	parentFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("__type", "parent")))
	if err := Check(searcher.GetIndexReader(), parentFilter); err != nil {
		t.Fatalf("CheckJoinIndex.check: %v", err)
	}
	childFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("child", "true")))
	query := NewToParentBlockJoinQuery(search.NewTermQuery(index.NewTerm("child", "true")), parentFilter, None)

	must := mustSortField(t)

	// Sort by ascending with a smaller missing value
	sortField := must(NewToParentBlockJoinSortFieldMissing("sort_numeric_val", spi.SortFieldTypeInt, false, nil, int32(5), parentFilter, childFilter))
	topDocs := mustSearchSorted(t, searcher, query, 10, sortField)
	assertSortedTotals(t, topDocs, 2, 2)
	assertSortedHit(t, topDocs, 0, 2, 5)
	assertSortedHit(t, topDocs, 1, 5, 20)

	// Sort by descending with a smaller missing value
	sortField = must(NewToParentBlockJoinSortFieldMissing("sort_numeric_val", spi.SortFieldTypeInt, true, nil, int32(5), parentFilter, childFilter))
	topDocs = mustSearchSorted(t, searcher, query, 10, sortField)
	assertSortedTotals(t, topDocs, 2, 2)
	assertSortedHit(t, topDocs, 0, 5, 40)
	assertSortedHit(t, topDocs, 1, 2, 30)

	// Sort by ascending with a greater missing value
	sortField = must(NewToParentBlockJoinSortFieldMissing("sort_numeric_val", spi.SortFieldTypeInt, false, nil, int32(50), parentFilter, childFilter))
	topDocs = mustSearchSorted(t, searcher, query, 10, sortField)
	assertSortedTotals(t, topDocs, 2, 2)
	assertSortedHit(t, topDocs, 0, 5, 20)
	assertSortedHit(t, topDocs, 1, 2, 30)

	// Sort by descending with a greater missing value
	sortField = must(NewToParentBlockJoinSortFieldMissing("sort_numeric_val", spi.SortFieldTypeInt, true, nil, int32(50), parentFilter, childFilter))
	topDocs = mustSearchSorted(t, searcher, query, 10, sortField)
	assertSortedTotals(t, topDocs, 2, 2)
	assertSortedHit(t, topDocs, 0, 2, 50)
	assertSortedHit(t, topDocs, 1, 5, 40)

	// Sort by ascending with missing values first
	sortField = must(NewToParentBlockJoinSortFieldMissing("sort_string_val", spi.SortFieldTypeString, false, nil, nil, parentFilter, childFilter))
	topDocs = mustSearchSorted(t, searcher, query, 10, sortField)
	assertSortedTotals(t, topDocs, 2, 2)
	assertSortedHit(t, topDocs, 0, 2, nil)
	assertSortedHit(t, topDocs, 1, 5, "20")

	// Sort by descending with missing values last in child level and last in
	sortField = must(NewToParentBlockJoinSortFieldMissing("sort_string_val", spi.SortFieldTypeString, false, nil, search.STRING_LAST, parentFilter, childFilter))
	topDocs = mustSearchSorted(t, searcher, query, 10, sortField)
	assertSortedTotals(t, topDocs, 2, 2)
	assertSortedHit(t, topDocs, 0, 5, "20")
	assertSortedHit(t, topDocs, 1, 2, "30")

	// Sort by ascending with missing values first in child level and reverse order in parent
	sortField = must(NewToParentBlockJoinSortFieldOrderMissing("sort_string_val", spi.SortFieldTypeString, true, false, nil, search.STRING_FIRST, parentFilter, childFilter))
	topDocs = mustSearchSorted(t, searcher, query, 10, sortField)
	assertSortedTotals(t, topDocs, 2, 2)
	assertSortedHit(t, topDocs, 0, 5, "20")
	assertSortedHit(t, topDocs, 1, 2, nil)

	mustClose(t, searcher.GetIndexReader(), dir)
}

// fmtInt renders the decimal string of the BytesRef values ("20", "30", "40").
func fmtInt(v int64) string {
	return strconv.FormatInt(v, 10)
}

func notEqualSortField(t testing.TB, old *ToParentBlockJoinSortField, create func() *ToParentBlockJoinSortField) *ToParentBlockJoinSortField {
	t.Helper()
	newObj := create()
	if old.Equals(newObj) {
		t.Fatal("old.equals(newObj)")
	}
	if old == newObj {
		t.Fatal("assertNotSame(old, newObj)")
	}

	bro := create()
	if !newObj.Equals(bro) {
		t.Fatal("newObj != bro")
	}
	if newObj.HashCode() != bro.HashCode() {
		t.Fatal("newObj.hashCode() != bro.hashCode()")
	}
	if bro == newObj {
		t.Fatal("assertNotSame(bro, newObj)")
	}

	if old.Equals(bro) {
		t.Fatal("old.equals(bro)")
	}
	return newObj
}
