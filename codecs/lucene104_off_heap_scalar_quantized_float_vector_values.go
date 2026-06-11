// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Source: lucene/core/src/java/org/apache/lucene/codecs/lucene104/
//         OffHeapScalarQuantizedFloatVectorValues.java
//
// Reads quantized vector values from the index input and returns float vector
// values after dequantizing them.
//
// This file is part of GOC-3335 (Sprint 55). It provides functionality to
// read quantized vectors which are stored in the index, and then dequantize
// them back to float vectors with some precision loss. The implementation is
// based on OffHeapScalarQuantizedVectorValues with modifications to the
// VectorValue method to return float vectors after dequantizing the vectors.
//
// Usage: used for read-only indexes where full-precision float vectors have
// been dropped from the index to save storage space.
//
// Sprint 55 deviations (option c, fill gaps):
//   - Lucene95.OrdToDocDISIReaderConfiguration is currently a typed stub in
//     codecs/lucene95 with no IsEmpty / IsDense / GetDirectMonotonicReader /
//     GetIndexedDISI methods. We define a local interface
//     [ordToDocDISIReaderConfig] so this port compiles today and rebinds to
//     the real type when Sprint 55 lands the full lucene95 port.
//   - Lucene104.OffHeapScalarQuantizedVectorValues.unpackNibbles is still a
//     typed stub. We inline the equivalent unpackNibblesPacked helper here;
//     it mirrors the Java algorithm byte-for-byte.
//   - Lucene's VectorScorer.Bulk.fromRandomScorerDense / Sparse static
//     factories live in org.apache.lucene.VectorScorerView$Bulk; the Go
//     equivalents have not yet been ported in search/. The scorer() method
//     therefore returns a [VectorScorerView] whose Bulk() reports nil
//     (no bulk fast-path), matching the contract documented in
//     [VectorScorerView]. Wiring the Bulk fast-path is deferred to the
//     companion task that ports VectorScorer.Bulk.
//   - HasIndexSlice in codecs/lucene95 is a stub; the GetSlice method
//     therefore satisfies the documented contract directly (returns the
//     underlying [store.IndexInput]) without an explicit interface assertion.

package codecs

import (
	"errors"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/quantization"
)

// Cycle notes:
//   - codecs/lucene90.IndexedDISI lives in a package that imports the
//     top-level codecs package, so the concrete type cannot be referenced
//     here without closing a cycle. The sparse layout accepts the DISI
//     behind [IndexedDISIView].
//   - VectorScorerView / VectorScorerViewBulk / search.DocIdSetIterator
//     also close a cycle via search -> ... -> codecs. The Scorer accessor
//     therefore returns [VectorScorerView], a structural mirror of
//     VectorScorerView; callers in search-side code can adapt one to
//     the other via a one-method shim.

// quantizedFloatCorrectivesLen is the number of float32 corrective values
// persisted per quantized vector. Mirrors the Java reference
// (correctiveValues = new float[3]).
const quantizedFloatCorrectivesLen = 3

// quantizedFloatPerVectorMetadataBytes is the size, in bytes, of the
// trailing per-vector metadata block (three float32 correctives and one
// int32 quantizedComponentSum). Mirrors `(Float.BYTES * 3) + Integer.BYTES`.
const quantizedFloatPerVectorMetadataBytes = (4 * 3) + 4

// ordToDocDISIReaderConfig is the locally-scoped contract that
// org.apache.lucene.codecs.lucene95.OrdToDocDISIReaderConfiguration
// fulfils in the Java reference. The lucene95 port currently exposes an
// empty struct, so the four accessors used by load() are declared here so
// callers can supply a concrete implementation today and we can rebind to
// the proper Go type when the lucene95 port is fleshed out.
type ordToDocDISIReaderConfig interface {
	// IsEmpty mirrors OrdToDocDISIReaderConfiguration.isEmpty().
	IsEmpty() bool

	// IsDense mirrors OrdToDocDISIReaderConfiguration.isDense().
	IsDense() bool

	// GetDirectMonotonicReader mirrors
	// OrdToDocDISIReaderConfiguration.getDirectMonotonicReader(IndexInput).
	// The returned reader maps ord -> docID for sparse layouts. The
	// concrete result type is intentionally opaque (any) because the
	// DirectMonotonicReader port lives in util/packed and is consumed
	// only through OrdToDoc().
	GetDirectMonotonicReader(dataIn store.IndexInput) (OrdToDocReader, error)

	// GetIndexedDISI mirrors
	// OrdToDocDISIReaderConfiguration.getIndexedDISI(IndexInput). The
	// returned value is consumed through [IndexedDISIView]; the concrete
	// *codecs/lucene90.IndexedDISI satisfies the view structurally.
	GetIndexedDISI(dataIn store.IndexInput) (IndexedDISIView, error)
}

