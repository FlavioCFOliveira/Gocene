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
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/mutable"
)

// Port of
// lucene/grouping/src/test/org/apache/lucene/search/grouping/TestAllGroupHeadsCollector.java
// (Apache Lucene 10.5.0).

func TestAllGroupHeadsCollectorBasic(t *testing.T) {
	const groupField = "author"
	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newLogMergePolicy())
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	valueType := index.DocValuesTypeSorted

	addDoc := func(group *string, content string, id1 int64, id2 string) {
		doc := document.NewDocument()
		if group != nil {
			addGroupHeadsGroupField(t, doc, groupField, *group, valueType)
		}
		doc.Add(newTextField(t, "content", content, false))
		doc.Add(mustNumericDVField(t, "id_1", id1))
		doc.Add(mustSortedDVField(t, "id_2", id2))
		mustAddDocument(t, w, doc)
	}

	// 0
	addDoc(ptr("author1"), "random text", 1, "1")
	// 1
	addDoc(ptr("author1"), "some more random text blob", 2, "2")
	// 2
	addDoc(ptr("author1"), "some more random textual data", 3, "3")
	mustCommit(t, w) // To ensure a second segment
	// 3
	addDoc(ptr("author2"), "some random text", 4, "4")
	// 4
	addDoc(ptr("author3"), "some more random text", 5, "5")
	// 5
	addDoc(ptr("author3"), "random blob", 6, "6")
	// 6 -- no author field
	addDoc(nil, "random word stuck in alot of other text", 6, "6")
	// 7 -- no author field
	addDoc(nil, "random word stuck in alot of other text", 7, "7")

	reader := mustGetReader(t, w)
	indexSearcher := newSearcher(t, reader)

	mustClose(t, w)
	maxDoc := reader.MaxDoc()

	check := func(sortWithinGroup *search.Sort, term string, expected []int) {
		t.Helper()
		allGroupHeadsCollectorManager := createRandomGroupHeadsCollectorManager(groupField, sortWithinGroup)
		groupHeadsResult := searchGroupHeads(t, indexSearcher, search.NewTermQuery(index.NewTerm("content", term)), allGroupHeadsCollectorManager)
		if !arrayContains(expected, groupHeadsResult.RetrieveGroupHeads()) {
			t.Fatalf("content:%s group heads %v, expected %v", term, groupHeadsResult.RetrieveGroupHeads(), expected)
		}
		bits, err := groupHeadsResult.RetrieveGroupHeadsBits(maxDoc)
		if err != nil {
			t.Fatal(err)
		}
		if !openBitSetContains(t, expected, bits, maxDoc) {
			t.Fatalf("content:%s group head bits do not match %v", term, expected)
		}
	}

	sortWithinGroup := search.NewSort(search.NewSortFieldWithReverse("id_1", spi.SortFieldTypeInt, true))
	check(sortWithinGroup, "random", []int{2, 3, 5, 7})
	check(sortWithinGroup, "some", []int{2, 3, 4})
	check(sortWithinGroup, "blob", []int{1, 5})

	// STRING sort type triggers different implementation
	sortWithinGroup2 := search.NewSort(search.NewSortFieldWithReverse("id_2", spi.SortFieldTypeString, true))
	check(sortWithinGroup2, "random", []int{2, 3, 5, 7})

	sortWithinGroup3 := search.NewSort(search.NewSortFieldWithReverse("id_2", spi.SortFieldTypeString, false))
	// 7 b/c higher doc id wins, even if order of field is in not in reverse.
	check(sortWithinGroup3, "random", []int{0, 3, 4, 6})

	// query that matches no documents should return empty group heads
	allGroupHeadsCollectorManager := createRandomGroupHeadsCollectorManager(groupField, sortWithinGroup)
	groupHeadsResult := searchGroupHeads(t, indexSearcher, search.NewTermQuery(index.NewTerm("content", "nonexistent")), allGroupHeadsCollectorManager)
	if n := len(groupHeadsResult.RetrieveGroupHeads()); n != 0 {
		t.Fatalf("group heads: expected 0, got %d", n)
	}
	bits, err := groupHeadsResult.RetrieveGroupHeadsBits(maxDoc)
	if err != nil {
		t.Fatal(err)
	}
	if c := bits.(*util.FixedBitSet).Cardinality(); c != 0 {
		t.Fatalf("group head bits cardinality: expected 0, got %d", c)
	}

	mustClose(t, indexSearcher.GetIndexReader(), dir)
}

