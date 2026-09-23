// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	testindex "github.com/FlavioCFOliveira/Gocene/tests/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// This file is the port of
// lucene/grouping/src/test/org/apache/lucene/search/grouping/BaseGroupSelectorTestCase.java
// (Apache Lucene 10.5.0).
//
// The abstract generic class is rendered as baseGroupSelectorTestCase[T],
// whose three abstract methods are the function fields supplied by each
// concrete subclass (TestTermGroupSelector, TestDoubleRangeGroupSelector,
// TestLongRangeGroupSelector, TestValueSourceGroupSelector). The inherited
// test methods are the methods of this type; every subclass file declares
// one Go test per inherited method, as JUnit runs them per subclass.
type baseGroupSelectorTestCase[T any] struct {
	// addGroupField renders the abstract addGroupField(Document, int).
	addGroupField func(t testing.TB, document *document.Document, id int)
	// getGroupSelector renders the abstract getGroupSelector().
	getGroupSelector func() GroupSelector[T]
	// filterQuery renders the abstract filterQuery(T).
	filterQuery func(t testing.TB, groupValue T) search.Query
}

var baseGroupSelectorQueryTerms = []string{"foo", "bar", "baz"}

func (c *baseGroupSelectorTestCase[T]) randomTopLevelQuery() search.Query {
	return search.NewTermQuery(index.NewTerm("text", baseGroupSelectorQueryTerms[random().Intn(len(baseGroupSelectorQueryTerms))]))
}

// filtered renders new BooleanQuery.Builder().add(topLevel, MUST)
// .add(filterQuery(groupValue), FILTER).build().
func (c *baseGroupSelectorTestCase[T]) filtered(t testing.TB, topLevel search.Query, groupValue T) search.Query {
	t.Helper()
	return search.NewBooleanQueryBuilder().
		Add(topLevel, search.MUST).
		Add(c.filterQuery(t, groupValue), search.FILTER).
		Build()
}

// groupSort renders new Sort(new SortField("sort1", STRING), new
// SortField("sort2", LONG)).
func groupSelectorTestSort() *search.Sort {
	return search.NewSort(
		search.NewSortField("sort1", spi.SortFieldTypeString),
		search.NewSortField("sort2", spi.SortFieldTypeLong))
}

func mustGroupingSearch[T any](t testing.TB, gs *GroupingSearch, searcher *search.IndexSearcher,
	query search.Query, groupOffset, groupLimit int) *TopGroups[T] {
	t.Helper()
	tg, err := GroupingSearchSearch[T](gs, searcher, query, groupOffset, groupLimit)
	if err != nil {
		t.Fatalf("GroupingSearch.search: %v", err)
	}
	return tg
}

func mustSearchSort(t testing.TB, s *search.IndexSearcher, q search.Query, n int, sort *search.Sort) *search.TopFieldDocs {
	t.Helper()
	td, err := s.SearchWithSortNoScores(q, n, sort)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	return td
}

// javaEquals renders Objects.equals for the group and sort values the
// grouping tests compare (BytesRef, DoubleRange, LongRange, MutableValue,
// boxed numbers): the equivalence classes of java_util.go.
func javaEquals(a, b any) bool {
	return groupKey(a) == groupKey(b)
}

func (c *baseGroupSelectorTestCase[T]) testSortByRelevance(t *testing.T) {
	shard := newShard(t)
	c.indexRandomDocs(t, shard.writer)

	topLevel := c.randomTopLevelQuery()

	searcher := shard.getIndexSearcher(t)
	grouper := NewGroupingSearchGroupSelector(c.getGroupSelector())
	grouper.SetGroupDocsLimit(10)
	topGroups := mustGroupingSearch[T](t, grouper, searcher, topLevel, 0, 5)
	topDoc := mustSearch(t, searcher, topLevel, 1)
	for i, group := range topGroups.Groups {
		// Each group should have a result set equal to that returned by the top-level query,
		// filtered by the group value.
		td := mustSearch(t, searcher, c.filtered(t, topLevel, group.GroupValue), 10)
		assertScoreDocsEquals(t, group.ScoreDocs, td.ScoreDocs)
		if i == 0 {
			if td.ScoreDocs[0].Doc != topDoc.ScoreDocs[0].Doc {
				t.Fatalf("top doc: expected %d, got %d", td.ScoreDocs[0].Doc, topDoc.ScoreDocs[0].Doc)
			}
			if td.ScoreDocs[0].Score != topDoc.ScoreDocs[0].Score {
				t.Fatalf("top score: expected %v, got %v", td.ScoreDocs[0].Score, topDoc.ScoreDocs[0].Score)
			}
		}
	}

	shard.close(t)
}

