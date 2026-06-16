// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package testutil_test

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	idxtestutil "github.com/FlavioCFOliveira/Gocene/index/testutil"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/search/testutil"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// buildScoringIndex builds an index in which the term "alpha" occurs with
// varying frequencies so that per-document TF/IDF scores differ and exceed 1.0,
// and only in a strict subset of documents so idf > 0. This makes the block-max
// (CheckTopScores) and collector-path (CheckHitCollector) assertions non-vacuous
// rather than trivially passing on a constant-score query.
//
// The plan mirrors search/term_scorer_max_score_test.go: documents carrying
// "alpha" repeated freq times, plus several "beta"-only documents to keep
// docFreq("alpha") < maxDoc. Returns the searcher, the global doc ids that match
// field:alpha (in id order), and a cleanup func.
func buildScoringIndex(t *testing.T) (searcher *search.IndexSearcher, alphaDocs []int, cleanup func()) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	// Interleave commits (creating multiple segments) but never force-merge:
	// the real ForceMerge write-path is not yet complete (roadmap #114) and a
	// merge would drop postings. A multi-segment reader exercises the per-leaf
	// docBase rebasing the collector-path and block-max helpers depend on.
	riw := idxtestutil.NewWithConfig(mustWriter(t, dir, cfg), 7, idxtestutil.Config{
		CommitProbability:     0.5,
		ForceMergeProbability: -1,
	})

	alphaFreqs := []int{1, 5, 2, 9, 3, 7, 4, 6}
	docID := 0
	for _, freq := range alphaFreqs {
		doc := document.NewDocument()
		text := strings.TrimSpace(strings.Repeat("alpha ", freq))
		f, err := document.NewTextField("content", text, true)
		if err != nil {
			t.Fatalf("NewTextField: %v", err)
		}
		doc.Add(f)
		if err := riw.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
		alphaDocs = append(alphaDocs, docID)
		docID++
	}
	for i := 0; i < 4; i++ {
		doc := document.NewDocument()
		f, err := document.NewTextField("content", "beta", true)
		if err != nil {
			t.Fatalf("NewTextField: %v", err)
		}
		doc.Add(f)
		if err := riw.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
		docID++
	}

	if err := riw.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := riw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	searcher = search.NewIndexSearcher(reader)
	cleanup = func() {
		reader.Close()
		dir.Close()
	}
	return searcher, alphaDocs, cleanup
}

func TestCheckHitCollector_TermQuery(t *testing.T) {
	searcher, cleanup := buildIndex(t)
	defer cleanup()

	q := search.NewTermQuery(index.NewTerm("field", "aaa"))

	// Correct expected set passes.
	rt := &recordTB{}
	testutil.CheckHitCollector(rt, q, "field", searcher, []int{0, 2, 4})
	if rt.failed() {
		t.Fatalf("CheckHitCollector flagged a correct collector result set: errs=%v fatals=%v", rt.errs, rt.fatals)
	}

	// Wrong expected set fails.
	rt = &recordTB{}
	testutil.CheckHitCollector(rt, q, "field", searcher, []int{0, 1})
	if !rt.failed() {
		t.Fatalf("CheckHitCollector did not flag a wrong collector result set")
	}
}

// TestCheckHitCollector_MultiSegmentDocBase proves the collector path correctly
// rebases leaf-local doc ids by their segment docBase: the matching set must be
// the global ids, which it cross-checks against the top-docs path.
func TestCheckHitCollector_MultiSegmentDocBase(t *testing.T) {
	searcher, alphaDocs, cleanup := buildScoringIndex(t)
	defer cleanup()

	// Sanity: the index must actually be multi-segment, otherwise this test is
	// not exercising docBase rebasing.
	if dr, ok := searcher.GetIndexReader().(*index.DirectoryReader); ok {
		leaves, err := dr.Leaves()
		if err != nil {
			t.Fatalf("Leaves: %v", err)
		}
		if len(leaves) < 2 {
			t.Fatalf("expected a multi-segment index to exercise docBase rebasing, got %d leaf(s)", len(leaves))
		}
		t.Logf("multi-segment index with %d leaves", len(leaves))
	} else {
		t.Fatalf("expected a DirectoryReader")
	}

	q := search.NewTermQuery(index.NewTerm("content", "alpha"))
	rt := &recordTB{}
	testutil.CheckHitCollector(rt, q, "content", searcher, alphaDocs)
	if rt.failed() {
		t.Fatalf("CheckHitCollector flagged the correct multi-segment alpha set %v: errs=%v fatals=%v",
			alphaDocs, rt.errs, rt.fatals)
	}
}

