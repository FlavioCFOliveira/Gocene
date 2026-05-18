package matchhighlight

// MatchRegionRetriever resolves the per-field highlight regions for a single
// document. Mirrors
// org.apache.lucene.search.matchhighlight.MatchRegionRetriever.
type MatchRegionRetriever struct {
	supplier OffsetsRetrievalStrategySupplier
	iter     func(field string) MatchIterator
}

// NewMatchRegionRetriever builds the retriever.
func NewMatchRegionRetriever(supplier OffsetsRetrievalStrategySupplier, iter func(field string) MatchIterator) *MatchRegionRetriever {
	return &MatchRegionRetriever{supplier: supplier, iter: iter}
}

// Retrieve returns the (field → regions) map for a single document.
func (r *MatchRegionRetriever) Retrieve(fields []string) (map[string][]OffsetRange, error) {
	out := make(map[string][]OffsetRange, len(fields))
	for _, f := range fields {
		strategy := r.supplier(f)
		if strategy == nil {
			continue
		}
		regions, err := strategy.Retrieve(r.iter(f))
		if err != nil {
			return nil, err
		}
		out[f] = regions
	}
	return out, nil
}
