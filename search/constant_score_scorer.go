// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/util"

// ConstantScoreScorer is a Scorer that returns the same constant
// score for every matching document. It is the Go port of
// org.apache.lucene.search.ConstantScoreScorer (Lucene 10.4.0).
//
// All iteration is delegated to the wrapped DocIdSetIterator: DocID,
// NextDoc, Advance, Cost and DocIDRunEnd are forwarders. Score and
// GetMaxScore always return the constant score the scorer was built
// with — there is no per-document variability.
//
// The Java reference accepts a third constructor argument
// (TwoPhaseIterator) for queries that defer the expensive
// confirmation step. That overload is not yet wired in Gocene; the
// single-iterator constructor below matches every call site
// currently exercised by the search package (including the spatial
// query family). Once a TwoPhaseIterator surface lands, a
// NewConstantScoreScorerTwoPhase constructor can be added without
// breaking this API.
type ConstantScoreScorer struct {
	BaseScorer
	score         float32
	scoreMode     ScoreMode
	approximation DocIdSetIterator
	iterator      DocIdSetIterator

	// twoPhaseIterator mirrors the Java field of the same name: null for the
	// DocIdSetIterator-based constructor, the two-phase view otherwise.
	twoPhaseIterator *TwoPhaseIterator

	// wrapper is non-nil only in TOP_SCORES mode. SetMinCompetitiveScore swaps
	// its delegate for an empty iterator once minScore exceeds the constant
	// score, mirroring Lucene's DocIdSetIteratorWrapper.
	wrapper *constantScoreDISIWrapper
}

// constantScoreDISIWrapper mirrors the private DocIdSetIteratorWrapper in
// Lucene's ConstantScoreScorer: it forwards iteration to a swappable delegate
// so that SetMinCompetitiveScore can replace the delegate with an empty
// iterator and terminate iteration once the constant score is no longer
// competitive.
type constantScoreDISIWrapper struct {
	doc      int
	delegate DocIdSetIterator
}

func newConstantScoreDISIWrapper(delegate DocIdSetIterator) *constantScoreDISIWrapper {
	return &constantScoreDISIWrapper{doc: -1, delegate: delegate}
}

func (w *constantScoreDISIWrapper) DocID() int { return w.doc }

func (w *constantScoreDISIWrapper) NextDoc() (int, error) {
	doc, err := w.delegate.NextDoc()
	w.doc = doc
	return doc, err
}

func (w *constantScoreDISIWrapper) Advance(target int) (int, error) {
	doc, err := w.delegate.Advance(target)
	w.doc = doc
	return doc, err
}

func (w *constantScoreDISIWrapper) Cost() int64 { return w.delegate.Cost() }

func (w *constantScoreDISIWrapper) DocIDRunEnd() (int, error) { return w.doc + 1, nil }

// NewConstantScoreScorer builds a ConstantScoreScorer that yields
// score for every document emitted by disi. The same iterator is
// used both as the approximation (used when two-phase iteration is
// negotiated) and the confirmed iterator.
//
// In TOP_SCORES mode the iterator is wrapped so that SetMinCompetitiveScore
// can short-circuit iteration once the (constant) score is no longer
// competitive, mirroring the TOP_SCORES branch of Lucene's
// org.apache.lucene.search.ConstantScoreScorer constructor.
func NewConstantScoreScorer(score float32, scoreMode ScoreMode, disi DocIdSetIterator) *ConstantScoreScorer {
	s := &ConstantScoreScorer{
		score:     score,
		scoreMode: scoreMode,
	}
	if scoreMode == TOP_SCORES {
		s.wrapper = newConstantScoreDISIWrapper(disi)
		s.approximation = s.wrapper
		s.iterator = s.wrapper
	} else {
		s.approximation = disi
		s.iterator = disi
	}
	return s
}

// NewConstantScoreScorerFromTwoPhase builds a ConstantScoreScorer driven by a
// TwoPhaseIterator; the scorer then supports two-phase iteration.
//
// Mirrors the second Java constructor
// ConstantScoreScorer(float score, ScoreMode scoreMode, TwoPhaseIterator twoPhaseIterator).
// Java overloads the constructor on the third parameter's type; Go has no
// overloading, so the two forms are separate functions.
//
//	this.score = score;
//	this.scoreMode = scoreMode;
//	if (scoreMode == ScoreMode.TOP_SCORES) {
//	  this.approximation = new DocIdSetIteratorWrapper(twoPhaseIterator.approximation());
//	  this.twoPhaseIterator = new TwoPhaseIterator(this.approximation) {
//	    matches()   -> twoPhaseIterator.matches()
//	    matchCost() -> twoPhaseIterator.matchCost()
//	  };
//	} else {
//	  this.approximation = twoPhaseIterator.approximation();
//	  this.twoPhaseIterator = twoPhaseIterator;
//	}
//	this.disi = TwoPhaseIterator.asDocIdSetIterator(this.twoPhaseIterator);
func NewConstantScoreScorerFromTwoPhase(score float32, scoreMode ScoreMode, twoPhaseIterator *TwoPhaseIterator) *ConstantScoreScorer {
	s := &ConstantScoreScorer{
		score:     score,
		scoreMode: scoreMode,
	}
	if scoreMode == TOP_SCORES {
		s.wrapper = newConstantScoreDISIWrapper(twoPhaseIterator.Approximation())
		s.approximation = s.wrapper
		inner := twoPhaseIterator
		s.twoPhaseIterator = NewTwoPhaseIteratorWithMatchCost(
			s.approximation,
			func() (bool, error) { return inner.Matches() },
			inner.MatchCost(),
		)
	} else {
		s.approximation = twoPhaseIterator.Approximation()
		s.twoPhaseIterator = twoPhaseIterator
	}
	s.iterator = AsDocIdSetIterator(s.twoPhaseIterator)
	return s
}

