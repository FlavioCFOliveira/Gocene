// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.search.CoveringScorer.
package search

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// coveringScorer is a search.Scorer whose number of matching sub-scorers per
// document must meet a per-document minimum supplied by a search.LongValues.
//
// Mirrors org.apache.lucene.sandbox.search.CoveringScorer (package-private).
type coveringScorer struct {
	numScorers     int
	maxDoc         int
	minMatchValues search.LongValues

	matches  bool                // true if the current doc is already confirmed to match
	doc      int                 // current document ID
	topList  *search.DisiWrapper // linked list of wrappers on the current doc
	freq     int                 // number of sub-scorers on the current doc
	minMatch int64               // required match count for the current doc

	subScorers *search.DisiPriorityQueue
	cost       int64

	approximation search.DocIdSetIterator
	twoPhase      *search.TwoPhaseIterator
}

// newCoveringScorer builds a coveringScorer from a slice of sub-scorers.
func newCoveringScorer(scorers []search.Scorer, minMatchValues search.LongValues, maxDoc int) *coveringScorer {
	s := &coveringScorer{
		numScorers:     len(scorers),
		maxDoc:         maxDoc,
		minMatchValues: minMatchValues,
		doc:            -1,
		subScorers:     search.NewDisiPriorityQueue(len(scorers)),
	}

	var totalCost int64
	for _, sc := range scorers {
		s.subScorers.Add(search.NewDisiWrapper(sc, false))
		totalCost += sc.Cost()
	}
	s.cost = totalCost

	// approximation: iterates over candidate documents
	s.approximation = &coveringApproximation{s: s}
	s.twoPhase = search.NewTwoPhaseIteratorWithMatchCost(
		s.approximation,
		func() (bool, error) { return s.twoPhaseMatches() },
		float32(len(scorers)),
	)
	return s
}

// twoPhaseMatches is the Matches function for the embedded TwoPhaseIterator.
func (s *coveringScorer) twoPhaseMatches() (bool, error) {
	if s.matches {
		return true, nil
	}
	if s.topList == nil {
		if err := s.advanceAll(s.doc); err != nil {
			return false, err
		}
	}
	top := s.subScorers.Top()
	if top == nil || top.Doc() != s.doc {
		return false, nil
	}
	s.setTopListAndFreq()
	s.matches = s.freq >= int(s.minMatch)
	return s.matches, nil
}

// DocID returns the current document ID.
func (s *coveringScorer) DocID() int { return s.doc }

// DocIDRunEnd returns doc+1 as the run never spans more than one document.
func (s *coveringScorer) DocIDRunEnd() int { return s.doc + 1 }

// NextDoc advances to the next document via the TwoPhaseIterator DISI.
func (s *coveringScorer) NextDoc() (int, error) {
	return s.twoPhase.AsDocIdSetIterator().NextDoc()
}

// Advance advances to the first document at or beyond target via the TwoPhaseIterator DISI.
func (s *coveringScorer) Advance(target int) (int, error) {
	return s.twoPhase.AsDocIdSetIterator().Advance(target)
}

// Cost returns the total cost of all sub-scorers.
func (s *coveringScorer) Cost() int64 { return s.cost }

// Score sums the scores of all sub-scorers on the current document.
func (s *coveringScorer) Score() float32 {
	if err := s.setTopListAndFreqIfNecessary(); err != nil {
		return 0
	}
	var total float64
	for w := s.topList; w != nil; w = w.Next() {
		total += float64(w.Scorable().Score())
	}
	return float32(total)
}

// GetMaxScore returns +Inf; an upper bound cannot be computed cheaply.
func (s *coveringScorer) GetMaxScore(_ int) float32 {
	return float32(math.Inf(1))
}

// AdvanceShallow returns search.NO_MORE_DOCS, the default defined by
// org.apache.lucene.search.Scorer#advanceShallow. This scorer does not expose
// per-block impact information.
func (s *coveringScorer) AdvanceShallow(target int) (int, error) {
	return search.NO_MORE_DOCS, nil
}

