// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"fmt"
	"sort"
	"strconv"
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
// lucene/grouping/src/test/org/apache/lucene/search/grouping/TestDistinctValuesCollector.java
// (Apache Lucene 10.5.0), which extends AbstractGroupingTestCase.
//
// Java instantiates the grouping generics with the raw Comparable<Object> and
// casts the BytesRef or MutableValue selectors to it unchecked; the Go
// rendering is GroupSelector[any] over erasedGroupSelector.

const (
	distinctGroupField = "author"
	distinctCountField = "publisher"
)

// distinctGroupCount renders DistinctValuesCollector.GroupCount<Comparable<Object>, Comparable<Object>>.
type distinctGroupCount = DistinctValuesGroupCount[any, any]

func TestDistinctValuesCollectorSimple(t *testing.T) {
	rnd := random()
	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newLogMergePolicy())
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	doc := document.NewDocument()
	addDistinctField(t, doc, distinctGroupField, "1")
	addDistinctField(t, doc, distinctCountField, "1")
	doc.Add(newTextField(t, "content", "random text", false))
	doc.Add(newStringField(t, "id", "1", false))
	mustAddDocument(t, w, doc)

	// 1
	doc = document.NewDocument()
	addDistinctField(t, doc, distinctGroupField, "1")
	addDistinctField(t, doc, distinctCountField, "1")
	doc.Add(newTextField(t, "content", "some more random text blob", false))
	doc.Add(newStringField(t, "id", "2", false))
	mustAddDocument(t, w, doc)

	// 2
	doc = document.NewDocument()
	addDistinctField(t, doc, distinctGroupField, "1")
	addDistinctField(t, doc, distinctCountField, "2")
	doc.Add(newTextField(t, "content", "some more random textual data", false))
	doc.Add(newStringField(t, "id", "3", false))
	mustAddDocument(t, w, doc)
	mustCommit(t, w) // To ensure a second segment

	// 3 -- no count field
	doc = document.NewDocument()
	addDistinctField(t, doc, distinctGroupField, "2")
	doc.Add(newTextField(t, "content", "some random text", false))
	doc.Add(newStringField(t, "id", "4", false))
	mustAddDocument(t, w, doc)

	// 4
	doc = document.NewDocument()
	addDistinctField(t, doc, distinctGroupField, "3")
	addDistinctField(t, doc, distinctCountField, "1")
	doc.Add(newTextField(t, "content", "some more random text", false))
	doc.Add(newStringField(t, "id", "5", false))
	mustAddDocument(t, w, doc)

	// 5
	doc = document.NewDocument()
	addDistinctField(t, doc, distinctGroupField, "3")
	addDistinctField(t, doc, distinctCountField, "1")
	doc.Add(newTextField(t, "content", "random blob", false))
	doc.Add(newStringField(t, "id", "6", false))
	mustAddDocument(t, w, doc)

	// 6 -- no author field
	doc = document.NewDocument()
	doc.Add(newTextField(t, "content", "random word stuck in alot of other text", false))
	addDistinctField(t, doc, distinctCountField, "1")
	doc.Add(newStringField(t, "id", "6", false))
	mustAddDocument(t, w, doc)

	indexSearcher := newSearcher(t, mustGetReader(t, w))
	mustClose(t, w)

	cmp := func(groupCount1, groupCount2 *distinctGroupCount) int {
		if groupCount1.GroupValue == nil {
			if groupCount2.GroupValue == nil {
				return 0
			}
			return -1
		} else if groupCount2.GroupValue == nil {
			return 1
		}
		return compareComparable(groupCount1.GroupValue, groupCount2.GroupValue)
	}

	useValueSource := rnd.Intn(2) == 0
	groupSelectorFactory := createDistinctGroupSelectorFactory(distinctGroupField, useValueSource)
	valueSelectorFactory := createDistinctGroupSelectorFactory(distinctCountField, useValueSource)

	// === Search for content:random
	searchGroups := mustFirstPassSearch(t, indexSearcher, search.NewTermQuery(index.NewTerm("content", "random")),
		groupSelectorFactory, search.NewSort(search.FieldScore), 0, 10)
	gcs := mustDistinctSearch(t, indexSearcher, search.NewTermQuery(index.NewTerm("content", "random")),
		groupSelectorFactory, searchGroups, valueSelectorFactory)
	sort.SliceStable(gcs, func(i, j int) bool { return cmp(gcs[i], gcs[j]) < 0 })
	assertIntEquals(t, 4, len(gcs))

	compareNull(t, gcs[0].GroupValue)
	countValues := append([]any(nil), gcs[0].UniqueValues...)
	assertIntEquals(t, 1, len(countValues))
	compareDistinct(t, "1", countValues[0])

	compareDistinct(t, "1", gcs[1].GroupValue)
	countValues = append([]any(nil), gcs[1].UniqueValues...)
	sortNullComparator(countValues)
	assertIntEquals(t, 2, len(countValues))
	compareDistinct(t, "1", countValues[0])
	compareDistinct(t, "2", countValues[1])

	compareDistinct(t, "2", gcs[2].GroupValue)
	countValues = append([]any(nil), gcs[2].UniqueValues...)
	assertIntEquals(t, 1, len(countValues))
	compareNull(t, countValues[0])

	compareDistinct(t, "3", gcs[3].GroupValue)
	countValues = append([]any(nil), gcs[3].UniqueValues...)
	assertIntEquals(t, 1, len(countValues))
	compareDistinct(t, "1", countValues[0])

	// === Search for content:some
	searchGroups = mustFirstPassSearch(t, indexSearcher, search.NewTermQuery(index.NewTerm("content", "some")),
		groupSelectorFactory, search.NewSort(search.FieldScore), 0, 10)
	gcs = mustDistinctSearch(t, indexSearcher, search.NewTermQuery(index.NewTerm("content", "some")),
		groupSelectorFactory, searchGroups, valueSelectorFactory)
	sort.SliceStable(gcs, func(i, j int) bool { return cmp(gcs[i], gcs[j]) < 0 })
	assertIntEquals(t, 3, len(gcs))

	compareDistinct(t, "1", gcs[0].GroupValue)
	countValues = append([]any(nil), gcs[0].UniqueValues...)
	assertIntEquals(t, 2, len(countValues))
	sortNullComparator(countValues)
	compareDistinct(t, "1", countValues[0])
	compareDistinct(t, "2", countValues[1])

	compareDistinct(t, "2", gcs[1].GroupValue)
	countValues = append([]any(nil), gcs[1].UniqueValues...)
	assertIntEquals(t, 1, len(countValues))
	compareNull(t, countValues[0])

	compareDistinct(t, "3", gcs[2].GroupValue)
	countValues = append([]any(nil), gcs[2].UniqueValues...)
	assertIntEquals(t, 1, len(countValues))
	compareDistinct(t, "1", countValues[0])

	// === Search for content:blob
	searchGroups = mustFirstPassSearch(t, indexSearcher, search.NewTermQuery(index.NewTerm("content", "blob")),
		groupSelectorFactory, search.NewSort(search.FieldScore), 0, 10)
	gcs = mustDistinctSearch(t, indexSearcher, search.NewTermQuery(index.NewTerm("content", "blob")),
		groupSelectorFactory, searchGroups, valueSelectorFactory)
	sort.SliceStable(gcs, func(i, j int) bool { return cmp(gcs[i], gcs[j]) < 0 })
	assertIntEquals(t, 2, len(gcs))

	compareDistinct(t, "1", gcs[0].GroupValue)
	countValues = append([]any(nil), gcs[0].UniqueValues...)
	// B/c the only one document matched with blob inside the author 1 group
	assertIntEquals(t, 1, len(countValues))
	compareDistinct(t, "1", countValues[0])

	compareDistinct(t, "3", gcs[1].GroupValue)
	countValues = append([]any(nil), gcs[1].UniqueValues...)
	assertIntEquals(t, 1, len(countValues))
	compareDistinct(t, "1", countValues[0])

	mustClose(t, indexSearcher.GetIndexReader(), dir)
}

