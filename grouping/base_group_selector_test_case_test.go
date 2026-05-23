package grouping

// baseGroupSelectorTestCase is the Go counterpart of
// org.apache.lucene.search.grouping.BaseGroupSelectorTestCase from Lucene 10.4.0.
//
// The Java original is an abstract base class whose concrete subclasses plug in
// a GroupSelector and execute integration tests against a RandomIndexWriter +
// IndexSearcher. In Gocene's current stage the full search stack is not wired
// into the grouping package, so the integration test methods
// (testSortByRelevance, testSortGroups, testSortWithinGroups, testGroupHeads,
// testGroupHeadsWithSort) are deferred.
//
// What is exercised here:
//   - The sharded-merge contract (testShardedGrouping → mergeSearchGroupsSharded)
//   - The ignore-docs-without-group-field selector contract
//     (testIgnoreDocsWithoutGroupField → testGroupSelectorEmptyExclusion)

import (
	"testing"
)

// groupSelectorTestCase is the Go interface that concrete selector test cases
// must implement, mirroring the abstract methods of BaseGroupSelectorTestCase.
type groupSelectorTestCase[T comparable] interface {
	// newGroupSelector returns a fresh GroupSelector for this test case.
	newGroupSelector() GroupSelector
	// valueForDoc returns the simulated double/long value (and a bool indicating
	// whether the document has a value) for a given doc ID.
	valueForDoc(doc int) (T, bool)
}

// TestBaseGroupSelectorTestCase_MergeSearchGroupsSharded reproduces the
// sharded-grouping merge assertion from testShardedGrouping: groups collected
// from individual shards, then merged with MergeSearchGroups, must equal the
// groups that would be collected from the combined index.
//
// This test uses string group keys (term-based grouping) and a simple
// string-order comparator, keeping it independent of the search layer.
func TestBaseGroupSelectorTestCase_MergeSearchGroupsSharded(t *testing.T) {
	// Simulate four shards, each contributing two groups.
	// Combined, the top-5 groups in sort order should be a-e.
	shards := [][]*SearchGroup[string]{
		{
			{GroupValue: "a", SortValues: []any{"a"}},
			{GroupValue: "c", SortValues: []any{"c"}},
		},
		{
			{GroupValue: "b", SortValues: []any{"b"}},
			{GroupValue: "d", SortValues: []any{"d"}},
		},
		{
			{GroupValue: "e", SortValues: []any{"e"}},
			{GroupValue: "g", SortValues: []any{"g"}},
		},
		{
			{GroupValue: "f", SortValues: []any{"f"}},
			{GroupValue: "h", SortValues: []any{"h"}},
		},
	}

	merged := MergeSearchGroups(shards, 0, 5, compareStrAsc)
	if len(merged) != 5 {
		t.Fatalf("expected 5 merged groups, got %d: %v", len(merged), merged)
	}
	want := []string{"a", "b", "c", "d", "e"}
	for i, g := range merged {
		if g.GroupValue != want[i] {
			t.Errorf("[%d] GroupValue = %q, want %q", i, g.GroupValue, want[i])
		}
	}
}

// TestBaseGroupSelectorTestCase_MergeSearchGroupsShardedWithOffset reproduces
// the offset variant of the sharded grouping merge.
func TestBaseGroupSelectorTestCase_MergeSearchGroupsShardedWithOffset(t *testing.T) {
	shard := []*SearchGroup[string]{
		{GroupValue: "a", SortValues: []any{"a"}},
		{GroupValue: "b", SortValues: []any{"b"}},
		{GroupValue: "c", SortValues: []any{"c"}},
		{GroupValue: "d", SortValues: []any{"d"}},
	}
	merged := MergeSearchGroups([][]*SearchGroup[string]{shard}, 2, 2, compareStrAsc)
	if len(merged) != 2 {
		t.Fatalf("expected 2 merged groups, got %d", len(merged))
	}
	if merged[0].GroupValue != "c" || merged[1].GroupValue != "d" {
		t.Errorf("unexpected groups after offset=2: %v", merged)
	}
}

