package search

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TwoPhaseIterator is returned by a Scorer to expose an approximation of a DocIdSetIterator.
// When the Approximation()'s NextDoc() or Advance() return, Matches() needs to be checked
// to know whether the returned doc ID actually matches.
type TwoPhaseIterator interface {
	// Approximation returns a DocIdSetIterator that is a superset of the matching documents.
	Approximation() DocIdSetIterator

	// Matches returns whether the current doc ID that Approximation() is on matches.
	// This method should only be called when the iterator is positioned -- ie. not
	// when DocID() is -1 or NO_MORE_DOCS -- and at most once.
	Matches() (bool, error)

	// MatchCost returns an estimate of the expected cost to determine that a single
	// document Matches(). Returns an expected cost in number of simple operations
	// like addition, multiplication, comparing two numbers and indexing an array.
	// The returned value must be positive.
	MatchCost() float32

	// DocIDRunEnd returns the end of the run of consecutive doc IDs that match
	// this TwoPhaseIterator and that contains the current doc ID of the approximation.
	// Returns one plus the last doc ID of the run.
	DocIDRunEnd() (int, error)

	// IntoBitSet loads the doc IDs that both belong to the Approximation() and Matches() match,
	// and are in [Approximation().DocID(), upTo), into bitSet, the document whose ID
	// is i being stored at bit i - offset. Upon return the Approximation()
	// is positioned on the first doc that is >= upTo, mirroring DocIdSetIterator.IntoBitSet.
	IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error
}

// AsDocIdSetIterator returns a DocIdSetIterator view of the provided TwoPhaseIterator.
func AsDocIdSetIterator(tpi TwoPhaseIterator) DocIdSetIterator {
	return &twoPhaseIteratorAsDocIdSetIterator{
		twoPhaseIterator: tpi,
		in:               tpi.Approximation(),
	}
}

// UnwrapTwoPhaseIterator returns the wrapped TwoPhaseIterator if the given DocIdSetIterator
// was created with AsDocIdSetIterator. Otherwise it returns nil.
func UnwrapTwoPhaseIterator(iter DocIdSetIterator) TwoPhaseIterator {
	if tpi, ok := iter.(*twoPhaseIteratorAsDocIdSetIterator); ok {
		return tpi.twoPhaseIterator
	}
	return nil
}

type twoPhaseIteratorAsDocIdSetIterator struct {
	twoPhaseIterator TwoPhaseIterator
	in               DocIdSetIterator
}

func (it *twoPhaseIteratorAsDocIdSetIterator) DocID() int {
	return it.in.DocID()
}

func (it *twoPhaseIteratorAsDocIdSetIterator) NextDoc() (int, error) {
	doc, err := it.in.NextDoc()
	if err != nil {
		return 0, err
	}
	for {
		if doc == NO_MORE_DOCS {
			return NO_MORE_DOCS, nil
		}
		matches, err := it.twoPhaseIterator.Matches()
		if err != nil {
			return 0, err
		}
		if matches {
			return doc, nil
		}
		doc, err = it.in.NextDoc()
		if err != nil {
			return 0, err
		}
	}
}

func (it *twoPhaseIteratorAsDocIdSetIterator) Advance(target int) (int, error) {
	doc, err := it.in.Advance(target)
	if err != nil {
		return 0, err
	}
	for {
		if doc == NO_MORE_DOCS {
			return NO_MORE_DOCS, nil
		}
		matches, err := it.twoPhaseIterator.Matches()
		if err != nil {
			return 0, err
		}
		if matches {
			return doc, nil
		}
		doc, err = it.in.NextDoc()
		if err != nil {
			return 0, err
		}
	}
}

func (it *twoPhaseIteratorAsDocIdSetIterator) DocIDRunEnd() int {
	return it.in.DocIDRunEnd()
}

func (it *twoPhaseIteratorAsDocIdSetIterator) Cost() int64 {
	return it.in.Cost()
}

// DefaultDocIDRunEnd provides the default implementation of DocIDRunEnd.
func DefaultDocIDRunEnd(tpi TwoPhaseIterator) (int, error) {
	return tpi.Approximation().DocID(), nil
}

// DefaultIntoBitSet provides the default implementation of IntoBitSet.
func DefaultIntoBitSet(tpi TwoPhaseIterator, upTo int, bitSet *util.FixedBitSet, offset int) error {
	approx := tpi.Approximation()
	for doc := approx.DocID(); doc < upTo; {
		matches, err := tpi.Matches()
		if err != nil {
			return err
		}
		if matches {
			bitSet.Set(doc - offset)
		}
		doc, err = approx.NextDoc()
		if err != nil {
			return err
		}
	}
	return nil
}
