package search

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ReqExclScorer is a Scorer for queries with a required subscorer and an
// excluding (prohibited) sub Scorer.
//
// Mirrors org.apache.lucene.search.ReqExclScorer (Lucene 10.5.0,
// lucene/core/src/java/org/apache/lucene/search/ReqExclScorer.java).
type ReqExclScorer struct {
	BaseScorer

	reqScorer Scorer
	// approximations of the scorers, or the scorers themselves if they don't
	// support approximations
	reqApproximation  DocIdSetIterator
	exclApproximation DocIdSetIterator
	// two-phase views of the scorers, or nil if they do not support approximations
	reqTwoPhaseIterator  *TwoPhaseIterator
	exclTwoPhaseIterator *TwoPhaseIterator
}

// NewReqExclScorer constructs a ReqExclScorer.
//
// Mirrors ReqExclScorer(Scorer reqScorer, Scorer exclScorer):
//
//	this.reqScorer = reqScorer;
//	reqTwoPhaseIterator = reqScorer.twoPhaseIterator();
//	if (reqTwoPhaseIterator == null) { reqApproximation = reqScorer.iterator(); }
//	else { reqApproximation = reqTwoPhaseIterator.approximation(); }
//	exclTwoPhaseIterator = exclScorer.twoPhaseIterator();
//	if (exclTwoPhaseIterator == null) { exclApproximation = exclScorer.iterator(); }
//	else { exclApproximation = exclTwoPhaseIterator.approximation(); }
func NewReqExclScorer(reqScorer Scorer, exclScorer Scorer) *ReqExclScorer {
	s := &ReqExclScorer{reqScorer: reqScorer}
	s.reqTwoPhaseIterator = reqScorer.TwoPhaseIterator()
	if s.reqTwoPhaseIterator == nil {
		s.reqApproximation = reqScorer.Iterator()
	} else {
		s.reqApproximation = s.reqTwoPhaseIterator.Approximation()
	}
	s.exclTwoPhaseIterator = exclScorer.TwoPhaseIterator()
	if s.exclTwoPhaseIterator == nil {
		s.exclApproximation = exclScorer.Iterator()
	} else {
		s.exclApproximation = s.exclTwoPhaseIterator.Approximation()
	}
	return s
}

// reqExclMatchesOrNull confirms whether or not the given TwoPhaseIterator
// matches on the current document.
//
// Mirrors the private static helper
// `matchesOrNull(TwoPhaseIterator it) { return it == null || it.matches(); }`.
func reqExclMatchesOrNull(it *TwoPhaseIterator) (bool, error) {
	if it == nil {
		return true, nil
	}
	return it.Matches()
}

// Iterator mirrors ReqExclScorer.iterator(), whose body is
// `return TwoPhaseIterator.asDocIdSetIterator(twoPhaseIterator());`.
func (s *ReqExclScorer) Iterator() DocIdSetIterator {
	return AsDocIdSetIterator(s.TwoPhaseIterator())
}

// DocID mirrors ReqExclScorer.docID(), whose body is
// `return reqApproximation.docID();`.
func (s *ReqExclScorer) DocID() int {
	return s.reqApproximation.DocID()
}

// Score mirrors ReqExclScorer.score(), whose body is `return reqScorer.score();`.
func (s *ReqExclScorer) Score() (float32, error) {
	return s.reqScorer.Score()
}

// AdvanceShallow mirrors ReqExclScorer.advanceShallow(int), whose body is
// `return reqScorer.advanceShallow(target);`.
func (s *ReqExclScorer) AdvanceShallow(target int) (int, error) {
	return s.reqScorer.AdvanceShallow(target)
}

// GetMaxScore mirrors ReqExclScorer.getMaxScore(int), whose body is
// `return reqScorer.getMaxScore(upTo);`.
func (s *ReqExclScorer) GetMaxScore(upTo int) (float32, error) {
	return s.reqScorer.GetMaxScore(upTo)
}

// SetMinCompetitiveScore mirrors
// ReqExclScorer.setMinCompetitiveScore(float), whose body is
// `reqScorer.setMinCompetitiveScore(score);` — the score of this scorer is the
// same as the score of reqScorer.
func (s *ReqExclScorer) SetMinCompetitiveScore(score float32) error {
	return s.reqScorer.SetMinCompetitiveScore(score)
}

// GetChildren mirrors ReqExclScorer.getChildren(), whose body is
// `return Collections.singleton(new ChildScorable(reqScorer, "MUST"));`.
func (s *ReqExclScorer) GetChildren() ([]ChildScorable, error) {
	return []ChildScorable{{Child: s.reqScorer, Relationship: "MUST"}}, nil
}

// reqExclAdvanceCost is the estimation of the number of operations required to
// call DISI.advance.
//
// Mirrors `private static final int ADVANCE_COST = 10;`.
const reqExclAdvanceCost = 10

