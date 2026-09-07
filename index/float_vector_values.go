package index

import (
	"errors"
	"fmt"
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

// CheckField checks the Vector Encoding of a field.
// This is the Go port of FloatVectorValues.checkField.
func CheckField(in LeafReader, field string) error {
	fi := in.GetFieldInfos().FieldInfo(field)
	if fi != nil && fi.HasVectorValues() && fi.GetVectorEncoding() != VectorEncodingFloat32 {
		return fmt.Errorf("unexpected vector encoding (%s) for field %s (expected=%s)",
			fi.GetVectorEncoding(), field, VectorEncodingFloat32)
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
	return &denseDocIndexIterator{
		size: f.Size(),
	}
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

type acceptOrdsBitSet struct {
	acceptDocs util.Bits
	size       int
}

func (b *acceptOrdsBitSet) Get(index int) bool {
	return b.acceptDocs.Get(b.OrdToDoc(index))
}

func (b *acceptOrdsBitSet) Length() int {
	return b.size
}

func (b *acceptOrdsBitSet) OrdToDoc(index int) int {
	return index
}

type denseDocIndexIterator struct {
	size int
	doc  int
}

func NewDenseDocIndexIterator(size int) *denseDocIndexIterator {
	return &denseDocIndexIterator{
		size: size,
		doc:  -1,
	}
}

func (it *denseDocIndexIterator) DocID() int {
	return it.doc
}

func (it *denseDocIndexIterator) Index() int {
	return it.doc
}

func (it *denseDocIndexIterator) NextDoc() (int, error) {
	if it.doc >= it.size-1 {
		it.doc = NO_MORE_DOCS
	} else {
		it.doc++
	}
	return it.doc, nil
}

func (it *denseDocIndexIterator) Advance(target int) (int, error) {
	if target >= it.size {
		it.doc = NO_MORE_DOCS
	} else {
		it.doc = target
	}
	return it.doc, nil
}

func (it *denseDocIndexIterator) Cost() int64 {
	return int64(it.size)
}
