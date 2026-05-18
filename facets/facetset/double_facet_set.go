package facetset

import "math"

// DoubleFacetSet packs N float64 values using the Lucene NumericUtils
// sortable-long encoding (8 bytes BE per dim). Mirrors
// org.apache.lucene.facet.facetset.DoubleFacetSet.
type DoubleFacetSet struct {
	values []float64
}

// NewDoubleFacetSet builds a DoubleFacetSet from the supplied values.
func NewDoubleFacetSet(values ...float64) *DoubleFacetSet {
	out := make([]float64, len(values))
	copy(out, values)
	return &DoubleFacetSet{values: out}
}

// Dims returns the dimensionality.
func (s *DoubleFacetSet) Dims() int { return len(s.values) }

// SizeInBytes returns 8 * dims.
func (s *DoubleFacetSet) SizeInBytes() int { return 8 * len(s.values) }

// PackValues writes each value as sortable big-endian int64.
func (s *DoubleFacetSet) PackValues(dest []byte) int {
	for i, v := range s.values {
		PutInt64BE(dest, i*8, doubleToSortableLong(v))
	}
	return s.SizeInBytes()
}

// GetComparableValues returns the sortable int64 encoding per dim.
func (s *DoubleFacetSet) GetComparableValues() []int64 {
	out := make([]int64, len(s.values))
	for i, v := range s.values {
		out[i] = doubleToSortableLong(v)
	}
	return out
}

// Values returns a copy of the float64 slice.
func (s *DoubleFacetSet) Values() []float64 {
	out := make([]float64, len(s.values))
	copy(out, s.values)
	return out
}

// doubleToSortableLong mirrors Lucene NumericUtils.sortableDoubleBits.
func doubleToSortableLong(f float64) int64 {
	bits := math.Float64bits(f)
	if bits>>63 != 0 {
		bits ^= 0x7fffffffffffffff
	}
	return int64(bits)
}

var _ FacetSet = (*DoubleFacetSet)(nil)
