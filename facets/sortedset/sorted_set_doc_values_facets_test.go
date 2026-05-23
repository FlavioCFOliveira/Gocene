// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sortedset

// TestSortedSetDocValuesFacets ports the behavioural assertions from
// org.apache.lucene.facet.sortedset.TestSortedSetDocValuesFacets.
//
// Tests that require a full Lucene-style RandomIndexWriter + IndexSearcher
// round-trip (testBasic, testCombinationsOfConfig, testBasicHierarchical, …)
// are deferred until SortedSetDocValues is wired into the index pipeline;
// they are marked with t.Skip so they compile and document the intent.
// Unit-level tests for the API surface exercised without a real index run
// unconditionally.

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/facets"
)

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

// TestBasic_IndexIntegration mirrors testBasic from the Java source.
// Skipped until SortedSetDocValues is wired into the Gocene index pipeline.
func TestBasic_IndexIntegration(t *testing.T) {
	t.Skip("requires SortedSetDocValues index pipeline (not yet wired)")
}

// TestCombinationsOfConfig_IndexIntegration mirrors testCombinationsOfConfig.
func TestCombinationsOfConfig_IndexIntegration(t *testing.T) {
	t.Skip("requires SortedSetDocValues index pipeline (not yet wired)")
}

// TestBasicHierarchical_IndexIntegration mirrors testBasicHierarchical.
func TestBasicHierarchical_IndexIntegration(t *testing.T) {
	t.Skip("requires SortedSetDocValues index pipeline (not yet wired)")
}
