// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene99

import (
	"errors"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	codecs_lucene90 "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/quantization"
)

// Deviations from the Java reference (Lucene 10.4.0):
//
//   - OrdToDocDISIReaderConfiguration in codecs/lucene95 is an empty stub;
//     we declare a local interface [offHeap99OrdToDocConfig] matching the four
//     accessors consumed by LoadQuantizedFloat.
//   - Java's abstract class hierarchy (OffHeapQuantizedFloatVectorValues,
//     DenseOffHeapVectorValues, SparseOffHeapVectorValues,
//     EmptyOffHeapVectorValues) is replaced by a single concrete struct
//     with a [offHeap99Variant] strategy interface — same approach as the
//     lucene104 port in codecs/lucene104_off_heap_scalar_quantized_float_vector_values.go.
//   - VectorScorer / DocIdSetIterator import cycles are avoided with local
//     view interfaces mirroring codecs.VectorScorerView /
//     codecs.DocIDSetIteratorView.

// offHeap99OrdToDocConfig is the contract that
// org.apache.lucene.codecs.lucene95.OrdToDocDISIReaderConfiguration fulfils.
type offHeap99OrdToDocConfig interface {
	IsDense() bool
	IsEmpty() bool
	GetDirectMonotonicReader(dataIn store.IndexInput) (ordToDocReader99, error)
	GetIndexedDISI(dataIn store.IndexInput) (*codecs_lucene90.IndexedDISI, error)
}

// ordToDocReader99 mirrors DirectMonotonicReader — the only method consumed
// by the sparse variant.
type ordToDocReader99 interface {
	Get(ord int64) (int64, error)
}

// noMoreDocs99 mirrors DocIdSetIterator.NO_MORE_DOCS.
const noMoreDocs99 = 2147483647

// OffHeapQuantizedFloatVectorValues reads quantized byte vectors from the
// index and dequantizes them on the fly to []float32.
//
// Port of
// org.apache.lucene.backward_codecs.lucene99.OffHeapQuantizedFloatVectorValues
// (Lucene 10.4.0).
//
// Instances are not safe for concurrent use; use Copy to obtain an independent
// iterator.
type OffHeapQuantizedFloatVectorValues struct {
	dimension          int
	size               int
	numBytes           int // compressed bytes per vector (without corrective)
	byteSize           int // numBytes + Float.BYTES
	compress           bool
	scalarQuantizer    *quantization.ScalarQuantizer
	similarityFunction codecs.VectorSimilarityFunction
	vectorsScorer      codecs.FlatVectorsScorer

	slice                   store.IndexInput
	floatValue              []float32
	binaryValue             []byte
	curOrd                  int
	scoreCorrectionConstant float32

	variant offHeap99Variant
}

// offHeap99Variant captures layout-specific behaviour.
type offHeap99Variant interface {
	iterator(parent *OffHeapQuantizedFloatVectorValues) index.DocIndexIterator
	ordToDoc(parent *OffHeapQuantizedFloatVectorValues, ord int) int
	getAcceptOrds(parent *OffHeapQuantizedFloatVectorValues, acceptDocs util.Bits) util.Bits
	copy(parent *OffHeapQuantizedFloatVectorValues) (*OffHeapQuantizedFloatVectorValues, error)
	scorer(parent *OffHeapQuantizedFloatVectorValues, target []float32) (codecs.VectorScorerView, error)
}

func newOffHeapQuantizedFloatVectorValues(
	dimension, size int,
	scalarQuantizer *quantization.ScalarQuantizer,
	compress bool,
	similarityFunction codecs.VectorSimilarityFunction,
	vectorsScorer codecs.FlatVectorsScorer,
	slice store.IndexInput,
	variant offHeap99Variant,
) *OffHeapQuantizedFloatVectorValues {
	var numBytes int
	if scalarQuantizer.GetBits() <= 4 && compress {
		numBytes = (dimension + 1) >> 1
	} else {
		numBytes = dimension
	}
	return &OffHeapQuantizedFloatVectorValues{
		dimension:          dimension,
		size:               size,
		numBytes:           numBytes,
		byteSize:           numBytes + 4, // numBytes + Float.BYTES
		compress:           compress,
		scalarQuantizer:    scalarQuantizer,
		similarityFunction: similarityFunction,
		vectorsScorer:      vectorsScorer,
		slice:              slice,
		floatValue:         make([]float32, dimension),
		binaryValue:        make([]byte, dimension),
		curOrd:             -1,
		variant:            variant,
	}
}