func TestAllGroupHeadsCollectorRandom(t *testing.T) {
	numberOfRuns := atLeast(1)
	for iter := 0; iter < numberOfRuns; iter++ {
		if verbose {
			fmt.Printf("TEST: iter=%d total=%d\n", iter, numberOfRuns)
		}

		numDocs := nextInt(100, 1000) * 1 // RANDOM_MULTIPLIER = 1
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
				// For that reason we don't generate empty string groups.
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
		w := newRandomIndexWriterWithConfig(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))

		doc := document.NewDocument()
		docNoGroup := document.NewDocument()
		valuesField := mustSortedDVField(t, "group", "")
		doc.Add(valuesField)
		sort1 := mustSortedDVField(t, "sort1", "")
		doc.Add(sort1)
		docNoGroup.Add(sort1)
		sort2 := mustSortedDVField(t, "sort2", "")
		doc.Add(sort2)
		docNoGroup.Add(sort2)
		sort3 := mustSortedDVField(t, "sort3", "")
		doc.Add(sort3)
		docNoGroup.Add(sort3)
		content := newTextField(t, "content", "", false)
		doc.Add(content)
		docNoGroup.Add(content)
		idDV := mustNumericDVField(t, "id", 0)
		doc.Add(idDV)
		docNoGroup.Add(idDV)
		groupDocs := make([]*groupHeadsGroupDoc, numDocs)
		for i := 0; i < numDocs; i++ {
			var groupValue *util.BytesRef
			if random().Intn(24) == 17 {
				// So we test the "doc doesn't have the group'd
				// field" case:
				groupValue = nil
			} else {
				groupValue = groups[random().Intn(len(groups))]
			}

			groupDoc := &groupHeadsGroupDoc{
				id:      i,
				group:   groupValue,
				sort1:   groups[random().Intn(len(groups))],
				sort2:   groups[random().Intn(len(groups))],
				sort3:   util.NewBytesRef([]byte(fmt.Sprintf("%05d", i))),
				content: contentStrings[random().Intn(len(contentStrings))],
			}

			if verbose {
				group := "null"
				if groupDoc.group != nil {
					group = groupDoc.group.Utf8ToString()
				}
				fmt.Printf("  doc content=%s id=%d group=%s sort1=%s sort2=%s sort3=%s\n", groupDoc.content, i,
					group, groupDoc.sort1.Utf8ToString(), groupDoc.sort2.Utf8ToString(), groupDoc.sort3.Utf8ToString())
			}

			groupDocs[i] = groupDoc
			if groupDoc.group != nil {
				valuesField.SetBytesValue([]byte(groupDoc.group.Utf8ToString()))
			}
			sort1.SetBytesValue(groupDoc.sort1.ValidBytes())
			sort2.SetBytesValue(groupDoc.sort2.ValidBytes())
			sort3.SetBytesValue(groupDoc.sort3.ValidBytes())
			content.SetStringValue(groupDoc.content)
			idDV.SetLongValue(int64(groupDoc.id))
			if groupDoc.group == nil {
				mustAddDocument(t, w, docNoGroup)
			} else {
				mustAddDocument(t, w, doc)
			}
		}

		r := mustGetReader(t, w)
		mustClose(t, w)

		values, err := index.MultiDocValuesGetNumericValues(r, "id")
		if err != nil {
			t.Fatal(err)
		}
		docIDToFieldID := make([]int, numDocs)
		fieldIDToDocID := make([]int, numDocs)
		for i := 0; i < numDocs; i++ {
			if d := mustNextDoc(t, values); d != i {
				t.Fatalf("nextDoc: expected %d, got %d", i, d)
			}
			v, err := values.LongValue()
			if err != nil {
				t.Fatal(err)
			}
			fieldID := int(v)
			docIDToFieldID[i] = fieldID
			fieldIDToDocID[fieldID] = i
		}

		s := newSearcher(t, r)

		seenIDs := map[int]struct{}{}
		for contentID := 0; contentID < 3; contentID++ {
			hits := mustSearch(t, s, search.NewTermQuery(index.NewTerm("content", "real"+strconv.Itoa(contentID))), numDocs).ScoreDocs
			for _, hit := range hits {
				idValue := docIDToFieldID[hit.Doc]
				gd := groupDocs[idValue]
				if gd.id != idValue {
					t.Fatalf("group doc id %d != %d", gd.id, idValue)
				}
				seenIDs[idValue] = struct{}{}
				if gd.score != 0.0 {
					t.Fatalf("group doc %d scored twice", gd.id)
				}
				gd.score = hit.Score
			}
		}

		// make sure all groups were seen across the hits
		if len(groupDocs) != len(seenIDs) {
			t.Fatalf("seen ids: expected %d, got %d", len(groupDocs), len(seenIDs))
		}

		// make sure scores are sane
		for _, gd := range groupDocs {
			if math.IsInf(float64(gd.score), 0) || gd.score != gd.score {
				t.Fatalf("score of doc %d is not finite: %v", gd.id, gd.score)
			}
			if !(gd.score >= 0.0) {
				t.Fatalf("score of doc %d is negative: %v", gd.id, gd.score)
			}
		}

		for searchIter := 0; searchIter < 100; searchIter++ {
			if verbose {
				fmt.Printf("TEST: searchIter=%d\n", searchIter)
			}

			searchTerm := "real" + strconv.Itoa(random().Intn(3))
			sortByScoreOnly := random().Intn(2) == 0
			sortWithinGroup := getRandomGroupHeadsSort(sortByScoreOnly)
			allGroupHeadsCollectorManager := createRandomGroupHeadsCollectorManager("group", sortWithinGroup)
			groupHeadsResult := searchGroupHeads(t, s, search.NewTermQuery(index.NewTerm("content", searchTerm)), allGroupHeadsCollectorManager)
			expectedGroupHeads := createExpectedGroupHeads(t, searchTerm, groupDocs, sortWithinGroup, sortByScoreOnly, fieldIDToDocID)
			actualGroupHeads := groupHeadsResult.RetrieveGroupHeads()
			// The actual group heads contains Lucene ids. Need to change them into our id value.
			for i := range actualGroupHeads {
				actualGroupHeads[i] = docIDToFieldID[actualGroupHeads[i]]
			}
			// Allows us the easily iterate and assert the actual and expected results.
			sort.Ints(expectedGroupHeads)
			sort.Ints(actualGroupHeads)

			if verbose {
				fmt.Printf("CollectorManager: %T\n", allGroupHeadsCollectorManager)
				fmt.Printf("Sort within group: %v\n", sortWithinGroup)
				fmt.Printf("Num group: %d\n", numGroups)
				fmt.Printf("Num doc: %d\n", numDocs)
			}

			if len(expectedGroupHeads) != len(actualGroupHeads) {
				t.Fatalf("group heads: expected %v, got %v", expectedGroupHeads, actualGroupHeads)
			}
			for i := range expectedGroupHeads {
				if expectedGroupHeads[i] != actualGroupHeads[i] {
					t.Fatalf("group heads: expected %v, got %v", expectedGroupHeads, actualGroupHeads)
				}
			}
		}

		mustClose(t, r, dir)
	}
}

