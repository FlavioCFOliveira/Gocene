// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package testutil hosts search-side test helpers ported from Apache
// Lucene 10.4.0's lucene-test-framework. It provides [CheckHits], the
// canonical utility for asserting that a query matches an expected set
// of documents, that two result lists are equal, and that per-document
// score explanations are self-consistent.
//
// Lucene reference:
//
//	lucene/test-framework/src/java/org/apache/lucene/tests/search/CheckHits.java
//
// In addition to the result-set and explanation validators, this package
// ports the three collector- and scorer-path validators from Lucene's
// CheckHits: [CheckHitCollector] (the docBase-aware Collector contract,
// roadmap #10), [CheckMatches] (Weight.Matches is non-null on every hit),
// and [CheckTopScores] (the block-max Scorer surface from roadmap #129 —
// AdvanceShallow / GetMaxScore / SetMinCompetitiveScore — produces a valid
// upper bound at every position). All three drive a real in-memory
// IndexSearcher.
package testutil

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// scoreTolerance mirrors CheckHits.checkEqual's 1.0e-6f tolerance for
// comparing the scores of two hit lists.
const scoreTolerance = 1.0e-6

// TB is the subset of testing.TB used by the CheckHits helpers.
// *testing.T and *testing.B satisfy it. Defining a local interface,
// rather than depending on testing.TB directly, keeps the assertion
// surface explicit and lets the package's own tests supply a recording
// stub to exercise both the pass and fail paths. Following Lucene's
// JUnit-throwing semantics, every helper returns immediately after a
// Fatalf so a stub that does not abort the goroutine still stops.
type TB interface {
	Helper()
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
}

// QueryString renders q for assertion messages, mirroring Lucene's
// Query.toString(field). A query may opt in to field-aware rendering
// by implementing ToString(field string) string; otherwise a
// fmt.Stringer is used, falling back to %v.
func QueryString(q search.Query, field string) string {
	switch v := q.(type) {
	case interface{ ToString(string) string }:
		return v.ToString(field)
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", q)
	}
}

// CheckHits tests that query matches exactly the expected set of
// document ids using the top-docs API. Following Lucene, it requests
// max(10, len(results)*2) hits and compares the returned doc ids as a
// set (order-independent).
func CheckHits(t TB, query search.Query, defaultField string, searcher *search.IndexSearcher, results []int) {
	t.Helper()

	n := len(results) * 2
	if n < 10 {
		n = 10
	}
	top, err := searcher.Search(query, n)
	if err != nil {
		t.Fatalf("CheckHits: search failed for [[%s]]: %v", QueryString(query, defaultField), err)
		return
	}

	correct := intSet(results)
	actual := make(map[int]struct{}, len(top.ScoreDocs))
	for _, hit := range top.ScoreDocs {
		actual[hit.Doc] = struct{}{}
	}

	if !setsEqual(correct, actual) {
		t.Errorf("%s\n  expected docs: %v\n  actual docs:   %v",
			QueryString(query, defaultField), sortedKeys(correct), sortedKeys(actual))
	}
}

// CheckHitCollector tests that a query matches the expected set of documents
// using a Collector (rather than the top-docs API). This is the collector-path
// equality check: documents are collected if they "match" regardless of score,
// so the global doc ids gathered must equal the expected set as a set.
//
// It is the Go port of CheckHits.checkHitCollector. The Lucene reference also
// re-runs the query against readers wrapped at three slicing offsets
// (QueryUtils.wrapUnderlyingReader with i in {-1,0,1}) to exercise docBase
// rebasing; Gocene has no public reader-wrapping utility yet, so this port
// instead asserts the collector path against the searcher directly and, as an
// equivalent rebasing check, confirms the collector's per-leaf docBase
// (context.DocBase()) was applied by comparing the collected global ids against
// the top-docs ids for the same query.
func CheckHitCollector(t TB, query search.Query, defaultField string, searcher *search.IndexSearcher, results []int) {
	t.Helper()

	correct := intSet(results)

	collector := newSetCollector()
	if err := searcher.SearchWithCollector(query, collector); err != nil {
		t.Fatalf("CheckHitCollector: search failed for [[%s]]: %v", QueryString(query, defaultField), err)
		return
	}
	actual := collector.bag
	if !setsEqual(correct, actual) {
		t.Errorf("Simple: %s\n  expected docs: %v\n  collected docs: %v",
			QueryString(query, defaultField), sortedKeys(correct), sortedKeys(actual))
		return
	}

	// Cross-check the collector path against the top-docs path: both must agree
	// on the matching set. This catches a docBase rebasing regression — if a leaf
	// collector failed to add context.DocBase() the multi-segment ids would
	// diverge from the top-docs ids even though the single-segment set matched.
	n := len(results) * 2
	if n < 10 {
		n = 10
	}
	top, err := searcher.Search(query, n)
	if err != nil {
		t.Fatalf("CheckHitCollector: top-docs cross-check failed for [[%s]]: %v",
			QueryString(query, defaultField), err)
		return
	}
	topSet := make(map[int]struct{}, len(top.ScoreDocs))
	for _, hit := range top.ScoreDocs {
		topSet[hit.Doc] = struct{}{}
	}
	if !setsEqual(actual, topSet) {
		t.Errorf("Collector vs TopDocs: %s\n  collected docs: %v\n  topdocs docs:  %v",
			QueryString(query, defaultField), sortedKeys(actual), sortedKeys(topSet))
	}
}

