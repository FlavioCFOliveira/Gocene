package join

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ParentChildrenBlockJoinQuery returns all the matching child documents for a specific parent document
// indexed together in the same block.
type ParentChildrenBlockJoinQuery struct {
	search.BaseQuery
	parentFilter BitSetProducer
	childQuery   search.Query
	parentDocId  int
}

// NewParentChildrenBlockJoinQuery creates a ParentChildrenBlockJoinQuery.
func NewParentChildrenBlockJoinQuery(parentFilter BitSetProducer, childQuery search.Query, parentDocId int) *ParentChildrenBlockJoinQuery {
	return &ParentChildrenBlockJoinQuery{
		parentFilter: parentFilter,
		childQuery:   childQuery,
		parentDocId:  parentDocId,
	}
}

func (q *ParentChildrenBlockJoinQuery) Visit(visitor search.QueryVisitor) {
	visitor.VisitLeaf(q)
}

func (q *ParentChildrenBlockJoinQuery) Rewrite(searcher *search.IndexSearcher) (search.Query, error) {
	childRewrite, err := q.childQuery.Rewrite(searcher)
	if err != nil {
		return nil, err
	}
	if childRewrite != q.childQuery {
		return NewParentChildrenBlockJoinQuery(q.parentFilter, childRewrite, q.parentDocId), nil
	}
	return q, nil
}

func (q *ParentChildrenBlockJoinQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	childWeight, err := q.childQuery.CreateWeight(searcher, needsScores, boost)
	if err != nil {
		return nil, err
	}

	leaves := searcher.IndexReader.Leaves()
	leafIndex := -1
	for i, leaf := range leaves {
		if q.parentDocId >= leaf.DocBase && q.parentDocId < leaf.DocBase+leaf.NumDocs {
			leafIndex = i
			break
		}
	}

	return &parentChildrenBlockJoinWeight{
		BaseWeight:   search.BaseWeight{Query: q},
		childWeight:  childWeight,
		leafIndex:    leafIndex,
		parentDocId:  q.parentDocId,
		parentFilter: q.parentFilter,
	}, nil
}

type parentChildrenBlockJoinWeight struct {
	search.BaseWeight
	childWeight  search.Weight
	leafIndex    int
	parentDocId  int
	parentFilter BitSetProducer
}

func (w *parentChildrenBlockJoinWeight) ScorerSupplier(context *index.LeafReaderContext) (search.ScorerSupplier, error) {
	if context.Ord != w.leafIndex {
		return nil, nil
	}

	localParentDocId := w.parentDocId - context.DocBase
	if localParentDocId == 0 {
		return nil, nil
	}

	parents, err := w.parentFilter.GetBitSet(context)
	if err != nil {
		return nil, err
	}
	firstChildDocId := parents.PrevSetBit(localParentDocId-1) + 1
	if firstChildDocId == localParentDocId {
		return nil, nil
	}

	childrenScorer, err := w.childWeight.Scorer(context)
	if err != nil {
		return nil, err
	}
	if childrenScorer == nil {
		return nil, nil
	}

	return &parentChildrenScorerSupplier{
		childrenScorer:   childrenScorer,
		firstChildDocId: firstChildDocId,
		localParentDocId: localParentDocId,
	}, nil
}

type parentChildrenScorerSupplier struct {
	childrenScorer   search.Scorer
	firstChildDocId int
	localParentDocId int
}

func (s *parentChildrenScorerSupplier) Get(leadCost int64) (search.Scorer, error) {
	return &parentChildrenScorer{
		childrenScorer:   s.childrenScorer,
		firstChildDocId: s.firstChildDocId,
		localParentDocId: s.localParentDocId,
	}, nil
}

func (s *parentChildrenScorerSupplier) Cost() int64 {
	return s.childrenScorer.Cost()
}

func (s *parentChildrenScorerSupplier) SetTopLevelScoringClause() error {
	return s.childrenScorer.SetMinCompetitiveScore(0)
}

func (s *parentChildrenScorerSupplier) BulkScorer() (search.BulkScorer, error) {
	return search.NewDefaultBulkScorer(s.Get(0))
}

type parentChildrenScorer struct {
	childrenScorer   search.Scorer
	firstChildDocId  int
	localParentDocId int
}

func (s *parentChildrenScorer) DocID() int {
	return s.childrenScorer.DocID()
}

func (s *parentChildrenScorer) Iterator() util.DocIdSetIterator {
	return &parentChildrenIterator{scorer: s}
}

type parentChildrenIterator struct {
	scorer *parentChildrenScorer
}

func (it *parentChildrenIterator) DocID() int {
	return it.scorer.childrenScorer.DocID()
}

func (it *parentChildrenIterator) NextDoc() (int, error) {
	return it.Advance(it.DocID() + 1)
}

func (it *parentChildrenIterator) Advance(target int) (int, error) {
	t := target
	if t < it.scorer.firstChildDocId {
		t = it.scorer.firstChildDocId
	}
	if t >= it.scorer.localParentDocId {
		return search.NO_MORE_DOCS, nil
	}
	advanced, err := it.scorer.childrenScorer.Iterator().Advance(t)
	if err != nil {
		return 0, err
	}
	if advanced >= it.scorer.localParentDocId {
		return search.NO_MORE_DOCS, nil
	}
	return advanced, nil
}

func (it *parentChildrenIterator) Cost() int64 {
	return it.scorer.childrenScorer.Cost()
}

func (s *parentChildrenScorer) Score() (float32, error) {
	return s.childrenScorer.Score()
}

func (s *parentChildrenScorer) GetMaxScore(upTo int) (float32, error) {
	return 1e38, nil
}

func (s *parentChildrenScorer) SetMinCompetitiveScore(minScore float32) error {
	return s.childrenScorer.SetMinCompetitiveScore(minScore)
}

func (s *parentChildrenScorer) TwoPhaseIterator() search.TwoPhaseIterator {
	return nil
}