func TestDistinctValuesCollectorRandom(t *testing.T) {
	rnd := random()
	numberOfRuns := atLeast(1)
	for indexIter := 0; indexIter < numberOfRuns; indexIter++ {
		context := createDistinctIndexContext(t)
		for searchIter := 0; searchIter < 100; searchIter++ {
			searcher := newSearcher(t, context.indexReader)
			term := context.contentStrings[rnd.Intn(len(context.contentStrings))]
			groupSort := search.NewSort(search.NewSortField("id", spi.SortFieldTypeString))
			topN := 1 + rnd.Intn(10)

			expectedResult := createDistinctExpectedResult(context, term, groupSort, topN)

			useValueSource := rnd.Intn(2) == 0
			groupSelectorFactory := createDistinctGroupSelectorFactory(distinctGroupField, useValueSource)
			valueSelectorFactory := createDistinctGroupSelectorFactory(distinctCountField, useValueSource)

			searchGroups := mustFirstPassSearch(t, searcher, search.NewTermQuery(index.NewTerm("content", term)),
				groupSelectorFactory, groupSort, 0, topN)

			if len(searchGroups) == 0 {
				assertIntEquals(t, 0, len(expectedResult))
				continue
			}

			actualResult := mustDistinctSearch(t, searcher, search.NewTermQuery(index.NewTerm("content", term)),
				groupSelectorFactory, searchGroups, valueSelectorFactory)

			if verbose {
				fmt.Println("Index iter=" + strconv.Itoa(indexIter))
				fmt.Println("Search iter=" + strconv.Itoa(searchIter))
				fmt.Printf("useValueSource=%v\n", useValueSource)
				fmt.Println("Search term=" + term)
				fmt.Printf("1st pass groups=%v\n", searchGroups)
				fmt.Println("Expected:")
				printDistinctGroups(expectedResult)
				fmt.Println("Actual:")
				printDistinctGroups(actualResult)
			}

			assertIntEquals(t, len(expectedResult), len(actualResult))
			for i := range expectedResult {
				expected := expectedResult[i]
				actual := actualResult[i]
				assertDistinctValues(t, expected.GroupValue, actual.GroupValue)
				assertIntEquals(t, len(expected.UniqueValues), len(actual.UniqueValues))
				expectedUniqueValues := append([]any(nil), expected.UniqueValues...)
				sortNullComparator(expectedUniqueValues)
				actualUniqueValues := append([]any(nil), actual.UniqueValues...)
				sortNullComparator(actualUniqueValues)
				for j := range expectedUniqueValues {
					assertDistinctValues(t, expectedUniqueValues[j], actualUniqueValues[j])
				}
			}
		}
		mustClose(t, context.indexReader, context.directory)
	}
}

