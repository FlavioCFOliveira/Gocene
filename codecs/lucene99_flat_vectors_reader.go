// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.4.0:
//
//	Licensed to the Apache Software Foundation (ASF) under one or more
//	contributor license agreements. See the NOTICE file distributed with
//	this work for additional information regarding copyright ownership.
//	The ASF licenses this file to You under the Apache License, Version 2.0
//	(the "License"); you may not use this file except in compliance with
//	the License. You may obtain a copy of the License at
//
//	    http://www.apache.org/licenses/LICENSE-2.0

package codecs

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	utilhnsw "github.com/FlavioCFOliveira/Gocene/util/hnsw"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// Lucene99FlatVectorsReader reads raw vector values written by
// [Lucene99FlatVectorsWriter]. It is the Go port of
// org.apache.lucene.codecs.lucene99.Lucene99FlatVectorsReader
// (Lucene 10.4.0), covering the dense, empty and sparse cases.
//
// For the sparse case (rmp #4755) the per-field entry carries the
// OrdToDocDISIReaderConfiguration state (IndexedDISI doc-id set offset/length
// + DirectMonotonicReader ord->doc meta) so the reader can reconstruct the
// sparse vector view.
type Lucene99FlatVectorsReader struct {
	fieldInfos *index.FieldInfos
	fields     map[int]*lucene99FlatFieldEntry // keyed by field number
	vectorData store.IndexInput                // open .vec file
	closed     bool
}

// lucene99FlatFieldEntry mirrors the Java FieldEntry record for the
// flat format, plus the embedded OrdToDocDISIReaderConfiguration state.
type lucene99FlatFieldEntry struct {
	similarityFunction index.VectorSimilarityFunction
	vectorEncoding     index.VectorEncoding
	vectorDataOffset   int64
	vectorDataLength   int64
	dimension          int
	size               int

	// docsWithFieldOffset distinguishes the storage cases:
	//   -2 : empty (no vectors)
	//   -1 : dense (every doc has a vector; ord == doc)
	//  >=0 : sparse (the .vec offset of the IndexedDISI doc-id set)
	docsWithFieldOffset int64

	// The following fields are populated only for the sparse case
	// (docsWithFieldOffset >= 0). They mirror the like-named fields of
	// org.apache.lucene.codecs.lucene95.OrdToDocDISIReaderConfiguration.
	docsWithFieldLength int64
	jumpTableEntryCount int
	denseRankPower      byte
	addressesOffset     int64
	addressesLength     int64
	ordToDocMeta        *packed.DirectMonotonicMeta
}

// NewLucene99FlatVectorsReader reads and validates the `.vemf` header and
// per-field entries, then opens the `.vec` data file. Mirrors the Java
// constructor Lucene99FlatVectorsReader(SegmentReadState, FlatVectorsScorer);
// the scorer parameter is omitted (Gocene resolves the scorer inline).
func NewLucene99FlatVectorsReader(state *SegmentReadState) (*Lucene99FlatVectorsReader, error) {
	r := &Lucene99FlatVectorsReader{
		fieldInfos: state.FieldInfos,
		fields:     make(map[int]*lucene99FlatFieldEntry),
	}

	versionMeta, err := r.readMetadata(state)
	if err != nil {
		return nil, err
	}

	dataName := store.SegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, lucene99FlatDataExtension)
	dataIn, err := state.Directory.OpenInput(dataName, store.IOContextRead)
	if err != nil {
		return nil, fmt.Errorf("lucene99 flat: open data %q: %w", dataName, err)
	}
	id := state.SegmentInfo.GetID()
	versionData, err := CheckIndexHeader(
		dataIn, lucene99FlatDataCodecName,
		lucene99FlatVersionStart, lucene99FlatVersionCurrent,
		id, state.SegmentSuffix,
	)
	if err != nil {
		_ = dataIn.Close()
		return nil, fmt.Errorf("lucene99 flat: data header %q: %w", dataName, err)
	}
	if versionData != versionMeta {
		_ = dataIn.Close()
		return nil, fmt.Errorf("lucene99 flat: format versions mismatch: meta=%d, data=%d",
			versionMeta, versionData)
	}
	if _, err := RetrieveChecksum(dataIn); err != nil {
		_ = dataIn.Close()
		return nil, fmt.Errorf("lucene99 flat: retrieve data checksum %q: %w", dataName, err)
	}
	r.vectorData = dataIn
	return r, nil
}

