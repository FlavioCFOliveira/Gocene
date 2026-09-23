// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Port of
// lucene/grouping/src/test/org/apache/lucene/search/grouping/TestGroupFacetCollector.java
// (Apache Lucene 10.5.0), which extends AbstractGroupingTestCase.

func mustSearchCollector(t testing.TB, s *search.IndexSearcher, q search.Query, c search.Collector) {
	t.Helper()
	if err := s.SearchWithCollector(q, c); err != nil {
		t.Fatalf("search: %v", err)
	}
}

func mustMergeSegmentResults(t testing.TB, c TermGroupFacetCollector, size, minCount int, orderByCount bool) *GroupedFacetResult {
	t.Helper()
	r, err := c.MergeSegmentResults(size, minCount, orderByCount)
	if err != nil {
		t.Fatalf("mergeSegmentResults: %v", err)
	}
	return r
}

// assertFacetEntry asserts entry value and count.
func assertFacetEntry(t testing.TB, entry *FacetEntry, value string, count int) {
	t.Helper()
	if entry.Value.Utf8ToString() != value {
		t.Fatalf("facet value: expected %q, got %q", value, entry.Value.Utf8ToString())
	}
	if entry.Count != count {
		t.Fatalf("facet %q count: expected %d, got %d", value, count, entry.Count)
	}
}

func assertCounts(t testing.TB, result *GroupedFacetResult, totalCount, totalMissingCount int) {
	t.Helper()
	if result.GetTotalCount() != totalCount {
		t.Fatalf("totalCount: expected %d, got %d", totalCount, result.GetTotalCount())
	}
	if result.GetTotalMissingCount() != totalMissingCount {
		t.Fatalf("totalMissingCount: expected %d, got %d", totalMissingCount, result.GetTotalMissingCount())
	}
}

