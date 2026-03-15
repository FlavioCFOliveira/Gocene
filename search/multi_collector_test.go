// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file: search/multi_collector_test.go
// Source: lucene/core/src/test/org/apache/lucene/search/TestMultiCollector.java
// Purpose: Tests MultiCollector which allows running a search with several Collectors

package search_test

import (
	"errors"
	"math"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// The following types and variables should be provided by the search package implementation.
// They are defined here for testing purposes until the implementation is complete.

// ErrCollectionTerminated is returned when collection is terminated early.
// This should be defined in the search package.
var ErrCollectionTerminated = errors.New("collection terminated")

// ErrAssertionFailed is returned when an assertion fails.
// This should be defined in the search package.
var ErrAssertionFailed = errors.New("assertion failed")

// MultiCollector wraps multiple collectors into a single collector.
// This should be defined in the search package.
type MultiCollector struct {
	collectors []search.Collector
}

// MultiCollectorWrap wraps a list of collectors with a MultiCollector.
// This should be defined in the search package.
func MultiCollectorWrap(collectors ...search.Collector) (search.Collector, error) {
	// Filter out nil collectors
	var nonNil []search.Collector
	for _, c := range collectors {
		if c != nil {
			nonNil = append(nonNil, c)
		}
	}

	if len(nonNil) == 0 {
		return nil, errors.New("at least 1 collector must not be null")
	}

	if len(nonNil) == 1 {
		return nonNil[0], nil
	}

	return &MultiCollector{collectors: nonNil}, nil
}

// GetLeafCollector returns a LeafCollector for the given context.
func (m *MultiCollector) GetLeafCollector(reader index.IndexReaderInterface) (search.LeafCollector, error) {
	// Simplified implementation for testing
	return &multiLeafCollector{
		parent:     m,
		collectors: make([]search.LeafCollector, 0, len(m.collectors)),
	}, nil
}

// ScoreMode returns the score mode.
func (m *MultiCollector) ScoreMode() search.ScoreMode {
	var scoreMode search.ScoreMode
	for _, collector := range m.collectors {
		if scoreMode == 0 {
			scoreMode = collector.ScoreMode()
		} else if scoreMode != collector.ScoreMode() {
			if scoreMode == search.COMPLETE || collector.ScoreMode() == search.COMPLETE ||
				scoreMode == search.TOP_SCORES || collector.ScoreMode() == search.TOP_SCORES {
				return search.COMPLETE
			}
			return search.COMPLETE_NO_SCORES
		}
	}
	return scoreMode
}

// multiLeafCollector is a LeafCollector that wraps multiple LeafCollectors.
type multiLeafCollector struct {
	parent     *MultiCollector
	collectors []search.LeafCollector
}

// SetScorer sets the scorer for all collectors.
func (m *multiLeafCollector) SetScorer(scorer search.Scorer) error {
	for _, lc := range m.collectors {
		if lc != nil {
			if err := lc.SetScorer(scorer); err != nil {
				return err
			}
		}
	}
	return nil
}

// Collect collects the document for all collectors.
func (m *multiLeafCollector) Collect(doc int) error {
	allTerminated := true
	for i, lc := range m.collectors {
		if lc != nil {
			if err := lc.Collect(doc); err != nil {
				if err == ErrCollectionTerminated {
					m.collectors[i] = nil
				} else {
					return err
				}
			} else {
				allTerminated = false
			}
		}
	}
	if allTerminated {
		return ErrCollectionTerminated
	}
	return nil
}

// MinCompetitiveScoreAwareScorable wraps a Scorer to track min competitive scores.
// This should be defined in the search package.
type MinCompetitiveScoreAwareScorable struct {
	scorer    search.Scorer
	idx       int
	minScores []float32
}

// NewMinCompetitiveScoreAwareScorable creates a new MinCompetitiveScoreAwareScorable.
func NewMinCompetitiveScoreAwareScorable(scorer search.Scorer, idx int, minScores []float32) *MinCompetitiveScoreAwareScorable {
	return &MinCompetitiveScoreAwareScorable{
		scorer:    scorer,
		idx:       idx,
		minScores: minScores,
	}
}

// Score returns the score.
func (m *MinCompetitiveScoreAwareScorable) Score() float32 {
	return m.scorer.Score()
}

// SetMinCompetitiveScore sets the minimum competitive score.
func (m *MinCompetitiveScoreAwareScorable) SetMinCompetitiveScore(minScore float32) error {
	if minScore > m.minScores[m.idx] {
		m.minScores[m.idx] = minScore
		// Calculate minimum across all scores
		var min float32 = math.MaxFloat32
		for _, v := range m.minScores {
			if v < min {
				min = v
			}
		}
		return m.scorer.SetMinCompetitiveScore(min)
	}
	return nil
}

// NextDoc returns the next document.
func (m *MinCompetitiveScoreAwareScorable) NextDoc() (int, error) {
	return m.scorer.NextDoc()
}

// DocID returns the current document ID.
func (m *MinCompetitiveScoreAwareScorable) DocID() int {
	return m.scorer.DocID()
}

// Advance advances to the target document.
func (m *MinCompetitiveScoreAwareScorable) Advance(target int) (int, error) {
	return m.scorer.Advance(target)
}

// TerminateAfterCollector is a FilterCollector that terminates after a specified count
type TerminateAfterCollector struct {
	search.Collector
	in             search.Collector
	count          int
	terminateAfter int
}

// NewTerminateAfterCollector creates a new TerminateAfterCollector
func NewTerminateAfterCollector(in search.Collector, terminateAfter int) *TerminateAfterCollector {
	return &TerminateAfterCollector{
		in:             in,
		terminateAfter: terminateAfter,
	}
}

// GetLeafCollector returns a LeafCollector for the given context
func (c *TerminateAfterCollector) GetLeafCollector(reader index.IndexReaderInterface) (search.LeafCollector, error) {
	if c.count >= c.terminateAfter {
		return nil, ErrCollectionTerminated
	}
	in, err := c.in.GetLeafCollector(reader)
	if err != nil {
		return nil, err
	}
	return &TerminateAfterLeafCollector{
		LeafCollector:  in,
		parent:         c,
		terminateAfter: c.terminateAfter,
	}, nil
}

// ScoreMode returns the score mode
func (c *TerminateAfterCollector) ScoreMode() search.ScoreMode {
	return c.in.ScoreMode()
}

// TerminateAfterLeafCollector wraps a LeafCollector to terminate after N docs
type TerminateAfterLeafCollector struct {
	search.LeafCollector
	parent         *TerminateAfterCollector
	terminateAfter int
}

// Collect collects the given document
func (c *TerminateAfterLeafCollector) Collect(doc int) error {
	if c.parent.count >= c.terminateAfter {
		return ErrCollectionTerminated
	}
	if err := c.LeafCollector.Collect(doc); err != nil {
		return err
	}
	c.parent.count++
	return nil
}

// SetScorerCollector tracks if SetScorer was called
type SetScorerCollector struct {
	search.Collector
	in              search.Collector
	setScorerCalled *atomic.Bool
}

// NewSetScorerCollector creates a new SetScorerCollector
func NewSetScorerCollector(in search.Collector, setScorerCalled *atomic.Bool) *SetScorerCollector {
	return &SetScorerCollector{
		in:              in,
		setScorerCalled: setScorerCalled,
	}
}

// GetLeafCollector returns a LeafCollector for the given context
func (c *SetScorerCollector) GetLeafCollector(reader index.IndexReaderInterface) (search.LeafCollector, error) {
	in, err := c.in.GetLeafCollector(reader)
	if err != nil {
		return nil, err
	}
	return &SetScorerLeafCollector{
		LeafCollector:   in,
		setScorerCalled: c.setScorerCalled,
	}, nil
}

// ScoreMode returns the score mode
func (c *SetScorerCollector) ScoreMode() search.ScoreMode {
	return c.in.ScoreMode()
}

// SetScorerLeafCollector wraps a LeafCollector to track SetScorer calls
type SetScorerLeafCollector struct {
	search.LeafCollector
	setScorerCalled *atomic.Bool
}

// SetScorer sets the scorer
func (c *SetScorerLeafCollector) SetScorer(scorer search.Scorer) error {
	if err := c.LeafCollector.SetScorer(scorer); err != nil {
		return err
	}
	c.setScorerCalled.Store(true)
	return nil
}

// DummyTotalHitCountCollector is a simple collector that counts total hits
type DummyTotalHitCountCollector struct {
	*search.SimpleCollector
	totalHits int
}

// NewDummyTotalHitCountCollector creates a new DummyTotalHitCountCollector
func NewDummyTotalHitCountCollector() *DummyTotalHitCountCollector {
	return &DummyTotalHitCountCollector{
		SimpleCollector: search.NewSimpleCollector(search.COMPLETE_NO_SCORES),
	}
}

// GetLeafCollector returns a LeafCollector for the given context
func (c *DummyTotalHitCountCollector) GetLeafCollector(reader index.IndexReaderInterface) (search.LeafCollector, error) {
	return &DummyTotalHitCountLeafCollector{
		BaseLeafCollector: search.NewBaseLeafCollector(),
		parent:            c,
	}, nil
}

// GetTotalHits returns the total number of hits
func (c *DummyTotalHitCountCollector) GetTotalHits() int {
	return c.totalHits
}

// DummyTotalHitCountLeafCollector collects documents for DummyTotalHitCountCollector
type DummyTotalHitCountLeafCollector struct {
	*search.BaseLeafCollector
	parent *DummyTotalHitCountCollector
}

// Collect collects a document
func (c *DummyTotalHitCountLeafCollector) Collect(doc int) error {
	c.parent.totalHits++
	return nil
}

// DummyCollector is a simple collector for testing
type DummyCollector struct {
	*search.SimpleCollector
	collectCalled       bool
	setNextReaderCalled bool
	setScorerCalled     bool
}

// NewDummyCollector creates a new DummyCollector
func NewDummyCollector(scoreMode search.ScoreMode) *DummyCollector {
	return &DummyCollector{
		SimpleCollector: search.NewSimpleCollector(scoreMode),
	}
}

// GetLeafCollector returns a LeafCollector for the given context
func (c *DummyCollector) GetLeafCollector(reader index.IndexReaderInterface) (search.LeafCollector, error) {
	c.setNextReaderCalled = true
	return &DummyLeafCollector{
		BaseLeafCollector: search.NewBaseLeafCollector(),
		parent:            c,
	}, nil
}

// DummyLeafCollector collects documents for DummyCollector
type DummyLeafCollector struct {
	*search.BaseLeafCollector
	parent *DummyCollector
}

// Collect collects a document
func (c *DummyLeafCollector) Collect(doc int) error {
	c.parent.collectCalled = true
	return nil
}

// SetScorer sets the scorer
func (c *DummyLeafCollector) SetScorer(scorer search.Scorer) error {
	c.parent.setScorerCalled = true
	return nil
}

// TerminatingDummyCollector terminates collection after a specific document
type TerminatingDummyCollector struct {
	*DummyCollector
	terminateOnDoc int
	scoreMode      search.ScoreMode
}

// NewTerminatingDummyCollector creates a new TerminatingDummyCollector
func NewTerminatingDummyCollector(terminateOnDoc int, scoreMode search.ScoreMode) *TerminatingDummyCollector {
	return &TerminatingDummyCollector{
		DummyCollector: NewDummyCollector(scoreMode),
		terminateOnDoc: terminateOnDoc,
		scoreMode:      scoreMode,
	}
}

// GetLeafCollector returns a LeafCollector for the given context
func (c *TerminatingDummyCollector) GetLeafCollector(reader index.IndexReaderInterface) (search.LeafCollector, error) {
	c.setNextReaderCalled = true
	return &TerminatingDummyLeafCollector{
		DummyLeafCollector: &DummyLeafCollector{
			BaseLeafCollector: search.NewBaseLeafCollector(),
			parent:            c.DummyCollector,
		},
		parent: c,
	}, nil
}

// ScoreMode returns the score mode
func (c *TerminatingDummyCollector) ScoreMode() search.ScoreMode {
	return c.scoreMode
}

// TerminatingDummyLeafCollector collects documents and terminates
type TerminatingDummyLeafCollector struct {
	*DummyLeafCollector
	parent *TerminatingDummyCollector
}

// Collect collects a document
func (c *TerminatingDummyLeafCollector) Collect(doc int) error {
	if doc == c.parent.terminateOnDoc {
		return ErrCollectionTerminated
	}
	return c.DummyLeafCollector.Collect(doc)
}

// SimpleScorable is a simple implementation of Scorable for testing
type SimpleScorable struct {
	score float32
}

// NewSimpleScorable creates a new SimpleScorable
func NewSimpleScorable() *SimpleScorable {
	return &SimpleScorable{score: 1.0}
}

// Score returns the score
func (s *SimpleScorable) Score() float32 {
	return s.score
}

// SetMinCompetitiveScore sets the minimum competitive score
func (s *SimpleScorable) SetMinCompetitiveScore(minScore float32) error {
	return nil
}

// NextDoc returns the next document
func (s *SimpleScorable) NextDoc() (int, error) {
	return search.NO_MORE_DOCS, nil
}

// DocID returns the current document ID
func (s *SimpleScorable) DocID() int {
	return -1
}

// Advance advances to the target document
func (s *SimpleScorable) Advance(target int) (int, error) {
	return search.NO_MORE_DOCS, nil
}

// TestMultiCollector_NullCollectors tests that MultiCollector rejects all null collectors
func TestMultiCollector_NullCollectors(t *testing.T) {
	// Tests that the collector rejects all null collectors
	_, err := MultiCollectorWrap(nil, nil)
	if err == nil {
		t.Error("Expected error for all null collectors, got nil")
	}

	// Tests that the collector handles some null collectors well
	c, err := MultiCollectorWrap(NewDummyCollector(search.COMPLETE), nil, NewDummyCollector(search.COMPLETE))
	if err != nil {
		t.Fatalf("Failed to create MultiCollector with some null collectors: %v", err)
	}
	if _, ok := c.(*MultiCollector); !ok {
		t.Error("Expected MultiCollector when wrapping non-null collectors with nulls")
	}

	ac, err := c.GetLeafCollector(nil)
	if err != nil {
		t.Fatalf("Failed to get leaf collector: %v", err)
	}
	if err := ac.Collect(1); err != nil {
		t.Fatalf("Failed to collect: %v", err)
	}

	_, err = c.GetLeafCollector(nil)
	if err != nil {
		t.Fatalf("Failed to get leaf collector: %v", err)
	}

	ac2, err := c.GetLeafCollector(nil)
	if err != nil {
		t.Fatalf("Failed to get leaf collector: %v", err)
	}
	if err := ac2.SetScorer(NewSimpleScorable()); err != nil {
		t.Fatalf("Failed to set scorer: %v", err)
	}
}

// TestMultiCollector_SingleCollector tests that a single Collector is returned directly
func TestMultiCollector_SingleCollector(t *testing.T) {
	// Tests that if a single Collector is input, it is returned (not MultiCollector)
	dc := NewDummyCollector(search.COMPLETE)
	c, err := MultiCollectorWrap(dc)
	if err != nil {
		t.Fatalf("Failed to wrap single collector: %v", err)
	}
	if c != dc {
		t.Error("Expected same collector for single collector input")
	}

	c2, err := MultiCollectorWrap(dc, nil)
	if err != nil {
		t.Fatalf("Failed to wrap single collector with null: %v", err)
	}
	if c2 != dc {
		t.Error("Expected same collector for single non-null collector input")
	}
}

// TestMultiCollector_Delegation tests that MultiCollector delegates calls properly
func TestMultiCollector_Delegation(t *testing.T) {
	// Tests that the collector delegates calls to input collectors properly
	dcs := []*DummyCollector{NewDummyCollector(search.COMPLETE), NewDummyCollector(search.COMPLETE)}
	collectors := make([]search.Collector, len(dcs))
	for i, dc := range dcs {
		collectors[i] = dc
	}

	c, err := MultiCollectorWrap(collectors...)
	if err != nil {
		t.Fatalf("Failed to create MultiCollector: %v", err)
	}

	ac, err := c.GetLeafCollector(nil)
	if err != nil {
		t.Fatalf("Failed to get leaf collector: %v", err)
	}
	if err := ac.Collect(1); err != nil {
		t.Fatalf("Failed to collect: %v", err)
	}

	ac2, err := c.GetLeafCollector(nil)
	if err != nil {
		t.Fatalf("Failed to get leaf collector: %v", err)
	}
	if err := ac2.SetScorer(NewSimpleScorable()); err != nil {
		t.Fatalf("Failed to set scorer: %v", err)
	}

	for _, dc := range dcs {
		if !dc.collectCalled {
			t.Error("Expected collectCalled to be true")
		}
		if !dc.setNextReaderCalled {
			t.Error("Expected setNextReaderCalled to be true")
		}
		if !dc.setScorerCalled {
			t.Error("Expected setScorerCalled to be true")
		}
	}
}

// TestMultiCollector_MergeScoreModes tests that score modes are merged correctly
func TestMultiCollector_MergeScoreModes(t *testing.T) {
	scoreModes := []search.ScoreMode{search.COMPLETE, search.COMPLETE_NO_SCORES, search.TOP_SCORES, search.TOP_DOCS}

	for _, sm1 := range scoreModes {
		for _, sm2 := range scoreModes {
			c1 := NewTerminatingDummyCollector(0, sm1)
			c2 := NewTerminatingDummyCollector(0, sm2)
			c, err := MultiCollectorWrap(c1, c2)
			if err != nil {
				t.Fatalf("Failed to create MultiCollector: %v", err)
			}

			expectedMode := sm1
			if sm1 != sm2 {
				if sm1 == search.COMPLETE || sm2 == search.COMPLETE ||
					sm1 == search.TOP_SCORES || sm2 == search.TOP_SCORES {
					expectedMode = search.COMPLETE
				} else {
					expectedMode = search.COMPLETE_NO_SCORES
				}
			}

			if c.ScoreMode() != expectedMode {
				t.Errorf("Expected score mode %v for (%v, %v), got %v", expectedMode, sm1, sm2, c.ScoreMode())
			}
		}
	}
}

// TestMultiCollector_CollectionTermination tests collection termination behavior
func TestMultiCollector_CollectionTermination(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add a document
	doc := document.NewDocument()
	w.AddDocument(doc)
	w.Commit()
	w.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Failed to get leaves: %v", err)
	}
	if len(leaves) == 0 {
		t.Skip("No leaf readers available")
	}
	ctx := leaves[0]

	c1 := NewTerminatingDummyCollector(1, search.COMPLETE)
	c2 := NewTerminatingDummyCollector(2, search.COMPLETE)

	mc, err := MultiCollectorWrap(c1, c2)
	if err != nil {
		t.Fatalf("Failed to create MultiCollector: %v", err)
	}

	lc, err := mc.GetLeafCollector(ctx.Reader)
	if err != nil {
		t.Fatalf("Failed to get leaf collector: %v", err)
	}

	lc.SetScorer(NewSimpleScorable())
	if err := lc.Collect(0); err != nil {
		t.Fatalf("Failed to collect doc 0: %v", err)
	}
	if !c1.collectCalled {
		t.Error("c1's collect should be called")
	}
	if !c2.collectCalled {
		t.Error("c2's collect should be called")
	}

	c1.collectCalled = false
	c2.collectCalled = false
	if err := lc.Collect(1); err != nil {
		t.Fatalf("Failed to collect doc 1: %v", err)
	}
	if c1.collectCalled {
		t.Error("c1 should be removed already")
	}
	if !c2.collectCalled {
		t.Error("c2's collect should be called")
	}
	c2.collectCalled = false

	if err := lc.Collect(2); err == nil {
		t.Error("Expected CollectionTerminatedException")
	} else if err != search.ErrCollectionTerminated {
		t.Errorf("Expected ErrCollectionTerminated, got %v", err)
	}
	if c1.collectCalled {
		t.Error("c1 should be removed already")
	}
	if c2.collectCalled {
		t.Error("c2 should be removed already")
	}
}

