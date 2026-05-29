// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package join contains tests porting
// org.apache.lucene.search.join.TestBlockJoinSorting.
//
// testNestedSorting sorts parent docs by an aggregate of a child SortedDocValues
// field via ToParentBlockJoinSortField + searcher.search(query, n, sort), wired
// end-to-end on top of the field-sorted-over-DocValues search subsystem
// (rmp #4778) and the ToParentBlockJoinSortField / BlockJoinSelector.wrap
// plumbing (rmp #4779 / #4758).
//
// Deviations from the Lucene reference, both behaviourally exact for this
// deterministic corpus:
//
//  1. Lucene's child query and child filter use PrefixQuery(field2) to select
//     every child that has a field2 value. Gocene's PrefixQuery yields a nil
//     weight (rmp #4760), so it is substituted by an equivalent SHOULD set of
//     TermQuery(field2, letter) over the corpus alphabet. Every child here has a
//     field2 value, so the substitution selects exactly the same children. The
//     fourth variant uses a real TermQuery(filter_1, "T"), matching Lucene
//     verbatim.
//  2. Lucene puts the indexed term and the SortedDocValues on the same field
//     name (field2). Gocene drops the DocValuesType when a field name carries
//     both an indexed field and a DocValues field in the same document (rmp
//     #4780), so the per-child sort value is stored in a sibling
//     SortedDocValuesField named field2dv carrying the same letter. The sort
//     therefore reads the identical values; only the field name differs.
//
// The index is built in a single segment (no intermediate commits) so the parent
// global doc ids are 3, 7, 11, 15, 19, 23, 27 exactly as the Lucene assertions
// expect.
package join

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// sortChild builds one child document for the sorting corpus: a StringField
// field2 (for term-based child queries/filters), a SortedDocValuesField field2dv
// carrying the same letter (the value the parent is sorted by; see deviation #2),
// and a StringField filter_1.
func sortChild(t *testing.T, field2, filter1 string) index.Document {
	t.Helper()
	d := document.NewDocument()
	d.Add(mustStringField(t, "field2", field2, false))
	dv, err := document.NewSortedDocValuesField("field2dv", []byte(field2))
	if err != nil {
		t.Fatalf("NewSortedDocValuesField(field2dv=%q): %v", field2, err)
	}
	d.Add(dv)
	d.Add(mustStringField(t, "filter_1", filter1, false))
	return d
}

// sortParent builds the trailing parent document of a block.
func sortParent(t *testing.T, field1 string) index.Document {
	t.Helper()
	d := document.NewDocument()
	d.Add(mustStringField(t, "__type", "parent", false))
	d.Add(mustStringField(t, "field1", field1, false))
	return d
}

// allField2 builds the PrefixQuery(field2) substitute: a SHOULD set of
// TermQuery(field2, letter) over the corpus alphabet a..o. It selects every
// child that has a field2 value, exactly like the Lucene PrefixQuery.
func allField2() search.Query {
	bq := search.NewBooleanQuery()
	for c := 'a'; c <= 'o'; c++ {
		bq.Add(search.NewTermQuery(index.NewTerm("field2", string(c))), search.SHOULD)
	}
	return bq
}

// sortValues extracts the FieldDoc[0] sort value of every hit as a string.
func sortValues(t *testing.T, td *search.TopFieldDocs) []string {
	t.Helper()
	out := make([]string, len(td.FieldDocs))
	for i, fd := range td.FieldDocs {
		if len(fd.Fields) != 1 {
			t.Fatalf("hit %d: Fields len = %d, want 1", i, len(fd.Fields))
		}
		b, ok := fd.Fields[0].([]byte)
		if !ok {
			t.Fatalf("hit %d: Fields[0] = %v (%T), want []byte", i, fd.Fields[0], fd.Fields[0])
		}
		out[i] = string(b)
	}
	return out
}

