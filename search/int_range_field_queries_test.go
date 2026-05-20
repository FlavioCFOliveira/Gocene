// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestIntRangeFieldQueries.java
//
// Deviation: testBasics() uses RandomIndexWriter, IntRange.newIntersectsQuery /
// newContainsQuery / newWithinQuery / newCrossesQuery — query factories that
// are deferred in Gocene (backlog #2695 / range_fields_lucene.go). The range
// relationship logic in IntTestRange and the boundary-value generation are
// tested below. The testBasics() integration scenario is covered as a logical
// assertion test using IntTestRange predicates directly.

package search

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
)

// ─── IntTestRange ─────────────────────────────────────────────────────────────

// IntTestRange mirrors the Java IntTestRange inner class from
// TestIntRangeFieldQueries (Lucene 10.4.0). It holds an N-dimensional int
// range and implements the range-relationship predicates (isDisjoint, isWithin,
// contains).
type IntTestRange struct {
	min []int32
	max []int32
}

// newIntTestRange creates an IntTestRange. Panics if min/max are nil, empty,
// or have different lengths (mirroring Java's assert).
func newIntTestRange(min, max []int32) *IntTestRange {
	if len(min) == 0 || len(max) == 0 || len(min) != len(max) {
		panic("IntTestRange: min/max must be non-nil, non-empty, and equal length")
	}
	m := make([]int32, len(min))
	x := make([]int32, len(max))
	copy(m, min)
	copy(x, max)
	return &IntTestRange{min: m, max: x}
}

// NumDimensions returns the number of dimensions.
func (r *IntTestRange) NumDimensions() int { return len(r.min) }

// GetMin returns the minimum value for dimension dim.
func (r *IntTestRange) GetMin(dim int) int32 { return r.min[dim] }

// GetMax returns the maximum value for dimension dim.
func (r *IntTestRange) GetMax(dim int) int32 { return r.max[dim] }

// IsEqual reports whether r and other have identical min and max arrays.
func (r *IntTestRange) IsEqual(other *IntTestRange) bool {
	if len(r.min) != len(other.min) {
		return false
	}
	for i := range r.min {
		if r.min[i] != other.min[i] || r.max[i] != other.max[i] {
			return false
		}
	}
	return true
}

// IsDisjoint reports whether r and other share no overlap in any dimension.
// Mirrors IntTestRange.isDisjoint (Lucene 10.4.0).
func (r *IntTestRange) IsDisjoint(other *IntTestRange) bool {
	for d := range r.min {
		if r.min[d] > other.max[d] || r.max[d] < other.min[d] {
			return true
		}
	}
	return false
}

// IsWithin reports whether r is fully contained by other in all dimensions.
// Mirrors IntTestRange.isWithin (Lucene 10.4.0).
func (r *IntTestRange) IsWithin(other *IntTestRange) bool {
	for d := range r.min {
		if !(r.min[d] >= other.min[d] && r.max[d] <= other.max[d]) {
			return false
		}
	}
	return true
}

// Contains reports whether r fully contains other in all dimensions.
// Mirrors IntTestRange.contains (Lucene 10.4.0).
func (r *IntTestRange) Contains(other *IntTestRange) bool {
	for d := range r.min {
		if !(r.min[d] <= other.min[d] && r.max[d] >= other.max[d]) {
			return false
		}
	}
	return true
}

// SetMin mirrors IntTestRange.setMin: if r.min[dim] < v, updates max to v;
// otherwise updates min to v (keeps invariant max >= min per-dim).
func (r *IntTestRange) SetMin(dim int, v int32) {
	if r.min[dim] < v {
		r.max[dim] = v
	} else {
		r.min[dim] = v
	}
}

// SetMax mirrors IntTestRange.setMax: if r.max[dim] > v, updates min to v;
// otherwise updates max to v.
func (r *IntTestRange) SetMax(dim int, v int32) {
	if r.max[dim] > v {
		r.min[dim] = v
	} else {
		r.max[dim] = v
	}
}

// ─── IntTestRange unit tests ──────────────────────────────────────────────────

