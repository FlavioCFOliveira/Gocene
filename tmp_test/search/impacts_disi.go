// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "math"

// ImpactsDISI is a DocIdSetIterator wrapper that skips documents whose
// per-block impact-derived max score is below a minimum competitive score.
//
// Mirrors org.apache.lucene.search.ImpactsDISI.
type ImpactsDISI struct {
	in       DocIdSetIterator
	maxes    *MaxScoreCache
	minScore float32
	upTo     int
	maxScore float32
}

// NewImpactsDISI wraps in with a max-score cache.
func NewImpactsDISI(in DocIdSetIterator, maxes *MaxScoreCache) *ImpactsDISI {
	return &ImpactsDISI{
		in:       in,
		maxes:    maxes,
		minScore: float32(math.Inf(-1)),
		upTo:     -1,
	}
}

// GetMaxScoreCache returns the wrapped MaxScoreCache.
func (i *ImpactsDISI) GetMaxScoreCache() *MaxScoreCache { return i.maxes }

// SetMinCompetitiveScore updates the skip threshold.
func (i *ImpactsDISI) SetMinCompetitiveScore(minScore float32) { i.minScore = minScore }

// DocID returns the underlying iterator's current doc id.
func (i *ImpactsDISI) DocID() int { return i.in.DocID() }

// NextDoc advances to the next document that is competitive under the current
// minimum competitive score.
func (i *ImpactsDISI) NextDoc() (int, error) {
	doc, err := i.in.NextDoc()
	if err != nil {
		return doc, err
	}
	return i.maybeSkip(doc)
}

// Advance positions the iterator at the next competitive document at or
// beyond target.
func (i *ImpactsDISI) Advance(target int) (int, error) {
	doc, err := i.in.Advance(target)
	if err != nil {
		return doc, err
	}
	return i.maybeSkip(doc)
}

// Cost forwards to the underlying iterator.
func (i *ImpactsDISI) Cost() int64 { return i.in.Cost() }

// DocIDRunEnd forwards to the underlying iterator.
func (i *ImpactsDISI) DocIDRunEnd() int { return i.in.DocIDRunEnd() }

func (i *ImpactsDISI) maybeSkip(doc int) (int, error) {
	for doc != NO_MORE_DOCS {
		if doc > i.upTo {
			next, err := i.maxes.AdvanceShallow(doc)
			if err != nil {
				return doc, err
			}
			i.upTo = next
			i.maxScore = i.maxes.GetMaxScore(next)
		}
		if i.maxScore >= i.minScore {
			return doc, nil
		}
		// Skip past the end of the block.
		nd, err := i.in.Advance(i.upTo + 1)
		if err != nil {
			return doc, err
		}
		doc = nd
	}
	return doc, nil
}