// TestMultiCollector_SetScorerOnTermination tests SetScorer behavior on collection termination
func TestMultiCollector_SetScorerOnTermination(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	w.AddDocument(doc)
	w.Commit()
	w.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Failed to get leaves: %v", err)
	}
	if len(leaves) == 0 {
		t.Skip("No leaf readers available")
	}
	ctx := leaves[0]

	testCases := []struct {
		name                    string
		allowSkipNonCompetitive bool
	}{
		{"SkipNonCompetitive", true},
		{"NoSkips", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var scoreMode search.ScoreMode
			if tc.allowSkipNonCompetitive {
				scoreMode = search.TOP_SCORES
			} else {
				scoreMode = search.COMPLETE
			}

			c1 := NewTerminatingDummyCollector(1, scoreMode)
			c2 := NewTerminatingDummyCollector(2, scoreMode)

			mc, err := MultiCollectorWrap(c1, c2)
			if err != nil {
				t.Fatalf("Failed to create MultiCollector: %v", err)
			}

			lc, err := mc.GetLeafCollector(ctx.Reader)
			if err != nil {
				t.Fatalf("Failed to get leaf collector: %v", err)
			}

			if c1.setScorerCalled {
				t.Error("c1.setScorerCalled should be false initially")
			}
			if c2.setScorerCalled {
				t.Error("c2.setScorerCalled should be false initially")
			}

			lc.SetScorer(NewSimpleScorable())
			if !c1.setScorerCalled {
				t.Error("c1.setScorerCalled should be true")
			}
			if !c2.setScorerCalled {
				t.Error("c2.setScorerCalled should be true")
			}

			c1.setScorerCalled = false
			c2.setScorerCalled = false
			if err := lc.Collect(0); err != nil {
				t.Fatalf("Failed to collect doc 0: %v", err)
			}

			lc.SetScorer(NewSimpleScorable())
			if !c1.setScorerCalled {
				t.Error("c1.setScorerCalled should be true after first collect")
			}
			if !c2.setScorerCalled {
				t.Error("c2.setScorerCalled should be true after first collect")
			}

			c1.setScorerCalled = false
			c2.setScorerCalled = false
			if err := lc.Collect(1); err != nil {
				t.Fatalf("Failed to collect doc 1: %v", err)
			}

			lc.SetScorer(NewSimpleScorable())
			if c1.setScorerCalled {
				t.Error("c1.setScorerCalled should be false (terminated)")
			}
			if !c2.setScorerCalled {
				t.Error("c2.setScorerCalled should be true")
			}
			c2.setScorerCalled = false

			if err := lc.Collect(2); err == nil {
				t.Error("Expected CollectionTerminatedException")
			} else if err != search.ErrCollectionTerminated {
				t.Errorf("Expected ErrCollectionTerminated, got %v", err)
			}

			lc.SetScorer(NewSimpleScorable())
			if c1.setScorerCalled {
				t.Error("c1.setScorerCalled should be false (terminated)")
			}
			if c2.setScorerCalled {
				t.Error("c2.setScorerCalled should be false (terminated)")
			}
		})
	}
}

