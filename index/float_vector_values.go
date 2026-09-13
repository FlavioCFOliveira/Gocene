package index

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// FloatVectorValues provides access to per-document floating point vector values.
// This is the Go port of Lucene's org.apache.lucene.index.FloatVectorValues.
type FloatVectorValues interface {
	KnnVectorValues
	// VectorValue returns the vector value for the given vector ordinal.
	// It is illegal to call this method if ord is not in [0, Size() - 1].
	VectorValue(ord int) ([]float32, error)

	// Copy creates a new copy of this FloatVectorValues.
	CopyFloatVectorValues() (FloatVectorValues, error)

	// Scorer returns a VectorScorer for the given query vector and the current FloatVectorValues.
	Scorer(target []float32) (interface{}, error)

	// Rescorer rescores using the given query vector and the current FloatVectorValues.
	Rescorer(target []float32) (interface{}, error)
}

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

func (f *floatVectorValuesFromFloats) OrdToDoc(ord int) int {
	return ord
}

func (f *floatVectorValuesFromFloats) Prefetch(ordsToPrefetch []int, numOrds int) error {
	return nil
}

func (f *floatVectorValuesFromFloats) Copy() (KnnVectorValues, error) {
	return f, nil
}

func (f *floatVectorValuesFromFloats) GetEncoding() VectorEncoding {
	return VectorEncodingFloat32
}

func (f *floatVectorValuesFromFloats) GetVectorByteLength() int {
	return f.Dimension() * VectorEncodingByteSize(f.GetEncoding())
}

func (f *floatVectorValuesFromFloats) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	// Default implementation from KnnVectorValues
	if acceptDocs == nil {
		return nil
	}
	return &acceptOrdsBitSet{
		acceptDocs: acceptDocs,
		size:       f.Size(),
	}
}

func (f *floatVectorValuesFromFloats) Iterator() util.DocIndexIterator {
	return newDenseDocIndexIterator(f.Size())
}

// denseDocIndexIterator is the Go port of the anonymous DocIndexIterator
// returned by KnnVectorValues#createDenseIterator(): doc == ord over a dense
// [0, size) range, starting unpositioned at -1.
type denseDocIndexIterator struct {
	doc  int
	size int
}

// newDenseDocIndexIterator returns an iterator positioned before the first
// document, matching createDenseIterator()'s `int doc = -1`.
func newDenseDocIndexIterator(size int) *denseDocIndexIterator {
	return &denseDocIndexIterator{doc: -1, size: size}
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

func (f *floatVectorValuesFromFloats) Scorer(target []float32) (interface{}, error) {
	return nil, errors.New("not implemented")
}

func (f *floatVectorValuesFromFloats) Rescorer(target []float32) (interface{}, error) {
	return f.Scorer(target)
}

func (it *denseDocIndexIterator) DocID() int {
	return it.doc
}

func (it *denseDocIndexIterator) Index() int {
	return it.doc
}

func (it *denseDocIndexIterator) NextDoc() (int, error) {
	if it.doc >= it.size-1 {
		it.doc = util.NO_MORE_DOCS
	} else {
		it.doc++
	}
	return it.doc, nil
}

func (it *denseDocIndexIterator) Advance(target int) (int, error) {
	if target >= it.size {
		it.doc = util.NO_MORE_DOCS
	} else {
		it.doc = target
	}
	return it.doc, nil
}

// DocIDRunEnd returns the exclusive end of the current run. The range is
// dense, so every remaining document matches and the run ends at size,
// mirroring createDenseIterator()'s docIDRunEnd().
func (it *denseDocIndexIterator) DocIDRunEnd() (int, error) {
	return it.size, nil
}

func (it *denseDocIndexIterator) Cost() int64 {
	return int64(it.size)
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (it *denseDocIndexIterator) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(it, upTo, bitSet, offset)
}
