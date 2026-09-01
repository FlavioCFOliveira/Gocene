package join

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ParentsChildrenBlockJoinQuery returns the matching child documents for matching parent documents
// indexed together in the same block.
type ParentsChildrenBlockJoinQuery struct {
	search.BaseQuery
	parentFilter         BitSetProducer
	parentQuery          search.Query
	childQuery           search.Query
	childLimitPerParent  int
	scoreCombiner       func(float32, float32) float32
}

const DefaultChildLimitPerParent = 2147483647

// NewParentsChildrenBlockJoinQuery creates a ParentsChildrenBlockJoinQuery.
func NewParentsChildrenBlockJoinQuery(parentFilter BitSetProducer, parentQuery, childQuery search.Query, childLimitPerParent int) *ParentsChildrenBlockJoinQuery {
	return NewParentsChildrenBlockJoinQueryWithCombiner(parentFilter, parentQuery, childQuery, childLimitPerParent, func(a, b float32) float32 { return a + b })
}

// NewParentsChildrenBlockJoinQueryWithCombiner creates a ParentsChildrenBlockJoinQuery with a custom score combiner.
func NewParentsChildrenBlockJoinQueryWithCombiner(parentFilter BitSetProducer, parentQuery, childQuery search.Query, childLimitPerParent int, scoreCombiner func(float32, float32) float32) *ParentsChildrenBlockJoinQuery {
	if childLimitPerParent <= 0 {
		panic("childLimitPerParent must be > 0")
	}
	return &ParentsChildrenBlockJoinQuery{
		parentFilter:        parentFilter,
		parentQuery:         parentQuery,
		childQuery:          childQuery,
		childLimitPerParent: childLimitPerParent,
		scoreCombiner:       scoreCombiner,
	}
}

func (q *ParentsChildrenBlockJoinQuery) Visit(visitor search.QueryVisitor) {
	visitor.VisitLeaf(q)
}

func (q *ParentsChildrenBlockJoinQuery) Rewrite(searcher *search.IndexSearcher) (search.Query, error) {
	parentRewrite, err := q.parentQuery.Rewrite(searcher)
	if err != nil {
		return nil, err
	}
	childRewrite, err := q.childQuery.Rewrite(searcher)
	if err != nil {
		return nil, err
	}
	if parentRewrite != q.parentQuery || childRewrite != q.childQuery {
		return NewParentsChildrenBlockJoinQueryWithCombiner(q.parentFilter, parentRewrite, childRewrite, q.childLimitPerParent, q.scoreCombiner), nil
	}
	return q, nil
}

func (q *ParentsChildrenBlockJoinQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	parentWeight, err := q.parentQuery.CreateWeight(searcher, needsScores, boost)
	if err != nil {
		return nil, err
	}
	childWeight, err := q.childQuery.CreateWeight(searcher, needsScores, boost)
	if err != nil {
		return nil, err
	}

	return &parentsChildrenBlockJoinWeight{
		BaseWeight:          search.BaseWeight{Query: q},
		parentFilter:        q.parentFilter,
		parentWeight:        parentWeight,
		childWeight:         childWeight,
		childLimitPerParent: q.childLimitPerParent,
		doScores:            needsScores,
		scoreCombiner:       q.scoreCombiner,
	}, nil
}

type parentsChildrenBlockJoinWeight struct {
	search.BaseWeight
	parentFilter        BitSetProducer
	parentWeight        search.Weight
	childWeight         search.Weight
	childLimitPerParent int
	doScores            bool
	scoreCombiner       func(float32, float32) float32
}

func (w *parentsChildrenBlockJoinWeight) ScorerSupplier(context *index.LeafReaderContext) (search.ScorerSupplier, error) {
	parentBits, err := w.parentFilter.GetBitSet(context)
	if err != nil {
		return nil, err
	}
	if parentBits == nil {
		return nil, nil
	}

	parentScorerSupplier := w.parentWeight.ScorerSupplier(context)
	childScorerSupplier := w.childWeight.ScorerSupplier(context)

	if parentScorerSupplier == nil || childScorerSupplier == nil {
		return nil, nil
	}

	return &parentsChildrenScorerSupplier{
		parentBits:          parentBits,
		parentScorerSupplier: parentScorerSupplier,
		childScorerSupplier:  childScorerSupplier,
		childLimitPerParent: w.childLimitPerParent,
		doScores:            w.doScores,
		scoreCombiner:       w.scoreCombiner,
	}, nil
}

