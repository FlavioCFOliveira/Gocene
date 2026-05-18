package facetset

import "math"

// FloatFacetSet packs N float32 values using the Lucene-sortable big-endian
// encoding (NumericUtils.floatToSortableInt). Mirrors
// org.apache.lucene.facet.facetset.FloatFacetSet.
type FloatFacetSet struct {
	values []float32
}

// NewFloatFacetSet builds a FloatFacetSet from the supplied values.
func NewFloatFacetSet(values ...float32) *FloatFacetSet {
	out := make([]float32, len(values))
	copy(out, values)
	return &FloatFacetSet{values: out}
}

// Dims returns the dimensionality.
func (s *FloatFacetSet) Dims() int { return len(s.values) }

// SizeInBytes returns 4 * dims.
func (s *FloatFacetSet) SizeInBytes() int { return 4 * len(s.values) }

// PackValues writes each value as sortable big-endian int32.
func (s *FloatFacetSet) PackValues(dest []byte) int {
	for i, v := range s.values {
		PutInt32BE(dest, i*4, floatToSortableInt(v))
	}
	return s.SizeInBytes()
}

// GetComparableValues widens each sortable int32 to int64.
func (s *FloatFacetSet) GetComparableValues() []int64 {
	out := make([]int64, len(s.values))
	for i, v := range s.values {
		out[i] = int64(floatToSortableInt(v))
	}
	return out
}

// Values returns a copy of the underlying float32 slice.
func (s *FloatFacetSet) Values() []float32 {
	out := make([]float32, len(s.values))
	copy(out, s.values)
	return out
}

// floatToSortableInt is the Lucene NumericUtils.sortableFloatBits encoding:
// positive bit patterns survive unchanged (sign bit = 0 → positive signed int),
// while negative bit patterns get their lower 31 bits flipped, leaving the
// sign bit set so they remain negative — and ordered correctly — under signed
// comparison.
func floatToSortableInt(f float32) int32 {
	bits := math.Float32bits(f)
	if bits>>31 != 0 {
		bits ^= 0x7fffffff
	}
	return int32(bits)
}

var _ FacetSet = (*FloatFacetSet)(nil)
