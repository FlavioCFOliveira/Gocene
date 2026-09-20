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

	"github.com/FlavioCFOliveira/Gocene/util"
)

// BlockMaxConjunctionBulkScorer is a BulkScorer for top-level conjunctions
// over clauses without two-phase iterators. It scores documents by first
// advancing all iterators to agreement and then accumulating clause scores.
//
// Mirrors org.apache.lucene.search.BlockMaxConjunctionBulkScorer (Lucene 10.4.0).
//
// Deviations from Java:
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
		return s[i].Iterator().Cost() < s[j].Iterator().Cost()
	})

	iterators := make([]DocIdSetIterator, len(s))
	for i, sc := range s {
		iterators[i] = sc.Iterator()
	}

	return &BlockMaxConjunctionBulkScorer{
		scorers:   s,
		iterators: iterators,
		lead:      iterators[0],
		scorable:  &blockMaxSimpleScorable{},
		maxDoc:    maxDoc,
	}, nil
}

// Score scores documents in [min, max) that satisfy the conjunction,
// delivering each accepted match to collector, and returns the first document
// the lead iterator advanced to on or beyond max (or NO_MORE_DOCS).
//
// Mirrors BlockMaxConjunctionBulkScorer.score(LeafCollector, Bits, int, int),
// degraded to a plain conjunction walk because advanceShallow /
// nextDocsAndScores are not on Gocene's Scorer interface. acceptDocs is a
// util.Bits filter (nil accepts all).
func (bs *BlockMaxConjunctionBulkScorer) Score(collector LeafCollector, acceptDocs util.Bits, min, max int) (int, error) {
	// Mirrors BlockMaxConjunctionBulkScorer.score(...): collector.setScorer(scorable).
	if err := collector.SetScorer(bs.scorable); err != nil {
		return 0, err
	}

	doc := bs.lead.DocID()
	if doc < min {
		var err error
		doc, err = bs.lead.Advance(min)
		if err != nil {
			return 0, err
		}
	}

outer:
	for doc < max {
		// Filter by acceptDocs if provided.
		if acceptDocs != nil && !acceptDocs.Get(doc) {
			var err error
			doc, err = bs.lead.NextDoc()
			if err != nil {
				return 0, err
			}
			continue outer
		}

		// Advance all other iterators to doc.
		for i := 1; i < len(bs.iterators); i++ {
			it := bs.iterators[i]
			other := it.DocID()
			if other < doc {
				var err error
				other, err = it.Advance(doc)
				if err != nil {
					return 0, err
				}
			}
			if other != doc {
				// Mismatch: advance lead to the next candidate.
				var err error
				doc, err = bs.lead.Advance(other)
				if err != nil {
					return 0, err
				}
				continue outer
			}
		}

		// All iterators agree on doc — compute score.
		var total float64
		for _, sc := range bs.scorers {
			sc0, err := sc.Score()
			if err != nil {
				return 0, err
			}
			total += float64(sc0)
		}
		bs.scorable.score = float32(total)
		if err := collector.Collect(doc); err != nil {
			return 0, err
		}

		var err error
		doc, err = bs.lead.NextDoc()
		if err != nil {
			return 0, err
		}
	}
	return doc, nil
}

// Cost returns the cost of iterating, which is the cost of the least
// costly (lead) iterator.
//
// Mirrors BlockMaxConjunctionBulkScorer.cost().
func (bs *BlockMaxConjunctionBulkScorer) Cost() int64 {
	return bs.lead.Cost()
}

var _ BulkScorer = (*BlockMaxConjunctionBulkScorer)(nil)
