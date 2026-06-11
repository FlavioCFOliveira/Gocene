// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSortedSetSortField.java
//
// Tests for SortedSetSortField: equality, serialization, constructor variants,
// missing value validation, and integration with a real index.

package search_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestSortedSetSortField_Equals mirrors TestSortedSetSortField.testEquals.
func TestSortedSetSortField_Equals(t *testing.T) {
	sf := search.NewSortedSetSortField("a", false)

	if sf.Equals(nil) {
		t.Errorf("sf.Equals(nil) = true, want false")
	}
	if !sf.Equals(sf) {
		t.Errorf("sf.Equals(sf) = false (identity), want true")
	}

	sf2 := search.NewSortedSetSortField("a", false)
	if !sf.Equals(sf2) {
		t.Errorf("sf.Equals(sf2) = false (same values), want true")
	}
	if sf.HashCode() != sf2.HashCode() {
		t.Errorf("equal fields have different hash codes: %d vs %d", sf.HashCode(), sf2.HashCode())
	}

	if sf.Equals(search.NewSortedSetSortField("a", true)) {
		t.Errorf("sf.Equals(reverse=true) = true, want false")
	}
	if sf.Equals(search.NewSortedSetSortField("b", false)) {
		t.Errorf("sf.Equals(field=b) = true, want false")
	}
	if sf.Equals(search.NewSortedSetSortFieldWithSelector("a", false, search.SortedSetSelectorMax)) {
		t.Errorf("sf.Equals(selector=MAX) = true, want false")
	}
	if sf.Equals("foo") {
		t.Errorf("sf.Equals(string) = true, want false")
	}
}

// TestSortedSetSortField_DefaultSelector verifies the default selector is MIN.
func TestSortedSetSortField_DefaultSelector(t *testing.T) {
	sf := search.NewSortedSetSortField("f", false)
	if sf.GetSelector() != search.SortedSetSelectorMin {
		t.Errorf("default selector = %v, want SortedSetSelectorMin", sf.GetSelector())
	}
}

// TestSortedSetSortField_AllSelectors verifies all selector values can be
// constructed.
func TestSortedSetSortField_AllSelectors(t *testing.T) {
	selectors := []search.SortedSetSelectorType{
		search.SortedSetSelectorMin,
		search.SortedSetSelectorMax,
		search.SortedSetSelectorMiddleMin,
		search.SortedSetSelectorMiddleMax,
	}
	for _, sel := range selectors {
		sf := search.NewSortedSetSortFieldWithSelector("f", false, sel)
		if sf.GetSelector() != sel {
			t.Errorf("selector round-trip: got %v, want %v", sf.GetSelector(), sel)
		}
	}
}

// TestSortedSetSortField_SetMissingValue_Valid verifies STRING_FIRST and
// STRING_LAST are accepted.
func TestSortedSetSortField_SetMissingValue_Valid(t *testing.T) {
	sf := search.NewSortedSetSortField("f", false)
	if err := sf.SetMissingValue(search.STRING_FIRST); err != nil {
		t.Errorf("SetMissingValue(STRING_FIRST) = %v, want nil", err)
	}
	sf2 := search.NewSortedSetSortField("f", false)
	if err := sf2.SetMissingValue(search.STRING_LAST); err != nil {
		t.Errorf("SetMissingValue(STRING_LAST) = %v, want nil", err)
	}
}

// TestSortedSetSortField_SetMissingValue_Invalid verifies that non-sentinel
// values are rejected.
func TestSortedSetSortField_SetMissingValue_Invalid(t *testing.T) {
	sf := search.NewSortedSetSortField("f", false)
	err := sf.SetMissingValue("arbitrary string")
	if err == nil {
		t.Errorf("SetMissingValue(arbitrary) = nil, want error")
	}
	err = sf.SetMissingValue(42)
	if err == nil {
		t.Errorf("SetMissingValue(int) = nil, want error")
	}
}

