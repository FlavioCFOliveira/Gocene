// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestElevationComparator.java
// (Apache Lucene 10.5.0), including the package-private class
// ElevationComparatorSource declared in the same file.

package search_test

import (
	"errors"
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
)

// elevationComparatorTestCase renders the fields of TestElevationComparator.
type elevationComparatorTestCase struct {
	searcher *search.IndexSearcher
	// priority renders Map<BytesRef, Integer>, keyed by the BytesRef bytes.
	priority map[string]int
}

// ecSetUp renders setUp(); the returned function renders tearDown().
func ecSetUp(t *testing.T) (*elevationComparatorTestCase, func()) {
	t.Helper()
	tc := &elevationComparatorTestCase{priority: map[string]int{}}
	directory := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(testanalysis.NewMockAnalyzerRandom(random()))
	iwc.SetMaxBufferedDocs(2)
	mp := newLogMergePolicy()
	mp.SetMergeFactor(1000) // newLogMergePolicy(1000)
	iwc.SetMergePolicy(mp)
	iwc.SetSimilarity(search.NewClassicSimilarity())
	writer := mustNewIndexWriter(t, directory, iwc)
	mustAddDocument(t, writer, tc.adoc(t, []string{"id", "a", "title", "ipod", "str_s", "a"}))
	mustAddDocument(t, writer, tc.adoc(t, []string{"id", "b", "title", "ipod ipod", "str_s", "b"}))
	mustAddDocument(t, writer, tc.adoc(t, []string{"id", "c", "title", "ipod ipod ipod", "str_s", "c"}))
	mustAddDocument(t, writer, tc.adoc(t, []string{"id", "x", "title", "boosted", "str_s", "x"}))
	mustAddDocument(t, writer, tc.adoc(t, []string{"id", "y", "title", "boosted boosted", "str_s", "y"}))
	mustAddDocument(t, writer, tc.adoc(t, []string{"id", "z", "title", "boosted boosted boosted", "str_s", "z"}))

	reader := mustOpenDirectoryReaderFromWriter(t, writer)
	mustClose(t, writer)

	tearDown := func() {
		mustClose(t, reader, directory)
	}
	tc.searcher = newSearcher(t, reader)
	tc.searcher.SetSimilarity(search.NewLuceneBM25Similarity())
	return tc, tearDown
}

func TestElevationComparatorSorting(t *testing.T) {
	tc, tearDown := ecSetUp(t)
	defer tearDown()
	tc.runTest(t, false)
}

func TestElevationComparatorSortingReversed(t *testing.T) {
	tc, tearDown := ecSetUp(t)
	defer tearDown()
	tc.runTest(t, true)
}

// runTest renders the private runTest(boolean).
func (tc *elevationComparatorTestCase) runTest(t *testing.T, reversed bool) {
	t.Helper()
	newq := search.NewBooleanQueryBuilder()
	query := search.NewTermQuery(index.NewTerm("title", "ipod"))

	newq.Add(query, search.SHOULD)
	newq.Add(tc.getElevatedQuery([]string{"id", "a", "id", "x"}), search.SHOULD)

	sort := search.NewSort(
		search.NewSortFieldCustom("id", newElevationComparatorSource(tc.priority), false),
		search.NewSortFieldWithReverse("", spi.SortFieldTypeScore, reversed))

	manager, err := search.NewTopFieldCollectorManager(sort, 50, nil, math.MaxInt32)
	if err != nil {
		t.Fatalf("new TopFieldCollectorManager: %v", err)
	}
	topDocs, err := search.SearchWithCollectorManager(tc.searcher, newq.Build(), manager)
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	nDocsReturned := len(topDocs.ScoreDocs)

	assertIntEquals(t, 4, nDocsReturned)

	// 0 & 3 were elevated
	assertIntEquals(t, 0, topDocs.ScoreDocs[0].Doc)
	assertIntEquals(t, 3, topDocs.ScoreDocs[1].Doc)

	if reversed {
		assertIntEquals(t, 1, topDocs.ScoreDocs[2].Doc)
		assertIntEquals(t, 2, topDocs.ScoreDocs[3].Doc)
	} else {
		assertIntEquals(t, 2, topDocs.ScoreDocs[2].Doc)
		assertIntEquals(t, 1, topDocs.ScoreDocs[3].Doc)
	}
}

// getElevatedQuery renders the private getElevatedQuery(String[]).
func (tc *elevationComparatorTestCase) getElevatedQuery(vals []string) search.Query {
	b := search.NewBooleanQueryBuilder()
	max := (len(vals) / 2) + 5
	for i := 0; i < len(vals)-1; i += 2 {
		b.Add(search.NewTermQuery(index.NewTerm(vals[i], vals[i+1])), search.SHOULD)
		tc.priority[vals[i+1]] = max
		max--
		// System.out.println(" pri doc=" + vals[i+1] + " pri=" + (1+max));
	}
	q := b.Build()
	return search.NewBoostQuery(q, 0)
}