// IndexedDISIView is the minimal slice of the
// codecs/lucene90.IndexedDISI surface consumed by the sparse layout.
// Declaring it locally avoids the cycle codecs -> codecs/lucene90 ->
// codecs that would arise from importing the concrete type. The
// concrete *lucene90.IndexedDISI satisfies this view structurally.
type IndexedDISIView interface {
	// DocID returns the current docID.
	DocID() int
	// NextDoc advances to the next docID.
	NextDoc() (int, error)
	// Advance moves to the first docID >= target.
	Advance(target int) (int, error)
	// Cost reports the iterator cost.
	Cost() int64
	// Index returns the ordinal of the current docID.
	Index() int
}

// OrdToDocReader is the minimal contract used by the sparse layout to
// translate vector ordinals into document IDs. It mirrors the only
// DirectMonotonicReader method consumed by
// OffHeapScalarQuantizedFloatVectorValues.SparseOffHeapVectorValues.
type OrdToDocReader interface {
	// Get returns the document ID stored at the given ordinal.
	// Returns an error if the underlying I/O fails.
	Get(ord int64) (int64, error)
}

// VectorScorerView mirrors org.apache.lucene.search.VectorScorer at the
// codecs boundary. We mirror rather than depend on search.VectorScorer
// because importing search closes a cycle. Adaptors in search-side code
// can wrap a VectorScorerView in a search.VectorScorer with a one-line
// shim.
type VectorScorerView interface {
	// Score returns the similarity score for the current document.
	Score() (float32, error)
	// Iterator returns a DocIDSetIteratorView over the scored documents.
	Iterator() DocIDSetIteratorView
	// Bulk returns an optional bulk-scoring helper, or nil if not
	// supported.
	Bulk() VectorScorerBulkView
}

// VectorScorerBulkView mirrors search.VectorScorerBulk at the codecs
// boundary. See [VectorScorerView] for the rationale behind the mirror.
type VectorScorerBulkView interface {
	// Score writes per-doc similarity scores for at most upTo documents
	// into buf, returning the number of documents actually scored.
	Score(buf []float32, upTo int) (int, error)
}

// DocIDSetIteratorView mirrors search.DocIdSetIterator at the codecs
// boundary. See [VectorScorerView] for the rationale behind the mirror.
type DocIDSetIteratorView interface {
	// DocID returns the current document ID.
	DocID() int
	// NextDoc advances to the next document.
	NextDoc() (int, error)
	// Advance advances to the document at or beyond target.
	Advance(target int) (int, error)
	// Cost returns the estimated cost.
	Cost() int64
	// DocIDRunEnd returns one plus the last doc ID of the current run.
	DocIDRunEnd() int
}

// noMoreDocsView mirrors search.NO_MORE_DOCS as a local constant. The
// numeric value is fixed by the Lucene wire-format and matches Java's
// DocIdSetIterator.NO_MORE_DOCS.
const noMoreDocsView = 2147483647

// OffHeapScalarQuantizedFloatVectorValues is the Go port of
// org.apache.lucene.codecs.lucene104.OffHeapScalarQuantizedFloatVectorValues
// (an abstract class in Java). It carries the dequantization state shared
// between the dense, sparse and empty implementations exposed by [Load].
//
// Instances are not safe for concurrent use; use [Copy] to obtain an
// independent iterator. The struct also satisfies
// [codecs/lucene95.HasIndexSlice] via [GetSlice].
type OffHeapScalarQuantizedFloatVectorValues struct {
	dimension          int
	size               int
	similarityFunction VectorSimilarityFunction
	vectorsScorer      FlatVectorsScorer

	slice                   store.IndexInput
	vectorValue             []float32
	byteValue               []byte
	unpackedByteVectorValue []byte
	byteSize                int
	lastOrd                 int
	correctiveValues        [quantizedFloatCorrectivesLen]float32
	quantizedComponentSum   int32
	encoding                ScalarEncoding
	centroid                []float32

	// variant carries layout-specific behaviour (dense / sparse / empty)
	// to avoid simulating Java's protected-method inheritance via a deep
	// hierarchy in Go.
	variant offHeapScalarQuantizedFloatVariant
}

