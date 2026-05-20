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
)

// ConjunctionBulkScorer is a BulkScorer that implements the AND (conjunction)
// of two or more scorers.  It focuses on regular DocIdSetIterators; two-phase
// scorers are handled by other scorers (e.g. BlockMaxConjunctionBulkScorer).
//
// Mirrors org.apache.lucene.search.ConjunctionBulkScorer (Lucene 10.4.0).
//
// Deviations from Java:
//   - Java score(LeafCollector, Bits, int, int) uses a min/max window and
//     a raw Bits acceptDocs.  Gocene BulkScorer.Score takes a Collector and
//     a DocIdSetIterator acceptDocs without a window; the loop runs to
//     exhaustion.
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

// Score iterates all matching documents and calls collector.Collect for each.
// acceptDocs is an optional DocIdSetIterator filter; pass nil to accept all.
//
// Mirrors ConjunctionBulkScorer.score(LeafCollector, Bits, int, int) without
// the min/max window.
func (bs *ConjunctionBulkScorer) Score(collector Collector, acceptDocs DocIdSetIterator) error {
	lc, err := collector.GetLeafCollector(nil)
	if err != nil {
		return err
	}
	if err := lc.SetScorer(bs.scorer); err != nil {
		return err
	}

	lead1 := bs.lead1
	lead2 := bs.lead2
	others := bs.others

	// Position lead1 at the first document.
	doc, err := lead1.NextDoc()
	if err != nil {
		return err
	}

	// Handle the case where lead1 == lead2 on the very first doc.
	if doc != NO_MORE_DOCS && doc == lead2.DocID() {
		match := true
		for _, it := range others {
			if it.DocID() < doc {
				next, err := it.Advance(doc)
				if err != nil {
					return err
				}
				if next != doc {
					doc, err = lead1.Advance(next)
					if err != nil {
						return err
					}
					match = false
					break
				}
			}
		}
		if match && isAccepted(acceptDocs, doc) {
			if err := lc.Collect(doc); err != nil {
				return err
			}
		}
		if match {
			doc, err = lead1.NextDoc()
			if err != nil {
				return err
			}
		}
	}

advanceHead:
	for doc != NO_MORE_DOCS {
		// acceptDocs filter.
		if acceptDocs != nil {
			adoc, aerr := advanceTo(acceptDocs, doc)
			if aerr != nil {
				return aerr
			}
			if adoc != doc {
				// doc not accepted — advance lead1 to next accepted doc.
				doc, err = lead1.Advance(adoc)
				if err != nil {
					return err
				}
				continue
			}
		}

		// Advance lead2 to reach current doc.
		next2 := lead2.DocID()
		if next2 < doc {
			next2, err = lead2.Advance(doc)
			if err != nil {
				return err
			}
		}
		if next2 != doc {
			doc, err = lead1.Advance(next2)
			if err != nil {
				return err
			}
			continue
		}

		// Advance all other clauses.
		for _, it := range others {
			if it.DocID() < doc {
				next, err := it.Advance(doc)
				if err != nil {
					return err
				}
				if next != doc {
					doc, err = lead1.Advance(next)
					if err != nil {
						return err
					}
					continue advanceHead
				}
			}
		}

		if err := lc.Collect(doc); err != nil {
			return err
		}
		doc, err = lead1.NextDoc()
		if err != nil {
			return err
		}
	}

	return nil
}

// isAccepted reports whether doc is accepted by acceptDocs.
// If acceptDocs is nil, all documents are accepted.
func isAccepted(acceptDocs DocIdSetIterator, doc int) bool {
	if acceptDocs == nil {
		return true
	}
	adoc := acceptDocs.DocID()
	if adoc == doc {
		return true
	}
	if adoc > doc {
		return false
	}
	next, _ := acceptDocs.Advance(doc)
	return next == doc
}

// advanceTo advances disi to at least target and returns its new docID.
func advanceTo(disi DocIdSetIterator, target int) (int, error) {
	if disi.DocID() >= target {
		return disi.DocID(), nil
	}
	return disi.Advance(target)
}

// Cost returns the cost of the cheapest clause (lead1).
//
// Mirrors ConjunctionBulkScorer.cost().
func (bs *ConjunctionBulkScorer) Cost() int64 {
	return bs.lead1.Cost()
}

var _ BulkScorer = (*ConjunctionBulkScorer)(nil)
