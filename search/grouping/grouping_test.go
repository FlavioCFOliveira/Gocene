// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/valuesource"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/mutable"
)

// Port of
// lucene/grouping/src/test/org/apache/lucene/search/grouping/TestGrouping.java
// (Apache Lucene 10.5.0).
//
// TODO
//   - should test relevance sort too
//   - test null
//   - test ties
//   - test compound sort
//
// Java's wildcard collectors and managers (FirstPassGroupingCollector<?>,
// TopGroupsCollectorManager<?>, ...) hold either BytesRef groups
// (TermGroupSelector) or MutableValue groups (ValueSourceGroupSelector); the
// Go rendering holds them as `any` and dispatches on the two instantiations,
// exactly where Java casts. compareGroupValue is the identical private method
// of TestGroupingSearch, declared once for the package in
// grouping_search_test.go.

func TestGroupingBasic(t *testing.T) {
	groupField := "author"

	customType := document.NewFieldType()
	customType.SetStored(true)

	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newLogMergePolicy())
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	addDoc := func(author *string, content, id string) {
		doc := document.NewDocument()
		if author != nil {
			addGroupingGroupField(t, doc, groupField, *author)
		}
		doc.Add(newTextField(t, "content", content, true))
		doc.Add(newField(t, "id", id, customType))
		mustAddDocument(t, w, doc)
	}
	// 0
	addDoc(ptr("author1"), "random text", "1")
	// 1
	addDoc(ptr("author1"), "some more random text", "2")
	// 2
	addDoc(ptr("author1"), "some more random textual data", "3")
	// 3
	addDoc(ptr("author2"), "some random text", "4")
	// 4
	addDoc(ptr("author3"), "some more random text", "5")
	// 5
	addDoc(ptr("author3"), "random", "6")
	// 6 -- no author field
	addDoc(nil, "random word stuck in alot of other text", "6")

	indexSearcher := newSearcher(t, mustGetReader(t, w))
	// This test relies on the fact that longer fields produce lower scores
	indexSearcher.SetSimilarity(search.NewLuceneBM25Similarity())
	mustClose(t, w)

	groupSort := search.RELEVANCE

	// TermGroupSelector or ValueSourceGroupSelector
	isTermGroupSelector := random().Intn(2) == 0
	firstPassGroupingCollectorManager := createFirstPassCollectorManager(t, isTermGroupSelector, groupField, groupSort, 0, 10)
	topGroups := searchFirstPassManager(t, indexSearcher, search.NewTermQuery(index.NewTerm("content", "random")), firstPassGroupingCollectorManager)

	topGroupsCollectorManager := createSecondPassCollectorManager(
		isTermGroupSelector, groupField, toByteRefSearchGroups(topGroups), groupSort, search.RELEVANCE, 0, 5, true)
	query := search.NewTermQuery(index.NewTerm("content", "random"))
	switch m := topGroupsCollectorManager.(type) {
	case *TopGroupsCollectorManager[*util.BytesRef]:
		groups, err := search.SearchWithCollectorManager[*TopGroupsCollector[*util.BytesRef], *TopGroups[*util.BytesRef]](indexSearcher, query, m)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		checkGroupingBasicGroups(t, groups)
	case *TopGroupsCollectorManager[mutable.MutableValue]:
		groups, err := search.SearchWithCollectorManager[*TopGroupsCollector[mutable.MutableValue], *TopGroups[mutable.MutableValue]](indexSearcher, query, m)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		checkGroupingBasicGroups(t, groups)
	}

	mustClose(t, indexSearcher.GetIndexReader(), dir)
}

// checkGroupingBasicGroups renders the assertions testBasic makes on its
// TopGroups<?>.
func checkGroupingBasicGroups[T any](t *testing.T, groups *TopGroups[T]) {
	t.Helper()
	if groups.MaxScore != groups.MaxScore {
		t.Fatal("groups.maxScore is NaN")
	}

	assertIntEquals(t, 7, groups.TotalHitCount)
	assertIntEquals(t, 7, groups.TotalGroupedHitCount)
	assertIntEquals(t, 4, len(groups.Groups))

	// relevance order: 5, 0, 3, 4, 1, 2, 6

	// the later a document is added the higher this docId
	// value
	group := groups.Groups[0]
	compareGroupValue(t, ptr("author3"), group.GroupValue)
	assertGroupDocs(t, group, 5, 4)
	if !(group.ScoreDocs[0].Score > group.ScoreDocs[1].Score) {
		t.Fatal("scoreDocs[0].score <= scoreDocs[1].score")
	}

	group = groups.Groups[1]
	compareGroupValue(t, ptr("author1"), group.GroupValue)
	assertGroupDocs(t, group, 0, 1, 2)
	if !(group.ScoreDocs[0].Score >= group.ScoreDocs[1].Score) {
		t.Fatal("scoreDocs[0].score < scoreDocs[1].score")
	}
	if !(group.ScoreDocs[1].Score >= group.ScoreDocs[2].Score) {
		t.Fatal("scoreDocs[1].score < scoreDocs[2].score")
	}

	group = groups.Groups[2]
	compareGroupValue(t, ptr("author2"), group.GroupValue)
	assertGroupDocs(t, group, 3)

	group = groups.Groups[3]
	compareGroupValue(t, nil, group.GroupValue)
	assertGroupDocs(t, group, 6)
}

func TestGroupingIgnoreDocsWithoutGroupField(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriterWithConfig(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))

	groupField := "group"
	// Add documents with group field
	doc := document.NewDocument()
	addGroupingGroupField(t, doc, groupField, "group1")
	// doc.add(new SortedDocValuesField("group", new BytesRef("group1")));
	doc.Add(newTextField(t, "content", "test", true))
	mustAddDocument(t, w, doc)

	doc = document.NewDocument()
	addGroupingGroupField(t, doc, groupField, "group2")
	doc.Add(newTextField(t, "content", "test", true))
	mustAddDocument(t, w, doc)

	// Add document without group field
	doc = document.NewDocument()
	doc.Add(newTextField(t, "content", "test", true))
	mustAddDocument(t, w, doc)

	reader := mustGetReader(t, w)
	mustClose(t, w)

	searcher := newSearcher(t, reader)

	// Test default behavior (include null group)
	firstPassGroupingCollectorManager1, err := NewFirstPassGroupingCollectorManager(
		func() GroupSelector[*util.BytesRef] { return NewTermGroupSelector(groupField) }, search.RELEVANCE, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	groups1, err := search.SearchWithCollectorManager[*FirstPassGroupingCollector[*util.BytesRef], []*SearchGroup[*util.BytesRef]](
		searcher, search.Instance, firstPassGroupingCollectorManager1)
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	assertIntEquals(t, 3, len(groups1)) // Should include null group

	// Test ignoring docs without group field
	firstPassGroupingCollectorManager2, err := NewFirstPassGroupingCollectorManagerIgnoringDocsWithoutGroupField(
		func() GroupSelector[*util.BytesRef] { return NewTermGroupSelector(groupField) }, search.RELEVANCE, 0, 10, true)
	if err != nil {
		t.Fatal(err)
	}
	groups2, err := search.SearchWithCollectorManager[*FirstPassGroupingCollector[*util.BytesRef], []*SearchGroup[*util.BytesRef]](
		searcher, search.Instance, firstPassGroupingCollectorManager2)
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	assertIntEquals(t, 2, len(groups2)) // Should exclude null group

	mustClose(t, reader, dir)
}