// Dimension returns the vector dimension.
func (v *OffHeapQuantizedFloatVectorValues) Dimension() int { return v.dimension }

// Size returns the number of vectors.
func (v *OffHeapQuantizedFloatVectorValues) Size() int { return v.size }

// GetEncoding returns FLOAT32: even though the stored bytes are quantized,
// vectors are dequantized on demand and presented as float32 values.
func (v *OffHeapQuantizedFloatVectorValues) GetEncoding() index.VectorEncoding {
	return index.VectorEncodingFloat32
}

// GetVectorByteLength returns the number of compressed bytes per vector
// (without the trailing corrective float).
func (v *OffHeapQuantizedFloatVectorValues) GetVectorByteLength() int { return v.numBytes }

// GetSlice returns the underlying IndexInput slice (HasIndexSlice contract).
func (v *OffHeapQuantizedFloatVectorValues) GetSlice() store.IndexInput { return v.slice }

// VectorValue dequantizes and returns the float vector for the given ordinal.
//
// Port of OffHeapQuantizedFloatVectorValues.vectorValue(int).
func (v *OffHeapQuantizedFloatVectorValues) VectorValue(targetOrd int) ([]float32, error) {
	if v.curOrd == targetOrd {
		return v.floatValue, nil
	}
	if err := v.slice.SetPosition(int64(targetOrd) * int64(v.byteSize)); err != nil {
		return nil, fmt.Errorf("lucene99 off-heap quantized float: seek to ord %d: %w", targetOrd, err)
	}
	if err := v.slice.ReadBytes(v.binaryValue[:v.numBytes]); err != nil {
		return nil, fmt.Errorf("lucene99 off-heap quantized float: read bytes: %w", err)
	}
	// Read one little-endian float32 corrective value.
	if sc, err := readOneLEFloat32(v.slice); err != nil {
		return nil, fmt.Errorf("lucene99 off-heap quantized float: read corrective: %w", err)
	} else {
		v.scoreCorrectionConstant = sc
	}
	// Decompress nibble-packed bytes if needed.
	decompressBytes99(v.binaryValue, v.numBytes)
	// Dequantize into floatValue.
	v.scalarQuantizer.DeQuantize(v.binaryValue[:v.dimension], v.floatValue)
	v.curOrd = targetOrd
	return v.floatValue, nil
}

// Iterator returns a DocIndexIterator over this vector set.
func (v *OffHeapQuantizedFloatVectorValues) Iterator() index.DocIndexIterator {
	return v.variant.iterator(v)
}

// OrdToDoc maps a vector ordinal to its document ID.
func (v *OffHeapQuantizedFloatVectorValues) OrdToDoc(ord int) int {
	return v.variant.ordToDoc(v, ord)
}

// GetAcceptOrds wraps acceptDocs for ordinal-based access.
func (v *OffHeapQuantizedFloatVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return v.variant.getAcceptOrds(v, acceptDocs)
}

// Copy returns an independent iterator over the same data.
func (v *OffHeapQuantizedFloatVectorValues) Copy() (codecs.KnnVectorValues, error) {
	return v.variant.copy(v)
}

// Scorer returns a VectorScorerView for the given target, or nil for the empty variant.
func (v *OffHeapQuantizedFloatVectorValues) Scorer(target []float32) (codecs.VectorScorerView, error) {
	return v.variant.scorer(v, target)
}