func TestGroupFacetCollectorSimple(t *testing.T) {
	const groupField = "hotel"
	customType := document.NewFieldType()
	customType.SetStored(true)

	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newLogMergePolicy())
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	useDv := true

	addDoc := func(hotel, airport, duration string, withAirport bool) {
		doc := document.NewDocument()
		addFacetField(t, doc, groupField, hotel, useDv)
		if withAirport {
			addFacetField(t, doc, "airport", airport, useDv)
		}
		addFacetField(t, doc, "duration", duration, useDv)
		mustAddDocument(t, w, doc)
	}

	// 0
	addDoc("a", "ams", "5", true)
	// 1
	addDoc("a", "dus", "10", true)
	// 2
	addDoc("b", "ams", "10", true)
	mustCommit(t, w) // To ensure a second segment
	// 3
	addDoc("b", "ams", "5", true)
	// 4
	addDoc("b", "ams", "5", true)

	indexSearcher := newSearcher(t, mustGetReader(t, w))

	hotelField := "hotel"
	airportField := "airport"
	durationField := "duration"
	if useDv {
		hotelField, airportField, durationField = "hotel_dv", "airport_dv", "duration_dv"
	}

	for _, limit := range []int{2, 10, 100, math.MaxInt32} {
		// any of these limits is plenty for the data we have

		groupedAirportFacetCollector := createRandomFacetCollector(hotelField, airportField, nil, false)
		mustSearchCollector(t, indexSearcher, search.Instance, groupedAirportFacetCollector)
		maxOffset := 5
		size := maxOffset + limit
		if limit == math.MaxInt32 {
			size = limit
		}
		airportResult := mustMergeSegmentResults(t, groupedAirportFacetCollector, size, 0, false)

		assertCounts(t, airportResult, 3, 0)

		entries := airportResult.GetFacetEntries(maxOffset, limit)
		assertIntEquals(t, 0, len(entries))

		entries = airportResult.GetFacetEntries(0, limit)
		assertIntEquals(t, 2, len(entries))
		assertFacetEntry(t, entries[0], "ams", 2)
		assertFacetEntry(t, entries[1], "dus", 1)

		entries = airportResult.GetFacetEntries(1, limit)
		assertIntEquals(t, 1, len(entries))
		assertFacetEntry(t, entries[0], "dus", 1)
	}

	groupedDurationFacetCollector := createRandomFacetCollector(hotelField, durationField, nil, false)
	mustSearchCollector(t, indexSearcher, search.Instance, groupedDurationFacetCollector)
	durationResult := mustMergeSegmentResults(t, groupedDurationFacetCollector, 10, 0, false)
	assertCounts(t, durationResult, 4, 0)

	entries := durationResult.GetFacetEntries(0, 10)
	assertIntEquals(t, 2, len(entries))
	assertFacetEntry(t, entries[0], "10", 2)
	assertFacetEntry(t, entries[1], "5", 2)

	// 5
	// missing airport
	addDoc("b", "", "5", useDv)
	// 6
	addDoc("b", "bru", "10", true)
	// 7
	addDoc("b", "bru", "15", true)
	// 8
	addDoc("a", "bru", "10", true)

	mustClose(t, indexSearcher.GetIndexReader())
	indexSearcher = newSearcher(t, mustGetReader(t, w))
	groupedAirportFacetCollector := createRandomFacetCollector(hotelField, airportField, nil, !useDv)
	mustSearchCollector(t, indexSearcher, search.Instance, groupedAirportFacetCollector)
	airportResult := mustMergeSegmentResults(t, groupedAirportFacetCollector, 3, 0, true)
	entries = airportResult.GetFacetEntries(1, 2)
	assertIntEquals(t, 2, len(entries))
	if useDv {
		assertCounts(t, airportResult, 6, 0)
		assertFacetEntry(t, entries[0], "bru", 2)
		assertFacetEntry(t, entries[1], "", 1)
	} else {
		assertCounts(t, airportResult, 5, 1)
		assertFacetEntry(t, entries[0], "bru", 2)
		assertFacetEntry(t, entries[1], "dus", 1)
	}

	groupedDurationFacetCollector = createRandomFacetCollector(hotelField, durationField, nil, false)
	mustSearchCollector(t, indexSearcher, search.Instance, groupedDurationFacetCollector)
	durationResult = mustMergeSegmentResults(t, groupedDurationFacetCollector, 10, 2, true)
	assertCounts(t, durationResult, 5, 0)

	entries = durationResult.GetFacetEntries(1, 1)
	assertIntEquals(t, 1, len(entries))
	assertFacetEntry(t, entries[0], "5", 2)

	// 9
	addDoc("c", "bru", "15", true)
	// 10
	addDoc("c", "dus", "10", true)

	mustClose(t, indexSearcher.GetIndexReader())
	indexSearcher = newSearcher(t, mustGetReader(t, w))
	groupedAirportFacetCollector = createRandomFacetCollector(hotelField, airportField, nil, false)
	mustSearchCollector(t, indexSearcher, search.Instance, groupedAirportFacetCollector)
	airportResult = mustMergeSegmentResults(t, groupedAirportFacetCollector, 10, 0, false)
	entries = airportResult.GetFacetEntries(0, 10)
	if useDv {
		assertCounts(t, airportResult, 8, 0)
		assertIntEquals(t, 4, len(entries))
		assertFacetEntry(t, entries[0], "", 1)
		assertFacetEntry(t, entries[1], "ams", 2)
		assertFacetEntry(t, entries[2], "bru", 3)
		assertFacetEntry(t, entries[3], "dus", 2)
	} else {
		assertCounts(t, airportResult, 7, 1)
		assertIntEquals(t, 3, len(entries))
		assertFacetEntry(t, entries[0], "ams", 2)
		assertFacetEntry(t, entries[1], "bru", 3)
		assertFacetEntry(t, entries[2], "dus", 2)
	}

	prefix := "1"
	groupedDurationFacetCollector = createRandomFacetCollector(hotelField, durationField, &prefix, false)
	mustSearchCollector(t, indexSearcher, search.Instance, groupedDurationFacetCollector)
	durationResult = mustMergeSegmentResults(t, groupedDurationFacetCollector, 10, 0, true)
	assertCounts(t, durationResult, 5, 0)

	entries = durationResult.GetFacetEntries(0, 10)
	assertIntEquals(t, 2, len(entries))
	assertFacetEntry(t, entries[0], "10", 3)
	assertFacetEntry(t, entries[1], "15", 2)

	mustClose(t, w, indexSearcher.GetIndexReader(), dir)
}

