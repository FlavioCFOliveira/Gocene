// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
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
	enum    index.ImpactsSource
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

// newLazyIndexImpactsSource wraps an index.ImpactsSource without materialising
// the first Impacts snapshot. This is the shape Lucene's ExactPhraseMatcher
// needs: its constructor hands mergeImpacts(...) straight to
// new MaxScoreCache(impactsSource, scorer) without reading it, and the snapshot
// is first read when ImpactsDISI shallow-advances. Since the Java constructor
// does not throw, the Go constructor must not be able to fail either.
func newLazyIndexImpactsSource(src index.ImpactsSource) *indexImpactsSource {
	return &indexImpactsSource{enum: src}
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

// simImpactScorer adapts a [SimScorer] to the ImpactSimScorer contract
// (Score(freq, norm)) consumed by MaxScoreCache.
//
// Faithfulness note. Lucene's MaxScoreCache scores impacts through
// Similarity.SimScorer.score(float freq, long norm); Gocene spells that method
// Score104(freq, norm) (see search/similarity.go). This adapter is therefore a
// pure name bridge between the two surfaces: the block-max upper bound is
// computed with exactly the scoring function the live scorer uses, preserving
// the getMaxScore >= score invariant.
type simImpactScorer struct {
	sim SimScorer
}

// newSimImpactScorer wraps a SimScorer. A nil sim yields a scorer whose Score
// always returns 0, matching TermScorer.Score's nil-sim behaviour where it
// would otherwise return the constant 1.0 — here 0 is the safe lower bound
// that never lets GetMaxScore exceed the (constant) live score, but the real
// wiring always supplies a non-nil sim when scores are needed.
func newSimImpactScorer(sim SimScorer) *simImpactScorer {
	return &simImpactScorer{sim: sim}
}

// Score returns the similarity score for the given impact frequency and
// encoded norm.
//
// Mirrors the Similarity.SimScorer.score(freq, norm) call made by
// MaxScoreCache.getMaxScoreForLevel.
func (s *simImpactScorer) Score(freq float32, norm int64) float32 {
	if s.sim == nil {
		return 0
	}
	// Guard against a non-finite freq (e.g. a future impact encoding using a
	// sentinel): clamp to the largest finite float32 so the score stays a real
	// upper bound rather than propagating NaN/Inf.
	if math.IsInf(float64(freq), 0) || math.IsNaN(float64(freq)) {
		freq = math.MaxFloat32
	}
	return s.sim.Score104(freq, norm)
}

// Compile-time assertion: simImpactScorer satisfies ImpactSimScorer.
var _ ImpactSimScorer = (*simImpactScorer)(nil)

// spiImpactsSource adapts an spi.ImpactsSource to the search.ImpactsSource
// contract consumed by MaxScoreCache.
//
// It exists alongside indexImpactsSource because Gocene declares the Lucene
// class org.apache.lucene.index.Impacts twice — once in index (whose
// GetImpacts(level) returns *index.FreqAndNormBuffer) and once in spi (whose
// GetImpacts(level) returns *util.FreqAndNormBuffer). The two are structurally
// identical but are distinct Go types, so a single adapter cannot serve both.
// TermsEnum.Impacts hands back the spi flavour; SlowImpactsEnum the index one.
type spiImpactsSource struct {
	enum    spi.ImpactsSource
	impacts spi.Impacts // snapshot refreshed on each AdvanceShallow
	scratch []Impact    // reused conversion buffer (allocation-free hot path)
}

// newSPIImpactsSource wraps an spi.ImpactsSource. It eagerly materialises the
// first Impacts snapshot so that NumLevels/GetDocIDUpTo/GetImpacts are valid
// before any AdvanceShallow call, matching Lucene where impactsSource.getImpacts
// is always callable once the enum is positioned.
func newSPIImpactsSource(enum spi.ImpactsSource) (*spiImpactsSource, error) {
	s := &spiImpactsSource{enum: enum}
	if err := s.refresh(); err != nil {
		return nil, err
	}
	return s, nil
}

// refresh re-reads the current Impacts snapshot from the underlying enum.
func (s *spiImpactsSource) refresh() error {
	imp, err := s.enum.GetImpacts()
	if err != nil {
		return err
	}
	s.impacts = imp
	return nil
}

// AdvanceShallow shallow-advances the underlying source to target, refreshes the
// Impacts snapshot, and returns the inclusive upper doc id of the level-0 block.
// Mirrors MaxScoreCache.advanceShallow.
func (s *spiImpactsSource) AdvanceShallow(target int) (int, error) {
	if err := s.enum.AdvanceShallow(target); err != nil {
		return 0, err
	}
	if err := s.refresh(); err != nil {
		return 0, err
	}
	return s.GetDocIDUpTo(0), nil
}

// NumLevels returns the number of impact levels in the current snapshot.
func (s *spiImpactsSource) NumLevels() int {
	if s.impacts == nil {
		return 0
	}
	return s.impacts.NumLevels()
}

// GetDocIDUpTo returns the inclusive upper doc id for level.
func (s *spiImpactsSource) GetDocIDUpTo(level int) int {
	if s.impacts == nil {
		return NO_MORE_DOCS
	}
	return s.impacts.GetDocIDUpTo(level)
}

// GetImpacts converts the level's (freq, norm) buffer into a []search.Impact,
// reusing the scratch slice across calls.
func (s *spiImpactsSource) GetImpacts(level int) []Impact {
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

// Compile-time assertion: spiImpactsSource satisfies the search ImpactsSource
// contract consumed by MaxScoreCache.
var _ ImpactsSource = (*spiImpactsSource)(nil)
