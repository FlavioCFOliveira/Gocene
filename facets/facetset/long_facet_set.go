package facetset

// LongFacetSet packs N int64 values as 8-byte big-endian bytes. Mirrors
// org.apache.lucene.facet.facetset.LongFacetSet.
type LongFacetSet struct {
	values []int64
}

// NewLongFacetSet builds a LongFacetSet from the supplied values.
func NewLongFacetSet(values ...int64) *LongFacetSet {
	out := make([]int64, len(values))
	copy(out, values)
	return &LongFacetSet{values: out}
}

// Dims returns the dimensionality.
func (s *LongFacetSet) Dims() int { return len(s.values) }

// SizeInBytes returns 8 * dims.
func (s *LongFacetSet) SizeInBytes() int { return 8 * len(s.values) }

// PackValues writes each dimension as big-endian int64 into dest.
func (s *LongFacetSet) PackValues(dest []byte) int {
	for i, v := range s.values {
		PutInt64BE(dest, i*8, v)
	}
	return s.SizeInBytes()
}

// GetComparableValues returns the values verbatim.
func (s *LongFacetSet) GetComparableValues() []int64 {
	out := make([]int64, len(s.values))
	copy(out, s.values)
	return out
}

// Values returns a copy of the int64 slice.
func (s *LongFacetSet) Values() []int64 {
	out := make([]int64, len(s.values))
	copy(out, s.values)
	return out
}

var _ FacetSet = (*LongFacetSet)(nil)