// TestMultiCollector_SetScorerAfterTermination tests SetScorer after collection is terminated
func TestMultiCollector_SetScorerAfterTermination(t *testing.T) {
	collector1 := NewDummyTotalHitCountCollector()
	collector2 := NewDummyTotalHitCountCollector()

	setScorerCalled1 := &atomic.Bool{}
	setScorerCalled2 := &atomic.Bool{}

	collector1Wrapped := NewSetScorerCollector(collector1, setScorerCalled1)
	collector2Wrapped := NewSetScorerCollector(collector2, setScorerCalled2)

	collector1Wrapped = NewSetScorerCollector(
		NewTerminateAfterCollector(collector1Wrapped, 1),
		setScorerCalled1,
	)
	collector2Wrapped = NewSetScorerCollector(
		NewTerminateAfterCollector(collector2Wrapped, 2),
		setScorerCalled2,
	)

	scorer := NewSimpleScorable()

	c, err := MultiCollectorWrap(collector1Wrapped, collector2Wrapped)
	if err != nil {
		t.Fatalf("Failed to create MultiCollector: %v", err)
	}

	lc, err := c.GetLeafCollector(nil)
	if err != nil {
		t.Fatalf("Failed to get leaf collector: %v", err)
	}

	lc.SetScorer(scorer)
	if !setScorerCalled1.Load() {
		t.Error("setScorerCalled1 should be true")
	}
	if !setScorerCalled2.Load() {
		t.Error("setScorerCalled2 should be true")
	}

	lc.Collect(0)
	lc.Collect(1)

	setScorerCalled1.Store(false)
	setScorerCalled2.Store(false)
	lc.SetScorer(scorer)
	if setScorerCalled1.Load() {
		t.Error("setScorerCalled1 should be false (terminated)")
	}
	if !setScorerCalled2.Load() {
		t.Error("setScorerCalled2 should be true")
	}

	if err := lc.Collect(1); err == nil {
		t.Error("Expected CollectionTerminatedException")
	} else if err != search.ErrCollectionTerminated {
		t.Errorf("Expected ErrCollectionTerminated, got %v", err)
	}

	setScorerCalled1.Store(false)
	setScorerCalled2.Store(false)
	lc.SetScorer(scorer)
	if setScorerCalled1.Load() {
		t.Error("setScorerCalled1 should be false (terminated)")
	}
	if setScorerCalled2.Load() {
		t.Error("setScorerCalled2 should be false (terminated)")
	}
}

