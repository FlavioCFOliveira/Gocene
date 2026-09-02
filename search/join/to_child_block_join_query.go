package join

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

const (
	InvalidQueryMessage = "Parent query must not match any docs besides parent filter. Combine them as must (+) and must-not (-) clauses to find a problem doc. docID="
	IllegalAdvanceOnParent = "Expect to be advanced on child docs only. got docID="
)

// ToChildBlockJoinQuery joins in reverse: you provide a Query
// matching parent documents and it joins down to child documents.
type ToChildBlockJoinQuery struct {
	search.BaseQuery
	parentsFilter BitSetProducer
	parentQuery   search.Query
}

// NewToChildBlockJoinQuery creates a ToChildBlockJoinQuery.
func NewToChildBlockJoinQuery(parentQuery search.Query, parentsFilter BitSetProducer) *ToChildBlockJoinQuery {
	return &ToChildBlockJoinQuery{
		parentsFilter: parentsFilter,
		parentQuery:   parentQuery,
	}
}

func (q *ToChildBlockJoinQuery) Visit(visitor search.QueryVisitor) {
	visitor.VisitLeaf(q)
}

func (q *ToChildBlockJoinQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	parentWeight, err := q.parentQuery.CreateWeight(searcher, needsScores, boost)
	if err != nil {
		return nil, err
	}
	return &toChildBlockJoinWeight{
		FilterWeight:  *search.NewFilterWeightWithQuery(q, parentWeight),
		parentsFilter: q.parentsFilter,
		doScores:      needsScores,
	}, nil
}

func (q *ToChildBlockJoinQuery) GetParentQuery() search.Query {
	return q.parentQuery
}

type toChildBlockJoinWeight struct {
	search.FilterWeight
	parentsFilter BitSetProducer
	doScores      bool
}

func (w *toChildBlockJoinWeight) ScorerSupplier(context *index.LeafReaderContext) (search.ScorerSupplier, error) {
	parentScorer, err := w.in.Scorer(context)
	if err != nil {
		return nil, err
	}
	if parentScorer == nil {
		return nil, nil
	}

	parents, err := w.parentsFilter.GetBitSet(context)
	if err != nil {
		return nil, err
	}
	if parents == nil {
		return nil, nil
	}

	scorer := newToChildBlockJoinScorer(parentScorer, parents, w.doScores)
	return &defaultScorerSupplier{scorer: scorer}, nil
}

type defaultScorerSupplier struct {
	scorer search.Scorer
}

func (s *defaultScorerSupplier) Get(leadCost int64) (search.Scorer, error) {
	return s.scorer, nil
}

func (s *defaultScorerSupplier) Cost() int64 {
	return s.scorer.Cost()
}

func (s *defaultScorerSupplier) SetTopLevelScoringClause() error {
	return s.scorer.SetMinCompetitiveScore(0) // Simple propagation
}

func (s *defaultScorerSupplier) BulkScorer() (search.BulkScorer, error) {
	return search.NewDefaultBulkScorer(s.scorer), nil
}

func (w *toChildBlockJoinWeight) Explain(context *index.LeafReaderContext, doc int) (search.Explanation, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	bjScorer := scorer.(*toChildBlockJoinScorer)
	if bjScorer.iterator().DocID() != doc {
		if _, err := bjScorer.iterator().Advance(doc); err != nil {
			return nil, err
		}
		if bjScorer.iterator().DocID() != doc {
			return search.ExplanationNoMatch("Not a match"), nil
		}
	}
	parentDoc := bjScorer.getParentDoc()
	return search.ExplanationMatch(
		bjScorer.Score(),
		fmt.Sprintf("Score based on parent document %d", parentDoc+context.DocBase),
		w.in.Explain(context, parentDoc),
	), nil
}

type toChildBlockJoinScorer struct {
	search.Scorer
	parentScorer search.Scorer
	parentIt     util.DocIdSetIterator
	parentBits   util.BitSet
	doScores     bool
	parentScore  float32
	childDoc     int
	parentDoc    int
}

func newToChildBlockJoinScorer(parentScorer search.Scorer, parentBits util.BitSet, doScores bool) *toChildBlockJoinScorer {
	return &toChildBlockJoinScorer{
		parentScorer: parentScorer,
		parentIt:     parentScorer.Iterator(),
		parentBits:   parentBits,
		doScores:     doScores,
		childDoc:     -1,
		parentDoc:    0,
	}
}