// TestBlockJoinSorting_NestedSorting corresponds to
// TestBlockJoinSorting.testNestedSorting. It indexes seven parent/child blocks
// and sorts the parents by an aggregate (MIN/MAX) of the child field2
// SortedDocValues via ToParentBlockJoinSortField driving searcher.SearchWithSort.
func TestBlockJoinSorting_NestedSorting(t *testing.T) {
	dir, w := newBlockWriter(t)

	// Block 0: children a,b,c -> parent doc 3 (field1=a)
	addBlock(t, w, sortChild(t, "a", "T"), sortChild(t, "b", "T"), sortChild(t, "c", "T"), sortParent(t, "a"))
	// Block 1: children c,d,e -> parent doc 7 (field1=b)
	addBlock(t, w, sortChild(t, "c", "T"), sortChild(t, "d", "T"), sortChild(t, "e", "T"), sortParent(t, "b"))
	// Block 2: children e,f,g -> parent doc 11 (field1=c)
	addBlock(t, w, sortChild(t, "e", "T"), sortChild(t, "f", "T"), sortChild(t, "g", "T"), sortParent(t, "c"))
	// Block 3: children g,h,i -> parent doc 15 (field1=d), filter_1 g=T h=F i=F
	addBlock(t, w, sortChild(t, "g", "T"), sortChild(t, "h", "F"), sortChild(t, "i", "F"), sortParent(t, "d"))
	// Block 4: children i,j,k -> parent doc 19 (field1=f), filter_1 all F
	addBlock(t, w, sortChild(t, "i", "F"), sortChild(t, "j", "F"), sortChild(t, "k", "F"), sortParent(t, "f"))
	// Block 5: children k,l,m -> parent doc 23 (field1=g), filter_1 all T
	addBlock(t, w, sortChild(t, "k", "T"), sortChild(t, "l", "T"), sortChild(t, "m", "T"), sortParent(t, "g"))
	// Block 6: children m,n,o -> parent doc 27 (field1=i), filter_1 m=T n=F o=F
	addBlock(t, w, sortChild(t, "m", "T"), sortChild(t, "n", "F"), sortChild(t, "o", "F"), sortParent(t, "i"))

	r, s := commitAndOpen(t, dir, w)

	parentFilter := newQueryBitSetParents("__type", "parent")
	if err := Check(r, parentFilter); err != nil {
		t.Fatalf("Check: %v", err)
	}
	childFilter := NewQueryBitSetProducer(allField2())
	query := NewToParentBlockJoinQuery(allField2(), parentFilter, None)

	assertHits := func(name string, td *search.TopFieldDocs, wantTotal int64, wantDocs []int, wantVals []string) {
		t.Helper()
		if td.TotalHits.Value != wantTotal {
			t.Fatalf("%s: totalHits = %d, want %d", name, td.TotalHits.Value, wantTotal)
		}
		if len(td.ScoreDocs) != len(wantDocs) {
			t.Fatalf("%s: scoreDocs len = %d, want %d", name, len(td.ScoreDocs), len(wantDocs))
		}
		if got := docOrderJoin(td); !equalIntSlices(got, wantDocs) {
			t.Fatalf("%s: doc order = %v, want %v", name, got, wantDocs)
		}
		if got := sortValues(t, td); !equalStringSlices(got, wantVals) {
			t.Fatalf("%s: sort values = %v, want %v", name, got, wantVals)
		}
	}

	// Variant 1: sort by field ascending, order first (MIN, ascending).
	sf1, err := NewToParentBlockJoinSortField("field2dv", search.SortFieldTypeString, false, parentFilter, childFilter)
	if err != nil {
		t.Fatalf("NewToParentBlockJoinSortField v1: %v", err)
	}
	td, err := s.SearchWithSort(query, 5, sf1.Sort())
	if err != nil {
		t.Fatalf("SearchWithSort v1: %v", err)
	}
	assertHits("v1 MIN asc", td, 7,
		[]int{3, 7, 11, 15, 19},
		[]string{"a", "c", "e", "g", "i"})

	// Variant 2: sort by field ascending, order last (MAX, ascending).
	sf2, err := NewToParentBlockJoinSortFieldOrder("field2dv", search.SortFieldTypeString, false, true, parentFilter, childFilter)
	if err != nil {
		t.Fatalf("NewToParentBlockJoinSortFieldOrder v2: %v", err)
	}
	td, err = s.SearchWithSort(query, 5, sf2.Sort())
	if err != nil {
		t.Fatalf("SearchWithSort v2: %v", err)
	}
	assertHits("v2 MAX asc", td, 7,
		[]int{3, 7, 11, 15, 19},
		[]string{"c", "e", "g", "i", "k"})

	// Variant 3: sort by field descending, order last (MAX, descending).
	sf3, err := NewToParentBlockJoinSortField("field2dv", search.SortFieldTypeString, true, parentFilter, childFilter)
	if err != nil {
		t.Fatalf("NewToParentBlockJoinSortField v3: %v", err)
	}
	td, err = s.SearchWithSort(query, 5, sf3.Sort())
	if err != nil {
		t.Fatalf("SearchWithSort v3: %v", err)
	}
	assertHits("v3 MAX desc", td, 7,
		[]int{27, 23, 19, 15, 11},
		[]string{"o", "m", "k", "i", "g"})

	// Variant 4: sort by field descending, order last, childFilter filter_1:T.
	childFilter1T := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("filter_1", "T")))
	query1T := NewToParentBlockJoinQuery(search.NewTermQuery(index.NewTerm("filter_1", "T")), parentFilter, None)
	sf4, err := NewToParentBlockJoinSortField("field2dv", search.SortFieldTypeString, true, parentFilter, childFilter1T)
	if err != nil {
		t.Fatalf("NewToParentBlockJoinSortField v4: %v", err)
	}
	td, err = s.SearchWithSort(query1T, 5, sf4.Sort())
	if err != nil {
		t.Fatalf("SearchWithSort v4: %v", err)
	}
	assertHits("v4 MAX desc filter_1:T", td, 6,
		[]int{23, 27, 11, 15, 7},
		[]string{"m", "m", "g", "g", "e"})
}