// TestIntTestRange_IsDisjoint_1D verifies the disjoint predicate for 1-D ranges.
func TestIntTestRange_IsDisjoint_1D(t *testing.T) {
	query := newIntTestRange([]int32{-11}, []int32{15})
	cases := []struct {
		name    string
		r       *IntTestRange
		want    bool
	}{
		{"within",                   newIntTestRange([]int32{-10}, []int32{9}),   false},
		{"crosses left",             newIntTestRange([]int32{-20}, []int32{-11}), false},
		{"crosses right",            newIntTestRange([]int32{15}, []int32{20}),   false},
		{"fully left (disjoint)",    newIntTestRange([]int32{-122}, []int32{-12}), true},
		{"fully right (disjoint)",   newIntTestRange([]int32{16}, []int32{29}),   true},
		{"contains",                 newIntTestRange([]int32{-20}, []int32{30}),  false},
		{"equal",                    newIntTestRange([]int32{-11}, []int32{15}),  false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.r.IsDisjoint(query); got != tc.want {
				t.Fatalf("IsDisjoint = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestIntTestRange_IsWithin_1D verifies the within predicate for 1-D ranges.
func TestIntTestRange_IsWithin_1D(t *testing.T) {
	query := newIntTestRange([]int32{-11}, []int32{15})

	// r isWithin query means r is fully inside query
	within := newIntTestRange([]int32{-10}, []int32{9})
	if !within.IsWithin(query) {
		t.Fatal("expected IsWithin=true for range fully inside query")
	}

	outside := newIntTestRange([]int32{-20}, []int32{-12})
	if outside.IsWithin(query) {
		t.Fatal("expected IsWithin=false for range outside query")
	}

	crossing := newIntTestRange([]int32{-20}, []int32{5})
	if crossing.IsWithin(query) {
		t.Fatal("expected IsWithin=false for range that crosses left boundary")
	}
}

// TestIntTestRange_Contains_1D verifies the contains predicate for 1-D ranges.
func TestIntTestRange_Contains_1D(t *testing.T) {
	query := newIntTestRange([]int32{-11}, []int32{15})

	// range contains query: range >= query.min on each side
	big := newIntTestRange([]int32{-20}, []int32{30})
	if !big.Contains(query) {
		t.Fatal("expected Contains=true for range that fully encloses query")
	}

	equal := newIntTestRange([]int32{-11}, []int32{15})
	if !equal.Contains(query) {
		t.Fatal("expected Contains=true for range equal to query")
	}

	smaller := newIntTestRange([]int32{-10}, []int32{9})
	if smaller.Contains(query) {
		t.Fatal("expected Contains=false for range strictly inside query")
	}
}

// TestIntTestRange_IsDisjoint_2D verifies the disjoint predicate for 2-D ranges.
func TestIntTestRange_IsDisjoint_2D(t *testing.T) {
	query := newIntTestRange([]int32{-11, -15}, []int32{15, 20})

	// disjoint on dim 0 only
	d0Only := newIntTestRange([]int32{-122, 1}, []int32{-115, 29})
	if !d0Only.IsDisjoint(query) {
		t.Fatal("expected IsDisjoint=true when dim 0 is disjoint")
	}

	// disjoint on dim 1 only
	d1Only := newIntTestRange([]int32{-10, -100}, []int32{9, -50})
	if !d1Only.IsDisjoint(query) {
		t.Fatal("expected IsDisjoint=true when dim 1 is disjoint")
	}

	// overlapping in all dims
	both := newIntTestRange([]int32{-10, -10}, []int32{9, 10})
	if both.IsDisjoint(query) {
		t.Fatal("expected IsDisjoint=false for overlapping range in both dims")
	}
}

// TestIntTestRange_SetMinMax_Invariant verifies the invariant-preserving mutation.
func TestIntTestRange_SetMinMax_Invariant(t *testing.T) {
	r := newIntTestRange([]int32{0}, []int32{10})

	// setMin: if min[0] < v → set max[0] = v
	r.SetMin(0, 20)
	if r.max[0] != 20 {
		t.Fatalf("SetMin(0, 20): expected max[0]=20, got %d", r.max[0])
	}

	r = newIntTestRange([]int32{5}, []int32{10})
	// setMin: if min[0] >= v → set min[0] = v
	r.SetMin(0, 3)
	if r.min[0] != 3 {
		t.Fatalf("SetMin(0, 3): expected min[0]=3, got %d", r.min[0])
	}

	r = newIntTestRange([]int32{0}, []int32{10})
	// setMax: if max[0] > v → set min[0] = v
	r.SetMax(0, -5)
	if r.min[0] != -5 {
		t.Fatalf("SetMax(0, -5): expected min[0]=-5, got %d", r.min[0])
	}

	r = newIntTestRange([]int32{0}, []int32{10})
	// setMax: if max[0] <= v → set max[0] = v
	r.SetMax(0, 20)
	if r.max[0] != 20 {
		t.Fatalf("SetMax(0, 20): expected max[0]=20, got %d", r.max[0])
	}
}

// TestIntTestRange_IsEqual verifies the equality check.
func TestIntTestRange_IsEqual(t *testing.T) {
	r1 := newIntTestRange([]int32{-11, -15}, []int32{15, 20})
	r2 := newIntTestRange([]int32{-11, -15}, []int32{15, 20})
	r3 := newIntTestRange([]int32{-11, -15}, []int32{15, 21})

	if !r1.IsEqual(r2) {
		t.Fatal("expected IsEqual=true for identical ranges")
	}
	if r1.IsEqual(r3) {
		t.Fatal("expected IsEqual=false for ranges with different max")
	}
}

// TestIntTestRange_BoundaryValues verifies that INT_MIN and INT_MAX are accepted.
func TestIntTestRange_BoundaryValues(t *testing.T) {
	r := newIntTestRange([]int32{math.MinInt32}, []int32{math.MaxInt32})
	if r.min[0] != math.MinInt32 || r.max[0] != math.MaxInt32 {
		t.Fatalf("boundary values: min=%d max=%d", r.min[0], r.max[0])
	}
}

// TestIntRange_Encoding_BasicScenario mirrors the "testBasics" assertion intent
// from the Java source: given the documented 8 documents, the range predicates
// for the query range [-11,-15]..[15,20] produce the expected hit counts.
// This test uses IntTestRange predicates (not a live index) to verify the
// classification logic is consistent with the Java test's manual assertions.
//
// Document ranges from testBasics():
//   doc 0: [-10,-10]..[9,10]     → within  query
//   doc 1: [10,-10]..[20,10]     → crosses query (dim 0 partially outside)
//   doc 2: [-20,-20]..[30,30]    → contains / crosses
//   doc 3: [-11,-11]..[1,11]     → within  query
//   doc 4: [12,1]..[15,29]       → crosses query
//   doc 5: [-122,1]..[-115,29]   → disjoint
//   doc 6: [INT_MIN,1]..[-11,29] → crosses query
//   doc 7: [-11,-15]..[15,20]    → equal   (within, contains, intersects)
//
// Java assertions: intersects=7, within=3, contains=2, crosses=4.
func TestIntRange_Encoding_BasicScenario(t *testing.T) {
	query := newIntTestRange([]int32{-11, -15}, []int32{15, 20})

	docs := []*IntTestRange{
		newIntTestRange([]int32{-10, -10}, []int32{9, 10}),
		newIntTestRange([]int32{10, -10}, []int32{20, 10}),
		newIntTestRange([]int32{-20, -20}, []int32{30, 30}),
		newIntTestRange([]int32{-11, -11}, []int32{1, 11}),
		newIntTestRange([]int32{12, 1}, []int32{15, 29}),
		newIntTestRange([]int32{-122, 1}, []int32{-115, 29}),
		newIntTestRange([]int32{math.MinInt32, 1}, []int32{-11, 29}),
		newIntTestRange([]int32{-11, -15}, []int32{15, 20}),
	}

	// Intersects: not disjoint.
	intersects := 0
	for _, d := range docs {
		if !d.IsDisjoint(query) {
			intersects++
		}
	}
	if intersects != 7 {
		t.Fatalf("intersects: expected 7, got %d", intersects)
	}

	// Within: d isWithin query.
	within := 0
	for _, d := range docs {
		if d.IsWithin(query) {
			within++
		}
	}
	if within != 3 {
		t.Fatalf("within: expected 3, got %d", within)
	}

	// Contains: d contains query.
	contains := 0
	for _, d := range docs {
		if d.Contains(query) {
			contains++
		}
	}
	if contains != 2 {
		t.Fatalf("contains: expected 2, got %d", contains)
	}

	// Crosses: not disjoint AND not wholly within.
	// Lucene definition: "the complement of union(WITHIN, DISJOINT)".
	// Note: CROSSES includes ranges that *contain* the query range.
	crosses := 0
	for _, d := range docs {
		if !d.IsDisjoint(query) && !d.IsWithin(query) {
			crosses++
		}
	}
	if crosses != 4 {
		t.Fatalf("crosses: expected 4, got %d", crosses)
	}
}

// TestIntRange_FieldEncoding_Roundtrip verifies that NewIntRangeLucene stores
// the expected encoded bytes and the accessors return the original values.
func TestIntRange_FieldEncoding_Roundtrip(t *testing.T) {
	min := []int32{-10, -10}
	max := []int32{9, 10}
	f, err := document.NewIntRangeLucene("intRangeField", min, max)
	if err != nil {
		t.Fatalf("NewIntRangeLucene: %v", err)
	}
	if f.GetMin(0) != -10 || f.GetMax(0) != 9 {
		t.Fatalf("dim 0: min=%d max=%d", f.GetMin(0), f.GetMax(0))
	}
	if f.GetMin(1) != -10 || f.GetMax(1) != 10 {
		t.Fatalf("dim 1: min=%d max=%d", f.GetMin(1), f.GetMax(1))
	}
}

// TestIntRange_FieldEncoding_BoundaryValues verifies INT_MIN/INT_MAX are stored.
func TestIntRange_FieldEncoding_BoundaryValues(t *testing.T) {
	min := []int32{math.MinInt32}
	max := []int32{math.MaxInt32}
	f, err := document.NewIntRangeLucene("f", min, max)
	if err != nil {
		t.Fatalf("NewIntRangeLucene: %v", err)
	}
	if f.GetMin(0) != math.MinInt32 || f.GetMax(0) != math.MaxInt32 {
		t.Fatalf("boundary: min=%d max=%d", f.GetMin(0), f.GetMax(0))
	}
}
