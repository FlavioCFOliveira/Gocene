// Package facetset implements multi-dimensional facet matching (the
// org.apache.lucene.facet.facetset package): a FacetSet is a tuple of numeric
// dimensions assigned to a single document and a FacetSetMatcher inspects
// those tuples to decide which document matches which facet cell.
package facetset

import "encoding/binary"

// FacetSet is the abstract per-document tuple of N numeric dimensions.
// Implementations (DoubleFacetSet, FloatFacetSet, IntFacetSet, LongFacetSet)
// fix the wire representation. Mirrors org.apache.lucene.facet.facetset.FacetSet.
type FacetSet interface {
	// Dims returns the number of dimensions in the tuple.
	Dims() int

	// PackValues writes the wire representation of this set into dest and
	// returns the number of bytes written.
	PackValues(dest []byte) int

	// SizeInBytes returns the size of the wire representation in bytes.
	SizeInBytes() int

	// GetComparableValues returns the dimensions as int64 keys suitable for
	// range matchers. Implementations use Lucene's sortable encodings for
	// float/double values.
	GetComparableValues() []int64
}

// PutInt32BE writes an int32 in big-endian order at the given offset.
func PutInt32BE(b []byte, off int, v int32) {
	binary.BigEndian.PutUint32(b[off:off+4], uint32(v))
}

// PutInt64BE writes an int64 in big-endian order at the given offset.
func PutInt64BE(b []byte, off int, v int64) {
	binary.BigEndian.PutUint64(b[off:off+8], uint64(v))
}
