// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file: search/multi_collector_test.go
// Source: lucene/core/src/test/org/apache/lucene/search/TestMultiCollector.java
//
// Tests MultiCollector, which allows running a search with several Collectors.
//
// These tests are an internal (package search) test on purpose: several of the
// upstream assertions inspect the concrete type of the Scorable handed to each
// child collector (ScoreCachingWrappingScorer, MinCompetitiveScoreAwareScorable,
// FilterScorable). Gocene models those as the unexported wrapper types
// *ScoreCachingWrappingScorer, *minCompetitiveScoreAwareScorer and
// *ignoreMinCompetitiveScorer, so the equivalent assertions can only be made
// from inside the package.
//
// Gocene deviations from the Lucene reference:
//   - Lucene drives full-index tests through a CollectorManager; Gocene has no
//     CollectorManager, so the full-search tests drive a single MultiCollector
//     through IndexSearcher.SearchWithCollector and assert the same observable
//     per-collector counts.
//   - Lucene's getLeafCollector(LeafReaderContext) accepts a null context in
//     the unit tests; the Gocene equivalent passes a nil *index.LeafReaderContext
//     and the collectors under test tolerate it (they do not read the context).
//   - CollectionTerminatedException is a control-flow signal returned as an
//     error and detected with IsCollectionTerminated, rather than thrown/caught.

package search

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// ───────────────────────── test scorers ─────────────────────────

// simpleTestScorer is the analogue of Lucene's SimpleScorable: a Scorable with
// a constant zero score. It embeds BaseDocIdSetIterator to satisfy the iterator
// half of the Scorer interface (the MultiCollector tests never iterate it).
type simpleTestScorer struct {
	BaseDocIdSetIterator
	score float32
}

func (s *simpleTestScorer) Score() float32               { return s.score }
func (s *simpleTestScorer) GetMaxScore(upTo int) float32 { return s.score }
func (s *simpleTestScorer) AdvanceShallow(int) (int, error) {
	return NO_MORE_DOCS, nil
}

// minScoreRecordingScorer records the minimum competitive score forwarded to it,
// mirroring the anonymous Scorable used by Lucene's testMinCompetitiveScore.
type minScoreRecordingScorer struct {
	BaseDocIdSetIterator
	minCompetitiveScore float32
}

func (s *minScoreRecordingScorer) Score() float32               { return 0 }
func (s *minScoreRecordingScorer) GetMaxScore(upTo int) float32 { return 0 }
func (s *minScoreRecordingScorer) AdvanceShallow(int) (int, error) {
	return NO_MORE_DOCS, nil
}
func (s *minScoreRecordingScorer) SetMinCompetitiveScore(minScore float32) error {
	s.minCompetitiveScore = minScore
	return nil
}

var _ MinCompetitiveScorer = (*minScoreRecordingScorer)(nil)

// failOnMinScoreScorer fails the test if setMinCompetitiveScore is ever called,
// the analogue of the Scorable that throws AssertionError in
// Lucene's testDisablesSetMinScore.
type failOnMinScoreScorer struct {
	BaseDocIdSetIterator
	t *testing.T
}

func (s *failOnMinScoreScorer) Score() float32               { return 0 }
func (s *failOnMinScoreScorer) GetMaxScore(upTo int) float32 { return 0 }
func (s *failOnMinScoreScorer) AdvanceShallow(int) (int, error) {
	return NO_MORE_DOCS, nil
}
func (s *failOnMinScoreScorer) SetMinCompetitiveScore(minScore float32) error {
	s.t.Fatalf("setMinCompetitiveScore must not be called, got %v", minScore)
	return nil
}

var _ MinCompetitiveScorer = (*failOnMinScoreScorer)(nil)

// The terminateAfterCollector and setScorerCollector below play the role of
// Lucene's FilterCollector/FilterLeafCollector delegators. A standalone
// FilterCollector port is unnecessary in Go because each test wrapper embeds
// its own delegate field directly.

// ───────────────────────── dummy collectors ─────────────────────────

// dummyCollector is the Go port of Lucene's DummyCollector: a Collector that
// records whether collect, setScorer and getLeafCollector ("setNextReader")
// were called.
type dummyCollector struct {
	collectCalled       bool
	setNextReaderCalled bool
	setScorerCalled     bool
	scoreMode           ScoreMode
}

func newDummyCollector() *dummyCollector { return &dummyCollector{scoreMode: COMPLETE} }