// readMetadata reads the `.vemf` header and per-field entries. It returns
// the meta version so the caller can cross-check the data file.
func (r *Lucene99FlatVectorsReader) readMetadata(state *SegmentReadState) (int32, error) {
	metaName := store.SegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, lucene99FlatMetaExtension)
	metaRaw, err := state.Directory.OpenInput(metaName, store.IOContextRead)
	if err != nil {
		return 0, fmt.Errorf("lucene99 flat: open meta %q: %w", metaName, err)
	}
	meta := store.NewChecksumIndexInput(metaRaw)

	var versionMeta int32
	var readErr error
	func() {
		id := state.SegmentInfo.GetID()
		v, e := CheckIndexHeader(
			meta, lucene99FlatMetaCodecName,
			lucene99FlatVersionStart, lucene99FlatVersionCurrent,
			id, state.SegmentSuffix,
		)
		if e != nil {
			readErr = e
			return
		}
		versionMeta = v
		readErr = r.readFields(meta)
	}()

	_, footerErr := CheckFooter(meta)
	_ = metaRaw.Close()
	if readErr != nil {
		return 0, fmt.Errorf("lucene99 flat: read meta %q: %w", metaName, readErr)
	}
	if footerErr != nil {
		return 0, fmt.Errorf("lucene99 flat: meta footer %q: %w", metaName, footerErr)
	}
	return versionMeta, nil
}

// readFields parses all per-field entries until the -1 sentinel.
func (r *Lucene99FlatVectorsReader) readFields(meta store.DataInput) error {
	for {
		fieldNum, err := meta.ReadInt()
		if err != nil {
			return fmt.Errorf("reading field number: %w", err)
		}
		if fieldNum == -1 {
			break
		}
		info := r.fieldInfos.GetByNumber(int(fieldNum))
		if info == nil {
			return fmt.Errorf("invalid field number %d", fieldNum)
		}
		entry, err := r.readFieldEntry(meta, info)
		if err != nil {
			return fmt.Errorf("field %d: %w", fieldNum, err)
		}
		r.fields[int(fieldNum)] = entry
	}
	return nil
}

// readFieldEntry parses one FieldEntry from the meta stream. Mirrors
// Java's FieldEntry.create + the OrdToDoc sentinel parse, restricted to
// the dense/empty cases (rmp #4731).
func (r *Lucene99FlatVectorsReader) readFieldEntry(meta store.DataInput, info *index.FieldInfo) (*lucene99FlatFieldEntry, error) {
	encOrd, err := meta.ReadInt()
	if err != nil {
		return nil, err
	}
	enc := index.VectorEncoding(encOrd)

	simOrd, err := meta.ReadInt()
	if err != nil {
		return nil, err
	}
	if int(simOrd) < 0 || int(simOrd) >= len(lucene99HnswSimilarityOrdinals) {
		return nil, fmt.Errorf("invalid similarity ordinal: %d", simOrd)
	}
	sim := lucene99HnswSimilarityOrdinals[simOrd]

	vectorDataOffset, err := meta.ReadVLong()
	if err != nil {
		return nil, err
	}
	vectorDataLength, err := meta.ReadVLong()
	if err != nil {
		return nil, err
	}
	dimV, err := store.ReadVInt(meta)
	if err != nil {
		return nil, err
	}
	size, err := meta.ReadInt()
	if err != nil {
		return nil, err
	}

	// OrdToDocDISIReaderConfiguration.fromStoredMeta. docsWithFieldOffset
	// distinguishes empty(-2) / dense(-1) / sparse(>=0). Mirrors the Java
	// fromStoredMeta read order exactly.
	docsWithFieldOffset, err := meta.ReadLong()
	if err != nil {
		return nil, err
	}
	docsWithFieldLength, err := meta.ReadLong()
	if err != nil {
		return nil, err
	}
	jumpTableEntryCount, err := meta.ReadShort()
	if err != nil {
		return nil, err
	}
	denseRankPower, err := meta.ReadByte()
	if err != nil {
		return nil, err
	}

	var addressesOffset, addressesLength int64
	var ordToDocMeta *packed.DirectMonotonicMeta
	if docsWithFieldOffset > -1 {
		// Sparse: read the DirectMonotonicWriter header that records the
		// ord->doc mapping. Mirrors the docsWithFieldOffset > -1 branch of
		// OrdToDocDISIReaderConfiguration.fromStoredMeta.
		addressesOffset, err = meta.ReadLong()
		if err != nil {
			return nil, err
		}
		blockShift, e := store.ReadVInt(meta)
		if e != nil {
			return nil, e
		}
		ordToDocMeta, err = packed.LoadDirectMonotonicMeta(meta, int64(size), int(blockShift))
		if err != nil {
			return nil, fmt.Errorf("load ord-to-doc monotonic meta: %w", err)
		}
		addressesLength, err = meta.ReadLong()
		if err != nil {
			return nil, err
		}
	}

	// Consistency checks mirroring the Java FieldEntry constructor.
	if sim != info.VectorSimilarityFunction() {
		return nil, fmt.Errorf("inconsistent similarity for field %q: %v != %v",
			info.Name(), sim, info.VectorSimilarityFunction())
	}
	if int(dimV) != info.VectorDimension() {
		return nil, fmt.Errorf("inconsistent dimension for field %q: %d != %d",
			info.Name(), dimV, info.VectorDimension())
	}

	return &lucene99FlatFieldEntry{
		similarityFunction:  sim,
		vectorEncoding:      enc,
		vectorDataOffset:    vectorDataOffset,
		vectorDataLength:    vectorDataLength,
		dimension:           int(dimV),
		size:                int(size),
		docsWithFieldOffset: docsWithFieldOffset,
		docsWithFieldLength: docsWithFieldLength,
		jumpTableEntryCount: int(jumpTableEntryCount),
		denseRankPower:      byte(denseRankPower),
		addressesOffset:     addressesOffset,
		addressesLength:     addressesLength,
		ordToDocMeta:        ordToDocMeta,
	}, nil
}