func (c *baseGroupSelectorTestCase[T]) testSortGroups(t *testing.T) {
	shard := newShard(t)
	c.indexRandomDocs(t, shard.writer)
	searcher := shard.getIndexSearcher(t)

	topLevel := c.randomTopLevelQuery()

	grouper := NewGroupingSearchGroupSelector(c.getGroupSelector())
	grouper.SetGroupDocsLimit(10)
	sort := groupSelectorTestSort()
	grouper.SetGroupSort(sort)
	topGroups := mustGroupingSearch[T](t, grouper, searcher, topLevel, 0, 5)
	topDoc := mustSearchSort(t, searcher, topLevel, 1, sort)
	for i, group := range topGroups.Groups {
		// We're sorting the groups by a defined Sort, but each group itself should be ordered
		// by doc relevance, and should be equal to the results of a top-level query filtered
		// by the group value
		td := mustSearch(t, searcher, c.filtered(t, topLevel, group.GroupValue), 10)
		assertScoreDocsEquals(t, group.ScoreDocs, td.ScoreDocs)
		// The top group should have sort values equal to the sort values of the top doc of
		// a top-level search sorted by the same Sort; subsequent groups should have sort values
		// that compare lower than their predecessor.
		if i > 0 {
			assertSortsBefore(t, topGroups.Groups[i-1], group)
		} else {
			fields := topDoc.FieldDocs[0].Fields
			if len(fields) != len(topGroups.Groups[0].GroupSortValues) {
				t.Fatalf("groupSortValues length: expected %d, got %d", len(fields), len(topGroups.Groups[0].GroupSortValues))
			}
			for k := range fields {
				if !javaEquals(fields[k], topGroups.Groups[0].GroupSortValues[k]) {
					t.Fatalf("groupSortValues[%d]: expected %v, got %v", k, fields[k], topGroups.Groups[0].GroupSortValues[k])
				}
			}
		}
	}

	shard.close(t)
}

func (c *baseGroupSelectorTestCase[T]) testSortWithinGroups(t *testing.T) {
	shard := newShard(t)
	c.indexRandomDocs(t, shard.writer)
	searcher := shard.getIndexSearcher(t)

	topLevel := c.randomTopLevelQuery()

	grouper := NewGroupingSearchGroupSelector(c.getGroupSelector())
	grouper.SetGroupDocsLimit(10)
	sort := groupSelectorTestSort()
	grouper.SetSortWithinGroup(sort)

	topGroups := mustGroupingSearch[T](t, grouper, searcher, topLevel, 0, 5)
	topDoc := mustSearch(t, searcher, topLevel, 1)

	for i, group := range topGroups.Groups {
		// Check top-level ordering by score: first group's maxScore should be equal to the
		// top score returned by a simple search with no grouping; subsequent groups should
		// all have equal or lower maxScores
		if i == 0 {
			if topDoc.ScoreDocs[0].Score != topGroups.Groups[0].MaxScore {
				t.Fatalf("maxScore: expected %v, got %v", topDoc.ScoreDocs[0].Score, topGroups.Groups[0].MaxScore)
			}
		} else if !(group.MaxScore <= topGroups.Groups[i-1].MaxScore) {
			t.Fatalf("group %d maxScore %v > previous %v", i, group.MaxScore, topGroups.Groups[i-1].MaxScore)
		}
		// Groups themselves are ordered by a defined Sort, and each should give the same result as
		// the top-level query, filtered by the group value, with the same Sort
		td := mustSearchSort(t, searcher, c.filtered(t, topLevel, group.GroupValue), 10, sort)
		assertScoreDocsEquals(t, td.ScoreDocs, group.ScoreDocs)
	}

	shard.close(t)
}

