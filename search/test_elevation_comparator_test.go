// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/search/TestElevationComparator.java
//
// This test exercises a custom FieldComparatorSource (an "elevation" comparator
// that promotes selected documents to the top of a field-sorted result set). It
// builds a six-document index, runs a SHOULD(ipod) OR SHOULD(elevated ids) query
// sorted by [elevation(id), SCORE], and asserts the two elevated documents
// (ids "a" and "x", docs 0 and 3) sort first, with the remaining ipod documents
// ordered by score (reversed for the SortingReversed variant).
//
// Deviation: Gocene's index/search Similarity is not swapped (no
// IndexWriterConfig.SetSimilarity); the default similarity is used throughout.
// The assertions only constrain document ORDER (elevated first, then by score),
// which is preserved under any monotonic TF-based similarity, so the comparison
// remains faithful.
package search_test

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

func TestElevationComparator_Sorting(t *testing.T) {
	runElevationTest(t, false)
}

func TestElevationComparator_SortingReversed(t *testing.T) {
	runElevationTest(t, true)
}

// runElevationTest mirrors TestElevationComparator.runTest.
func runElevationTest(t *testing.T, reversed bool) {
	priority := map[string]int{}
	searcher, cleanup := buildElevationIndex(t)
	defer cleanup()

	// BooleanQuery: SHOULD(title:ipod) OR SHOULD(elevated id query, boost 0).
	newq := search.NewBooleanQueryBuilder()
	newq.Add(search.NewTermQuery(index.NewTerm("title", "ipod")), search.SHOULD)
	newq.Add(getElevatedQuery(priority, []string{"id", "a", "id", "x"}), search.SHOULD)

	// Sort: [elevation(id) ascending, SCORE].
	elevation := search.NewSortFieldCustom("id", &elevationComparatorSource{priority: priority}, false)
	scoreSort := &search.SortField{Type: spi.SortFieldTypeScore, Reverse: reversed}
	sort := search.NewSort(elevation, scoreSort)

	mgr, err := search.NewTopFieldCollectorManager(sort, 50, nil, 1<<30)
	if err != nil {
		t.Fatalf("NewTopFieldCollectorManager: %v", err)
	}
	collector, err := mgr.NewCollector()
	if err != nil {
		t.Fatalf("NewCollector: %v", err)
	}
	if err := searcher.SearchWithCollector(newq.Build(), collector); err != nil {
		t.Fatalf("SearchWithCollector: %v", err)
	}
	topDocs, err := mgr.Reduce([]*search.TopFieldCollector{collector})
	if err != nil {
		t.Fatalf("Reduce: %v", err)
	}

	if got := len(topDocs.ScoreDocs); got != 4 {
		t.Fatalf("nDocsReturned = %d, want 4", got)
	}

	// 0 (id a) & 3 (id x) were elevated.
	if topDocs.ScoreDocs[0].Doc != 0 {
		t.Errorf("scoreDocs[0].doc = %d, want 0", topDocs.ScoreDocs[0].Doc)
	}
	if topDocs.ScoreDocs[1].Doc != 3 {
		t.Errorf("scoreDocs[1].doc = %d, want 3", topDocs.ScoreDocs[1].Doc)
	}

	if reversed {
		if topDocs.ScoreDocs[2].Doc != 1 {
			t.Errorf("scoreDocs[2].doc = %d, want 1", topDocs.ScoreDocs[2].Doc)
		}
		if topDocs.ScoreDocs[3].Doc != 2 {
			t.Errorf("scoreDocs[3].doc = %d, want 2", topDocs.ScoreDocs[3].Doc)
		}
	} else {
		if topDocs.ScoreDocs[2].Doc != 2 {
			t.Errorf("scoreDocs[2].doc = %d, want 2", topDocs.ScoreDocs[2].Doc)
		}
		if topDocs.ScoreDocs[3].Doc != 1 {
			t.Errorf("scoreDocs[3].doc = %d, want 1", topDocs.ScoreDocs[3].Doc)
		}
	}
}

// buildElevationIndex builds the six-document corpus, each with a tokenized
// "title" and a SortedDocValuesField "id".
func buildElevationIndex(t *testing.T) (*search.IndexSearcher, func()) {
	t.Helper()
	ix := newIntegrationIndex(t)
	rows := [][2]string{
		{"a", "ipod"},
		{"b", "ipod ipod"},
		{"c", "ipod ipod ipod"},
		{"x", "boosted"},
		{"y", "boosted boosted"},
		{"z", "boosted boosted boosted"},
	}
	for _, row := range rows {
		id, title := row[0], row[1]
		doc := document.NewDocument()
		tf, err := document.NewTextField("title", title, true)
		if err != nil {
			t.Fatalf("NewTextField: %v", err)
		}
		doc.Add(tf)
		idf, err := document.NewTextField("id", id, true)
		if err != nil {
			t.Fatalf("NewTextField(id): %v", err)
		}
		doc.Add(idf)
		dv, err := document.NewSortedDocValuesField("id", []byte(id))
		if err != nil {
			t.Fatalf("NewSortedDocValuesField: %v", err)
		}
		doc.Add(dv)
		ix.addDoc(doc)
	}
	return ix.searcher()
}

// getElevatedQuery mirrors TestElevationComparator.getElevatedQuery: it builds a
// SHOULD disjunction of TermQueries over (field,value) pairs, records descending
// priorities for each value, and wraps the result in a zero-boost BoostQuery so
// the elevated clause contributes matches without scores.
func getElevatedQuery(priority map[string]int, vals []string) search.Query {
	b := search.NewBooleanQueryBuilder()
	max := (len(vals) / 2) + 5
	for i := 0; i < len(vals)-1; i += 2 {
		b.Add(search.NewTermQuery(index.NewTerm(vals[i], vals[i+1])), search.SHOULD)
		priority[vals[i+1]] = max
		max--
	}
	return search.NewBoostQuery(b.Build(), 0)
}

// elevationComparatorSource is the Go port of the anonymous
// FieldComparatorSource in TestElevationComparator. NewComparator builds a
// comparator that orders documents by their recorded elevation priority
// (descending), reading the "id" SortedDocValues per leaf.
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