// getFieldEntry resolves the entry for field, validating the expected
// encoding. Mirrors Java's getFieldEntry.
func (r *Lucene99FlatVectorsReader) getFieldEntry(field string, expected index.VectorEncoding) (*lucene99FlatFieldEntry, error) {
	info := r.fieldInfos.GetByName(field)
	if info == nil {
		return nil, fmt.Errorf("lucene99 flat: field %q not found", field)
	}
	entry, ok := r.fields[info.Number()]
	if !ok {
		return nil, fmt.Errorf("lucene99 flat: field %q has no vector entry", field)
	}
	if entry.vectorEncoding != expected {
		return nil, fmt.Errorf("lucene99 flat: field %q is encoded as %v, expected %v",
			field, entry.vectorEncoding, expected)
	}
	return entry, nil
}

// flatFloatVectorValues is the common surface exposed by the dense, empty
// and sparse off-heap float32 vector views, the Go renderings of
// org.apache.lucene.codecs.lucene95.OffHeapFloatVectorValues and its Dense,
// Sparse and Empty subclasses: the full FloatVectorValues surface plus the
// configured similarity function.
type flatFloatVectorValues interface {
	index.FloatVectorValues
	similarity() index.VectorSimilarityFunction
}

// flatByteVectorValues is the byte analogue of [flatFloatVectorValues]
// (org.apache.lucene.codecs.lucene95.OffHeapByteVectorValues).
type flatByteVectorValues interface {
	index.ByteVectorValues
	similarity() index.VectorSimilarityFunction
}

// floatVectorValues loads the off-heap float32 vectors for field. It returns
// a dense, sparse or empty view depending on the field's
// docsWithFieldOffset.
func (r *Lucene99FlatVectorsReader) floatVectorValues(field string) (flatFloatVectorValues, error) {
	entry, err := r.getFieldEntry(field, index.VectorEncodingFloat32)
	if err != nil {
		return nil, err
	}
	if entry.docsWithFieldOffset == -2 {
		return newFlatDenseFloatVectorValues(entry.dimension, 0, nil, entry.similarityFunction), nil
	}
	slice, err := r.vectorData.Slice("vector-data", entry.vectorDataOffset, entry.vectorDataLength)
	if err != nil {
		return nil, fmt.Errorf("lucene99 flat: slice float vectors for %q: %w", field, err)
	}
	if entry.docsWithFieldOffset == -1 {
		return newFlatDenseFloatVectorValues(entry.dimension, entry.size, slice, entry.similarityFunction), nil
	}
	return r.newFlatSparseFloatVectorValues(entry, slice)
}

// byteVectorValues loads the off-heap byte vectors for field. See
// [floatVectorValues] for the dense/sparse/empty dispatch.
func (r *Lucene99FlatVectorsReader) byteVectorValues(field string) (flatByteVectorValues, error) {
	entry, err := r.getFieldEntry(field, index.VectorEncodingByte)
	if err != nil {
		return nil, err
	}
	if entry.docsWithFieldOffset == -2 {
		return newFlatDenseByteVectorValues(entry.dimension, 0, nil, entry.similarityFunction), nil
	}
	slice, err := r.vectorData.Slice("vector-data", entry.vectorDataOffset, entry.vectorDataLength)
	if err != nil {
		return nil, fmt.Errorf("lucene99 flat: slice byte vectors for %q: %w", field, err)
	}
	if entry.docsWithFieldOffset == -1 {
		return newFlatDenseByteVectorValues(entry.dimension, entry.size, slice, entry.similarityFunction), nil
	}
	return r.newFlatSparseByteVectorValues(entry, slice)
}

// GetFloatVectorValues returns the float vectors for field. Mirrors
// Lucene99FlatVectorsReader.getFloatVectorValues(String), which loads the
// dense, sparse or empty OffHeapFloatVectorValues of the field entry.
func (r *Lucene99FlatVectorsReader) GetFloatVectorValues(field string) (index.FloatVectorValues, error) {
	values, err := r.floatVectorValues(field)
	if err != nil {
		return nil, err
	}
	return values, nil
}

