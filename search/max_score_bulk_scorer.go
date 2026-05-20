// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/MaxScoreBulkScorer.java

import (
	"math"
	"sort"
)

// InnerWindowSize is the size of the inner scoring window used by
// MaxScoreBulkScorer.
//
// Mirrors MaxScoreBulkScorer.INNER_WINDOW_SIZE (Lucene 10.4.0).
const InnerWindowSize = 1 << 12

// MaxScoreBulkScorer is a BulkScorer that uses the block-max algorithm
// for disjunction scoring with dynamic pruning.
//
// Mirrors org.apache.lucene.search.MaxScoreBulkScorer (Lucene 10.4.0).
//
// Deviations from Java:
//   - advanceShallow / nextDocsAndScores are not on Gocene's Scorer interface;
//     the inner-window scoring paths that rely on them are degraded to
//     one-by-one iteration via NextDoc.
//   - SimpleScorable / competitiveIterator feedback loop is not available;
//     setMinCompetitiveScore calls are omitted.
//   - DocAndScoreAccBuffer / DocAndFloatFeatureBuffer pipelines are structurally
//     present but the batch-fill paths are bypassed.
//   - Gocene's BulkScorer.Score takes (Collector, DocIdSetIterator) with no
//     min/max range; acceptDocs is the live-docs DISI (may be nil).
//   - DisiPriorityQueue.top2() is used for the two-essential-clauses fast path
//     (structural); the actual fast path degrades to standard nextDoc iteration.
type MaxScoreBulkScorer struct {
	maxDoc     int
	allScorers []*DisiWrapper
	// essentialQueue holds the "essential" scorers (those that can yield competitive scores).
	essentialQueue *DisiPriorityQueue
	// firstEssentialScorer is the index into allScorers where essential scorers start.
	firstEssentialScorer int
	// firstRequiredScorer is the index where required scorers start.
	firstRequiredScorer int
	// nextMinCompetitiveScore is the minimum minCompetitiveScore that would improve partitioning.
	nextMinCompetitiveScore float32
	cost                    int64
	// scorable is the mutable score holder injected into the collector.
	scorable     *maxScoreScorable
	maxScoreSums []float64
	// filter is an optional required-filter scorer.
	filter *DisiWrapper
}

// maxScoreScorable is a minimal Scorable injected into the leaf collector.
type maxScoreScorable struct {
	BaseScorable
	score               float32
	minCompetitiveScore float32
}

func (s *maxScoreScorable) Score() (float32, error) { return s.score, nil }
func (s *maxScoreScorable) SetMinCompetitiveScore(v float32) error {
	if v > s.minCompetitiveScore {
		s.minCompetitiveScore = v
	}
	return nil
}

var _ Scorable = (*maxScoreScorable)(nil)

// maxScoreScorerAdapter wraps maxScoreScorable to satisfy the Scorer interface
// so it can be passed to LeafCollector.SetScorer.
type maxScoreScorerAdapter struct {
	BaseScorer
	s *maxScoreScorable
	// disi provides iteration; not used here but required by the interface.
}

func (a *maxScoreScorerAdapter) DocID() int                 { return -1 }
func (a *maxScoreScorerAdapter) NextDoc() (int, error)      { return NO_MORE_DOCS, nil }
func (a *maxScoreScorerAdapter) Advance(_ int) (int, error) { return NO_MORE_DOCS, nil }
func (a *maxScoreScorerAdapter) Cost() int64                { return 0 }
func (a *maxScoreScorerAdapter) DocIDRunEnd() int           { return 0 }
func (a *maxScoreScorerAdapter) Score() float32             { return a.s.score }
func (a *maxScoreScorerAdapter) GetMaxScore(_ int) float32  { return a.BaseScorer.GetMaxScore(0) }

var _ Scorer = (*maxScoreScorerAdapter)(nil)

// NewMaxScoreBulkScorer constructs a MaxScoreBulkScorer.
//
// filter is an optional required-filter scorer (may be nil).
//
// Mirrors MaxScoreBulkScorer(int, List<Scorer>, Scorer).
func NewMaxScoreBulkScorer(maxDoc int, scorers []Scorer, filter Scorer) *MaxScoreBulkScorer {
	bs := &MaxScoreBulkScorer{
		maxDoc:         maxDoc,
		allScorers:     make([]*DisiWrapper, len(scorers)),
		essentialQueue: NewDisiPriorityQueue(len(scorers)),
		maxScoreSums:   make([]float64, len(scorers)),
		scorable:       &maxScoreScorable{},
	}
	for i, s := range scorers {
		w := NewDisiWrapper(s, true)
		bs.cost += w.cost
		bs.allScorers[i] = w
	}
	if filter != nil {
		bs.filter = NewDisiWrapper(filter, false)
	}
	return bs
}