func (c *baseGroupSelectorTestCase[T]) testGroupHeads(t *testing.T) {
	shard := newShard(t)
	c.indexRandomDocs(t, shard.writer)
	searcher := shard.getIndexSearcher(t)

	topLevel := c.randomTopLevelQuery()

	groupSelector := c.getGroupSelector()
	grouping := NewGroupingSearchGroupSelector(groupSelector)
	grouping.SetAllGroups(true)
	grouping.SetAllGroupHeads(true)

	mustGroupingSearch[T](t, grouping, searcher, topLevel, 0, 1)
	matchingGroups := GroupingSearchGetAllMatchingGroups[T](grouping)

	// The number of hits from the top-level query should equal the sum of
	// the number of hits from the query filtered by each group value in turn
	totalHits := mustCount(t, searcher, topLevel)
	groupHits := 0
	for _, groupValue := range matchingGroups {
		groupHits += mustCount(t, searcher, c.filtered(t, topLevel, groupValue))
	}
	if totalHits != groupHits {
		t.Fatalf("totalHits %d != groupHits %d", totalHits, groupHits)
	}

	groupHeads := grouping.GetAllGroupHeads()
	cardinality := 0
	for i := 0; i < groupHeads.Length(); i++ {
		if groupHeads.Get(i) {
			cardinality++
		}
	}
	// We should have one set bit per matching group
	if len(matchingGroups) != cardinality {
		t.Fatalf("matching groups %d != group heads %d", len(matchingGroups), cardinality)
	}

	// Each group head should correspond to the topdoc of a search filtered by
	// that group
	for _, groupValue := range matchingGroups {
		td := mustSearch(t, searcher, c.filtered(t, topLevel, groupValue), 1)
		if !groupHeads.Get(td.ScoreDocs[0].Doc) {
			t.Fatalf("doc %d is not a group head", td.ScoreDocs[0].Doc)
		}
	}

	shard.close(t)
}

func (c *baseGroupSelectorTestCase[T]) testGroupHeadsWithSort(t *testing.T) {
	shard := newShard(t)
	c.indexRandomDocs(t, shard.writer)
	searcher := shard.getIndexSearcher(t)

	topLevel := c.randomTopLevelQuery()

	sort := groupSelectorTestSort()
	groupSelector := c.getGroupSelector()
	grouping := NewGroupingSearchGroupSelector(groupSelector)
	grouping.SetAllGroups(true)
	grouping.SetAllGroupHeads(true)
	grouping.SetSortWithinGroup(sort)

	mustGroupingSearch[T](t, grouping, searcher, topLevel, 0, 1)
	matchingGroups := GroupingSearchGetAllMatchingGroups[T](grouping)

	groupHeads := grouping.GetAllGroupHeads()
	cardinality := 0
	for i := 0; i < groupHeads.Length(); i++ {
		if groupHeads.Get(i) {
			cardinality++
		}
	}
	// We should have one set bit per matching group
	if len(matchingGroups) != cardinality {
		t.Fatalf("matching groups %d != group heads %d", len(matchingGroups), cardinality)
	}

	// Each group head should correspond to the topdoc of a search filtered by
	// that group using the same within-group sort
	for _, groupValue := range matchingGroups {
		td := mustSearchSort(t, searcher, c.filtered(t, topLevel, groupValue), 1, sort)
		if !groupHeads.Get(td.ScoreDocs[0].Doc) {
			t.Fatalf("doc %d is not a group head", td.ScoreDocs[0].Doc)
		}
	}

	shard.close(t)
}