func printDistinctGroups(results []*distinctGroupCount) {
	for i, group := range results {
		gv := group.GroupValue
		if b, ok := gv.(*util.BytesRef); ok {
			fmt.Printf("%d: groupValue=%s\n", i, b.Utf8ToString())
		} else {
			fmt.Printf("%d: groupValue=%v\n", i, gv)
		}
		for _, o := range group.UniqueValues {
			if b, ok := o.(*util.BytesRef); ok {
				fmt.Println("  " + b.Utf8ToString())
			} else {
				fmt.Printf("  %v\n", o)
			}
		}
	}
}

func assertIntEquals(t testing.TB, expected, actual int) {
	t.Helper()
	if expected != actual {
		t.Fatalf("expected %d, got %d", expected, actual)
	}
}

func assertDistinctValues(t testing.TB, expected, actual any) {
	t.Helper()
	if expected == nil {
		compareNull(t, actual)
	} else {
		compareDistinct(t, expected.(*util.BytesRef).Utf8ToString(), actual)
	}
}

// compareDistinct renders the private compare(String, Object).
func compareDistinct(t testing.TB, expected string, groupValue any) {
	t.Helper()
	switch v := groupValue.(type) {
	case *util.BytesRef:
		if expected != v.Utf8ToString() {
			t.Fatalf("expected %q, got %q", expected, v.Utf8ToString())
		}
	case float64:
		d, err := strconv.ParseFloat(expected, 64)
		if err != nil || d != v {
			t.Fatalf("expected %q, got %v", expected, v)
		}
	case int64:
		l, err := strconv.ParseInt(expected, 10, 64)
		if err != nil || l != v {
			t.Fatalf("expected %q, got %v", expected, v)
		}
	case mutable.MutableValue:
		mutableValue := mutable.NewMutableValueStr()
		mutableValue.Value = expected
		if !javaEquals(mutableValue, v) {
			t.Fatalf("expected %q, got %v", expected, v)
		}
	default:
		t.Fatalf("unexpected value %T", groupValue)
	}
}

