package matchhighlight

// OffsetsRetrievalStrategySupplier resolves the OffsetsRetrievalStrategy to
// use for a given field. Mirrors
// org.apache.lucene.search.matchhighlight.OffsetsRetrievalStrategySupplier.
type OffsetsRetrievalStrategySupplier func(field string) OffsetsRetrievalStrategy