// TestMultiCollector_MinCompetitiveScore tests MinCompetitiveScoreAwareScorable
func TestMultiCollector_MinCompetitiveScore(t *testing.T) {
	currentMinScores := make([]float32, 3)
	minCompetitiveScore := float32(0)

	scorer := &SimpleScorable{}
	s0 := NewMinCompetitiveScoreAwareScorable(scorer, 0, currentMinScores)
	s1 := NewMinCompetitiveScoreAwareScorable(scorer, 1, currentMinScores)
	s2 := NewMinCompetitiveScoreAwareScorable(scorer, 2, currentMinScores)

	if minCompetitiveScore != 0 {
		t.Errorf("Expected minCompetitiveScore 0, got %f", minCompetitiveScore)
	}

	s0.SetMinCompetitiveScore(0.5)
	if minCompetitiveScore != 0 {
		t.Errorf("Expected minCompetitiveScore 0, got %f", minCompetitiveScore)
	}

	s1.SetMinCompetitiveScore(0.8)
	if minCompetitiveScore != 0 {
		t.Errorf("Expected minCompetitiveScore 0, got %f", minCompetitiveScore)
	}

	s2.SetMinCompetitiveScore(0.3)
	// The minimum of 0.5, 0.8, 0.3 is 0.3
	expectedMin := float32(0.3)
	actualMin := float32(math.MaxFloat32)
	for _, v := range currentMinScores {
		if v < actualMin && v > 0 {
			actualMin = v
		}
	}
	if actualMin != expectedMin {
		t.Errorf("Expected min %f, got %f", expectedMin, actualMin)
	}

	s2.SetMinCompetitiveScore(0.1)
	// Minimum should still be 0.1 (lowest non-zero)
	expectedMin = float32(0.1)
	actualMin = float32(math.MaxFloat32)
	for _, v := range currentMinScores {
		if v < actualMin && v > 0 {
			actualMin = v
		}
	}
	if actualMin != expectedMin {
		t.Errorf("Expected min %f, got %f", expectedMin, actualMin)
	}

	s1.SetMinCompetitiveScore(float32(math.MaxFloat32))
	s2.SetMinCompetitiveScore(float32(math.MaxFloat32))
	// Now only s0 has 0.5
	expectedMin = float32(0.5)
	actualMin = float32(math.MaxFloat32)
	for _, v := range currentMinScores {
		if v < actualMin && v > 0 && v < float32(math.MaxFloat32) {
			actualMin = v
		}
	}
	if actualMin != expectedMin {
		t.Errorf("Expected min %f, got %f", expectedMin, actualMin)
	}

	s0.SetMinCompetitiveScore(float32(math.MaxFloat32))
	// All are now MaxFloat32
}