// setCollector gathers global document ids into a set, mirroring Lucene's
// CheckHits.SetCollector. It is a docBase-aware Collector: each leaf collector
// captures its segment's docBase from the LeafReaderContext and rebases the
// leaf-local doc ids it is handed, so the bag holds top-level ids.
//
// Its ScoreMode is COMPLETE_NO_SCORES — collection is score-independent.
type setCollector struct {
	*search.SimpleCollector
	bag map[int]struct{}
}

func newSetCollector() *setCollector {
	return &setCollector{
		SimpleCollector: search.NewSimpleCollector(search.COMPLETE_NO_SCORES),
		bag:             make(map[int]struct{}),
	}
}

func (c *setCollector) GetLeafCollector(context *index.LeafReaderContext) (search.LeafCollector, error) {
	docBase := 0
	if context != nil {
		docBase = context.DocBase()
	}
	return &setLeafCollector{bag: c.bag, docBase: docBase}, nil
}

// setLeafCollector adds doc+docBase to the shared bag for each collected doc.
type setLeafCollector struct {
	*search.BaseLeafCollector
	bag     map[int]struct{}
	docBase int
}

func (c *setLeafCollector) SetScorer(scorer search.Scorer) error { return nil }

func (c *setLeafCollector) Collect(doc int) error {
	c.bag[doc+c.docBase] = struct{}{}
	return nil
}

// CheckMatches asserts that Weight.Matches returns a non-null Matches for every
// document matching the query, and that the immediately preceding (non-matching)
// document yields a null Matches. It is the Go port of CheckHits.checkMatches /
// CheckHits.MatchesAsserter.
//
// The Weight is built with COMPLETE_NO_SCORES (matches checking is
// score-independent), exactly as Lucene's MatchesAsserter does. Collection is
// driven through a Collector so the per-leaf context (and thus the leaf-local
// doc id passed to Weight.Matches, plus the segment's docBase for diagnostics)
// is available, faithfully reproducing the SimpleCollector contract.
func CheckMatches(t TB, query search.Query, searcher *search.IndexSearcher) {
	t.Helper()

	rewritten, err := query.Rewrite(searcher.GetIndexReader())
	if err != nil {
		t.Fatalf("CheckMatches: rewrite failed: %v", err)
		return
	}
	weight, err := searcher.CreateWeight(rewritten, search.COMPLETE_NO_SCORES, 1.0)
	if err != nil {
		t.Fatalf("CheckMatches: createWeight failed: %v", err)
		return
	}
	if weight == nil {
		// No weight means no matches; nothing to assert (and nothing should match).
		return
	}

	collector := &matchesAsserter{t: t, weight: weight}
	if err := searcher.SearchWithCollector(rewritten, collector); err != nil {
		t.Fatalf("CheckMatches: search failed for [[%s]]: %v", QueryString(query, ""), err)
	}
}

// matchesAsserter is the Collector that performs the Weight.Matches assertions,
// porting CheckHits.MatchesAsserter.
type matchesAsserter struct {
	*search.SimpleCollector
	t      TB
	weight search.Weight
}

func (a *matchesAsserter) ScoreMode() search.ScoreMode { return search.COMPLETE_NO_SCORES }

func (a *matchesAsserter) GetLeafCollector(context *index.LeafReaderContext) (search.LeafCollector, error) {
	return &matchesAsserterLeaf{t: a.t, weight: a.weight, context: context, lastCheckedDoc: -1}, nil
}

type matchesAsserterLeaf struct {
	*search.BaseLeafCollector
	t       TB
	weight  search.Weight
	context *index.LeafReaderContext
	// lastCheckedDoc tracks the previously collected (leaf-local) doc id so that
	// the gap between two consecutive hits can be probed for a null Matches on
	// the immediately preceding non-matching doc, mirroring MatchesAsserter.
	lastCheckedDoc int
	collectedOnce  bool
}

func (c *matchesAsserterLeaf) SetScorer(scorer search.Scorer) error { return nil }

func (c *matchesAsserterLeaf) Collect(doc int) error {
	matches, err := c.weight.Matches(c.context, doc)
	if err != nil {
		c.t.Errorf("CheckMatches: Matches(doc=%d) errored for query %s: %v",
			doc, QueryString(c.weight.GetQuery(), ""), err)
		return nil
	}
	if matches == nil {
		c.t.Errorf("Unexpected null Matches object in doc %d for query %s",
			doc, QueryString(c.weight.GetQuery(), ""))
		return nil
	}
	if c.collectedOnce && c.lastCheckedDoc != doc-1 {
		prev, err := c.weight.Matches(c.context, doc-1)
		if err != nil {
			c.t.Errorf("CheckMatches: Matches(doc=%d) errored for query %s: %v",
				doc-1, QueryString(c.weight.GetQuery(), ""), err)
		} else if prev != nil {
			c.t.Errorf("Unexpected non-null Matches object in non-matching doc %d for query %s",
				doc-1, QueryString(c.weight.GetQuery(), ""))
		}
	}
	c.collectedOnce = true
	c.lastCheckedDoc = doc
	return nil
}