// compareNull renders the private compareNull(Object).
func compareNull(t testing.TB, groupValue any) {
	t.Helper()
	if groupValue == nil {
		return // term based impl...
	}
	// DV based impls..
	switch v := groupValue.(type) {
	case *util.BytesRef:
		if v.Utf8ToString() != "" {
			t.Fatalf("expected \"\", got %q", v.Utf8ToString())
		}
	case float64:
		if v != 0.0 {
			t.Fatalf("expected 0.0, got %v", v)
		}
	case int64:
		if v != 0 {
			t.Fatalf("expected 0, got %v", v)
		}
	// Function based impl
	case mutable.MutableValue:
		if v.Exists() {
			t.Fatalf("expected a missing value, got %v", v)
		}
	default:
		t.Fatalf("unexpected value %T", groupValue)
	}
}

func addDistinctField(t testing.TB, doc *document.Document, field, value string) {
	t.Helper()
	doc.Add(mustSortedDVField(t, field, value))
}

// createDistinctGroupSelectorFactory renders the private static
// createGroupSelectorFactory(String, boolean) and its unchecked
// (GroupSelector<T>) casts.
func createDistinctGroupSelectorFactory(field string, useValueSource bool) func() GroupSelector[any] {
	if useValueSource {
		return func() GroupSelector[any] {
			return eraseGroupSelector[mutable.MutableValue](
				NewValueSourceGroupSelector(valuesource.NewBytesRefFieldSource(field), function.Context{}))
		}
	}
	return func() GroupSelector[any] {
		return eraseGroupSelector[*util.BytesRef](NewTermGroupSelector(field))
	}
}

// createDistinctExpectedResult renders the private createExpectedResult(
// IndexContext, String, Sort, int).
func createDistinctExpectedResult(context *distinctIndexContext, term string, groupSort *search.Sort, topN int) []*distinctGroupCount {
	result := make([]*distinctGroupCount, 0)
	groupCounts := context.searchTermToGroupCounts[term]
	i := 0
	for _, group := range groupCounts.keys {
		if topN <= i {
			break
		}
		i++
		uniqueValues := make([]any, 0)
		seen := map[any]struct{}{}
		for _, val := range groupCounts.values[group] {
			var v any
			if val != nil {
				v = util.NewBytesRef([]byte(*val))
			}
			if _, dup := seen[groupKey(v)]; dup {
				continue
			}
			seen[groupKey(v)] = struct{}{}
			uniqueValues = append(uniqueValues, v)
		}
		var groupValue any
		if group != nil {
			groupValue = util.NewBytesRef([]byte(*group))
		}
		result = append(result, NewDistinctValuesGroupCount[any, any](groupValue, uniqueValues))
	}
	return result
}