// TestSortedSetSortField_String verifies the toString format.
func TestSortedSetSortField_String(t *testing.T) {
	sf := search.NewSortedSetSortField("myField", false)
	s := sf.String()
	if !strings.Contains(s, "myField") {
		t.Errorf("String() %q does not contain field name", s)
	}
	if !strings.Contains(s, "sortedset") {
		t.Errorf("String() %q does not contain 'sortedset'", s)
	}

	sfRev := search.NewSortedSetSortField("f", true)
	if !strings.Contains(sfRev.String(), "!") {
		t.Errorf("reversed String() %q missing '!'", sfRev.String())
	}

	sfMissFirst := search.NewSortedSetSortField("f", false)
	_ = sfMissFirst.SetMissingValue(search.STRING_FIRST)
	if !strings.Contains(sfMissFirst.String(), "STRING_FIRST") {
		t.Errorf("String() %q missing STRING_FIRST", sfMissFirst.String())
	}
}

// TestSortedSetSortField_Serialization verifies Serialize / ReadSortedSetSortField
// round-trip for all selector and missing-value combinations.
func TestSortedSetSortField_Serialization(t *testing.T) {
	cases := []struct {
		name         string
		field        string
		reverse      bool
		selector     search.SortedSetSelectorType
		missingValue interface{}
	}{
		{"min-asc", "f", false, search.SortedSetSelectorMin, nil},
		{"max-desc", "g", true, search.SortedSetSelectorMax, nil},
		{"middle-min-first", "h", false, search.SortedSetSelectorMiddleMin, search.STRING_FIRST},
		{"middle-max-last", "i", true, search.SortedSetSelectorMiddleMax, search.STRING_LAST},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orig := search.NewSortedSetSortFieldFull(tc.field, tc.reverse, tc.selector, tc.missingValue)
			out := store.NewByteArrayDataOutput(256)
			if err := orig.Serialize(out); err != nil {
				t.Fatalf("Serialize: %v", err)
			}
			in := store.NewByteArrayDataInput(out.GetBytes())
			got, err := search.ReadSortedSetSortField(in)
			if err != nil {
				t.Fatalf("ReadSortedSetSortField: %v", err)
			}
			if !orig.Equals(got) {
				t.Errorf("round-trip Equals = false; orig=%v got=%v", orig.String(), got.String())
			}
		})
	}
}

// TestSortedSetSortField_ForwardIndexIntegration uses the integration harness
// to index documents with SortedSetDocValuesField and verifies sort order via
// IndexSearcher.SearchWithSort with SortedSetSortField.
func TestSortedSetSortField_ForwardIndexIntegration(t *testing.T) {
	ix := newIntegrationIndex(t)

	// Index three documents with single-valued SortedSetDocValuesField values.
	// The tokens "b", "a", "c" should sort ascending to "a", "b", "c".
	values := []string{"b", "a", "c"}
	for _, val := range values {
		doc := document.NewDocument()
		// Also add a StringField so MatchAllDocsQuery finds the document.
		sf, err := document.NewStringField("id", val, false)
		if err != nil {
			t.Fatalf("NewStringField: %v", err)
		}
		doc.Add(sf)
		dv, err := document.NewSortedSetDocValuesField("field", [][]byte{[]byte(val)})
		if err != nil {
			t.Fatalf("NewSortedSetDocValuesField: %v", err)
		}
		doc.Add(dv)
		ix.addDoc(doc)
	}

	s, cleanup := ix.searcher()
	defer cleanup()

	// Sort ascending by SortedSetSortField.
	sf := search.NewSortedSetSortField("field", false)
	sort := search.NewSort(sf)
	top, err := s.SearchWithSort(search.NewMatchAllDocsQuery(), 10, sort)
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}

	if top.TotalHits.Value != 3 {
		t.Fatalf("expected 3 hits, got %d", top.TotalHits.Value)
	}

	// Verify sort order: docs should be ordered a, b, c (ascending).
	// We retrieve stored id field for each doc.
	gotOrder := make([]string, len(top.ScoreDocs))
	for i, sd := range top.ScoreDocs {
		doc, err := s.Doc(sd.Doc)
		if err != nil {
			t.Fatalf("Doc(%d): %v", sd.Doc, err)
		}
		gotOrder[i] = doc.Get("id")
	}

	wantOrder := []string{"a", "b", "c"}
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Errorf("sort order[%d] = %q, want %q", i, gotOrder[i], wantOrder[i])
		}
	}
}