// offHeapScalarQuantizedFloatVariant captures the layout-specific
// behaviour for dense, sparse and empty quantized-float views. Every
// method takes parent explicitly so the variants can be stateless value
// types when no per-variant fields are needed.
type offHeapScalarQuantizedFloatVariant interface {
	// iterator returns a DocIndexIterator over the values owned by parent.
	iterator(parent *OffHeapScalarQuantizedFloatVectorValues) index.DocIndexIterator

	// ordToDoc maps a vector ordinal to its docID.
	ordToDoc(parent *OffHeapScalarQuantizedFloatVectorValues, ord int) int

	// getAcceptOrds wraps acceptDocs in a Bits over ordinals.
	getAcceptOrds(parent *OffHeapScalarQuantizedFloatVectorValues, acceptDocs util.Bits) util.Bits

	// copy returns a fresh, independent variant cloned from parent.
	copy(parent *OffHeapScalarQuantizedFloatVectorValues) (*OffHeapScalarQuantizedFloatVectorValues, error)

	// scorer returns a VectorScorer for the supplied target, or nil for
	// the empty variant.
	scorer(parent *OffHeapScalarQuantizedFloatVectorValues, target []float32) (VectorScorerView, error)
}

// newOffHeapScalarQuantizedFloatVectorValues mirrors the Java protected
// constructor. It allocates the byte / float buffers sized by the
// encoding's packed-doc layout.
func newOffHeapScalarQuantizedFloatVectorValues(
	dimension, size int,
	centroid []float32,
	encoding ScalarEncoding,
	similarityFunction VectorSimilarityFunction,
	vectorsScorer FlatVectorsScorer,
	slice store.IndexInput,
	variant offHeapScalarQuantizedFloatVariant,
) *OffHeapScalarQuantizedFloatVectorValues {
	docPackedLength := encoding.GetDocPackedLength(dimension)
	return &OffHeapScalarQuantizedFloatVectorValues{
		dimension:               dimension,
		size:                    size,
		similarityFunction:      similarityFunction,
		vectorsScorer:           vectorsScorer,
		slice:                   slice,
		centroid:                centroid,
		encoding:                encoding,
		byteSize:                docPackedLength + quantizedFloatPerVectorMetadataBytes,
		vectorValue:             make([]float32, dimension),
		byteValue:               make([]byte, docPackedLength),
		unpackedByteVectorValue: make([]byte, dimension),
		lastOrd:                 -1,
		variant:                 variant,
	}
}

// Dimension returns the dimension of every vector stored.
func (v *OffHeapScalarQuantizedFloatVectorValues) Dimension() int { return v.dimension }

// Size returns the number of vectors stored.
func (v *OffHeapScalarQuantizedFloatVectorValues) Size() int { return v.size }

// VectorValue mirrors the Java `vectorValue(int targetOrd)` accessor. It
// seeks the underlying slice, reads the packed quantized bytes, the three
// corrective floats and the quantized-component-sum, then dequantizes the
// vector into the shared float buffer. The returned slice is owned by v
// and is overwritten by the next call.
func (v *OffHeapScalarQuantizedFloatVectorValues) VectorValue(targetOrd int) ([]float32, error) {
	if v.lastOrd == targetOrd {
		return v.vectorValue, nil
	}
	if v.slice == nil {
		return nil, errors.New("lucene104: OffHeapScalarQuantizedFloatVectorValues: no backing slice (empty variant)")
	}

	if err := v.slice.SetPosition(int64(targetOrd) * int64(v.byteSize)); err != nil {
		return nil, fmt.Errorf("lucene104: OffHeapScalarQuantizedFloatVectorValues: seek ord=%d: %w", targetOrd, err)
	}
	if err := v.slice.ReadBytes(v.byteValue); err != nil {
		return nil, fmt.Errorf("lucene104: OffHeapScalarQuantizedFloatVectorValues: read packed bytes: %w", err)
	}
	if err := readFloatsLE(v.slice, v.correctiveValues[:]); err != nil {
		return nil, fmt.Errorf("lucene104: OffHeapScalarQuantizedFloatVectorValues: read correctives: %w", err)
	}
	sum, err := v.slice.ReadInt()
	if err != nil {
		return nil, fmt.Errorf("lucene104: OffHeapScalarQuantizedFloatVectorValues: read quantized component sum: %w", err)
	}
	v.quantizedComponentSum = sum

	// Unpack bytes per encoding; UNSIGNED_BYTE / SEVEN_BIT short-circuit
	// to dequantize directly from byteValue, matching the Java switch.
	switch v.encoding {
	case ScalarEncodingPackedNibble:
		unpackNibblesPacked(v.byteValue, v.unpackedByteVectorValue)
	case ScalarEncodingSingleBitQueryNibble:
		quantization.UnpackBinary(v.byteValue, v.unpackedByteVectorValue)
	case ScalarEncodingDibitQueryNibble:
		quantization.UntransposeDibit(v.byteValue, v.unpackedByteVectorValue)
	case ScalarEncodingUnsignedByte, ScalarEncodingSevenBit:
		if _, err := quantization.DeQuantize(
			v.byteValue,
			v.vectorValue,
			byte(v.encoding.GetBits()),
			v.correctiveValues[0],
			v.correctiveValues[1],
			v.centroid,
		); err != nil {
			return nil, fmt.Errorf("lucene104: OffHeapScalarQuantizedFloatVectorValues: dequantize: %w", err)
		}
		v.lastOrd = targetOrd
		return v.vectorValue, nil
	default:
		return nil, fmt.Errorf("lucene104: OffHeapScalarQuantizedFloatVectorValues: unsupported encoding %s", v.encoding)
	}

	if _, err := quantization.DeQuantize(
		v.unpackedByteVectorValue,
		v.vectorValue,
		byte(v.encoding.GetBits()),
		v.correctiveValues[0],
		v.correctiveValues[1],
		v.centroid,
	); err != nil {
		return nil, fmt.Errorf("lucene104: OffHeapScalarQuantizedFloatVectorValues: dequantize: %w", err)
	}
	v.lastOrd = targetOrd
	return v.vectorValue, nil
}