func (c *baseGroupSelectorTestCase[T]) testShardedGrouping(t *testing.T) {
	control := newShard(t)

	shardCount := random().Intn(3) + 2 // between 2 and 4 shards
	shards := make([]*shard, shardCount)
	for i := range shards {
		shards[i] = newShard(t)
	}

	texts := []string{"foo", "bar", "bar baz", "foo foo bar"}

	// Create a bunch of random documents, and index them - once into the control index,
	// and once into a randomly picked shard.

	numDocs := atLeast(200)
	for i := 0; i < numDocs; i++ {
		doc := c.randomDoc(t, i, texts)
		mustAddDocument(t, control.writer, doc)
		s := random().Intn(shardCount)
		mustAddDocument(t, shards[s].writer, doc)
	}

	topLevel := c.randomTopLevelQuery()

	sort := groupSelectorTestSort()

	// A grouped query run in two phases against the control should give us the same
	// result as the query run against shards and merged back together after each phase.

	firstPassGroupingCollectorManager, err := NewFirstPassGroupingCollectorManager(c.getGroupSelector, sort, 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	singletonGroups, err := search.SearchWithCollectorManager[*FirstPassGroupingCollector[T], []*SearchGroup[T]](
		control.getIndexSearcher(t), topLevel, firstPassGroupingCollectorManager)
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	shardGroups := make([][]*SearchGroup[T], 0, len(shards))
	for _, s := range shards {
		fcm, err := NewFirstPassGroupingCollectorManager(c.getGroupSelector, sort, 0, 5)
		if err != nil {
			t.Fatal(err)
		}
		topGroups, err := search.SearchWithCollectorManager[*FirstPassGroupingCollector[T], []*SearchGroup[T]](
			s.getIndexSearcher(t), topLevel, fcm)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		shardGroups = append(shardGroups, topGroups)
	}
	mergedGroups := MergeSearchGroups(shardGroups, 0, 5, sort)
	assertSearchGroupsEqual(t, singletonGroups, mergedGroups)

	withinGroupOffset := random().Intn(numDocs)
	topGroupsCollectorManager := NewTopGroupsCollectorManager(
		c.getGroupSelector, singletonGroups, sort, search.RELEVANCE, withinGroupOffset, 5, true)
	singletonTopGroups, err := search.SearchWithCollectorManager[*TopGroupsCollector[T], *TopGroups[T]](
		control.getIndexSearcher(t), topLevel, topGroupsCollectorManager)
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	// TODO why does SearchGroup.merge() take a list but TopGroups.merge() take an array?
	shardTopGroups := make([]*TopGroups[T], len(shards))
	for j, s := range shards {
		scm := NewTopGroupsCollectorManager(
			c.getGroupSelector, mergedGroups, sort, search.RELEVANCE, 0, withinGroupOffset+5, true)
		shardTopGroups[j], err = search.SearchWithCollectorManager[*TopGroupsCollector[T], *TopGroups[T]](
			s.getIndexSearcher(t), topLevel, scm)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
	}
	mergedTopGroups, err := MergeTopGroups(shardTopGroups, sort, search.RELEVANCE, withinGroupOffset, 5, ScoreMergeModeNone)
	if err != nil {
		t.Fatalf("TopGroups.merge: %v", err)
	}
	if mergedTopGroups == nil {
		t.Fatal("mergedTopGroups is null")
	}

	if singletonTopGroups.TotalGroupedHitCount != mergedTopGroups.TotalGroupedHitCount {
		t.Fatalf("totalGroupedHitCount: %d != %d", singletonTopGroups.TotalGroupedHitCount, mergedTopGroups.TotalGroupedHitCount)
	}
	if singletonTopGroups.TotalHitCount != mergedTopGroups.TotalHitCount {
		t.Fatalf("totalHitCount: %d != %d", singletonTopGroups.TotalHitCount, mergedTopGroups.TotalHitCount)
	}
	if !equalIntPtr(singletonTopGroups.TotalGroupCount, mergedTopGroups.TotalGroupCount) {
		t.Fatalf("totalGroupCount: %v != %v", singletonTopGroups.TotalGroupCount, mergedTopGroups.TotalGroupCount)
	}
	if len(singletonTopGroups.Groups) != len(mergedTopGroups.Groups) {
		t.Fatalf("groups: %d != %d", len(singletonTopGroups.Groups), len(mergedTopGroups.Groups))
	}
	for i := range singletonTopGroups.Groups {
		if !javaEquals(singletonTopGroups.Groups[i].GroupValue, mergedTopGroups.Groups[i].GroupValue) {
			t.Fatalf("groups[%d].groupValue: %v != %v", i, singletonTopGroups.Groups[i].GroupValue, mergedTopGroups.Groups[i].GroupValue)
		}
		if len(singletonTopGroups.Groups[i].ScoreDocs) != len(mergedTopGroups.Groups[i].ScoreDocs) {
			t.Fatalf("groups[%d].scoreDocs: %d != %d", i, len(singletonTopGroups.Groups[i].ScoreDocs), len(mergedTopGroups.Groups[i].ScoreDocs))
		}
	}

	control.close(t)
	for _, s := range shards {
		s.close(t)
	}
}

// equalIntPtr renders Objects.equals over two nullable Integers.
func equalIntPtr(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// assertSearchGroupsEqual renders assertEquals(Collection<SearchGroup<T>>,
// Collection<SearchGroup<T>>) over two lists: equal sizes and pairwise
// SearchGroup.equals.
func assertSearchGroupsEqual[T any](t testing.TB, expected, actual []*SearchGroup[T]) {
	t.Helper()
	if len(expected) != len(actual) {
		t.Fatalf("search groups: expected %v, got %v", expected, actual)
	}
	for i := range expected {
		if !expected[i].Equals(actual[i]) {
			t.Fatalf("search groups: expected %v, got %v", expected, actual)
		}
	}
}

// randomDoc renders the document body shared by testShardedGrouping and
// indexRandomDocs.
func (c *baseGroupSelectorTestCase[T]) randomDoc(t testing.TB, i int, texts []string) *document.Document {
	t.Helper()
	doc := document.NewDocument()
	doc.Add(mustNumericDVField(t, "id", int64(i)))
	doc.Add(newTextField(t, "name", strconv.Itoa(i), true))
	doc.Add(newTextField(t, "text", texts[random().Intn(len(texts))], false))
	doc.Add(mustSortedDVField(t, "sort1", "sort"+strconv.Itoa(random().Intn(4))))
	doc.Add(mustNumericDVField(t, "sort2", randomNextLong(random())))
	c.addGroupField(t, doc, i)
	return doc
}

func (c *baseGroupSelectorTestCase[T]) indexRandomDocs(t testing.TB, w *testindex.RandomIndexWriter) {
	t.Helper()
	texts := []string{"foo", "bar", "bar baz", "foo foo bar"}

	numDocs := atLeast(200)
	for i := 0; i < numDocs; i++ {
		mustAddDocument(t, w, c.randomDoc(t, i, texts))
	}
}

func (c *baseGroupSelectorTestCase[T]) testIgnoreDocsWithoutGroupField(t *testing.T) {
	shard := newShard(t)

	// Add documents with group field
	doc := document.NewDocument()
	doc.Add(newTextField(t, "text", "foo", false))
	c.addGroupField(t, doc, 1)
	mustAddDocument(t, shard.writer, doc)

	doc = document.NewDocument()
	doc.Add(newTextField(t, "text", "foo", false))
	c.addGroupField(t, doc, 2)
	mustAddDocument(t, shard.writer, doc)

	// Add document without group field
	doc = document.NewDocument()
	doc.Add(newTextField(t, "text", "foo", false))
	mustAddDocument(t, shard.writer, doc)

	searcher := shard.getIndexSearcher(t)
	query := search.NewTermQuery(index.NewTerm("text", "foo"))

	// Test default behavior (include null group)
	grouping1 := NewGroupingSearchGroupSelector(c.getGroupSelector())
	groups1 := mustGroupingSearch[T](t, grouping1, searcher, query, 0, 10)
	defaultGroupCount := len(groups1.Groups)

	// Test ignoring docs without group field
	grouping2 := NewGroupingSearchGroupSelector(c.getGroupSelector())
	grouping2.SetIgnoreDocsWithoutGroupField(true)
	groups2 := mustGroupingSearch[T](t, grouping2, searcher, query, 0, 10)
	ignoreGroupCount := len(groups2.Groups)

	if !(ignoreGroupCount <= defaultGroupCount) {
		t.Fatalf("Expected ignoreGroupCount <= defaultGroupCount, got %d vs %d", ignoreGroupCount, defaultGroupCount)
	}

	shard.close(t)
}

func assertSortsBefore[T any](t testing.TB, first, second *GroupDocs[T]) {
	t.Helper()
	groupSortValues := second.GroupSortValues
	prevSortValues := first.GroupSortValues
	if !(prevSortValues[0].(*util.BytesRef).BytesRefCompareTo(groupSortValues[0].(*util.BytesRef)) <= 0) {
		t.Fatalf("sort values out of order: %v then %v", prevSortValues, groupSortValues)
	}
	if javaEquals(prevSortValues[0], groupSortValues[0]) {
		if !(prevSortValues[1].(int64) <= groupSortValues[1].(int64)) {
			t.Fatalf("sort values out of order: %v then %v", prevSortValues, groupSortValues)
		}
	}
}

// --- field helpers (Java `new XxxField(...)` constructors) --------------------

func mustSortedDVField(t testing.TB, name, value string) *document.SortedDocValuesField {
	t.Helper()
	f, err := document.NewSortedDocValuesField(name, []byte(value))
	if err != nil {
		t.Fatalf("new SortedDocValuesField: %v", err)
	}
	return f
}

func mustNumericDVField(t testing.TB, name string, value int64) *document.NumericDocValuesField {
	t.Helper()
	f, err := document.NewNumericDocValuesField(name, value)
	if err != nil {
		t.Fatalf("new NumericDocValuesField: %v", err)
	}
	return f
}

// pointNewRangeQueryBlocker names the static range query factories of the
// point fields: DoublePoint.newRangeQuery and LongPoint.newRangeQuery return
// org.apache.lucene.search.PointRangeQuery, and Gocene's document package
// cannot import search (search imports document), so neither is ported.
func pointNewRangeQueryBlocker(class string) string {
	return fmt.Sprintf("requires org.apache.lucene.document.%s.newRangeQuery(String, ...) (not ported)", class)
}

// randomNextLong renders Random.nextLong() over the full long range.
func randomNextLong(r *rand.Rand) int64 {
	return int64(r.Uint64())
}
