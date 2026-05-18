package taxonomy

// OrdinalMappingLeafReader is the helper used during taxonomy merges to
// remap ordinals from a source segment to the destination segment. The Java
// port wraps a LeafReader; the Go port focuses on the ordinal mapping
// primitive since the reader is implemented elsewhere. Mirrors
// org.apache.lucene.facet.taxonomy.OrdinalMappingLeafReader.
type OrdinalMappingLeafReader struct {
	ordMap []int
}

// NewOrdinalMappingLeafReader builds a remapper backed by a precomputed
// src->dst ordinal map.
func NewOrdinalMappingLeafReader(ordMap []int) *OrdinalMappingLeafReader {
	clone := make([]int, len(ordMap))
	copy(clone, ordMap)
	return &OrdinalMappingLeafReader{ordMap: clone}
}

// MapOrdinal translates srcOrd into the destination ordinal or -1 when the
// ordinal is unknown to this reader.
func (r *OrdinalMappingLeafReader) MapOrdinal(srcOrd int) int {
	if srcOrd < 0 || srcOrd >= len(r.ordMap) {
		return -1
	}
	return r.ordMap[srcOrd]
}

// MapOrdinals translates each source ordinal in-place using the same mapping.
func (r *OrdinalMappingLeafReader) MapOrdinals(srcOrds []int) {
	for i, s := range srcOrds {
		srcOrds[i] = r.MapOrdinal(s)
	}
}
