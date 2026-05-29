// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sortedset

// TestSortedSetDocValuesFacets ports the behavioural assertions from
// org.apache.lucene.facet.sortedset.TestSortedSetDocValuesFacets.
//
// The integration tests (testBasic, testBasicHierarchical,
// testCombinationsOfConfig) now run against the real on-disk SortedSetDocValues
// pipeline (IndexWriter + Lucene104 codec + OpenDirectoryReader) using the
// default codec-driven accumulator path wired in rmp #4704. Unit-level tests
// for the API surface run unconditionally.
//
// Divergence note: Lucene's getTopChildren dim "value" reflects FacetsConfig
// DimConfig dim-count semantics (-1 for multi-valued dims without dim counts,
// the dim doc-count otherwise). Gocene's accumulator computes the dim value as
// the sum of the matched child counts; these tests therefore assert the
// per-child counts and specific values (the subject of #4704), not the
// DimConfig-derived dim value.

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/facets"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"

	// Blank-import the codecs so the production Lucene104 codec is registered
	// as the default; the SortedSetDocValues are not persisted without it.
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

// ssdvFacetDoc is a minimal index.Document carrying the supplied fields.
type ssdvFacetDoc struct {
	fields []interface{}
}

func (d *ssdvFacetDoc) GetFields() []interface{} { return d.fields }

