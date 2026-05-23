// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/ConjunctionSpans.java

package spans

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ConjunctionSpans is the common base for span iterators that require multiple
// sub spans to match in a document.
//
// Mirrors org.apache.lucene.queries.spans.ConjunctionSpans (abstract, package-private).
//
// Deviations from Java:
//   - Java's abstract class uses ConjunctionUtils.createConjunction to build the
//     conjunction DISI.  Gocene uses search.CreateConjunctionFromLists and
//     search.AddIteratorToConjunctionLists / AddTwoPhaseIteratorToConjunctionLists.
//   - TwoPhaseCurrentDocMatches is a function field instead of an abstract method.
type ConjunctionSpans struct {
	// SubSpans holds the sub-span iterators in query order.
	SubSpans []Spans
	// Conjunction is used to advance to the next doc that has all clauses.
	Conjunction search.DocIdSetIterator
	// AtFirstInCurrentDoc is true when no position has been enumerated yet in
	// the current document (ensures start/end return -1 before the first call
	// to NextStartPosition).
	AtFirstInCurrentDoc bool
	// OneExhaustedInCurrentDoc is true when at least one sub-span ran out of
	// positions in the current document.
	OneExhaustedInCurrentDoc bool

	// twoPhaseCurrentDocMatches is called by toMatchDoc to test whether the
	// current document actually contains matching span positions.  Subclasses
	// set this field in their constructor.
	twoPhaseCurrentDocMatches func() (bool, error)

	// totalMatchCost is the sum of position costs / TwoPhaseIterator match
	// costs across all sub-spans, used to build the TwoPhaseIterator.
	totalMatchCost float32
}

// NewConjunctionSpans constructs a ConjunctionSpans from the given sub-spans.
// matchFn implements the abstract twoPhaseCurrentDocMatches logic.
func NewConjunctionSpans(subSpans []Spans, matchFn func() (bool, error)) (*ConjunctionSpans, error) {
	if len(subSpans) < 2 {
		return nil, errTooFewSubSpans(len(subSpans))
	}

	// Build the conjunction DISI using the search-package helpers.
	allIters := make([]search.DocIdSetIterator, 0, len(subSpans))
	twoPhaseIters := make([]*search.TwoPhaseIterator, 0)
	var totalMatchCost float32
	for _, s := range subSpans {
		tpi := s.AsTwoPhaseIterator()
		if tpi != nil {
			search.AddTwoPhaseIteratorToConjunctionLists(tpi, &allIters, &twoPhaseIters)
			totalMatchCost += tpi.MatchCost()
		} else {
			search.AddIteratorToConjunctionLists(s, &allIters, &twoPhaseIters)
			totalMatchCost += s.PositionsCost()
		}
	}
	conjunction := search.CreateConjunctionFromLists(allIters, twoPhaseIters)

	cs := &ConjunctionSpans{
		SubSpans:            make([]Spans, len(subSpans)),
		Conjunction:         conjunction,
		AtFirstInCurrentDoc: true,
		totalMatchCost:      totalMatchCost,
	}
	copy(cs.SubSpans, subSpans)
	cs.twoPhaseCurrentDocMatches = matchFn
	return cs, nil
}

// errTooFewSubSpans returns an error for fewer than 2 sub-spans.
func errTooFewSubSpans(n int) error {
	return &tooFewSubSpansError{n: n}
}

type tooFewSubSpansError struct{ n int }

func (e *tooFewSubSpansError) Error() string {
	return "ConjunctionSpans: need at least 2 sub-spans, got " + itoa(e.n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append(buf, byte('0'+n%10))
		n /= 10
	}
	if neg {
		buf = append(buf, '-')
	}
	// reverse
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}

// DocID returns the current document ID.
func (cs *ConjunctionSpans) DocID() int { return cs.Conjunction.DocID() }

// Cost returns the estimated iteration cost.
func (cs *ConjunctionSpans) Cost() int64 { return cs.Conjunction.Cost() }

// DocIDRunEnd returns the conservative upper bound for the current run.
func (cs *ConjunctionSpans) DocIDRunEnd() int { return cs.DocID() + 1 }

// NextDoc advances to the next matching document.
func (cs *ConjunctionSpans) NextDoc() (int, error) {
	docID, err := cs.Conjunction.NextDoc()
	if err != nil {
		return 0, err
	}
	if docID == search.NO_MORE_DOCS {
		return search.NO_MORE_DOCS, nil
	}
	return cs.toMatchDoc()
}

// Advance advances to the first matching document >= target.
func (cs *ConjunctionSpans) Advance(target int) (int, error) {
	docID, err := cs.Conjunction.Advance(target)
	if err != nil {
		return 0, err
	}
	if docID == search.NO_MORE_DOCS {
		return search.NO_MORE_DOCS, nil
	}
	return cs.toMatchDoc()
}

// toMatchDoc advances through documents until one passes twoPhaseCurrentDocMatches.
func (cs *ConjunctionSpans) toMatchDoc() (int, error) {
	cs.OneExhaustedInCurrentDoc = false
	for {
		matched, err := cs.twoPhaseCurrentDocMatches()
		if err != nil {
			return 0, err
		}
		if matched {
			return cs.DocID(), nil
		}
		docID, err := cs.Conjunction.NextDoc()
		if err != nil {
			return 0, err
		}
		if docID == search.NO_MORE_DOCS {
			return search.NO_MORE_DOCS, nil
		}
	}
}

// AsTwoPhaseIterator returns a TwoPhaseIterator view of this ConjunctionSpans.
func (cs *ConjunctionSpans) AsTwoPhaseIterator() *search.TwoPhaseIterator {
	matchCost := cs.totalMatchCost
	matchFn := cs.twoPhaseCurrentDocMatches
	return search.NewTwoPhaseIteratorWithMatchCost(
		cs.Conjunction,
		func() (bool, error) { return matchFn() },
		matchCost,
	)
}

// PositionsCost panics: asTwoPhaseIterator never returns nil for ConjunctionSpans.
func (cs *ConjunctionSpans) PositionsCost() float32 {
	panic("ConjunctionSpans.PositionsCost() is not supported; use AsTwoPhaseIterator")
}

// GetSubSpans returns the sub-span array.
func (cs *ConjunctionSpans) GetSubSpans() []Spans { return cs.SubSpans }