// GetChildren returns the matching child scorers as ChildScorables.
func (s *coveringScorer) GetChildren() ([]search.ChildScorable, error) {
	if err := s.setTopListAndFreqIfNecessary(); err != nil {
		return nil, err
	}
	var out []search.ChildScorable
	for w := s.topList; w != nil; w = w.Next() {
		out = append(out, search.ChildScorable{Child: nil, Relationship: "SHOULD"})
		_ = w // scorer not exposed via ChildScorable.Child (Scorable vs Scorer mismatch)
	}
	return out, nil
}

// TwoPhaseIterator exposes the two-phase iterator for external use.
func (s *coveringScorer) TwoPhaseIterator() *search.TwoPhaseIterator {
	return s.twoPhase
}

var _ search.Scorer = (*coveringScorer)(nil)

// advanceAll advances all sub-scorers to at least target.
func (s *coveringScorer) advanceAll(target int) error {
	top := s.subScorers.Top()
	for top != nil && top.Doc() < target {
		if _, err := top.Advance(target); err != nil {
			return err
		}
		top = s.subScorers.UpdateTop()
	}
	return nil
}

// setTopListAndFreq collects all wrappers sharing the minimum doc into topList.
func (s *coveringScorer) setTopListAndFreq() {
	s.topList = s.subScorers.TopList()
	s.freq = 0
	for w := s.topList; w != nil; w = w.Next() {
		s.freq++
	}
}

// setTopListAndFreqIfNecessary ensures topList is populated for the current doc.
func (s *coveringScorer) setTopListAndFreqIfNecessary() error {
	if s.topList == nil {
		if err := s.advanceAll(s.doc); err != nil {
			return err
		}
		s.setTopListAndFreq()
	}
	return nil
}

// setMinMatch updates s.minMatch for the current s.doc.
func (s *coveringScorer) setMinMatch() error {
	if s.doc >= s.maxDoc {
		s.minMatch = 1
		return nil
	}
	ok, err := s.minMatchValues.AdvanceExact(s.doc)
	if err != nil {
		return err
	}
	if ok {
		v, err := s.minMatchValues.LongValue()
		if err != nil {
			return err
		}
		if v < 1 {
			v = 1
		}
		s.minMatch = v
	} else {
		s.minMatch = math.MaxInt64
	}
	return nil
}

// coveringApproximation is the DocIdSetIterator used as the approximation for
// the TwoPhaseIterator embedded in coveringScorer.
type coveringApproximation struct {
	s *coveringScorer
}

func (a *coveringApproximation) DocID() int { return a.s.doc }

func (a *coveringApproximation) NextDoc() (int, error) {
	return a.Advance(a.s.doc + 1)
}

func (a *coveringApproximation) Advance(target int) (int, error) {
	s := a.s
	// reset state
	s.matches = false
	s.topList = nil

	s.doc = target
	if err := s.setMinMatch(); err != nil {
		return 0, err
	}

	top := s.subScorers.Top()
	numMatches := 0
	maxPotentialMatches := s.numScorers
	for top != nil && top.Doc() < target {
		if int64(maxPotentialMatches) < s.minMatch {
			// Cannot possibly reach minMatch; skip to next candidate.
			if target >= s.maxDoc-1 {
				s.doc = search.NO_MORE_DOCS
			} else {
				s.doc = target + 1
			}
			if err := s.setMinMatch(); err != nil {
				return 0, err
			}
			return s.doc, nil
		}
		newDoc, err := top.Advance(target)
		if err != nil {
			return 0, err
		}
		isMatch := newDoc == target
		top = s.subScorers.UpdateTop()
		if isMatch {
			numMatches++
			if int64(numMatches) >= s.minMatch {
				// Enough matches found; confirm early.
				s.matches = true
				return s.doc, nil
			}
		} else {
			maxPotentialMatches--
		}
	}

	if top != nil {
		s.doc = top.Doc()
	} else {
		s.doc = search.NO_MORE_DOCS
	}
	if err := s.setMinMatch(); err != nil {
		return 0, err
	}
	return s.doc, nil
}

func (a *coveringApproximation) Cost() int64 { return int64(a.s.maxDoc) }

func (a *coveringApproximation) DocIDRunEnd() int { return a.s.doc + 1 }

var _ search.DocIdSetIterator = (*coveringApproximation)(nil)
