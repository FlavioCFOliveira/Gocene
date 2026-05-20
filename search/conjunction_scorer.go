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
//   lucene/core/src/java/org/apache/lucene/search/ConjunctionScorer.java

// ConjunctionScorer is a Scorer for conjunctions — sets of queries all of
// which are required.
//
// Mirrors org.apache.lucene.search.ConjunctionScorer (Lucene 10.4.0).
//
// Deviations from Java:
//   - Java ConjunctionScorer is package-private; the Go port exports it so
//     companion packages can construct it.
//   - Java ConjunctionScorer.getChildren() returns ChildScorable(Scorer, "MUST")
//     for each required clause.  In Gocene, Scorer does not implement Scorable
//     (they are structurally incompatible interfaces), so GetChildren returns
//     nil.  A future bridging sprint will revisit.
//   - Java's advanceShallow and setMinCompetitiveScore are not on the Gocene
//     Scorer interface and are omitted.
//   - TwoPhaseIterator() is surfaced via the optional HasTwoPhaseIterator
//     helper (which inspects the wrapped DISI) rather than as an interface
//     method.
type ConjunctionScorer struct {
	BaseDocIdSetIterator
	BaseScorer
	disi     DocIdSetIterator
	scorers  []Scorer
	required []Scorer
}

// NewConjunctionScorer builds a ConjunctionScorer.
//
// required is the full set of clauses (all must match).
// scorers is the subset whose scores contribute to the final score; it must
// be a subset of required.
//
// Mirrors ConjunctionScorer(Collection<Scorer>, Collection<Scorer>).
func NewConjunctionScorer(required, scorers []Scorer) *ConjunctionScorer {
	disi := IntersectScorers(required)
	sc := make([]Scorer, len(scorers))
	copy(sc, scorers)
	req := make([]Scorer, len(required))
	copy(req, required)
	return &ConjunctionScorer{
		disi:     disi,
		scorers:  sc,
		required: req,
	}
}

// TwoPhaseIterator returns the TwoPhaseIterator embedded in the conjunction DISI,
// or nil if none.
//
// Mirrors ConjunctionScorer.twoPhaseIterator().
func (s *ConjunctionScorer) TwoPhaseIterator() *TwoPhaseIterator {
	return HasTwoPhaseIterator(s.disi)
}

// DocID returns the current document ID.
//
// Mirrors ConjunctionScorer.docID().
func (s *ConjunctionScorer) DocID() int { return s.disi.DocID() }

// NextDoc advances to the next matching document.
func (s *ConjunctionScorer) NextDoc() (int, error) { return s.disi.NextDoc() }

// Advance advances to the first document at or beyond target.
func (s *ConjunctionScorer) Advance(target int) (int, error) { return s.disi.Advance(target) }

// Cost returns the cost estimate of the conjunction.
func (s *ConjunctionScorer) Cost() int64 { return s.disi.Cost() }

// DocIDRunEnd returns the end of the current run.
func (s *ConjunctionScorer) DocIDRunEnd() int {
	d := s.disi.DocID()
	if d >= NO_MORE_DOCS {
		return NO_MORE_DOCS
	}
	return d + 1
}

// Score sums the scores of all scoring sub-scorers.
//
// Mirrors ConjunctionScorer.score().
func (s *ConjunctionScorer) Score() float32 {
	var sum float64
	for _, sc := range s.scorers {
		sum += float64(sc.Score())
	}
	return float32(sum)
}

// GetMaxScore returns the sum of the max scores of all scoring sub-scorers
// that are positioned on or before upTo.
//
// Mirrors ConjunctionScorer.getMaxScore(int).
func (s *ConjunctionScorer) GetMaxScore(upTo int) float32 {
	var maxScore float64
	for _, sc := range s.scorers {
		if sc.DocID() <= upTo {
			maxScore += float64(sc.GetMaxScore(upTo))
		}
	}
	return float32(maxScore)
}

// GetDISI returns the underlying conjunction DocIdSetIterator.
// This is provided for callers that need direct iterator access.
func (s *ConjunctionScorer) GetDISI() DocIdSetIterator { return s.disi }

// GetRequired returns the required scorers (a copy).
func (s *ConjunctionScorer) GetRequired() []Scorer {
	out := make([]Scorer, len(s.required))
	copy(out, s.required)
	return out
}

// Compile-time assertion.
var _ Scorer = (*ConjunctionScorer)(nil)