// GetByteVectorValues returns the byte vectors for field. Mirrors
// Lucene99FlatVectorsReader.getByteVectorValues(String).
func (r *Lucene99FlatVectorsReader) GetByteVectorValues(field string) (index.ByteVectorValues, error) {
	values, err := r.byteVectorValues(field)
	if err != nil {
		return nil, err
	}
	return values, nil
}

// newFlatSparseFloatVectorValues builds the sparse float32 view, opening the
// IndexedDISI doc-id set and the DirectMonotonicReader ord->doc mapping from
// the .vec file. Mirrors OffHeapFloatVectorValues.SparseOffHeapVectorValues.
func (r *Lucene99FlatVectorsReader) newFlatSparseFloatVectorValues(
	entry *lucene99FlatFieldEntry, slice store.IndexInput,
) (*flatSparseFloatVectorValues, error) {
	ordToDoc, disiFactory, err := r.sparseOrdToDoc(entry)
	if err != nil {
		return nil, err
	}
	disi, err := disiFactory()
	if err != nil {
		return nil, err
	}
	return &flatSparseFloatVectorValues{
		reader:    r,
		entry:     entry,
		dimension: entry.dimension,
		size:      entry.size,
		byteSize:  entry.dimension * floatBytes,
		slice:     slice,
		sim:       entry.similarityFunction,
		ordToDoc:  ordToDoc,
		disi:      disi,
		lastOrd:   -1,
		value:     make([]float32, entry.dimension),
	}, nil
}

// newFlatSparseByteVectorValues is the byte analogue of
// [newFlatSparseFloatVectorValues].
func (r *Lucene99FlatVectorsReader) newFlatSparseByteVectorValues(
	entry *lucene99FlatFieldEntry, slice store.IndexInput,
) (*flatSparseByteVectorValues, error) {
	ordToDoc, disiFactory, err := r.sparseOrdToDoc(entry)
	if err != nil {
		return nil, err
	}
	disi, err := disiFactory()
	if err != nil {
		return nil, err
	}
	return &flatSparseByteVectorValues{
		reader:    r,
		entry:     entry,
		dimension: entry.dimension,
		size:      entry.size,
		byteSize:  entry.dimension,
		slice:     slice,
		sim:       entry.similarityFunction,
		ordToDoc:  ordToDoc,
		disi:      disi,
		lastOrd:   -1,
		value:     make([]byte, entry.dimension),
	}, nil
}

// sparseOrdToDoc builds the shared sparse state used by both the float and
// byte sparse views: the DirectMonotonicReader ord->doc mapping and a factory
// that produces fresh IndexedDISI doc-id iterators over the .vec file. The
// IndexedDISI reader is the package-local, little-endian dvIndexedDISI (codecs
// cannot import codecs/lucene90 — import cycle).
func (r *Lucene99FlatVectorsReader) sparseOrdToDoc(
	entry *lucene99FlatFieldEntry,
) (*packed.DirectMonotonicReader, func() (*dvIndexedDISI, error), error) {
	addrSlice, err := dvSliceRandomAccess(r.vectorData, entry.addressesOffset, entry.addressesLength)
	if err != nil {
		return nil, nil, fmt.Errorf("lucene99 flat: slice sparse ord-to-doc addresses: %w", err)
	}
	ordToDoc, err := packed.NewDirectMonotonicReader(entry.ordToDocMeta, addrSlice)
	if err != nil {
		return nil, nil, fmt.Errorf("lucene99 flat: sparse ord-to-doc reader: %w", err)
	}
	disiFactory := func() (*dvIndexedDISI, error) {
		return newDVIndexedDISI(
			r.vectorData, entry.docsWithFieldOffset, entry.docsWithFieldLength,
			entry.jumpTableEntryCount, entry.denseRankPower, int64(entry.size),
		)
	}
	return ordToDoc, disiFactory, nil
}

// randomVectorScorerFloat returns a [utilhnsw.RandomVectorScorer] that
// scores the float32 target against every stored vector for field. Mirrors
// Java's getRandomVectorScorer(String, float[]).
func (r *Lucene99FlatVectorsReader) randomVectorScorerFloat(field string, target []float32) (utilhnsw.RandomVectorScorer, error) {
	values, err := r.floatVectorValues(field)
	if err != nil {
		return nil, err
	}
	if len(target) != values.Dimension() {
		return nil, fmt.Errorf("lucene99 flat: query dim %d != field dim %d",
			len(target), values.Dimension())
	}
	return newFlatFloatQueryScorer(values, target), nil
}

