// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// indexImpactsSource adapts an index.ImpactsEnum to the search.ImpactsSource
// contract consumed by MaxScoreCache. It bridges two differences between the
// index and search layers:
//
//  1. Sentinel translation. index.Impacts.GetDocIDUpTo returns the index
//     exhaustion sentinel index.NO_MORE_DOCS (-1) when a level covers the
//     remaining postings list (this is what SlowImpactsEnum reports). The
//     search-side MaxScoreCache compares the returned value against an upTo
//     expressed in the search doc-id space, where the exhaustion sentinel is
//     search.NO_MORE_DOCS (math.MaxInt32). Without translation, a level that
//     covers "everything" (-1) would never be selected, so this adapter maps
//     index.NO_MORE_DOCS -> search.NO_MORE_DOCS.
//
//  2. Buffer shape. index exposes per-level (freq, norm) pairs through a
//     *index.FreqAndNormBuffer (parallel slices honouring the Size invariant);
//     MaxScoreCache consumes them as a []search.Impact. GetImpacts performs
//     the conversion against a reused scratch slice to keep the hot path
//     allocation-free across repeated calls within the same block.
//
// This mirrors how Lucene's org.apache.lucene.search.MaxScoreCache consumes
// org.apache.lucene.index.Impacts directly: advanceShallow shallow-advances the
// ImpactsEnum, refreshes the Impacts snapshot, and reports getDocIdUpTo(0) as
// the block boundary.
type indexImpactsSource struct {
	enum    index.ImpactsEnum
	impacts index.Impacts // snapshot refreshed on each AdvanceShallow
	scratch []Impact      // reused conversion buffer (allocation-free hot path)
}

// newIndexImpactsSource wraps an index.ImpactsEnum. It eagerly materialises the
// first Impacts snapshot so that NumLevels/GetDocIDUpTo/GetImpacts are valid
// before any AdvanceShallow call, matching Lucene where impactsSource.getImpacts
// is always callable once the enum is positioned.
func newIndexImpactsSource(enum index.ImpactsEnum) (*indexImpactsSource, error) {
	s := &indexImpactsSource{enum: enum}
	if err := s.refresh(); err != nil {
		return nil, err
	}
	return s, nil
}

// refresh re-reads the current Impacts snapshot from the underlying enum.
func (s *indexImpactsSource) refresh() error {
	imp, err := s.enum.GetImpacts()
	if err != nil {
		return err
	}
	s.impacts = imp
	return nil
}

// AdvanceShallow shallow-advances the underlying ImpactsEnum to target, refreshes
// the Impacts snapshot, and returns the inclusive upper doc id of the level-0
// block in search doc-id space. Mirrors MaxScoreCache.advanceShallow, which
// calls impactsSource.advanceShallow(target) then returns impacts.getDocIdUpTo(0).
func (s *indexImpactsSource) AdvanceShallow(target int) (int, error) {
	if err := s.enum.AdvanceShallow(target); err != nil {
		return 0, err
	}
	if err := s.refresh(); err != nil {
		return 0, err
	}
	return s.docIDUpTo(0), nil
}

// NumLevels returns the number of impact levels in the current snapshot.
func (s *indexImpactsSource) NumLevels() int {
	if s.impacts == nil {
		return 0
	}
	return s.impacts.NumLevels()
}

// GetDocIDUpTo returns the inclusive upper doc id for level in search doc-id space.
func (s *indexImpactsSource) GetDocIDUpTo(level int) int {
	return s.docIDUpTo(level)
}

// docIDUpTo reads the index-space upTo for level and translates the index
// exhaustion sentinel (index.NO_MORE_DOCS) to the search exhaustion sentinel.
func (s *indexImpactsSource) docIDUpTo(level int) int {
	if s.impacts == nil {
		return NO_MORE_DOCS
	}
	upTo := s.impacts.GetDocIDUpTo(level)
	if upTo == index.NO_MORE_DOCS {
		return NO_MORE_DOCS
	}
	return upTo
}