// LoadQuantizedFloat constructs the appropriate variant (dense, sparse or empty)
// based on the OrdToDoc configuration.
//
// Port of OffHeapQuantizedFloatVectorValues.load (Lucene 10.4.0).
func LoadQuantizedFloat(
	configuration offHeap99OrdToDocConfig,
	dimension, size int,
	scalarQuantizer *quantization.ScalarQuantizer,
	similarityFunction codecs.VectorSimilarityFunction,
	vectorsScorer codecs.FlatVectorsScorer,
	compress bool,
	quantizedVectorDataOffset, quantizedVectorDataLength int64,
	vectorData store.IndexInput,
) (*OffHeapQuantizedFloatVectorValues, error) {
	if configuration == nil {
		return nil, errors.New("lucene99: LoadQuantizedFloat: configuration is nil")
	}
	if size == 0 || configuration.IsEmpty() {
		return newEmptyOffHeap99(dimension, similarityFunction, vectorsScorer), nil
	}
	bytesSlice, err := vectorData.Slice(
		"quantized-float-vector-data",
		quantizedVectorDataOffset, quantizedVectorDataLength)
	if err != nil {
		return nil, fmt.Errorf("lucene99: LoadQuantizedFloat: slice vector data: %w", err)
	}
	if configuration.IsDense() {
		return newOffHeapQuantizedFloatVectorValues(
			dimension, size, scalarQuantizer, compress,
			similarityFunction, vectorsScorer, bytesSlice,
			&denseOffHeap99Variant{},
		), nil
	}
	return newSparseOffHeap99(
		configuration, dimension, size, scalarQuantizer, compress,
		vectorData, similarityFunction, vectorsScorer, bytesSlice,
	)
}

// ---------------------------------------------------------------------------
// Dense variant
// ---------------------------------------------------------------------------

type denseOffHeap99Variant struct{}

func (denseOffHeap99Variant) iterator(parent *OffHeapQuantizedFloatVectorValues) index.DocIndexIterator {
	return newDenseDocIter99(parent.size)
}

func (denseOffHeap99Variant) ordToDoc(_ *OffHeapQuantizedFloatVectorValues, ord int) int {
	return ord
}

func (denseOffHeap99Variant) getAcceptOrds(_ *OffHeapQuantizedFloatVectorValues, acceptDocs util.Bits) util.Bits {
	return acceptDocs
}

func (denseOffHeap99Variant) copy(parent *OffHeapQuantizedFloatVectorValues) (*OffHeapQuantizedFloatVectorValues, error) {
	cloned := parent.slice.Clone()
	return newOffHeapQuantizedFloatVectorValues(
		parent.dimension, parent.size, parent.scalarQuantizer, parent.compress,
		parent.similarityFunction, parent.vectorsScorer, cloned,
		denseOffHeap99Variant{},
	), nil
}

func (denseOffHeap99Variant) scorer(parent *OffHeapQuantizedFloatVectorValues, target []float32) (codecs.VectorScorerView, error) {
	cp, err := denseOffHeap99Variant{}.copy(parent)
	if err != nil {
		return nil, err
	}
	it := cp.Iterator()
	scorer, err := parent.vectorsScorer.GetRandomVectorScorer(parent.similarityFunction, cp, target)
	if err != nil {
		return nil, err
	}
	return &quantizedFloatScorerView99{it: it, scorer: scorer}, nil
}

// ---------------------------------------------------------------------------
// Sparse variant
// ---------------------------------------------------------------------------

type sparseOffHeap99Variant struct {
	ordToDocReader ordToDocReader99
	disi           *codecs_lucene90.IndexedDISI
	configuration  offHeap99OrdToDocConfig
	dataIn         store.IndexInput
}

func newSparseOffHeap99(
	configuration offHeap99OrdToDocConfig,
	dimension, size int,
	scalarQuantizer *quantization.ScalarQuantizer,
	compress bool,
	dataIn store.IndexInput,
	similarityFunction codecs.VectorSimilarityFunction,
	vectorsScorer codecs.FlatVectorsScorer,
	slice store.IndexInput,
) (*OffHeapQuantizedFloatVectorValues, error) {
	otr, err := configuration.GetDirectMonotonicReader(dataIn)
	if err != nil {
		return nil, fmt.Errorf("lucene99: sparse off-heap: get monotonic reader: %w", err)
	}
	disi, err := configuration.GetIndexedDISI(dataIn)
	if err != nil {
		return nil, fmt.Errorf("lucene99: sparse off-heap: get IndexedDISI: %w", err)
	}
	v := &sparseOffHeap99Variant{
		ordToDocReader: otr,
		disi:           disi,
		configuration:  configuration,
		dataIn:         dataIn,
	}
	return newOffHeapQuantizedFloatVectorValues(
		dimension, size, scalarQuantizer, compress,
		similarityFunction, vectorsScorer, slice, v,
	), nil
}