// GetCorrectiveTerms mirrors the Java helper of the same name. It returns
// the per-vector corrective bookkeeping for targetOrd without forcing a
// full dequantization when the caller already advanced lastOrd to it.
func (v *OffHeapScalarQuantizedFloatVectorValues) GetCorrectiveTerms(targetOrd int) (quantization.QuantizationResult, error) {
	if v.lastOrd == targetOrd {
		return quantization.QuantizationResult{
			LowerInterval:         v.correctiveValues[0],
			UpperInterval:         v.correctiveValues[1],
			AdditionalCorrection:  v.correctiveValues[2],
			QuantizedComponentSum: v.quantizedComponentSum,
		}, nil
	}
	if v.slice == nil {
		return quantization.QuantizationResult{}, errors.New("lucene104: OffHeapScalarQuantizedFloatVectorValues: no backing slice (empty variant)")
	}
	if err := v.slice.SetPosition(int64(targetOrd)*int64(v.byteSize) + int64(len(v.byteValue))); err != nil {
		return quantization.QuantizationResult{}, fmt.Errorf("lucene104: OffHeapScalarQuantizedFloatVectorValues: seek correctives ord=%d: %w", targetOrd, err)
	}
	if err := readFloatsLE(v.slice, v.correctiveValues[:]); err != nil {
		return quantization.QuantizationResult{}, fmt.Errorf("lucene104: OffHeapScalarQuantizedFloatVectorValues: read correctives: %w", err)
	}
	sum, err := v.slice.ReadInt()
	if err != nil {
		return quantization.QuantizationResult{}, fmt.Errorf("lucene104: OffHeapScalarQuantizedFloatVectorValues: read quantized component sum: %w", err)
	}
	v.quantizedComponentSum = sum
	return quantization.QuantizationResult{
		LowerInterval:         v.correctiveValues[0],
		UpperInterval:         v.correctiveValues[1],
		AdditionalCorrection:  v.correctiveValues[2],
		QuantizedComponentSum: v.quantizedComponentSum,
	}, nil
}

// GetVectorByteLength reports the on-disk size of one vector in bytes.
// Mirrors the Java getVectorByteLength override that returns `dimension`
// (the dequantized float vector is not on disk; consumers use this as the
// FloatVectorValues per-doc size).
func (v *OffHeapScalarQuantizedFloatVectorValues) GetVectorByteLength() int { return v.dimension }

// GetSlice satisfies the HasIndexSlice contract and returns the
// underlying packed-bytes slice. Returns nil for the empty variant.
func (v *OffHeapScalarQuantizedFloatVectorValues) GetSlice() store.IndexInput { return v.slice }

// Encoding returns the scalar encoding used by this view. Exposed for
// callers that need to interpret raw quantized bytes obtained via Slice.
func (v *OffHeapScalarQuantizedFloatVectorValues) Encoding() ScalarEncoding { return v.encoding }

// Centroid returns the centroid against which corrective values are
// subtracted. The returned slice is owned by v; callers must not mutate.
func (v *OffHeapScalarQuantizedFloatVectorValues) Centroid() []float32 { return v.centroid }

