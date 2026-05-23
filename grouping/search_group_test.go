package grouping

import (
	"strings"
	"testing"
)

// TestSearchGroupString verifies the toString contract matches the Java original.
func TestSearchGroupString(t *testing.T) {
	g := NewSearchGroup("author1", []any{1.5})
	s := g.String()
	if !strings.Contains(s, "author1") {
		t.Errorf("String() = %q, want groupValue in output", s)
	}
	if !strings.Contains(s, "SearchGroup") {
		t.Errorf("String() = %q, want SearchGroup prefix", s)
	}
}

// TestSearchGroupEqual verifies that equality is based solely on GroupValue,
// mirroring SearchGroup.equals in Lucene 10.4.0.
func TestSearchGroupEqual(t *testing.T) {
	a := NewSearchGroup("x", []any{1})
	b := NewSearchGroup("x", []any{2}) // different sort values, same group key
	c := NewSearchGroup("y", []any{1})

	if !a.Equal(b) {
		t.Error("groups with same GroupValue must be equal regardless of SortValues")
	}
	if a.Equal(c) {
		t.Error("groups with different GroupValue must not be equal")
	}
	if a.Equal(nil) {
		t.Error("Equal(nil) must return false")
	}
}

// TestSearchGroupNewClonesValues ensures the constructor copies SortValues,
// so the caller's slice cannot mutate the SearchGroup's state.
func TestSearchGroupNewClonesValues(t *testing.T) {
	vals := []any{42}
	g := NewSearchGroup("k", vals)
	vals[0] = 99
	if g.SortValues[0].(int) != 42 {
		t.Error("NewSearchGroup must copy SortValues, not hold a reference")
	}
}

// compareStrAsc is a simple comparator over single-element string sort-value slices.
func compareStrAsc(a, b []any) int {
	sa, sb := a[0].(string), b[0].(string)
	switch {
	case sa < sb:
		return -1
	case sa > sb:
		return 1
	}
	return 0
}

// TestMergeSearchGroupsBasic verifies that MergeSearchGroups returns the
// top-N groups in comparator order across two shards, mirroring
// SearchGroup.merge in Lucene 10.4.0.
func TestMergeSearchGroupsBasic(t *testing.T) {
	// Shard 0: groups "a" and "c"
	// Shard 1: groups "b" and "d"
	shard0 := []*SearchGroup[string]{
		{GroupValue: "a", SortValues: []any{"a"}},
		{GroupValue: "c", SortValues: []any{"c"}},
	}
	shard1 := []*SearchGroup[string]{
		{GroupValue: "b", SortValues: []any{"b"}},
		{GroupValue: "d", SortValues: []any{"d"}},
	}

	merged := MergeSearchGroups([][]*SearchGroup[string]{shard0, shard1}, 0, 3, compareStrAsc)
	if len(merged) != 3 {
		t.Fatalf("expected 3 merged groups, got %d: %v", len(merged), merged)
	}
	want := []string{"a", "b", "c"}
	for i, g := range merged {
		if g.GroupValue != want[i] {
			t.Errorf("merged[%d].GroupValue = %q, want %q", i, g.GroupValue, want[i])
		}
	}
}

// TestMergeSearchGroupsOffset verifies that the offset parameter skips the
// leading groups.
func TestMergeSearchGroupsOffset(t *testing.T) {
	shard0 := []*SearchGroup[string]{
		{GroupValue: "a", SortValues: []any{"a"}},
		{GroupValue: "b", SortValues: []any{"b"}},
		{GroupValue: "c", SortValues: []any{"c"}},
	}

	merged := MergeSearchGroups([][]*SearchGroup[string]{shard0}, 1, 2, compareStrAsc)
	if len(merged) != 2 {
		t.Fatalf("expected 2 merged groups, got %d", len(merged))
	}
	if merged[0].GroupValue != "b" {
		t.Errorf("expected first result to be \"b\", got %q", merged[0].GroupValue)
	}
}

// TestMergeSearchGroupsDeduplicate verifies that the same group appearing in
// multiple shards is deduplicated and the best sort-value wins.
func TestMergeSearchGroupsDeduplicate(t *testing.T) {
	// "x" appears in both shards with different sort values; "a" (better) wins.
	shard0 := []*SearchGroup[string]{
		{GroupValue: "x", SortValues: []any{"b"}},
	}
	shard1 := []*SearchGroup[string]{
		{GroupValue: "x", SortValues: []any{"a"}},
	}

	merged := MergeSearchGroups([][]*SearchGroup[string]{shard0, shard1}, 0, 2, compareStrAsc)
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged group, got %d", len(merged))
	}
	if merged[0].SortValues[0].(string) != "a" {
		t.Errorf("expected winning sort value \"a\", got %q", merged[0].SortValues[0])
	}
}

// TestMergeSearchGroupsEmpty verifies that nil is returned for empty input.
func TestMergeSearchGroupsEmpty(t *testing.T) {
	result := MergeSearchGroups([][]*SearchGroup[string]{}, 0, 5, compareStrAsc)
	if result != nil {
		t.Errorf("expected nil for empty shards, got %v", result)
	}
}

// TestMergeSearchGroupsAllEmpty verifies that nil is returned when all shards
// are empty.
func TestMergeSearchGroupsAllEmpty(t *testing.T) {
	result := MergeSearchGroups([][]*SearchGroup[string]{{}, {}}, 0, 5, compareStrAsc)
	if result != nil {
		t.Errorf("expected nil for all-empty shards, got %v", result)
	}
}