func (c *dummyCollector) GetLeafCollector(context *index.LeafReaderContext) (LeafCollector, error) {
	c.setNextReaderCalled = true
	return c, nil
}
func (c *dummyCollector) ScoreMode() ScoreMode { return c.scoreMode }
func (c *dummyCollector) SetScorer(scorer Scorer) error {
	c.setScorerCalled = true
	return nil
}
func (c *dummyCollector) Collect(doc int) error {
	c.collectCalled = true
	return nil
}

var (
	_ Collector     = (*dummyCollector)(nil)
	_ LeafCollector = (*dummyCollector)(nil)
)

// terminatingDummyCollector is the Go port of Lucene's TerminatingDummyCollector:
// a dummyCollector whose Collect returns a CollectionTerminatedException when it
// sees a specific doc id.
type terminatingDummyCollector struct {
	dummyCollector
	terminateOnDoc int
}

func newTerminatingDummyCollector(terminateOnDoc int, scoreMode ScoreMode) *terminatingDummyCollector {
	c := &terminatingDummyCollector{terminateOnDoc: terminateOnDoc}
	c.scoreMode = scoreMode
	return c
}

func (c *terminatingDummyCollector) GetLeafCollector(context *index.LeafReaderContext) (LeafCollector, error) {
	c.setNextReaderCalled = true
	return c, nil
}

func (c *terminatingDummyCollector) Collect(doc int) error {
	if doc == c.terminateOnDoc {
		return NewCollectionTerminatedException()
	}
	return c.dummyCollector.Collect(doc)
}

// dummyTotalHitCountCollector is the Go port of the test-only
// DummyTotalHitCountCollector: it counts collected documents. It needs scores
// to be disabled, matching ScoreMode COMPLETE_NO_SCORES.
type dummyTotalHitCountCollector struct {
	totalHits int
}

func (c *dummyTotalHitCountCollector) GetLeafCollector(context *index.LeafReaderContext) (LeafCollector, error) {
	return c, nil
}
func (c *dummyTotalHitCountCollector) ScoreMode() ScoreMode          { return COMPLETE_NO_SCORES }
func (c *dummyTotalHitCountCollector) SetScorer(scorer Scorer) error { return nil }
func (c *dummyTotalHitCountCollector) Collect(doc int) error {
	c.totalHits++
	return nil
}
func (c *dummyTotalHitCountCollector) getTotalHits() int { return c.totalHits }

var (
	_ Collector     = (*dummyTotalHitCountCollector)(nil)
	_ LeafCollector = (*dummyTotalHitCountCollector)(nil)
)

// terminateAfterCollector is the Go port of Lucene's TerminateAfterCollector:
// a FilterCollector that signals CollectionTerminatedException once it has
// collected terminateAfter documents (across leaves), both from
// getLeafCollector and from collect.
type terminateAfterCollector struct {
	in             Collector
	count          int
	terminateAfter int
}

func newTerminateAfterCollector(in Collector, terminateAfter int) *terminateAfterCollector {
	return &terminateAfterCollector{in: in, terminateAfter: terminateAfter}
}

func (c *terminateAfterCollector) ScoreMode() ScoreMode { return c.in.ScoreMode() }

func (c *terminateAfterCollector) GetLeafCollector(context *index.LeafReaderContext) (LeafCollector, error) {
	if c.count >= c.terminateAfter {
		return nil, NewCollectionTerminatedException()
	}
	in, err := c.in.GetLeafCollector(context)
	if err != nil {
		return nil, err
	}
	return &terminateAfterLeafCollector{in: in, parent: c}, nil
}

type terminateAfterLeafCollector struct {
	in     LeafCollector
	parent *terminateAfterCollector
}

func (c *terminateAfterLeafCollector) SetScorer(scorer Scorer) error { return c.in.SetScorer(scorer) }
func (c *terminateAfterLeafCollector) Collect(doc int) error {
	if c.parent.count >= c.parent.terminateAfter {
		return NewCollectionTerminatedException()
	}
	if err := c.in.Collect(doc); err != nil {
		return err
	}
	c.parent.count++
	return nil
}
func (c *terminateAfterLeafCollector) Finish() error { return finishLeafCollector(c.in) }

// setScorerCollector is the Go port of Lucene's SetScorerCollector: a
// FilterCollector that records when setScorer is invoked on its leaf collector.
type setScorerCollector struct {
	in              Collector
	setScorerCalled *bool
}