func (s *sparseOffHeap99Variant) iterator(_ *OffHeapQuantizedFloatVectorValues) index.DocIndexIterator {
	return &indexedDISIIter99{disi: s.disi}
}

func (s *sparseOffHeap99Variant) ordToDoc(_ *OffHeapQuantizedFloatVectorValues, ord int) int {
	doc, err := s.ordToDocReader.Get(int64(ord))
	if err != nil {
		return 0
	}
	return int(doc)
}

func (s *sparseOffHeap99Variant) getAcceptOrds(parent *OffHeapQuantizedFloatVectorValues, acceptDocs util.Bits) util.Bits {
	if acceptDocs == nil {
		return nil
	}
	return &ordinalBits99{accept: acceptDocs, v: parent}
}

func (s *sparseOffHeap99Variant) copy(parent *OffHeapQuantizedFloatVectorValues) (*OffHeapQuantizedFloatVectorValues, error) {
	cloned := parent.slice.Clone()
	return newSparseOffHeap99(
		s.configuration, parent.dimension, parent.size,
		parent.scalarQuantizer, parent.compress, s.dataIn,
		parent.similarityFunction, parent.vectorsScorer, cloned,
	)
}

func (s *sparseOffHeap99Variant) scorer(parent *OffHeapQuantizedFloatVectorValues, target []float32) (codecs.VectorScorerView, error) {
	cp, err := s.copy(parent)
	if err != nil {
		return nil, err
	}
	it := cp.Iterator()
	scorer, err := parent.vectorsScorer.GetRandomVectorScorer(parent.similarityFunction, cp, target)
	if err != nil {
		return nil, err
	}
	return &quantizedFloatScorerView99{it: it, scorer: scorer}, nil
}

// ---------------------------------------------------------------------------
// Empty variant
// ---------------------------------------------------------------------------

type emptyOffHeap99Variant struct{}

func newEmptyOffHeap99(
	dimension int,
	similarityFunction codecs.VectorSimilarityFunction,
	vectorsScorer codecs.FlatVectorsScorer,
) *OffHeapQuantizedFloatVectorValues {
	// Use bits=7 as a no-op placeholder matching the Java:
	// new ScalarQuantizer(-1, 1, (byte) 7).
	sq, _ := quantization.NewScalarQuantizer(-1, 1, 7) //nolint:errcheck // valid args: -1<1, bits=7
	return newOffHeapQuantizedFloatVectorValues(
		dimension, 0, sq, false,
		similarityFunction, vectorsScorer, nil,
		emptyOffHeap99Variant{},
	)
}

func (emptyOffHeap99Variant) iterator(parent *OffHeapQuantizedFloatVectorValues) index.DocIndexIterator {
	return newDenseDocIter99(0)
}

func (emptyOffHeap99Variant) ordToDoc(_ *OffHeapQuantizedFloatVectorValues, _ int) int {
	panic("lucene99: emptyOffHeap99Variant.ordToDoc not supported")
}

func (emptyOffHeap99Variant) getAcceptOrds(_ *OffHeapQuantizedFloatVectorValues, _ util.Bits) util.Bits {
	return nil
}

func (emptyOffHeap99Variant) copy(_ *OffHeapQuantizedFloatVectorValues) (*OffHeapQuantizedFloatVectorValues, error) {
	return nil, errors.New("lucene99: emptyOffHeap99Variant.copy not supported")
}