// CheckTopScores verifies block-max top-score correctness for a query. It is the
// Go port of CheckHits.checkTopScores: first it confirms the top hits are
// computed identically under the COMPLETE and TOP_SCORES collector paths (for
// numHits 1 and 10), then it walks the matches asserting that the exposed max
// scores and block boundaries (AdvanceShallow / GetMaxScore) are valid upper
// bounds and that SetMinCompetitiveScore never strands a competitive document.
//
// rng drives the randomized advance/min-score choices, mirroring the Random
// parameter Lucene threads through. Pass a seeded *rand.Rand for reproducibility.
func CheckTopScores(t TB, rng *rand.Rand, query search.Query, searcher *search.IndexSearcher) {
	t.Helper()
	doCheckTopScores(t, query, searcher, 1)
	doCheckTopScores(t, query, searcher, 10)
	doCheckMaxScores(t, rng, query, searcher)
}

// doCheckTopScores asserts the COMPLETE and TOP_SCORES top-docs are equal for
// the given numHits, porting CheckHits.doCheckTopScores. In Lucene the two
// TopScoreDocCollectorManagers differ only in totalHitsThreshold (Integer.MAX
// vs 1), which toggles dynamic pruning; the produced top hits must be identical
// regardless.
func doCheckTopScores(t TB, query search.Query, searcher *search.IndexSearcher, numHits int) {
	t.Helper()

	completeMgr, err := search.NewTopScoreDocCollectorManager(numHits, nil, math.MaxInt32)
	if err != nil {
		t.Fatalf("doCheckTopScores: complete manager: %v", err)
		return
	}
	topScoresMgr, err := search.NewTopScoreDocCollectorManager(numHits, nil, 1)
	if err != nil {
		t.Fatalf("doCheckTopScores: topScores manager: %v", err)
		return
	}

	complete, err := searchWithManager(searcher, query, completeMgr)
	if err != nil {
		t.Fatalf("doCheckTopScores: complete search: %v", err)
		return
	}
	topScores, err := searchWithManager(searcher, query, topScoresMgr)
	if err != nil {
		t.Fatalf("doCheckTopScores: topScores search: %v", err)
		return
	}
	CheckEqual(t, query, complete.ScoreDocs, topScores.ScoreDocs)
}

// searchWithManager runs a search through a TopScoreDocCollectorManager and
// reduces the per-leaf collectors into a single TopDocs, mirroring
// IndexSearcher.search(Query, CollectorManager). Gocene's searcher drives a
// single Collector per search, so the manager's single collector is used and
// then reduced.
func searchWithManager(searcher *search.IndexSearcher, query search.Query, mgr *search.TopScoreDocCollectorManager) (*search.TopDocs, error) {
	collector, err := mgr.NewCollector()
	if err != nil {
		return nil, err
	}
	if err := searcher.SearchWithCollector(query, collector); err != nil {
		return nil, err
	}
	return mgr.Reduce([]*search.TopScoreDocCollector{collector})
}

// doCheckMaxScores walks the matches of query under the block-max (TOP_SCORES)
// scorer surface and asserts that, at every visited document, the live score
// never exceeds the GetMaxScore upper bound for the current block, that the
// COMPLETE and TOP_SCORES scorers agree on every score, and that
// SetMinCompetitiveScore (when supported) never strands a competitive document.
// It is the Go port of CheckHits.doCheckMaxScores.
//
// Adaptation to Gocene: a Gocene Scorer IS a DocIdSetIterator (it embeds the
// interface), so "s.iterator()" is the scorer itself; the optional
// TwoPhaseIterator is obtained via search.AsTwoPhaseIterator; and
// setMinCompetitiveScore is the optional search.MinCompetitiveScorer interface
// (a scorer that cannot prune simply does not implement it, which is legal and
// leaves the bound assertions intact).
func doCheckMaxScores(t TB, rng *rand.Rand, query search.Query, searcher *search.IndexSearcher) {
	t.Helper()

	rewritten, err := query.Rewrite(searcher.GetIndexReader())
	if err != nil {
		t.Fatalf("doCheckMaxScores: rewrite: %v", err)
		return
	}
	w1, err := searcher.CreateWeight(rewritten, search.COMPLETE, 1.0)
	if err != nil {
		t.Fatalf("doCheckMaxScores: createWeight COMPLETE: %v", err)
		return
	}
	w2, err := searcher.CreateWeight(rewritten, search.TOP_SCORES, 1.0)
	if err != nil {
		t.Fatalf("doCheckMaxScores: createWeight TOP_SCORES: %v", err)
		return
	}

	leaves, err := leavesOf(searcher)
	if err != nil {
		t.Fatalf("doCheckMaxScores: leaves: %v", err)
		return
	}

	// Pass 1: iterate all matches, checking boundaries and max scores.
	for _, ctx := range leaves {
		if stop := checkMaxScoresLeafSequential(t, rng, w1, w2, ctx); stop {
			return
		}
	}

	// Pass 2: the same invariants while advancing by random deltas.
	for _, ctx := range leaves {
		if stop := checkMaxScoresLeafAdvancing(t, rng, w1, w2, ctx); stop {
			return
		}
	}
}