// randomVectorScorerByte returns a [utilhnsw.RandomVectorScorer] that
// scores the byte target against every stored vector for field. Mirrors
// Java's getRandomVectorScorer(String, byte[]).
func (r *Lucene99FlatVectorsReader) randomVectorScorerByte(field string, target []byte) (utilhnsw.RandomVectorScorer, error) {
	values, err := r.byteVectorValues(field)
	if err != nil {
		return nil, err
	}
	if len(target) != values.Dimension() {
		return nil, fmt.Errorf("lucene99 flat: query dim %d != field dim %d",
			len(target), values.Dimension())
	}
	return newFlatByteQueryScorer(values, target), nil
}

// CheckIntegrity verifies the checksum of the `.vec` file.
// GetMergeInstance returns the receiver.
//
// Mirrors Lucene99FlatVectorsReader.getMergeInstance() of Apache Lucene 10.5.0,
// whose body updates the .vec read advice to SEQUENTIAL and then returns this.
// Gocene's store.IndexInput carries no IO-context hint, so only the "return
// this" half has an observable counterpart here.
func (r *Lucene99FlatVectorsReader) GetMergeInstance() (KnnVectorsReader, error) {
	return r, nil
}

// FinishMerge reverts the merge-time state.
//
// Mirrors Lucene99FlatVectorsReader.finishMerge() of Apache Lucene 10.5.0,
// whose body restores the .vec read advice that GetMergeInstance changed.
// Gocene's store.IndexInput carries no IO-context hint, so there is nothing to
// restore.
func (r *Lucene99FlatVectorsReader) FinishMerge() error { return nil }

// GetOffHeapByteSize reports the .vec bytes this reader would like off-heap for
// the given field.
//
// Mirrors Lucene99FlatVectorsReader.getOffHeapByteSize(FieldInfo) of Apache
// Lucene 10.5.0: Map.of(VECTOR_DATA_EXTENSION, entry.vectorDataLength()).
func (r *Lucene99FlatVectorsReader) GetOffHeapByteSize(fieldInfo *index.FieldInfo) map[string]int64 {
	if fieldInfo == nil {
		return map[string]int64{}
	}
	entry, ok := r.fields[fieldInfo.Number()]
	if !ok {
		return map[string]int64{}
	}
	return map[string]int64{lucene99FlatDataExtension: entry.vectorDataLength}
}

func (r *Lucene99FlatVectorsReader) CheckIntegrity() error {
	if r.closed {
		return errors.New("lucene99 flat: reader closed")
	}
	_, err := ChecksumEntireFile(r.vectorData)
	return err
}

// Close releases the `.vec` file handle. Close is idempotent.
func (r *Lucene99FlatVectorsReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	if r.vectorData != nil {
		return r.vectorData.Close()
	}
	return nil
}

// ---------------------------------------------------------------------------
// flatDenseFloatVectorValues — dense off-heap float32 vectors (ord == doc).
//
// Implements utilhnsw.KnnVectorValues (Dimension/Size/OrdToDoc/
// GetAcceptOrds/Iterator) plus VectorValue(ord) for the scorer. Because
// the dense case maps ord == doc, the index-side docID-keyed views are
// trivially derivable.
// ---------------------------------------------------------------------------

type flatDenseFloatVectorValues struct {
	dimension int
	size      int
	byteSize  int
	slice     store.IndexInput // nil for the empty case
	sim       index.VectorSimilarityFunction

	lastOrd int
	value   []float32
}

func newFlatDenseFloatVectorValues(
	dimension, size int, slice store.IndexInput, sim index.VectorSimilarityFunction,
) *flatDenseFloatVectorValues {
	return &flatDenseFloatVectorValues{
		dimension: dimension,
		size:      size,
		byteSize:  dimension * floatBytes,
		slice:     slice,
		sim:       sim,
		lastOrd:   -1,
		value:     make([]float32, dimension),
	}
}

func (v *flatDenseFloatVectorValues) Dimension() int       { return v.dimension }
func (v *flatDenseFloatVectorValues) Size() int            { return v.size }
func (v *flatDenseFloatVectorValues) OrdToDoc(ord int) int { return ord }
func (v *flatDenseFloatVectorValues) similarity() index.VectorSimilarityFunction {
	return v.sim
}
func (v *flatDenseFloatVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	// Dense: the ordinal space is the doc space, so the accept bits map
	// through unchanged. Mirrors DenseOffHeapVectorValues.getAcceptOrds; the
	// empty view mirrors EmptyOffHeapVectorValues.getAcceptOrds, which
	// returns null.
	if v.slice == nil {
		return nil
	}
	return acceptDocs
}