func TestGroupingAllDocsWithoutGroupField(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriterWithConfig(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))

	// Add documents without group field
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		doc.Add(newTextField(t, "content", "test", true))
		mustAddDocument(t, w, doc)
	}

	reader := mustGetReader(t, w)
	mustClose(t, w)

	searcher := newSearcher(t, reader)

	// Test ignoring docs without group field when all docs lack the field
	firstPassGroupingCollectorManager, err := NewFirstPassGroupingCollectorManagerIgnoringDocsWithoutGroupField(
		func() GroupSelector[*util.BytesRef] { return NewTermGroupSelector("group") }, search.RELEVANCE, 0, 10, true)
	if err != nil {
		t.Fatal(err)
	}
	groups, err := search.SearchWithCollectorManager[*FirstPassGroupingCollector[*util.BytesRef], []*SearchGroup[*util.BytesRef]](
		searcher, search.Instance, firstPassGroupingCollectorManager)
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	if groups == nil {
		t.Fatal("groups is null")
	}
	if len(groups) != 0 {
		t.Fatalf("groups: expected empty, got %d", len(groups))
	}

	mustClose(t, reader, dir)
}

func TestGroupingFirstPassGroupingCollectorManagerConstructor(t *testing.T) {
	_, e1 := NewFirstPassGroupingCollectorManager(
		func() GroupSelector[*util.BytesRef] { return NewTermGroupSelector("group") }, search.RELEVANCE, -1, 10)
	if e1 == nil {
		t.Fatal("expected IllegalArgumentException")
	}
	if !strings.Contains(e1.Error(), "groupOffset must be >= 0") {
		t.Fatalf("unexpected message: %v", e1)
	}

	_, e2 := NewFirstPassGroupingCollectorManager(
		func() GroupSelector[*util.BytesRef] { return NewTermGroupSelector("group") }, search.RELEVANCE, 0, 0)
	if e2 == nil {
		t.Fatal("expected IllegalArgumentException")
	}
	if !strings.Contains(e2.Error(), "topNGroups must be >= 1") {
		t.Fatalf("unexpected message: %v", e2)
	}
}

