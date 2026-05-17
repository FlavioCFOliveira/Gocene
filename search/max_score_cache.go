// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "math"

// ImpactsSource is the minimal contract MaxScoreCache needs from a source of
// per-level impact information. Concrete codec scorers implement this.
type ImpactsSource interface {
	// AdvanceShallow positions the source at target and returns the end of
	// the run of consecutive doc IDs covered by the impact block.
	AdvanceShallow(target int) (int, error)
	// NumLevels returns the number of impact levels available at the current
	// position.
	NumLevels() int
	// GetDocIDUpTo returns the inclusive upper doc id for the given impact
	// level at the current position.
	GetDocIDUpTo(level int) int
	// GetImpacts returns the (freq, norm) pairs for the given impact level.
	GetImpacts(level int) []Impact
}

// ImpactSimScorer is the minimal scoring contract MaxScoreCache needs to translate
// (freq, norm) impacts into a similarity score.
type ImpactSimScorer interface {
	Score(freq float32, norm int64) float32
}

// MaxScoreCache caches maximum similarity scores derived from per-level
// impacts so that callers can decide cheaply whether a block of documents is
// competitive under a given minimum-score threshold.
//
// Mirrors org.apache.lucene.search.MaxScoreCache.
type MaxScoreCache struct {
	src    ImpactsSource
	scorer ImpactSimScorer
	maxes  []float32 // cached max score per impact level (NaN means uncached)
	global float32
}

// NewMaxScoreCache creates an empty cache wired to src and scorer. The global
// max score is initialised to -Inf and is updated as levels are visited.
func NewMaxScoreCache(src ImpactsSource, scorer ImpactSimScorer) *MaxScoreCache {
	return &MaxScoreCache{src: src, scorer: scorer, global: float32(math.Inf(-1))}
}

// AdvanceShallow forwards to the source.
func (c *MaxScoreCache) AdvanceShallow(target int) (int, error) {
	return c.src.AdvanceShallow(target)
}

// GetMaxScore returns the highest cached max-score whose level upper bound is
// at least upTo. If no level applies, the global max is returned.
func (c *MaxScoreCache) GetMaxScore(upTo int) float32 {
	n := c.src.NumLevels()
	c.ensureCacheSize(n)
	best := c.global
	for level := 0; level < n; level++ {
		if c.src.GetDocIDUpTo(level) < upTo {
			continue
		}
		s := c.getMaxScoreForLevel(level)
		if s > best {
			best = s
		}
		break
	}
	return best
}

// GetSkipUpTo identifies an upper doc id below which the source can be safely
// skipped because no impact level exceeds minScore. Returns -1 if no skipping
// is possible.
func (c *MaxScoreCache) GetSkipUpTo(minScore float32) int {
	n := c.src.NumLevels()
	c.ensureCacheSize(n)
	for level := 0; level < n; level++ {
		if c.getMaxScoreForLevel(level) >= minScore {
			if level == 0 {
				return -1
			}
			return c.src.GetDocIDUpTo(level - 1)
		}
	}
	if n > 0 {
		return c.src.GetDocIDUpTo(n - 1)
	}
	return -1
}

func (c *MaxScoreCache) getMaxScoreForLevel(level int) float32 {
	if !math.IsNaN(float64(c.maxes[level])) {
		return c.maxes[level]
	}
	max := float32(0)
	for _, im := range c.src.GetImpacts(level) {
		s := c.scorer.Score(float32(im.Freq), im.Norm)
		if s > max {
			max = s
		}
	}
	c.maxes[level] = max
	if max > c.global {
		c.global = max
	}
	return max
}

func (c *MaxScoreCache) ensureCacheSize(n int) {
	if cap(c.maxes) < n {
		c.maxes = make([]float32, n)
	} else {
		c.maxes = c.maxes[:n]
	}
	for i := range c.maxes {
		c.maxes[i] = float32(math.NaN())
	}
}

// Impact is the structural placeholder for a (freq, norm) impact entry.
// Concrete codecs are expected to satisfy this shape.
type Impact struct {
	Freq int
	Norm int64
}