func newSetScorerCollector(in Collector, flag *bool) *setScorerCollector {
	return &setScorerCollector{in: in, setScorerCalled: flag}
}

func (c *setScorerCollector) ScoreMode() ScoreMode { return c.in.ScoreMode() }
func (c *setScorerCollector) GetLeafCollector(context *index.LeafReaderContext) (LeafCollector, error) {
	in, err := c.in.GetLeafCollector(context)
	if err != nil {
		return nil, err
	}
	return &setScorerLeafCollector{in: in, setScorerCalled: c.setScorerCalled}, nil
}

type setScorerLeafCollector struct {
	in              LeafCollector
	setScorerCalled *bool
}

func (c *setScorerLeafCollector) SetScorer(scorer Scorer) error {
	if err := c.in.SetScorer(scorer); err != nil {
		return err
	}
	*c.setScorerCalled = true
	return nil
}
func (c *setScorerLeafCollector) Collect(doc int) error { return c.in.Collect(doc) }
func (c *setScorerLeafCollector) Finish() error         { return finishLeafCollector(c.in) }

// scorerCheckCollector is the Go port of the anonymous collector(scoreMode,
// expectedScorer) helper in the Lucene test: its leaf collector's setScorer
// unwraps FilterScorable layers and asserts the underlying scorer type matches
// what is expected. In Gocene the wrapper layers are *ignoreMinCompetitiveScorer
// (the FilterScorable analogue) and the expected types are matched with a
// caller-supplied predicate.
type scorerCheckCollector struct {
	scoreMode ScoreMode
	check     func(scorer Scorer) error
}

func collectorWithScorerCheck(scoreMode ScoreMode, check func(scorer Scorer) error) *scorerCheckCollector {
	return &scorerCheckCollector{scoreMode: scoreMode, check: check}
}

func (c *scorerCheckCollector) ScoreMode() ScoreMode { return c.scoreMode }
func (c *scorerCheckCollector) GetLeafCollector(context *index.LeafReaderContext) (LeafCollector, error) {
	return &scorerCheckLeafCollector{check: c.check}, nil
}

type scorerCheckLeafCollector struct {
	check func(scorer Scorer) error
}

func (c *scorerCheckLeafCollector) SetScorer(scorer Scorer) error { return c.check(scorer) }
func (c *scorerCheckLeafCollector) Collect(doc int) error         { return nil }

// unwrapIgnoreMinCompetitive peels off any *ignoreMinCompetitiveScorer layers,
// the Gocene analogue of unwrapping FilterScorable to reach the real scorer.
func unwrapIgnoreMinCompetitive(scorer Scorer) Scorer {
	for {
		w, ok := scorer.(*ignoreMinCompetitiveScorer)
		if !ok {
			return scorer
		}
		scorer = w.Scorer
	}
}

// ───────────────────────── helpers ─────────────────────────

// oneDocLeafContext builds a one-document index and returns its single leaf
// reader context, mirroring the RandomIndexWriter + reader.leaves().get(0)
// setup shared by several Lucene tests. The returned cleanup closes the reader
// and directory.
func oneDocLeafContext(t *testing.T) (*index.LeafReaderContext, func()) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := w.AddDocument(document.NewDocument()); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}
	if len(leaves) == 0 {
		t.Fatal("expected at least one leaf")
	}
	cleanup := func() {
		_ = reader.Close()
		_ = dir.Close()
	}
	return leaves[0], cleanup
}

// ───────────────────────── tests ─────────────────────────

// TestMultiCollector_NullCollectors ports testNullCollectors: wrap rejects all
// nil collectors and tolerates some nil ones.
func TestMultiCollector_NullCollectors(t *testing.T) {
	if _, err := MultiCollectorWrap(nil, nil); err == nil {
		t.Fatal("expected error when all collectors are nil")
	}

	c, err := MultiCollectorWrap(newDummyCollector(), nil, newDummyCollector())
	if err != nil {
		t.Fatalf("MultiCollectorWrap: %v", err)
	}
	if _, ok := c.(*MultiCollector); !ok {
		t.Fatalf("expected *MultiCollector, got %T", c)
	}
	// Exercise getLeafCollector/collect/setScorer to ensure no nil dereference.
	ac, err := c.GetLeafCollector(nil)
	if err != nil {
		t.Fatalf("GetLeafCollector: %v", err)
	}
	if err := ac.Collect(1); err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if _, err := c.GetLeafCollector(nil); err != nil {
		t.Fatalf("GetLeafCollector: %v", err)
	}
	ac, err = c.GetLeafCollector(nil)
	if err != nil {
		t.Fatalf("GetLeafCollector: %v", err)
	}
	if err := ac.SetScorer(&simpleTestScorer{}); err != nil {
		t.Fatalf("SetScorer: %v", err)
	}
}

