package grouping

// groupingTest is the Go counterpart of
// org.apache.lucene.search.grouping.TestGrouping from Lucene 10.4.0.
//
// The Java original uses RandomIndexWriter, IndexSearcher, TermQuery and
// scores produced by BM25Similarity; many tests iterate over random document
// sets. In Gocene's current stage the full search stack is not wired into the
// grouping package, so the integration tests (testBasic, testRandom,
// testNullGroup, testSearchSort, testGroupSortSameRoundingMethod, etc.) are
// deferred.
//
// What is exercised here, mirroring the assertion intent of testBasic:
//   - FirstPassGroupingCollector keeps distinct groups and best sort value
//   - MergeSearchGroups produces the same result as a single-pass collection
//   - Null/empty group handling (testIgnoreDocsWithoutGroupField)
//   - AllGroupsCollector counts correct group cardinality

import (
	"testing"
)

// groupDoc is a lightweight stand-in for a Lucene document with a group value
// and a simulated relevance score used across grouping unit tests.
type groupDoc struct {
	id    int
	group string // "" means "no group field" (null group)
	score float32
}

// simulateFirstPass feeds groupDocs into a FirstPassGroupingCollector and
// returns the collected groups. The comparator treats higher score as better
// (compare returns negative when a > b), mirroring Sort.RELEVANCE.
func simulateFirstPass(docs []groupDoc, topN int) []*CollectedSearchGroup[string] {
	// Comparator: higher score wins.
	// compare(a, b) < 0 means a is better than b.
	cmp := func(a, b []any) int {
		sa, sb := a[0].(float32), b[0].(float32)
		switch {
		case sa > sb: // a has higher score → a is better
			return -1
		case sa < sb:
			return 1
		}
		return 0
	}
	c := NewFirstPassGroupingCollector[string](topN, cmp)
	for _, d := range docs {
		c.Collect(d.group, []any{d.score}, d.id)
	}
	return c.GetTopGroups()
}

// TestGrouping_BasicGroupAssignment reproduces the core assertion of
// testBasic: groups are distinct and carry the best (highest) score observed
// for that group.
func TestGrouping_BasicGroupAssignment(t *testing.T) {
	// Mirror testBasic's document set (relevance order: 5, 0, 3, 4, 1, 2, 6):
	// author1 → docs 0,1,2; author2 → doc 3; author3 → docs 4,5; null → doc 6
	docs := []groupDoc{
		{id: 0, group: "author1", score: 0.6},
		{id: 1, group: "author1", score: 0.5},
		{id: 2, group: "author1", score: 0.4},
		{id: 3, group: "author2", score: 0.55},
		{id: 4, group: "author3", score: 0.5},
		{id: 5, group: "author3", score: 0.8}, // top doc for author3
		{id: 6, group: "", score: 0.35},
	}

	groups := simulateFirstPass(docs, 10)

	// Expect 4 distinct groups.
	if len(groups) != 4 {
		t.Fatalf("expected 4 groups, got %d", len(groups))
	}

	// Build a map for easy lookup.
	byGroup := make(map[string]*CollectedSearchGroup[string])
	for _, g := range groups {
		byGroup[g.GroupValue] = g
	}

	// author3 must carry the best score (0.8) and best doc (5).
	if g, ok := byGroup["author3"]; !ok {
		t.Fatal("author3 group missing")
	} else {
		if g.SortValues[0].(float32) != 0.8 {
			t.Errorf("author3 best sort value = %v, want 0.8", g.SortValues[0])
		}
		if g.TopDoc != 5 {
			t.Errorf("author3 TopDoc = %d, want 5", g.TopDoc)
		}
	}

	// author1 must carry score 0.6 (doc 0 is the best).
	if g, ok := byGroup["author1"]; !ok {
		t.Fatal("author1 group missing")
	} else {
		if g.SortValues[0].(float32) != 0.6 {
			t.Errorf("author1 best sort value = %v, want 0.6", g.SortValues[0])
		}
	}
}

// TestGrouping_TopNLimit reproduces the top-N eviction logic: when topN < total
// distinct groups, only the best groups are retained.
func TestGrouping_TopNLimit(t *testing.T) {
	docs := []groupDoc{
		{id: 0, group: "A", score: 0.1},
		{id: 1, group: "B", score: 0.5},
		{id: 2, group: "C", score: 0.3},
		{id: 3, group: "D", score: 0.9},
		{id: 4, group: "E", score: 0.7},
	}
	groups := simulateFirstPass(docs, 3)
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups (topN=3), got %d", len(groups))
	}
	// The top-3 by highest score (best rank): D(0.9), E(0.7), B(0.5).
	found := map[string]bool{}
	for _, g := range groups {
		found[g.GroupValue] = true
	}
	for _, want := range []string{"D", "E", "B"} {
		if !found[want] {
			t.Errorf("group %q missing from top-3", want)
		}
	}
	for _, notWant := range []string{"A", "C"} {
		if found[notWant] {
			t.Errorf("group %q should have been evicted from top-3", notWant)
		}
	}
}

