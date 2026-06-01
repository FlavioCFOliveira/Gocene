// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

// Ported from Apache Lucene 10.4.0:
//   lucene/join/src/java/org/apache/lucene/search/join/ToParentBlockJoinQuery.java
//   (BlockJoinBulkScorer + BatchAwareLeafCollector + Score)

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// blockJoinScore aggregates child scores for a single parent according to the
// configured ScoreMode. It is the Go port of the inner Score class in
// ToParentBlockJoinQuery (Lucene 10.4.0).
type blockJoinScore struct {
	scoreMode ScoreMode
	score     float64
	freq      int
}

func newBlockJoinScore(scoreMode ScoreMode) *blockJoinScore {
	return &blockJoinScore{scoreMode: scoreMode}
}

// reset seeds the aggregate with the first child's score. Mirrors Score.reset.
func (s *blockJoinScore) reset(firstChild search.Scorer) {
	if s.scoreMode == None {
		s.score = 0
	} else {
		s.score = float64(firstChild.Score())
	}
	s.freq = 1
}

// addChildScore folds a subsequent child's score into the aggregate. Mirrors
// Score.addChildScore.
func (s *blockJoinScore) addChildScore(child search.Scorer) {
	var childScore float64
	if s.scoreMode != None {
		childScore = float64(child.Score())
	}
	s.freq++
	switch s.scoreMode {
	case Total, Avg:
		s.score += childScore
	case Min:
		if childScore < s.score {
			s.score = childScore
		}
	case Max:
		if childScore > s.score {
			s.score = childScore
		}
	case None:
		// no score contribution
	}
}

// value returns the aggregated score, dividing by freq for Avg. Mirrors
// Score.score().
func (s *blockJoinScore) value() float32 {
	score := s.score
	if s.scoreMode == Avg && s.freq > 0 {
		score /= float64(s.freq)
	}
	return float32(score)
}

// BlockJoinBulkScorer evaluates all child hits per parent in batches, emitting
// one collect call per parent with the ScoreMode-aggregated score. It is the Go
// port of ToParentBlockJoinQuery.BlockJoinBulkScorer (Lucene 10.4.0).
//
// Deviation from Java: Gocene's LeafCollector.SetScorer takes a search.Scorer
// (not a Scorable), so the per-parent score view handed to the wrapped
// collector is a small search.Scorer (blockJoinBatchScorable) that also
// implements search.MinCompetitiveScorer to forward the hint to the real child
// scorer for ScoreMode.None/Max.
type BlockJoinBulkScorer struct {
	childBulkScorer search.BulkScorer
	scoreMode       ScoreMode
	parents         *FixedBitSet
	parentsLength   int
}

// NewBlockJoinBulkScorer builds a BlockJoinBulkScorer over a child bulk scorer.
//
// Mirrors BlockJoinBulkScorer(BulkScorer, BitSet, ScoreMode).
func NewBlockJoinBulkScorer(childBulkScorer search.BulkScorer, parents *FixedBitSet, scoreMode ScoreMode) *BlockJoinBulkScorer {
	return &BlockJoinBulkScorer{
		childBulkScorer: childBulkScorer,
		scoreMode:       scoreMode,
		parents:         parents,
		parentsLength:   parents.Length(),
	}
}

// Score scores parent documents whose children fall in [min, max), emitting one
// collect per parent with the aggregated score, and returns the next parent doc
// on or after max (or NO_MORE_DOCS once the last parent has been scored).
//
// Faithful port of BlockJoinBulkScorer.score(LeafCollector, Bits, int, int).
func (bs *BlockJoinBulkScorer) Score(collector search.LeafCollector, acceptDocs util.Bits, min, max int) (int, error) {
	if min == max {
		return bs.scoringCompleteCheck(max, max), nil
	}

	// Subtract one because max is exclusive w.r.t. score but inclusive w.r.t.
	// PrevSetBit.
	clampedMax := bs.parentsLength
	if max < clampedMax {
		clampedMax = max
	}
	lastParent := bs.parents.PrevSetBit(clampedMax - 1)
	prevParent := -1
	if min != 0 {
		prevParent = bs.parents.PrevSetBit(min - 1)
	}
	if lastParent == prevParent {
		// No parent docs in this range.
		return bs.scoringCompleteCheck(max, max), nil
	}

	wrapped := bs.wrapCollector(collector)
	if _, err := bs.childBulkScorer.Score(wrapped, acceptDocs, prevParent+1, lastParent+1); err != nil {
		return 0, err
	}
	if err := wrapped.endBatch(); err != nil {
		return 0, err
	}

	return bs.scoringCompleteCheck(lastParent+1, max), nil
}

// scoringCompleteCheck returns NO_MORE_DOCS once the last parent in the bit set
// has been scored. Mirrors BlockJoinBulkScorer.scoringCompleteCheck.
func (bs *BlockJoinBulkScorer) scoringCompleteCheck(innerMax, returnedMax int) int {
	if innerMax >= bs.parentsLength {
		return search.NO_MORE_DOCS
	}
	return returnedMax
}