// TestMultiCollector_SingleCollector ports testSingleCollector: a single
// collector is returned unchanged (same instance), not wrapped.
func TestMultiCollector_SingleCollector(t *testing.T) {
	dc := newDummyCollector()

	c, err := MultiCollectorWrap(dc)
	if err != nil {
		t.Fatalf("MultiCollectorWrap: %v", err)
	}
	if c != Collector(dc) {
		t.Fatalf("expected same instance for single collector, got %T", c)
	}

	c, err = MultiCollectorWrap(dc, nil)
	if err != nil {
		t.Fatalf("MultiCollectorWrap: %v", err)
	}
	if c != Collector(dc) {
		t.Fatalf("expected same instance with trailing nil, got %T", c)
	}
}

// TestMultiCollector_Delegation ports testCollector: the MultiCollector
// delegates getLeafCollector/collect/setScorer to every wrapped collector.
func TestMultiCollector_Delegation(t *testing.T) {
	dcs := []*dummyCollector{newDummyCollector(), newDummyCollector()}

	c, err := MultiCollectorWrap(dcs[0], dcs[1])
	if err != nil {
		t.Fatalf("MultiCollectorWrap: %v", err)
	}

	ac, err := c.GetLeafCollector(nil)
	if err != nil {
		t.Fatalf("GetLeafCollector: %v", err)
	}
	if err := ac.Collect(1); err != nil {
		t.Fatalf("Collect: %v", err)
	}
	ac, err = c.GetLeafCollector(nil)
	if err != nil {
		t.Fatalf("GetLeafCollector: %v", err)
	}
	if err := ac.SetScorer(&simpleTestScorer{}); err != nil {
		t.Fatalf("SetScorer: %v", err)
	}

	for i, dc := range dcs {
		if !dc.collectCalled {
			t.Errorf("collector %d: collect was not called", i)
		}
		if !dc.setNextReaderCalled {
			t.Errorf("collector %d: getLeafCollector was not called", i)
		}
		if !dc.setScorerCalled {
			t.Errorf("collector %d: setScorer was not called", i)
		}
	}
}

// TestMultiCollector_MergeScoreModes ports testMergeScoreModes over every pair
// of score modes.
func TestMultiCollector_MergeScoreModes(t *testing.T) {
	modes := []ScoreMode{COMPLETE, COMPLETE_NO_SCORES, TOP_SCORES, TOP_DOCS}
	for _, sm1 := range modes {
		for _, sm2 := range modes {
			c1 := newTerminatingDummyCollector(0, sm1)
			c2 := newTerminatingDummyCollector(0, sm2)
			c, err := MultiCollectorWrap(c1, c2)
			if err != nil {
				t.Fatalf("MultiCollectorWrap: %v", err)
			}
			got := c.ScoreMode()
			var want ScoreMode
			switch {
			case sm1 == sm2:
				want = sm1
			case sm1.needsScores() || sm2.needsScores():
				want = COMPLETE
			default:
				want = COMPLETE_NO_SCORES
			}
			if got != want {
				t.Errorf("scoreMode(%v,%v) = %v, want %v", sm1, sm2, got, want)
			}
		}
	}
}

// TestMultiCollector_CollectionTermination ports testCollectionTermination: a
// terminated child is dropped, the rest keep collecting, and once all have
// terminated the leaf collector signals CollectionTerminatedException.
func TestMultiCollector_CollectionTermination(t *testing.T) {
	c1 := newTerminatingDummyCollector(1, COMPLETE)
	c2 := newTerminatingDummyCollector(2, COMPLETE)

	mc, err := MultiCollectorWrap(c1, c2)
	if err != nil {
		t.Fatalf("MultiCollectorWrap: %v", err)
	}
	lc, err := mc.GetLeafCollector(nil)
	if err != nil {
		t.Fatalf("GetLeafCollector: %v", err)
	}
	if err := lc.SetScorer(&simpleTestScorer{}); err != nil {
		t.Fatalf("SetScorer: %v", err)
	}

	if err := lc.Collect(0); err != nil {
		t.Fatalf("Collect(0): %v", err)
	}
	if !c1.collectCalled {
		t.Error("c1 collect should be called for doc 0")
	}
	if !c2.collectCalled {
		t.Error("c2 collect should be called for doc 0")
	}

	c1.collectCalled = false
	c2.collectCalled = false
	if err := lc.Collect(1); err != nil { // c1 terminates on doc 1
		t.Fatalf("Collect(1): %v", err)
	}
	if c1.collectCalled {
		t.Error("c1 should be removed already")
	}
	if !c2.collectCalled {
		t.Error("c2 collect should be called for doc 1")
	}

	c2.collectCalled = false
	err = lc.Collect(2) // c2 terminates on doc 2 -> all terminated
	if !IsCollectionTerminated(err) {
		t.Fatalf("Collect(2) = %v, want CollectionTerminatedException", err)
	}
	if c1.collectCalled || c2.collectCalled {
		t.Error("no collector should be called after all terminated")
	}
}

