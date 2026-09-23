// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestConjunctions.java
// (Apache Lucene 10.5.0).

package search_test

import (
	"math"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

const (
	conjunctionsF1 = "title"
	conjunctionsF2 = "body"
)

// conjunctionsSetUp renders setUp(); the returned function renders tearDown().
func conjunctionsSetUp(t *testing.T) (*search.IndexSearcher, func()) {
	t.Helper()
	analyzer := testanalysis.NewMockAnalyzerRandom(random())
	dir := newDirectory()
	config := newIndexWriterConfigWithAnalyzer(analyzer)
	config.SetMergePolicy(newLogMergePolicy()) // we will use docids to validate
	writer := newRandomIndexWriterWithConfig(t, dir, config)
	mustAddDocument(t, writer, conjunctionsDoc(t, "lucene", "lucene is a very popular search engine library"))
	mustAddDocument(t, writer, conjunctionsDoc(t, "solr", "solr is a very popular search server and is using lucene"))
	mustAddDocument(t, writer, conjunctionsDoc(t,
		"nutch",
		"nutch is an internet search engine with web crawler and is using lucene and hadoop"))
	reader := mustGetReader(t, writer)
	mustClose(t, writer)
	tearDown := func() {
		mustClose(t, reader, dir)
	}
	searcher := newSearcher(t, reader)
	searcher.SetSimilarity(search.NewRawTFSimilarity())
	return searcher, tearDown
}

// conjunctionsDoc renders the static doc(String, String).
func conjunctionsDoc(t *testing.T, v1, v2 string) *document.Document {
	t.Helper()
	doc := document.NewDocument()
	doc.Add(mustStringField(t, conjunctionsF1, v1, true))
	f2, err := document.NewTextField(conjunctionsF2, v2, true)
	if err != nil {
		t.Fatalf("new TextField: %v", err)
	}
	doc.Add(f2)
	return doc
}

func TestConjunctionsTermConjunctionsWithOmitTF(t *testing.T) {
	searcher, tearDown := conjunctionsSetUp(t)
	defer tearDown()
	bq := search.NewBooleanQueryBuilder()
	bq.Add(search.NewTermQuery(index.NewTerm(conjunctionsF1, "nutch")), search.MUST)
	bq.Add(search.NewTermQuery(index.NewTerm(conjunctionsF2, "is")), search.MUST)
	td := mustSearch(t, searcher, bq.Build(), 3)
	if td.TotalHits.Value != 1 {
		t.Fatalf("totalHits = %d, want 1", td.TotalHits.Value)
	}
	if math.Abs(float64(3-td.ScoreDocs[0].Score)) > 0.001 { // f1:nutch + f2:is + f2:is
		t.Fatalf("score = %v, want 3", td.ScoreDocs[0].Score)
	}
}

func TestConjunctionsScorerGetChildren(t *testing.T) {
	// setUp() runs before every test method of the class.
	_, tearDown := conjunctionsSetUp(t)
	defer tearDown()

	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	doc := document.NewDocument()
	doc.Add(newTextField(t, "field", "a b", false))
	mustAddDocument(t, w, doc)
	r := mustOpenDirectoryReaderFromWriter(t, w)
	b := search.NewBooleanQueryBuilder()
	b.Add(search.NewTermQuery(index.NewTerm("field", "a")), search.MUST)
	b.Add(search.NewTermQuery(index.NewTerm("field", "b")), search.FILTER)
	q := b.Build()
	s := search.NewIndexSearcher(r)
	if _, err := search.SearchWithCollectorManager[*conjunctionsTestCollector, struct{}](s, q, &conjunctionsTestCollectorManager{t: t}); err != nil {
		t.Fatalf("search: %v", err)
	}
	if err := util.CloseAll(r, w, dir); err != nil {
		t.Fatal(err)
	}
}

// conjunctionsTestCollectorManager renders the anonymous
// CollectorManager<TestCollector, Void> of testScorerGetChildren.
type conjunctionsTestCollectorManager struct {
	t *testing.T
}

func (m *conjunctionsTestCollectorManager) NewCollector() (*conjunctionsTestCollector, error) {
	c := &conjunctionsTestCollector{t: m.t}
	c.Outer = c
	return c, nil
}

func (m *conjunctionsTestCollectorManager) Reduce(collectors []*conjunctionsTestCollector) (struct{}, error) {
	for _, collector := range collectors {
		if !collector.setScorerCalled.Load() {
			m.t.Error("assertTrue(collector.setScorerCalled.get())")
		}
	}
	return struct{}{}, nil
}

// conjunctionsTestCollector renders the private static class TestCollector.
type conjunctionsTestCollector struct {
	search.BaseSimpleCollector
	search.BaseLeafCollector
	t               *testing.T
	setScorerCalled atomic.Bool
}

// SetWeight renders setWeight(Weight).
func (c *conjunctionsTestCollector) SetWeight(weight search.Weight) {
	t := c.t
	query := weight.GetQuery().(*search.BooleanQuery)
	clauseList := query.Clauses()
	if len(clauseList) != 2 {
		t.Errorf("clauses: expected 2, got %d", len(clauseList))
	}
	terms := map[string]struct{}{}
	for _, clause := range clauseList {
		tq, ok := clause.Query().(*search.TermQuery)
		if !ok {
			panic(util.NewAssertionError(nil))
		}
		term := tq.GetTerm()
		if term.Field != "field" {
			t.Errorf("field: expected field, got %s", term.Field)
		}
		terms[term.Text()] = struct{}{}
	}
	if len(terms) != 2 {
		t.Errorf("terms: expected 2, got %d", len(terms))
	}
	if _, ok := terms["a"]; !ok {
		t.Error("assertTrue(terms.contains(\"a\"))")
	}
	if _, ok := terms["b"]; !ok {
		t.Error("assertTrue(terms.contains(\"b\"))")
	}
}

// SetScorer renders setScorer(Scorable).
func (c *conjunctionsTestCollector) SetScorer(s search.Scorable) error {
	childScorers, err := s.GetChildren()
	if err != nil {
		return err
	}
	c.setScorerCalled.Store(true)
	if len(childScorers) != 2 {
		c.t.Errorf("childScorers: expected 2, got %d", len(childScorers))
	}
	return nil
}

func (c *conjunctionsTestCollector) Collect(doc int) error { return nil }

func (c *conjunctionsTestCollector) ScoreMode() search.ScoreMode { return search.COMPLETE }

func (c *conjunctionsTestCollector) GetLeafCollector(context *index.LeafReaderContext) (search.LeafCollector, error) {
	if err := c.DoSetNextReader(context); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *conjunctionsTestCollector) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

func (c *conjunctionsTestCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

func (c *conjunctionsTestCollector) CompetitiveIterator() (search.DocIdSetIterator, error) {
	return c.BaseLeafCollector.CompetitiveIterator()
}

func (c *conjunctionsTestCollector) Finish() error { return c.BaseLeafCollector.Finish() }