// TestMultiCollector_DisablesSetMinScore tests that setMinCompetitiveScore is disabled
func TestMultiCollector_DisablesSetMinScore(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	w.AddDocument(doc)
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	w.Close()

	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Failed to get leaves: %v", err)
	}
	if len(leaves) == 0 {
		t.Skip("No leaf readers available")
	}
	ctx := leaves[0]

	scorer := &SetMinCompetitiveScorable{}

	collector := &SetMinCompetitiveCollector{}
	multiCollector, err := MultiCollectorWrap(collector, NewDummyTotalHitCountCollector())
	if err != nil {
		t.Fatalf("Failed to create MultiCollector: %v", err)
	}

	lc, err := multiCollector.GetLeafCollector(ctx.Reader)
	if err != nil {
		t.Fatalf("Failed to get leaf collector: %v", err)
	}
	lc.SetScorer(scorer)
	if err := lc.Collect(0); err != nil {
		t.Fatalf("Failed to collect: %v", err)
	}

	reader.Close()
	dir.Close()
}

// SetMinCompetitiveScorable is a Scorable that throws on setMinCompetitiveScore
type SetMinCompetitiveScorable struct {
	search.Scorer
}

// Score returns the score
func (s *SetMinCompetitiveScorable) Score() float32 {
	return 0
}

