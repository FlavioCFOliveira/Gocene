package facetset

import "encoding/binary"

// FacetSetsField is the BinaryDocValues payload that captures one or more
// FacetSets per document. The Java port writes a single binary value whose
// layout is: number-of-sets (vint), dims (vint), then each FacetSet packed
// back-to-back via PackValues. Mirrors
// org.apache.lucene.facet.facetset.FacetSetsField.
type FacetSetsField struct {
	Name string
	Sets []FacetSet
}

// NewFacetSetsField builds the field with the supplied name and FacetSets.
// The constructor enforces that every FacetSet shares the same Dims.
func NewFacetSetsField(name string, sets ...FacetSet) *FacetSetsField {
	if len(sets) == 0 {
		panic("FacetSetsField requires at least one FacetSet")
	}
	dims := sets[0].Dims()
	for _, s := range sets[1:] {
		if s.Dims() != dims {
			panic("FacetSetsField: all sets must share the same Dims")
		}
	}
	return &FacetSetsField{Name: name, Sets: append([]FacetSet(nil), sets...)}
}

// BinaryValue returns the serialized BinaryDocValues payload.
func (f *FacetSetsField) BinaryValue() []byte {
	if len(f.Sets) == 0 {
		return nil
	}
	dims := f.Sets[0].Dims()
	totalBytes := 0
	for _, s := range f.Sets {
		totalBytes += s.SizeInBytes()
	}
	out := make([]byte, 0, 8+totalBytes)
	out = appendVInt(out, uint32(len(f.Sets)))
	out = appendVInt(out, uint32(dims))
	tmp := make([]byte, 0, 64)
	for _, s := range f.Sets {
		if cap(tmp) < s.SizeInBytes() {
			tmp = make([]byte, s.SizeInBytes())
		}
		tmp = tmp[:s.SizeInBytes()]
		s.PackValues(tmp)
		out = append(out, tmp...)
	}
	return out
}

// appendVInt writes a Lucene-style variable-length unsigned int.
func appendVInt(buf []byte, v uint32) []byte {
	var tmp [binary.MaxVarintLen32]byte
	n := binary.PutUvarint(tmp[:], uint64(v))
	return append(buf, tmp[:n]...)
}