// scorerPair pulls a COMPLETE scorer (s1) and a TOP_SCORES scorer (s2, via its
// ScorerSupplier with SetTopLevelScoringClause) for a leaf, returning their
// approximations. It returns ok=false when the caller should skip the leaf
// after the early s1==nil / s2==nil handling below, and stop=true if a fatal
// assertion already fired.
func makeTopScorer(w search.Weight, ctx *index.LeafReaderContext) (search.Scorer, error) {
	ss, err := w.ScorerSupplier(ctx)
	if err != nil {
		return nil, err
	}
	if ss == nil {
		return nil, nil
	}
	ss.SetTopLevelScoringClause()
	return ss.Get(math.MaxInt64)
}

// iteratorExhausted reports whether s.NextDoc() immediately yields NO_MORE_DOCS,
// used to validate the "one scorer is nil" branches.
func iteratorExhausted(t TB, s search.Scorer) bool {
	if s == nil {
		return true
	}
	doc, err := s.NextDoc()
	if err != nil {
		t.Errorf("doCheckMaxScores: NextDoc on lone scorer: %v", err)
		return true
	}
	return doc == search.NO_MORE_DOCS
}

func checkMaxScoresLeafSequential(t TB, rng *rand.Rand, w1, w2 search.Weight, ctx *index.LeafReaderContext) (stop bool) {
	s1, err := w1.Scorer(ctx)
	if err != nil {
		t.Fatalf("doCheckMaxScores: w1.Scorer: %v", err)
		return true
	}
	s2, err := makeTopScorer(w2, ctx)
	if err != nil {
		t.Fatalf("doCheckMaxScores: w2 top scorer: %v", err)
		return true
	}
	if s1 == nil {
		if !iteratorExhausted(t, s2) {
			t.Errorf("doCheckMaxScores: s1==nil but s2 has matches")
		}
		return false
	}
	if s2 == nil {
		if !iteratorExhausted(t, s1) {
			t.Errorf("doCheckMaxScores: s2==nil but s1 has matches")
		}
		return false
	}

	tp1 := search.AsTwoPhaseIterator(s1)
	tp2 := search.AsTwoPhaseIterator(s2)
	approx1 := approximationOf(s1, tp1)
	approx2 := approximationOf(s2, tp2)

	upTo := -1
	var maxScore, minScore float32

	doc2, err := approx2.NextDoc()
	if err != nil {
		t.Fatalf("doCheckMaxScores: approx2.NextDoc: %v", err)
		return true
	}
	for {
		// Advance approx1 up to doc2; intermediate docs that match the
		// approximation but lie below doc2 must be non-competitive (score<minScore).
		doc1, err := approx1.NextDoc()
		if err != nil {
			t.Fatalf("doCheckMaxScores: approx1.NextDoc: %v", err)
			return true
		}
		for doc1 < doc2 {
			if ok, err := twoPhaseMatches(tp1); err != nil {
				t.Fatalf("doCheckMaxScores: tp1.Matches: %v", err)
				return true
			} else if ok {
				if s1.Score() >= minScore {
					t.Errorf("doCheckMaxScores: skipped doc %d had score %v >= minScore %v",
						doc1, s1.Score(), minScore)
				}
			}
			doc1, err = approx1.NextDoc()
			if err != nil {
				t.Fatalf("doCheckMaxScores: approx1.NextDoc: %v", err)
				return true
			}
		}
		if doc1 != doc2 {
			t.Errorf("doCheckMaxScores: doc1=%d != doc2=%d", doc1, doc2)
		}
		if doc2 == search.NO_MORE_DOCS {
			return false
		}

		if doc2 > upTo {
			upTo, err = s2.AdvanceShallow(doc2)
			if err != nil {
				t.Fatalf("doCheckMaxScores: AdvanceShallow: %v", err)
				return true
			}
			if upTo < doc2 {
				t.Errorf("doCheckMaxScores: AdvanceShallow(%d)=%d < target", doc2, upTo)
			}
			maxScore = s2.GetMaxScore(upTo)
		}

		ok2, err := twoPhaseMatches(tp2)
		if err != nil {
			t.Fatalf("doCheckMaxScores: tp2.Matches: %v", err)
			return true
		}
		if ok2 {
			if ok1, err := twoPhaseMatches(tp1); err != nil {
				t.Fatalf("doCheckMaxScores: tp1.Matches: %v", err)
				return true
			} else if !ok1 {
				t.Errorf("doCheckMaxScores: doc %d matched by s2 but not s1", doc2)
			}
			score := s2.Score()
			if s1.Score() != score {
				t.Errorf("doCheckMaxScores: doc %d score mismatch s1=%v s2=%v", doc2, s1.Score(), score)
			}
			if score > maxScore {
				t.Errorf("doCheckMaxScores: doc %d score %v > maxScore %v up to %d", doc2, score, maxScore, upTo)
			}
			if score >= minScore && rng.Intn(10) == 0 {
				minScore = score
				setMinCompetitiveScore(t, s2, minScore)
			}
		}

		doc2, err = approx2.NextDoc()
		if err != nil {
			t.Fatalf("doCheckMaxScores: approx2.NextDoc: %v", err)
			return true
		}
	}
}