func arrayContains(expected, actual []int) bool {
	// in some cases the actual docs aren't sorted by docid. This method expects that.
	sort.Ints(actual)
	if len(expected) != len(actual) {
		return false
	}

	for _, e := range expected {
		found := false
		for _, a := range actual {
			if e == a {
				found = true
				break
			}
		}

		if !found {
			return false
		}
	}

	return true
}

func openBitSetContains(t testing.TB, expectedDocs []int, actual util.Bits, maxDoc int) bool {
	t.Helper()
	actualBitSet, ok := actual.(*util.FixedBitSet)
	if util.AssertsEnabled() && !ok {
		t.Fatal(util.NewAssertionError("actual instanceof FixedBitSet"))
	}
	if len(expectedDocs) != actualBitSet.Cardinality() {
		return false
	}

	expected := mustFixedBitSet(t, maxDoc)
	for _, expectedDoc := range expectedDocs {
		expected.Set(expectedDoc)
	}

	for docID := expected.NextSetBit(0); docID != search.NO_MORE_DOCS; {
		if !actual.Get(docID) {
			return false
		}
		if docID+1 >= expected.Length() {
			docID = search.NO_MORE_DOCS
		} else {
			docID = expected.NextSetBit(docID + 1)
		}
	}

	return true
}

func mustFixedBitSet(t testing.TB, numBits int) *util.FixedBitSet {
	t.Helper()
	bs, err := util.NewFixedBitSet(numBits)
	if err != nil {
		t.Fatalf("new FixedBitSet: %v", err)
	}
	return bs
}

func createExpectedGroupHeads(t testing.TB, searchTerm string, groupDocs []*groupHeadsGroupDoc, docSort *search.Sort,
	sortByScoreOnly bool, fieldIDToDocID []int) []int {
	t.Helper()
	groupHeads := newGroupMap[*util.BytesRef, []*groupHeadsGroupDoc]()
	for _, groupDoc := range groupDocs {
		if !strings.HasPrefix(groupDoc.content, searchTerm) {
			continue
		}

		list, ok := groupHeads.get(groupDoc.group)
		if !ok {
			groupHeads.put(groupDoc.group, []*groupHeadsGroupDoc{groupDoc})
			continue
		}
		groupHeads.put(groupDoc.group, append(list, groupDoc))
	}

	allGroupHeads := make([]int, 0, groupHeads.size())
	comparator := getGroupHeadsComparator(t, docSort, sortByScoreOnly, fieldIDToDocID)
	for _, docs := range groupHeads.values() {
		sort.SliceStable(docs, func(i, j int) bool { return comparator(docs[i], docs[j]) < 0 })
		allGroupHeads = append(allGroupHeads, docs[0].id)
	}

	return allGroupHeads
}