func (emptyOffHeap99Variant) scorer(_ *OffHeapQuantizedFloatVectorValues, _ []float32) (codecs.VectorScorerView, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// decompressBytes99 expands nibble-packed bytes in-place.
// If numBytes == len(compressed) the data is already byte-per-nibble; no-op.
// If 2*numBytes == len(compressed), each byte in [0..numBytes) holds two nibbles;
// the upper nibble is written to [0..numBytes) and the lower to [numBytes..2*numBytes).
//
// Port of OffHeapQuantizedByteVectorValues.decompressBytes (Lucene 10.4.0).
func decompressBytes99(compressed []byte, numBytes int) {
	if numBytes == len(compressed) {
		return
	}
	for i := 0; i < numBytes; i++ {
		compressed[numBytes+i] = compressed[i] & 0x0F
		compressed[i] = (compressed[i] >> 4) & 0x0F
	}
}

// readOneLEFloat32 reads a single little-endian float32 from in.
func readOneLEFloat32(in store.IndexInput) (float32, error) {
	var buf [4]byte
	if err := in.ReadBytes(buf[:]); err != nil {
		return 0, err
	}
	bits := uint32(buf[0]) | uint32(buf[1])<<8 | uint32(buf[2])<<16 | uint32(buf[3])<<24
	return math.Float32frombits(bits), nil
}

// ---------------------------------------------------------------------------
// Dense DocIndexIterator
// ---------------------------------------------------------------------------

type denseDocIter99 struct {
	doc  int
	size int
}

func newDenseDocIter99(size int) *denseDocIter99 {
	return &denseDocIter99{doc: -1, size: size}
}

func (d *denseDocIter99) DocID() int { return d.doc }

func (d *denseDocIter99) NextDoc() (int, error) { return d.Advance(d.doc + 1) }

func (d *denseDocIter99) Advance(target int) (int, error) {
	if target >= d.size {
		d.doc = noMoreDocs99
		return d.doc, nil
	}
	d.doc = target
	return d.doc, nil
}

func (d *denseDocIter99) Cost() int64 { return int64(d.size) }

func (d *denseDocIter99) Index() int { return d.doc }

// ---------------------------------------------------------------------------
// IndexedDISI DocIndexIterator wrapper
// ---------------------------------------------------------------------------

type indexedDISIIter99 struct {
	disi *codecs_lucene90.IndexedDISI
}

func (i *indexedDISIIter99) DocID() int { return i.disi.DocID() }

func (i *indexedDISIIter99) NextDoc() (int, error) { return i.disi.NextDoc() }

func (i *indexedDISIIter99) Advance(target int) (int, error) { return i.disi.Advance(target) }

func (i *indexedDISIIter99) Cost() int64 { return i.disi.Cost() }

func (i *indexedDISIIter99) Index() int { return i.disi.Index() }

// ---------------------------------------------------------------------------
// Ordinal-keyed Bits (sparse variant's getAcceptOrds)
// ---------------------------------------------------------------------------

type ordinalBits99 struct {
	accept util.Bits
	v      *OffHeapQuantizedFloatVectorValues
}

func (b *ordinalBits99) Get(index int) bool {
	return b.accept.Get(b.v.OrdToDoc(index))
}

func (b *ordinalBits99) Length() int { return b.v.size }

// ---------------------------------------------------------------------------
// VectorScorerView
// ---------------------------------------------------------------------------

// FlatRandomVectorScorer99 is the minimal scorer contract consumed by the
// VectorScorerView wrapper. It mirrors codecs.FlatRandomVectorScorer.
type FlatRandomVectorScorer99 interface {
	Score(node int) (float32, error)
}

type quantizedFloatScorerView99 struct {
	it     index.DocIndexIterator
	scorer FlatRandomVectorScorer99
}

func (s *quantizedFloatScorerView99) Score() (float32, error) {
	return s.scorer.Score(s.it.Index())
}

func (s *quantizedFloatScorerView99) Iterator() codecs.DocIDSetIteratorView {
	return &docIndexIterToView99{it: s.it}
}

func (s *quantizedFloatScorerView99) Bulk() codecs.VectorScorerBulkView { return nil }

// docIndexIterToView99 adapts index.DocIndexIterator to codecs.DocIDSetIteratorView.
type docIndexIterToView99 struct{ it index.DocIndexIterator }

func (d *docIndexIterToView99) DocID() int                 { return d.it.DocID() }
func (d *docIndexIterToView99) NextDoc() (int, error)      { return d.it.NextDoc() }
func (d *docIndexIterToView99) Advance(t int) (int, error) { return d.it.Advance(t) }
func (d *docIndexIterToView99) Cost() int64                { return d.it.Cost() }
func (d *docIndexIterToView99) DocIDRunEnd() int           { return noMoreDocs99 }