func checkMaxScoresLeafAdvancing(t TB, rng *rand.Rand, w1, w2 search.Weight, ctx *index.LeafReaderContext) (stop bool) {
	s1, err := w1.Scorer(ctx)
	if err != nil {
		t.Fatalf("doCheckMaxScores(adv): w1.Scorer: %v", err)
		return true
	}
	s2, err := makeTopScorer(w2, ctx)
	if err != nil {
		t.Fatalf("doCheckMaxScores(adv): w2 top scorer: %v", err)
		return true
	}
	if s1 == nil {
		if !iteratorExhausted(t, s2) {
			t.Errorf("doCheckMaxScores(adv): s1==nil but s2 has matches")
		}
		return false
	}
	if s2 == nil {
		if !iteratorExhausted(t, s1) {
			t.Errorf("doCheckMaxScores(adv): s2==nil but s1 has matches")
		}
		return false
	}

	tp1 := search.AsTwoPhaseIterator(s1)
	tp2 := search.AsTwoPhaseIterator(s2)
	approx1 := approximationOf(s1, tp1)
	approx2 := approximationOf(s2, tp2)

	upTo := -1
	var maxScore, minScore float32

	for {
		doc2 := s2.DocID()
		advance := rng.Intn(2) == 0
		var target int
		if !advance {
			target = doc2 + 1
		} else {
			delta := 1 + rng.Intn(512)
			if delta > search.NO_MORE_DOCS-doc2 {
				delta = search.NO_MORE_DOCS - doc2
			}
			target = doc2 + delta
		}

		if target > upTo && rng.Intn(2) == 0 {
			delta := rng.Intn(512)
			if delta > search.NO_MORE_DOCS-target {
				delta = search.NO_MORE_DOCS - target
			}
			upTo = target + delta
			m, err := s2.AdvanceShallow(target)
			if err != nil {
				t.Fatalf("doCheckMaxScores(adv): AdvanceShallow: %v", err)
				return true
			}
			if m < target {
				t.Errorf("doCheckMaxScores(adv): AdvanceShallow(%d)=%d < target", target, m)
			}
			maxScore = s2.GetMaxScore(upTo)
		}

		if advance {
			doc2, err = approx2.Advance(target)
		} else {
			doc2, err = approx2.NextDoc()
		}
		if err != nil {
			t.Fatalf("doCheckMaxScores(adv): approx2 step: %v", err)
			return true
		}

		doc1, err := approx1.Advance(target)
		if err != nil {
			t.Fatalf("doCheckMaxScores(adv): approx1.Advance: %v", err)
			return true
		}
		for doc1 < doc2 {
			if ok, err := twoPhaseMatches(tp1); err != nil {
				t.Fatalf("doCheckMaxScores(adv): tp1.Matches: %v", err)
				return true
			} else if ok {
				if s1.Score() >= minScore {
					t.Errorf("doCheckMaxScores(adv): skipped doc %d had score %v >= minScore %v",
						doc1, s1.Score(), minScore)
				}
			}
			doc1, err = approx1.NextDoc()
			if err != nil {
				t.Fatalf("doCheckMaxScores(adv): approx1.NextDoc: %v", err)
				return true
			}
		}
		if doc1 != doc2 {
			t.Errorf("doCheckMaxScores(adv): doc1=%d != doc2=%d", doc1, doc2)
		}
		if doc2 == search.NO_MORE_DOCS {
			return false
		}

		ok2, err := twoPhaseMatches(tp2)
		if err != nil {
			t.Fatalf("doCheckMaxScores(adv): tp2.Matches: %v", err)
			return true
		}
		if ok2 {
			if ok1, err := twoPhaseMatches(tp1); err != nil {
				t.Fatalf("doCheckMaxScores(adv): tp1.Matches: %v", err)
				return true
			} else if !ok1 {
				t.Errorf("doCheckMaxScores(adv): doc %d matched by s2 but not s1", doc2)
			}
			score := s2.Score()
			if s1.Score() != score {
				t.Errorf("doCheckMaxScores(adv): doc %d score mismatch s1=%v s2=%v", doc2, s1.Score(), score)
			}
			if doc2 > upTo {
				upTo, err = s2.AdvanceShallow(doc2)
				if err != nil {
					t.Fatalf("doCheckMaxScores(adv): AdvanceShallow: %v", err)
					return true
				}
				if upTo < doc2 {
					t.Errorf("doCheckMaxScores(adv): AdvanceShallow(%d)=%d < target", doc2, upTo)
				}
				maxScore = s2.GetMaxScore(upTo)
			}
			if score > maxScore {
				t.Errorf("doCheckMaxScores(adv): doc %d score %v > maxScore %v", doc2, score, maxScore)
			}
			if score >= minScore && rng.Intn(10) == 0 {
				minScore = score
				setMinCompetitiveScore(t, s2, minScore)
			}
		}
	}
}