// TestMultiCollector_SetScorerOnTermination ports
// testSetScorerOnCollectionTerminationSkipNonCompetitive (TOP_SCORES children).
func TestMultiCollector_SetScorerOnTermination(t *testing.T) {
	doTestSetScorerOnCollectionTermination(t, true)
}

// TestMultiCollector_SetScorerAfterTermination ports testSetScorerAfterCollectionTerminated:
// after a child terminates, setScorer is no longer propagated to it.
func TestMultiCollector_SetScorerAfterTermination(t *testing.T) {
	col1 := Collector(&dummyTotalHitCountCollector{})
	col2 := Collector(&dummyTotalHitCountCollector{})

	var setScorerCalled1, setScorerCalled2 bool
	col1 = newSetScorerCollector(col1, &setScorerCalled1)
	col2 = newSetScorerCollector(col2, &setScorerCalled2)

	col1 = newTerminateAfterCollector(col1, 1)
	col2 = newTerminateAfterCollector(col2, 2)

	scorer := &simpleTestScorer{}

	collector, err := MultiCollectorWrap(col1, col2)
	if err != nil {
		t.Fatalf("MultiCollectorWrap: %v", err)
	}

	lc, err := collector.GetLeafCollector(nil)
	if err != nil {
		t.Fatalf("GetLeafCollector: %v", err)
	}
	if err := lc.SetScorer(scorer); err != nil {
		t.Fatalf("SetScorer: %v", err)
	}
	if !setScorerCalled1 || !setScorerCalled2 {
		t.Fatal("both setScorer should have been called")
	}

	if err := lc.Collect(0); err != nil {
		t.Fatalf("Collect(0): %v", err)
	}
	if err := lc.Collect(1); err != nil { // col1 hits its terminateAfter
		t.Fatalf("Collect(1): %v", err)
	}

	setScorerCalled1, setScorerCalled2 = false, false
	if err := lc.SetScorer(scorer); err != nil {
		t.Fatalf("SetScorer: %v", err)
	}
	if setScorerCalled1 {
		t.Error("setScorer should NOT be called on terminated col1")
	}
	if !setScorerCalled2 {
		t.Error("setScorer should be called on live col2")
	}

	if err := lc.Collect(1); !IsCollectionTerminated(err) { // col2 terminates -> all terminated
		t.Fatalf("Collect(1) = %v, want CollectionTerminatedException", err)
	}

	setScorerCalled1, setScorerCalled2 = false, false
	if err := lc.SetScorer(scorer); err != nil {
		t.Fatalf("SetScorer: %v", err)
	}
	if setScorerCalled1 || setScorerCalled2 {
		t.Error("setScorer should not be called once all collectors terminated")
	}
}