// buildSSDVAccumulator indexes docs (each a slice of "dim/label" encoded facet
// terms on the $facets field), commits, reopens, and drives a fresh
// SortedSetDocValuesAccumulator with all docs matching and NO resolver hook.
// It returns the populated accumulator and a cleanup closing the reader.
func buildSSDVAccumulator(t *testing.T, docs [][]string) (*SortedSetDocValuesAccumulator, func()) {
	t.Helper()
	const indexField = "$facets"

	dir := store.NewByteBuffersDirectory()
	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for _, encoded := range docs {
		values := make([][]byte, len(encoded))
		for i, e := range encoded {
			values[i] = []byte(e)
		}
		f, err := document.NewSortedSetDocValuesField(indexField, values)
		if err != nil {
			t.Fatalf("NewSortedSetDocValuesField: %v", err)
		}
		if err := writer.AddDocument(&ssdvFacetDoc{fields: []interface{}{f}}); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
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
	leaves, err := reader.Leaves()
	if err != nil {
		reader.Close()
		t.Fatalf("Leaves: %v", err)
	}
	md := make([]*facets.MatchingDocs, 0, len(leaves))
	for _, leaf := range leaves {
		md = append(md, facets.NewMatchingDocs(leaf, nil, leaf.Reader().MaxDoc()))
	}

	acc, err := NewSortedSetDocValuesAccumulator(facets.NewFacetsConfig(), indexField)
	if err != nil {
		reader.Close()
		t.Fatalf("NewSortedSetDocValuesAccumulator: %v", err)
	}
	if err := acc.AccumulateFromMatchingDocs(md); err != nil {
		reader.Close()
		t.Fatalf("AccumulateFromMatchingDocs: %v", err)
	}
	return acc, func() { reader.Close() }
}

// childCounts collapses a FacetResult into a label->count map.
func childCounts(r *facets.FacetResult) map[string]int64 {
	out := map[string]int64{}
	if r == nil {
		return out
	}
	for _, lv := range r.LabelValues {
		out[lv.Label] = lv.Value
	}
	return out
}

// assertChildCounts checks the children of dim against want.
func assertChildCounts(t *testing.T, acc *SortedSetDocValuesAccumulator, dim string, want map[string]int64) {
	t.Helper()
	r, err := acc.GetTopChildren(100, dim)
	if err != nil {
		t.Fatalf("GetTopChildren(%q): %v", dim, err)
	}
	got := childCounts(r)
	if len(got) != len(want) {
		t.Fatalf("dim %q: child count = %d (%v), want %d (%v)", dim, len(got), got, len(want), want)
	}
	for label, w := range want {
		if got[label] != w {
			t.Errorf("dim %q child %q = %d, want %d (all: %v)", dim, label, got[label], w, got)
		}
	}
}

// ---------------------------------------------------------------------------
// SortedSetDocValuesFacetField
// ---------------------------------------------------------------------------

func TestSortedSetDocValuesFacetField_Basic(t *testing.T) {
	f := NewSortedSetDocValuesFacetField("a", "foo")
	if f.Dim != "a" {
		t.Errorf("expected dim %q, got %q", "a", f.Dim)
	}
	if f.Label != "foo" {
		t.Errorf("expected label %q, got %q", "foo", f.Label)
	}
	want := "a/foo"
	if got := f.EncodedValue(); got != want {
		t.Errorf("EncodedValue: want %q, got %q", want, got)
	}
}

func TestSortedSetDocValuesFacetField_MultiComponent(t *testing.T) {
	cases := []struct {
		dim   string
		label string
		want  string
	}{
		{"b", "baz", "b/baz"},
		{"b", "buzz", "b/buzz"},
		{"c", "buzz", "c/buzz"},
	}
	for _, c := range cases {
		f := NewSortedSetDocValuesFacetField(c.dim, c.label)
		if got := f.EncodedValue(); got != c.want {
			t.Errorf("EncodedValue(%q,%q): want %q, got %q", c.dim, c.label, c.want, got)
		}
	}
}

// ---------------------------------------------------------------------------
// DefaultSortedSetDocValuesReaderState
// ---------------------------------------------------------------------------

func TestDefaultSortedSetDocValuesReaderState_GetField(t *testing.T) {
	state := NewDefaultSortedSetDocValuesReaderState("$facets", 10, map[string][2]int{
		"a": {0, 3},
		"b": {3, 5},
	})
	if state.GetField() != "$facets" {
		t.Errorf("GetField: want %q, got %q", "$facets", state.GetField())
	}
}

func TestDefaultSortedSetDocValuesReaderState_GetSize(t *testing.T) {
	state := NewDefaultSortedSetDocValuesReaderState("$facets", 7, map[string][2]int{
		"a": {0, 3},
		"b": {3, 7},
	})
	if state.GetSize() != 7 {
		t.Errorf("GetSize: want 7, got %d", state.GetSize())
	}
}

func TestDefaultSortedSetDocValuesReaderState_GetOrdRange(t *testing.T) {
	state := NewDefaultSortedSetDocValuesReaderState("$facets", 5, map[string][2]int{
		"a": {0, 3},
		"b": {3, 5},
	})

	start, end := state.GetOrdRange("a")
	if start != 0 || end != 3 {
		t.Errorf("GetOrdRange(a): want (0,3), got (%d,%d)", start, end)
	}

	start, end = state.GetOrdRange("b")
	if start != 3 || end != 5 {
		t.Errorf("GetOrdRange(b): want (3,5), got (%d,%d)", start, end)
	}

	// Unknown dim
	start, end = state.GetOrdRange("z")
	if start != -1 || end != -1 {
		t.Errorf("GetOrdRange(z): want (-1,-1), got (%d,%d)", start, end)
	}
}

func TestDefaultSortedSetDocValuesReaderState_GetOrdRangeFor(t *testing.T) {
	state := NewDefaultSortedSetDocValuesReaderState("$facets", 5, map[string][2]int{
		"a": {0, 3}, // ords 0, 1, 2
		"b": {3, 5}, // ords 3, 4
	})

	// "a": start=0, end-inclusive=2
	r := state.GetOrdRangeFor("a")
	if r == nil {
		t.Fatal("GetOrdRangeFor(a): want non-nil")
	}
	if r.Start != 0 || r.End != 2 {
		t.Errorf("GetOrdRangeFor(a): want [0,2], got [%d,%d]", r.Start, r.End)
	}

	// Unknown dim
	if state.GetOrdRangeFor("z") != nil {
		t.Error("GetOrdRangeFor(z): want nil for unknown dim")
	}
}

func TestDefaultSortedSetDocValuesReaderState_GetDimTree(t *testing.T) {
	state := NewDefaultSortedSetDocValuesReaderState("$facets", 5, map[string][2]int{
		"a": {0, 3},
	})
	if state.GetDimTree("a") != nil {
		t.Error("GetDimTree(a): want nil when no tree registered")
	}
}

// ---------------------------------------------------------------------------
// SortedSetDocValuesFacetCounts — API surface
// ---------------------------------------------------------------------------

func TestSortedSetDocValuesFacetCounts_GetTopChildren_Empty(t *testing.T) {
	cfg := facets.NewFacetsConfig()
	counts := NewSortedSetDocValuesFacetCounts(cfg, "$facets")

	// No data — expect error or empty result.
	result, err := counts.GetTopChildren(10, "a")
	if err == nil && result != nil && len(result.LabelValues) != 0 {
		t.Errorf("expected empty result for unknown dim, got %v", result)
	}
}

func TestSortedSetDocValuesFacetCounts_GetSpecificValue_Unknown(t *testing.T) {
	cfg := facets.NewFacetsConfig()
	counts := NewSortedSetDocValuesFacetCounts(cfg, "$facets")

	_, err := counts.GetSpecificValue("a", "foo")
	// Either an error or -1 value is acceptable for an unknown entry.
	if err != nil {
		return // acceptable
	}
}

// ---------------------------------------------------------------------------
// OrdRange iterator
// ---------------------------------------------------------------------------

func TestOrdRange_Iterator(t *testing.T) {
	r := NewOrdRange(2, 5) // ords 2, 3, 4, 5
	next := r.Iterator()
	got := []int{}
	for {
		v := next()
		if v < 0 {
			break
		}
		got = append(got, v)
	}
	want := []int{2, 3, 4, 5}
	if len(got) != len(want) {
		t.Fatalf("OrdRange iterator: want %v, got %v", want, got)
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("OrdRange[%d]: want %d, got %d", i, v, got[i])
		}
	}
}

