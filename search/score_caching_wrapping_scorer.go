// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// ScoreCachingWrappingScorer is a Scorable which wraps another one and caches
// the score of the current document. Successive calls to Score() return the
// same result and do not invoke the wrapped Scorable's score(), unless the
// current document has changed.
//
// Mirrors the final class org.apache.lucene.search.ScoreCachingWrappingScorer,
// which extends Scorable — not Scorer — and therefore declares exactly
// score(), setMinCompetitiveScore(float) and getChildren(), plus the static
// wrap(LeafCollector) factory.
type ScoreCachingWrappingScorer struct {
	BaseScorable
	scoreIsCached bool
	curScore      float32
	in            Scorable
}

// newScoreCachingWrappingScorer mirrors the private constructor
// ScoreCachingWrappingScorer(Scorable).
func newScoreCachingWrappingScorer(scorer Scorable) *ScoreCachingWrappingScorer {
	return &ScoreCachingWrappingScorer{in: scorer}
}

// WrapScoreCachingLeafCollector wraps the provided LeafCollector so that scores
// are computed lazily and cached if accessed multiple times.
//
// Mirrors the static ScoreCachingWrappingScorer.wrap(LeafCollector).
func WrapScoreCachingLeafCollector(collector LeafCollector) LeafCollector {
	if w, ok := collector.(*scoreCachingWrappingLeafCollector); ok {
		return w
	}
	return &scoreCachingWrappingLeafCollector{in: collector}
}

// Score returns the cached value if available, otherwise computes and caches it.
//
// Mirrors ScoreCachingWrappingScorer.score().
func (s *ScoreCachingWrappingScorer) Score() (float32, error) {
	if !s.scoreIsCached {
		sc, err := s.in.Score()
		if err != nil {
			return 0, err
		}
		s.curScore = sc
		s.scoreIsCached = true
	}
	return s.curScore, nil
}

// SetMinCompetitiveScore mirrors
// ScoreCachingWrappingScorer.setMinCompetitiveScore(float).
func (s *ScoreCachingWrappingScorer) SetMinCompetitiveScore(minScore float32) error {
	return s.in.SetMinCompetitiveScore(minScore)
}

// GetChildren mirrors ScoreCachingWrappingScorer.getChildren(), whose body is
// Collections.singleton(new ChildScorable(in, "CACHED")).
func (s *ScoreCachingWrappingScorer) GetChildren() ([]ChildScorable, error) {
	return []ChildScorable{{Child: s.in, Relationship: "CACHED"}}, nil
}

// scoreCachingWrappingLeafCollector wraps a LeafCollector so that the Scorer it
// receives is a ScoreCachingWrappingScorer, computing scores lazily and caching
// them across the (possibly several) child collectors that read them for the
// same document.
//
// This is the Go port of
// org.apache.lucene.search.ScoreCachingWrappingScorer.ScoreCachingWrappingLeafCollector
// (obtained via ScoreCachingWrappingScorer.wrap(LeafCollector)).
type scoreCachingWrappingLeafCollector struct {
	BaseLeafCollector
	in     LeafCollector
	scorer *ScoreCachingWrappingScorer
}

// newScoreCachingLeafCollector is the package-internal spelling of
// WrapScoreCachingLeafCollector, kept for the existing MultiCollector call site.
func newScoreCachingLeafCollector(in LeafCollector) LeafCollector {
	return WrapScoreCachingLeafCollector(in)
}

// SetScorer wraps the incoming scorer in a ScoreCachingWrappingScorer and
// forwards that to the inner leaf collector.
func (c *scoreCachingWrappingLeafCollector) SetScorer(scorer Scorable) error {
	c.scorer = newScoreCachingWrappingScorer(scorer)
	return c.in.SetScorer(c.scorer)
}

// Collect invalidates the per-document cache before delegating, so each new
// document recomputes its score on first access.
func (c *scoreCachingWrappingLeafCollector) Collect(doc int) error {
	if c.scorer != nil {
		// Invalidate cache when collecting a new doc
		c.scorer.scoreIsCached = false
	}
	return c.in.Collect(doc)
}

// Finish forwards to the inner leaf collector when it supports finishing,
// preserving the MultiCollector terminate-and-drain semantics through the
// caching wrapper.
func (c *scoreCachingWrappingLeafCollector) Finish() error {
	if f, ok := c.in.(leafCollectorFinisher); ok {
		return f.Finish()
	}
	return nil
}

// Ensure scoreCachingWrappingLeafCollector implements LeafCollector and the optional
// finisher.
var (
	_ LeafCollector         = (*scoreCachingWrappingLeafCollector)(nil)
	_ leafCollectorFinisher = (*scoreCachingWrappingLeafCollector)(nil)
)

// CollectRange mirrors the default body of LeafCollector.collectRange(int, int)
// in Apache Lucene 10.5.0.
func (s *scoreCachingWrappingLeafCollector) CollectRange(min, max int) error {
	return DefaultCollectRange(s, min, max)
}

// CollectStream mirrors the default body of LeafCollector.collect(DocIdStream)
// in Apache Lucene 10.5.0.
func (s *scoreCachingWrappingLeafCollector) CollectStream(stream DocIdStream) error {
	return DefaultCollectStream(s, stream)
}

// Apache Lucene 10.5.0 declares ScoreCachingWrappingScorer as a Scorable, not
// as a Scorer: it therefore has no iterator(), no nextDocsAndScores and no
// intoBitSet. The three members that used to sit here had no counterpart in the
// Java class.