// TestSortedSetSortField_ReverseIndexIntegration verifies descending sort order
// using a real index.
func TestSortedSetSortField_ReverseIndexIntegration(t *testing.T) {
	ix := newIntegrationIndex(t)

	values := []string{"b", "a", "c"}
	for _, val := range values {
		doc := document.NewDocument()
		sf, err := document.NewStringField("id", val, false)
		if err != nil {
			t.Fatalf("NewStringField: %v", err)
		}
		doc.Add(sf)
		dv, err := document.NewSortedSetDocValuesField("field", [][]byte{[]byte(val)})
		if err != nil {
			t.Fatalf("NewSortedSetDocValuesField: %v", err)
		}
		doc.Add(dv)
		ix.addDoc(doc)
	}

	s, cleanup := ix.searcher()
	defer cleanup()

	// Sort descending by SortedSetSortField.
	sf := search.NewSortedSetSortField("field", true)
	sort := search.NewSort(sf)
	top, err := s.SearchWithSort(search.NewMatchAllDocsQuery(), 10, sort)
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}

	if top.TotalHits.Value != 3 {
		t.Fatalf("expected 3 hits, got %d", top.TotalHits.Value)
	}

	gotOrder := make([]string, len(top.ScoreDocs))
	for i, sd := range top.ScoreDocs {
		doc, err := s.Doc(sd.Doc)
		if err != nil {
			t.Fatalf("Doc(%d): %v", sd.Doc, err)
		}
		gotOrder[i] = doc.Get("id")
	}

	wantOrder := []string{"c", "b", "a"}
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Errorf("reverse sort order[%d] = %q, want %q", i, gotOrder[i], wantOrder[i])
		}
	}
}

// TestSortedSetSortField_MaxSelectorIntegration verifies the MAX selector
// picks the highest value from multi-valued SortedSetDocValuesField.
func TestSortedSetSortField_MaxSelectorIntegration(t *testing.T) {
	ix := newIntegrationIndex(t)

	// Two documents with multi-valued SortedSetDocValues.
	// Doc 0: {"a", "c"} sorted via MAX selector -> "c"
	// Doc 1: {"b", "d"} sorted via MAX selector -> "d"
	doc0 := document.NewDocument()
	sf0, _ := document.NewStringField("id", "doc0", false)
	doc0.Add(sf0)
	dv0, _ := document.NewSortedSetDocValuesField("field", [][]byte{[]byte("a"), []byte("c")})
	doc0.Add(dv0)
	ix.addDoc(doc0)

	doc1 := document.NewDocument()
	sf1, _ := document.NewStringField("id", "doc1", false)
	doc1.Add(sf1)
	dv1, _ := document.NewSortedSetDocValuesField("field", [][]byte{[]byte("b"), []byte("d")})
	doc1.Add(dv1)
	ix.addDoc(doc1)

	s, cleanup := ix.searcher()
	defer cleanup()

	// Sort ascending by MAX selector.
	sf := search.NewSortedSetSortFieldWithSelector("field", false, search.SortedSetSelectorMax)
	sort := search.NewSort(sf)
	top, err := s.SearchWithSort(search.NewMatchAllDocsQuery(), 10, sort)
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}

	if top.TotalHits.Value != 2 {
		t.Fatalf("expected 2 hits, got %d", top.TotalHits.Value)
	}

	gotOrder := make([]string, len(top.ScoreDocs))
	for i, sd := range top.ScoreDocs {
		d, err := s.Doc(sd.Doc)
		if err != nil {
			t.Fatalf("Doc(%d): %v", sd.Doc, err)
		}
		gotOrder[i] = d.Get("id")
	}

	// MAX("a","c") = "c", MAX("b","d") = "d", ascending -> doc0, doc1
	wantOrder := []string{"doc0", "doc1"}
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Errorf("MAX sort order[%d] = %q, want %q", i, gotOrder[i], wantOrder[i])
		}
	}
}