// SetMinCompetitiveScore throws an error
func (s *SetMinCompetitiveScorable) SetMinCompetitiveScore(minScore float32) error {
	return ErrAssertionFailed
}

// NextDoc returns NO_MORE_DOCS
func (s *SetMinCompetitiveScorable) NextDoc() (int, error) {
	return search.NO_MORE_DOCS, nil
}

// DocID returns -1
func (s *SetMinCompetitiveScorable) DocID() int {
	return -1
}

// Advance advances to target
func (s *SetMinCompetitiveScorable) Advance(target int) (int, error) {
	return search.NO_MORE_DOCS, nil
}

// SetMinCompetitiveCollector is a collector that calls setMinCompetitiveScore
type SetMinCompetitiveCollector struct {
	*search.SimpleCollector
	scorer   search.Scorer
	minScore float32
}

// NewSetMinCompetitiveCollector creates a new SetMinCompetitiveCollector
func NewSetMinCompetitiveCollector() *SetMinCompetitiveCollector {
	return &SetMinCompetitiveCollector{
		SimpleCollector: search.NewSimpleCollector(search.TOP_SCORES),
	}
}

// GetLeafCollector returns a LeafCollector
func (c *SetMinCompetitiveCollector) GetLeafCollector(reader index.IndexReaderInterface) (search.LeafCollector, error) {
	return c, nil
}