func (s *toChildBlockJoinScorer) GetChildren() ([]search.ChildScorable, error) {
	return []search.ChildScorable{{Child: s.parentScorer, Relationship: "BLOCK_JOIN"}}, nil
}

func (s *toChildBlockJoinScorer) Iterator() util.DocIdSetIterator {
	return &toChildBlockJoinIterator{scorer: s}
}

type toChildBlockJoinIterator struct {
	scorer *toChildBlockJoinScorer
}

func (it *toChildBlockJoinIterator) DocID() int {
	return it.scorer.childDoc
}

func (it *toChildBlockJoinIterator) NextDoc() (int, error) {
	for {
		if it.scorer.childDoc+1 == it.scorer.parentDoc {
			for {
				nextParent, err := it.scorer.parentIt.NextDoc()
				if err != nil {
					return 0, err
				}
				it.scorer.parentDoc = nextParent
				if err := it.scorer.validateParentDoc(); err != nil {
					return 0, err
				}

				if it.scorer.parentDoc == search.NO_MORE_DOCS {
					it.scorer.childDoc = search.NO_MORE_DOCS
					return it.scorer.childDoc, nil
				}

				it.scorer.childDoc = 1 + it.scorer.parentBits.PrevSetBit(it.scorer.parentDoc-1)
				if it.scorer.childDoc == it.scorer.parentDoc {
					continue
				}
				if it.scorer.childDoc < it.scorer.parentDoc {
					if it.scorer.doScores {
						score, _ := it.scorer.parentScorer.Score()
						it.scorer.parentScore = score
					}
					return it.scorer.childDoc, nil
				}
			}
		} else {
			it.scorer.childDoc++
			return it.scorer.childDoc, nil
		}
	}
}

func (it *toChildBlockJoinIterator) Advance(childTarget int) (int, error) {
	if childTarget >= it.scorer.parentDoc {
		if childTarget == search.NO_MORE_DOCS {
			it.scorer.childDoc = search.NO_MORE_DOCS
			it.scorer.parentDoc = search.NO_MORE_DOCS
			return it.scorer.childDoc, nil
		}
		nextParent, err := it.scorer.parentIt.Advance(childTarget + 1)
		if err != nil {
			return 0, err
		}
		it.scorer.parentDoc = nextParent
		if err := it.scorer.validateParentDoc(); err != nil {
			return 0, err
		}

		for {
			firstChild := it.scorer.parentBits.PrevSetBit(it.scorer.parentDoc-1) + 1
			if firstChild != it.scorer.parentDoc {
				if firstChild < it.scorer.parentDoc {
					if childTarget > firstChild {
						childTarget = firstChild
					}
					if it.scorer.doScores {
						score, _ := it.scorer.parentScorer.Score()
						it.scorer.parentScore = score
					}
					it.scorer.childDoc = childTarget
					return it.scorer.childDoc, nil
				}
			}
			nextParent, err := it.scorer.parentIt.NextDoc()
			if err != nil {
				return 0, err
			}
			it.scorer.parentDoc = nextParent
			if err := it.scorer.validateParentDoc(); err != nil {
				return 0, err
			}
			if it.scorer.parentDoc == search.NO_MORE_DOCS {
				it.scorer.childDoc = search.NO_MORE_DOCS
				return it.scorer.childDoc, nil
			}
		}
	}
	it.scorer.childDoc = childTarget
	return it.scorer.childDoc, nil
}

func (it *toChildBlockJoinIterator) Cost() int64 {
	return it.scorer.parentIt.Cost()
}

func (s *toChildBlockJoinScorer) validateParentDoc() error {
	if s.parentDoc != search.NO_MORE_DOCS && !s.parentBits.Get(s.parentDoc) {
		return fmt.Errorf("%s%d", InvalidQueryMessage, s.parentDoc)
	}
	return nil
}

func (s *toChildBlockJoinScorer) DocID() int {
	return s.childDoc
}

func (s *toChildBlockJoinScorer) Score() (float32, error) {
	return s.parentScore, nil
}

func (s *toChildBlockJoinScorer) GetMaxScore(upTo int) (float32, error) {
	return 1e38, nil
}

func (s *toChildBlockJoinScorer) getParentDoc() int {
	return s.parentDoc
}

func (s *toChildBlockJoinScorer) getParentDocInternal() int {
	return s.parentDoc
}

// ParentDoc is a getter for the current parent doc.
func (s *toChildBlockJoinScorer) ParentDoc() int {
	return s.parentDoc
}