// GetImpacts converts the level's (freq, norm) buffer into a []search.Impact,
// reusing the scratch slice across calls.
func (s *indexImpactsSource) GetImpacts(level int) []Impact {
	if s.impacts == nil {
		return nil
	}
	buf := s.impacts.GetImpacts(level)
	if buf == nil {
		return nil
	}
	n := buf.Size
	if cap(s.scratch) < n {
		s.scratch = make([]Impact, n)
	} else {
		s.scratch = s.scratch[:n]
	}
	for i := 0; i < n; i++ {
		s.scratch[i] = Impact{Freq: buf.Freqs[i], Norm: buf.Norms[i]}
	}
	return s.scratch
}

// Compile-time assertion: indexImpactsSource satisfies the search ImpactsSource
// contract consumed by MaxScoreCache.
var _ ImpactsSource = (*indexImpactsSource)(nil)

// legacySimImpactScorer adapts the legacy [SimScorer] (Score(doc, freq)) to the
// ImpactSimScorer contract (Score(freq, norm)) consumed by MaxScoreCache.
//
// Faithfulness note. Lucene's MaxScoreCache scores impacts through
// Similarity.SimScorer.score(freq, norm), and TermScorer.score applies the
// per-document norm. Gocene's TermWeight/TermScorer use the legacy SimScorer
// surface, whose Score(doc, freq) deliberately does NOT apply norms (see
// term_weight.go: "Norms are not consulted because the legacy scoring path does
// not apply them"). The block-max upper bound MUST be computed with the very
// same scoring function the scorer uses for live documents, otherwise it could
// under-estimate and violate the getMaxScore >= score invariant. Therefore this
// adapter:
//
//   - ignores norm (exactly as the live Score path does), and
//   - is monotonically non-decreasing in freq for the legacy similarities used
//     here (ClassicSimScorer: sqrt(freq)*idf*boost; BaseSimScorer: constant 1),
//     so feeding the per-block maximum freq yields the per-block maximum score.
//
// The doc argument is irrelevant to these legacy scorers (they ignore it), so a
// fixed placeholder doc is passed. This makes GetMaxScore a correct (and, for
// real codec impacts, tight) upper bound while staying byte-faithful to how the
// legacy path scores live documents.
type legacySimImpactScorer struct {
	sim SimScorer
}

// newLegacySimImpactScorer wraps a legacy SimScorer. A nil sim yields a scorer
// whose Score always returns 0, matching TermScorer.Score's nil-sim behaviour
// where it would otherwise return the constant 1.0 — here 0 is the safe lower
// bound that never lets GetMaxScore exceed the (constant) live score, but the
// real wiring always supplies a non-nil sim when scores are needed.
func newLegacySimImpactScorer(sim SimScorer) *legacySimImpactScorer {
	return &legacySimImpactScorer{sim: sim}
}

// Score returns the legacy similarity score for the given impact frequency,
// ignoring norm to match the legacy live-scoring path. The negative placeholder
// doc is never consulted by the legacy similarities.
func (s *legacySimImpactScorer) Score(freq float32, _ int64) float32 {
	if s.sim == nil {
		return 0
	}
	// Guard against a non-finite freq (e.g. a future impact encoding using a
	// sentinel): clamp to the largest finite float32 so the score stays a real
	// upper bound rather than propagating NaN/Inf.
	if math.IsInf(float64(freq), 0) || math.IsNaN(float64(freq)) {
		freq = math.MaxFloat32
	}
	return s.sim.Score(impactScorerPlaceholderDoc, freq)
}

// impactScorerPlaceholderDoc is the doc id handed to the legacy SimScorer when
// computing block-max scores. Legacy similarities ignore the doc id (they do
// not apply norms), so any value is safe; -1 is used to signal "no live doc".
const impactScorerPlaceholderDoc = -1

// Compile-time assertion: legacySimImpactScorer satisfies ImpactSimScorer.
var _ ImpactSimScorer = (*legacySimImpactScorer)(nil)