// linkedGroupCounts renders LinkedHashMap<String, Set<String>> with nullable
// keys and values: keys in insertion order.
type linkedGroupCounts struct {
	keys   []*string
	values map[*string][]*string
	index  map[string]*string
}

func newLinkedGroupCounts() *linkedGroupCounts {
	return &linkedGroupCounts{values: map[*string][]*string{}, index: map[string]*string{}}
}

// key interns the nullable group value so equal strings share one map key.
func (m *linkedGroupCounts) key(group *string) (*string, bool) {
	if group == nil {
		for _, k := range m.keys {
			if k == nil {
				return nil, true
			}
		}
		return nil, false
	}
	k, ok := m.index[*group]
	return k, ok
}

func (m *linkedGroupCounts) add(group, count *string) {
	k, ok := m.key(group)
	if !ok {
		k = group
		if group != nil {
			m.index[*group] = group
		}
		m.keys = append(m.keys, k)
	}
	for _, c := range m.values[k] {
		if (c == nil && count == nil) || (c != nil && count != nil && *c == *count) {
			return
		}
	}
	m.values[k] = append(m.values[k], count)
}

func createDistinctIndexContext(t *testing.T) *distinctIndexContext {
	t.Helper()
	rnd := random()

	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newLogMergePolicy())
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	numDocs := 86 + rnd.Intn(1087)*1 // RANDOM_MULTIPLIER = 1
	groupValues := make([]string, numDocs/5)
	countValues := make([]string, numDocs/10)
	for i := range groupValues {
		groupValues[i] = generateRandomNonEmptyString()
	}
	for i := range countValues {
		countValues[i] = generateRandomNonEmptyString()
	}

	contentStrings := make([]string, 0)
	searchTermToGroupCounts := map[string]*linkedGroupCounts{}
	for i := 1; i <= numDocs; i++ {
		var groupValue *string
		if rnd.Intn(23) != 14 {
			v := groupValues[rnd.Intn(len(groupValues))]
			groupValue = &v
		}
		var countValue *string
		if rnd.Intn(21) != 13 {
			v := countValues[rnd.Intn(len(countValues))]
			countValue = &v
		}
		content := "random" + strconv.Itoa(rnd.Intn(numDocs/20))
		groupToCounts, ok := searchTermToGroupCounts[content]
		if !ok {
			// Groups sort always DOCID asc...
			groupToCounts = newLinkedGroupCounts()
			searchTermToGroupCounts[content] = groupToCounts
			contentStrings = append(contentStrings, content)
		}

		groupToCounts.add(groupValue, countValue)

		doc := document.NewDocument()
		id := fmt.Sprintf("%09d", i)
		doc.Add(newStringField(t, "id", id, true))
		doc.Add(mustSortedDVField(t, "id", id))
		if groupValue != nil {
			addDistinctField(t, doc, distinctGroupField, *groupValue)
		}
		if countValue != nil {
			addDistinctField(t, doc, distinctCountField, *countValue)
		}
		doc.Add(newTextField(t, "content", content, true))
		mustAddDocument(t, w, doc)
	}

	reader := mustGetReader(t, w)
	if verbose {
		for docID := 0; docID < reader.MaxDoc(); docID++ {
			doc := storedDocument(t, mustStoredFields(t, reader), docID)
			fmt.Printf("docID=%d id=%v content=%v author=%v publisher=%v\n", docID,
				doc.Get("id"), doc.Get("content"), doc.Get("author"), doc.Get("publisher"))
		}
	}

	mustClose(t, w)
	return &distinctIndexContext{
		directory:               dir,
		indexReader:             reader,
		searchTermToGroupCounts: searchTermToGroupCounts,
		contentStrings:          contentStrings,
	}
}

// distinctIndexContext renders the private record IndexContext.
type distinctIndexContext struct {
	directory               store.Directory
	indexReader             *index.DirectoryReader
	searchTermToGroupCounts map[string]*linkedGroupCounts
	contentStrings          []string
}

