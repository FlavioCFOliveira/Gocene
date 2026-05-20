// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/DisjunctionMaxBulkScorer.java

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// dmWindowSize is the number of documents scored per window, same as BooleanScorer.
const dmWindowSize = 4096

// disjunctionMaxBulkScorer is a BulkScorer for DisjunctionMaxQuery when the
// tie-break multiplier is zero.  It processes documents in fixed-size windows
// to amortise per-document overhead.
//
// Ported from org.apache.lucene.search.DisjunctionMaxBulkScorer (package-private).
type disjunctionMaxBulkScorer struct {
	// windowMatches records which relative positions within the current window
	// have at least one matching document.  Size is dmWindowSize+1 to ease
	// nextSetBit iteration past the last bit.
	windowMatches *util.FixedBitSet

	// windowScores accumulates the running per-document maximum score.
	windowScores []float32

	// scorers is a min-heap ordered by the next document each sub-scorer will
	// produce; the head is always the scorer with the smallest next doc.
	scorers *util.PriorityQueue[*dmBulkScorerAndNext]

	// topLevelScorer is the Scorer presented to the outer collector;
	// its current score is updated to the aggregated window score before each
	// collect call.
	topLevelScorer *dmSimpleScorer

	// totalCost is the sum of sub-scorer costs, cached at construction.
	totalCost int64
}

// dmBulkScorerAndNext pairs a windowed sub-BulkScorer with the next document
// it will produce (the return value of the last ScoreWindow call).
type dmBulkScorerAndNext struct {
	scorer windowedBulkScorer
	next   int
}

// windowedBulkScorer is the internal windowed scoring contract, matching
// Lucene's BulkScorer.score(LeafCollector, Bits, int, int) -> int.
// The return value is the first document beyond max that the scorer has
// advanced to, or NO_MORE_DOCS when exhausted.
type windowedBulkScorer interface {
	// ScoreWindow scores documents in [min, max), delivering them to
	// collector (which may filter via acceptDocs), and returns the first
	// document the scorer advanced to at or beyond max.
	ScoreWindow(collector LeafCollector, acceptDocs util.Bits, min, max int) (int, error)

	// Cost returns an upper bound on the number of documents this scorer may
	// produce.
	Cost() int64
}

// dmSimpleScorer is a minimal Scorer that wraps a mutable score and satisfies
// Gocene's LeafCollector.SetScorer(Scorer) contract.  The DocIdSetIterator
// methods are no-ops because the replay phase in disjunctionMaxBulkScorer
// drives document iteration directly.
type dmSimpleScorer struct {
	BaseDocIdSetIterator
	score               float32
	minCompetitiveScore float32
}

// Score returns the most recently set score.
func (s *dmSimpleScorer) Score() float32 { return s.score }

// GetMaxScore returns the most recently set score as the upper bound.
func (s *dmSimpleScorer) GetMaxScore(_ int) float32 { return s.score }

// SetMinCompetitiveScore stores the hint for propagation to sub-scorers.
func (s *dmSimpleScorer) SetMinCompetitiveScore(minScore float32) error {
	s.minCompetitiveScore = minScore
	return nil
}

// newDisjunctionMaxBulkScorer creates a disjunctionMaxBulkScorer from at
// least two windowed sub-scorers.
func newDisjunctionMaxBulkScorer(scorers []windowedBulkScorer) (*disjunctionMaxBulkScorer, error) {
	if len(scorers) < 2 {
		panic("disjunctionMaxBulkScorer requires at least 2 sub-scorers")
	}

	windowMatches, err := util.NewFixedBitSet(dmWindowSize + 1)
	if err != nil {
		return nil, err
	}

	pq, err := util.NewPriorityQueue[*dmBulkScorerAndNext](
		len(scorers),
		func(a, b *dmBulkScorerAndNext) bool { return a.next < b.next },
	)
	if err != nil {
		return nil, err
	}

	var totalCost int64
	for _, sc := range scorers {
		totalCost += sc.Cost()
		pq.Add(&dmBulkScorerAndNext{scorer: sc, next: 0})
	}

	return &disjunctionMaxBulkScorer{
		windowMatches:  windowMatches,
		windowScores:   make([]float32, dmWindowSize),
		scorers:        pq,
		topLevelScorer: &dmSimpleScorer{},
		totalCost:      totalCost,
	}, nil
}

// scoreRange scores documents in [min, max) and delivers each matching
// document to collector, returning the first document at or beyond max the
// head scorer advanced to.
//
// This mirrors org.apache.lucene.search.DisjunctionMaxBulkScorer.score().
func (d *disjunctionMaxBulkScorer) scoreRange(
	collector LeafCollector,
	acceptDocs util.Bits,
	min, max int,
) (int, error) {
	top := d.scorers.Top()

	for top.next < max {
		windowMin := top.next
		if windowMin < min {
			windowMin = min
		}
		windowMax := int(util.MathUnsignedMin(int32(max), int32(windowMin+dmWindowSize)))

		// Phase 1: fill the window arrays with matches and max-scores from
		// each sub-scorer whose next doc falls within [windowMin, windowMax).
		innerCollector := &dmWindowLeafCollector{
			windowMin:     windowMin,
			windowMatches: d.windowMatches,
			windowScores:  d.windowScores,
			minCompScore:  d.topLevelScorer.minCompetitiveScore,
		}
		for {
			next, err := top.scorer.ScoreWindow(innerCollector, acceptDocs, windowMin, windowMax)
			if err != nil {
				return 0, err
			}
			top.next = next
			d.scorers.UpdateTop()
			top = d.scorers.Top()
			if top.next >= windowMax {
				break
			}
		}

		// Phase 2: replay matches to the outer collector.
		if err := collector.SetScorer(d.topLevelScorer); err != nil {
			return 0, err
		}
		for windowDoc := d.windowMatches.NextSetBit(0); windowDoc >= 0; windowDoc = d.windowMatches.NextSetBit(windowDoc + 1) {
			if windowDoc >= dmWindowSize+1 {
				break
			}
			doc := windowMin + windowDoc
			d.topLevelScorer.score = d.windowScores[windowDoc]
			if err := collector.Collect(doc); err != nil {
				return 0, err
			}
		}

		// Phase 3: clear window state for next iteration.
		d.windowMatches.ClearAll()
		for i := range d.windowScores {
			d.windowScores[i] = 0
		}
	}

	return top.next, nil
}

// Cost returns the sum of costs of all sub-scorers.
func (d *disjunctionMaxBulkScorer) Cost() int64 {
	return d.totalCost
}

// dmWindowLeafCollector is used during the window-fill phase.  It records
// the maximum score per window slot and which slots have at least one hit.
type dmWindowLeafCollector struct {
	windowMin     int
	windowMatches *util.FixedBitSet
	windowScores  []float32
	minCompScore  float32
	innerScorer   Scorer
}

// SetScorer stores the inner scorer for score retrieval during Collect.
func (w *dmWindowLeafCollector) SetScorer(scorer Scorer) error {
	w.innerScorer = scorer
	return nil
}

// Collect records the hit in the window arrays.
func (w *dmWindowLeafCollector) Collect(doc int) error {
	delta := doc - w.windowMin
	if delta < 0 || delta >= dmWindowSize {
		return nil
	}
	score := w.innerScorer.Score()
	w.windowMatches.Set(delta)
	if score > w.windowScores[delta] {
		w.windowScores[delta] = score
	}
	return nil
}

// Ensure dmWindowLeafCollector implements LeafCollector.
var _ LeafCollector = (*dmWindowLeafCollector)(nil)