// VectorValue returns the float32 vector at ordinal ord. The returned
// slice is the receiver's reusable buffer; callers must copy to retain it
// past the next call. Mirrors OffHeapFloatVectorValues.vectorValue.
func (v *flatDenseFloatVectorValues) VectorValue(ord int) ([]float32, error) {
	if ord < 0 || ord >= v.size {
		return nil, fmt.Errorf("lucene99 flat: float ordinal %d out of range [0,%d)", ord, v.size)
	}
	if v.lastOrd == ord {
		return v.value, nil
	}
	if err := v.slice.SetPosition(int64(ord) * int64(v.byteSize)); err != nil {
		return nil, err
	}
	if err := readFloatsLE(v.slice, v.value); err != nil {
		return nil, err
	}
	v.lastOrd = ord
	return v.value, nil
}

// Iterator returns a dense iterator. Mirrors DenseOffHeapVectorValues.iterator()
// and EmptyOffHeapVectorValues.iterator(), which both return
// createDenseIterator().
func (v *flatDenseFloatVectorValues) Iterator() index.DocIndexIterator {
	return spi.CreateDenseIterator(v)
}

// Copy returns a view over a clone of the slice. Mirrors
// DenseOffHeapVectorValues.copy(); for the empty view (nil slice) it mirrors
// EmptyOffHeapVectorValues.copy(), which throws UnsupportedOperationException.
func (v *flatDenseFloatVectorValues) Copy() (index.KnnVectorValues, error) {
	return v.CopyFloatVectorValues()
}

// CopyFloatVectorValues is the covariant copy(); see [flatDenseFloatVectorValues.Copy].
func (v *flatDenseFloatVectorValues) CopyFloatVectorValues() (index.FloatVectorValues, error) {
	if v.slice == nil {
		return nil, errors.New("lucene99 flat: copy of empty vector values is not supported")
	}
	return newFlatDenseFloatVectorValues(v.dimension, v.size, v.slice.Clone(), v.sim), nil
}

// Prefetch mirrors OffHeapFloatVectorValues.prefetch(int[], int).
func (v *flatDenseFloatVectorValues) Prefetch(ordsToPrefetch []int, numOrds int) error {
	return flatPrefetch(v.slice, v.byteSize, ordsToPrefetch, numOrds)
}

// GetEncoding carries the FloatVectorValues.getEncoding override.
func (v *flatDenseFloatVectorValues) GetEncoding() index.VectorEncoding {
	return index.VectorEncodingFloat32
}

// GetVectorByteLength carries the KnnVectorValues.getVectorByteLength default.
func (v *flatDenseFloatVectorValues) GetVectorByteLength() int {
	return v.Dimension() * index.VectorEncodingByteSize(v.GetEncoding())
}

// Scorer mirrors DenseOffHeapVectorValues.scorer(query): it scores a copy of
// these values against query, reading the score of the ordinal equal to the
// iterator's current document. The empty view mirrors
// EmptyOffHeapVectorValues.scorer, which returns null.
func (v *flatDenseFloatVectorValues) Scorer(query []float32) (util.VectorScorer, error) {
	if v.slice == nil {
		return nil, nil
	}
	values, err := v.CopyFloatVectorValues()
	if err != nil {
		return nil, err
	}
	cp := values.(*flatDenseFloatVectorValues)
	if len(query) != cp.Dimension() {
		return nil, fmt.Errorf("lucene99 flat: query dim %d != field dim %d", len(query), cp.Dimension())
	}
	iterator := cp.Iterator()
	return &flatDenseVectorScorer{scorer: newFlatFloatQueryScorer(cp, query), iterator: iterator}, nil
}

// Rescorer carries the FloatVectorValues.rescorer default.
func (v *flatDenseFloatVectorValues) Rescorer(target []float32) (util.VectorScorer, error) {
	return v.Scorer(target)
}

// ---------------------------------------------------------------------------
// flatDenseByteVectorValues — dense off-heap byte vectors (ord == doc).
// ---------------------------------------------------------------------------

type flatDenseByteVectorValues struct {
	dimension int
	size      int
	byteSize  int
	slice     store.IndexInput // nil for the empty case
	sim       index.VectorSimilarityFunction

	lastOrd int
	value   []byte
}

func newFlatDenseByteVectorValues(
	dimension, size int, slice store.IndexInput, sim index.VectorSimilarityFunction,
) *flatDenseByteVectorValues {
	return &flatDenseByteVectorValues{
		dimension: dimension,
		size:      size,
		byteSize:  dimension, // 1 byte per sample
		slice:     slice,
		sim:       sim,
		lastOrd:   -1,
		value:     make([]byte, dimension),
	}
}

func (v *flatDenseByteVectorValues) Dimension() int       { return v.dimension }
func (v *flatDenseByteVectorValues) Size() int            { return v.size }
func (v *flatDenseByteVectorValues) OrdToDoc(ord int) int { return ord }
func (v *flatDenseByteVectorValues) similarity() index.VectorSimilarityFunction {
	return v.sim
}
func (v *flatDenseByteVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	if v.slice == nil {
		return nil
	}
	return acceptDocs
}