func getRandomGroupHeadsSort(scoreOnly bool) *search.Sort {
	sortFields := make([]*search.SortField, 0)
	if random().Intn(7) == 2 || scoreOnly {
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
	if random().Intn(2) == 0 && !scoreOnly {
		sortFields = append(sortFields, search.NewSortField("sort3", spi.SortFieldTypeString))
	} else if !scoreOnly {
		sortFields = append(sortFields, search.NewSortField("id", spi.SortFieldTypeInt))
	}
	return search.NewSort(sortFields...)
}

func getGroupHeadsComparator(t testing.TB, sort *search.Sort, sortByScoreOnly bool, fieldIDToDocID []int) func(d1, d2 *groupHeadsGroupDoc) int {
	sortFields := sort.GetSort()
	return func(d1, d2 *groupHeadsGroupDoc) int {
		for _, sf := range sortFields {
			var cmp int
			if sf.Type == spi.SortFieldTypeScore {
				if d1.score > d2.score {
					cmp = -1
				} else if d1.score < d2.score {
					cmp = 1
				} else if sortByScoreOnly {
					cmp = fieldIDToDocID[d1.id] - fieldIDToDocID[d2.id]
				} else {
					cmp = 0
				}
			} else if sf.Field == "sort1" {
				cmp = d1.sort1.BytesRefCompareTo(d2.sort1)
			} else if sf.Field == "sort2" {
				cmp = d1.sort2.BytesRefCompareTo(d2.sort2)
			} else if sf.Field == "sort3" {
				cmp = d1.sort3.BytesRefCompareTo(d2.sort3)
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

// createRandomGroupHeadsCollectorManager renders the private
// createRandomCollectorManager(String, Sort), whose
// AllGroupHeadsCollectorManager<?> holds either MutableValue or BytesRef
// groups.
func createRandomGroupHeadsCollectorManager(groupField string, sortWithinGroup *search.Sort) any {
	if random().Intn(2) == 0 {
		vs := valuesource.NewBytesRefFieldSource(groupField)
		return NewAllGroupHeadsCollectorManager(func() GroupSelector[mutable.MutableValue] {
			return NewValueSourceGroupSelector(vs, function.Context{})
		}, sortWithinGroup)
	}
	return NewAllGroupHeadsCollectorManager(func() GroupSelector[*util.BytesRef] {
		return NewTermGroupSelector(groupField)
	}, sortWithinGroup)
}

// searchGroupHeads renders indexSearcher.search(query, manager) for the
// AllGroupHeadsCollectorManager<?> wildcard.
func searchGroupHeads(t testing.TB, s *search.IndexSearcher, q search.Query, manager any) *GroupHeadsResult {
	t.Helper()
	var result *GroupHeadsResult
	var err error
	switch m := manager.(type) {
	case *AllGroupHeadsCollectorManager[*util.BytesRef]:
		result, err = search.SearchWithCollectorManager[*AllGroupHeadsCollector[*util.BytesRef], *GroupHeadsResult](s, q, m)
	case *AllGroupHeadsCollectorManager[mutable.MutableValue]:
		result, err = search.SearchWithCollectorManager[*AllGroupHeadsCollector[mutable.MutableValue], *GroupHeadsResult](s, q, m)
	default:
		panic(fmt.Sprintf("unexpected manager %T", manager))
	}
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	return result
}

func addGroupHeadsGroupField(t testing.TB, doc *document.Document, groupField, value string, valueType index.DocValuesType) {
	t.Helper()
	switch valueType {
	case index.DocValuesTypeBinary:
		f, err := document.NewBinaryDocValuesField(groupField, []byte(value))
		if err != nil {
			t.Fatal(err)
		}
		doc.Add(f)
	case index.DocValuesTypeSorted:
		doc.Add(mustSortedDVField(t, groupField, value))
	default:
		t.Fatal("unhandled type")
	}
}

// groupHeadsGroupDoc renders the private static class GroupDoc.
type groupHeadsGroupDoc struct {
	id    int
	group *util.BytesRef
	sort1 *util.BytesRef
	sort2 *util.BytesRef
	sort3 *util.BytesRef
	// content must be "realN ..."
	content string
	score   float32
}
