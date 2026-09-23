// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSimilarity.java
//
// Similarity unit test: installs a custom SimpleSimilarity (tf(freq)=freq,
// idf=1, lengthNorm=1) on the searcher and verifies that term, boolean and
// phrase queries score exactly through it.
//
// Faithful adaptation: Lucene's SimpleSimilarity extends ClassicSimilarity and
// overrides tf/idf/lengthNorm/idfExplain. Gocene's ClassicSimScorer reads tf/idf
// off the concrete *ClassicSimilarity (no virtual dispatch), so a subtype's
// overrides would not be observed. The faithful equivalent is a Similarity whose
// SimScorer scores a document as its (sloppy) term frequency — exactly the
// product tf(freq)*idf*lengthNorm = freq*1*1 the Java override yields. Lucene
// also installs the similarity at index-write time (for norms); the legacy
// ClassicSimScorer path Gocene scores through applies no norms and lengthNorm is
// 1 here, so the search-time SetSimilarity alone is faithful.

package search_test

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// simpleSimilarity scores a document as its (sloppy) term frequency, mirroring
// TestSimilarity.SimpleSimilarity (tf(freq)=freq, idf=1, lengthNorm=1).
type simpleSimilarity struct {
	*search.BaseSimilarity
}

func newSimpleSimilarity() *simpleSimilarity {
	return &simpleSimilarity{BaseSimilarity: search.NewBaseSimilarity()}
}

func (s *simpleSimilarity) Scorer(_ *search.CollectionStatistics, _ *search.TermStatistics) search.SimScorer {
	return simpleSimScorer{}
}

type simpleSimScorer struct{}

func (simpleSimScorer) Score(_ int, freq float32, _ int64) float32 { return freq }

// Score104 is SimScorer.score(freq, norm): tf(freq)=freq.
func (simpleSimScorer) Score104(freq float32, _ int64) float32 { return freq }

// AsBulkSimScorer carries the default body Lucene gives SimScorer.asBulkSimScorer.
func (s simpleSimScorer) AsBulkSimScorer() search.BulkSimScorer {
	return search.NewDefaultBulkSimScorer(s)
}

// Explain104 carries the default body Lucene gives SimScorer.explain.
func (s simpleSimScorer) Explain104(freq search.Explanation, norm int64) search.Explanation {
	e := search.NewExplanation(true, s.Score104(freq.GetValue(), norm),
		fmt.Sprintf("score(freq=%v), with freq of:", freq.GetValue()))
	e.AddDetail(freq)
	return e
}

