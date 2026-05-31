// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// MultiCollector is a Collector which allows running a search with several
// Collectors. Use MultiCollectorWrap to build one while filtering out nil
// collectors and unwrapping the trivial single-collector case.
//
// This is the Go port of org.apache.lucene.search.MultiCollector.
//
// NOTE: When mixing collectors that want to skip low-scoring hits
// (ScoreMode TOP_SCORES) with ones that require seeing all hits — such as
// mixing TopScoreDocCollector and TotalHitCountCollector — it is usually
// faster to run the query twice, once per collector, rather than using this
// wrapper on a single search.
type MultiCollector struct {
	cacheScores bool
	collectors  []Collector
}

// ErrNoCollectors is returned by MultiCollectorWrap when no non-nil collector
// is supplied. It mirrors the IllegalArgumentException Lucene throws from
// MultiCollector.wrap with the same message.
var ErrNoCollectors = errors.New("at least 1 collector must not be null")

// MultiCollectorWrap wraps the given collectors with a MultiCollector.
//
// It works like Lucene's MultiCollector.wrap:
//   - nil collectors are filtered out so they are not used during collection;
//   - if exactly one real (non-nil) collector remains, it is returned as-is
//     (no MultiCollector wrapper);
//   - otherwise a *MultiCollector wrapping the non-nil collectors is returned.
//
// It returns ErrNoCollectors if zero collectors were supplied, or all of them
// were nil.
func MultiCollectorWrap(collectors ...Collector) (Collector, error) {
	// For the caller's convenience nil collectors are allowed; they are found
	// and dropped here so they cost nothing at collection time.
	n := 0
	for _, c := range collectors {
		if c != nil {
			n++
		}
	}

	switch {
	case n == 0:
		return nil, ErrNoCollectors
	case n == 1:
		for _, c := range collectors {
			if c != nil {
				return c, nil
			}
		}
		return nil, ErrNoCollectors // unreachable, kept for completeness
	default:
		colls := make([]Collector, 0, n)
		for _, c := range collectors {
			if c != nil {
				colls = append(colls, c)
			}
		}
		return newMultiCollector(colls), nil
	}
}

// NewMultiCollector creates a MultiCollector wrapping the given collectors
// without the nil-filtering / single-collector-shortcut logic of
// MultiCollectorWrap. Prefer MultiCollectorWrap, which faithfully matches
// Lucene's MultiCollector.wrap; this constructor is retained for callers that
// already guarantee at least two non-nil collectors.
func NewMultiCollector(collectors ...Collector) *MultiCollector {
	return newMultiCollector(collectors)
}

// newMultiCollector is the private constructor shared by the factories; it
// computes cacheScores exactly as Lucene's private MultiCollector constructor
// does (cache scores when at least two children need scores).
func newMultiCollector(collectors []Collector) *MultiCollector {
	numNeedsScores := 0
	for _, c := range collectors {
		if c.ScoreMode().needsScores() {
			numNeedsScores++
		}
	}
	return &MultiCollector{
		cacheScores: numNeedsScores >= 2,
		collectors:  collectors,
	}
}

// ScoreMode merges the score modes of the wrapped collectors.
//
// This mirrors Lucene exactly: it starts from the first collector's mode and,
// on the first disagreement, collapses to COMPLETE if any of the two modes
// needs scores, or COMPLETE_NO_SCORES otherwise.
func (c *MultiCollector) ScoreMode() ScoreMode {
	var scoreMode ScoreMode
	first := true
	for _, collector := range c.collectors {
		cm := collector.ScoreMode()
		if first {
			scoreMode = cm
			first = false
		} else if scoreMode != cm {
			// If score modes disagree, do not try to be smart: use one of the
			// COMPLETE modes depending on whether scores are needed or not.
			if scoreMode.needsScores() || cm.needsScores() {
				scoreMode = COMPLETE
			} else {
				scoreMode = COMPLETE_NO_SCORES
			}
		}
	}
	if first {
		// No collectors at all; match Lucene's previous Gocene behaviour.
		return COMPLETE_NO_SCORES
	}
	return scoreMode
}

