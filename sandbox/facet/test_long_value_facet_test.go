// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.facet.TestLongValueFacet.
//
// Deviations from Java:
//   - testBasic, testOnlyBigLongs, testRandomSingleValued,
//     testRandomMultiValued, testDuplicateLongValues all require
//     IndexSearcher / RandomIndexWriter and the full facet pipeline with
//     NumericDocValues. Deferred to backlog #2693.
//   - The present tests exercise the LongAggregationsFacetRecorder struct
//     (the recorder backing long-value facets) in isolation.
package facet

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/sandbox/facet/recorders"
)

// TestLongValueFacet_RecorderBasic exercises LongAggregationsFacetRecorder
// with values analogous to the "Units" numeric field in testBasic.
func TestLongValueFacet_RecorderBasic(t *testing.T) {
	// Simulate "Publish Date" ordinals 1 and 2, with per-doc values from a
	// numeric doc-values field.
	docValues := map[int]int64{0: 9, 1: 3, 2: 7, 3: 4, 4: 5, 5: 6}
	r := recorders.NewLongAggregationsFacetRecorder(func(docID int) (int64, bool) {
		v, ok := docValues[docID]
		return v, ok
	})

	// Doc 0 → ordinal 1, doc 1 → ordinal 2, ...
	for docID := 0; docID < 6; docID++ {
		r.SetDoc(docID)
		r.Record(docID % 2) // alternate between ordinals 0 and 1
	}

	// Ordinal 0: docs 0, 2, 4 → values 9, 7, 5 → sum 21
	if got := r.Sums[0]; got != 21 {
		t.Errorf("Sums[0] = %d; want 21", got)
	}
	// Ordinal 1: docs 1, 3, 5 → values 3, 4, 6 → sum 13
	if got := r.Sums[1]; got != 13 {
		t.Errorf("Sums[1] = %d; want 13", got)
	}
}

// TestLongValueFacet_RecorderBigLongs mirrors testOnlyBigLongs: records int64
// values at the extremes.
func TestLongValueFacet_RecorderBigLongs(t *testing.T) {
	const minLong int64 = -9223372036854775808
	const maxLong int64 = 9223372036854775807
	docValues := map[int]int64{0: minLong, 1: maxLong}
	r := recorders.NewLongAggregationsFacetRecorder(func(docID int) (int64, bool) {
		v, ok := docValues[docID]
		return v, ok
	})
	r.SetDoc(0)
	r.Record(0)
	r.SetDoc(1)
	r.Record(1)

	if r.Sums[0] != minLong {
		t.Errorf("Sums[0] = %d; want %d", r.Sums[0], minLong)
	}
	if r.Sums[1] != maxLong {
		t.Errorf("Sums[1] = %d; want %d", r.Sums[1], maxLong)
	}
}
