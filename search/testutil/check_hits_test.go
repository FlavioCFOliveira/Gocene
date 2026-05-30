// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package testutil_test

import (
	"fmt"
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

// recordTB is a recording stub satisfying testutil.TB so the tests can
// assert on both the pass and fail paths of the CheckHits helpers
// without aborting the real *testing.T.
type recordTB struct {
	errs   []string
	fatals []string
}

func (r *recordTB) Helper() {}
func (r *recordTB) Errorf(format string, args ...interface{}) {
	r.errs = append(r.errs, fmt.Sprintf(format, args...))
}
func (r *recordTB) Fatalf(format string, args ...interface{}) {
	r.fatals = append(r.fatals, fmt.Sprintf(format, args...))
}
func (r *recordTB) failed() bool { return len(r.errs) > 0 || len(r.fatals) > 0 }

// buildIndex builds a single-segment index using RandomIndexWriter and
// returns a searcher. Docs (field "field"):
//
//	0: "aaa bbb"   1: "bbb ccc"   2: "aaa ccc"   3: "ddd"   4: "aaa"
//
// TermQuery field:aaa therefore matches docs {0, 2, 4}.
func buildIndex(t *testing.T) (*search.IndexSearcher, func()) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	// Exercise RandomIndexWriter's commit interleaving but disable the
	// random force-merge: the real ForceMerge data write-path is not yet
	// implemented (roadmap #114), so a merge would drop the postings and
	// the segment would read back empty. A negative probability clamps to
	// zero in NewWithConfig.
	riw := idxtestutil.NewWithConfig(mustWriter(t, dir, cfg), 42, idxtestutil.Config{
		CommitProbability:     0.06,
		ForceMergeProbability: -1,
	})
	contents := []string{"aaa bbb", "bbb ccc", "aaa ccc", "ddd", "aaa"}
	for _, c := range contents {
		doc := document.NewDocument()
		f, err := document.NewTextField("field", c, true)
		if err != nil {
			t.Fatalf("NewTextField: %v", err)
		}
		doc.Add(f)
		if err := riw.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
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
	searcher := search.NewIndexSearcher(reader)
	cleanup := func() {
		reader.Close()
		dir.Close()
	}
	return searcher, cleanup
}

// mustWriter builds an IndexWriter or fails the test.
func mustWriter(t *testing.T, dir store.Directory, cfg *index.IndexWriterConfig) *index.IndexWriter {
	t.Helper()
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	return w
}

func TestCheckHits_MatchingSet(t *testing.T) {
	searcher, cleanup := buildIndex(t)
	defer cleanup()

	q := search.NewTermQuery(index.NewTerm("field", "aaa"))

	// Correct expected set passes.
	rt := &recordTB{}
	testutil.CheckHits(rt, q, "field", searcher, []int{0, 2, 4})
	if rt.failed() {
		t.Fatalf("CheckHits flagged a correct result set: errs=%v fatals=%v", rt.errs, rt.fatals)
	}

	// Wrong expected set fails.
	rt = &recordTB{}
	testutil.CheckHits(rt, q, "field", searcher, []int{0, 1})
	if !rt.failed() {
		t.Fatalf("CheckHits did not flag a wrong result set")
	}
}

func TestCheckDocIds_Order(t *testing.T) {
	hits := []*search.ScoreDoc{
		search.NewScoreDoc(3, 1.0, -1),
		search.NewScoreDoc(7, 0.5, -1),
	}
	rt := &recordTB{}
	testutil.CheckDocIds(rt, "ok", []int{3, 7}, hits)
	if rt.failed() {
		t.Fatalf("CheckDocIds flagged a correct order: %v", rt.errs)
	}

	rt = &recordTB{}
	testutil.CheckDocIds(rt, "bad", []int{7, 3}, hits)
	if !rt.failed() {
		t.Fatalf("CheckDocIds did not flag a wrong order")
	}

	rt = &recordTB{}
	testutil.CheckDocIds(rt, "len", []int{3}, hits)
	if !rt.failed() {
		t.Fatalf("CheckDocIds did not flag a length mismatch")
	}
}

func TestCheckEqual_ScoresAndDocs(t *testing.T) {
	q := search.NewTermQuery(index.NewTerm("field", "aaa"))
	a := []*search.ScoreDoc{search.NewScoreDoc(0, 1.25, -1), search.NewScoreDoc(2, 0.75, -1)}
	bSame := []*search.ScoreDoc{search.NewScoreDoc(0, 1.25, -1), search.NewScoreDoc(2, 0.75, -1)}
	bDocDiff := []*search.ScoreDoc{search.NewScoreDoc(1, 1.25, -1), search.NewScoreDoc(2, 0.75, -1)}
	bScoreDiff := []*search.ScoreDoc{search.NewScoreDoc(0, 1.25, -1), search.NewScoreDoc(2, 0.80, -1)}

	rt := &recordTB{}
	testutil.CheckEqual(rt, q, a, bSame)
	if rt.failed() {
		t.Fatalf("CheckEqual flagged identical lists: %v %v", rt.errs, rt.fatals)
	}

	rt = &recordTB{}
	testutil.CheckEqual(rt, q, a, bDocDiff)
	if !rt.failed() {
		t.Fatalf("CheckEqual did not flag a doc id difference")
	}

	rt = &recordTB{}
	testutil.CheckEqual(rt, q, a, bScoreDiff)
	if !rt.failed() {
		t.Fatalf("CheckEqual did not flag a score difference beyond tolerance")
	}

	// Within tolerance must pass.
	bTiny := []*search.ScoreDoc{search.NewScoreDoc(0, 1.25, -1), search.NewScoreDoc(2, 0.75+1e-7, -1)}
	rt = &recordTB{}
	testutil.CheckEqual(rt, q, a, bTiny)
	if rt.failed() {
		t.Fatalf("CheckEqual flagged a sub-tolerance score difference: %v %v", rt.errs, rt.fatals)
	}
}

func TestCheckHitsQuery_Combined(t *testing.T) {
	searcher, cleanup := buildIndex(t)
	defer cleanup()

	q := search.NewTermQuery(index.NewTerm("field", "aaa"))
	top, err := searcher.Search(q, 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	results := make([]int, len(top.ScoreDocs))
	for i, h := range top.ScoreDocs {
		results[i] = h.Doc
	}

	rt := &recordTB{}
	testutil.CheckHitsQuery(rt, q, top.ScoreDocs, top.ScoreDocs, results)
	if rt.failed() {
		t.Fatalf("CheckHitsQuery flagged identical hit lists: %v %v", rt.errs, rt.fatals)
	}
}

func TestCheckExplanations_TermQuery(t *testing.T) {
	searcher, cleanup := buildIndex(t)
	defer cleanup()

	q := search.NewTermQuery(index.NewTerm("field", "aaa"))

	rt := &recordTB{}
	testutil.CheckExplanations(rt, q, "field", searcher, false)
	if rt.failed() {
		t.Fatalf("CheckExplanations flagged a TermQuery: errs=%v fatals=%v", rt.errs, rt.fatals)
	}
}

func TestCheckNoMatchExplanations_TermQuery(t *testing.T) {
	searcher, cleanup := buildIndex(t)
	defer cleanup()

	q := search.NewTermQuery(index.NewTerm("field", "aaa"))

	// Docs {0,2,4} match; {1,3} must not match.
	rt := &recordTB{}
	testutil.CheckNoMatchExplanations(rt, q, "field", searcher, []int{0, 2, 4})
	if rt.failed() {
		t.Fatalf("CheckNoMatchExplanations flagged correct non-matches: errs=%v fatals=%v", rt.errs, rt.fatals)
	}

	// If we wrongly claim only {0} matches, docs 2 and 4 are scanned and
	// (being real matches) must trip the assertion.
	rt = &recordTB{}
	testutil.CheckNoMatchExplanations(rt, q, "field", searcher, []int{0})
	if !rt.failed() {
		t.Fatalf("CheckNoMatchExplanations did not flag a matching doc claimed as non-match")
	}
}

func TestVerifyExplanation_SumOfDeep(t *testing.T) {
	// 5.0 = sum of: [2.0, 3.0]
	root := search.MatchExplanation(5.0, "5.0, sum of:")
	root.AddDetail(search.MatchExplanation(2.0, "weight a"))
	root.AddDetail(search.MatchExplanation(3.0, "weight b"))

	rt := &recordTB{}
	testutil.VerifyExplanation(rt, "q", 0, 5.0, true, root)
	if rt.failed() {
		t.Fatalf("VerifyExplanation flagged a consistent sum-of tree: errs=%v fatals=%v", rt.errs, rt.fatals)
	}

	// A wrong top-level score must be flagged.
	rt = &recordTB{}
	testutil.VerifyExplanation(rt, "q", 0, 4.0, false, root)
	if !rt.failed() {
		t.Fatalf("VerifyExplanation did not flag a wrong top-level score")
	}

	// Details that don't combine to the parent value must be flagged.
	bad := search.MatchExplanation(5.0, "5.0, sum of:")
	bad.AddDetail(search.MatchExplanation(2.0, "weight a"))
	bad.AddDetail(search.MatchExplanation(2.0, "weight b")) // 2+2 != 5
	rt = &recordTB{}
	testutil.VerifyExplanation(rt, "q", 0, 5.0, true, bad)
	if !rt.failed() {
		t.Fatalf("VerifyExplanation did not flag inconsistent sub-details")
	}
}

func TestVerifyExplanation_ProductOfDeep(t *testing.T) {
	// 6.0 = product of: [2.0, 3.0]
	root := search.MatchExplanation(6.0, "6.0, product of:")
	root.AddDetail(search.MatchExplanation(2.0, "a"))
	root.AddDetail(search.MatchExplanation(3.0, "b"))
	rt := &recordTB{}
	testutil.VerifyExplanation(rt, "q", 0, 6.0, true, root)
	if rt.failed() {
		t.Fatalf("VerifyExplanation flagged a consistent product-of tree: %v %v", rt.errs, rt.fatals)
	}
}

func TestHits2StrAndTopDocsString(t *testing.T) {
	hits := []*search.ScoreDoc{search.NewScoreDoc(0, 1.0, -1), search.NewScoreDoc(2, 0.5, -1)}
	s := testutil.Hits2Str(hits, hits, 0, 0)
	if !strings.Contains(s, "length1=2") || !strings.Contains(s, "doc0=1") {
		t.Fatalf("Hits2Str unexpected: %q", s)
	}

	td := search.NewTopDocs(&search.TotalHits{Value: 2}, hits)
	ts := testutil.TopDocsString(td, 0, 0)
	if !strings.Contains(ts, "totalHits=2") || !strings.Contains(ts, "doc=0") {
		t.Fatalf("TopDocsString unexpected: %q", ts)
	}
}

func TestQueryString(t *testing.T) {
	q := search.NewTermQuery(index.NewTerm("field", "aaa"))
	s := testutil.QueryString(q, "field")
	if s == "" {
		t.Fatalf("QueryString returned empty")
	}
}