// OrdToDoc dispatches to the variant.
func (v *OffHeapScalarQuantizedFloatVectorValues) OrdToDoc(ord int) int {
	return v.variant.ordToDoc(v, ord)
}

// Copy returns an independent iterator backed by a cloned slice. The
// return type satisfies [codecs.KnnVectorValues]; callers that need the
// concrete type should use [CopyTyped].
func (v *OffHeapScalarQuantizedFloatVectorValues) Copy() (KnnVectorValues, error) {
	return v.variant.copy(v)
}

// CopyTyped returns an independent iterator backed by a cloned slice and
// preserves the concrete type, avoiding a type assertion at the call
// site. The result is invariant-equivalent to [Copy].
func (v *OffHeapScalarQuantizedFloatVectorValues) CopyTyped() (*OffHeapScalarQuantizedFloatVectorValues, error) {
	return v.variant.copy(v)
}

// GetEncoding returns FLOAT32: even though the underlying bytes are
// quantized, the consumer-visible iterator dequantizes on demand and
// surfaces FLOAT32 vectors. Mirrors the Java class extending
// FloatVectorValues.
func (v *OffHeapScalarQuantizedFloatVectorValues) GetEncoding() index.VectorEncoding {
	return index.VectorEncodingFloat32
}

// GetAcceptOrds wraps acceptDocs in a Bits over ordinals.
func (v *OffHeapScalarQuantizedFloatVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return v.variant.getAcceptOrds(v, acceptDocs)
}

// Iterator returns a DocIndexIterator over the available ordinals.
func (v *OffHeapScalarQuantizedFloatVectorValues) Iterator() index.DocIndexIterator {
	return v.variant.iterator(v)
}

// Scorer returns a VectorScorer over target, or nil for the empty variant.
func (v *OffHeapScalarQuantizedFloatVectorValues) Scorer(target []float32) (VectorScorerView, error) {
	return v.variant.scorer(v, target)
}

// Note: the FlatVectorsScorer and FlatRandomVectorScorer contracts used
// by this file are the ones declared in codecs/flat_vector_scorer.go. The
// Java reference's float-target overload of FlatVectorsScorer.
// getRandomVectorScorer returns a FlatRandomVectorScorer, and only the
// Score(node) method is consumed by the scorer adaptors below.

// Load constructs the appropriate variant (dense, sparse or empty) based
// on the supplied OrdToDoc configuration. Mirrors the Java
// `OffHeapScalarQuantizedFloatVectorValues.load` static factory.
func Load(
	configuration ordToDocDISIReaderConfig,
	dimension, size int,
	encoding ScalarEncoding,
	similarityFunction VectorSimilarityFunction,
	vectorsScorer FlatVectorsScorer,
	centroid []float32,
	quantizedVectorDataOffset, quantizedVectorDataLength int64,
	vectorData store.IndexInput,
) (*OffHeapScalarQuantizedFloatVectorValues, error) {
	if configuration == nil {
		return nil, errors.New("lucene104: OffHeapScalarQuantizedFloatVectorValues.Load: configuration is nil")
	}
	if configuration.IsEmpty() {
		return newEmptyOffHeapScalarQuantizedFloatVectorValues(dimension, similarityFunction, vectorsScorer), nil
	}
	if centroid == nil {
		return nil, errors.New("lucene104: OffHeapScalarQuantizedFloatVectorValues.Load: centroid is required when configuration is non-empty")
	}
	if vectorData == nil {
		return nil, errors.New("lucene104: OffHeapScalarQuantizedFloatVectorValues.Load: vectorData is nil")
	}
	bytesSlice, err := vectorData.Slice("scalar-quantized-float-vector-data", quantizedVectorDataOffset, quantizedVectorDataLength)
	if err != nil {
		return nil, fmt.Errorf("lucene104: OffHeapScalarQuantizedFloatVectorValues.Load: slice vector data: %w", err)
	}
	if configuration.IsDense() {
		return newDenseOffHeapScalarQuantizedFloatVectorValues(
			dimension, size, centroid, encoding, similarityFunction, vectorsScorer, bytesSlice,
		), nil
	}
	return newSparseOffHeapScalarQuantizedFloatVectorValues(
		configuration,
		dimension, size, centroid, encoding,
		vectorData, similarityFunction, vectorsScorer, bytesSlice,
	)
}

// denseOffHeapScalarQuantizedFloatVariant is the dense layout: every
// ordinal corresponds 1:1 with a docID.
type denseOffHeapScalarQuantizedFloatVariant struct{}