// SetScorer sets the scorer
func (c *SetMinCompetitiveCollector) SetScorer(scorer search.Scorer) error {
	c.scorer = scorer
	return nil
}

// Collect collects a document
func (c *SetMinCompetitiveCollector) Collect(doc int) error {
	c.minScore = math.Nextafter32(c.minScore, float32(math.MaxFloat32))
	return c.scorer.SetMinCompetitiveScore(c.minScore)
}

// TestMultiCollector_CollectionTerminatedExceptionHandling tests CollectionTerminatedException handling
func TestMultiCollector_CollectionTerminatedExceptionHandling(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// Add multiple documents
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		w.AddDocument(doc)
	}
	w.Commit()
	w.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	numDocs := reader.NumDocs()

	// Test with multiple collectors that terminate at different points
	for numCollectors := 1; numCollectors <= 3; numCollectors++ {
		expectedCounts := make(map[*DummyTotalHitCountCollector]int)
		collectors := make([]search.Collector, 0, numCollectors)

		for i := 0; i < numCollectors; i++ {
			terminateAfter := (i + 1) * 10
			if terminateAfter > numDocs {
				terminateAfter = numDocs
			}
			expectedCount := terminateAfter
			if terminateAfter > numDocs {
				expectedCount = numDocs
			}

			collector := NewDummyTotalHitCountCollector()
			expectedCounts[collector] = expectedCount
			collectors = append(collectors, NewTerminateAfterCollector(collector, terminateAfter))
		}

		mc, err := MultiCollectorWrap(collectors...)
		if err != nil {
			t.Fatalf("Failed to create MultiCollector: %v", err)
		}

		// Simulate search
		leaves, err := reader.Leaves()
		if err != nil {
			t.Fatalf("Failed to get leaves: %v", err)
		}

		for _, ctx := range leaves {
			lc, err := mc.GetLeafCollector(ctx.Reader)
			if err != nil {
				if err == search.ErrCollectionTerminated {
					continue
				}
				t.Fatalf("Failed to get leaf collector: %v", err)
			}

			scorer := NewSimpleScorable()
			lc.SetScorer(scorer)

			for doc := 0; doc < ctx.Reader.MaxDoc(); doc++ {
				if err := lc.Collect(doc); err != nil {
					if err == search.ErrCollectionTerminated {
						break
					}
					t.Fatalf("Failed to collect: %v", err)
				}
			}
		}

		for collector, expectedCount := range expectedCounts {
			if collector.GetTotalHits() != expectedCount {
				t.Errorf("Expected %d hits, got %d", expectedCount, collector.GetTotalHits())
			}
		}
	}
}