type parentsChildrenScorerSupplier struct {
	parentBits          util.BitSet
	parentScorerSupplier search.ScorerSupplier
	childScorerSupplier  search.ScorerSupplier
	childLimitPerParent int
	doScores            bool
	scoreCombiner       func(float32, float32) float32
}

func (s *parentsChildrenScorerSupplier) Get(leadCost int64) (search.Scorer, error) {
	parentScorer, err := s.parentScorerSupplier.Get(leadCost)
	if err != nil {
		return nil, err
	}
	childScorer, err := s.childScorerSupplier.Get(leadCost)
	if err != nil {
		return nil, err
	}
	return &parentsChildrenBlockJoinScorer{
		parentBits:          s.parentBits,
		parentScorer:        parentScorer,
		childScorer:         childScorer,
		childLimitPerParent: s.childLimitPerParent,
		doScores:            s.doScores,
		scoreCombiner:       s.scoreCombiner,
	}, nil
}

func (s *parentsChildrenScorerSupplier) Cost() int64 {
	pCost := s.parentScorerSupplier.Cost()
	cCost := s.childScorerSupplier.Cost()
	if s.childLimitPerParent == DefaultChildLimitPerParent {
		return cCost
	}
	if pCost*int64(s.childLimitPerParent) < cCost {
		return pCost * int64(s.childLimitPerParent)
	}
	return cCost
}

func (s *parentsChildrenScorerSupplier) SetTopLevelScoringClause() error {
	if err := s.parentScorerSupplier.SetTopLevelScoringClause(); err != nil {
		return err
	}
	return s.childScorerSupplier.SetTopLevelScoringClause()
}

func (s *parentsChildrenScorerSupplier) BulkScorer() (search.BulkScorer, error) {
	return search.NewDefaultBulkScorer(s.Get(0))
}

type parentsChildrenBlockJoinScorer struct {
	parentBits          util.BitSet
	parentScorer        search.Scorer
	childScorer         search.Scorer
	childLimitPerParent int
	doScores            bool
	scoreCombiner       func(float32, float32) float32
	parentScore         float32
	childScore          float32
	parentDoc           int
	childDoc            int
	childDocCount       int
}

func (s *parentsChildrenBlockJoinScorer) DocID() int {
	return s.childDoc
}

func (s *parentsChildrenBlockJoinScorer) Iterator() search.DocIdSetIterator {
	return &parentsChildrenIterator{scorer: s}
}

type parentsChildrenIterator struct {
	scorer *parentsChildrenBlockJoinScorer
}

func (it *parentsChildrenIterator) DocID() int {
	return it.scorer.childDoc
}

func (it *parentsChildrenIterator) NextDoc() (int, error) {
	if it.scorer.childDocCount < it.scorer.childLimitPerParent {
		next, err := it.scorer.childScorer.Iterator().NextDoc()
		if err != nil {
			return 0, err
		}
		it.scorer.childDoc = next
	}

	if it.scorer.childDocCount >= it.scorer.childLimitPerParent || it.scorer.childDoc >= it.scorer.parentDoc {
		it.scorer.childDocCount = 0
		nextParent, err := it.scorer.parentScorer.Iterator().NextDoc()
		if err != nil {
			return 0, err
		}
		it.scorer.parentDoc = nextParent
		if it.scorer.parentDoc == 0 {
			nextParent, err := it.scorer.parentScorer.Iterator().NextDoc()
			if err != nil {
				return 0, err
			}
			it.scorer.parentDoc = nextParent
		}
		if err := it.scorer.validateParentDoc(); err != nil {
			return 0, err
		}
	}

	if err := it.scorer.alignParentAndChildIterator(); err != nil {
		return 0, err
	}

	if it.scorer.childScorer.Iterator().DocID() == search.NO_MORE_DOCS || it.scorer.parentScorer.Iterator().DocID() == search.NO_MORE_DOCS {
		it.scorer.childDoc = search.NO_MORE_DOCS
		it.scorer.parentDoc = search.NO_MORE_DOCS
		return it.scorer.childDoc, nil
	}

	if it.scorer.doScores {
		it.scorer.childScore, _ = it.scorer.childScorer.Score()
		it.scorer.parentScore, _ = it.scorer.parentScorer.Score()
	}

	it.scorer.childDocCount++
	return it.scorer.childDoc, nil
}