// GetLeafCollector returns a LeafCollector for the given context.
//
// Each child's GetLeafCollector is invoked; a child that returns a
// CollectionTerminatedException is skipped (it does not need this segment). If
// no child wants the segment, a CollectionTerminatedException is returned so
// the search loop skips the whole leaf. When exactly one leaf collector
// survives and the global/leaf score modes allow it, that single leaf
// collector is returned directly. Otherwise a multiLeafCollector is built and,
// when cacheScores is set (>= 2 scoring children), wrapped with a
// ScoreCachingWrappingScorer-aware leaf so children share cached scores.
func (c *MultiCollector) GetLeafCollector(context *index.LeafReaderContext) (LeafCollector, error) {
	leafCollectors := make([]LeafCollector, 0, len(c.collectors))
	var leafScoreMode ScoreMode
	haveScoreMode := false

	for _, collector := range c.collectors {
		leafCollector, err := collector.GetLeafCollector(context)
		if err != nil {
			if IsCollectionTerminated(err) {
				// This leaf collector does not need this segment.
				continue
			}
			return nil, err
		}
		if !haveScoreMode {
			leafScoreMode = collector.ScoreMode()
			haveScoreMode = true
		} else if leafScoreMode != collector.ScoreMode() {
			leafScoreMode = COMPLETE
		}
		leafCollectors = append(leafCollectors, leafCollector)
	}

	if len(leafCollectors) == 0 {
		return nil, NewCollectionTerminatedException()
	}

	scoreMode := c.ScoreMode()

	// Return the single surviving leaf collector directly when it wants to skip
	// low-scoring hits (TOP_SCORES) and the global score mode allows it, or when
	// it does not want to skip at all. Only when a single leaf wants TOP_SCORES
	// but the global mode forbids skipping must it be wrapped (so the
	// setMinCompetitiveScore call is suppressed).
	if len(leafCollectors) == 1 &&
		(scoreMode == TOP_SCORES || leafScoreMode != TOP_SCORES) {
		return leafCollectors[0], nil
	}

	var collector LeafCollector = newMultiLeafCollector(leafCollectors, scoreMode == TOP_SCORES)
	if c.cacheScores {
		collector = newScoreCachingLeafCollector(collector)
	}
	return collector, nil
}

// GetCollectors returns the wrapped collectors. Provided for advanced use
// cases, mirroring Lucene's MultiCollector.getCollectors.
func (c *MultiCollector) GetCollectors() []Collector {
	return c.collectors
}

// Ensure MultiCollector implements Collector.
var _ Collector = (*MultiCollector)(nil)

// multiLeafCollector forwards setScorer/collect to a set of child leaf
// collectors, dropping any child that terminates and signalling
// CollectionTerminatedException once they have all terminated.
//
// This is the Go port of MultiCollector.MultiLeafCollector.
type multiLeafCollector struct {
	collectors               []LeafCollector
	minScores                []float32
	skipNonCompetitiveScores bool
}

// newMultiLeafCollector builds a multiLeafCollector. When skipNonCompetitive is
// true the per-child minScores slice is allocated so setScorer can wire up
// MinCompetitiveScoreAwareScorers.
func newMultiLeafCollector(collectors []LeafCollector, skipNonCompetitive bool) *multiLeafCollector {
	m := &multiLeafCollector{
		collectors:               collectors,
		skipNonCompetitiveScores: skipNonCompetitive,
	}
	if skipNonCompetitive {
		m.minScores = make([]float32, len(collectors))
	}
	return m
}

// SetScorer forwards the scorer to every live child.
//
// When skipNonCompetitiveScores is set, each child receives its own
// minCompetitiveScoreAwareScorer so the shared minimum competitive score is
// only raised once all children agree (port of
// MultiCollector.MinCompetitiveScoreAwareScorable). Otherwise the scorer is
// wrapped so setMinCompetitiveScore is ignored — that way, if one wrapped
// collector wants to skip low-scoring hits, the others still see all hits
// (port of the FilterScorable used in Lucene's else branch).
func (m *multiLeafCollector) SetScorer(scorer Scorer) error {
	if m.skipNonCompetitiveScores {
		for i, c := range m.collectors {
			if c != nil {
				wrapped := newMinCompetitiveScoreAwareScorer(scorer, i, m.minScores)
				if err := c.SetScorer(wrapped); err != nil {
					return err
				}
			}
		}
		return nil
	}

	wrapped := newIgnoreMinCompetitiveScorer(scorer)
	for _, c := range m.collectors {
		if c != nil {
			if err := c.SetScorer(wrapped); err != nil {
				return err
			}
		}
	}
	return nil
}

// Collect forwards the document to every live child. A child that returns a
// CollectionTerminatedException is finished (if it supports finishing) and
// dropped; when all children have terminated a CollectionTerminatedException is
// returned so the caller stops collecting on this leaf.
func (m *multiLeafCollector) Collect(doc int) error {
	for i := range m.collectors {
		collector := m.collectors[i]
		if collector == nil {
			continue
		}
		if err := collector.Collect(doc); err != nil {
			if IsCollectionTerminated(err) {
				if err := finishLeafCollector(collector); err != nil {
					return err
				}
				m.collectors[i] = nil
				if m.allCollectorsTerminated() {
					return NewCollectionTerminatedException()
				}
				continue
			}
			return err
		}
	}
	return nil
}