// TestBlockJoinSorting_SortFieldDescriptor verifies the structural accessors of
// ToParentBlockJoinSortField with the Lucene-faithful constructor signature.
func TestBlockJoinSorting_SortFieldDescriptor(t *testing.T) {
	parents := newQueryBitSetParents("__type", "parent")
	children := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("field2", "a")))

	sf, err := NewToParentBlockJoinSortField("childField", search.SortFieldTypeInt, false, parents, children)
	if err != nil {
		t.Fatalf("NewToParentBlockJoinSortField: %v", err)
	}
	if sf.Field() != "childField" {
		t.Errorf("Field = %q, want %q", sf.Field(), "childField")
	}
	if sf.Type() != search.SortFieldTypeInt {
		t.Errorf("Type = %v, want SortFieldTypeInt", sf.Type())
	}
	if sf.Reverse() {
		t.Error("Reverse should be false")
	}
	if !sf.IsAscending() {
		t.Error("IsAscending() should be true")
	}
	// MIN selection for ascending order.
	if got := sf.selectorType(); got != BlockJoinMin {
		t.Errorf("selectorType() = %v, want BlockJoinMin", got)
	}
	// The produced SortField must carry the field and STRING/numeric type.
	if produced := sf.SortField(); produced.Field != "childField" || produced.Type != search.SortFieldTypeInt {
		t.Errorf("SortField() = {Field:%q Type:%v}, want {childField Int}", produced.Field, produced.Type)
	}

	// An unsupported type is rejected.
	if _, err := NewToParentBlockJoinSortField("f", search.SortFieldTypeScore, false, parents, children); err == nil {
		t.Error("expected error for unsupported sort type SCORE")
	}
}

// docOrderJoin returns the doc ids of a TopFieldDocs in result order.
func docOrderJoin(td *search.TopFieldDocs) []int {
	out := make([]int, len(td.ScoreDocs))
	for i, sd := range td.ScoreDocs {
		out[i] = sd.Doc
	}
	return out
}

func equalIntSlices(a, b []int) bool {
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

func equalStringSlices(a, b []string) bool {
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