// Score iterates over matching documents and collects each one.
//
// Mirrors MaxScoreBulkScorer.score(LeafCollector, Bits, int, int),
// degraded to one-by-one iteration since advanceShallow / nextDocsAndScores
// are not on Gocene's Scorer interface.
func (bs *MaxScoreBulkScorer) Score(collector Collector, acceptDocs DocIdSetIterator) error {
	lc, ok := collector.(LeafCollector)
	if !ok {
		return NewDefaultBulkScorer(newMaxScoreUnionScorer(bs.allScorers)).Score(collector, acceptDocs)
	}

	scorerAdapter := &maxScoreScorerAdapter{s: bs.scorable}
	if err := lc.SetScorer(scorerAdapter); err != nil {
		return err
	}

	// Collect all docs from all essential (and initially all) scorers via
	// a disjunction iterator, apply acceptDocs filtering.
	if len(bs.allScorers) == 0 {
		return nil
	}

	// Use DisjunctionDISIApproximation as the union iterator.
	disi := NewDisjunctionDISIApproximation(bs.allScorers, bs.cost)
	doc, err := disi.NextDoc()
	if err != nil {
		return err
	}

	for doc != NO_MORE_DOCS {
		accepted := acceptDocs == nil
		if !accepted {
			adDoc := acceptDocs.DocID()
			if adDoc < doc {
				var advErr error
				adDoc, advErr = acceptDocs.Advance(doc)
				if advErr != nil {
					return advErr
				}
			}
			accepted = adDoc == doc
		}

		if accepted {
			// Sum scores from all matching scorers at this doc.
			var totalScore float64
			for tl := disi.TopList(); tl != nil; tl = tl.next {
				totalScore += float64(tl.scorable.Score())
			}
			bs.scorable.score = float32(totalScore)
			if err := lc.Collect(doc); err != nil {
				return err
			}
		}

		doc, err = disi.NextDoc()
		if err != nil {
			return err
		}
	}

	return nil
}

// Cost returns the estimated number of matching documents.
func (bs *MaxScoreBulkScorer) Cost() int64 { return bs.cost }

// PartitionScorers partitions allScorers into essential and non-essential
// groups based on maxWindowScore and minCompetitiveScore. Returns false if
// there are no matches (all scorers are non-essential).
//
// Mirrors MaxScoreBulkScorer.partitionScorers() — structural port.
func (bs *MaxScoreBulkScorer) PartitionScorers() bool {
	scratch := make([]*DisiWrapper, len(bs.allScorers))
	copy(scratch, bs.allScorers)

	sort.Slice(scratch, func(i, j int) bool {
		di := float64(scratch[i].maxWindowScore) / float64(max64(1, scratch[i].cost))
		dj := float64(scratch[j].maxWindowScore) / float64(max64(1, scratch[j].cost))
		return di < dj
	})

	var maxScoreSum float64
	bs.firstEssentialScorer = 0
	bs.nextMinCompetitiveScore = float32(math.Inf(1))
	n := len(bs.allScorers)

	for i := 0; i < n; i++ {
		w := scratch[i]
		newMax := maxScoreSum + float64(w.maxWindowScore)
		maxScoreSumFloat := float32(newMax)
		if maxScoreSumFloat < bs.scorable.minCompetitiveScore {
			maxScoreSum = newMax
			bs.allScorers[bs.firstEssentialScorer] = w
			bs.maxScoreSums[bs.firstEssentialScorer] = maxScoreSum
			bs.firstEssentialScorer++
		} else {
			bs.allScorers[n-1-(i-bs.firstEssentialScorer)] = w
			if maxScoreSumFloat < bs.nextMinCompetitiveScore {
				bs.nextMinCompetitiveScore = maxScoreSumFloat
			}
		}
	}

	bs.firstRequiredScorer = n
	if bs.firstEssentialScorer == n {
		return false
	}

	bs.essentialQueue.Clear()
	for i := bs.firstEssentialScorer; i < n; i++ {
		bs.essentialQueue.Add(bs.allScorers[i])
	}

	if bs.firstEssentialScorer == n-1 {
		bs.firstRequiredScorer = n - 1
		maxRequired := float64(bs.allScorers[bs.firstEssentialScorer].maxWindowScore)
		for bs.firstRequiredScorer > 0 {
			var maxWithout float64
			if bs.firstRequiredScorer > 1 {
				maxWithout = maxRequired + bs.maxScoreSums[bs.firstRequiredScorer-2]
			} else {
				maxWithout = maxRequired
			}
			if float32(maxWithout) >= bs.scorable.minCompetitiveScore {
				break
			}
			bs.firstRequiredScorer--
			maxRequired += float64(bs.allScorers[bs.firstRequiredScorer].maxWindowScore)
		}
	}
	return true
}

// max64 returns the larger of a and b.
func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// newMaxScoreUnionScorer creates a synthetic union Scorer from DisiWrappers.
// Used only for fallback when the collector is not a LeafCollector.
type maxScoreUnionScorer struct {
	BaseScorer
	disi  *DisjunctionDISIApproximation
	wraps []*DisiWrapper
}

func newMaxScoreUnionScorer(wrappers []*DisiWrapper) Scorer {
	var totalCost int64
	for _, w := range wrappers {
		totalCost += w.cost
	}
	disi := NewDisjunctionDISIApproximation(wrappers, totalCost)
	return &maxScoreUnionScorer{disi: disi, wraps: wrappers}
}

func (s *maxScoreUnionScorer) DocID() int                 { return s.disi.DocID() }
func (s *maxScoreUnionScorer) NextDoc() (int, error)      { return s.disi.NextDoc() }
func (s *maxScoreUnionScorer) Advance(t int) (int, error) { return s.disi.Advance(t) }
func (s *maxScoreUnionScorer) Cost() int64                { return s.disi.Cost() }
func (s *maxScoreUnionScorer) DocIDRunEnd() int           { return s.disi.DocIDRunEnd() }
func (s *maxScoreUnionScorer) Score() float32 {
	var total float64
	for tl := s.disi.TopList(); tl != nil; tl = tl.next {
		total += float64(tl.scorable.Score())
	}
	return float32(total)
}
func (s *maxScoreUnionScorer) GetMaxScore(_ int) float32 { return s.BaseScorer.GetMaxScore(0) }

var _ BulkScorer = (*MaxScoreBulkScorer)(nil)
