package index

import (
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// ByteVectorValues provides access to per-document floating point vector values indexed as byte vectors.
type ByteVectorValues interface {
	KnnVectorValues

	// VectorValue returns the vector value for the given vector ordinal.
	VectorValue(ord int) ([]byte, error)

	// Copy creates a new copy of this ByteVectorValues.
	CopyByteVectorValues() (ByteVectorValues, error)

	// Scorer returns a VectorScorer for the given query vector.
	Scorer(query []byte) (util.VectorScorer, error)
}

// Rescorer rescores using the given query vector and the current ByteVectorValues.
func Rescorer(bvv ByteVectorValues, target []byte) (util.VectorScorer, error) {
	return bvv.Scorer(target)
}

// CheckField checks the Vector Encoding of a field.
func CheckField(leafReader interface{ GetFieldInfos() interface{ FieldInfo(string) interface{ HasVectorValues() bool; GetVectorEncoding() VectorEncoding } } }, field string) error {
	fi := leafReader.GetFieldInfos().FieldInfo(field)
	if fi != nil && fi.HasVectorValues() && fi.GetVectorEncoding() != Byte {
		return fmt.Errorf("unexpected vector encoding (%v) for field %s (expected=Byte)", fi.GetVectorEncoding(), field)
	}
	return nil
}