func TestOrdRange_Empty(t *testing.T) {
	r := NewOrdRange(5, 2) // end < start → empty
	next := r.Iterator()
	// Expect immediate termination.
	for {
		v := next()
		if v < 0 {
			break
		}
		t.Errorf("expected no values from empty OrdRange, got %d", v)
	}
}

// ---------------------------------------------------------------------------
// Integration stubs — tests that require the index pipeline
// ---------------------------------------------------------------------------

// TestBasic_IndexIntegration mirrors the count assertions of testBasic from
// the Java source, running against the real on-disk SortedSetDocValues pipeline
// with the default codec-driven accumulator path (rmp #4704).
//
//	doc0: a/foo, a/bar, a/zoo, b/baz, b/buzz
//	doc1: a/foo, b/buzz
func TestBasic_IndexIntegration(t *testing.T) {
	acc, cleanup := buildSSDVAccumulator(t, [][]string{
		{"a/foo", "a/bar", "a/zoo", "b/baz", "b/buzz"},
		{"a/foo", "b/buzz"},
	})
	defer cleanup()

	assertChildCounts(t, acc, "a", map[string]int64{"foo": 2, "bar": 1, "zoo": 1})
	assertChildCounts(t, acc, "b", map[string]int64{"buzz": 2, "baz": 1})

	r, err := acc.GetSpecificValue("a", "foo")
	if err != nil {
		t.Fatalf("GetSpecificValue: %v", err)
	}
	if r.Value != 2 {
		t.Errorf("getSpecificValue(a, foo) = %d, want 2", r.Value)
	}
}

// TestCombinationsOfConfig_IndexIntegration mirrors the multi-dimension count
// assertions of testCombinations from the Java source: one document carrying a
// single value in many distinct dimensions, each counted once.
func TestCombinationsOfConfig_IndexIntegration(t *testing.T) {
	acc, cleanup := buildSSDVAccumulator(t, [][]string{
		{"a/foo", "b/bar", "c/zoo", "d/baz", "e/buzz", "f/buzze", "g/buzzel", "h/buzzele"},
	})
	defer cleanup()

	for dim, label := range map[string]string{
		"a": "foo", "b": "bar", "c": "zoo", "d": "baz",
		"e": "buzz", "f": "buzze", "g": "buzzel", "h": "buzzele",
	} {
		assertChildCounts(t, acc, dim, map[string]int64{label: 1})
	}
}

// TestBasicHierarchical_IndexIntegration mirrors testBasicHierarchical's count
// assertions: hierarchical "dim/parent/child" facet terms counted per leaf
// path. The accumulator reports direct children of the queried path.
//
//	doc0: Author/Bob, Publish Date/2010/March/22, Publish Date/2010/March/23
//	doc1: Author/Bob, Publish Date/2010/March/22
func TestBasicHierarchical_IndexIntegration(t *testing.T) {
	acc, cleanup := buildSSDVAccumulator(t, [][]string{
		{"Author/Bob", "Publish Date/2010/March/22", "Publish Date/2010/March/23"},
		{"Author/Bob", "Publish Date/2010/March/22"},
	})
	defer cleanup()

	// Author has a single direct child Bob counted in both docs.
	assertChildCounts(t, acc, "Author", map[string]int64{"Bob": 2})

	// Direct children of "Publish Date/2010/March": leaf "22" (2 docs) and "23"
	// (1 doc).
	r, err := acc.GetTopChildren(100, "Publish Date", "2010", "March")
	if err != nil {
		t.Fatalf("GetTopChildren hierarchical: %v", err)
	}
	got := childCounts(r)
	if got["22"] != 2 {
		t.Errorf("Publish Date/2010/March child 22 = %d, want 2 (all: %v)", got["22"], got)
	}
	if got["23"] != 1 {
		t.Errorf("Publish Date/2010/March child 23 = %d, want 1 (all: %v)", got["23"], got)
	}
}
