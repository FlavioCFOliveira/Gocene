package taxonomy

import "encoding/binary"

// IntAssociationFacetField is an AssociationFacetField whose payload is a
// single int32 encoded as 4 big-endian bytes. Mirrors
// org.apache.lucene.facet.taxonomy.IntAssociationFacetField.
type IntAssociationFacetField struct {
	*AssociationFacetField
	Value int32
}

// NewIntAssociationFacetField builds an IntAssociationFacetField.
func NewIntAssociationFacetField(value int32, dim string, path ...string) *IntAssociationFacetField {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, uint32(value))
	return &IntAssociationFacetField{
		AssociationFacetField: NewAssociationFacetField(dim, path, payload),
		Value:                 value,
	}
}

// IntAssociationFromBytes decodes the 4-byte payload back to int32.
func IntAssociationFromBytes(b []byte) int32 {
	if len(b) < 4 {
		return 0
	}
	return int32(binary.BigEndian.Uint32(b[:4]))
}