// approximationOf returns the iterator a scorer's matches should be driven
// through: the TwoPhaseIterator's approximation when one is present, otherwise
// the scorer itself (a Gocene Scorer is a DocIdSetIterator).
func approximationOf(s search.Scorer, tp *search.TwoPhaseIterator) search.DocIdSetIterator {
	if tp != nil {
		return tp.Approximation()
	}
	return s
}

// twoPhaseMatches reports whether the current approximation document is a true
// match. A nil TwoPhaseIterator means every approximation hit is a true match.
func twoPhaseMatches(tp *search.TwoPhaseIterator) (bool, error) {
	if tp == nil {
		return true, nil
	}
	return tp.Matches()
}

// setMinCompetitiveScore forwards the hint when the scorer supports pruning.
// A scorer that does not implement search.MinCompetitiveScorer simply cannot
// prune (legal), so the hint is dropped without failing the assertion.
func setMinCompetitiveScore(t TB, s search.Scorer, minScore float32) {
	if mcs, ok := s.(search.MinCompetitiveScorer); ok {
		if err := mcs.SetMinCompetitiveScore(minScore); err != nil {
			t.Errorf("doCheckMaxScores: SetMinCompetitiveScore(%v): %v", minScore, err)
		}
	}
}

// leavesOf returns the leaf contexts of the searcher's reader. It supports the
// DirectoryReader (multi-segment) shape used by the test harness and a single
// leaf reader; each context carries the segment's docBase and ordinal.
func leavesOf(searcher *search.IndexSearcher) ([]*index.LeafReaderContext, error) {
	reader := searcher.GetIndexReader()
	if dr, ok := reader.(*index.DirectoryReader); ok {
		return dr.Leaves()
	}
	return []*index.LeafReaderContext{index.NewLeafReaderContext(reader, nil, 0, 0)}, nil
}

// CheckDocIds tests that hits has exactly the expected doc ids in the
// given order.
func CheckDocIds(t TB, mes string, results []int, hits []*search.ScoreDoc) {
	t.Helper()
	if len(hits) != len(results) {
		t.Errorf("%s nr of hits: expected %d, got %d", mes, len(results), len(hits))
		return
	}
	for i := range results {
		if hits[i].Doc != results[i] {
			t.Errorf("%s doc nrs for hit %d: expected %d, got %d", mes, i, results[i], hits[i].Doc)
		}
	}
}

// CheckHitsQuery tests that two queries produce the expected document
// order and that the two hit lists are equal (doc ids and scores).
func CheckHitsQuery(t TB, query search.Query, hits1, hits2 []*search.ScoreDoc, results []int) {
	t.Helper()
	CheckDocIds(t, "hits1", results, hits1)
	CheckDocIds(t, "hits2", results, hits2)
	CheckEqual(t, query, hits1, hits2)
}

// CheckEqual asserts that two hit lists are equal in length, doc ids,
// and scores (within scoreTolerance).
func CheckEqual(t TB, query search.Query, hits1, hits2 []*search.ScoreDoc) {
	t.Helper()
	if len(hits1) != len(hits2) {
		t.Fatalf("Unequal lengths: hits1=%d,hits2=%d", len(hits1), len(hits2))
		return
	}
	for i := range hits1 {
		if hits1[i].Doc != hits2[i].Doc {
			t.Fatalf("Hit %d docnumbers don't match\n%sfor query:%s",
				i, Hits2Str(hits1, hits2, 0, 0), QueryString(query, ""))
			return
		}
		if math.Abs(float64(hits1[i].Score-hits2[i].Score)) > scoreTolerance {
			t.Fatalf("Hit %d, doc nrs %d and %d\nunequal       : %v\n           and: %v\nfor query:%s",
				i, hits1[i].Doc, hits2[i].Doc, hits1[i].Score, hits2[i].Score, QueryString(query, ""))
			return
		}
	}
}

