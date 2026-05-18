package facetset

// DimRange describes an inclusive interval [Min, Max] over a single
// comparable int64 dimension. Mirrors
// org.apache.lucene.facet.facetset.DimRange.
type DimRange struct {
	Min int64
	Max int64
}

// NewDimRange constructs an inclusive range. The constructor swaps min/max
// if they are inverted so callers never have to worry about ordering.
func NewDimRange(min, max int64) DimRange {
	if min > max {
		min, max = max, min
	}
	return DimRange{Min: min, Max: max}
}

// Contains reports whether v lies within the range, inclusive on both ends.
func (r DimRange) Contains(v int64) bool {
	return v >= r.Min && v <= r.Max
}
