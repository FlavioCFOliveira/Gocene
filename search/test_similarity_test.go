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

func (simpleSimScorer) Score(_ int, freq float32) float32 { return freq }

func TestSimilarity_Similarity(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
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
		if addErr := w.AddDocument(doc); addErr != nil {
			t.Fatalf("AddDocument: %v", addErr)
		}
	}
	addDoc("a c")
	addDoc("a c b")
	if err = w.Commit(); err != nil {
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

	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(a), search.SHOULD)
	bq.Add(search.NewTermQuery(b), search.SHOULD)
	// Each matching doc's score must equal doc+base+1 (a:freq=1 + b:freq=1
	// where present), exercising the SHOULD-sum scoring path.
	assertScoreCollector(t, searcher, bq, func(t *testing.T, docBase, doc int, score float32) {
		want := float32(doc + docBase + 1)
		if score != want {
			t.Errorf("doc %d (base %d): score = %v, want %v", doc, docBase, score, want)
		}
	})

	pq := search.NewPhraseQuery(a.Field, a, c)
	assertScore(t, searcher, pq, 1.0)

	pq2 := search.NewPhraseQueryWithSlop(2, a.Field, a, b)
	assertScore(t, searcher, pq2, 0.5)
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

type scoreAssertingCollector struct {
	t     *testing.T
	check func(t *testing.T, docBase, doc int, score float32)
}

func (c *scoreAssertingCollector) ScoreMode() search.ScoreMode { return search.COMPLETE }

func (c *scoreAssertingCollector) GetLeafCollector(ctx *index.LeafReaderContext) (search.LeafCollector, error) {
	return &scoreAssertingLeafCollector{parent: c, docBase: ctx.DocBase()}, nil
}

type scoreAssertingLeafCollector struct {
	parent  *scoreAssertingCollector
	docBase int
	scorer  search.Scorer
}

func (lc *scoreAssertingLeafCollector) SetScorer(scorer search.Scorer) error {
	lc.scorer = scorer
	return nil
}

func (lc *scoreAssertingLeafCollector) Collect(doc int) error {
	lc.parent.check(lc.parent.t, lc.docBase, doc, lc.scorer.Score())
	return nil
}