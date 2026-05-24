// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package utils

import (
	"testing"
)

// stubRecorder holds a static ord→count map and a fixed iteration order.
type stubRecorder struct {
	counts map[int]int
	ords   []int
}

func (r *stubRecorder) RecordedOrds() OrdinalIterEx {
	return &sliceIter{ords: r.ords}
}

func (r *stubRecorder) GetCount(ord int) int { return r.counts[ord] }

type sliceIter struct {
	ords []int
	pos  int
}

func (s *sliceIter) NextOrd() int {
	if s.pos >= len(s.ords) {
		return NoMoreOrdsEx
	}
	v := s.ords[s.pos]
	s.pos++
	return v
}

// stubLabel maps ordinals to strings "ord<n>".
type stubLabel struct{}

func (stubLabel) GetLabel(ord int) string {
	return "ord" + string(rune('0'+ord))
}

// TestBaseFacetBuilderConfig_DefaultSortByCount verifies that the default sort
// is by descending count and that all matching ordinals are included.
func TestBaseFacetBuilderConfig_DefaultSortByCount(t *testing.T) {
	rec := &stubRecorder{
		counts: map[int]int{0: 5, 1: 10, 2: 3},
		ords:   []int{0, 1, 2},
	}
	cfg := NewBaseFacetBuilderConfig("dim")
	fr := cfg.GetResult(rec, 18, stubLabel{}, rec.RecordedOrds())

	if fr.Dim != "dim" {
		t.Errorf("Dim = %q; want %q", fr.Dim, "dim")
	}
	if fr.Value != 18 {
		t.Errorf("Value = %d; want 18", fr.Value)
	}
	if len(fr.LabelValues) != 3 {
		t.Fatalf("LabelValues len = %d; want 3", len(fr.LabelValues))
	}
	// Descending count: 10, 5, 3
	wantCounts := []int64{10, 5, 3}
	for i, lv := range fr.LabelValues {
		if lv.Value != wantCounts[i] {
			t.Errorf("LabelValues[%d].Value = %d; want %d", i, lv.Value, wantCounts[i])
		}
	}
}

// TestBaseFacetBuilderConfig_WithTopN verifies that topN limits the results.
func TestBaseFacetBuilderConfig_WithTopN(t *testing.T) {
	rec := &stubRecorder{
		counts: map[int]int{0: 5, 1: 10, 2: 3},
		ords:   []int{0, 1, 2},
	}
	cfg := NewBaseFacetBuilderConfig("dim").WithTopN(2)
	fr := cfg.GetResult(rec, 18, stubLabel{}, rec.RecordedOrds())

	if len(fr.LabelValues) != 2 {
		t.Fatalf("LabelValues len = %d; want 2", len(fr.LabelValues))
	}
	// Top 2 by count: 10, 5
	if fr.LabelValues[0].Value != 10 {
		t.Errorf("LabelValues[0].Value = %d; want 10", fr.LabelValues[0].Value)
	}
	if fr.LabelValues[1].Value != 5 {
		t.Errorf("LabelValues[1].Value = %d; want 5", fr.LabelValues[1].Value)
	}
}

// TestBaseFacetBuilderConfig_WithSortByOrdinal verifies ascending ordinal sort.
func TestBaseFacetBuilderConfig_WithSortByOrdinal(t *testing.T) {
	rec := &stubRecorder{
		counts: map[int]int{0: 5, 2: 10, 4: 3},
		ords:   []int{4, 2, 0},
	}
	cfg := NewBaseFacetBuilderConfig("dim").WithSortByOrdinal()
	fr := cfg.GetResult(rec, 18, stubLabel{}, rec.RecordedOrds())

	if len(fr.LabelValues) != 3 {
		t.Fatalf("LabelValues len = %d; want 3", len(fr.LabelValues))
	}
	// Ascending ordinal: 0, 2, 4
	wantOrds := []int64{5, 10, 3} // counts at ords 0, 2, 4
	for i, lv := range fr.LabelValues {
		if lv.Value != wantOrds[i] {
			t.Errorf("LabelValues[%d].Value = %d; want %d (by ordinal order)", i, lv.Value, wantOrds[i])
		}
	}
}

// TestBaseFacetBuilderConfig_EmptyOrds verifies a result with no ordinals.
func TestBaseFacetBuilderConfig_EmptyOrds(t *testing.T) {
	rec := &stubRecorder{
		counts: map[int]int{},
		ords:   []int{},
	}
	cfg := NewBaseFacetBuilderConfig("dim", "path1")
	fr := cfg.GetResult(rec, 0, stubLabel{}, rec.RecordedOrds())

	if fr.ChildCount != 0 {
		t.Errorf("ChildCount = %d; want 0", fr.ChildCount)
	}
	if len(fr.LabelValues) != 0 {
		t.Errorf("LabelValues = %v; want empty", fr.LabelValues)
	}
	if len(fr.Path) != 1 || fr.Path[0] != "path1" {
		t.Errorf("Path = %v; want [path1]", fr.Path)
	}
}

// TestBaseFacetBuilderConfig_WithSortByCountRestores verifies that
// WithSortByCount re-applies count sort after calling WithSortByOrdinal.
func TestBaseFacetBuilderConfig_WithSortByCountRestores(t *testing.T) {
	rec := &stubRecorder{
		counts: map[int]int{0: 1, 1: 3, 2: 2},
		ords:   []int{0, 1, 2},
	}
	cfg := NewBaseFacetBuilderConfig("dim").WithSortByOrdinal().WithSortByCount()
	fr := cfg.GetResult(rec, 6, stubLabel{}, rec.RecordedOrds())

	if len(fr.LabelValues) != 3 {
		t.Fatalf("LabelValues len = %d; want 3", len(fr.LabelValues))
	}
	if fr.LabelValues[0].Value != 3 {
		t.Errorf("LabelValues[0].Value = %d; want 3 (highest count first)", fr.LabelValues[0].Value)
	}
}
