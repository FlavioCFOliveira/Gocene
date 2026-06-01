// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// ScoreCachingWrappingScorer wraps a Scorer and caches the score of the
// current document. Repeated Score() calls return the cached value until the
// underlying scorer advances to a new document.
//
// Mirrors org.apache.lucene.search.ScoreCachingWrappingScorer.
type ScoreCachingWrappingScorer struct {
	inner   Scorer
	cached  bool
	score   float32
	lastDoc int
}

// WrapScoreCachingScorer returns a ScoreCachingWrappingScorer around inner.
// If inner is already a ScoreCachingWrappingScorer, it is returned unchanged.
func WrapScoreCachingScorer(inner Scorer) Scorer {
	if inner == nil {
		return nil
	}
	if w, ok := inner.(*ScoreCachingWrappingScorer); ok {
		return w
	}
	return &ScoreCachingWrappingScorer{inner: inner, lastDoc: -2}
}

// DocID delegates to the inner scorer and invalidates the cached score when
// the document changes.
func (s *ScoreCachingWrappingScorer) DocID() int {
	d := s.inner.DocID()
	if d != s.lastDoc {
		s.cached = false
		s.lastDoc = d
	}
	return d
}

// NextDoc advances the inner scorer and invalidates the cache.
func (s *ScoreCachingWrappingScorer) NextDoc() (int, error) {
	d, err := s.inner.NextDoc()
	s.cached = false
	s.lastDoc = d
	return d, err
}

// Advance positions the inner scorer at target and invalidates the cache.
func (s *ScoreCachingWrappingScorer) Advance(target int) (int, error) {
	d, err := s.inner.Advance(target)
	s.cached = false
	s.lastDoc = d
	return d, err
}

// Cost forwards to the inner scorer.
func (s *ScoreCachingWrappingScorer) Cost() int64 { return s.inner.Cost() }

// DocIDRunEnd forwards to the inner scorer so callers that exploit
// consecutive-run skipping continue to work.
func (s *ScoreCachingWrappingScorer) DocIDRunEnd() int { return s.inner.DocIDRunEnd() }

// Score returns the cached value if available, otherwise computes and caches it.
func (s *ScoreCachingWrappingScorer) Score() float32 {
	if !s.cached {
		s.score = s.inner.Score()
		s.cached = true
	}
	return s.score
}

// GetMaxScore forwards to the inner scorer.
func (s *ScoreCachingWrappingScorer) GetMaxScore(upTo int) float32 {
	return s.inner.GetMaxScore(upTo)
}

// AdvanceShallow forwards to the inner scorer so block boundaries and block-max
// upper bounds match the wrapped scorer.
func (s *ScoreCachingWrappingScorer) AdvanceShallow(target int) (int, error) {
	return s.inner.AdvanceShallow(target)
}

// invalidate drops the cached score so the next Score() recomputes it. This is
// used by scoreCachingLeafCollector to reset the cache on each collected
// document, mirroring the scoreIsCached=false reset in Lucene's
// ScoreCachingWrappingLeafCollector.collect.
func (s *ScoreCachingWrappingScorer) invalidate() {
	s.cached = false
}

// scoreCachingLeafCollector wraps a LeafCollector so that the Scorer it
// receives is a ScoreCachingWrappingScorer, computing scores lazily and caching
// them across the (possibly several) child collectors that read them for the
// same document.
//
// This is the Go port of
// org.apache.lucene.search.ScoreCachingWrappingScorer.ScoreCachingWrappingLeafCollector
// (obtained via ScoreCachingWrappingScorer.wrap(LeafCollector)).
type scoreCachingLeafCollector struct {
	in     LeafCollector
	scorer *ScoreCachingWrappingScorer
}

// newScoreCachingLeafCollector wraps in so scores are cached. If in is already
// a scoreCachingLeafCollector it is returned unchanged, matching Lucene's wrap.
func newScoreCachingLeafCollector(in LeafCollector) LeafCollector {
	if w, ok := in.(*scoreCachingLeafCollector); ok {
		return w
	}
	return &scoreCachingLeafCollector{in: in}
}

// SetScorer wraps the incoming scorer in a ScoreCachingWrappingScorer and
// forwards that to the inner leaf collector.
func (c *scoreCachingLeafCollector) SetScorer(scorer Scorer) error {
	c.scorer = &ScoreCachingWrappingScorer{inner: scorer, lastDoc: -2}
	return c.in.SetScorer(c.scorer)
}

// Collect invalidates the per-document cache before delegating, so each new
// document recomputes its score on first access.
func (c *scoreCachingLeafCollector) Collect(doc int) error {
	if c.scorer != nil {
		c.scorer.invalidate()
	}
	return c.in.Collect(doc)
}

// Finish forwards to the inner leaf collector when it supports finishing,
// preserving the MultiCollector terminate-and-drain semantics through the
// caching wrapper.
func (c *scoreCachingLeafCollector) Finish() error {
	if f, ok := c.in.(leafCollectorFinisher); ok {
		return f.Finish()
	}
	return nil
}

// Ensure scoreCachingLeafCollector implements LeafCollector and the optional
// finisher.
var (
	_ LeafCollector         = (*scoreCachingLeafCollector)(nil)
	_ leafCollectorFinisher = (*scoreCachingLeafCollector)(nil)
)
