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
//   lucene/core/src/java/org/apache/lucene/search/ConjunctionBulkScorer.java

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// ConjunctionBulkScorer is a BulkScorer that implements the AND (conjunction)
// of two or more scorers.  It focuses on regular DocIdSetIterators; two-phase
// scorers are handled by other scorers (e.g. BlockMaxConjunctionBulkScorer).
//
// Mirrors org.apache.lucene.search.ConjunctionBulkScorer (Lucene 10.4.0).
//
// Deviations from Java:
//   - ScorerUtil.likelyTermScorer is not ported; Scorer.Score() is used
//     directly.
//   - collector.competitiveIterator() is not on Gocene's LeafCollector; that
//     path is omitted.
//   - Scorer.Score() returns float32 (not (float32,error)); Scorable.Score()
//     returns (float32,error) — these are different Go interfaces, so
//     ChildScorable cannot wrap []Scorer directly.  GetChildren returns nil.
type ConjunctionBulkScorer struct {
	scoringScorers []Scorer
	lead1          DocIdSetIterator
	lead2          DocIdSetIterator
	others         []DocIdSetIterator
	scorer         *conjunctionScorerAdapter
}

// NewConjunctionBulkScorer builds a ConjunctionBulkScorer from two scorer
// lists: scorers that contribute to the score and those that only filter.
//
// Mirrors ConjunctionBulkScorer(List<Scorer>, List<Scorer>).
func NewConjunctionBulkScorer(requiredScoring, requiredNoScoring []Scorer) (*ConjunctionBulkScorer, error) {
	numClauses := len(requiredScoring) + len(requiredNoScoring)
	if numClauses <= 1 {
		return nil, fmt.Errorf("ConjunctionBulkScorer: expected 2 or more clauses, got %d", numClauses)
	}

	allScorers := make([]Scorer, 0, numClauses)
	allScorers = append(allScorers, requiredScoring...)
	allScorers = append(allScorers, requiredNoScoring...)

	iterators := make([]DocIdSetIterator, numClauses)
	for i, sc := range allScorers {
		iterators[i] = sc // Scorer embeds DocIdSetIterator
	}
	sort.Slice(iterators, func(i, j int) bool {
		return iterators[i].Cost() < iterators[j].Cost()
	})

	lead1 := iterators[0]
	lead2 := iterators[1]
	others := make([]DocIdSetIterator, len(iterators)-2)
	copy(others, iterators[2:])

	scoringScorers := make([]Scorer, len(requiredScoring))
	copy(scoringScorers, requiredScoring)

	cbs := &ConjunctionBulkScorer{
		scoringScorers: scoringScorers,
		lead1:          lead1,
		lead2:          lead2,
		others:         others,
	}
	cbs.scorer = &conjunctionScorerAdapter{parent: cbs}
	return cbs, nil
}

// conjunctionScorerAdapter is a no-DISI Scorer that aggregates the scores of
// all scoring sub-scorers.  It is injected via SetScorer so collectors can
// call Score().  Navigation is driven by the BulkScorer loop itself;
// BaseDocIdSetIterator provides the stubs.
type conjunctionScorerAdapter struct {
	BaseScorer
	BaseDocIdSetIterator
	parent *ConjunctionBulkScorer
}

// Score sums the scores of all scoring sub-scorers.
func (a *conjunctionScorerAdapter) Score() float32 {
	var sum float64
	for _, sc := range a.parent.scoringScorers {
		sum += float64(sc.Score())
	}
	return float32(sum)
}

// GetMaxScore returns 0; the adapter is not used for block-max pruning.
func (a *conjunctionScorerAdapter) GetMaxScore(_ int) float32 { return 0 }

// Score scores documents in [min, max) that match every clause, applying the
// acceptDocs filter (nil accepts all), and returns lead1's docID after the
// window (the next matching document on or after max, or NO_MORE_DOCS).
//
// Faithful port of ConjunctionBulkScorer.score(LeafCollector, Bits, int, int).
// collector.competitiveIterator() is not on Gocene's LeafCollector, so that
// extra iterator is never appended to others.
func (bs *ConjunctionBulkScorer) Score(collector LeafCollector, acceptDocs util.Bits, min, max int) (int, error) {
	lead1 := bs.lead1
	lead2 := bs.lead2
	others := bs.others

	if lead1.DocID() < min {
		if _, err := lead1.Advance(min); err != nil {
			return 0, err
		}
	}

	if lead1.DocID() >= max {
		return lead1.DocID(), nil
	}

	if err := collector.SetScorer(bs.scorer); err != nil {
		return 0, err
	}

	// In the main loop we rely on the invariant lead1.docID() > lead2.docID().
	// It's possible they are equal on the first document of a scoring window, so
	// that case is handled separately here.
	if lead1.DocID() == lead2.DocID() {
		doc := lead1.DocID()
		if acceptDocs == nil || acceptDocs.Get(doc) {
			match := true
			for _, it := range others {
				if it.DocID() < doc {
					next, err := it.Advance(doc)
					if err != nil {
						return 0, err
					}
					if next != doc {
						if _, err := lead1.Advance(next); err != nil {
							return 0, err
						}
						match = false
						break
					}
				}
			}
			if match {
				if err := collector.Collect(doc); err != nil {
					return 0, err
				}
				if _, err := lead1.NextDoc(); err != nil {
					return 0, err
				}
			}
		} else {
			if _, err := lead1.NextDoc(); err != nil {
				return 0, err
			}
		}
	}

advanceHead:
	for doc := lead1.DocID(); doc < max; {
		if acceptDocs != nil && !acceptDocs.Get(doc) {
			var err error
			doc, err = lead1.NextDoc()
			if err != nil {
				return 0, err
			}
			continue
		}

		// Maintain the invariant lead2.docID() < lead1.docID().
		next2, err := lead2.Advance(doc)
		if err != nil {
			return 0, err
		}
		if next2 != doc {
			doc, err = lead1.Advance(next2)
			if err != nil {
				return 0, err
			}
			if doc != next2 {
				continue
			} else if doc >= max {
				break
			} else if acceptDocs != nil && !acceptDocs.Get(doc) {
				doc, err = lead1.NextDoc()
				if err != nil {
					return 0, err
				}
				continue
			}
		}

		for _, it := range others {
			if it.DocID() < doc {
				next, err := it.Advance(doc)
				if err != nil {
					return 0, err
				}
				if next != doc {
					doc, err = lead1.Advance(next)
					if err != nil {
						return 0, err
					}
					continue advanceHead
				}
			}
		}

		if err := collector.Collect(doc); err != nil {
			return 0, err
		}
		doc, err = lead1.NextDoc()
		if err != nil {
			return 0, err
		}
	}

	return lead1.DocID(), nil
}

// Cost returns the cost of the cheapest clause (lead1).
//
// Mirrors ConjunctionBulkScorer.cost().
func (bs *ConjunctionBulkScorer) Cost() int64 {
	return bs.lead1.Cost()
}

var _ BulkScorer = (*ConjunctionBulkScorer)(nil)
