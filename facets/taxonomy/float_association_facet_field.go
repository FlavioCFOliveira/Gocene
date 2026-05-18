package taxonomy

import (
	"encoding/binary"
	"math"
)

// FloatAssociationFacetField is an AssociationFacetField whose payload is a
// single float32 encoded as big-endian IEEE 754 bits. Mirrors
// org.apache.lucene.facet.taxonomy.FloatAssociationFacetField.
type FloatAssociationFacetField struct {
	*AssociationFacetField
	Value float32
}

// NewFloatAssociationFacetField builds a FloatAssociationFacetField.
func NewFloatAssociationFacetField(value float32, dim string, path ...string) *FloatAssociationFacetField {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, math.Float32bits(value))
	return &FloatAssociationFacetField{
		AssociationFacetField: NewAssociationFacetField(dim, path, payload),
		Value:                 value,
	}
}

// FloatAssociationFromBytes decodes the 4-byte payload back to float32.
func FloatAssociationFromBytes(b []byte) float32 {
	if len(b) < 4 {
		return 0
	}
	return math.Float32frombits(binary.BigEndian.Uint32(b[:4]))
}
