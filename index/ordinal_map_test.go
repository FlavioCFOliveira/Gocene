// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newSDV is a test-only alias for stubSortedDocValues (declared in
// sorted_doc_values_terms_enum_test.go).
func newSDV(terms ...string) SortedDocValues { return newStubSDV(terms...) }

// ---------------------------------------------------------------------------
// segmentMap
// ---------------------------------------------------------------------------

func TestSegmentMap_DescendingWeightOrder(t *testing.T) {
	t.Parallel()
	weights := []int64{10, 50, 30}
	sm := buildSegmentMap(weights)
	// Expected sorted order (desc): segment 1 (50), segment 2 (30), segment 0 (10)
	wantNewToOld := []int{1, 2, 0}
	for i, got := range sm.newToOld {
		if got != wantNewToOld[i] {
			t.Errorf("newToOld[%d] = %d, want %d", i, got, wantNewToOld[i])
		}
	}
	// oldToNew must be inverse
	for newIdx, oldIdx := range sm.newToOld {
		if sm.oldToNew[oldIdx] != newIdx {
			t.Errorf("oldToNew[%d] = %d, want %d", oldIdx, sm.oldToNew[oldIdx], newIdx)
		}
	}
}

func TestSegmentMap_EqualWeights(t *testing.T) {
	t.Parallel()
	weights := []int64{5, 5, 5}
	sm := buildSegmentMap(weights)
	// Stable sort must keep original order
	for i, oldIdx := range sm.newToOld {
		if oldIdx != i {
			t.Errorf("newToOld[%d] = %d, want %d (stable)", i, oldIdx, i)
		}
	}
}

func TestSegmentMap_SingleSegment(t *testing.T) {
	t.Parallel()
	sm := buildSegmentMap([]int64{100})
	if sm.newToOld[0] != 0 || sm.oldToNew[0] != 0 {
		t.Errorf("single-segment map wrong: newToOld=%v oldToNew=%v", sm.newToOld, sm.oldToNew)
	}
}

// ---------------------------------------------------------------------------
// OrdinalMap.Build — single segment
// ---------------------------------------------------------------------------

func TestBuildOrdinalMapFromSortedValues_SingleSegment(t *testing.T) {
	t.Parallel()
	sdv := newSDV("bar", "cat", "dog", "foo")
	owner := NewCacheKey()

	om, err := BuildOrdinalMapFromSortedValues(owner, []SortedDocValues{sdv}, 0)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := om.GetValueCount(); got != 4 {
		t.Errorf("GetValueCount = %d, want 4", got)
	}

	// Single segment: globalOrds must be identity (segOrd == globalOrd).
	globalOrds := om.GetGlobalOrds(0)
	if len(globalOrds) != 4 {
		t.Fatalf("GetGlobalOrds(0) len = %d, want 4", len(globalOrds))
	}
	for i, g := range globalOrds {
		if g != int64(i) {
			t.Errorf("GetGlobalOrds(0)[%d] = %d, want %d", i, g, i)
		}
	}

	// GetFirstSegmentOrd and GetFirstSegmentNumber for every global ord.
	for i := int64(0); i < 4; i++ {
		if got := om.GetFirstSegmentOrd(i); got != i {
			t.Errorf("GetFirstSegmentOrd(%d) = %d, want %d", i, got, i)
		}
		if got := om.GetFirstSegmentNumber(i); got != 0 {
			t.Errorf("GetFirstSegmentNumber(%d) = %d, want 0", i, got)
		}
	}
}