// Finish forwards to every live child that supports finishing, matching
// Lucene's MultiLeafCollector.finish. It is exposed so callers that hold a
// multiLeafCollector through the optional finisher interface can drain it.
func (m *multiLeafCollector) Finish() error {
	for _, collector := range m.collectors {
		if collector != nil {
			if err := finishLeafCollector(collector); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *multiLeafCollector) allCollectorsTerminated() bool {
	for _, c := range m.collectors {
		if c != nil {
			return false
		}
	}
	return true
}

// leafCollectorFinisher is the optional LeafCollector extension for leaf
// collectors that have a finish() step. It is kept off the LeafCollector
// interface (so existing implementations keep compiling) and detected via type
// assertion, matching the way Gocene treats other optional collector hooks.
type leafCollectorFinisher interface {
	Finish() error
}

// finishLeafCollector calls Finish on collector if it implements the optional
// finisher interface; otherwise it is a no-op.
func finishLeafCollector(collector LeafCollector) error {
	if f, ok := collector.(leafCollectorFinisher); ok {
		return f.Finish()
	}
	return nil
}

// Ensure multiLeafCollector implements LeafCollector and the optional finisher.
var (
	_ LeafCollector         = (*multiLeafCollector)(nil)
	_ leafCollectorFinisher = (*multiLeafCollector)(nil)
)

// ignoreMinCompetitiveScorer wraps a Scorer so that setMinCompetitiveScore is
// ignored while every other Scorer/MinCompetitiveScorer call delegates to the
// inner scorer. It is the Scorer-model equivalent of Lucene's anonymous
// FilterScorable subclass used in MultiLeafCollector.setScorer.
type ignoreMinCompetitiveScorer struct {
	Scorer
}

func newIgnoreMinCompetitiveScorer(inner Scorer) *ignoreMinCompetitiveScorer {
	return &ignoreMinCompetitiveScorer{Scorer: inner}
}

// SetMinCompetitiveScore deliberately does nothing, so wrapping two collectors
// where one wants to skip low-scoring hits still lets the other see all hits.
func (s *ignoreMinCompetitiveScorer) SetMinCompetitiveScore(minScore float32) error {
	return nil
}

// minCompetitiveScoreAwareScorer wraps a Scorer for a single child of a
// TOP_SCORES MultiCollector. It records that child's requested minimum and only
// propagates the shared minimum (the smallest across all children) to the inner
// scorer, so the search does not skip a document any child still considers
// competitive.
//
// This is the Go port of MultiCollector.MinCompetitiveScoreAwareScorable.
type minCompetitiveScoreAwareScorer struct {
	Scorer
	idx       int
	minScores []float32
}

func newMinCompetitiveScoreAwareScorer(inner Scorer, idx int, minScores []float32) *minCompetitiveScoreAwareScorer {
	return &minCompetitiveScoreAwareScorer{Scorer: inner, idx: idx, minScores: minScores}
}

// SetMinCompetitiveScore records this child's minimum and forwards the shared
// minimum to the inner scorer when this child raised it.
func (s *minCompetitiveScoreAwareScorer) SetMinCompetitiveScore(minScore float32) error {
	if minScore > s.minScores[s.idx] {
		s.minScores[s.idx] = minScore
		if mc, ok := s.Scorer.(MinCompetitiveScorer); ok {
			return mc.SetMinCompetitiveScore(s.minScore())
		}
	}
	return nil
}

// minScore returns the smallest minimum competitive score requested across all
// children, mirroring the Java implementation (initialised to Float.MAX_VALUE).
func (s *minCompetitiveScoreAwareScorer) minScore() float32 {
	min := float32(math.MaxFloat32)
	for _, v := range s.minScores {
		if v < min {
			min = v
		}
	}
	return min
}

// Ensure the scorer wrappers satisfy Scorer and MinCompetitiveScorer.
var (
	_ Scorer               = (*ignoreMinCompetitiveScorer)(nil)
	_ MinCompetitiveScorer = (*ignoreMinCompetitiveScorer)(nil)
	_ Scorer               = (*minCompetitiveScoreAwareScorer)(nil)
	_ MinCompetitiveScorer = (*minCompetitiveScoreAwareScorer)(nil)
)