// VectorValue returns the byte vector at ordinal ord. The returned slice
// is the receiver's reusable buffer.
func (v *flatDenseByteVectorValues) VectorValue(ord int) ([]byte, error) {
	if ord < 0 || ord >= v.size {
		return nil, fmt.Errorf("lucene99 flat: byte ordinal %d out of range [0,%d)", ord, v.size)
	}
	if v.lastOrd == ord {
		return v.value, nil
	}
	if err := v.slice.SetPosition(int64(ord) * int64(v.byteSize)); err != nil {
		return nil, err
	}
	if err := v.slice.ReadBytes(v.value, 0, len(v.value)); err != nil {
		return nil, err
	}
	v.lastOrd = ord
	return v.value, nil
}

// Iterator returns a dense iterator. Mirrors DenseOffHeapVectorValues.iterator()
// and EmptyOffHeapVectorValues.iterator(), which both return
// createDenseIterator().
func (v *flatDenseByteVectorValues) Iterator() index.DocIndexIterator {
	return spi.CreateDenseIterator(v)
}

// Copy returns a view over a clone of the slice. Mirrors
// DenseOffHeapVectorValues.copy(); for the empty view (nil slice) it mirrors
// EmptyOffHeapVectorValues.copy(), which throws UnsupportedOperationException.
func (v *flatDenseByteVectorValues) Copy() (index.KnnVectorValues, error) {
	return v.CopyByteVectorValues()
}

// CopyByteVectorValues is the covariant copy(); see [flatDenseByteVectorValues.Copy].
func (v *flatDenseByteVectorValues) CopyByteVectorValues() (index.ByteVectorValues, error) {
	if v.slice == nil {
		return nil, errors.New("lucene99 flat: copy of empty vector values is not supported")
	}
	return newFlatDenseByteVectorValues(v.dimension, v.size, v.slice.Clone(), v.sim), nil
}

// Prefetch mirrors OffHeapByteVectorValues.prefetch(int[], int).
func (v *flatDenseByteVectorValues) Prefetch(ordsToPrefetch []int, numOrds int) error {
	return flatPrefetch(v.slice, v.byteSize, ordsToPrefetch, numOrds)
}

// GetEncoding carries the ByteVectorValues.getEncoding override.
func (v *flatDenseByteVectorValues) GetEncoding() index.VectorEncoding {
	return index.VectorEncodingByte
}

// GetVectorByteLength carries the KnnVectorValues.getVectorByteLength default.
func (v *flatDenseByteVectorValues) GetVectorByteLength() int {
	return v.Dimension() * index.VectorEncodingByteSize(v.GetEncoding())
}

// Scorer mirrors DenseOffHeapVectorValues.scorer(query): it scores a copy of
// these values against query, reading the score of the ordinal equal to the
// iterator's current document. The empty view mirrors
// EmptyOffHeapVectorValues.scorer, which returns null.
func (v *flatDenseByteVectorValues) Scorer(query []byte) (util.VectorScorer, error) {
	if v.slice == nil {
		return nil, nil
	}
	values, err := v.CopyByteVectorValues()
	if err != nil {
		return nil, err
	}
	cp := values.(*flatDenseByteVectorValues)
	if len(query) != cp.Dimension() {
		return nil, fmt.Errorf("lucene99 flat: query dim %d != field dim %d", len(query), cp.Dimension())
	}
	iterator := cp.Iterator()
	return &flatDenseVectorScorer{scorer: newFlatByteQueryScorer(cp, query), iterator: iterator}, nil
}

// Rescorer carries the ByteVectorValues.rescorer default.
func (v *flatDenseByteVectorValues) Rescorer(target []byte) (util.VectorScorer, error) {
	return v.Scorer(target)
}

// ---------------------------------------------------------------------------
// VectorScorers returned by the off-heap views' Scorer methods, and the shared
// prefetch body.
// ---------------------------------------------------------------------------

// flatDenseVectorScorer is the anonymous VectorScorer returned by
// DenseOffHeapVectorValues.scorer(query) in OffHeapFloatVectorValues and
// OffHeapByteVectorValues: the current document is the ordinal to score.
type flatDenseVectorScorer struct {
	scorer   utilhnsw.RandomVectorScorer
	iterator index.DocIndexIterator
}

// Score scores the iterator's current document.
func (s *flatDenseVectorScorer) Score() (float32, error) {
	return s.scorer.Score(s.iterator.DocID())
}

// Iterator returns the iterator over the scored copy.
func (s *flatDenseVectorScorer) Iterator() util.DocIdSetIterator { return s.iterator }

// flatSparseVectorScorer is the anonymous VectorScorer returned by
// SparseOffHeapVectorValues.scorer(query): the iterator's current index is the
// ordinal to score.
type flatSparseVectorScorer struct {
	scorer   utilhnsw.RandomVectorScorer
	iterator index.DocIndexIterator
}

