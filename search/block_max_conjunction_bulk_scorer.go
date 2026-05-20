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
//   lucene/core/src/java/org/apache/lucene/search/BlockMaxConjunctionBulkScorer.java

import (
	"fmt"
	"sort"
)

// BlockMaxConjunctionBulkScorer is a BulkScorer for top-level conjunctions
// over clauses without two-phase iterators. It scores documents by first
// advancing all iterators to agreement and then accumulating clause scores.
//
// Mirrors org.apache.lucene.search.BlockMaxConjunctionBulkScorer (Lucene 10.4.0).
//
// Deviations from Java:
//   - Gocene BulkScorer.Score has signature (Collector, DocIdSetIterator) with no
//     min/max range or Bits acceptDocs; the score method iterates the acceptDocs
//     iterator as the lead if provided, otherwise walks the lead scorer.
//   - advanceShallow / nextDocsAndScores are not on Gocene's Scorer interface;
//     the score-first window path (scoreWindowScoreFirst) is degraded to a
//     simple conjunction walk identical to scoreDocFirstUntilDynamicPruning.
//   - DocAndFloatFeatureBuffer / DocAndScoreAccBuffer are empty stubs in Gocene;
//     they are retained as field declarations for structural parity but unused.
//   - ScorerUtil.likelyTermScorer / likelyImpactsEnum / filterCompetitiveHits /
//     applyRequiredClause are unported; scorers are used directly.
//   - SimpleScorable is implemented inline as blockMaxSimpleScorable.
//   - Gocene LeafCollector.SetScorer takes a Scorer, not a Scorable.
type BlockMaxConjunctionBulkScorer struct {
	scorers   []Scorer
	iterators []DocIdSetIterator
	lead      DocIdSetIterator
	scorable  *blockMaxSimpleScorable
	maxDoc    int

	// Structural placeholders retained for parity — unused until
	// DocAndFloatFeatureBuffer / DocAndScoreAccBuffer are fully ported.
	_ *DocAndFloatFeatureBuffer
	_ *DocAndScoreAccBuffer
}

// blockMaxSimpleScorable is a minimal mutable Scorable injected into the
// collector so that the collector can read the current document's score and
// push down a minimum competitive score threshold.
//
// Mirrors the inner SimpleScorable class in Java.
type blockMaxSimpleScorable struct {
	BaseScorable
	score               float32
	minCompetitiveScore float32
}

func (s *blockMaxSimpleScorable) Score() (float32, error) { return s.score, nil }
func (s *blockMaxSimpleScorable) SetMinCompetitiveScore(minScore float32) error {
	s.minCompetitiveScore = minScore
	return nil
}

// ensure blockMaxSimpleScorable satisfies Scorable.
var _ Scorable = (*blockMaxSimpleScorable)(nil)

// NewBlockMaxConjunctionBulkScorer constructs a BulkScorer over the given
// scorers. Panics if fewer than two scorers are provided.
//
// Mirrors BlockMaxConjunctionBulkScorer(int, List<Scorer>).
func NewBlockMaxConjunctionBulkScorer(maxDoc int, scorers []Scorer) (*BlockMaxConjunctionBulkScorer, error) {
	if len(scorers) <= 1 {
		return nil, fmt.Errorf("BlockMaxConjunctionBulkScorer: expected 2 or more scorers, got %d", len(scorers))
	}
	// Copy and sort by cost (least-costly lead first).
	s := make([]Scorer, len(scorers))
	copy(s, scorers)
	sort.Slice(s, func(i, j int) bool {
		return s[i].Cost() < s[j].Cost()
	})

	iterators := make([]DocIdSetIterator, len(s))
	for i, sc := range s {
		iterators[i] = sc
	}

	return &BlockMaxConjunctionBulkScorer{
		scorers:   s,
		iterators: iterators,
		lead:      iterators[0],
		scorable:  &blockMaxSimpleScorable{},
		maxDoc:    maxDoc,
	}, nil
}