func TestGroupFacetCollectorMVGroupedFacetingWithDeletes(t *testing.T) {
	const groupField = "hotel"
	customType := document.NewFieldType()
	customType.SetStored(true)

	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(index.NewNoMergePolicy())
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	useDv := true

	// Cannot assert this since we use NoMergePolicy:
	w.SetDoRandomForceMergeAssert(false)

	airport := func(value string) *document.SortedSetDocValuesField {
		f, err := document.NewSortedSetDocValuesField("airport", [][]byte{[]byte(value)})
		if err != nil {
			t.Fatal(err)
		}
		return f
	}

	// 0
	doc := document.NewDocument()
	doc.Add(newStringField(t, "x", "x", false))
	mustAddDocument(t, w, doc)

	// 1
	doc = document.NewDocument()
	addFacetField(t, doc, groupField, "a", useDv)
	doc.Add(airport("ams"))
	mustAddDocument(t, w, doc)

	mustCommit(t, w)
	if _, err := w.DeleteDocumentsQuery(search.NewTermQuery(index.NewTerm("airport", "ams"))); err != nil {
		t.Fatalf("deleteDocuments: %v", err)
	}

	// 2
	doc = document.NewDocument()
	addFacetField(t, doc, groupField, "a", useDv)
	doc.Add(airport("ams"))
	mustAddDocument(t, w, doc)

	// 3
	doc = document.NewDocument()
	addFacetField(t, doc, groupField, "a", useDv)
	doc.Add(airport("dus"))
	mustAddDocument(t, w, doc)

	// 4
	doc = document.NewDocument()
	addFacetField(t, doc, groupField, "b", useDv)
	doc.Add(airport("ams"))
	mustAddDocument(t, w, doc)

	// 5
	doc = document.NewDocument()
	addFacetField(t, doc, groupField, "b", useDv)
	doc.Add(airport("ams"))
	mustAddDocument(t, w, doc)

	// 6
	doc = document.NewDocument()
	addFacetField(t, doc, groupField, "b", useDv)
	doc.Add(airport("ams"))
	mustAddDocument(t, w, doc)
	mustCommit(t, w)

	// 7
	doc = document.NewDocument()
	doc.Add(newStringField(t, "x", "x", false))
	mustAddDocument(t, w, doc)
	mustCommit(t, w)

	mustClose(t, w)
	indexSearcher := newSearcher(t, mustOpenDirectoryReader(t, dir))
	groupedAirportFacetCollector := createRandomFacetCollector(groupField+"_dv", "airport", nil, true)
	mustSearchCollector(t, indexSearcher, search.Instance, groupedAirportFacetCollector)
	airportResult := mustMergeSegmentResults(t, groupedAirportFacetCollector, 10, 0, false)
	assertCounts(t, airportResult, 3, 1)

	entries := airportResult.GetFacetEntries(0, 10)
	assertIntEquals(t, 2, len(entries))
	assertFacetEntry(t, entries[0], "ams", 2)
	assertFacetEntry(t, entries[1], "dus", 1)

	mustClose(t, indexSearcher.GetIndexReader(), dir)
}

func addFacetField(t testing.TB, doc *document.Document, field, value string, canUseIDV bool) {
	t.Helper()
	if util.AssertsEnabled() && !canUseIDV {
		t.Fatal(util.NewAssertionError("canUseIDV"))
	}
	doc.Add(mustSortedDVField(t, field+"_dv", value))
}

