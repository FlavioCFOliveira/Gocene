package facetset

import (
	"encoding/binary"
	"math"
)

// FacetSetDecoder reconstructs typed values from the big-endian wire form
// emitted by FacetSet.PackValues. Mirrors
// org.apache.lucene.facet.facetset.FacetSetDecoder.
type FacetSetDecoder func(src []byte, off int, dims int, out []int64) int

// IntDecoder decodes N int32 values stored as 4-byte big-endian.
func IntDecoder(src []byte, off, dims int, out []int64) int {
	for i := 0; i < dims; i++ {
		out[i] = int64(int32(binary.BigEndian.Uint32(src[off+i*4 : off+i*4+4])))
	}
	return dims * 4
}

// LongDecoder decodes N int64 values stored as 8-byte big-endian.
func LongDecoder(src []byte, off, dims int, out []int64) int {
	for i := 0; i < dims; i++ {
		out[i] = int64(binary.BigEndian.Uint64(src[off+i*8 : off+i*8+8]))
	}
	return dims * 8
}

// FloatDecoder decodes N float32 values stored with the sortable-int encoding
// and widens them to int64.
func FloatDecoder(src []byte, off, dims int, out []int64) int {
	for i := 0; i < dims; i++ {
		out[i] = int64(int32(binary.BigEndian.Uint32(src[off+i*4 : off+i*4+4])))
	}
	return dims * 4
}

// DoubleDecoder decodes N float64 values stored with the sortable-long
// encoding.
func DoubleDecoder(src []byte, off, dims int, out []int64) int {
	for i := 0; i < dims; i++ {
		out[i] = int64(binary.BigEndian.Uint64(src[off+i*8 : off+i*8+8]))
	}
	return dims * 8
}

// SortableIntToFloat recovers a float32 from its sortable-int encoding. The
// encoding is its own inverse: positive values pass through, negative values
// have their lower 31 bits flipped.
func SortableIntToFloat(v int32) float32 {
	bits := uint32(v)
	if bits>>31 != 0 {
		bits ^= 0x7fffffff
	}
	return math.Float32frombits(bits)
}

// SortableLongToDouble recovers a float64 from its sortable-long encoding.
func SortableLongToDouble(v int64) float64 {
	bits := uint64(v)
	if bits>>63 != 0 {
		bits ^= 0x7fffffffffffffff
	}
	return math.Float64frombits(bits)
}
