package index

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// FloatVectorValues provides access to per-document floating point vector
// values. It is the Go port of org.apache.lucene.index.FloatVectorValues;
// the declaration lives in spi (see [KnnVectorValues]).
type FloatVectorValues = spi.FloatVectorValues

// CheckFloatVectorField checks the Vector Encoding of a field.
// This is the Go port of FloatVectorValues.checkField.
func CheckFloatVectorField(in LeafReader, field string) error {
	fi := in.GetFieldInfos().FieldInfoByName(field)
	if fi != nil && fi.VectorDimension() != 0 && fi.VectorEncoding() != VectorEncodingFloat32 {
		return fmt.Errorf("unexpected vector encoding (%s) for field %s (expected=%s)",
			fi.VectorEncoding(), field, VectorEncodingFloat32)
	}
	return nil
}

// FromFloats creates a FloatVectorValues from a list of float arrays.
// This is the Go port of FloatVectorValues.fromFloats.
func FromFloats(vectors [][]float32, dim int) FloatVectorValues {
	return &floatVectorValuesFromFloats{
		vectors: vectors,
		dim:     dim,
	}
}

// floatVectorValuesFromFloats is the anonymous FloatVectorValues returned by
// FloatVectorValues.fromFloats.
type floatVectorValuesFromFloats struct {
	vectors [][]float32
	dim     int
}

func (f *floatVectorValuesFromFloats) Dimension() int {
	return f.dim
}

func (f *floatVectorValuesFromFloats) Size() int {
	return len(f.vectors)
}

// OrdToDoc carries the KnnVectorValues.ordToDoc default, which returns ord.
func (f *floatVectorValuesFromFloats) OrdToDoc(ord int) int {
	return ord
}

// Prefetch carries the KnnVectorValues.prefetch default, which does nothing.
func (f *floatVectorValuesFromFloats) Prefetch(ordsToPrefetch []int, numOrds int) error {
	return nil
}

func (f *floatVectorValuesFromFloats) Copy() (KnnVectorValues, error) {
	return f, nil
}

// GetEncoding carries the FloatVectorValues.getEncoding override.
func (f *floatVectorValuesFromFloats) GetEncoding() VectorEncoding {
	return VectorEncodingFloat32
}

// GetVectorByteLength carries the KnnVectorValues.getVectorByteLength default.
func (f *floatVectorValuesFromFloats) GetVectorByteLength() int {
	return f.Dimension() * VectorEncodingByteSize(f.GetEncoding())
}

// GetAcceptOrds carries the KnnVectorValues.getAcceptOrds default.
func (f *floatVectorValuesFromFloats) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return spi.DefaultGetAcceptOrds(f, acceptDocs)
}

func (f *floatVectorValuesFromFloats) Iterator() DocIndexIterator {
	return spi.CreateDenseIterator(f)
}

func (f *floatVectorValuesFromFloats) VectorValue(ord int) ([]float32, error) {
	if ord < 0 || ord >= len(f.vectors) {
		return nil, errors.New("index out of bounds")
	}
	return f.vectors[ord], nil
}

func (f *floatVectorValuesFromFloats) CopyFloatVectorValues() (FloatVectorValues, error) {
	return f, nil
}

// Scorer carries the FloatVectorValues.scorer default, which throws
// UnsupportedOperationException.
func (f *floatVectorValuesFromFloats) Scorer(target []float32) (util.VectorScorer, error) {
	return nil, errors.New("not implemented")
}

// Rescorer carries the FloatVectorValues.rescorer default, which returns
// scorer(target).
func (f *floatVectorValuesFromFloats) Rescorer(target []float32) (util.VectorScorer, error) {
	return f.Scorer(target)
}