func newDenseOffHeapScalarQuantizedFloatVectorValues(
	dimension, size int,
	centroid []float32,
	encoding ScalarEncoding,
	similarityFunction VectorSimilarityFunction,
	vectorsScorer FlatVectorsScorer,
	slice store.IndexInput,
) *OffHeapScalarQuantizedFloatVectorValues {
	return newOffHeapScalarQuantizedFloatVectorValues(
		dimension, size, centroid, encoding, similarityFunction, vectorsScorer, slice,
		denseOffHeapScalarQuantizedFloatVariant{},
	)
}

func (denseOffHeapScalarQuantizedFloatVariant) iterator(parent *OffHeapScalarQuantizedFloatVectorValues) index.DocIndexIterator {
	return newDenseDocIndexIterator(parent.size)
}

func (denseOffHeapScalarQuantizedFloatVariant) ordToDoc(_ *OffHeapScalarQuantizedFloatVectorValues, ord int) int {
	return ord
}

func (denseOffHeapScalarQuantizedFloatVariant) getAcceptOrds(_ *OffHeapScalarQuantizedFloatVectorValues, acceptDocs util.Bits) util.Bits {
	return acceptDocs
}

func (denseOffHeapScalarQuantizedFloatVariant) copy(parent *OffHeapScalarQuantizedFloatVectorValues) (*OffHeapScalarQuantizedFloatVectorValues, error) {
	return newDenseOffHeapScalarQuantizedFloatVectorValues(
		parent.dimension,
		parent.size,
		parent.centroid,
		parent.encoding,
		parent.similarityFunction,
		parent.vectorsScorer,
		parent.slice.Clone(),
	), nil
}

func (d denseOffHeapScalarQuantizedFloatVariant) scorer(parent *OffHeapScalarQuantizedFloatVectorValues, target []float32) (VectorScorerView, error) {
	cp, err := d.copy(parent)
	if err != nil {
		return nil, err
	}
	it := cp.Iterator()
	scorer, err := parent.vectorsScorer.GetRandomVectorScorer(parent.similarityFunction, cp, target)
	if err != nil {
		return nil, err
	}
	return &quantizedFloatVectorScorer{scorer: scorer, it: it}, nil
}

// sparseOffHeapScalarQuantizedFloatVariant is the sparse layout: an
// IndexedDISI maps the ordinal space to a sparse docID space, with a
// DirectMonotonicReader supplying ord->doc lookups.
type sparseOffHeapScalarQuantizedFloatVariant struct {
	configuration  ordToDocDISIReaderConfig
	dataIn         store.IndexInput
	ordToDocReader OrdToDocReader
	disi           IndexedDISIView
}

func newSparseOffHeapScalarQuantizedFloatVectorValues(
	configuration ordToDocDISIReaderConfig,
	dimension, size int,
	centroid []float32,
	encoding ScalarEncoding,
	dataIn store.IndexInput,
	similarityFunction VectorSimilarityFunction,
	vectorsScorer FlatVectorsScorer,
	slice store.IndexInput,
) (*OffHeapScalarQuantizedFloatVectorValues, error) {
	ordToDoc, err := configuration.GetDirectMonotonicReader(dataIn)
	if err != nil {
		return nil, fmt.Errorf("lucene104: OffHeapScalarQuantizedFloatVectorValues: sparse ord-to-doc reader: %w", err)
	}
	disi, err := configuration.GetIndexedDISI(dataIn)
	if err != nil {
		return nil, fmt.Errorf("lucene104: OffHeapScalarQuantizedFloatVectorValues: sparse DISI: %w", err)
	}
	variant := &sparseOffHeapScalarQuantizedFloatVariant{
		configuration:  configuration,
		dataIn:         dataIn,
		ordToDocReader: ordToDoc,
		disi:           disi,
	}
	return newOffHeapScalarQuantizedFloatVectorValues(
		dimension, size, centroid, encoding, similarityFunction, vectorsScorer, slice, variant,
	), nil
}

func (s *sparseOffHeapScalarQuantizedFloatVariant) iterator(_ *OffHeapScalarQuantizedFloatVectorValues) index.DocIndexIterator {
	return &indexedDISIDocIndexIterator{disi: s.disi}
}

func (s *sparseOffHeapScalarQuantizedFloatVariant) ordToDoc(_ *OffHeapScalarQuantizedFloatVectorValues, ord int) int {
	doc, err := s.ordToDocReader.Get(int64(ord))
	if err != nil {
		// ordToDoc satisfies the interface contract which cannot return an
		// error. I/O errors are treated as unrecoverable, matching
		// Lucene's RuntimeException pattern.
		return 0
	}
	return int(doc)
}