// adoc renders the private adoc(String[]).
func (tc *elevationComparatorTestCase) adoc(t *testing.T, vals []string) *document.Document {
	t.Helper()
	doc := document.NewDocument()
	for i := 0; i < len(vals)-2; i += 2 {
		doc.Add(newTextField(t, vals[i], vals[i+1], true))
		if vals[i] == "id" {
			dv, err := document.NewSortedDocValuesField(vals[i], []byte(vals[i+1]))
			if err != nil {
				t.Fatalf("new SortedDocValuesField: %v", err)
			}
			doc.Add(dv)
		}
	}
	return doc
}

// newElevationComparatorSource renders ElevationComparatorSource(Map<BytesRef, Integer>).
func newElevationComparatorSource(boosts map[string]int) *elevationComparatorSource {
	return &elevationComparatorSource{priority: boosts}
}

// elevationComparatorSource renders the package-private class
// ElevationComparatorSource.
type elevationComparatorSource struct {
	priority map[string]int
}

func (s *elevationComparatorSource) NewComparator(fieldname string, numHits int, pruning search.Pruning, reversed bool) search.FieldComparator {
	return &elevationComparator{
		priority:  s.priority,
		fieldname: fieldname,
		values:    make([]int, numHits),
	}
}

// elevationComparator mirrors the anonymous FieldComparator<Integer> returned
// by ElevationComparatorSource.newComparator.
type elevationComparator struct {
	priority  map[string]int
	fieldname string
	values    []int
	bottomVal int
}

// GetLeafComparator mirrors the anonymous LeafFieldComparator of the upstream
// comparator, bound to context.
func (c *elevationComparator) GetLeafComparator(context *index.LeafReaderContext) (search.LeafFieldComparator, error) {
	return &elevationLeafComparator{parent: c, context: context}, nil
}

// Compare orders slots by priority descending; values are small enough that
// there is no overflow concern.
func (c *elevationComparator) Compare(slot1, slot2 int) int {
	return c.values[slot2] - c.values[slot1]
}

// CompareValues orders priorities descending; values are small enough that
// there is no overflow concern.
func (c *elevationComparator) CompareValues(first, second any) int {
	return second.(int) - first.(int)
}

// SetTopValue throws UnsupportedOperationException upstream.
func (c *elevationComparator) SetTopValue(value any) {
	panic("elevationComparator.SetTopValue: unsupported operation")
}

// Value returns the per-slot priority.
func (c *elevationComparator) Value(slot int) any { return c.values[slot] }

// SetSingleSort carries the default body Lucene gives FieldComparator.setSingleSort.
func (c *elevationComparator) SetSingleSort() {}

// DisableSkipping carries the default body Lucene gives FieldComparator.disableSkipping.
func (c *elevationComparator) DisableSkipping() {}

// elevationLeafComparator mirrors the anonymous LeafFieldComparator.
type elevationLeafComparator struct {
	parent  *elevationComparator
	context *index.LeafReaderContext
}

func (l *elevationLeafComparator) SetBottom(slot int) error {
	l.parent.bottomVal = l.parent.values[slot]
	return nil
}

// CompareTop throws UnsupportedOperationException upstream.
func (l *elevationLeafComparator) CompareTop(doc int) (int, error) {
	return 0, errors.New("elevationLeafComparator.CompareTop: unsupported operation")
}

// docVal returns the elevation priority recorded for doc's id term, or 0 when
// the document has no id value or no recorded priority.
func (l *elevationLeafComparator) docVal(doc int) (int, error) {
	idIndex, err := index.GetSorted(l.context.LeafReader(), l.parent.fieldname)
	if err != nil {
		return 0, err
	}
	advanced, err := idIndex.Advance(doc)
	if err != nil {
		return 0, err
	}
	if advanced != doc {
		return 0, nil
	}
	ord, err := idIndex.OrdValue()
	if err != nil {
		return 0, err
	}
	term, err := idIndex.LookupOrd(ord)
	if err != nil {
		return 0, err
	}
	if prio, ok := l.parent.priority[string(term)]; ok {
		return prio, nil
	}
	return 0, nil
}

func (l *elevationLeafComparator) CompareBottom(doc int) (int, error) {
	v, err := l.docVal(doc)
	if err != nil {
		return 0, err
	}
	return v - l.parent.bottomVal, nil
}

func (l *elevationLeafComparator) Copy(slot, doc int) error {
	v, err := l.docVal(doc)
	if err != nil {
		return err
	}
	l.parent.values[slot] = v
	return nil
}

func (l *elevationLeafComparator) SetScorer(scorer search.Scorable) error { return nil }

// CompetitiveIterator carries the default body Lucene gives
// LeafFieldComparator.competitiveIterator.
func (l *elevationLeafComparator) CompetitiveIterator() (search.DocIdSetIterator, error) {
	return nil, nil
}

// SetHitsThresholdReached carries the default body Lucene gives
// LeafFieldComparator.setHitsThresholdReached.
func (l *elevationLeafComparator) SetHitsThresholdReached() error { return nil }

var (
	_ search.FieldComparatorSource = (*elevationComparatorSource)(nil)
	_ search.FieldComparator       = (*elevationComparator)(nil)
	_ search.LeafFieldComparator   = (*elevationLeafComparator)(nil)
)
