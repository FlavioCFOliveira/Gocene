// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.search.TestMultiRangeQueries.
//
// Deviations from Java:
//   - Tests that require IndexSearcher / RandomIndexWriter (testDuelWithStandardDisjunction,
//     testDoubleRandomMultiRangeQuery, testLongRandomMultiRangeQuery, testFloatRandomMultiRangeQuery,
//     testFloatPointMultiRangeQuery, testLongPointMultiRangeQuery, testDoublePointMultiRangeQuery,
//     testIntPointMultiRangeQuery, testRandomRewrite, testOneDimensionCount, testIntRandomMultiRangeQuery)
//     are deferred to backlog until the Gocene IndexSearcher / MultiRangeQuery search pipeline is
//     available (backlog #2693).
//   - testToString and testEqualsAndHashCode are deferred for the same reason; the builder stubs
//     do not yet expose Build() or an equality contract.
//   - The present tests exercise the builder accumulation API (AddRange) and the structural
//     properties of the multi-range builder types.
package search

import (
	"testing"

	document_sb "github.com/FlavioCFOliveira/Gocene/sandbox/document"
)

// TestMultiRangeQueries_DoublePointBuilderAccumulates verifies that
// DoublePointMultiRangeBuilder.AddRange accumulates ranges correctly.
func TestMultiRangeQueries_DoublePointBuilderAccumulates(t *testing.T) {
	b := document_sb.NewDoublePointMultiRangeBuilder("point")
	b.AddRange([]float64{111.3, 294.2, 502.8}, []float64{117.3, 301.4, 514.5})
	b.AddRange([]float64{15.3, 4.5, 415.7}, []float64{200.2, 402.4, 583.6})

	if len(b.Ranges) != 2 {
		t.Fatalf("expected 2 ranges, got %d", len(b.Ranges))
	}
	if b.Ranges[0].Min[0] != 111.3 {
		t.Errorf("range[0].Min[0] = %v; want 111.3", b.Ranges[0].Min[0])
	}
	if b.Ranges[1].Max[2] != 583.6 {
		t.Errorf("range[1].Max[2] = %v; want 583.6", b.Ranges[1].Max[2])
	}
}

// TestMultiRangeQueries_LongPointBuilderAccumulates verifies that
// LongPointMultiRangeBuilder.AddRange accumulates ranges correctly.
func TestMultiRangeQueries_LongPointBuilderAccumulates(t *testing.T) {
	b := document_sb.NewLongPointMultiRangeBuilder("point")
	b.AddRange([]int64{111, 294, 502}, []int64{117, 301, 514})
	b.AddRange([]int64{15, 412, 415}, []int64{200, 567, 642})

	if len(b.Ranges) != 2 {
		t.Fatalf("expected 2 ranges, got %d", len(b.Ranges))
	}
	if b.Ranges[0].Min[0] != 111 {
		t.Errorf("range[0].Min[0] = %d; want 111", b.Ranges[0].Min[0])
	}
	if b.Ranges[1].Max[1] != 567 {
		t.Errorf("range[1].Max[1] = %d; want 567", b.Ranges[1].Max[1])
	}
}

// TestMultiRangeQueries_FloatPointBuilderAccumulates verifies that
// FloatPointMultiRangeBuilder.AddRange accumulates ranges correctly.
func TestMultiRangeQueries_FloatPointBuilderAccumulates(t *testing.T) {
	b := document_sb.NewFloatPointMultiRangeBuilder("point")
	b.AddRange([]float32{111.3, 294.4, 502.2}, []float32{117.7, 301.2, 514.4})
	b.AddRange([]float32{15.3, 412.2, 415.9}, []float32{200.2, 567.4, 642.3})

	if len(b.Ranges) != 2 {
		t.Fatalf("expected 2 ranges, got %d", len(b.Ranges))
	}
	if b.Ranges[0].Min[0] != 111.3 {
		t.Errorf("range[0].Min[0] = %v; want 111.3", b.Ranges[0].Min[0])
	}
}

// TestMultiRangeQueries_IntPointBuilderAccumulates verifies that
// IntPointMultiRangeBuilder.AddRange accumulates ranges correctly.
func TestMultiRangeQueries_IntPointBuilderAccumulates(t *testing.T) {
	b := document_sb.NewIntPointMultiRangeBuilder("point")
	b.AddRange([]int32{111, 294, 502}, []int32{117, 301, 514})
	b.AddRange([]int32{15, 412, 415}, []int32{200, 567, 642})

	if len(b.Ranges) != 2 {
		t.Fatalf("expected 2 ranges, got %d", len(b.Ranges))
	}
	if b.Ranges[1].Min[2] != 415 {
		t.Errorf("range[1].Min[2] = %d; want 415", b.Ranges[1].Min[2])
	}
}

// TestMultiRangeQueries_BuilderIsolation verifies that AddRange copies the
// slices so that later mutations of the original arrays do not affect the
// builder state.
func TestMultiRangeQueries_BuilderIsolation(t *testing.T) {
	b := document_sb.NewLongPointMultiRangeBuilder("point")
	lower := []int64{10, 20}
	upper := []int64{30, 40}
	b.AddRange(lower, upper)

	// Mutate originals after AddRange.
	lower[0] = 999
	upper[1] = 888

	if b.Ranges[0].Min[0] != 10 {
		t.Errorf("builder was not isolated from caller slice: Min[0] = %d; want 10", b.Ranges[0].Min[0])
	}
	if b.Ranges[0].Max[1] != 40 {
		t.Errorf("builder was not isolated from caller slice: Max[1] = %d; want 40", b.Ranges[0].Max[1])
	}
}

// TestMultiRangeQueries_EmptyBuilder verifies a freshly created builder has
// no ranges.
func TestMultiRangeQueries_EmptyBuilder(t *testing.T) {
	tests := []struct {
		name  string
		count int
	}{
		{"double", len(document_sb.NewDoublePointMultiRangeBuilder("f").Ranges)},
		{"long", len(document_sb.NewLongPointMultiRangeBuilder("f").Ranges)},
		{"float", len(document_sb.NewFloatPointMultiRangeBuilder("f").Ranges)},
		{"int", len(document_sb.NewIntPointMultiRangeBuilder("f").Ranges)},
	}
	for _, tc := range tests {
		if tc.count != 0 {
			t.Errorf("%s builder: expected 0 ranges initially, got %d", tc.name, tc.count)
		}
	}
}