// TestSortedSetSortField_MultiSegmentSort verifies cross-segment sort ordering.
func TestSortedSetSortField_MultiSegmentSort(t *testing.T) {
	ix := newIntegrationIndex(t)

	// Add docs "b", then commit, then "a", then commit, then "c".
	for _, val := range []string{"b", "a", "c"} {
		doc := document.NewDocument()
		sf, _ := document.NewStringField("id", val, false)
		doc.Add(sf)
		dv, _ := document.NewSortedSetDocValuesField("field", [][]byte{[]byte(val)})
		doc.Add(dv)
		ix.addDoc(doc)
		ix.commit()
	}

	s, cleanup := ix.searcher()
	defer cleanup()

	sf := search.NewSortedSetSortField("field", false)
	sort := search.NewSort(sf)
	top, err := s.SearchWithSort(search.NewMatchAllDocsQuery(), 10, sort)
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}

	if top.TotalHits.Value != 3 {
		t.Fatalf("expected 3 hits, got %d", top.TotalHits.Value)
	}

	gotOrder := make([]string, len(top.ScoreDocs))
	for i, sd := range top.ScoreDocs {
		d, err := s.Doc(sd.Doc)
		if err != nil {
			t.Fatalf("Doc(%d): %v", sd.Doc, err)
		}
		gotOrder[i] = d.Get("id")
	}

	wantOrder := []string{"a", "b", "c"}
	if !sort.StringsAreSorted(gotOrder) {
		t.Errorf("sort order = %v, want sorted %v", gotOrder, wantOrder)
	}
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Errorf("multi-segment sort[%d] = %q, want %q", i, gotOrder[i], wantOrder[i])
		}
	}
}

// TestSortedSetSortField_MissingFirstIntegration verifies STRING_FIRST sorting
// when one document has no value for the sort field.
func TestSortedSetSortField_MissingFirstIntegration(t *testing.T) {
	ix := newIntegrationIndex(t)

	// Doc with value "b".
	doc1 := document.NewDocument()
	sf1, _ := document.NewStringField("id", "has-value", false)
	doc1.Add(sf1)
	dv1, _ := document.NewSortedSetDocValuesField("field", [][]byte{[]byte("b")})
	doc1.Add(dv1)
	ix.addDoc(doc1)

	// Doc with no SortedSetDocValuesField (missing value).
	doc2 := document.NewDocument()
	sf2, _ := document.NewStringField("id", "missing", false)
	doc2.Add(sf2)
	ix.addDoc(doc2)

	s, cleanup := ix.searcher()
	defer cleanup()

	sf := search.NewSortedSetSortField("field", false)
	_ = sf.SetMissingValue(search.STRING_FIRST)
	sort := search.NewSort(sf)
	top, err := s.SearchWithSort(search.NewMatchAllDocsQuery(), 10, sort)
	if err != nil {
		t.Fatalf("SearchWithSort: %v", err)
	}

	if top.TotalHits.Value != 2 {
		t.Fatalf("expected 2 hits, got %d", top.TotalHits.Value)
	}

	gotOrder := make([]string, len(top.ScoreDocs))
	for i, sd := range top.ScoreDocs {
		d, err := s.Doc(sd.Doc)
		if err != nil {
			t.Fatalf("Doc(%d): %v", sd.Doc, err)
		}
		gotOrder[i] = d.Get("id")
	}

	// STRING_FIRST: missing value doc should come first.
	if gotOrder[0] != "missing" {
		t.Errorf("STRING_FIRST: first doc = %q, want 'missing'", gotOrder[0])
	}
}
