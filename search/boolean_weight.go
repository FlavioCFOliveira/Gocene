package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// BooleanWeight is the weight for a BooleanQuery.
type BooleanWeight struct {
	BaseWeight
	query      *BooleanQuery
	searcher   *IndexSearcher
	scoreMode  ScoreMode
	boost      float32
	childWeights []Weight
}

func NewBooleanWeight(query *BooleanQuery, searcher *IndexSearcher, scoreMode ScoreMode, boost float32) *BooleanWeight {
	bw := &BooleanWeight{
		BaseWeight: BaseWeight{query: query},
		query:      query,
		searcher:   searcher,
		scoreMode:   scoreMode,
		boost:      boost,
	}

	// Build child weights
	for _, clause := range query.Clauses() {
		w, _ := clause.Query().CreateWeight(searcher, scoreMode, 1.0)
		bw.childWeights = append(bw.childWeights, w)
	}

	return bw
}

func (bw *BooleanWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	subs := make(map[Occur][]ScorerSupplier)
	subs[MUST] = []ScorerSupplier{}
	subs[SHOULD] = []ScorerSupplier{}
	subs[MUST_NOT] = []ScorerSupplier{}
	subs[FILTER] = []ScorerSupplier{}

	for _, clause := range bw.query.Clauses() {
		w, err := clause.Query().CreateWeight(bw.searcher, bw.scoreMode, 1.0)
		if err != nil {
			return nil, err
		}
		supplier, err := w.ScorerSupplier(context)
		if err != nil {
			return nil, err
		}
		if supplier != nil {
			subs[clause.Occur()] = append(subs[clause.Occur()], supplier)
		}
	}

	return NewBooleanScorerSupplier(bw, subs, bw.scoreMode, bw.query.GetMinimumNumberShouldMatch(), context.Reader().MaxDoc()), nil
}

var _ Weight = (*BooleanWeight)(nil)