func TestGroupFacetCollectorRandom(t *testing.T) {
	rnd := random()
	numberOfRuns := atLeast(1)
	for indexIter := 0; indexIter < numberOfRuns; indexIter++ {
		multipleFacetsPerDocument := rnd.Intn(2) == 0
		context := createFacetIndexContext(t, multipleFacetsPerDocument)
		searcher := newSearcher(t, context.indexReader)

		if verbose {
			fmt.Printf("TEST: searcher=%v\n", searcher)
		}

		for searchIter := 0; searchIter < 100; searchIter++ {
			if verbose {
				fmt.Printf("TEST: searchIter=%d\n", searchIter)
			}
			searchTerm := context.contentStrings[rnd.Intn(len(context.contentStrings))]
			limit := rnd.Intn(len(context.facetValues))
			offset := rnd.Intn(len(context.facetValues) - limit)
			size := offset + limit
			minCount := 0
			if rnd.Intn(2) != 0 {
				minCount = rnd.Intn(1 + context.facetWithMostGroups/10)
			}
			orderByCount := rnd.Intn(2) == 0
			randomStr := getFromSet(context.facetValues, rnd.Intn(len(context.facetValues)))
			var facetPrefix *string
			if randomStr != nil {
				runes := []rune(*randomStr)
				codePointLen := len(runes)
				randomLen := rnd.Intn(codePointLen)
				if codePointLen != randomLen-1 {
					if rnd.Intn(2) != 0 {
						p := string(runes[randomLen:])
						facetPrefix = &p
					}
				}
			}

			expectedFacetResult := createExpectedFacetResult(searchTerm, context, offset, limit, minCount, orderByCount, facetPrefix)
			groupFacetCollector := createRandomFacetCollector("group", "facet", facetPrefix, multipleFacetsPerDocument)
			mustSearchCollector(t, searcher, search.NewTermQuery(index.NewTerm("content", searchTerm)), groupFacetCollector)
			actualFacetResult := mustMergeSegmentResults(t, groupFacetCollector, size, minCount, orderByCount)

			expectedFacetEntries := expectedFacetResult.facetEntries
			actualFacetEntries := actualFacetResult.GetFacetEntries(offset, limit)

			if actualFacetResult.GetTotalCount() != expectedFacetResult.totalCount {
				t.Fatalf("totalCount: expected %d, got %d", expectedFacetResult.totalCount, actualFacetResult.GetTotalCount())
			}
			if actualFacetResult.GetTotalMissingCount() != expectedFacetResult.totalMissingCount {
				t.Fatalf("totalMissingCount: expected %d, got %d", expectedFacetResult.totalMissingCount, actualFacetResult.GetTotalMissingCount())
			}
			assertIntEquals(t, len(expectedFacetEntries), len(actualFacetEntries))
			for i := range expectedFacetEntries {
				expectedFacetEntry := expectedFacetEntries[i]
				actualFacetEntry := actualFacetEntries[i]
				if !javaEquals(expectedFacetEntry.Value, actualFacetEntry.Value) {
					t.Fatalf("i=%d: %s != %s", i, expectedFacetEntry.Value.Utf8ToString(), actualFacetEntry.Value.Utf8ToString())
				}
				if expectedFacetEntry.Count != actualFacetEntry.Count {
					t.Fatalf("i=%d: %d != %d", i, expectedFacetEntry.Count, actualFacetEntry.Count)
				}
			}
		}

		mustClose(t, context.indexReader, context.dir)
	}
}

// nullFacetKey stands for the null key of Java's HashMap<String, ...>; no
// facet value of this index takes it.
const nullFacetKey = "\x00<null>"

// javaStringCompare renders String.compareTo: lexicographic over UTF-16 code
// units.
func javaStringCompare(a, b string) int {
	ua := utf16.Encode([]rune(a))
	ub := utf16.Encode([]rune(b))
	n := min(len(ua), len(ub))
	for i := 0; i < n; i++ {
		if ua[i] != ub[i] {
			return int(ua[i]) - int(ub[i])
		}
	}
	return len(ua) - len(ub)
}

// sortedSetBytesValueBlocker names the production defect that keeps a reused
// SortedSetDocValuesField from being re-valued: Java's
// SortedSetDocValuesField holds one BytesRef that Field.setBytesValue
// replaces, while Gocene's holds [][]byte.
const sortedSetBytesValueBlocker = "requires SortedSetDocValuesField.setBytesValue(BytesRef): Gocene's " +
	"SortedSetDocValuesField holds [][]byte instead of one BytesRef (not ported)"

