package search

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TwoPhaseVerifier defines the verification phase of a two-phase iterator.
type TwoPhaseVerifier interface {
	// Matches returns whether the current doc ID of the approximation matches.
	Matches() (bool, error)
	// MatchCost returns an estimate of the expected cost to determine if a single document matches.
	MatchCost() float32
}

// TwoPhaseIterator exposes an approximation of a DocIdSetIterator.
// When the Approximation()'s NextDoc() or Advance() return, Matches() needs to be checked
// in order to know whether the returned doc ID actually matches.
type TwoPhaseIterator struct {
	approximation DocIdSetIterator
	verifier      TwoPhaseVerifier
}

// NewTwoPhaseIterator creates a new TwoPhaseIterator with the given approximation and verifier.
func NewTwoPhaseIterator(approx DocIdSetIterator, verifier TwoPhaseVerifier) *TwoPhaseIterator {
	return &TwoPhaseIterator{
		approximation: approx,
		verifier:      verifier,
	}
}

// Approximation returns a DocIdSetIterator that is a superset of the matching documents.
func (t *TwoPhaseIterator) Approximation() DocIdSetIterator {
	return t.approximation
}

// Matches returns whether the current doc ID that Approximation() is on matches.
// This method should only be called when the iterator is positioned and at most once.
func (t *TwoPhaseIterator) Matches() (bool, error) {
	return t.verifier.Matches()
}

// MatchCost returns an estimate of the expected cost to determine that a single document matches.
func (t *TwoPhaseIterator) MatchCost() float32 {
	return t.verifier.MatchCost()
}

// DocIDRunEnd returns the end of the run of consecutive doc IDs that match this TwoPhaseIterator
// and that contains the current doc ID of the approximation.
// DocIDRunEnd renders TwoPhaseIterator.docIDRunEnd(). The Java default body
// returns the current doc ID of the approximation; a subclass that overrides
// the method is rendered by a verifier implementing
// [TwoPhaseDocIDRunEndOverride], whose DocIDRunEnd is then dispatched to.
func (t *TwoPhaseIterator) DocIDRunEnd() (int, error) {
	if o, ok := t.verifier.(TwoPhaseDocIDRunEndOverride); ok {
		return o.DocIDRunEnd()
	}
	return t.approximation.DocID(), nil
}

// TwoPhaseDocIDRunEndOverride is implemented by a [TwoPhaseVerifier] whose Java
// TwoPhaseIterator subclass overrides docIDRunEnd(). An override that falls
// back to super.docIDRunEnd() returns the approximation's current doc ID.
type TwoPhaseDocIDRunEndOverride interface {
	DocIDRunEnd() (int, error)
}

// IntoBitSet loads the doc IDs that both belong to the Approximation() and Matches() match,
// and are in [approximation().DocID(), upTo), into bitSet.
func (t *TwoPhaseIterator) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	approx := t.approximation
	for doc := approx.DocID(); doc < upTo; {
		match, err := t.Matches()
		if err != nil {
			return err
		}
		if match {
			bitSet.Set(doc - offset)
		}
		doc, err = approx.NextDoc()
		if err != nil {
			return err
		}
	}
	return nil
}

// AsDocIdSetIterator returns a DocIdSetIterator view of the provided TwoPhaseIterator.
func AsDocIdSetIterator(t *TwoPhaseIterator) DocIdSetIterator {
	return &twoPhaseIteratorAsDocIdSetIterator{t: t}
}

// Unwrap returns the wrapped TwoPhaseIterator if the given iterator was created with AsDocIdSetIterator.
func Unwrap(iter DocIdSetIterator) *TwoPhaseIterator {
	if t, ok := iter.(*twoPhaseIteratorAsDocIdSetIterator); ok {
		return t.t
	}
	return nil
}

type twoPhaseIteratorAsDocIdSetIterator struct {
	t *TwoPhaseIterator
}

func (i *twoPhaseIteratorAsDocIdSetIterator) DocID() int {
	return i.t.approximation.DocID()
}

func (i *twoPhaseIteratorAsDocIdSetIterator) NextDoc() (int, error) {
	doc, err := i.t.approximation.NextDoc()
	if err != nil {
		return 0, err
	}
	return i.doNext(doc)
}

func (i *twoPhaseIteratorAsDocIdSetIterator) Advance(target int) (int, error) {
	doc, err := i.t.approximation.Advance(target)
	if err != nil {
		return 0, err
	}
	return i.doNext(doc)
}

// DocIDRunEnd renders the docIDRunEnd() TwoPhaseIteratorAsDocIdSetIterator
// inherits from DocIdSetIterator through FilterDocIdSetIterator (which does not
// override it): docID() + 1. Delegating to the approximation would report a run
// of unverified candidates as matches.
func (i *twoPhaseIteratorAsDocIdSetIterator) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(i)
}

func (i *twoPhaseIteratorAsDocIdSetIterator) Cost() int64 {
	return int64(i.t.approximation.Cost())
}

func (i *twoPhaseIteratorAsDocIdSetIterator) doNext(doc int) (int, error) {
	for {
		if doc == NO_MORE_DOCS {
			return NO_MORE_DOCS, nil
		}
		match, err := i.t.Matches()
		if err != nil {
			return 0, err
		}
		if match {
			return doc, nil
		}
		doc, err = i.t.approximation.NextDoc()
		if err != nil {
			return 0, err
		}
	}
}

// NewTwoPhaseIteratorWithMatchCost builds a TwoPhaseIterator over approx whose
// Matches() calls matches and whose MatchCost() returns matchCost.
//
// It is the Go rendering of the anonymous TwoPhaseIterator subclass that
// Lucene creates wherever a query needs a one-off verification step:
//
//	new TwoPhaseIterator(approximation) {
//	  @Override public boolean matches() throws IOException { ... }
//	  @Override public float matchCost() { return matchCost; }
//	}
//
// Java's TwoPhaseIterator declares matches() and matchCost() abstract; this
// port renders that pair as the TwoPhaseVerifier interface, so a one-off
// subclass becomes a verifier built from a function and a constant cost.
func NewTwoPhaseIteratorWithMatchCost(
	approx DocIdSetIterator,
	matches func() (bool, error),
	matchCost float32,
) *TwoPhaseIterator {
	return NewTwoPhaseIterator(approx, funcTwoPhaseVerifier{matches: matches, matchCost: matchCost})
}

// funcTwoPhaseVerifier adapts a matches function and a constant match cost to
// TwoPhaseVerifier.
type funcTwoPhaseVerifier struct {
	matches   func() (bool, error)
	matchCost float32
}

// Matches calls the wrapped matches function.
func (v funcTwoPhaseVerifier) Matches() (bool, error) { return v.matches() }

// MatchCost returns the constant match cost supplied at construction time.
func (v funcTwoPhaseVerifier) MatchCost() float32 { return v.matchCost }

// IntoBitSet mirrors the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene 10.5.0.
func (t *twoPhaseIteratorAsDocIdSetIterator) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return DefaultIntoBitSet(t, upTo, bitSet, offset)
}