// doTestSetScorerOnCollectionTermination ports doTestSetScorerOnCollectionTermination.
func doTestSetScorerOnCollectionTermination(t *testing.T, allowSkipNonCompetitive bool) {
	t.Helper()
	mode := COMPLETE
	if allowSkipNonCompetitive {
		mode = TOP_SCORES
	}
	c1 := newTerminatingDummyCollector(1, mode)
	c2 := newTerminatingDummyCollector(2, mode)

	mc, err := MultiCollectorWrap(c1, c2)
	if err != nil {
		t.Fatalf("MultiCollectorWrap: %v", err)
	}
	lc, err := mc.GetLeafCollector(nil)
	if err != nil {
		t.Fatalf("GetLeafCollector: %v", err)
	}

	if c1.setScorerCalled || c2.setScorerCalled {
		t.Fatal("setScorer not yet called")
	}
	if err := lc.SetScorer(&simpleTestScorer{}); err != nil {
		t.Fatalf("SetScorer: %v", err)
	}
	if !c1.setScorerCalled || !c2.setScorerCalled {
		t.Fatal("setScorer should be called on both")
	}
	c1.setScorerCalled, c2.setScorerCalled = false, false
	if err := lc.Collect(0); err != nil {
		t.Fatalf("Collect(0): %v", err)
	}

	if err := lc.SetScorer(&simpleTestScorer{}); err != nil {
		t.Fatalf("SetScorer: %v", err)
	}
	if !c1.setScorerCalled || !c2.setScorerCalled {
		t.Fatal("setScorer should be called on both before any termination")
	}
	c1.setScorerCalled, c2.setScorerCalled = false, false

	if err := lc.Collect(1); err != nil { // c1 terminates
		t.Fatalf("Collect(1): %v", err)
	}
	if err := lc.SetScorer(&simpleTestScorer{}); err != nil {
		t.Fatalf("SetScorer: %v", err)
	}
	if c1.setScorerCalled {
		t.Error("setScorer should NOT be called on terminated c1")
	}
	if !c2.setScorerCalled {
		t.Error("setScorer should be called on live c2")
	}
	c2.setScorerCalled = false

	if err := lc.Collect(2); !IsCollectionTerminated(err) { // c2 terminates -> all terminated
		t.Fatalf("Collect(2) = %v, want CollectionTerminatedException", err)
	}
	if err := lc.SetScorer(&simpleTestScorer{}); err != nil {
		t.Fatalf("SetScorer: %v", err)
	}
	if c1.setScorerCalled || c2.setScorerCalled {
		t.Error("setScorer should not be called once all collectors terminated")
	}
}

// TestMultiCollector_MinCompetitiveScore ports testMinCompetitiveScore: the
// shared minimum competitive score is only raised when every child has raised
// its own minimum, and equals the smallest per-child minimum.
func TestMultiCollector_MinCompetitiveScore(t *testing.T) {
	currentMinScores := make([]float32, 3)
	scorer := &minScoreRecordingScorer{}

	s0 := newMinCompetitiveScoreAwareScorer(scorer, 0, currentMinScores)
	s1 := newMinCompetitiveScoreAwareScorer(scorer, 1, currentMinScores)
	s2 := newMinCompetitiveScoreAwareScorer(scorer, 2, currentMinScores)

	assertMin := func(want float32) {
		t.Helper()
		if scorer.minCompetitiveScore != want {
			t.Fatalf("minCompetitiveScore = %v, want %v", scorer.minCompetitiveScore, want)
		}
	}

	assertMin(0)
	mustSetMin(t, s0, 0.5)
	assertMin(0) // s1, s2 still 0 -> min is 0
	mustSetMin(t, s1, 0.8)
	assertMin(0) // s2 still 0
	mustSetMin(t, s2, 0.3)
	assertMin(0.3) // now all > 0; min(0.5,0.8,0.3) = 0.3
	mustSetMin(t, s2, 0.1)
	assertMin(0.3) // 0.1 < current 0.3 for idx2, ignored (not > minScores[2])
	mustSetMin(t, s1, math.MaxFloat32)
	assertMin(0.3) // min(0.5, MAX, 0.3) = 0.3
	mustSetMin(t, s2, math.MaxFloat32)
	assertMin(0.5) // min(0.5, MAX, MAX) = 0.5
	mustSetMin(t, s0, math.MaxFloat32)
	assertMin(math.MaxFloat32)
}

func mustSetMin(t *testing.T, s MinCompetitiveScorer, v float32) {
	t.Helper()
	if err := s.SetMinCompetitiveScore(v); err != nil {
		t.Fatalf("SetMinCompetitiveScore: %v", err)
	}
}

// TestMultiCollector_DisablesSetMinScore ports testDisablesSetMinScore: when a
// TOP_SCORES collector is mixed with a non-scoring one, the global score mode
// is COMPLETE, so setMinCompetitiveScore must be suppressed (the underlying
// scorer must never see it).
func TestMultiCollector_DisablesSetMinScore(t *testing.T) {
	ctx, cleanup := oneDocLeafContext(t)
	defer cleanup()

	scorer := &failOnMinScoreScorer{t: t}

	collector := newMinScoreSettingCollector()
	multiCollector, err := MultiCollectorWrap(collector, &dummyTotalHitCountCollector{})
	if err != nil {
		t.Fatalf("MultiCollectorWrap: %v", err)
	}
	leafCollector, err := multiCollector.GetLeafCollector(ctx)
	if err != nil {
		t.Fatalf("GetLeafCollector: %v", err)
	}
	if err := leafCollector.SetScorer(scorer); err != nil {
		t.Fatalf("SetScorer: %v", err)
	}
	if err := leafCollector.Collect(0); err != nil { // must not call setMinCompetitiveScore
		t.Fatalf("Collect: %v", err)
	}
}