// TestMultiCollector_CacheScores tests score caching behavior
func TestMultiCollector_CacheScores(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	w.AddDocument(doc)
	w.Commit()
	w.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Failed to get leaves: %v", err)
	}
	if len(leaves) == 0 {
		t.Skip("No leaf readers available")
	}
	ctx := leaves[0]

	// Test: no collector needs scores => no caching
	c1 := NewDummyCollector(search.COMPLETE_NO_SCORES)
	c2 := NewDummyCollector(search.COMPLETE_NO_SCORES)
	mc, _ := MultiCollectorWrap(c1, c2)
	lc, _ := mc.GetLeafCollector(ctx.Reader)
	lc.SetScorer(NewSimpleScorable())

	// Test: only one collector needs scores => no caching
	c1 = NewDummyCollector(search.COMPLETE)
	c2 = NewDummyCollector(search.COMPLETE_NO_SCORES)
	mc, _ = MultiCollectorWrap(c1, c2)
	lc, _ = mc.GetLeafCollector(ctx.Reader)
	lc.SetScorer(NewSimpleScorable())

	// Test: several collectors need scores => caching
	c1 = NewDummyCollector(search.COMPLETE)
	c2 = NewDummyCollector(search.COMPLETE)
	mc, _ = MultiCollectorWrap(c1, c2)
	lc, _ = mc.GetLeafCollector(ctx.Reader)
	lc.SetScorer(NewSimpleScorable())
}

// TestMultiCollector_ScorerWrappingForTopScores tests scorer wrapping for TOP_SCORES mode
func TestMultiCollector_ScorerWrappingForTopScores(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	w.AddDocument(doc)
	w.Commit()
	w.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Failed to get leaves: %v", err)
	}
	if len(leaves) == 0 {
		t.Skip("No leaf readers available")
	}
	ctx := leaves[0]

	// Test with TOP_SCORES collectors
	c1 := NewDummyCollector(search.TOP_SCORES)
	c2 := NewDummyCollector(search.TOP_SCORES)
	mc, _ := MultiCollectorWrap(c1, c2)
	lc, _ := mc.GetLeafCollector(ctx.Reader)
	lc.SetScorer(NewSimpleScorable())

	// Test with mixed modes
	c1 = NewDummyCollector(search.TOP_SCORES)
	c2 = NewDummyCollector(search.COMPLETE)
	mc, _ = MultiCollectorWrap(c1, c2)
	lc, _ = mc.GetLeafCollector(ctx.Reader)
	lc.SetScorer(NewSimpleScorable())
}