func TestCheckMatches_TermQuery(t *testing.T) {
	searcher, cleanup := buildIndex(t)
	defer cleanup()

	q := search.NewTermQuery(index.NewTerm("field", "aaa"))

	rt := &recordTB{}
	testutil.CheckMatches(rt, q, searcher)
	if rt.failed() {
		t.Fatalf("CheckMatches flagged a TermQuery whose Matches is non-null on every hit: errs=%v fatals=%v",
			rt.errs, rt.fatals)
	}
}

// TestCheckMatches_DetectsNullMatches confirms CheckMatches is not vacuous: a
// query whose Weight.Matches always returns nil must trip the assertion on its
// first hit. brokenMatchQuery wraps a real TermQuery but overrides Matches.
func TestCheckMatches_DetectsNullMatches(t *testing.T) {
	searcher, cleanup := buildIndex(t)
	defer cleanup()

	inner := search.NewTermQuery(index.NewTerm("field", "aaa"))
	q := &nullMatchesQuery{inner: inner}

	rt := &recordTB{}
	testutil.CheckMatches(rt, q, searcher)
	if !rt.failed() {
		t.Fatalf("CheckMatches did not flag a query whose Matches is always null")
	}
}

func TestCheckTopScores_TermQuery(t *testing.T) {
	searcher, alphaDocs, cleanup := buildScoringIndex(t)
	defer cleanup()
	_ = alphaDocs

	searcher.SetSimilarity(search.NewRawTFSimilarity())

	q := search.NewTermQuery(index.NewTerm("content", "alpha"))

	// Confirm the query produces varying scores above 1.0 so the block-max
	// upper-bound assertion is non-vacuous (a constant 1.0 stub would pass
	// trivially). The global max must dominate any per-doc score.
	top, err := searcher.Search(q, 100)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	var maxObserved float32
	for _, h := range top.ScoreDocs {
		if h.Score > maxObserved {
			maxObserved = h.Score
		}
	}
	if maxObserved <= 1.0 {
		t.Fatalf("test not exercising the >1.0 score path: max observed score %v", maxObserved)
	}
	t.Logf("alpha TermQuery max score = %v across %d hits", maxObserved, len(top.ScoreDocs))

	rng := rand.New(rand.NewSource(12345))
	rt := &recordTB{}
	testutil.CheckTopScores(rt, rng, q, searcher)
	if rt.failed() {
		t.Fatalf("CheckTopScores flagged a real TermQuery: errs=%v fatals=%v", rt.errs, rt.fatals)
	}
}

// nullMatchesQuery wraps a Query but forces its Weight.Matches to always return
// nil, so CheckMatches must flag it. Rewrite/CreateWeight delegate to a wrapping
// weight that overrides Matches; everything else delegates to the inner weight.
type nullMatchesQuery struct {
	search.BaseQuery
	inner search.Query
}

func (q *nullMatchesQuery) Rewrite(reader search.IndexReader) (search.Query, error) {
	// Keep the wrapper across rewrite so CreateWeight installs the null-Matches
	// weight; the inner query is rewritten underneath.
	rw, err := q.inner.Rewrite(reader)
	if err != nil {
		return nil, err
	}
	if rw == q.inner {
		return q, nil
	}
	return &nullMatchesQuery{inner: rw}, nil
}

func (q *nullMatchesQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	inner, err := q.inner.CreateWeight(searcher, needsScores, boost)
	if err != nil {
		return nil, err
	}
	return &nullMatchesWeight{Weight: inner}, nil
}

func (q *nullMatchesQuery) String() string {
	return "nullMatches(" + testutil.QueryString(q.inner, "") + ")"
}

// nullMatchesWeight delegates to an inner Weight but always reports nil Matches.
type nullMatchesWeight struct {
	search.Weight
}

func (w *nullMatchesWeight) Matches(context *index.LeafReaderContext, doc int) (search.Matches, error) {
	return nil, nil
}