// DocID returns the doc the iterator currently sits on.
func (s *ConstantScoreScorer) DocID() int { return s.iterator.DocID() }

// NextDoc advances to the next matching document.
func (s *ConstantScoreScorer) NextDoc() (int, error) { return s.iterator.NextDoc() }

// Advance positions the iterator at or beyond target.
func (s *ConstantScoreScorer) Advance(target int) (int, error) {
	return s.iterator.Advance(target)
}

// Cost returns the underlying iterator's cost estimate.
func (s *ConstantScoreScorer) Cost() int64 { return s.iterator.Cost() }

// DocIDRunEnd returns the exclusive end of the current run of
// consecutive doc IDs.
func (s *ConstantScoreScorer) DocIDRunEnd() (int, error) { return s.iterator.DocIDRunEnd() }

// Score returns the constant score this scorer was built with.
//
// Mirrors ConstantScoreScorer.score(), whose body is `return score;`.
func (s *ConstantScoreScorer) Score() (float32, error) { return s.score, nil }

// GetMaxScore returns the constant score regardless of upTo because
// every document carries the same score.
//
// Mirrors ConstantScoreScorer.getMaxScore(int), whose body is `return score;`.
func (s *ConstantScoreScorer) GetMaxScore(_ int) (float32, error) { return s.score, nil }

// TwoPhaseIterator mirrors ConstantScoreScorer.twoPhaseIterator(), whose body
// is `return twoPhaseIterator;` — nil when the scorer was built from a plain
// DocIdSetIterator, and the (possibly re-wrapped) two-phase view otherwise.
func (s *ConstantScoreScorer) TwoPhaseIterator() *TwoPhaseIterator {
	return s.twoPhaseIterator
}

// AdvanceShallow returns NO_MORE_DOCS, the default defined by
// org.apache.lucene.search.Scorer#advanceShallow. Lucene's ConstantScoreScorer
// does not override advanceShallow (it only overrides getMaxScore), so the
// whole remaining postings list is treated as a single block.
func (s *ConstantScoreScorer) AdvanceShallow(target int) (int, error) {
	return NO_MORE_DOCS, nil
}

// SetMinCompetitiveScore terminates iteration when minScore exceeds the
// constant score this scorer produces, but only in TOP_SCORES mode (where the
// iterator was wrapped to allow it). This is the Go port of
// ConstantScoreScorer.setMinCompetitiveScore: once minScore > score, no further
// document can be competitive, so the wrapped delegate is replaced with an
// empty iterator. In any other ScoreMode the call is a no-op.
func (s *ConstantScoreScorer) SetMinCompetitiveScore(minScore float32) error {
	if s.scoreMode == TOP_SCORES && minScore > s.score && s.wrapper != nil {
		s.wrapper.delegate = Empty()
	}
	return nil
}

// GetScoreMode returns the ScoreMode this scorer was constructed
// with. Exposed so suppliers can inspect the mode they propagated to
// the scorer; not part of the Lucene API but a thin getter that
// avoids handing out the unexported field.
func (s *ConstantScoreScorer) GetScoreMode() ScoreMode { return s.scoreMode }

// GetApproximation returns the approximation iterator handed to the
// constructor. In Lucene this is exposed via TwoPhaseIterator.
// approximation(); the helper here is the bare equivalent until that
// type lands.
func (s *ConstantScoreScorer) GetApproximation() DocIdSetIterator { return s.approximation }

// Ensure ConstantScoreScorer implements Scorer and the optional
// MinCompetitiveScorer extension.
var (
	_ Scorer               = (*ConstantScoreScorer)(nil)
	_ MinCompetitiveScorer = (*ConstantScoreScorer)(nil)
)

// IntoBitSet mirrors the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene 10.5.0.
func (c *constantScoreDISIWrapper) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return DefaultIntoBitSet(c, upTo, bitSet, offset)
}

// Iterator mirrors ConstantScoreScorer.iterator() of Apache Lucene 10.5.0,
// which returns the wrapped disi.
func (s *ConstantScoreScorer) Iterator() DocIdSetIterator {
	return s.iterator
}

// NextDocsAndScores mirrors the concrete body of
// Scorer.nextDocsAndScores(int, Bits, DocAndFloatFeatureBuffer) in Apache
// Lucene 10.5.0.
func (c *ConstantScoreScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) error {
	return DefaultNextDocsAndScores(c, upTo, liveDocs, buffer)
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (s *ConstantScoreScorer) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(s, upTo, bitSet, offset)
}