func createFacetIndexContext(t *testing.T, multipleFacetValuesPerDocument bool) *facetIndexContext {
	t.Helper()
	rnd := random()
	numDocs := nextInt(138, 1145) * 1 // RANDOM_MULTIPLIER = 1
	numGroups := nextInt(1, numDocs/4)
	numFacets := nextInt(1, numDocs/6)

	if verbose {
		fmt.Printf("TEST: numDocs=%d numGroups=%d\n", numDocs, numGroups)
	}

	groups := make([]string, 0, numGroups)
	for i := 0; i < numGroups; i++ {
		groups = append(groups, generateRandomNonEmptyString())
	}
	facetValues := make([]string, 0, numFacets)
	for i := 0; i < numFacets; i++ {
		facetValues = append(facetValues, generateRandomNonEmptyString())
	}
	contentBrs := make([]string, nextInt(2, 20))
	if verbose {
		fmt.Println("TEST: create fake content")
	}
	for contentIDX := range contentBrs {
		contentBrs[contentIDX] = generateRandomNonEmptyString()
		if verbose {
			fmt.Println("  content=" + contentBrs[contentIDX])
		}
	}

	dir := newDirectory()
	writer := newRandomIndexWriterWithConfig(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	doc := document.NewDocument()
	docNoGroup := document.NewDocument()
	docNoFacet := document.NewDocument()
	docNoGroupNoFacet := document.NewDocument()
	group := newStringField(t, "group", "", false)
	groupDc := mustSortedDVField(t, "group", "")
	doc.Add(groupDc)
	docNoFacet.Add(groupDc)
	doc.Add(group)
	docNoFacet.Add(group)
	var svFacetString *document.StringField
	var svFacetDV *document.SortedDocValuesField
	var mvFacetFields []*document.SortedSetDocValuesField
	if !multipleFacetValuesPerDocument {
		svFacetString = newStringField(t, "facet", "", false)
		doc.Add(svFacetString)
		docNoGroup.Add(svFacetString)
		svFacetDV = mustSortedDVField(t, "facet", "")
		doc.Add(svFacetDV)
		docNoGroup.Add(svFacetDV)
	} else {
		n := 1
		if multipleFacetValuesPerDocument {
			n = 2 + rnd.Intn(6)
		}
		mvFacetFields = make([]*document.SortedSetDocValuesField, n)
		for i := range mvFacetFields {
			f, err := document.NewSortedSetDocValuesField("facet", [][]byte{{}})
			if err != nil {
				t.Fatal(err)
			}
			mvFacetFields[i] = f
			doc.Add(f)
			docNoGroup.Add(f)
		}
	}
	content := newStringField(t, "content", "", false)
	doc.Add(content)
	docNoGroup.Add(content)
	docNoFacet.Add(content)
	docNoGroupNoFacet.Add(content)

	uniqueFacetValues := newJavaStringTreeSet()
	searchTermToFacetToGroups := map[string]map[string]map[string]struct{}{}
	facetWithMostGroups := 0
	for i := 0; i < numDocs; i++ {
		var groupValue string
		if rnd.Intn(24) == 17 {
			// So we test the "doc doesn't have the group'd
			// field" case:
			groupValue = ""
		} else {
			groupValue = groups[rnd.Intn(len(groups))]
		}

		contentStr := contentBrs[rnd.Intn(len(contentBrs))]
		if _, ok := searchTermToFacetToGroups[contentStr]; !ok {
			searchTermToFacetToGroups[contentStr] = map[string]map[string]struct{}{}
		}
		facetToGroups := searchTermToFacetToGroups[contentStr]

		facetVals := make([]string, 0)
		addFacetValue := func(facetValue string) {
			uniqueFacetValues.add(facetValue)
			if _, ok := facetToGroups[facetValue]; !ok {
				facetToGroups[facetValue] = map[string]struct{}{}
			}
			groupsInFacet := facetToGroups[facetValue]
			groupsInFacet[groupValue] = struct{}{}
			if len(groupsInFacet) > facetWithMostGroups {
				facetWithMostGroups = len(groupsInFacet)
			}
		}
		if !multipleFacetValuesPerDocument {
			facetValue := facetValues[rnd.Intn(len(facetValues))]
			addFacetValue(facetValue)
			svFacetString.SetStringValue(facetValue)
			svFacetDV.SetBytesValue([]byte(facetValue))
			facetVals = append(facetVals, facetValue)
		} else {
			for range mvFacetFields {
				facetValue := facetValues[rnd.Intn(len(facetValues))]
				addFacetValue(facetValue)
				// facetField.setBytesValue(new BytesRef(facetValue));
				t.Fatal(sortedSetBytesValueBlocker)
				facetVals = append(facetVals, facetValue)
			}
		}

		if verbose {
			fmt.Printf("  doc content=%s group=%s facetVals=%v\n", contentStr, groupValue, facetVals)
		}

		// groupValue renders Java's nullable String; this index never leaves it null.
		groupValueRef := &groupValue
		if groupValueRef != nil {
			groupDc.SetBytesValue([]byte(*groupValueRef))
			group.SetStringValue(*groupValueRef)
		} else {
			// TODO: not true
			// DV cannot have missing values:
			groupDc.SetBytesValue([]byte{})
		}
		content.SetStringValue(contentStr)
		if groupValueRef == nil && len(facetVals) == 0 {
			mustAddDocument(t, writer, docNoGroupNoFacet)
		} else if len(facetVals) == 0 {
			mustAddDocument(t, writer, docNoFacet)
		} else if groupValueRef == nil {
			mustAddDocument(t, writer, docNoGroup)
		} else {
			mustAddDocument(t, writer, doc)
		}
	}

	reader := mustGetReader(t, writer)
	mustClose(t, writer)

	return &facetIndexContext{
		searchTermToFacetGroups: searchTermToFacetToGroups,
		indexReader:             reader,
		numDocs:                 numDocs,
		dir:                     dir,
		facetWithMostGroups:     facetWithMostGroups,
		numGroups:               numGroups,
		contentStrings:          contentBrs,
		facetValues:             uniqueFacetValues.values,
	}
}

// javaStringTreeSet renders the TreeSet<String> of createIndexContext, ordered
// by String.compareTo (the null branch of its comparator is unused: no value
// is null).
type javaStringTreeSet struct {
	values []string
}

func newJavaStringTreeSet() *javaStringTreeSet { return &javaStringTreeSet{} }

func (s *javaStringTreeSet) add(v string) {
	i := sort.Search(len(s.values), func(i int) bool { return javaStringCompare(s.values[i], v) >= 0 })
	if i < len(s.values) && s.values[i] == v {
		return
	}
	s.values = append(s.values, "")
	copy(s.values[i+1:], s.values[i:])
	s.values[i] = v
}

// expectedGroupedFacetResult renders the private record GroupedFacetResult.
type expectedGroupedFacetResult struct {
	totalCount        int
	totalMissingCount int
	facetEntries      []*FacetEntry
}

func createExpectedFacetResult(searchTerm string, context *facetIndexContext, offset, limit, minCount int,
	orderByCount bool, facetPrefix *string) *expectedGroupedFacetResult {
	facetGroups, ok := context.searchTermToFacetGroups[searchTerm]
	if !ok {
		facetGroups = map[string]map[string]struct{}{}
	}

	totalCount := 0
	totalMissCount := 0
	var facetValues []string
	if facetPrefix != nil {
		for _, facetValue := range context.facetValues {
			if strings.HasPrefix(facetValue, *facetPrefix) {
				facetValues = append(facetValues, facetValue)
			}
		}
	} else {
		facetValues = context.facetValues
	}

	entries := make([]*FacetEntry, 0, len(facetGroups))
	// also includes facets with count 0
	for _, facetValue := range facetValues {
		groups := facetGroups[facetValue]
		count := len(groups)
		if count >= minCount {
			entries = append(entries, NewFacetEntry(util.NewBytesRef([]byte(facetValue)), count))
		}
		totalCount += count
	}

	// Only include null count when no facet prefix is specified
	if facetPrefix == nil {
		// facetGroups.get(null): the facet values of this index are never null, so the
		// null key is never present and totalMissCount stays 0.
		if groups, ok := facetGroups[nullFacetKey]; ok {
			totalMissCount = len(groups)
		}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if orderByCount {
			if cmp := b.Count - a.Count; cmp != 0 {
				return cmp < 0
			}
		}
		return a.Value.BytesRefCompareTo(b.Value) < 0
	})

	endOffset := offset + limit
	var entriesResult []*FacetEntry
	if offset >= len(entries) {
		entriesResult = []*FacetEntry{}
	} else if endOffset >= len(entries) {
		entriesResult = entries[offset:]
	} else {
		entriesResult = entries[offset:endOffset]
	}
	return &expectedGroupedFacetResult{totalCount: totalCount, totalMissingCount: totalMissCount, facetEntries: entriesResult}
}

func createRandomFacetCollector(groupField, facetField string, facetPrefix *string, multipleFacetsPerDocument bool) TermGroupFacetCollector {
	var facetPrefixBR *util.BytesRef
	if facetPrefix != nil {
		facetPrefixBR = util.NewBytesRef([]byte(*facetPrefix))
	}
	return CreateTermGroupFacetCollector(groupField, facetField, multipleFacetsPerDocument, facetPrefixBR, random().Intn(1024))
}

func getFromSet(set []string, index int) *string {
	currentIndex := 0
	for _, bytesRef := range set {
		if currentIndex == index {
			v := bytesRef
			return &v
		}
		currentIndex++
	}

	return nil
}

// facetIndexContext renders the private static class IndexContext.
type facetIndexContext struct {
	numDocs                 int
	indexReader             *index.DirectoryReader
	searchTermToFacetGroups map[string]map[string]map[string]struct{}
	facetValues             []string
	dir                     store.Directory
	facetWithMostGroups     int
	numGroups               int
	contentStrings          []string
}