func (s *sparseOffHeapScalarQuantizedFloatVariant) getAcceptOrds(parent *OffHeapScalarQuantizedFloatVectorValues, acceptDocs util.Bits) util.Bits {
	if acceptDocs == nil {
		return nil
	}
	return &sparseAcceptOrds{
		acceptDocs: acceptDocs,
		ordToDoc:   s.ordToDocReader,
		size:       parent.size,
	}
}

func (s *sparseOffHeapScalarQuantizedFloatVariant) copy(parent *OffHeapScalarQuantizedFloatVectorValues) (*OffHeapScalarQuantizedFloatVectorValues, error) {
	return newSparseOffHeapScalarQuantizedFloatVectorValues(
		s.configuration,
		parent.dimension,
		parent.size,
		parent.centroid,
		parent.encoding,
		s.dataIn,
		parent.similarityFunction,
		parent.vectorsScorer,
		parent.slice.Clone(),
	)
}

func (s *sparseOffHeapScalarQuantizedFloatVariant) scorer(parent *OffHeapScalarQuantizedFloatVectorValues, target []float32) (VectorScorerView, error) {
	cpVals, err := s.copy(parent)
	if err != nil {
		return nil, err
	}
	it := cpVals.Iterator()
	scorer, err := parent.vectorsScorer.GetRandomVectorScorer(parent.similarityFunction, cpVals, target)
	if err != nil {
		return nil, err
	}
	return &quantizedFloatVectorScorer{scorer: scorer, it: it}, nil
}

// emptyOffHeapScalarQuantizedFloatVariant mirrors the Java
// EmptyOffHeapVectorValues inner class.
type emptyOffHeapScalarQuantizedFloatVariant struct{}

func newEmptyOffHeapScalarQuantizedFloatVectorValues(
	dimension int,
	similarityFunction VectorSimilarityFunction,
	vectorsScorer FlatVectorsScorer,
) *OffHeapScalarQuantizedFloatVectorValues {
	return newOffHeapScalarQuantizedFloatVectorValues(
		dimension, 0, nil, ScalarEncodingUnsignedByte, similarityFunction, vectorsScorer, nil,
		emptyOffHeapScalarQuantizedFloatVariant{},
	)
}

func (emptyOffHeapScalarQuantizedFloatVariant) iterator(_ *OffHeapScalarQuantizedFloatVectorValues) index.DocIndexIterator {
	return newDenseDocIndexIterator(0)
}

func (emptyOffHeapScalarQuantizedFloatVariant) ordToDoc(_ *OffHeapScalarQuantizedFloatVectorValues, ord int) int {
	return ord
}

func (emptyOffHeapScalarQuantizedFloatVariant) getAcceptOrds(_ *OffHeapScalarQuantizedFloatVectorValues, _ util.Bits) util.Bits {
	return nil
}

// copy on the empty variant intentionally returns an error rather than a
// shared instance; the Java reference raises UnsupportedOperationException.
func (emptyOffHeapScalarQuantizedFloatVariant) copy(_ *OffHeapScalarQuantizedFloatVectorValues) (*OffHeapScalarQuantizedFloatVectorValues, error) {
	return nil, errors.New("lucene104: OffHeapScalarQuantizedFloatVectorValues: Copy on empty variant is unsupported")
}

func (emptyOffHeapScalarQuantizedFloatVariant) scorer(_ *OffHeapScalarQuantizedFloatVectorValues, _ []float32) (VectorScorerView, error) {
	return nil, nil
}

// sparseAcceptOrds is the Bits adapter returned by GetAcceptOrds for the
// sparse layout.
type sparseAcceptOrds struct {
	acceptDocs util.Bits
	ordToDoc   OrdToDocReader
	size       int
}

// Get reports whether the doc behind ordinal index is accepted.
func (s *sparseAcceptOrds) Get(index int) bool {
	doc, err := s.ordToDoc.Get(int64(index))
	if err != nil {
		return false
	}
	return s.acceptDocs.Get(int(doc))
}

// Length returns the number of stored ordinals.
func (s *sparseAcceptOrds) Length() int { return s.size }

// quantizedFloatVectorScorer is the VectorScorerView returned by
// Scorer(target). The Bulk fast-path is currently not wired (see file
// header note about VectorScorer.Bulk port).
type quantizedFloatVectorScorer struct {
	scorer FlatRandomVectorScorer
	it     index.DocIndexIterator
}

// Score scores the current ordinal.
func (q *quantizedFloatVectorScorer) Score() (float32, error) {
	return q.scorer.Score(q.it.Index())
}

