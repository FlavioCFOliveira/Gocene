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
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	utilhnsw "github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// Lucene99FlatVectorsReader reads raw vector values written by
// [Lucene99FlatVectorsWriter]. It is the Go port of
// org.apache.lucene.codecs.lucene99.Lucene99FlatVectorsReader
// (Lucene 10.4.0), restricted to the dense and empty cases (rmp #4731).
//
// Deviation from the Java reference (rmp #4731):
//
//  1. Only dense (docsWithFieldOffset == -1) and empty
//     (docsWithFieldOffset == -2) fields are supported. A sparse field
//     surfaces [errFlatSparseUnsupported] when its vectors are loaded; the
//     sparse IndexedDISI path is tracked by rmp #4755.
type Lucene99FlatVectorsReader struct {
	fieldInfos *index.FieldInfos
	fields     map[int]*lucene99FlatFieldEntry // keyed by field number
	vectorData store.IndexInput                // open .vec file
	closed     bool
}

// lucene99FlatFieldEntry mirrors the Java FieldEntry record for the
// flat format (dense/empty subset).
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
	//  >=0 : sparse (unsupported — rmp #4755)
	docsWithFieldOffset int64
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

	dataName := index.SegmentFileName(
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
	metaName := index.SegmentFileName(
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

	vectorDataOffset, err := store.ReadVLong(meta)
	if err != nil {
		return nil, err
	}
	vectorDataLength, err := store.ReadVLong(meta)
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

	// OrdToDocDISIReaderConfiguration.fromStoredMeta: read the sentinel
	// block. docsWithFieldOffset distinguishes empty(-2)/dense(-1)/sparse.
	docsWithFieldOffset, err := meta.ReadLong()
	if err != nil {
		return nil, err
	}
	if _, err := meta.ReadLong(); err != nil { // docsWithFieldLength
		return nil, err
	}
	if _, err := meta.ReadShort(); err != nil { // jumpTableEntryCount
		return nil, err
	}
	if _, err := meta.ReadByte(); err != nil { // denseRankPower
		return nil, err
	}
	if docsWithFieldOffset > -1 {
		// Sparse: the meta carries an additional DirectMonotonicWriter
		// header (addressesOffset, blockShift, meta, addressesLength). We
		// cannot decode the ordToDoc mapping without IndexedDISI; surface
		// the deferral cleanly rather than mis-parsing the stream.
		return nil, fmt.Errorf("%w (field %q)", errFlatSparseUnsupported, info.Name())
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

// floatVectorValues loads the dense off-heap float32 vectors for field.
// Returns an error for the sparse case (rmp #4755) and an empty view for
// the empty case.
func (r *Lucene99FlatVectorsReader) floatVectorValues(field string) (*flatDenseFloatVectorValues, error) {
	entry, err := r.getFieldEntry(field, index.VectorEncodingFloat32)
	if err != nil {
		return nil, err
	}
	if entry.docsWithFieldOffset == -2 {
		return newFlatDenseFloatVectorValues(entry.dimension, 0, nil, entry.similarityFunction), nil
	}
	if entry.docsWithFieldOffset != -1 {
		return nil, fmt.Errorf("%w (field %q)", errFlatSparseUnsupported, field)
	}
	slice, err := r.vectorData.Slice("vector-data", entry.vectorDataOffset, entry.vectorDataLength)
	if err != nil {
		return nil, fmt.Errorf("lucene99 flat: slice float vectors for %q: %w", field, err)
	}
	return newFlatDenseFloatVectorValues(entry.dimension, entry.size, slice, entry.similarityFunction), nil
}

// byteVectorValues loads the dense off-heap byte vectors for field.
func (r *Lucene99FlatVectorsReader) byteVectorValues(field string) (*flatDenseByteVectorValues, error) {
	entry, err := r.getFieldEntry(field, index.VectorEncodingByte)
	if err != nil {
		return nil, err
	}
	if entry.docsWithFieldOffset == -2 {
		return newFlatDenseByteVectorValues(entry.dimension, 0, nil, entry.similarityFunction), nil
	}
	if entry.docsWithFieldOffset != -1 {
		return nil, fmt.Errorf("%w (field %q)", errFlatSparseUnsupported, field)
	}
	slice, err := r.vectorData.Slice("vector-data", entry.vectorDataOffset, entry.vectorDataLength)
	if err != nil {
		return nil, fmt.Errorf("lucene99 flat: slice byte vectors for %q: %w", field, err)
	}
	return newFlatDenseByteVectorValues(entry.dimension, entry.size, slice, entry.similarityFunction), nil
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
func (v *flatDenseFloatVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	// Dense: the ordinal space is the doc space, so the accept bits map
	// through unchanged. Mirrors DenseOffHeapVectorValues.getAcceptOrds.
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

// Iterator returns a dense sequential (docID == ordinal) iterator.
func (v *flatDenseFloatVectorValues) Iterator() utilhnsw.DocIndexIterator {
	return &flatDenseIterator{size: v.size, cur: -1}
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
func (v *flatDenseByteVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
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
	if err := v.slice.ReadBytes(v.value); err != nil {
		return nil, err
	}
	v.lastOrd = ord
	return v.value, nil
}

// Iterator returns a dense sequential iterator.
func (v *flatDenseByteVectorValues) Iterator() utilhnsw.DocIndexIterator {
	return &flatDenseIterator{size: v.size, cur: -1}
}

// ---------------------------------------------------------------------------
// flatDenseIterator — DocIndexIterator with identity ordinal -> docID.
// ---------------------------------------------------------------------------

type flatDenseIterator struct {
	size int
	cur  int
}

func (it *flatDenseIterator) NextDoc() (int, error) {
	it.cur++
	if it.cur >= it.size {
		it.cur = it.size
		return util.NO_MORE_DOCS, nil
	}
	return it.cur, nil
}

func (it *flatDenseIterator) Index() int { return it.cur }

// ---------------------------------------------------------------------------
// Query-vs-node RandomVectorScorers over the dense off-heap values.
//
// These mirror DefaultFlatVectorScorer's FloatVectorScorer / ByteVectorScorer
// (a fixed query vector scored against every node ordinal). The similarity
// arithmetic is shared with the index-time helpers in
// lucene99_hnsw_mem_scorer.go to keep one canonical implementation in the
// codecs package (codecs cannot import codecs/hnsw — import cycle).
// ---------------------------------------------------------------------------

// flatFloatQueryScorer scores a fixed float32 query against the dense
// float vectors.
type flatFloatQueryScorer struct {
	values *flatDenseFloatVectorValues
	query  []float32
}

func newFlatFloatQueryScorer(values *flatDenseFloatVectorValues, query []float32) *flatFloatQueryScorer {
	cp := make([]float32, len(query))
	copy(cp, query)
	return &flatFloatQueryScorer{values: values, query: cp}
}

func (s *flatFloatQueryScorer) Score(node int) (float32, error) {
	v, err := s.values.VectorValue(node)
	if err != nil {
		return 0, err
	}
	return memComputeFloatSimilarity(s.values.sim, s.query, v), nil
}

func (s *flatFloatQueryScorer) BulkScore(nodes []int, scores []float32, numNodes int) (float32, error) {
	return utilhnsw.BulkScoreDefault(s, nodes, scores, numNodes)
}

func (s *flatFloatQueryScorer) MaxOrd() int          { return s.values.Size() }
func (s *flatFloatQueryScorer) OrdToDoc(ord int) int { return s.values.OrdToDoc(ord) }
func (s *flatFloatQueryScorer) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return s.values.GetAcceptOrds(acceptDocs)
}

// flatByteQueryScorer scores a fixed byte query against the dense byte
// vectors.
type flatByteQueryScorer struct {
	values *flatDenseByteVectorValues
	query  []byte
}

func newFlatByteQueryScorer(values *flatDenseByteVectorValues, query []byte) *flatByteQueryScorer {
	cp := make([]byte, len(query))
	copy(cp, query)
	return &flatByteQueryScorer{values: values, query: cp}
}

func (s *flatByteQueryScorer) Score(node int) (float32, error) {
	v, err := s.values.VectorValue(node)
	if err != nil {
		return 0, err
	}
	return memComputeByteSimilarity(s.values.sim, s.query, v), nil
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
	_ utilhnsw.KnnVectorValues    = (*flatDenseFloatVectorValues)(nil)
	_ utilhnsw.KnnVectorValues    = (*flatDenseByteVectorValues)(nil)
	_ utilhnsw.RandomVectorScorer = (*flatFloatQueryScorer)(nil)
	_ utilhnsw.RandomVectorScorer = (*flatByteQueryScorer)(nil)
)