// Cost delegates to the child bulk scorer. Mirrors BlockJoinBulkScorer.cost().
func (bs *BlockJoinBulkScorer) Cost() int64 {
	return bs.childBulkScorer.Cost()
}

// wrapCollector builds the per-batch BatchAwareLeafCollector that maps child
// collect calls onto parent collect calls with the aggregated score.
func (bs *BlockJoinBulkScorer) wrapCollector(collector search.LeafCollector) *batchAwareLeafCollector {
	return &batchAwareLeafCollector{
		in:                 collector,
		parents:            bs.parents,
		scoreMode:          bs.scoreMode,
		currentParentScore: newBlockJoinScore(bs.scoreMode),
		currentParent:      -1,
	}
}

// batchAwareLeafCollector wraps the outer collector and re-maps the child
// document stream into parent collect calls, aggregating per-parent scores.
// It is the Go port of the anonymous BatchAwareLeafCollector returned by
// BlockJoinBulkScorer.wrapCollector (Lucene 10.4.0).
type batchAwareLeafCollector struct {
	in                 search.LeafCollector
	parents            *FixedBitSet
	scoreMode          ScoreMode
	currentParentScore *blockJoinScore
	currentParent      int
	scorer             search.Scorer
}

// SetScorer records the real child scorer and forwards a per-parent score view
// to the outer collector. Mirrors the anonymous setScorer override: the view's
// Score() returns the current parent's aggregated score, and its
// SetMinCompetitiveScore forwards to the child scorer for None/Max.
func (c *batchAwareLeafCollector) SetScorer(scorer search.Scorer) error {
	if scorer == nil {
		return fmt.Errorf("BlockJoinBulkScorer: child scorer must not be nil")
	}
	c.scorer = scorer
	return c.in.SetScorer(&blockJoinBatchScorable{collector: c})
}

// Collect maps a child doc onto its parent, emitting the previous parent when a
// new parent block starts and accumulating child scores otherwise. Mirrors the
// anonymous collect override.
func (c *batchAwareLeafCollector) Collect(doc int) error {
	switch {
	case doc > c.currentParent:
		// Emit the current parent and set up scoring for the next parent.
		if c.currentParent >= 0 {
			if err := c.in.Collect(c.currentParent); err != nil {
				return err
			}
		}
		c.currentParent = c.parents.NextSetBit(doc)
		c.currentParentScore.reset(c.scorer)
	case doc == c.currentParent:
		return fmt.Errorf("%s%d, %T", childMatchesParentMessage, doc, c.scorer)
	default:
		c.currentParentScore.addChildScore(c.scorer)
	}
	return nil
}

// endBatch emits the final parent accumulated during the batch. Mirrors the
// anonymous endBatch override.
func (c *batchAwareLeafCollector) endBatch() error {
	if c.currentParent >= 0 {
		return c.in.Collect(c.currentParent)
	}
	return nil
}

// blockJoinBatchScorable is the per-parent score view handed to the outer
// collector. Its Score() returns the current parent's aggregated score; its
// SetMinCompetitiveScore forwards to the real child scorer for None/Max.
// Iteration methods are inert because the bulk scorer drives document
// iteration directly.
type blockJoinBatchScorable struct {
	search.BaseDocIdSetIterator
	collector *batchAwareLeafCollector
}

func (s *blockJoinBatchScorable) Score() float32 {
	return s.collector.currentParentScore.value()
}

func (s *blockJoinBatchScorable) GetMaxScore(_ int) float32 {
	return s.collector.currentParentScore.value()
}

// AdvanceShallow returns search.NO_MORE_DOCS, the default defined by
// org.apache.lucene.search.Scorer#advanceShallow. This batch scorable does not
// expose per-block impact information.
func (s *blockJoinBatchScorable) AdvanceShallow(target int) (int, error) {
	return search.NO_MORE_DOCS, nil
}

// SetMinCompetitiveScore forwards the hint to the real child scorer only for
// ScoreMode.None/Max (mirrors the anonymous Scorable.setMinCompetitiveScore).
func (s *blockJoinBatchScorable) SetMinCompetitiveScore(minScore float32) error {
	if s.collector.scoreMode == None || s.collector.scoreMode == Max {
		if mc, ok := s.collector.scorer.(search.MinCompetitiveScorer); ok {
			return mc.SetMinCompetitiveScore(minScore)
		}
	}
	return nil
}

var (
	_ search.BulkScorer           = (*BlockJoinBulkScorer)(nil)
	_ search.LeafCollector        = (*batchAwareLeafCollector)(nil)
	_ search.Scorer               = (*blockJoinBatchScorable)(nil)
	_ search.MinCompetitiveScorer = (*blockJoinBatchScorable)(nil)
)
