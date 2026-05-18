package facetset

// IntFacetSet packs N int32 values into the FacetSet wire format (4 bytes
// per dimension, big-endian). Mirrors
// org.apache.lucene.facet.facetset.IntFacetSet.
type IntFacetSet struct {
	values []int32
}

// NewIntFacetSet builds an IntFacetSet from the supplied values.
func NewIntFacetSet(values ...int32) *IntFacetSet {
	out := make([]int32, len(values))
	copy(out, values)
	return &IntFacetSet{values: out}
}

// Dims returns the dimensionality.
func (s *IntFacetSet) Dims() int { return len(s.values) }

// SizeInBytes returns the wire size (4 bytes per dimension).
func (s *IntFacetSet) SizeInBytes() int { return 4 * len(s.values) }

// PackValues writes the big-endian bytes into dest.
func (s *IntFacetSet) PackValues(dest []byte) int {
	for i, v := range s.values {
		PutInt32BE(dest, i*4, v)
	}
	return s.SizeInBytes()
}

// GetComparableValues widens each int32 to int64 with sign preservation.
func (s *IntFacetSet) GetComparableValues() []int64 {
	out := make([]int64, len(s.values))
	for i, v := range s.values {
		out[i] = int64(v)
	}
	return out
}

// Values returns a copy of the underlying int32 slice.
func (s *IntFacetSet) Values() []int32 {
	out := make([]int32, len(s.values))
	copy(out, s.values)
	return out
}

var _ FacetSet = (*IntFacetSet)(nil)