// reqExclMatchCost mirrors the private static
// matchCost(DocIdSetIterator, TwoPhaseIterator, DocIdSetIterator, TwoPhaseIterator).
func reqExclMatchCost(
	reqApproximation DocIdSetIterator,
	reqTwoPhaseIterator *TwoPhaseIterator,
	exclApproximation DocIdSetIterator,
	exclTwoPhaseIterator *TwoPhaseIterator,
) float32 {
	matchCost := float32(2) // we perform 2 comparisons to advance exclApproximation
	if reqTwoPhaseIterator != nil {
		// this two-phase iterator must always be matched
		matchCost += reqTwoPhaseIterator.MatchCost()
	}

	// match cost of the prohibited clause: we need to advance the approximation
	// and match the two-phased iterator
	exclMatchCost := float32(reqExclAdvanceCost)
	if exclTwoPhaseIterator != nil {
		exclMatchCost += exclTwoPhaseIterator.MatchCost()
	}

	// upper value for the ratio of documents that reqApproximation matches that
	// exclApproximation also matches
	var ratio float32
	reqCost := reqApproximation.Cost()
	exclCost := exclApproximation.Cost()
	switch {
	case reqCost <= 0:
		ratio = 1
	case exclCost <= 0:
		ratio = 0
	default:
		minCost := reqCost
		if exclCost < minCost {
			minCost = exclCost
		}
		ratio = float32(minCost) / float32(reqCost)
	}
	matchCost += ratio * exclMatchCost

	return matchCost
}

// TwoPhaseIterator mirrors ReqExclScorer.twoPhaseIterator(): it returns a
// two-phase view that checks the cheaper of the two verification steps first.
func (s *ReqExclScorer) TwoPhaseIterator() *TwoPhaseIterator {
	matchCost := reqExclMatchCost(s.reqApproximation, s.reqTwoPhaseIterator, s.exclApproximation, s.exclTwoPhaseIterator)

	// exclDocOnCurrent positions exclApproximation on the current required doc,
	// mirroring the shared prologue of both anonymous matches() bodies:
	//
	//	final int doc = reqApproximation.docID();
	//	int exclDoc = exclApproximation.docID();
	//	if (exclDoc < doc) { exclDoc = exclApproximation.advance(doc); }
	exclDocOnCurrent := func() (doc int, exclDoc int, err error) {
		doc = s.reqApproximation.DocID()
		exclDoc = s.exclApproximation.DocID()
		if exclDoc < doc {
			exclDoc, err = s.exclApproximation.Advance(doc)
			if err != nil {
				return 0, 0, err
			}
		}
		return doc, exclDoc, nil
	}

	if s.reqTwoPhaseIterator == nil ||
		(s.exclTwoPhaseIterator != nil && s.reqTwoPhaseIterator.MatchCost() <= s.exclTwoPhaseIterator.MatchCost()) {
		// reqTwoPhaseIterator is LESS costly than exclTwoPhaseIterator, check it first
		return NewTwoPhaseIteratorWithMatchCost(s.reqApproximation, func() (bool, error) {
			doc, exclDoc, err := exclDocOnCurrent()
			if err != nil {
				return false, err
			}
			if exclDoc != doc {
				return reqExclMatchesOrNull(s.reqTwoPhaseIterator)
			}
			reqMatches, err := reqExclMatchesOrNull(s.reqTwoPhaseIterator)
			if err != nil {
				return false, err
			}
			if !reqMatches {
				return false, nil
			}
			exclMatches, err := reqExclMatchesOrNull(s.exclTwoPhaseIterator)
			if err != nil {
				return false, err
			}
			return !exclMatches, nil
		}, matchCost)
	}

	// reqTwoPhaseIterator is MORE costly than exclTwoPhaseIterator, check it last
	return NewTwoPhaseIteratorWithMatchCost(s.reqApproximation, func() (bool, error) {
		doc, exclDoc, err := exclDocOnCurrent()
		if err != nil {
			return false, err
		}
		if exclDoc != doc {
			return reqExclMatchesOrNull(s.reqTwoPhaseIterator)
		}
		exclMatches, err := reqExclMatchesOrNull(s.exclTwoPhaseIterator)
		if err != nil {
			return false, err
		}
		if exclMatches {
			return false, nil
		}
		return reqExclMatchesOrNull(s.reqTwoPhaseIterator)
	}, matchCost)
}

// NextDocsAndScores mirrors the concrete body of
// Scorer.nextDocsAndScores(int, Bits, DocAndFloatFeatureBuffer) in Apache
// Lucene 10.5.0, which ReqExclScorer inherits unchanged.
func (s *ReqExclScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) error {
	return DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}

var _ Scorer = (*ReqExclScorer)(nil)