// Hits2Str formats two hit lists for diagnostic messages, mirroring
// CheckHits.hits2str. end <= 0 means "to the longer of the two".
func Hits2Str(hits1, hits2 []*search.ScoreDoc, start, end int) string {
	var sb strings.Builder
	len1, len2 := len(hits1), len(hits2)
	if end <= 0 {
		end = len1
		if len2 > end {
			end = len2
		}
	}
	fmt.Fprintf(&sb, "Hits length1=%d\tlength2=%d\n", len1, len2)
	for i := start; i < end; i++ {
		fmt.Fprintf(&sb, "hit=%d:", i)
		if i < len1 {
			fmt.Fprintf(&sb, " doc%d=%v shardIndex=%d", hits1[i].Doc, hits1[i].Score, hits1[i].ShardIndex)
		} else {
			sb.WriteString("               ")
		}
		sb.WriteString(",\t")
		if i < len2 {
			fmt.Fprintf(&sb, " doc%d=%v shardIndex=%d", hits2[i].Doc, hits2[i].Score, hits2[i].ShardIndex)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// TopDocsString formats a TopDocs for diagnostic messages, mirroring
// CheckHits.topdocsString. end <= 0 means "to the end of scoreDocs".
func TopDocsString(docs *search.TopDocs, start, end int) string {
	var sb strings.Builder
	total := int64(0)
	if docs.TotalHits != nil {
		total = docs.TotalHits.Value
	}
	fmt.Fprintf(&sb, "TopDocs totalHits=%d top=%d\n", total, len(docs.ScoreDocs))
	if end <= 0 {
		end = len(docs.ScoreDocs)
	} else if end > len(docs.ScoreDocs) {
		end = len(docs.ScoreDocs)
	}
	for i := start; i < end; i++ {
		fmt.Fprintf(&sb, "\t%d) doc=%d\tscore=%v\n", i, docs.ScoreDocs[i].Doc, docs.ScoreDocs[i].Score)
	}
	return sb.String()
}

// CheckNoMatchExplanations tests that every document up to maxDoc which
// is *not* in the expected result set has an explanation that indicates
// a non-match.
func CheckNoMatchExplanations(t TB, q search.Query, defaultField string, searcher *search.IndexSearcher, results []int) {
	t.Helper()
	d := QueryString(q, defaultField)
	ignore := intSet(results)
	maxDoc := searcher.GetIndexReader().MaxDoc()
	for doc := 0; doc < maxDoc; doc++ {
		if _, skip := ignore[doc]; skip {
			continue
		}
		exp, err := searcher.Explain(q, doc)
		if err != nil {
			t.Errorf("Explanation of [[%s]] for #%d errored: %v", d, doc, err)
			continue
		}
		if exp == nil {
			t.Errorf("Explanation of [[%s]] for #%d is null", d, doc)
			continue
		}
		if exp.IsMatch() {
			t.Errorf("Explanation of [[%s]] for #%d doesn't indicate non-match: %s",
				d, doc, ExplanationString(exp))
		}
	}
}

// CheckExplanations asserts that the explanation value for every
// document matching a query corresponds with the true score. When deep
// is true, the sub-detail combine rule (product/sum/max/...) is also
// verified. Unlike the Lucene reference, which drives a collector, this
// port enumerates matches via the top-docs API (requesting every doc),
// which yields the same per-document score under COMPLETE scoring.
func CheckExplanations(t TB, query search.Query, defaultField string, searcher *search.IndexSearcher, deep bool) {
	t.Helper()
	d := QueryString(query, defaultField)
	maxDoc := searcher.GetIndexReader().MaxDoc()
	n := maxDoc
	if n < 1 {
		n = 1
	}
	top, err := searcher.Search(query, n)
	if err != nil {
		t.Fatalf("CheckExplanations: search failed for [[%s]]: %v", d, err)
		return
	}
	for _, hit := range top.ScoreDocs {
		exp, err := searcher.Explain(query, hit.Doc)
		if err != nil {
			t.Errorf("exception in explanation of [[%s]] for #%d: %v", d, hit.Doc, err)
			continue
		}
		if exp == nil {
			t.Errorf("Explanation of [[%s]] for #%d is null", d, hit.Doc)
			continue
		}
		VerifyExplanation(t, d, hit.Doc, hit.Score, deep, exp)
		if !exp.IsMatch() {
			t.Errorf("Explanation of [[%s]] for #%d does not indicate match: %s",
				d, hit.Doc, ExplanationString(exp))
		}
	}
}

// VerifyExplanation asserts that an explanation has the expected score
// and, optionally, that its sub-detail max/sum/product/factor combine
// to that score. This is a faithful port of CheckHits.verifyExplanation.
func VerifyExplanation(t TB, q string, doc int, score float32, deep bool, expl search.Explanation) {
	t.Helper()
	value := expl.GetValue()
	if value != score {
		t.Errorf("%s: score(doc=%d)=%v != explanationScore=%v Explanation: %s",
			q, doc, score, value, ExplanationString(expl))
	}

	if !deep {
		return
	}

	detail := expl.GetDetails()
	if strings.HasSuffix(expl.GetDescription(), "computed from:") {
		return // something more complicated.
	}
	descr := strings.ToLower(expl.GetDescription())
	if strings.HasPrefix(descr, "score based on ") && strings.Contains(descr, "child docs in range") {
		if len(detail) == 0 {
			t.Errorf("Child doc explanations are missing")
		}
	}
	if len(detail) > 0 && expl.IsMatch() {
		if len(detail) == 1 && !computedFromPattern(descr) {
			// Simple containment, unless it's a "freq of:" (which lets a
			// query explain how the freq is calculated); just verify the
			// contained explanation has the same score.
			if !strings.HasSuffix(expl.GetDescription(), "with freq of:") &&
				(score >= 0 || !strings.HasSuffix(expl.GetDescription(), "times others of:")) {
				VerifyExplanation(t, q, doc, score, deep, detail[0])
			}
		} else {
			// The explanation must either end with one of "product of:",
			// "sum of:", "max of:", be "computed as x from:", or read
			// "max plus <x> times others of:".
			var x float32
			productOf := strings.HasSuffix(descr, "product of:")
			sumOf := strings.HasSuffix(descr, "sum of:")
			maxOf := strings.HasSuffix(descr, "max of:")
			computedOf := strings.Index(descr, "computed as") > 0 && computedFromPattern(descr)
			maxTimesOthers := false
			if !(productOf || sumOf || maxOf || computedOf) {
				k1 := strings.Index(descr, "max plus ")
				if k1 >= 0 {
					k1 += len("max plus ")
					k2 := strings.IndexByte(descr[k1:], ' ')
					if k2 >= 0 {
						k2 += k1
						if f, err := strconv.ParseFloat(strings.TrimSpace(descr[k1:k2]), 32); err == nil {
							x = float32(f)
							if strings.TrimSpace(descr[k2:]) == "times others of:" {
								maxTimesOthers = true
							}
						}
					}
				}
			}
			if !(productOf || sumOf || maxOf || computedOf || maxTimesOthers) {
				t.Errorf("%s: multi valued explanation description=%q must be 'max of plus x times others', "+
					"'computed as x from:' or end with 'product of' or 'sum of:' or 'max of:' - %s",
					q, descr, ExplanationString(expl))
			}
			var sum float64
			var product float32 = 1
			max := float32(math.Inf(-1))
			var maxError float64
			for i := range detail {
				dval := detail[i].GetValue()
				VerifyExplanation(t, q, doc, dval, deep, detail[i])
				product *= dval
				sum += float64(dval)
				if dval > max {
					max = dval
				}
				if sumOf {
					// "sum of" is used by BooleanQuery; intermediate float
					// casts in ReqOptSumScorer require some leniency.
					maxError += float64(ulp32(dval)) * 2
				}
			}
			var combined float32
			switch {
			case productOf:
				combined = product
			case sumOf:
				combined = float32(sum)
			case maxOf:
				combined = max
			case maxTimesOthers:
				combined = float32(float64(max) + float64(x)*(sum-float64(max)))
			default:
				if !computedOf {
					t.Errorf("should never get here!")
				}
				combined = value
			}
			if math.Abs(float64(combined-value)) > maxError {
				t.Errorf("%s: actual subDetails combined==%v != value=%v Explanation: %s",
					q, combined, value, ExplanationString(expl))
			}
		}
	}
}

// --- Internal -------------------------------------------------------

// ExplanationString renders an Explanation tree for diagnostics,
// mirroring the layout of Lucene's Explanation.toString.
func ExplanationString(e search.Explanation) string {
	var sb strings.Builder
	explanationToString(&sb, e, 0)
	return sb.String()
}

func explanationToString(sb *strings.Builder, e search.Explanation, depth int) {
	for i := 0; i < depth; i++ {
		sb.WriteString("  ")
	}
	if e.IsMatch() {
		fmt.Fprintf(sb, "%v = %s\n", e.GetValue(), e.GetDescription())
	} else {
		fmt.Fprintf(sb, "%v = (NON-MATCH) %s\n", e.GetValue(), e.GetDescription())
	}
	for _, d := range e.GetDetails() {
		explanationToString(sb, d, depth+1)
	}
}

// computedFromPattern reports whether descr matches the Lucene
// ".*, computed as .* from:" anchored pattern. Java's Pattern.matches
// is full-string; we replicate it with substring checks ordered so the
// "computed as" precedes " from:" at the tail.
func computedFromPattern(descr string) bool {
	if !strings.HasSuffix(descr, " from:") {
		return false
	}
	ca := strings.Index(descr, ", computed as ")
	return ca >= 0 && ca < len(descr)-len(" from:")
}

// ulp32 returns the unit in the last place of x as a float32, matching
// Java's Math.ulp(float).
func ulp32(x float32) float32 {
	if x != x { // NaN
		return x
	}
	if math.IsInf(float64(x), 0) {
		return float32(math.Inf(1))
	}
	if x == 0 {
		return math.Float32frombits(1)
	}
	bits := math.Float32bits(x)
	// Step to the next representable magnitude and take the gap.
	next := math.Float32frombits(bits + 1)
	d := next - x
	if d < 0 {
		d = -d
	}
	return d
}

func intSet(vals []int) map[int]struct{} {
	s := make(map[int]struct{}, len(vals))
	for _, v := range vals {
		s[v] = struct{}{}
	}
	return s
}

func setsEqual(a, b map[int]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

func sortedKeys(s map[int]struct{}) []int {
	out := make([]int, 0, len(s))
	for k := range s {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}