// minScoreSettingCollector is the Go port of the anonymous TOP_SCORES collector
// in testDisablesSetMinScore that calls setMinCompetitiveScore on every collect.
type minScoreSettingCollector struct {
	scorer   Scorer
	minScore float32
}

func newMinScoreSettingCollector() *minScoreSettingCollector { return &minScoreSettingCollector{} }

func (c *minScoreSettingCollector) ScoreMode() ScoreMode { return TOP_SCORES }
func (c *minScoreSettingCollector) GetLeafCollector(context *index.LeafReaderContext) (LeafCollector, error) {
	return c, nil
}
func (c *minScoreSettingCollector) SetScorer(scorer Scorer) error {
	c.scorer = scorer
	return nil
}
func (c *minScoreSettingCollector) Collect(doc int) error {
	c.minScore = math.Nextafter32(c.minScore, math.MaxFloat32)
	if mc, ok := c.scorer.(MinCompetitiveScorer); ok {
		return mc.SetMinCompetitiveScore(c.minScore)
	}
	return nil
}

var (
	_ Collector     = (*minScoreSettingCollector)(nil)
	_ LeafCollector = (*minScoreSettingCollector)(nil)
)

// TestMultiCollector_CollectionTerminatedExceptionHandling ports
// testCollectionTerminatedExceptionHandling: each child terminates after its
// own threshold, and the per-child counts equal min(terminateAfter, numDocs).
func TestMultiCollector_CollectionTerminatedExceptionHandling(t *testing.T) {
	const numDocs = 200

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		// A stored+indexed field so MatchAllDocsQuery returns every document and
		// the segment has content; field name/value are irrelevant to the test.
		f, _ := document.NewTextField("content", "x", true)
		doc.Add(f)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	searcher := NewIndexSearcher(reader)

	type entry struct {
		collector     *dummyTotalHitCountCollector
		expectedCount int
	}
	terminateAfters := []int{0, 1, 50, numDocs, numDocs + 10}
	entries := make([]entry, 0, len(terminateAfters))
	collectors := make([]Collector, 0, len(terminateAfters))
	for _, ta := range terminateAfters {
		dc := &dummyTotalHitCountCollector{}
		expected := ta
		if expected > numDocs {
			expected = numDocs
		}
		entries = append(entries, entry{collector: dc, expectedCount: expected})
		collectors = append(collectors, newTerminateAfterCollector(dc, ta))
	}

	mc, err := MultiCollectorWrap(collectors...)
	if err != nil {
		t.Fatalf("MultiCollectorWrap: %v", err)
	}
	if err := searcher.SearchWithCollector(NewMatchAllDocsQuery(), mc); err != nil {
		t.Fatalf("SearchWithCollector: %v", err)
	}

	for i, e := range entries {
		if got := e.collector.getTotalHits(); got != e.expectedCount {
			t.Errorf("collector %d (terminateAfter=%d): totalHits = %d, want %d",
				i, terminateAfters[i], got, e.expectedCount)
		}
	}

// TestMultiCollector_CacheScores ports testCacheScoresIfNecessary: scores are
// only cached (the child sees a *ScoreCachingWrappingScorer) when at least two
// children need scores.
func TestMultiCollector_CacheScores(t *testing.T) {
	ctx, cleanup := oneDocLeafContext(t)
	defer cleanup()

	// The children see the per-child scorer the multiLeafCollector forwards,
	// which for the non-skip path is an *ignoreMinCompetitiveScorer wrapping
	// whatever the outer (possibly score-caching) leaf collector installed. We
	// peel that wrapper before checking, mirroring the FilterScorable-unwrap
	// loop in Lucene's collector() helper.
	expectScoreCaching := func(scorer Scorer) error {
		if _, ok := unwrapIgnoreMinCompetitive(scorer).(*ScoreCachingWrappingScorer); !ok {
			t.Errorf("expected *ScoreCachingWrappingScorer, got %T", scorer)
		}
		return nil
	}
	expectNotScoreCaching := func(scorer Scorer) error {
		if _, ok := unwrapIgnoreMinCompetitive(scorer).(*ScoreCachingWrappingScorer); ok {
			t.Errorf("did not expect *ScoreCachingWrappingScorer, got %T", scorer)
		}
		return nil
	}

	// no collector needs scores => no caching
	c1 := collectorWithScorerCheck(COMPLETE_NO_SCORES, expectNotScoreCaching)
	c2 := collectorWithScorerCheck(COMPLETE_NO_SCORES, expectNotScoreCaching)
	runWrappedSetScorer(t, ctx, c1, c2)

	// only one collector needs scores => no caching
	c1 = collectorWithScorerCheck(COMPLETE, expectNotScoreCaching)
	c2 = collectorWithScorerCheck(COMPLETE_NO_SCORES, expectNotScoreCaching)
	runWrappedSetScorer(t, ctx, c1, c2)

	// several collectors need scores => caching
	c1 = collectorWithScorerCheck(COMPLETE, expectScoreCaching)
	c2 = collectorWithScorerCheck(COMPLETE, expectScoreCaching)
	runWrappedSetScorer(t, ctx, c1, c2)
}

// TestMultiCollector_ScorerWrappingForTopScores ports testScorerWrappingForTopScores:
// two TOP_SCORES children each receive a *minCompetitiveScoreAwareScorer; mixing
// TOP_SCORES with COMPLETE (so cacheScores is true) wraps with a
// *ScoreCachingWrappingScorer.
func TestMultiCollector_ScorerWrappingForTopScores(t *testing.T) {
	ctx, cleanup := oneDocLeafContext(t)
	defer cleanup()

	expectMinCompetitiveAware := func(scorer Scorer) error {
		// The skip-non-competitive path forwards the MinCompetitiveScoreAware
		// scorer directly to each child (no ignore wrapper), matching Lucene.
		if _, ok := scorer.(*minCompetitiveScoreAwareScorer); !ok {
			t.Errorf("expected *minCompetitiveScoreAwareScorer, got %T", scorer)
		}
		return nil
	}
	expectScoreCaching := func(scorer Scorer) error {
		if _, ok := unwrapIgnoreMinCompetitive(scorer).(*ScoreCachingWrappingScorer); !ok {
			t.Errorf("expected *ScoreCachingWrappingScorer, got %T", scorer)
		}
		return nil
	}

	// Two TOP_SCORES collectors: global mode is TOP_SCORES, cacheScores is false
	// (only one of the two modes needs scores? both do, so cacheScores is true) —
	// but skipNonCompetitiveScores is true, so each child is wrapped with a
	// MinCompetitiveScoreAware scorer first. The score-caching wrapper sits on the
	// MultiLeafCollector, so the per-child scorer the children observe in setScorer
	// is the MinCompetitiveScoreAware one.
	c1 := collectorWithScorerCheck(TOP_SCORES, expectMinCompetitiveAware)
	c2 := collectorWithScorerCheck(TOP_SCORES, expectMinCompetitiveAware)
	runWrappedSetScorer(t, ctx, c1, c2)

	// TOP_SCORES mixed with COMPLETE: global mode is COMPLETE (disagreement, both
	// need scores), so skipNonCompetitiveScores is false and cacheScores is true;
	// the children see the *ScoreCachingWrappingScorer.
	c1 = collectorWithScorerCheck(TOP_SCORES, expectScoreCaching)
	c2 = collectorWithScorerCheck(COMPLETE, expectScoreCaching)
	runWrappedSetScorer(t, ctx, c1, c2)
}

// runWrappedSetScorer wraps c1 and c2 in a MultiCollector, obtains the leaf
// collector for ctx, and drives setScorer with a plain test scorer so the
// per-child scorer-type assertions in the check functions run.
func runWrappedSetScorer(t *testing.T, ctx *index.LeafReaderContext, c1, c2 Collector) {
	t.Helper()
	mc, err := MultiCollectorWrap(c1, c2)
	if err != nil {
		t.Fatalf("MultiCollectorWrap: %v", err)
	}
	lc, err := mc.GetLeafCollector(ctx)
	if err != nil {
		t.Fatalf("GetLeafCollector: %v", err)
	}
	if err := lc.SetScorer(&simpleTestScorer{}); err != nil {
		t.Fatalf("SetScorer: %v", err)
	}
}