func TestBuildOrdinalMapFromSortedValues_EmptySegment(t *testing.T) {
	t.Parallel()
	sdv := newSDV()
	owner := NewCacheKey()

	om, err := BuildOrdinalMapFromSortedValues(owner, []SortedDocValues{sdv}, 0)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := om.GetValueCount(); got != 0 {
		t.Errorf("GetValueCount = %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// OrdinalMap.Build — two segments (classic merge case)
// ---------------------------------------------------------------------------

// Global mapping for: bar→0, cat→1, dog→2, foo→3
// Segment 0: bar→0, foo→1
// Segment 1: cat→0, dog→1
//
// Expected segmentToGlobalOrds (sorted order = desc weight = 0,1 or 1,0?):
// Both have weight 2, so stable sort keeps order: sorted[0]=seg0, sorted[1]=seg1.
//
// seg0 mapped: bar→0 (delta 0), foo→1 (delta 2)
// seg1 mapped: cat→1 (delta 1), dog→2 (delta 1)

func TestBuildOrdinalMapFromSortedValues_TwoSegments(t *testing.T) {
	t.Parallel()
	seg0 := newSDV("bar", "foo")
	seg1 := newSDV("cat", "dog")
	owner := NewCacheKey()

	om, err := BuildOrdinalMapFromSortedValues(owner, []SortedDocValues{seg0, seg1}, 0)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := om.GetValueCount(); got != 4 {
		t.Errorf("GetValueCount = %d, want 4", got)
	}

	// Check GetGlobalOrds for segment 0.
	// seg0 has terms: bar(ord0)→global0, foo(ord1)→global3
	g0 := om.GetGlobalOrds(0)
	if len(g0) != 2 {
		t.Fatalf("GetGlobalOrds(0) len = %d, want 2", len(g0))
	}
	if g0[0] != 0 { // bar → global 0
		t.Errorf("GetGlobalOrds(0)[0] = %d, want 0 (bar)", g0[0])
	}
	if g0[1] != 3 { // foo → global 3
		t.Errorf("GetGlobalOrds(0)[1] = %d, want 3 (foo)", g0[1])
	}

	// Check GetGlobalOrds for segment 1.
	// seg1 has terms: cat(ord0)→global1, dog(ord1)→global2
	g1 := om.GetGlobalOrds(1)
	if len(g1) != 2 {
		t.Fatalf("GetGlobalOrds(1) len = %d, want 2", len(g1))
	}
	if g1[0] != 1 { // cat → global 1
		t.Errorf("GetGlobalOrds(1)[0] = %d, want 1 (cat)", g1[0])
	}
	if g1[1] != 2 { // dog → global 2
		t.Errorf("GetGlobalOrds(1)[1] = %d, want 2 (dog)", g1[1])
	}

	// GetFirstSegmentOrd.
	// bar(0): first seg is seg0 (sorted idx 0), segOrd=0 → firstSegOrd = 0-0=0
	if got := om.GetFirstSegmentOrd(0); got != 0 {
		t.Errorf("GetFirstSegmentOrd(0) = %d, want 0", got)
	}
	// cat(1): first seg is seg1 (sorted idx 1), segOrd=0 → globalOrd(1)-delta(1)=0
	if got := om.GetFirstSegmentOrd(1); got != 0 {
		t.Errorf("GetFirstSegmentOrd(1) = %d, want 0", got)
	}
	// dog(2): first seg is seg1, segOrd=1 → 2-1=1
	if got := om.GetFirstSegmentOrd(2); got != 1 {
		t.Errorf("GetFirstSegmentOrd(2) = %d, want 1", got)
	}
	// foo(3): first seg is seg0, segOrd=1 → 3-2=1
	if got := om.GetFirstSegmentOrd(3); got != 1 {
		t.Errorf("GetFirstSegmentOrd(3) = %d, want 1", got)
	}

	// GetFirstSegmentNumber.
	if got := om.GetFirstSegmentNumber(0); got != 0 { // bar in seg0
		t.Errorf("GetFirstSegmentNumber(0) = %d, want 0", got)
	}
	if got := om.GetFirstSegmentNumber(1); got != 1 { // cat in seg1
		t.Errorf("GetFirstSegmentNumber(1) = %d, want 1", got)
	}
	if got := om.GetFirstSegmentNumber(2); got != 1 { // dog in seg1
		t.Errorf("GetFirstSegmentNumber(2) = %d, want 1", got)
	}
	if got := om.GetFirstSegmentNumber(3); got != 0 { // foo in seg0
		t.Errorf("GetFirstSegmentNumber(3) = %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// OrdinalMap.Build — overlapping terms across segments
// ---------------------------------------------------------------------------

func TestBuildOrdinalMapFromSortedValues_OverlappingTerms(t *testing.T) {
	t.Parallel()
	// seg0: apple, banana, cherry
	// seg1: banana, cherry, date
	// Global: apple→0, banana→1, cherry→2, date→3
	seg0 := newSDV("apple", "banana", "cherry")
	seg1 := newSDV("banana", "cherry", "date")
	owner := NewCacheKey()

	om, err := BuildOrdinalMapFromSortedValues(owner, []SortedDocValues{seg0, seg1}, 0)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := om.GetValueCount(); got != 4 {
		t.Errorf("GetValueCount = %d, want 4", got)
	}

	// seg0: apple(0)→0, banana(1)→1, cherry(2)→2
	g0 := om.GetGlobalOrds(0)
	wantG0 := []int64{0, 1, 2}
	for i, want := range wantG0 {
		if g0[i] != want {
			t.Errorf("seg0 GetGlobalOrds[%d] = %d, want %d", i, g0[i], want)
		}
	}

	// seg1: banana(0)→1, cherry(1)→2, date(2)→3
	g1 := om.GetGlobalOrds(1)
	wantG1 := []int64{1, 2, 3}
	for i, want := range wantG1 {
		if g1[i] != want {
			t.Errorf("seg1 GetGlobalOrds[%d] = %d, want %d", i, g1[i], want)
		}
	}
}

// ---------------------------------------------------------------------------
// OrdinalMap.Build — three segments with weight ordering
// ---------------------------------------------------------------------------

func TestBuildOrdinalMapFromSortedValues_ThreeSegments_WeightOrdering(t *testing.T) {
	t.Parallel()
	// seg0 (weight 1): z
	// seg1 (weight 3): a, b, c
	// seg2 (weight 2): b, c
	// Global: a→0, b→1, c→2, z→3
	// Weight order descending: seg1(3), seg2(2), seg0(1) → sorted indices 0,1,2 map to orig 1,2,0
	seg0 := newSDV("z")
	seg1 := newSDV("a", "b", "c")
	seg2 := newSDV("b", "c")
	owner := NewCacheKey()

	om, err := BuildOrdinalMapFromSortedValues(owner, []SortedDocValues{seg0, seg1, seg2}, 0)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := om.GetValueCount(); got != 4 {
		t.Errorf("GetValueCount = %d, want 4", got)
	}

	// seg0: z(0)→3
	g0 := om.GetGlobalOrds(0)
	if len(g0) != 1 || g0[0] != 3 {
		t.Errorf("seg0 globalOrds = %v, want [3]", g0)
	}
	// seg1: a(0)→0, b(1)→1, c(2)→2
	g1 := om.GetGlobalOrds(1)
	if len(g1) != 3 || g1[0] != 0 || g1[1] != 1 || g1[2] != 2 {
		t.Errorf("seg1 globalOrds = %v, want [0 1 2]", g1)
	}
	// seg2: b(0)→1, c(1)→2
	g2 := om.GetGlobalOrds(2)
	if len(g2) != 2 || g2[0] != 1 || g2[1] != 2 {
		t.Errorf("seg2 globalOrds = %v, want [1 2]", g2)
	}
}

// ---------------------------------------------------------------------------
// OrdinalMap.Build — SortedSetDocValues path
// ---------------------------------------------------------------------------

// ordMapStubSSV is a minimal SortedSetDocValues backed by a sorted term list.
// Only LookupOrd and GetValueCount are used by the OrdinalMap build path.
// Named distinctly to avoid collision with stubSortedSetDocValues in
// sorted_set_doc_values_terms_enum_test.go.
type ordMapStubSSV struct {
	terms [][]byte
}

func (s *ordMapStubSSV) Get(int) ([]int, error)              { return nil, nil }
func (s *ordMapStubSSV) Advance(int) (int, error)            { return -1, nil }
func (s *ordMapStubSSV) AdvanceExact(int) (bool, error)      { return false, nil }
func (s *ordMapStubSSV) NextOrd() (int, error)               { return -1, nil }
func (s *ordMapStubSSV) NextDoc() (int, error)               { return -1, nil }
func (s *ordMapStubSSV) DocID() int                          { return -1 }
func (s *ordMapStubSSV) LookupOrd(ord int) ([]byte, error) {
	if ord < 0 || ord >= len(s.terms) {
		return nil, nil
	}
	return s.terms[ord], nil
}
func (s *ordMapStubSSV) GetValueCount() int { return len(s.terms) }

func newOrdMapSSV(terms ...string) SortedSetDocValues {
	out := make([][]byte, len(terms))
	for i, t := range terms {
		out[i] = []byte(t)
	}
	return &ordMapStubSSV{terms: out}
}

func TestBuildOrdinalMapFromSortedSetValues_TwoSegments(t *testing.T) {
	t.Parallel()
	seg0 := newOrdMapSSV("bar", "foo")
	seg1 := newOrdMapSSV("cat", "dog")
	owner := NewCacheKey()

	om, err := BuildOrdinalMapFromSortedSetValues(owner, []SortedSetDocValues{seg0, seg1}, 0)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := om.GetValueCount(); got != 4 {
		t.Errorf("GetValueCount = %d, want 4", got)
	}

	g0 := om.GetGlobalOrds(0)
	if len(g0) != 2 || g0[0] != 0 || g0[1] != 3 {
		t.Errorf("seg0 globalOrds = %v, want [0 3]", g0)
	}
	g1 := om.GetGlobalOrds(1)
	if len(g1) != 2 || g1[0] != 1 || g1[1] != 2 {
		t.Errorf("seg1 globalOrds = %v, want [1 2]", g1)
	}
}

// ---------------------------------------------------------------------------
// Boundary / edge cases
// ---------------------------------------------------------------------------

func TestOrdinalMap_OutOfRangeQueries(t *testing.T) {
	t.Parallel()
	om, err := BuildOrdinalMapFromSortedValues(NewCacheKey(), []SortedDocValues{newSDV("x")}, 0)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if got := om.GetGlobalOrds(-1); got != nil {
		t.Errorf("GetGlobalOrds(-1) = %v, want nil", got)
	}
	if got := om.GetGlobalOrds(99); got != nil {
		t.Errorf("GetGlobalOrds(99) = %v, want nil", got)
	}
	if got := om.GetFirstSegmentOrd(-1); got != -1 {
		t.Errorf("GetFirstSegmentOrd(-1) = %d, want -1", got)
	}
	if got := om.GetFirstSegmentOrd(99); got != -1 {
		t.Errorf("GetFirstSegmentOrd(99) = %d, want -1", got)
	}
	if got := om.GetFirstSegmentNumber(-1); got != -1 {
		t.Errorf("GetFirstSegmentNumber(-1) = %d, want -1", got)
	}
	if got := om.GetFirstSegmentNumber(99); got != -1 {
		t.Errorf("GetFirstSegmentNumber(99) = %d, want -1", got)
	}
}

func TestOrdinalMap_EmptyInput(t *testing.T) {
	t.Parallel()
	om, err := BuildOrdinalMapFromSortedValues(NewCacheKey(), []SortedDocValues{}, 0)
	if err != nil {
		t.Fatalf("Build empty: %v", err)
	}
	if got := om.GetValueCount(); got != 0 {
		t.Errorf("GetValueCount = %d, want 0", got)
	}
}

func TestOrdinalMap_RAMBytesUsed_NonZero(t *testing.T) {
	t.Parallel()
	seg0 := newSDV("a", "b")
	seg1 := newSDV("b", "c")
	om, err := BuildOrdinalMapFromSortedValues(NewCacheKey(), []SortedDocValues{seg0, seg1}, 0)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if om.RAMBytesUsed() <= 0 {
		t.Errorf("RAMBytesUsed = %d, want > 0", om.RAMBytesUsed())
	}
}