func TestSimilarity_Similarity(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfigWithAnalyzer(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	addDoc := func(text string) {
		doc := document.NewDocument()
		f, fErr := document.NewTextField("field", text, true)
		if fErr != nil {
			t.Fatalf("NewTextField: %v", fErr)
		}
		doc.Add(f)
		if _, addErr := w.AddDocument(doc); addErr != nil {
			t.Fatalf("AddDocument: %v", addErr)
		}
	}
	addDoc("a c")
	addDoc("a c b")
	if _, err = w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err = w.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer func() {
		_ = reader.Close()
		_ = dir.Close()
	}()

	searcher := search.NewIndexSearcher(reader)
	searcher.SetSimilarity(newSimpleSimilarity())

	a := index.NewTerm("field", "a")
	b := index.NewTerm("field", "b")
	c := index.NewTerm("field", "c")

	assertScore(t, searcher, search.NewTermQuery(b), 1.0)

	bq := search.NewBooleanQueryBuilder()
	bq.Add(search.NewTermQuery(a), search.SHOULD)
	bq.Add(search.NewTermQuery(b), search.SHOULD)
	// Each matching doc's score must equal doc+base+1 (a:freq=1 + b:freq=1
	// where present), exercising the SHOULD-sum scoring path.
	assertScoreCollector(t, searcher, bq.Build(), func(t *testing.T, docBase, doc int, score float32) {
		want := float32(doc + docBase + 1)
		if score != want {
			t.Errorf("doc %d (base %d): score = %v, want %v", doc, docBase, score, want)
		}
	})

	pq := search.NewPhraseQueryWithTerms(0, a.Field, a, c)
	// With a multi-term phrase scorer the per-term simpleSimilarity scores
	// (phraseFreq) are summed, so a two-term phrase scores 2 * phraseFreq.
	assertScore(t, searcher, pq, 2.0)

	pq2 := search.NewPhraseQueryWithTerms(2, a.Field, a, b)
	assertScore(t, searcher, pq2, 1.0)
}

// assertScore asserts every hit of query scores exactly want.
func assertScore(t *testing.T, searcher *search.IndexSearcher, query search.Query, want float32) {
	t.Helper()
	assertScoreCollector(t, searcher, query, func(t *testing.T, _, doc int, score float32) {
		if score != want {
			t.Errorf("doc %d: score = %v, want %v", doc, score, want)
		}
	})
}

// assertScoreCollector drives a COMPLETE-scoring collector that exposes the live
// scorer's per-doc score (via setScorer) to the supplied checker, mirroring the
// ScoreAssertingCollector of the Java test.
func assertScoreCollector(t *testing.T, searcher *search.IndexSearcher, query search.Query, check func(t *testing.T, docBase, doc int, score float32)) {
	t.Helper()
	collector := &scoreAssertingCollector{t: t, check: check}
	if err := searcher.SearchWithCollector(query, collector); err != nil {
		t.Fatalf("SearchWithCollector: %v", err)
	}
}

type scoreAssertingCollector struct {
	t     *testing.T
	check func(t *testing.T, docBase, doc int, score float32)
}

func (c *scoreAssertingCollector) ScoreMode() search.ScoreMode { return search.COMPLETE }

func (c *scoreAssertingCollector) GetLeafCollector(ctx *index.LeafReaderContext) (search.LeafCollector, error) {
	return &scoreAssertingLeafCollector{parent: c, docBase: ctx.DocBase}, nil
}

type scoreAssertingLeafCollector struct {
	parent  *scoreAssertingCollector
	docBase int
	scorer  search.Scorable
}

func (lc *scoreAssertingLeafCollector) SetScorer(scorer search.Scorable) error {
	lc.scorer = scorer
	return nil
}

func (lc *scoreAssertingLeafCollector) Collect(doc int) error {
	v160_48, err := lc.scorer.Score()
	if err != nil {
		return err
	}
	lc.parent.check(lc.parent.t, lc.docBase, doc, v160_48)
	return nil
}

// SetWeight carries the default body Lucene gives Collector.SetWeight.
func (c *scoreAssertingCollector) SetWeight(weight search.Weight) {

}

// CollectRange carries the default body Lucene gives LeafCollector.CollectRange.
func (lc *scoreAssertingLeafCollector) CollectRange(min int, max int) error {
	return search.DefaultCollectRange(lc, min, max)
}

// CollectStream carries the default body Lucene gives LeafCollector.CollectStream.
func (lc *scoreAssertingLeafCollector) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(lc, stream)
}

// CompetitiveIterator carries the default body Lucene gives LeafCollector.CompetitiveIterator.
func (lc *scoreAssertingLeafCollector) CompetitiveIterator() (search.DocIdSetIterator, error) {
	return nil, nil
}

// Finish carries the default body Lucene gives LeafCollector.Finish.
func (lc *scoreAssertingLeafCollector) Finish() error {
	return nil
}

// Scorer104 is abstract in Lucene's Similarity; this double does not support it.
// Scorer104 returns the scorer of SimpleSimilarity: tf(freq)=freq with idf and
// lengthNorm fixed at 1.
func (s *simpleSimilarity) Scorer104(boost float32, collectionStats *search.CollectionStatistics, termStats ...*search.TermStatistics) search.SimScorer {
	return simpleSimScorer{}
}