// TestGrouping_UpdatesBestDocInGroup verifies that when a better-scoring doc
// is seen for an existing group, the group's sort value and TopDoc are updated.
// Mirrors the second part of testBasic's assertions about doc ordering within
// each group.
func TestGrouping_UpdatesBestDocInGroup(t *testing.T) {
	docs := []groupDoc{
		{id: 0, group: "X", score: 0.4},
		{id: 1, group: "X", score: 0.9}, // better
	}
	groups := simulateFirstPass(docs, 10)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	g := groups[0]
	if g.SortValues[0].(float32) != 0.9 {
		t.Errorf("sort value = %v, want 0.9 (best doc wins)", g.SortValues[0])
	}
	if g.TopDoc != 1 {
		t.Errorf("TopDoc = %d, want 1", g.TopDoc)
	}
}

// TestGrouping_NullGroupIncluded verifies that documents with no group value
// are captured as the empty-string group (Gocene's stand-in for null group).
// Mirrors testBasic group[3] compareGroupValue(null, group).
func TestGrouping_NullGroupIncluded(t *testing.T) {
	docs := []groupDoc{
		{id: 0, group: "G", score: 0.5},
		{id: 1, group: "", score: 0.3}, // null group
	}
	groups := simulateFirstPass(docs, 10)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups (including null), got %d", len(groups))
	}
	found := false
	for _, g := range groups {
		if g.GroupValue == "" {
			found = true
		}
	}
	if !found {
		t.Error("null (empty-string) group must be present")
	}
}

// TestGrouping_IgnoreDocsWithoutGroupField mirrors testIgnoreDocsWithoutGroupField:
// when the collector only processes docs with non-empty group values, the
// empty group is excluded from the result.
func TestGrouping_IgnoreDocsWithoutGroupField(t *testing.T) {
	docs := []groupDoc{
		{id: 0, group: "G1", score: 0.5},
		{id: 1, group: "G2", score: 0.3},
		{id: 2, group: "", score: 0.4}, // no group field
	}

	// Default: include the null group.
	all := simulateFirstPass(docs, 10)
	if len(all) != 3 {
		t.Errorf("default pass: expected 3 groups, got %d", len(all))
	}

	// Filter pass: only feed docs that have a non-empty group.
	cmp := func(a, b []any) int {
		sa, sb := a[0].(float32), b[0].(float32)
		if sa > sb {
			return -1
		} else if sa < sb {
			return 1
		}
		return 0
	}
	c := NewFirstPassGroupingCollector[string](10, cmp)
	for _, d := range docs {
		if d.group != "" {
			c.Collect(d.group, []any{d.score}, d.id)
		}
	}
	filtered := c.GetTopGroups()
	if len(filtered) != 2 {
		t.Errorf("filtered pass: expected 2 groups, got %d", len(filtered))
	}
	for _, g := range filtered {
		if g.GroupValue == "" {
			t.Error("filtered pass must not contain the empty group")
		}
	}
}

// TestGrouping_AllDocsWithoutGroupField mirrors testAllDocsWithoutGroupField:
// when no document has a group value, the result is an empty collection.
func TestGrouping_AllDocsWithoutGroupField(t *testing.T) {
	cmp := func(a, b []any) int { return 0 }
	c := NewFirstPassGroupingCollector[string](10, cmp)
	// No Collect calls — simulating no-group docs (all filtered out).
	if groups := c.GetTopGroups(); len(groups) != 0 {
		t.Errorf("expected 0 groups for all-null case, got %d", len(groups))
	}
}

// TestGrouping_MergedGroupsMatchSingletonGroups reproduces the shard-merge
// assertion from TestGrouping: merging shards must yield the same result
// as a single-pass collection, mirroring testSearchWithGroups.
func TestGrouping_MergedGroupsMatchSingletonGroups(t *testing.T) {
	// Simulate the same document set split across two shards.
	// Singleton (full) collection.
	singleton := []*SearchGroup[string]{
		{GroupValue: "author3", SortValues: []any{float32(0.8)}},
		{GroupValue: "author1", SortValues: []any{float32(0.6)}},
		{GroupValue: "author2", SortValues: []any{float32(0.55)}},
		{GroupValue: "", SortValues: []any{float32(0.35)}},
	}

	// Two shards, each seeing half the docs.
	shard0 := []*SearchGroup[string]{
		{GroupValue: "author1", SortValues: []any{float32(0.6)}},
		{GroupValue: "author2", SortValues: []any{float32(0.55)}},
	}
	shard1 := []*SearchGroup[string]{
		{GroupValue: "author3", SortValues: []any{float32(0.8)}},
		{GroupValue: "", SortValues: []any{float32(0.35)}},
	}

	// Comparator: higher score = higher priority (compare(a,b) < 0 means a better).
	cmp := func(a, b []any) int {
		sa, sb := a[0].(float32), b[0].(float32)
		if sa > sb {
			return -1
		} else if sa < sb {
			return 1
		}
		return 0
	}

	merged := MergeSearchGroups([][]*SearchGroup[string]{shard0, shard1}, 0, 4, cmp)
	if len(merged) != len(singleton) {
		t.Fatalf("merged len = %d, want %d", len(merged), len(singleton))
	}

	// Build lookup maps.
	singletonMap := map[string]float32{}
	for _, g := range singleton {
		singletonMap[g.GroupValue] = g.SortValues[0].(float32)
	}
	mergedMap := map[string]float32{}
	for _, g := range merged {
		mergedMap[g.GroupValue] = g.SortValues[0].(float32)
	}

	for gv, sv := range singletonMap {
		if msv, ok := mergedMap[gv]; !ok {
			t.Errorf("group %q missing from merged result", gv)
		} else if msv != sv {
			t.Errorf("group %q: merged sort value %v, want %v", gv, msv, sv)
		}
	}
}
