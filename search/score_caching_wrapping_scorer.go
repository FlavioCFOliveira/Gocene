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