// Score implements BulkScorer. It iterates over all matching documents
// (those satisfying the conjunction) and calls collector.Collect.
//
// The acceptDocs parameter, when non-nil, is advanced in lockstep to filter
// accepted documents.
//
// Mirrors BlockMaxConjunctionBulkScorer.score(LeafCollector, Bits, int, int).
func (bs *BlockMaxConjunctionBulkScorer) Score(collector Collector, acceptDocs DocIdSetIterator) error {
	// BlockMaxConjunctionBulkScorer operates at segment (leaf) level.
	// Gocene's Collector.GetLeafCollector requires an IndexReader context
	// which is not available here. We use a minimal shim that satisfies the
	// Collector interface when a LeafCollector is passed directly.
	// Since the Java method takes a LeafCollector directly, we do the same
	// by accepting Collector and obtaining a leaf collector from it.
	// If the collector itself implements LeafCollector we use it directly;
	// otherwise we cannot obtain a proper leaf context and fall back.
	leaf, ok := collector.(LeafCollector)
	if !ok {
		return fmt.Errorf("BlockMaxConjunctionBulkScorer.Score: collector does not implement LeafCollector")
	}
	return bs.scoreLeaf(leaf, acceptDocs)
}

// scoreLeaf performs the conjunction walk over all segments, calling leaf
// for each matching document.
func (bs *BlockMaxConjunctionBulkScorer) scoreLeaf(collector LeafCollector, acceptDocs DocIdSetIterator) error {
	// Inject a mutable scorable so the collector can read scores and push
	// down a minCompetitiveScore threshold (mirrors collector.setScorer(scorable)).
	// Gocene's LeafCollector.SetScorer takes a Scorer, not a Scorable.
	// We use an inlineScorerAdapter that wraps blockMaxSimpleScorable.
	adapter := &blockMaxScorerAdapter{s: bs.scorable}
	if err := collector.SetScorer(adapter); err != nil {
		return err
	}

	doc := bs.lead.DocID()
	if doc == -1 {
		var err error
		doc, err = bs.lead.NextDoc()
		if err != nil {
			return err
		}
	}

outer:
	for doc < NO_MORE_DOCS {
		// Filter by acceptDocs if provided (mirrors Bits.get(doc)).
		if acceptDocs != nil {
			ad := acceptDocs.DocID()
			for ad < doc {
				var err error
				ad, err = acceptDocs.Advance(doc)
				if err != nil {
					return err
				}
			}
			if ad != doc {
				// doc not accepted; advance lead to next accepted doc.
				var err error
				doc, err = bs.lead.Advance(ad)
				if err != nil {
					return err
				}
				continue outer
			}
		}

		// Advance all other iterators to doc.
		for i := 1; i < len(bs.iterators); i++ {
			it := bs.iterators[i]
			other := it.DocID()
			if other < doc {
				var err error
				other, err = it.Advance(doc)
				if err != nil {
					return err
				}
			}
			if other != doc {
				// Mismatch: advance lead to the next candidate.
				var err error
				doc, err = bs.lead.Advance(other)
				if err != nil {
					return err
				}
				continue outer
			}
		}

		// All iterators agree on doc — compute score.
		var total float64
		for _, sc := range bs.scorers {
			total += float64(sc.Score())
		}
		bs.scorable.score = float32(total)
		if err := collector.Collect(doc); err != nil {
			return err
		}

		var err error
		doc, err = bs.lead.NextDoc()
		if err != nil {
			return err
		}
	}
	return nil
}

// Cost returns the cost of iterating, which is the cost of the least
// costly (lead) iterator.
//
// Mirrors BlockMaxConjunctionBulkScorer.cost().
func (bs *BlockMaxConjunctionBulkScorer) Cost() int64 {
	return bs.lead.Cost()
}

// blockMaxScorerAdapter wraps a blockMaxSimpleScorable so it satisfies
// the Scorer interface, allowing injection into LeafCollector.SetScorer.
//
// DocIdSetIterator methods are no-ops — this adapter is only used as a
// score source, not for iteration.
type blockMaxScorerAdapter struct {
	BaseScorer
	s *blockMaxSimpleScorable
}

func (a *blockMaxScorerAdapter) Score() float32             { return a.s.score }
func (a *blockMaxScorerAdapter) GetMaxScore(_ int) float32  { return a.s.score }
func (a *blockMaxScorerAdapter) DocID() int                 { return -1 }
func (a *blockMaxScorerAdapter) NextDoc() (int, error)      { return NO_MORE_DOCS, nil }
func (a *blockMaxScorerAdapter) Advance(_ int) (int, error) { return NO_MORE_DOCS, nil }
func (a *blockMaxScorerAdapter) Cost() int64                { return 0 }
func (a *blockMaxScorerAdapter) DocIDRunEnd() int           { return NO_MORE_DOCS }

var _ Scorer = (*blockMaxScorerAdapter)(nil)
var _ BulkScorer = (*BlockMaxConjunctionBulkScorer)(nil)