// sortNullComparator renders Collections.sort(list, new NullComparator()):
// null first, then Comparable.compareTo.
func sortNullComparator(values []any) {
	sort.SliceStable(values, func(i, j int) bool {
		a, b := values[i], values[j]
		switch {
		case a == nil && b == nil:
			return false
		case a == nil:
			return true
		case b == nil:
			return false
		default:
			return compareComparable(a, b) < 0
		}
	})
}

// compareComparable renders Comparable.compareTo over the group and value
// types these tests produce: BytesRef and MutableValue.
func compareComparable(a, b any) int {
	switch x := a.(type) {
	case *util.BytesRef:
		return x.BytesRefCompareTo(b.(*util.BytesRef))
	case mutable.MutableValue:
		return mutable.CompareTo(x, b.(mutable.MutableValue))
	}
	panic(fmt.Sprintf("not Comparable: %T", a))
}

func mustFirstPassSearch(t testing.TB, searcher *search.IndexSearcher, q search.Query,
	factory func() GroupSelector[any], groupSort *search.Sort, groupOffset, topN int) []*SearchGroup[any] {
	t.Helper()
	fcm, err := NewFirstPassGroupingCollectorManager(factory, groupSort, groupOffset, topN)
	if err != nil {
		t.Fatal(err)
	}
	groups, err := search.SearchWithCollectorManager[*FirstPassGroupingCollector[any], []*SearchGroup[any]](searcher, q, fcm)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	return groups
}

func mustDistinctSearch(t testing.TB, searcher *search.IndexSearcher, q search.Query,
	groupSelectorFactory func() GroupSelector[any], searchGroups []*SearchGroup[any],
	valueSelectorFactory func() GroupSelector[any]) []*distinctGroupCount {
	t.Helper()
	m := NewDistinctValuesCollectorManager(groupSelectorFactory, searchGroups, valueSelectorFactory)
	gcs, err := search.SearchWithCollectorManager[*DistinctValuesCollector[any, any], []*distinctGroupCount](searcher, q, m)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	return gcs
}

// erasedGroupSelector renders the unchecked cast of a GroupSelector<T> to the
// raw GroupSelector<Comparable<Object>>: values pass through as Object, with a
// null group value staying null.
type erasedGroupSelector[T any] struct {
	in GroupSelector[T]
}

func eraseGroupSelector[T any](in GroupSelector[T]) GroupSelector[any] {
	return &erasedGroupSelector[T]{in: in}
}

func (s *erasedGroupSelector[T]) SetNextReader(ctx *index.LeafReaderContext) error {
	return s.in.SetNextReader(ctx)
}

func (s *erasedGroupSelector[T]) SetScorer(scorer search.Scorable) error {
	return s.in.SetScorer(scorer)
}

func (s *erasedGroupSelector[T]) AdvanceTo(doc int) (GroupSelectorState, error) {
	return s.in.AdvanceTo(doc)
}

func (s *erasedGroupSelector[T]) CurrentValue() (any, error) {
	v, err := s.in.CurrentValue()
	return eraseNil(v), err
}

func (s *erasedGroupSelector[T]) CopyValue() (any, error) {
	v, err := s.in.CopyValue()
	return eraseNil(v), err
}

func (s *erasedGroupSelector[T]) SetGroups(groups []*SearchGroup[any]) {
	typed := make([]*SearchGroup[T], len(groups))
	for i, g := range groups {
		var groupValue T
		if g.GroupValue != nil {
			groupValue = g.GroupValue.(T)
		}
		typed[i] = &SearchGroup[T]{GroupValue: groupValue, SortValues: g.SortValues}
	}
	s.in.SetGroups(typed)
}

// eraseNil turns a typed nil group value into Java's null Object.
func eraseNil(v any) any {
	switch x := v.(type) {
	case *util.BytesRef:
		if x == nil {
			return nil
		}
	case mutable.MutableValue:
		if x == nil {
			return nil
		}
	}
	return v
}
