package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// FilterWeight contains another Weight and implements all methods by
// calling the contained weight's method.
type FilterWeight struct {
	BaseWeight
	in Weight
}

func NewFilterWeight(weight Weight) *FilterWeight {
	return &FilterWeight{
		BaseWeight: BaseWeight{query: weight.Query()},
		in:         weight,
	}
}

func NewFilterWeightWithQuery(query Query, weight Weight) *FilterWeight {
	return &FilterWeight{
		BaseWeight: BaseWeight{query: query},
		in:         weight,
	}
}

func (w *FilterWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return w.in.IsCacheable(ctx)
}

func (w *FilterWeight) Explain(ctx *index.LeafReaderContext, doc int) Explanation {
	return w.in.Explain(ctx, doc)
}

func (w *FilterWeight) Matches(ctx *index.LeafReaderContext, doc int) Matches {
	return w.in.Matches(ctx, doc)
}

func (w *FilterWeight) ScorerSupplier(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
	return w.in.ScorerSupplier(ctx)
}

func (w *FilterWeight) Count(ctx *index.LeafReaderContext) (int, error) {
	return w.in.Count(ctx)
}

var _ Weight = (*FilterWeight)(nil)
