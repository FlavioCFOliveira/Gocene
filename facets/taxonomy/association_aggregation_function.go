package taxonomy

// AssociationAggregationFunction tells TaxonomyFacetAssociations how to roll
// many per-document values into a single per-ordinal aggregate. Mirrors
// the abstract org.apache.lucene.facet.taxonomy.AssociationAggregationFunction.
type AssociationAggregationFunction interface {
	// Name returns the function's human-readable identifier.
	Name() string

	// Aggregate combines existing and incoming float32 values.
	Aggregate(existing, incoming float32) float32

	// AggregateInt combines existing and incoming int32 values.
	AggregateInt(existing, incoming int32) int32
}

// SumAggregation is the addition-based aggregator used for sums.
type SumAggregation struct{}

// Name returns "SUM".
func (SumAggregation) Name() string { return "SUM" }

// Aggregate returns existing + incoming.
func (SumAggregation) Aggregate(existing, incoming float32) float32 { return existing + incoming }

// AggregateInt returns existing + incoming.
func (SumAggregation) AggregateInt(existing, incoming int32) int32 { return existing + incoming }

// MaxAggregation keeps the larger of the two values.
type MaxAggregation struct{}

// Name returns "MAX".
func (MaxAggregation) Name() string { return "MAX" }

// Aggregate returns the larger value.
func (MaxAggregation) Aggregate(existing, incoming float32) float32 {
	if incoming > existing {
		return incoming
	}
	return existing
}

// AggregateInt returns the larger value.
func (MaxAggregation) AggregateInt(existing, incoming int32) int32 {
	if incoming > existing {
		return incoming
	}
	return existing
}

var (
	// SUM exposes a shared SumAggregation instance.
	SUM AssociationAggregationFunction = SumAggregation{}
	// MAX exposes a shared MaxAggregation instance.
	MAX AssociationAggregationFunction = MaxAggregation{}
)