// TestBaseGroupSelectorTestCase_GroupSelectorEmptyExclusion reproduces the
// testIgnoreDocsWithoutGroupField assertion: when a GroupSelector is configured
// to exclude the empty group, documents without a value must not be selected.
//
// This test uses DoubleRangeGroupSelector as a representative selector.
func TestBaseGroupSelectorTestCase_GroupSelectorEmptyExclusion(t *testing.T) {
	factory := buildDoubleFactory()

	// Docs 0-2 have values; doc 3 has no value.
	values := map[int]float64{0: 5.0, 1: 15.0, 2: 25.0}
	fn := func(doc int) (float64, bool) {
		v, ok := values[doc]
		return v, ok
	}
	s := NewDoubleRangeGroupSelector(factory, fn)

	// Default (first-pass): all docs accepted because inSecondPass == nil.
	for _, doc := range []int{0, 1, 2, 3} {
		if !s.AdvanceTo(doc) {
			t.Errorf("first pass: doc %d unexpectedly rejected", doc)
		}
	}

	// Second pass: only the three value groups (no nil group).
	// Doc 3 (no value) must be rejected.
	s.SetGroups([]*SearchGroup[*DoubleRange]{
		{GroupValue: NewDoubleRange("0-10", 0, 10)},
		{GroupValue: NewDoubleRange("10-20", 10, 20)},
		{GroupValue: NewDoubleRange("20-30", 20, 30)},
	})
	for _, doc := range []int{0, 1, 2} {
		if !s.AdvanceTo(doc) {
			t.Errorf("second pass: doc %d with value unexpectedly rejected", doc)
		}
	}
	if s.AdvanceTo(3) {
		t.Error("second pass: doc 3 (no value) must be rejected when empty group not in second pass")
	}
}

// TestBaseGroupSelectorTestCase_GroupSelectorEmptyInclusion verifies that when
// the nil group is in the second pass, docs without values are included —
// the counterpart to testIgnoreDocsWithoutGroupField's default-behavior branch.
func TestBaseGroupSelectorTestCase_GroupSelectorEmptyInclusion(t *testing.T) {
	factory := buildDoubleFactory()
	fn := func(doc int) (float64, bool) { return 0, false } // no doc has a value
	s := NewDoubleRangeGroupSelector(factory, fn)

	// Second pass includes the empty group.
	s.SetGroups([]*SearchGroup[*DoubleRange]{
		{GroupValue: nil},
	})
	for _, doc := range []int{0, 1, 2} {
		if !s.AdvanceTo(doc) {
			t.Errorf("doc %d should be accepted (empty group in second pass)", doc)
		}
	}
}

// TestBaseGroupSelectorTestCase_LongSelectorGroupBuckets exercises
// LongRangeGroupSelector to verify that multiple documents with the same
// long value are assigned to the same range bucket, and that documents
// in different buckets produce distinct group keys — the core correctness
// property exercised by BaseGroupSelectorTestCase's tests.
func TestBaseGroupSelectorTestCase_LongSelectorGroupBuckets(t *testing.T) {
	factory := buildLongFactory()
	values := map[int]int64{0: 5, 1: 5, 2: 15, 3: 25}
	fn := func(doc int) (int64, bool) {
		v, ok := values[doc]
		return v, ok
	}
	s := NewLongRangeGroupSelector(factory, fn)

	// Docs 0 and 1 share value 5 → same [0,10) bucket.
	s.AdvanceTo(0)
	g0 := s.CurrentValue()
	s.AdvanceTo(1)
	g1 := s.CurrentValue()
	if g0.Min != g1.Min || g0.Max != g1.Max {
		t.Errorf("docs 0,1 (value=5) must map to the same bucket: [%d,%d) vs [%d,%d)",
			g0.Min, g0.Max, g1.Min, g1.Max)
	}

	// Docs 0 and 2 have different buckets.
	s.AdvanceTo(2)
	g2 := s.CurrentValue()
	if g0.Min == g2.Min && g0.Max == g2.Max {
		t.Errorf("docs 0 and 2 must map to different buckets")
	}

	// Three distinct non-nil group values across docs 0,2,3.
	s.AdvanceTo(3)
	g3 := s.CurrentValue()
	if g2.Min == g3.Min && g2.Max == g3.Max {
		t.Errorf("docs 2 and 3 must map to different buckets")
	}
}