// Score scores the iterator's current ordinal.
func (s *flatSparseVectorScorer) Score() (float32, error) {
	return s.scorer.Score(s.iterator.Index())
}

// Iterator returns the iterator over the scored copy.
func (s *flatSparseVectorScorer) Iterator() util.DocIdSetIterator { return s.iterator }

// flatPrefetch is the body of OffHeapFloatVectorValues.prefetch and
// OffHeapByteVectorValues.prefetch: when more than one ordinal is requested,
// the byte range of each is prefetched from the slice. IndexInput.prefetch
// defaults to a no-op in Apache Lucene 10.5.0, so a slice without the
// prefetch capability does nothing.
func flatPrefetch(slice store.IndexInput, byteSize int, ordsToPrefetch []int, numOrds int) error {
	if ordsToPrefetch == nil {
		return nil
	}
	finalNumOrds := min(numOrds, len(ordsToPrefetch))
	if finalNumOrds <= 1 {
		return nil
	}
	p, ok := slice.(store.PrefetchableRandomAccessInput)
	if !ok {
		return nil
	}
	for i := 0; i < finalNumOrds; i++ {
		offset := int64(ordsToPrefetch[i]) * int64(byteSize)
		if err := p.Prefetch(offset, int64(byteSize)); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Query-vs-node RandomVectorScorers over the dense off-heap values.
//
// These mirror DefaultFlatVectorScorer's FloatVectorScorer / ByteVectorScorer
// (a fixed query vector scored against every node ordinal). The similarity
// arithmetic is shared with the index-time helpers in
// lucene99_hnsw_mem_scorer.go to keep one canonical implementation in the
// codecs package (codecs cannot import codecs/hnsw — import cycle).
// ---------------------------------------------------------------------------

// flatFloatQueryScorer scores a fixed float32 query against the float
// vectors (dense or sparse).
type flatFloatQueryScorer struct {
	values flatFloatVectorValues
	query  []float32
}

func newFlatFloatQueryScorer(values flatFloatVectorValues, query []float32) *flatFloatQueryScorer {
	cp := make([]float32, len(query))
	copy(cp, query)
	return &flatFloatQueryScorer{values: values, query: cp}
}

func (s *flatFloatQueryScorer) Score(node int) (float32, error) {
	v, err := s.values.VectorValue(node)
	if err != nil {
		return 0, err
	}
	return memComputeFloatSimilarity(s.values.similarity(), s.query, v), nil
}

func (s *flatFloatQueryScorer) BulkScore(nodes []int, scores []float32, numNodes int) (float32, error) {
	return utilhnsw.BulkScoreDefault(s, nodes, scores, numNodes)
}

func (s *flatFloatQueryScorer) MaxOrd() int          { return s.values.Size() }
func (s *flatFloatQueryScorer) OrdToDoc(ord int) int { return s.values.OrdToDoc(ord) }
func (s *flatFloatQueryScorer) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return s.values.GetAcceptOrds(acceptDocs)
}

// flatByteQueryScorer scores a fixed byte query against the byte vectors
// (dense or sparse).
type flatByteQueryScorer struct {
	values flatByteVectorValues
	query  []byte
}

func newFlatByteQueryScorer(values flatByteVectorValues, query []byte) *flatByteQueryScorer {
	cp := make([]byte, len(query))
	copy(cp, query)
	return &flatByteQueryScorer{values: values, query: cp}
}

func (s *flatByteQueryScorer) Score(node int) (float32, error) {
	v, err := s.values.VectorValue(node)
	if err != nil {
		return 0, err
	}
	return memComputeByteSimilarity(s.values.similarity(), s.query, v), nil
}

func (s *flatByteQueryScorer) BulkScore(nodes []int, scores []float32, numNodes int) (float32, error) {
	return utilhnsw.BulkScoreDefault(s, nodes, scores, numNodes)
}

func (s *flatByteQueryScorer) MaxOrd() int          { return s.values.Size() }
func (s *flatByteQueryScorer) OrdToDoc(ord int) int { return s.values.OrdToDoc(ord) }
func (s *flatByteQueryScorer) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return s.values.GetAcceptOrds(acceptDocs)
}

// Compile-time guards.
var (
	_ flatFloatVectorValues       = (*flatDenseFloatVectorValues)(nil)
	_ flatByteVectorValues        = (*flatDenseByteVectorValues)(nil)
	_ util.VectorScorer           = (*flatDenseVectorScorer)(nil)
	_ util.VectorScorer           = (*flatSparseVectorScorer)(nil)
	_ utilhnsw.RandomVectorScorer = (*flatFloatQueryScorer)(nil)
	_ utilhnsw.RandomVectorScorer = (*flatByteQueryScorer)(nil)
)