func TestGroupingFirstPassGroupingCollectorManagerReduceEmptyResult(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriterWithConfig(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	doc := document.NewDocument()
	addGroupingGroupField(t, doc, "author", "author1")
	doc.Add(newTextField(t, "content", "random text", false))
	mustAddDocument(t, w, doc)

	reader := mustGetReader(t, w)
	mustClose(t, w)
	searcher := newSearcher(t, reader)

	// query matches nothing — reduce should return empty, not null
	manager := createFirstPassCollectorManager(t, random().Intn(2) == 0, "author", search.RELEVANCE, 0, 10)
	result := searchFirstPassManager(t, searcher, search.NewTermQuery(index.NewTerm("content", "nonexistent")), manager)
	switch r := result.(type) {
	case []*SearchGroup[*util.BytesRef]:
		if r == nil || len(r) != 0 {
			t.Fatalf("expected an empty, non-null result, got %v", r)
		}
	case []*SearchGroup[mutable.MutableValue]:
		if r == nil || len(r) != 0 {
			t.Fatalf("expected an empty, non-null result, got %v", r)
		}
	}

	mustClose(t, reader, dir)
}

func TestGroupingSearchGroupMergeEmptyResult(t *testing.T) {
	// Empty shard list must return an empty collection, not null
	result := MergeSearchGroups([][]*SearchGroup[*util.BytesRef]{}, 0, 5, search.RELEVANCE)
	if result == nil {
		t.Fatal("result is null")
	}
	if len(result) != 0 {
		t.Fatalf("result: expected empty, got %d", len(result))
	}

	// When offset skips past all groups the result must also be an empty collection, not null
	group := &SearchGroup[*util.BytesRef]{}
	group.GroupValue = util.NewBytesRef([]byte("a"))
	group.SortValues = []any{float32(1.0)} // Float value for Sort.RELEVANCE
	shardGroups := [][]*SearchGroup[*util.BytesRef]{{group}}
	result = MergeSearchGroups(shardGroups, 10, 5, search.RELEVANCE) // offset 10 > 1 group
	if result == nil {
		t.Fatal("result is null")
	}
	if len(result) != 0 {
		t.Fatalf("result: expected empty, got %d", len(result))
	}
}

func addGroupingGroupField(t testing.TB, doc *document.Document, groupField, value string) {
	t.Helper()
	doc.Add(mustSortedDVField(t, groupField, value))
}

// createRandomFirstPassCollector renders the private
// createRandomFirstPassCollector(String, Sort, int): a
// FirstPassGroupingCollector<?> over MutableValue or BytesRef groups.
func createRandomFirstPassCollector(t testing.TB, groupField string, groupSort *search.Sort, topDocs int) any {
	t.Helper()
	if random().Intn(2) == 0 {
		vs := valuesource.NewBytesRefFieldSource(groupField)
		m, err := NewFirstPassGroupingCollectorManager(func() GroupSelector[mutable.MutableValue] {
			return NewValueSourceGroupSelector(vs, function.Context{})
		}, groupSort, 0, topDocs)
		if err != nil {
			t.Fatal(err)
		}
		c, err := m.NewCollector()
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	m, err := NewFirstPassGroupingCollectorManager(func() GroupSelector[*util.BytesRef] {
		return NewTermGroupSelector(groupField)
	}, groupSort, 0, topDocs)
	if err != nil {
		t.Fatal(err)
	}
	c, err := m.NewCollector()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// createFirstPassCollectorManager renders the private
// createFirstPassCollectorManager(boolean, String, Sort, int, int).
func createFirstPassCollectorManager(t testing.TB, isTermGroupSelector bool, groupField string, groupSort *search.Sort,
	groupOffset, topNGroups int) any {
	t.Helper()
	if isTermGroupSelector {
		m, err := NewFirstPassGroupingCollectorManager(func() GroupSelector[*util.BytesRef] {
			return NewTermGroupSelector(groupField)
		}, groupSort, groupOffset, topNGroups)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	vs := valuesource.NewBytesRefFieldSource(groupField)
	m, err := NewFirstPassGroupingCollectorManager(func() GroupSelector[mutable.MutableValue] {
		return NewValueSourceGroupSelector(vs, function.Context{})
	}, groupSort, groupOffset, topNGroups)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// searchFirstPassManager renders searcher.search(query,
// FirstPassGroupingCollectorManager<?>): the Collection<?> result.
func searchFirstPassManager(t testing.TB, s *search.IndexSearcher, q search.Query, manager any) any {
	t.Helper()
	switch m := manager.(type) {
	case *FirstPassGroupingCollectorManager[*util.BytesRef]:
		groups, err := search.SearchWithCollectorManager[*FirstPassGroupingCollector[*util.BytesRef], []*SearchGroup[*util.BytesRef]](s, q, m)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		return groups
	case *FirstPassGroupingCollectorManager[mutable.MutableValue]:
		groups, err := search.SearchWithCollectorManager[*FirstPassGroupingCollector[mutable.MutableValue], []*SearchGroup[mutable.MutableValue]](s, q, m)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		return groups
	}
	panic(fmt.Sprintf("unexpected manager %T", manager))
}

// createSecondPassCollector renders the private <T>
// createSecondPassCollector(FirstPassGroupingCollector, Sort, Sort, int, int,
// boolean) over the raw first pass collector.
func createSecondPassCollector(t testing.TB, firstPassGroupingCollector any, groupSort, sortWithinGroup *search.Sort,
	groupOffset, maxDocsPerGroup int, getMaxScores bool) any {
	t.Helper()
	switch c := firstPassGroupingCollector.(type) {
	case *FirstPassGroupingCollector[*util.BytesRef]:
		return newSecondPassFrom(t, c, groupSort, sortWithinGroup, groupOffset, maxDocsPerGroup, getMaxScores)
	case *FirstPassGroupingCollector[mutable.MutableValue]:
		return newSecondPassFrom(t, c, groupSort, sortWithinGroup, groupOffset, maxDocsPerGroup, getMaxScores)
	}
	panic(fmt.Sprintf("unexpected collector %T", firstPassGroupingCollector))
}

func newSecondPassFrom[T any](t testing.TB, firstPassGroupingCollector *FirstPassGroupingCollector[T],
	groupSort, sortWithinGroup *search.Sort, groupOffset, maxDocsPerGroup int, getMaxScores bool) *TopGroupsCollector[T] {
	t.Helper()
	searchGroups, err := firstPassGroupingCollector.GetTopGroups(groupOffset)
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewTopGroupsCollector(firstPassGroupingCollector.GetGroupSelector(), searchGroups, groupSort,
		sortWithinGroup, maxDocsPerGroup, getMaxScores)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// createSecondPassCollectorManager renders the private
// createSecondPassCollectorManager(boolean, String,
// Collection<SearchGroup<BytesRef>>, Sort, Sort, int, int, boolean): it
// basically converts searchGroups from MutableValue to BytesRef if grouping by
// ValueSource.
func createSecondPassCollectorManager(isTermGroupSelector bool, groupField string,
	searchGroups []*SearchGroup[*util.BytesRef], groupSort, sortWithinGroup *search.Sort, withinGroupOffset,
	maxDocsPerGroup int, getMaxScores bool) any {
	if isTermGroupSelector {
		return NewTopGroupsCollectorManager(func() GroupSelector[*util.BytesRef] { return NewTermGroupSelector(groupField) },
			searchGroups, groupSort, sortWithinGroup, withinGroupOffset, maxDocsPerGroup, getMaxScores)
	}
	vs := valuesource.NewBytesRefFieldSource(groupField)
	mvalSearchGroups := make([]*SearchGroup[mutable.MutableValue], 0, len(searchGroups))
	for _, mergedTopGroup := range searchGroups {
		sg := &SearchGroup[mutable.MutableValue]{}
		groupValue := mutable.NewMutableValueStr()
		if mergedTopGroup.GroupValue != nil {
			groupValue.Value = mergedTopGroup.GroupValue.Utf8ToString()
		} else {
			groupValue.SetExists(false)
		}
		sg.GroupValue = groupValue
		sg.SortValues = mergedTopGroup.SortValues
		mvalSearchGroups = append(mvalSearchGroups, sg)
	}
	return NewTopGroupsCollectorManager(func() GroupSelector[mutable.MutableValue] {
		return NewValueSourceGroupSelector(vs, function.Context{})
	}, mvalSearchGroups, groupSort, sortWithinGroup, withinGroupOffset, maxDocsPerGroup, getMaxScores)
}

// createAllGroupsCollector renders the private
// createAllGroupsCollector(FirstPassGroupingCollector<?>, String).
func createAllGroupsCollector(firstPassGroupingCollector any, groupField string) any {
	switch c := firstPassGroupingCollector.(type) {
	case *FirstPassGroupingCollector[*util.BytesRef]:
		return NewAllGroupsCollector(c.GetGroupSelector())
	case *FirstPassGroupingCollector[mutable.MutableValue]:
		return NewAllGroupsCollector(c.GetGroupSelector())
	}
	panic(fmt.Sprintf("unexpected collector %T", firstPassGroupingCollector))
}

// allGroupsGroupCount renders AllGroupsCollector<?>.getGroupCount().
func allGroupsGroupCount(allGroupsCollector any) int {
	switch c := allGroupsCollector.(type) {
	case *AllGroupsCollector[*util.BytesRef]:
		return c.GetGroupCount()
	case *AllGroupsCollector[mutable.MutableValue]:
		return c.GetGroupCount()
	}
	panic(fmt.Sprintf("unexpected collector %T", allGroupsCollector))
}

// mutableValueStrBytes renders ((MutableValueStr) mv).value.get().
func mutableValueStrBytes(mv mutable.MutableValue) *util.BytesRef {
	return util.NewBytesRef([]byte(mv.(*mutable.MutableValueStr).Value))
}

// getSearchGroups renders the private getSearchGroups(
// FirstPassGroupingCollector<?>, int).
func getSearchGroups(t testing.TB, c any, groupOffset int) []*SearchGroup[*util.BytesRef] {
	t.Helper()
	switch collector := c.(type) {
	case *FirstPassGroupingCollector[*util.BytesRef]:
		groups, err := collector.GetTopGroups(groupOffset)
		if err != nil {
			t.Fatal(err)
		}
		return groups
	case *FirstPassGroupingCollector[mutable.MutableValue]:
		mutableValueGroups, err := collector.GetTopGroups(groupOffset)
		if err != nil {
			t.Fatal(err)
		}
		if mutableValueGroups == nil {
			return nil
		}

		groups := make([]*SearchGroup[*util.BytesRef], 0, len(mutableValueGroups))
		for _, mutableValueGroup := range mutableValueGroups {
			sg := &SearchGroup[*util.BytesRef]{}
			if mutableValueGroup.GroupValue.Exists() {
				sg.GroupValue = mutableValueStrBytes(mutableValueGroup.GroupValue)
			}
			sg.SortValues = mutableValueGroup.SortValues
			groups = append(groups, sg)
		}
		return groups
	}
	t.Fatal("unexpected first pass collector")
	return nil
}

// toByteRefSearchGroups renders the private toByteRefSearchGroups(Collection<?>).
func toByteRefSearchGroups(groups any) []*SearchGroup[*util.BytesRef] {
	switch g := groups.(type) {
	case []*SearchGroup[*util.BytesRef]:
		return g
	case []*SearchGroup[mutable.MutableValue]:
		if len(g) == 0 {
			return []*SearchGroup[*util.BytesRef]{}
		}
		result := make([]*SearchGroup[*util.BytesRef], 0, len(g))
		for _, sg := range g {
			out := &SearchGroup[*util.BytesRef]{}
			mv := sg.GroupValue
			if mv.Exists() {
				out.GroupValue = mutableValueStrBytes(mv)
			}
			out.SortValues = sg.SortValues
			result = append(result, out)
		}
		return result
	}
	return nil
}

// toByteTopGroups renders the private toByteTopGroups(TopGroups<?>).
func toByteTopGroups(topGroups any) *TopGroups[*util.BytesRef] {
	switch tg := topGroups.(type) {
	case *TopGroups[*util.BytesRef]:
		return tg
	case *TopGroups[mutable.MutableValue]:
		if tg == nil {
			return nil
		}
		return mutableTopGroupsToBytes(tg)
	}
	return nil
}

// mutableTopGroupsToBytes converts MutableValueStr groups to BytesRef groups,
// the loop shared by toByteTopGroups and getTopGroups.
func mutableTopGroupsToBytes(mvalTopGroups *TopGroups[mutable.MutableValue]) *TopGroups[*util.BytesRef] {
	groups := make([]*GroupDocs[*util.BytesRef], 0, len(mvalTopGroups.Groups))
	for _, mvalGd := range mvalTopGroups.Groups {
		var groupValue *util.BytesRef
		if mvalGd.GroupValue.Exists() {
			groupValue = mutableValueStrBytes(mvalGd.GroupValue)
		}
		groups = append(groups, NewGroupDocs(float32(math.NaN()), mvalGd.MaxScore, mvalGd.TotalHits,
			mvalGd.ScoreDocs, groupValue, mvalGd.GroupSortValues))
	}
	return NewTopGroups(mvalTopGroups.GroupSort, mvalTopGroups.WithinGroupSort, mvalTopGroups.TotalHitCount,
		mvalTopGroups.TotalGroupedHitCount, groups, float32(math.NaN()))
}

// getGroupingTopGroups renders the private getTopGroups(TopGroupsCollector, int).
func getGroupingTopGroups(t testing.TB, c any, withinGroupOffset int) *TopGroups[*util.BytesRef] {
	t.Helper()
	switch collector := c.(type) {
	case *TopGroupsCollector[*util.BytesRef]:
		tg, err := collector.GetTopGroups(withinGroupOffset)
		if err != nil {
			t.Fatal(err)
		}
		return tg
	case *TopGroupsCollector[mutable.MutableValue]:
		mvalTopGroups, err := collector.GetTopGroups(withinGroupOffset)
		if err != nil {
			t.Fatal(err)
		}
		// NOTE: currenlty using diamond operator on MergedIterator (without explicit Term class)
		// causes
		// errors on Eclipse Compiler (ecj) used for javadoc lint
		return mutableTopGroupsToBytes(mvalTopGroups)
	}
	t.Fatal("unexpected second pass collector")
	return nil
}

// groupingGroupDoc renders the private static class GroupDoc.
type groupingGroupDoc struct {
	id    int
	group *util.BytesRef
	sort1 *util.BytesRef
	sort2 *util.BytesRef
	// content must be "realN ..."
	content string
	score   float32
	score2  float32
}

func getRandomGroupingSort() *search.Sort {
	sortFields := make([]*search.SortField, 0)
	if random().Intn(7) == 2 {
		sortFields = append(sortFields, search.FieldScore)
	} else {
		if random().Intn(2) == 0 {
			if random().Intn(2) == 0 {
				sortFields = append(sortFields, search.NewSortFieldWithReverse("sort1", spi.SortFieldTypeString, random().Intn(2) == 0))
			} else {
				sortFields = append(sortFields, search.NewSortFieldWithReverse("sort2", spi.SortFieldTypeString, random().Intn(2) == 0))
			}
		} else if random().Intn(2) == 0 {
			sortFields = append(sortFields, search.NewSortFieldWithReverse("sort1", spi.SortFieldTypeString, random().Intn(2) == 0))
			sortFields = append(sortFields, search.NewSortFieldWithReverse("sort2", spi.SortFieldTypeString, random().Intn(2) == 0))
		}
	}
	// Break ties:
	sortFields = append(sortFields, search.NewSortField("id", spi.SortFieldTypeInt))
	return search.NewSort(sortFields...)
}

func getGroupingComparator(t testing.TB, sort *search.Sort) func(d1, d2 *groupingGroupDoc) int {
	sortFields := sort.GetSort()
	return func(d1, d2 *groupingGroupDoc) int {
		for _, sf := range sortFields {
			var cmp int
			if sf.Type == spi.SortFieldTypeScore {
				if d1.score > d2.score {
					cmp = -1
				} else if d1.score < d2.score {
					cmp = 1
				} else {
					cmp = 0
				}
			} else if sf.Field == "sort1" {
				cmp = d1.sort1.BytesRefCompareTo(d2.sort1)
			} else if sf.Field == "sort2" {
				cmp = d1.sort2.BytesRefCompareTo(d2.sort2)
			} else {
				if sf.Field != "id" {
					t.Fatalf("expected sort field id, got %s", sf.Field)
				}
				cmp = d1.id - d2.id
			}
			if cmp != 0 {
				if sf.Reverse {
					return -cmp
				}
				return cmp
			}
		}
		// Our sort always fully tie breaks:
		t.Fatal("sort does not fully break ties")
		return 0
	}
}

// fillFields renders the private fillFields(GroupDoc, Sort): the boxed Float
// score, the BytesRef sort values and the Integer id.
func fillFields(t testing.TB, d *groupingGroupDoc, sort *search.Sort) []any {
	sortFields := sort.GetSort()
	fields := make([]any, len(sortFields))
	for fieldIDX, sf := range sortFields {
		var c any
		if sf.Type == spi.SortFieldTypeScore {
			c = d.score
		} else if sf.Field == "sort1" {
			c = d.sort1
		} else if sf.Field == "sort2" {
			c = d.sort2
		} else {
			if sf.Field != "id" {
				t.Fatalf("expected sort field id, got %s", sf.Field)
			}
			c = int32(d.id)
		}
		fields[fieldIDX] = c
	}
	return fields
}

func groupToString(b *util.BytesRef) string {
	if b == nil {
		return "null"
	}
	return b.Utf8ToString()
}

// expectedFieldDocs holds, for the FieldDocs slowGrouping builds, the
// FieldDoc behind each ScoreDoc of an expected GroupDocs: Java's ScoreDoc[]
// holds the FieldDocs themselves, Gocene's holds *ScoreDoc values.
type expectedFieldDocs map[*search.ScoreDoc]*search.FieldDoc

func slowGrouping(t testing.TB, groupDocs []*groupingGroupDoc, searchTerm string, getMaxScores, doAllGroups bool,
	groupSort, docSort *search.Sort, topNGroups, docsPerGroup, groupOffset, docOffset int,
	fieldDocs expectedFieldDocs) *TopGroups[*util.BytesRef] {

	groupSortComp := getGroupingComparator(t, groupSort)

	sort.SliceStable(groupDocs, func(i, j int) bool { return groupSortComp(groupDocs[i], groupDocs[j]) < 0 })
	groups := newGroupMap[*util.BytesRef, []*groupingGroupDoc]()
	sortedGroups := make([]*util.BytesRef, 0)
	sortedGroupFields := make([][]any, 0)

	totalHitCount := 0
	knownGroups := newGroupSet[*util.BytesRef]()

	for _, d := range groupDocs {
		// TODO: would be better to filter by searchTerm before sorting!
		if !strings.HasPrefix(d.content, searchTerm) {
			continue
		}
		totalHitCount++

		if doAllGroups {
			if !knownGroups.contains(d.group) {
				knownGroups.add(d.group)
			}
		}

		l, ok := groups.get(d.group)
		if !ok {
			sortedGroups = append(sortedGroups, d.group)
			sortedGroupFields = append(sortedGroupFields, fillFields(t, d, groupSort))
			l = make([]*groupingGroupDoc, 0)
		}
		groups.put(d.group, append(l, d))
	}

	if groupOffset >= len(sortedGroups) {
		// slice is out of bounds
		return nil
	}

	limit := min(groupOffset+topNGroups, groups.size())

	docSortComp := getGroupingComparator(t, docSort)
	result := make([]*GroupDocs[*util.BytesRef], limit-groupOffset)
	totalGroupedHitCount := 0
	for idx := groupOffset; idx < limit; idx++ {
		group := sortedGroups[idx]
		docs, _ := groups.get(group)
		totalGroupedHitCount += len(docs)
		sort.SliceStable(docs, func(i, j int) bool { return docSortComp(docs[i], docs[j]) < 0 })
		var hits []*search.ScoreDoc
		if len(docs) > docOffset {
			docIDXLimit := min(docOffset+docsPerGroup, len(docs))
			hits = make([]*search.ScoreDoc, docIDXLimit-docOffset)
			for docIDX := docOffset; docIDX < docIDXLimit; docIDX++ {
				d := docs[docIDX]
				fd := search.NewFieldDocWithFields(d.id, float32(math.NaN()), fillFields(t, d, docSort))
				hits[docIDX-docOffset] = fd.ScoreDoc
				fieldDocs[fd.ScoreDoc] = fd
			}
		} else {
			hits = []*search.ScoreDoc{}
		}

		result[idx-groupOffset] = NewGroupDocs(
			float32(math.NaN()),
			0.0,
			search.NewTotalHits(int64(len(docs)), search.EQUAL_TO),
			hits,
			group,
			sortedGroupFields[idx])
	}

	if doAllGroups {
		knownGroupsSize := knownGroups.size()
		return NewTopGroupsWithTotalGroupCount(
			NewTopGroups(groupSort.GetSort(), docSort.GetSort(), totalHitCount, totalGroupedHitCount, result,
				float32(math.NaN())),
			&knownGroupsSize)
	}
	return NewTopGroups(groupSort.GetSort(), docSort.GetSort(), totalHitCount, totalGroupedHitCount, result,
		float32(math.NaN()))
}

func getDocBlockReader(t testing.TB, dir store.Directory, groupDocs []*groupingGroupDoc) *index.DirectoryReader {
	t.Helper()
	// Coalesce by group, but in random order:
	shuffleGroupDocs(groupDocs)
	groupMap := newGroupMap[*util.BytesRef, []*groupingGroupDoc]()
	groupValues := make([]*util.BytesRef, 0)

	for _, groupDoc := range groupDocs {
		l, ok := groupMap.get(groupDoc.group)
		if !ok {
			groupValues = append(groupValues, groupDoc.group)
		}
		groupMap.put(groupDoc.group, append(l, groupDoc))
	}

	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newMergePolicyNoMock(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	updateDocs := make([][]*document.Document, 0)

	groupEndType := document.NewFieldTypeFrom(document.StringFieldTypeNotStored)
	groupEndType.SetIndexOptions(index.IndexOptionsDocs)
	groupEndType.SetOmitNorms(true)

	for _, group := range groupValues {
		docs := make([]*document.Document, 0)
		members, _ := groupMap.get(group)
		for _, groupValue := range members {
			doc := document.NewDocument()
			docs = append(docs, doc)
			if groupValue.group != nil {
				doc.Add(newStringField(t, "group", groupValue.group.Utf8ToString(), true))
				doc.Add(mustSortedDVFieldBytes(t, "group", util.BytesRefDeepCopyOf(groupValue.group)))
			}
			doc.Add(newStringField(t, "sort1", groupValue.sort1.Utf8ToString(), false))
			doc.Add(mustSortedDVFieldBytes(t, "sort1", util.BytesRefDeepCopyOf(groupValue.sort1)))
			doc.Add(newStringField(t, "sort2", groupValue.sort2.Utf8ToString(), false))
			doc.Add(mustSortedDVFieldBytes(t, "sort2", util.BytesRefDeepCopyOf(groupValue.sort2)))
			doc.Add(mustNumericDVField(t, "id", int64(groupValue.id)))
			doc.Add(newTextField(t, "content", groupValue.content, false))
		}
		// So we can pull filter marking last doc in block:
		groupEnd := newField(t, "groupend", "x", groupEndType)
		docs[len(docs)-1].Add(groupEnd)
		// Add as a doc block:
		mustAddDocuments(t, w, docs)
		if group != nil && random().Intn(7) == 4 {
			updateDocs = append(updateDocs, docs)
		}
	}

	for _, docs := range updateDocs {
		// Just replaces docs w/ same docs:
		if _, err := w.UpdateDocuments(index.NewTerm("group", docs[0].Get("group").StringValue()), docs); err != nil {
			t.Fatalf("updateDocuments: %v", err)
		}
	}

	r := mustGetReader(t, w)
	mustClose(t, w)

	return r
}

// shuffleGroupDocs renders Collections.shuffle(Arrays.asList(groupDocs), random()).
func shuffleGroupDocs(list []*groupingGroupDoc) {
	r := random()
	for i := len(list); i > 1; i-- {
		j := r.Intn(i)
		list[i-1], list[j] = list[j], list[i-1]
	}
}

func mustSortedDVFieldBytes(t testing.TB, name string, value *util.BytesRef) *document.SortedDocValuesField {
	t.Helper()
	f, err := document.NewSortedDocValuesField(name, value.ValidBytes())
	if err != nil {
		t.Fatalf("new SortedDocValuesField: %v", err)
	}
	return f
}

// shardState renders the private static class ShardState.
type shardState struct {
	subSearchers []*shardSearcher
	docStarts    []int
}

func newShardState(t testing.TB, s *search.IndexSearcher) *shardState {
	t.Helper()
	ctx := s.GetTopReaderContext()
	leaves, err := ctx.Leaves()
	if err != nil {
		t.Fatal(err)
	}
	st := &shardState{subSearchers: make([]*shardSearcher, len(leaves))}
	for searcherIDX := range st.subSearchers {
		st.subSearchers[searcherIDX] = newShardSearcher(leaves[searcherIDX], ctx)
	}

	st.docStarts = make([]int, len(st.subSearchers))
	for subIDX := range st.docStarts {
		st.docStarts[subIDX] = leaves[subIDX].DocBase
	}
	return st
}

func TestGroupingRandom(t *testing.T) {
	numberOfRuns := atLeast(1)
	for iter := 0; iter < numberOfRuns; iter++ {
		if verbose {
			fmt.Printf("TEST: iter=%d\n", iter)
		}

		numDocs := atLeast(100)
		// final int numDocs = _TestUtil.nextInt(random, 5, 20);
		numGroups := nextInt(1, numDocs)

		if verbose {
			fmt.Printf("TEST: numDocs=%d numGroups=%d\n", numDocs, numGroups)
		}

		groups := make([]*util.BytesRef, 0, numGroups)
		for i := 0; i < numGroups; i++ {
			var randomValue string
			for {
				// B/c of DV based impl we can't see the difference between an empty string and a
				// null value.
				// For that reason we don't generate empty string
				// groups.
				randomValue = randomRealisticUnicodeString(random())
				// randomValue = TestUtil.randomSimpleString(random());
				if randomValue != "" {
					break
				}
			}

			groups = append(groups, util.NewBytesRef([]byte(randomValue)))
		}
		contentStrings := make([]string, nextInt(2, 20))
		if verbose {
			fmt.Println("TEST: create fake content")
		}
		for contentIDX := range contentStrings {
			var sb strings.Builder
			sb.WriteString("real")
			sb.WriteString(strconv.Itoa(random().Intn(3)))
			sb.WriteString(" ")
			fakeCount := random().Intn(10)
			for fakeIDX := 0; fakeIDX < fakeCount; fakeIDX++ {
				sb.WriteString("fake ")
			}
			contentStrings[contentIDX] = sb.String()
			if verbose {
				fmt.Println("  content=" + sb.String())
			}
		}

		dir := newDirectory()
		iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
		iwc.SetMergePolicy(newMergePolicyNoMock(t))
		w := newRandomIndexWriterWithConfig(t, dir, iwc)
		doc := document.NewDocument()
		docNoGroup := document.NewDocument()
		idvGroupField := mustSortedDVField(t, "group", "")
		doc.Add(idvGroupField)
		docNoGroup.Add(idvGroupField)

		group := newStringField(t, "group", "", false)
		doc.Add(group)
		docNoGroup.Add(group)
		sort1 := mustSortedDVField(t, "sort1", "")
		doc.Add(sort1)
		docNoGroup.Add(sort1)
		sort2 := mustSortedDVField(t, "sort2", "")
		doc.Add(sort2)
		docNoGroup.Add(sort2)
		content := newTextField(t, "content", "", false)
		doc.Add(content)
		docNoGroup.Add(content)
		idDV := mustNumericDVField(t, "id", 0)
		doc.Add(idDV)
		docNoGroup.Add(idDV)
		groupDocs := make([]*groupingGroupDoc, numDocs)
		for i := 0; i < numDocs; i++ {
			var groupValue *util.BytesRef
			if random().Intn(24) == 17 {
				// So we test the "doc doesn't have the group'd
				// field" case:
				groupValue = nil
			} else {
				groupValue = groups[random().Intn(len(groups))]
			}
			groupDoc := &groupingGroupDoc{
				id:      i,
				group:   groupValue,
				sort1:   groups[random().Intn(len(groups))],
				sort2:   groups[random().Intn(len(groups))],
				content: contentStrings[random().Intn(len(contentStrings))],
			}
			if verbose {
				fmt.Printf("  doc content=%s id=%d group=%s sort1=%s sort2=%s\n", groupDoc.content, i,
					groupToString(groupDoc.group), groupDoc.sort1.Utf8ToString(), groupDoc.sort2.Utf8ToString())
			}

			groupDocs[i] = groupDoc
			if groupDoc.group != nil {
				group.SetStringValue(groupDoc.group.Utf8ToString())
				idvGroupField.SetBytesValue(util.BytesRefDeepCopyOf(groupDoc.group).ValidBytes())
			} else {
				// TODO: not true
				// Must explicitly set empty string, else eg if
				// the segment has all docs missing the field then
				// we get null back instead of empty BytesRef:
				idvGroupField.SetBytesValue([]byte{})
			}
			sort1.SetBytesValue(util.BytesRefDeepCopyOf(groupDoc.sort1).ValidBytes())
			sort2.SetBytesValue(util.BytesRefDeepCopyOf(groupDoc.sort2).ValidBytes())
			content.SetStringValue(groupDoc.content)
			idDV.SetLongValue(int64(groupDoc.id))
			if groupDoc.group == nil {
				mustAddDocument(t, w, docNoGroup)
			} else {
				mustAddDocument(t, w, doc)
			}
		}

		groupDocsByID := make([]*groupingGroupDoc, len(groupDocs))
		copy(groupDocsByID, groupDocs)

		r := mustGetReader(t, w)
		mustClose(t, w)

		docIDToID := readIDs(t, r)

		s := newSearcher(t, r)
		// This test relies on the fact that longer fields produce lower scores
		s.SetSimilarity(search.NewLuceneBM25Similarity())

		if verbose {
			fmt.Printf("\nTEST: searcher=%v\n", s)
		}

		shards := newShardState(t, s)

		seenIDs := map[int]struct{}{}
		for contentID := 0; contentID < 3; contentID++ {
			hits := mustSearch(t, s, search.NewTermQuery(index.NewTerm("content", "real"+strconv.Itoa(contentID))), numDocs).ScoreDocs
			for _, hit := range hits {
				idValue := docIDToID[hit.Doc]

				gd := groupDocs[idValue]
				seenIDs[idValue] = struct{}{}
				if gd.score != 0.0 {
					t.Fatalf("group doc %d scored twice", gd.id)
				}
				gd.score = hit.Score
				assertIntEquals(t, gd.id, idValue)
			}
		}

		// make sure all groups were seen across the hits
		assertIntEquals(t, len(groupDocs), len(seenIDs))

		for _, gd := range groupDocs {
			if math.IsInf(float64(gd.score), 0) || gd.score != gd.score {
				t.Fatalf("score of doc %d is not finite: %v", gd.id, gd.score)
			}
			if !(gd.score >= 0.0) {
				t.Fatalf("score of doc %d is negative: %v", gd.id, gd.score)
			}
		}

		// Build 2nd index, where docs are added in blocks by
		// group, so we can use single pass collector
		dirBlocks := newDirectory()
		rBlocks := getDocBlockReader(t, dirBlocks, groupDocs)
		lastDocInBlock := search.NewTermQuery(index.NewTerm("groupend", "x"))

		sBlocks := newSearcher(t, rBlocks)
		// This test relies on the fact that longer fields produce lower scores
		sBlocks.SetSimilarity(search.NewLuceneBM25Similarity())

		shardsBlocks := newShardState(t, sBlocks)

		// ReaderBlocks only increases maxDoc() vs reader, which
		// means a monotonic shift in scores, so we can
		// reliably remap them w/ Map:
		scoreMap := map[string]map[float32]float32{}

		docIDToIDBlocks := readIDs(t, rBlocks)

		// Tricky: must separately set .score2, because the doc
		// block index was created with possible deletions!
		for contentID := 0; contentID < 3; contentID++ {
			termScoreMap := map[float32]float32{}
			scoreMap["real"+strconv.Itoa(contentID)] = termScoreMap
			hits := mustSearch(t, sBlocks, search.NewTermQuery(index.NewTerm("content", "real"+strconv.Itoa(contentID))), numDocs).ScoreDocs
			for _, hit := range hits {
				gd := groupDocsByID[docIDToIDBlocks[hit.Doc]]
				if gd.score2 != 0.0 {
					t.Fatalf("group doc %d scored twice", gd.id)
				}
				gd.score2 = hit.Score
				assertIntEquals(t, gd.id, docIDToIDBlocks[hit.Doc])
				termScoreMap[gd.score] = gd.score2
			}
		}

		for searchIter := 0; searchIter < 100; searchIter++ {
			if verbose {
				fmt.Printf("\nTEST: searchIter=%d\n", searchIter)
			}

			searchTerm := "real" + strconv.Itoa(random().Intn(3))
			getMaxScores := random().Intn(2) == 0
			groupSort := getRandomGroupingSort()
			// final Sort groupSort = new Sort(new SortField[] {new SortField("sort1",
			// SortField.STRING), new SortField("id", SortField.INT)});
			docSort := getRandomGroupingSort()

			topNGroups := nextInt(1, 30)
			// final int topNGroups = 10;
			docsPerGroup := nextInt(1, 50)

			groupOffset := nextInt(0, (topNGroups-1)/2)
			// final int groupOffset = 0;

			docOffset := nextInt(0, docsPerGroup-1)
			// final int docOffset = 0;

			doCache := random().Intn(2) == 0
			doAllGroups := random().Intn(2) == 0

			groupField := "group"
			c1 := createRandomFirstPassCollector(t, groupField, groupSort, groupOffset+topNGroups)
			var cCache search.CachingCollector
			var c search.Collector

			var allGroupsCollector any
			if doAllGroups {
				allGroupsCollector = createAllGroupsCollector(c1, groupField)
			}

			useWrappingCollector := random().Intn(2) == 0

			if doCache {
				maxCacheMB := random().Float64()
				if verbose {
					fmt.Printf("TEST: maxCacheMB=%v\n", maxCacheMB)
				}

				if useWrappingCollector {
					if doAllGroups {
						cCache = search.CreateCachingCollectorWithOther(c1.(search.Collector), true, maxCacheMB)
						c = mustMultiCollectorWrap(t, cCache, allGroupsCollector.(search.Collector))
					} else {
						cCache = search.CreateCachingCollectorWithOther(c1.(search.Collector), true, maxCacheMB)
						c = cCache
					}
				} else {
					// Collect only into cache, then replay multiple times:
					cCache = search.CreateCachingCollector(true, maxCacheMB)
					c = cCache
				}
			} else {
				cCache = nil
				if doAllGroups {
					c = mustMultiCollectorWrap(t, c1.(search.Collector), allGroupsCollector.(search.Collector))
				} else {
					c = c1.(search.Collector)
				}
			}

			// Search top reader:
			query := search.NewTermQuery(index.NewTerm("content", searchTerm))

			mustSearchCollector(t, s, query, c)

			if doCache && !useWrappingCollector {
				if cCache.IsCached() {
					// Replay for first-pass grouping
					mustReplay(t, cCache, c1.(search.Collector))
					if doAllGroups {
						// Replay for all groups:
						mustReplay(t, cCache, allGroupsCollector.(search.Collector))
					}
				} else {
					// Replay by re-running search:
					mustSearchCollector(t, s, query, c1.(search.Collector))
					if doAllGroups {
						mustSearchCollector(t, s, query, allGroupsCollector.(search.Collector))
					}
				}
			}

			// Get 1st pass top groups
			topGroups := getSearchGroups(t, c1, groupOffset)
			var groupsResult *TopGroups[*util.BytesRef]

			// Get 1st pass top groups using shards

			topGroupsShards := searchShards(t, shards.subSearchers, query, groupSort, docSort, groupOffset, topNGroups,
				docOffset, docsPerGroup, getMaxScores, true, true)
			if topGroups != nil {
				c2 := createSecondPassCollector(t, c1, groupSort, docSort, groupOffset, docOffset+docsPerGroup, getMaxScores)
				if doCache {
					if cCache.IsCached() {
						if verbose {
							fmt.Println("TEST: cache is intact")
						}
						mustReplay(t, cCache, c2.(search.Collector))
					} else {
						if verbose {
							fmt.Println("TEST: cache was too large")
						}
						mustSearchCollector(t, s, query, c2.(search.Collector))
					}
				} else {
					mustSearchCollector(t, s, query, c2.(search.Collector))
				}

				if doAllGroups {
					tempTopGroups := getGroupingTopGroups(t, c2, docOffset)
					groupCount := allGroupsGroupCount(allGroupsCollector)
					groupsResult = NewTopGroupsWithTotalGroupCount(tempTopGroups, &groupCount)
				} else {
					groupsResult = getGroupingTopGroups(t, c2, docOffset)
				}
			} else if verbose {
				fmt.Println("TEST:   no results")
			}

			expectedFields := expectedFieldDocs{}
			expectedGroups := slowGrouping(t, groupDocs, searchTerm, getMaxScores, doAllGroups, groupSort, docSort,
				topNGroups, docsPerGroup, groupOffset, docOffset, expectedFields)

			assertGroupingTopGroupsEquals(t, docIDToID, expectedGroups, groupsResult, expectedFields, true, true, true)

			// Confirm merged shards match:
			assertGroupingTopGroupsEquals(t, docIDToID, expectedGroups, topGroupsShards, expectedFields, true, false, true)
			if topGroupsShards != nil {
				verifyShards(t, shards.docStarts, topGroupsShards)
			}

			rewritten, err := sBlocks.Rewrite(lastDocInBlock)
			if err != nil {
				t.Fatal(err)
			}
			lastDocWeight, err := sBlocks.CreateWeight(rewritten, search.COMPLETE_NO_SCORES, 1)
			if err != nil {
				t.Fatal(err)
			}
			c3, err := NewBlockGroupingCollector(groupSort, groupOffset+topNGroups,
				groupSort.NeedsScores() || docSort.NeedsScores(), lastDocWeight)
			if err != nil {
				t.Fatal(err)
			}
			var allGroupsCollector2 *AllGroupsCollector[*util.BytesRef]
			var c4 search.Collector
			if doAllGroups {
				// NOTE: must be "group" and not "group_dv"
				// (groupField) because we didn't index doc
				// values in the block index:
				allGroupsCollector2 = NewAllGroupsCollector[*util.BytesRef](NewTermGroupSelector("group"))
				c4 = mustMultiCollectorWrap(t, c3, allGroupsCollector2)
			} else {
				c4 = c3
			}
			// Get block grouping result:
			mustSearchCollector(t, sBlocks, query, c4)
			rawTopGroupsBlocks, err := c3.GetTopGroups(docSort, groupOffset, docOffset, docOffset+docsPerGroup)
			if err != nil {
				t.Fatal(err)
			}
			var tempTopGroupsBlocks *TopGroups[*util.BytesRef]
			if rawTopGroupsBlocks != nil {
				tempTopGroupsBlocks = uncheckedTopGroupsCast[*util.BytesRef](rawTopGroupsBlocks)
			}
			var groupsResultBlocks *TopGroups[*util.BytesRef]
			if doAllGroups && tempTopGroupsBlocks != nil {
				assertIntEquals(t, *tempTopGroupsBlocks.TotalGroupCount, allGroupsCollector2.GetGroupCount())
				groupCount := allGroupsCollector2.GetGroupCount()
				groupsResultBlocks = NewTopGroupsWithTotalGroupCount(tempTopGroupsBlocks, &groupCount)
			} else {
				groupsResultBlocks = tempTopGroupsBlocks
			}

			// Get shard'd block grouping result:
			topGroupsBlockShards := searchShards(t, shardsBlocks.subSearchers, query, groupSort, docSort, groupOffset,
				topNGroups, docOffset, docsPerGroup, getMaxScores, false, false)

			if expectedGroups != nil {
				// Fixup scores for reader2
				for _, groupDocsHits := range expectedGroups.Groups {
					for _, hit := range groupDocsHits.ScoreDocs {
						gd := groupDocsByID[hit.Doc]
						assertIntEquals(t, gd.id, hit.Doc)
						hit.Score = gd.score2
					}
				}

				sortFields := groupSort.GetSort()
				termScoreMap := scoreMap[searchTerm]
				for groupSortIDX := range sortFields {
					if sortFields[groupSortIDX].Type == spi.SortFieldTypeScore {
						for _, groupDocsHits := range expectedGroups.Groups {
							if groupDocsHits.GroupSortValues != nil {
								remapped, ok := termScoreMap[groupDocsHits.GroupSortValues[groupSortIDX].(float32)]
								if !ok {
									t.Fatal("remapped group sort value is null")
								}
								groupDocsHits.GroupSortValues[groupSortIDX] = remapped
							}
						}
					}
				}

				docSortFields := docSort.GetSort()
				for docSortIDX := range docSortFields {
					if docSortFields[docSortIDX].Type == spi.SortFieldTypeScore {
						for _, groupDocsHits := range expectedGroups.Groups {
							for _, scoreDoc := range groupDocsHits.ScoreDocs {
								hit := expectedFields[scoreDoc]
								if hit.Fields != nil {
									remapped, ok := termScoreMap[hit.Fields[docSortIDX].(float32)]
									if !ok {
										t.Fatal("remapped doc sort value is null")
									}
									hit.Fields[docSortIDX] = remapped
								}
							}
						}
					}
				}
			}

			assertGroupingTopGroupsEquals(t, docIDToIDBlocks, expectedGroups, groupsResultBlocks, expectedFields, false, true, false)
			assertGroupingTopGroupsEquals(t, docIDToIDBlocks, expectedGroups, topGroupsBlockShards, expectedFields, false, false, false)
		}

		mustClose(t, r, dir)

		mustClose(t, rBlocks, dirBlocks)
	}
}

// readIDs renders the MultiDocValues.getNumericValues(r, "id") loop that maps
// each docID to its id value.
func readIDs(t testing.TB, r *index.DirectoryReader) []int {
	t.Helper()
	values, err := index.MultiDocValuesGetNumericValues(r, "id")
	if err != nil {
		t.Fatal(err)
	}
	if values == nil {
		t.Fatal("id values are null")
	}
	docIDToID := make([]int, r.MaxDoc())
	for i := 0; i < r.MaxDoc(); i++ {
		assertIntEquals(t, i, mustNextDoc(t, values))
		v, err := values.LongValue()
		if err != nil {
			t.Fatal(err)
		}
		docIDToID[i] = int(v)
	}
	return docIDToID
}

func mustMultiCollectorWrap(t testing.TB, collectors ...search.Collector) search.Collector {
	t.Helper()
	c, err := search.MultiCollectorWrap(collectors...)
	if err != nil {
		t.Fatalf("MultiCollector.wrap: %v", err)
	}
	return c
}

func mustReplay(t testing.TB, cache search.CachingCollector, other search.Collector) {
	t.Helper()
	if err := cache.Replay(other); err != nil {
		t.Fatalf("replay: %v", err)
	}
}

func verifyShards(t testing.TB, docStarts []int, topGroups *TopGroups[*util.BytesRef]) {
	t.Helper()
	for _, group := range topGroups.Groups {
		for _, sd := range group.ScoreDocs {
			if want := index.ReaderUtilSubIndex(sd.Doc, docStarts); want != sd.ShardIndex {
				t.Fatalf("doc=%d wrong shard: expected %d, got %d", sd.Doc, want, sd.ShardIndex)
			}
		}
	}
}

func searchShards(t testing.TB, subSearchers []*shardSearcher, query search.Query, groupSort, docSort *search.Sort,
	groupOffset, topNGroups, docOffset, topNDocs int, getMaxScores, canUseIDV, preFlex bool) *TopGroups[*util.BytesRef] {
	t.Helper()
	// TODO: swap in caching, all groups collector hereassertEquals(expected.totalHitCount,
	// actual.totalHitCount);
	// too...
	if verbose {
		fmt.Printf("TEST: %d shards: %v canUseIDV=%v\n", len(subSearchers), subSearchers, canUseIDV)
	}
	// Run 1st pass collector to get top groups per shard
	shardGroups := make([][]*SearchGroup[*util.BytesRef], 0)
	groupField := "group"

	// TermGroupSelector or ValueSourceGroupSelector
	isTermGroupSelector := random().Intn(2) == 0

	for shardIDX := range subSearchers {
		firstPassGroupingCollectorManager := createFirstPassCollectorManager(t, isTermGroupSelector, groupField,
			groupSort, 0, groupOffset+topNGroups)

		topGroups := toByteRefSearchGroups(subSearchers[shardIDX].searchFirstPass(t, query, firstPassGroupingCollectorManager))
		shardGroups = append(shardGroups, topGroups)
	}

	mergedTopGroups := MergeSearchGroups(shardGroups, groupOffset, topNGroups, groupSort)

	if len(mergedTopGroups) != 0 {
		// Now 2nd pass:
		shardTopGroups := make([]*TopGroups[*util.BytesRef], len(subSearchers))
		for shardIDX := range subSearchers {
			secondPassCollectorManager := createSecondPassCollectorManager(isTermGroupSelector, groupField,
				mergedTopGroups, groupSort, docSort, 0, docOffset+topNDocs, getMaxScores)

			topGroupsRaw := subSearchers[shardIDX].searchSecondPass(t, query, secondPassCollectorManager)
			shardTopGroups[shardIDX] = toByteTopGroups(topGroupsRaw)
		}

		mergedGroups, err := MergeTopGroups(shardTopGroups, groupSort, docSort, docOffset, topNDocs, ScoreMergeModeNone)
		if err != nil {
			t.Fatal(err)
		}
		return mergedGroups
	}
	return nil
}

func assertGroupingTopGroupsEquals(t testing.TB, docIDtoID []int, expected, actual *TopGroups[*util.BytesRef],
	expectedFields expectedFieldDocs, verifyGroupValues, verifyTotalGroupCount, idvBasedImplsUsed bool) {
	t.Helper()
	if expected == nil {
		if actual != nil {
			t.Fatalf("expected null, got %v", actual)
		}
		return
	}
	if actual == nil {
		t.Fatal("actual is null")
	}

	if len(expected.Groups) != len(actual.Groups) {
		t.Fatalf("expected.groups.length != actual.groups.length: %d != %d", len(expected.Groups), len(actual.Groups))
	}
	if expected.TotalHitCount != actual.TotalHitCount {
		t.Fatalf("expected.totalHitCount != actual.totalHitCount: %d != %d", expected.TotalHitCount, actual.TotalHitCount)
	}
	if expected.TotalGroupedHitCount != actual.TotalGroupedHitCount {
		t.Fatalf("expected.totalGroupedHitCount != actual.totalGroupedHitCount: %d != %d",
			expected.TotalGroupedHitCount, actual.TotalGroupedHitCount)
	}
	if expected.TotalGroupCount != nil && verifyTotalGroupCount {
		if !equalIntPtr(expected.TotalGroupCount, actual.TotalGroupCount) {
			t.Fatalf("expected.totalGroupCount != actual.totalGroupCount: %v != %v", expected.TotalGroupCount, actual.TotalGroupCount)
		}
	}

	for groupIDX := range expected.Groups {
		if verbose {
			fmt.Printf("  check groupIDX=%d\n", groupIDX)
		}
		expectedGroup := expected.Groups[groupIDX]
		actualGroup := actual.Groups[groupIDX]
		if verifyGroupValues {
			if idvBasedImplsUsed {
				if actualGroup.GroupValue.Length == 0 {
					if expectedGroup.GroupValue != nil {
						t.Fatalf("group %d: expected a null group value, got %v", groupIDX, expectedGroup.GroupValue)
					}
				} else if !javaEquals(expectedGroup.GroupValue, actualGroup.GroupValue) {
					t.Fatalf("group %d value: expected %v, got %v", groupIDX, expectedGroup.GroupValue, actualGroup.GroupValue)
				}
			} else if !javaEquals(expectedGroup.GroupValue, actualGroup.GroupValue) {
				t.Fatalf("group %d value: expected %v, got %v", groupIDX, expectedGroup.GroupValue, actualGroup.GroupValue)
			}
		}
		assertObjectArrayEquals(t, expectedGroup.GroupSortValues, actualGroup.GroupSortValues)

		// TODO
		// assertEquals(expectedGroup.maxScore, actualGroup.maxScore);
		if expectedGroup.TotalHits.Value != actualGroup.TotalHits.Value {
			t.Fatalf("group %d totalHits: expected %d, got %d", groupIDX, expectedGroup.TotalHits.Value, actualGroup.TotalHits.Value)
		}

		expectedFDs := expectedGroup.ScoreDocs
		actualFDs := actualGroup.ScoreDocs

		assertIntEquals(t, len(expectedFDs), len(actualFDs))
		for docIDX := range expectedFDs {
			expectedFD := expectedFields[expectedFDs[docIDX]]
			actualFD := actualFDs[docIDX]
			assertIntEquals(t, expectedFD.Doc, docIDtoID[actualFD.Doc])
			// assertArrayEquals(expectedFD.fields, actualFD.fields): the actual FieldDoc behind the
			// ScoreDoc is unreachable.
			t.Fatal(fieldDocOfScoreDocBlocker)
		}
	}
}

// assertObjectArrayEquals renders assertArrayEquals(Object[], Object[]).
func assertObjectArrayEquals(t testing.TB, expected, actual []any) {
	t.Helper()
	if expected == nil || actual == nil {
		if (expected == nil) != (actual == nil) {
			t.Fatalf("arrays: expected %v, got %v", expected, actual)
		}
		return
	}
	if len(expected) != len(actual) {
		t.Fatalf("array lengths: expected %d, got %d", len(expected), len(actual))
	}
	for i := range expected {
		if !javaEquals(expected[i], actual[i]) {
			t.Fatalf("arrays first differed at element [%d]: expected %v, got %v", i, expected[i], actual[i])
		}
	}
}

// shardSearcherBlocker names what the private static class ShardSearcher
// needs: it extends IndexSearcher and calls the protected
// IndexSearcher.searchLeaf(LeafReaderContext, int, int, Weight, Collector),
// which Gocene keeps unexported.
const shardSearcherBlocker = "requires org.apache.lucene.search.IndexSearcher.searchLeaf(LeafReaderContext, int, int, " +
	"Weight, Collector) (protected; not exported by Gocene)"

// shardSearcher renders the private static class ShardSearcher, which extends
// IndexSearcher over the parent context and searches one leaf.
type shardSearcher struct {
	*search.IndexSearcher
	ctx *index.LeafReaderContext
}

func newShardSearcher(ctx *index.LeafReaderContext, parent index.IndexReaderContext) *shardSearcher {
	return &shardSearcher{IndexSearcher: search.NewIndexSearcherFromContext(parent), ctx: ctx}
}

// searchFirstPass renders the overridden
// search(Query, CollectorManager<C, T>) for a FirstPassGroupingCollectorManager<?>.
func (s *shardSearcher) searchFirstPass(t testing.TB, query search.Query, manager any) any {
	t.Helper()
	t.Fatal(shardSearcherBlocker)
	return nil
}

// searchSecondPass renders the overridden
// search(Query, CollectorManager<C, T>) for a TopGroupsCollectorManager<?>.
func (s *shardSearcher) searchSecondPass(t testing.TB, query search.Query, manager any) any {
	t.Helper()
	t.Fatal(shardSearcherBlocker)
	return nil
}

// String renders toString().
func (s *shardSearcher) String() string {
	return fmt.Sprintf("ShardSearcher(%v)", s.ctx.LeafReader())
}