func (it *parentsChildrenIterator) Advance(target int) (int, error) {
	if target <= it.scorer.childDoc {
		return it.scorer.childDoc, nil
	}

	next, err := it.scorer.childScorer.Iterator().Advance(target)
	if err != nil {
		return 0, err
	}
	it.scorer.childDoc = next
	if it.scorer.childDocCount >= it.scorer.childLimitPerParent || it.scorer.childDoc >= it.scorer.parentDoc {
		it.scorer.childDocCount = 0
		if it.scorer.childDoc <= it.scorer.parentDoc {
			nextParent, err := it.scorer.parentScorer.Iterator().NextDoc()
			if err != nil {
				return 0, err
			}
			it.scorer.parentDoc = nextParent
		} else {
			nextParent, err := it.scorer.parentScorer.Iterator().Advance(it.scorer.childDoc)
			if err != nil {
				return 0, err
			}
			it.scorer.parentDoc = nextParent
		}
		if err := it.scorer.validateParentDoc(); err != nil {
			return 0, err
		}
		if err := it.scorer.alignParentAndChildIterator(); err != nil {
			return 0, err
		}
	}

	if it.scorer.childScorer.Iterator().DocID() == search.NO_MORE_DOCS || it.scorer.parentScorer.Iterator().DocID() == search.NO_MORE_DOCS {
		it.scorer.childDoc = search.NO_MORE_DOCS
		it.scorer.parentDoc = search.NO_MORE_DOCS
		return it.scorer.childDoc, nil
	}

	if it.scorer.doScores {
		it.scorer.childScore, _ = it.scorer.childScorer.Score()
		it.scorer.parentScore, _ = it.scorer.parentScorer.Score()
	}

	it.scorer.childDocCount++
	return it.scorer.childDoc, nil
}

func (it *parentsChildrenIterator) Cost() int64 {
	return it.scorer.childScorer.Cost()
}

func (s *parentsChildrenBlockJoinScorer) validateParentDoc() error {
	if s.parentDoc != search.NO_MORE_DOCS && !s.parentBits.Get(s.parentDoc) {
		return fmt.Errorf("Parent query must not match any docs besides parent filter. docID=%d", s.parentDoc)
	}
	return nil
}

func (s *parentsChildrenBlockJoinScorer) alignParentAndChildIterator() error {
	for {
		if s.childScorer.Iterator().DocID() == search.NO_MORE_DOCS || s.parentScorer.Iterator().DocID() == search.NO_MORE_DOCS {
			return nil
		}
		firstChild := s.parentBits.PrevSetBit(s.parentDoc-1) + 1
		if s.childDoc >= firstChild && s.childDoc < s.parentDoc {
			break
		} else if s.childDoc < firstChild {
			next, err := s.childScorer.Iterator().Advance(firstChild)
			if err != nil {
				return err
			}
			s.childDoc = next
		} else {
			if s.childDoc == s.parentDoc {
				nextParent, err := s.parentScorer.Iterator().NextDoc()
				if err != nil {
					return err
				}
				s.parentDoc = nextParent
			} else {
				nextParent, err := s.parentScorer.Iterator().Advance(s.childDoc)
				if err != nil {
					return err
				}
				s.parentDoc = nextParent
			}
			if err := s.validateParentDoc(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *parentsChildrenBlockJoinScorer) Score() (float32, error) {
	if s.doScores {
		return s.scoreCombiner(s.parentScore, s.childScore), nil
	}
	return 1.0, nil
}

func (s *parentsChildrenBlockJoinScorer) GetMaxScore(upTo int) (float32, error) {
	return 1e38, nil
}

func (s *parentsChildrenBlockJoinScorer) SetMinCompetitiveScore(minScore float32) error {
	if err := s.parentScorer.SetMinCompetitiveScore(minScore); err != nil {
		return err
	}
	return s.childScorer.SetMinCompetitiveScore(minScore)
}

func (s *parentsChildrenBlockJoinScorer) TwoPhaseIterator() search.TwoPhaseIterator {
	return nil
}
