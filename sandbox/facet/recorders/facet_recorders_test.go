// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.facet.TestFacetRecorders.
//
// Deviations from Java:
//   - All test methods require IndexSearcher / RandomIndexWriter,
//     DirectoryTaxonomyWriter/Reader, and the full Gocene search pipeline.
//     Deferred to backlog #2693.
//   - The present tests exercise the recorder structs directly (CountFacetRecorder,
//     LongAggregationsFacetRecorder, MultiFacetsRecorder, SumReducer) to verify
//     their recording and reduction logic in isolation.
package recorders

import (
	"testing"
)

// TestFacetRecorders_CountFacetRecorder verifies that CountFacetRecorder
// correctly accumulates counts per ordinal.
func TestFacetRecorders_CountFacetRecorder(t *testing.T) {
	r := NewCountFacetRecorder()
	r.Record(0)
	r.Record(3)
	r.Record(3)
	r.Record(1)

	if got := r.Counts[0]; got != 1 {
		t.Errorf("Counts[0] = %d; want 1", got)
	}
	if got := r.Counts[1]; got != 1 {
		t.Errorf("Counts[1] = %d; want 1", got)
	}
	if got := r.Counts[3]; got != 2 {
		t.Errorf("Counts[3] = %d; want 2", got)
	}
	// Unrecorded ordinal should be zero.
	if got := r.Counts[99]; got != 0 {
		t.Errorf("Counts[99] = %d; want 0", got)
	}
}

// TestFacetRecorders_LongAggregationsFacetRecorder verifies that
// LongAggregationsFacetRecorder accumulates int64 values per ordinal.
func TestFacetRecorders_LongAggregationsFacetRecorder(t *testing.T) {
	// Simulate a doc-values source: doc 0 → 10, doc 1 → 20, doc 2 → 5.
	values := map[int]int64{0: 10, 1: 20, 2: 5}
	r := NewLongAggregationsFacetRecorder(func(docID int) (int64, bool) {
		v, ok := values[docID]
		return v, ok
	})

	r.SetDoc(0)
	r.Record(7) // ordinal 7 gets doc 0's value: 10
	r.SetDoc(1)
	r.Record(7) // ordinal 7 gets doc 1's value: 20 → total 30
	r.SetDoc(2)
	r.Record(3) // ordinal 3 gets doc 2's value: 5

	if got := r.Sums[7]; got != 30 {
		t.Errorf("Aggregations[7] = %d; want 30", got)
	}
	if got := r.Sums[3]; got != 5 {
		t.Errorf("Aggregations[3] = %d; want 5", got)
	}
}

// TestFacetRecorders_MultiFacetsRecorder verifies that MultiFacetsRecorder
// forwards Record calls to all sub-recorders.
func TestFacetRecorders_MultiFacetsRecorder(t *testing.T) {
	r1 := NewCountFacetRecorder()
	r2 := NewCountFacetRecorder()
	multi := NewMultiFacetsRecorder(r1, r2)

	multi.Record(5)
	multi.Record(5)
	multi.Record(9)

	if r1.Counts[5] != 2 {
		t.Errorf("r1.Counts[5] = %d; want 2", r1.Counts[5])
	}
	if r2.Counts[5] != 2 {
		t.Errorf("r2.Counts[5] = %d; want 2", r2.Counts[5])
	}
	if r1.Counts[9] != 1 {
		t.Errorf("r1.Counts[9] = %d; want 1", r1.Counts[9])
	}
}

// TestFacetRecorders_SumReducer verifies that SumReducer returns the sum of
// all values.
func TestFacetRecorders_SumReducer(t *testing.T) {
	s := SumReducer{}
	if got := s.Reduce([]int64{1, 2, 3, 4}); got != 10 {
		t.Errorf("SumReducer.Reduce([1,2,3,4]) = %d; want 10", got)
	}
	if got := s.Reduce(nil); got != 0 {
		t.Errorf("SumReducer.Reduce(nil) = %d; want 0", got)
	}
	if got := s.Reduce([]int64{-5, 5}); got != 0 {
		t.Errorf("SumReducer.Reduce([-5,5]) = %d; want 0", got)
	}
}
