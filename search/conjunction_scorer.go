package search

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ConjunctionScorer is the Scorer for conjunctions, sets of queries, all of
// which are required.
//
// Mirrors org.apache.lucene.search.ConjunctionScorer (Lucene 10.5.0,
// lucene/core/src/java/org/apache/lucene/search/ConjunctionScorer.java).
type ConjunctionScorer struct {
	BaseScorer
	disi     DocIdSetIterator
	scorers  []Scorer
	required []Scorer
}

// NewConjunctionScorer creates a new ConjunctionScorer; scorers must be a
// subset of required.
//
// Mirrors ConjunctionScorer(Collection<Scorer> required, Collection<Scorer> scorers):
//
//	this.disi = ConjunctionUtils.intersectScorers(required);
//	this.scorers = scorers.toArray(Scorer[]::new);
//	this.required = required;
func NewConjunctionScorer(required []Scorer, scorers []Scorer) *ConjunctionScorer {
	return &ConjunctionScorer{
		disi:     IntersectScorers(required),
		scorers:  scorers,
		required: required,
	}
}

// TwoPhaseIterator mirrors ConjunctionScorer.twoPhaseIterator(), whose body is
// `return TwoPhaseIterator.unwrap(disi);`.
func (s *ConjunctionScorer) TwoPhaseIterator() *TwoPhaseIterator {
	return Unwrap(s.disi)
}

// Iterator mirrors ConjunctionScorer.iterator(), whose body is `return disi;`.
func (s *ConjunctionScorer) Iterator() DocIdSetIterator {
	return s.disi
}

// DocID mirrors ConjunctionScorer.docID(), whose body is `return disi.docID();`.
func (s *ConjunctionScorer) DocID() int {
	return s.disi.DocID()
}

// Score mirrors ConjunctionScorer.score():
//
//	double sum = 0.0d;
//	for (Scorer scorer : scorers) {
//	  sum += scorer.score();
//	}
//	return (float) sum;
func (s *ConjunctionScorer) Score() (float32, error) {
	var sum float64
	for _, scorer := range s.scorers {
		v, err := scorer.Score()
		if err != nil {
			return 0, err
		}
		sum += float64(v)
	}
	return float32(sum), nil
}

// GetMaxScore mirrors ConjunctionScorer.getMaxScore(int):
//
//	double maxScore = 0;
//	for (Scorer s : scorers) {
//	  if (s.docID() <= upTo) {
//	    maxScore += s.getMaxScore(upTo);
//	  }
//	}
//	return (float) maxScore;
func (s *ConjunctionScorer) GetMaxScore(upTo int) (float32, error) {
	var maxScore float64
	for _, scorer := range s.scorers {
		if scorer.DocID() <= upTo {
			m, err := scorer.GetMaxScore(upTo)
			if err != nil {
				return 0, err
			}
			maxScore += float64(m)
		}
	}
	return float32(maxScore), nil
}

// AdvanceShallow mirrors ConjunctionScorer.advanceShallow(int):
//
//	if (scorers.length == 1) {
//	  return scorers[0].advanceShallow(target);
//	}
//	for (Scorer scorer : scorers) {
//	  scorer.advanceShallow(target);
//	}
//	return super.advanceShallow(target);
func (s *ConjunctionScorer) AdvanceShallow(target int) (int, error) {
	if len(s.scorers) == 1 {
		return s.scorers[0].AdvanceShallow(target)
	}
	for _, scorer := range s.scorers {
		if _, err := scorer.AdvanceShallow(target); err != nil {
			return 0, err
		}
	}
	return s.BaseScorer.AdvanceShallow(target)
}

// SetMinCompetitiveScore mirrors ConjunctionScorer.setMinCompetitiveScore(float):
//
//	// This scorer is only used for TOP_SCORES when there is a single scoring clause
//	if (scorers.length == 1) {
//	  scorers[0].setMinCompetitiveScore(minScore);
//	}
func (s *ConjunctionScorer) SetMinCompetitiveScore(minScore float32) error {
	if len(s.scorers) == 1 {
		return s.scorers[0].SetMinCompetitiveScore(minScore)
	}
	return nil
}

// GetChildren mirrors ConjunctionScorer.getChildren():
//
//	ArrayList<ChildScorable> children = new ArrayList<>();
//	for (Scorer scorer : required) {
//	  children.add(new ChildScorable(scorer, "MUST"));
//	}
//	return children;
func (s *ConjunctionScorer) GetChildren() ([]ChildScorable, error) {
	children := make([]ChildScorable, 0, len(s.required))
	for _, scorer := range s.required {
		children = append(children, ChildScorable{Child: scorer, Relationship: "MUST"})
	}
	return children, nil
}

// NextDocsAndScores mirrors the concrete body of
// Scorer.nextDocsAndScores(int, Bits, DocAndFloatFeatureBuffer) in Apache
// Lucene 10.5.0, which ConjunctionScorer inherits unchanged.
func (s *ConjunctionScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) error {
	return DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}

var _ Scorer = (*ConjunctionScorer)(nil)