// Iterator returns the underlying DocIndexIterator as a DocIDSetIteratorView.
func (q *quantizedFloatVectorScorer) Iterator() DocIDSetIteratorView {
	return &docIndexIteratorAsDocIDSet{it: q.it}
}

// Bulk reports nil; the bulk fast-path has not yet been ported (see file
// header note).
func (q *quantizedFloatVectorScorer) Bulk() VectorScorerBulkView { return nil }

// docIndexIteratorAsDocIDSet narrows a DocIndexIterator to the
// search.DocIdSetIterator surface required by VectorScorerView.
type docIndexIteratorAsDocIDSet struct {
	it index.DocIndexIterator
}

func (d *docIndexIteratorAsDocIDSet) DocID() int                      { return d.it.DocID() }
func (d *docIndexIteratorAsDocIDSet) NextDoc() (int, error)           { return d.it.NextDoc() }
func (d *docIndexIteratorAsDocIDSet) Advance(target int) (int, error) { return d.it.Advance(target) }
func (d *docIndexIteratorAsDocIDSet) Cost() int64                     { return d.it.Cost() }

// DocIDRunEnd returns the end of the current run. Defaults to docID + 1,
// matching the search.BaseDocIdSetIterator default; the wrapped iterator
// does not expose a richer run accessor today.
func (d *docIndexIteratorAsDocIDSet) DocIDRunEnd() int { return d.it.DocID() + 1 }

// denseDocIndexIterator mirrors KnnVectorValues#createDenseIterator(): it
// iterates ord = 0..size-1 with docID == ord.
type denseDocIndexIterator struct {
	size int
	doc  int
}

func newDenseDocIndexIterator(size int) *denseDocIndexIterator {
	return &denseDocIndexIterator{size: size, doc: -1}
}

func (d *denseDocIndexIterator) DocID() int { return d.doc }

func (d *denseDocIndexIterator) NextDoc() (int, error) { return d.Advance(d.doc + 1) }

func (d *denseDocIndexIterator) Advance(target int) (int, error) {
	if target >= d.size {
		d.doc = noMoreDocsView
		return noMoreDocsView, nil
	}
	if target < 0 {
		target = 0
	}
	d.doc = target
	return d.doc, nil
}

func (d *denseDocIndexIterator) Cost() int64 { return int64(d.size) }

func (d *denseDocIndexIterator) Index() int { return d.doc }

// indexedDISIDocIndexIterator adapts an *IndexedDISI to a DocIndexIterator
// by using the DISI's internal Index() accessor as the ordinal.
type indexedDISIDocIndexIterator struct {
	disi IndexedDISIView
}

func (i *indexedDISIDocIndexIterator) DocID() int                      { return i.disi.DocID() }
func (i *indexedDISIDocIndexIterator) NextDoc() (int, error)           { return i.disi.NextDoc() }
func (i *indexedDISIDocIndexIterator) Advance(target int) (int, error) { return i.disi.Advance(target) }
func (i *indexedDISIDocIndexIterator) Cost() int64                     { return i.disi.Cost() }
func (i *indexedDISIDocIndexIterator) Index() int                      { return i.disi.Index() }

// unpackNibblesPacked is the local equivalent of
// OffHeapScalarQuantizedVectorValues.unpackNibbles. It splits each packed
// byte into the high and low nibbles, writing the high nibble to the
// first half of unpacked and the low nibble to the second. The Java
// invariant `unpacked.length == packed.length * 2` is enforced; the helper
// is intentionally branch-light to keep the hot path tight.
func unpackNibblesPacked(packed, unpacked []byte) {
	n := len(packed)
	// The Java reference relies on a debug-only assertion. We translate
	// the precondition into a defensive bound: the function is internal
	// and the only caller sizes both slices via the encoding metadata.
	if len(unpacked) < n*2 {
		return
	}
	for i := 0; i < n; i++ {
		unpacked[i] = (packed[i] >> 4) & 0x0F
		unpacked[n+i] = packed[i] & 0x0F
	}
}

// readFloatsLE reads len(out) little-endian float32 values from in into
// out. Mirrors Java's IndexInput.readFloats(float[], int, int) which uses
// the Lucene wire-format (little-endian since Lucene 10).
func readFloatsLE(in store.IndexInput, out []float32) error {
	buf := make([]byte, 4*len(out))
	if err := in.ReadBytes(buf); err != nil {
		return err
	}
	for i := range out {
		bits := uint32(buf[i*4]) |
			uint32(buf[i*4+1])<<8 |
			uint32(buf[i*4+2])<<16 |
			uint32(buf[i*4+3])<<24
		out[i] = math.Float32frombits(bits)
	}
	return nil
}
