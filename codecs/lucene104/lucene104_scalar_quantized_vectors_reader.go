// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Source: lucene/core/src/java/org/apache/lucene/codecs/lucene104/
//
//	Lucene104ScalarQuantizedVectorsReader.java (Lucene 10.4.0)
//
// This is the Go port of Lucene's Lucene104ScalarQuantizedVectorsReader.
// It validates the .veq / .vemq CodecUtil framing and parses the per-field
// metadata, byte-for-byte compatible with Apache Lucene 10.4.0.
package lucene104

import (
	"errors"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/hnsw"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	utilhnsw "github.com/FlavioCFOliveira/Gocene/util/hnsw"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
	"github.com/FlavioCFOliveira/Gocene/util/quantization"
)

// Lucene104ScalarQuantizedFieldEntry holds the parsed .vemq per-field metadata.
// It mirrors the Java FieldEntry record, exposing the fields a reader needs to
// reconstruct the on-disk vectors. Exported so the round-trip test and the
// future search-integration reader (rmp #134) can build an
// [OffHeapScalarQuantizedFloatVectorValues] view via Load.
type Lucene104ScalarQuantizedFieldEntry struct {
	// VectorEncoding is the field's vector encoding (FLOAT32 / BYTE).
	VectorEncoding index.VectorEncoding
	// SimilarityFunction is the field's vector similarity function.
	SimilarityFunction index.VectorSimilarityFunction
	// Dimension is the raw vector dimension.
	Dimension int
	// VectorDataOffset is the .veq offset of the field's quantized vectors.
	VectorDataOffset int64
	// VectorDataLength is the .veq byte length of the field's quantized vectors.
	VectorDataLength int64
	// Size is the number of vectors stored for the field.
	Size int
	// Encoding is the scalar encoding used for the field (present when Size>0).
	Encoding quantization.ScalarEncoding
	// Centroid is the per-field centroid (present when Size>0).
	Centroid []float32
	// CentroidDP is the centroid square magnitude (present when Size>0).
	CentroidDP float32

	// DocsWithFieldOffset distinguishes empty(-2) / dense(-1) / sparse(>=0).
	DocsWithFieldOffset int64
	// The remaining fields carry the sparse OrdToDoc state.
	DocsWithFieldLength int64
	JumpTableEntryCount int
	DenseRankPower      byte
	AddressesOffset     int64
	AddressesLength     int64
	// OrdToDocMeta is the DirectMonotonic meta header for the sparse ord->doc
	// mapping (nil for dense/empty).
	OrdToDocMeta *packed.DirectMonotonicMeta
}

// Lucene104ScalarQuantizedVectorsReader validates the .veq / .vemq CodecUtil
// framing and parses the per-field metadata. It is a deliberately partial
// reader: it proves the writer's framing is sound (CheckIntegrity validates the
// .veq checksum) and exposes the parsed field entries for the round-trip read
// path, but the full search-integration surface (getFloatVectorValues, search)
// is tracked by rmp #134. The value-access methods are intentionally absent
// rather than stubbed with fabricated data.
type Lucene104ScalarQuantizedVectorsReader struct {
	*hnsw.BaseFlatVectorsReader
	encoding   quantization.ScalarEncoding
	fieldInfos *index.FieldInfos
	fields     map[int]*Lucene104ScalarQuantizedFieldEntry
	vectorData store.IndexInput
	closed     bool
}

// NewLucene104ScalarQuantizedVectorsReader opens the .veq data file, validates
// both files' CodecUtil index headers (and the .veq footer checksum), and
// parses the .vemq field records.
func NewLucene104ScalarQuantizedVectorsReader(state *codecs.SegmentReadState, encoding quantization.ScalarEncoding) (*Lucene104ScalarQuantizedVectorsReader, error) {
	if state == nil || state.SegmentInfo == nil || state.Directory == nil {
		return nil, errors.New("lucene104 sq: invalid SegmentReadState")
	}
	r := &Lucene104ScalarQuantizedVectorsReader{
		BaseFlatVectorsReader: &hnsw.BaseFlatVectorsReader{},
		encoding:              encoding,
		fieldInfos:            state.FieldInfos,
		fields:                make(map[int]*Lucene104ScalarQuantizedFieldEntry),
	}

	versionMeta, err := r.readMetadata(state)
	if err != nil {
		return nil, err
	}

	dataName := store.SegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, VectorDataExtension)
	dataIn, err := state.Directory.OpenInput(dataName, store.IOContextRead)
	if err != nil {
		return nil, fmt.Errorf("lucene104 sq: open data %q: %w", dataName, err)
	}
	id := state.SegmentInfo.GetID()
	versionData, err := codecs.CheckIndexHeader(
		dataIn, VectorDataCodecName, VersionStart, VersionCurrent, id, state.SegmentSuffix,
	)
	if err != nil {
		_ = dataIn.Close()
		return nil, fmt.Errorf("lucene104 sq: data header %q: %w", dataName, err)
	}
	if versionData != versionMeta {
		_ = dataIn.Close()
		return nil, fmt.Errorf("lucene104 sq: format versions mismatch: meta=%d, data=%d", versionMeta, versionData)
	}
	if _, err := codecs.RetrieveChecksum(dataIn); err != nil {
		_ = dataIn.Close()
		return nil, fmt.Errorf("lucene104 sq: retrieve data checksum %q: %w", dataName, err)
	}
	r.vectorData = dataIn
	return r, nil
}

// readMetadata reads and validates the .vemq header, parses every field record
// until the -1 sentinel, and checks the footer. Returns the meta version.
func (r *Lucene104ScalarQuantizedVectorsReader) readMetadata(state *codecs.SegmentReadState) (int32, error) {
	metaName := store.SegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, MetaExtension)
	metaRaw, err := state.Directory.OpenInput(metaName, store.IOContextRead)
	if err != nil {
		return 0, fmt.Errorf("lucene104 sq: open meta %q: %w", metaName, err)
	}
	meta := store.NewChecksumIndexInput(metaRaw)

	var versionMeta int32
	var readErr error
	func() {
		id := state.SegmentInfo.GetID()
		v, e := codecs.CheckIndexHeader(
			meta, MetaCodecName, VersionStart, VersionCurrent, id, state.SegmentSuffix,
		)
		if e != nil {
			readErr = e
			return
		}
		versionMeta = v
		readErr = r.readFields(meta)
	}()

	_, footerErr := store.CheckFooter(meta)
	_ = metaRaw.Close()
	if readErr != nil {
		return 0, fmt.Errorf("lucene104 sq: read meta %q: %w", metaName, readErr)
	}
	if footerErr != nil {
		return 0, fmt.Errorf("lucene104 sq: meta footer %q: %w", metaName, footerErr)
	}
	return versionMeta, nil
}

// readFields parses every per-field record until the -1 sentinel.
func (r *Lucene104ScalarQuantizedVectorsReader) readFields(meta store.DataInput) error {
	for {
		fieldNum, err := meta.ReadInt()
		if err != nil {
			return fmt.Errorf("reading field number: %w", err)
		}
		if fieldNum == -1 {
			break
		}
		var info *index.FieldInfo
		if r.fieldInfos != nil {
			info = r.fieldInfos.GetByNumber(int(fieldNum))
			if info == nil {
				return fmt.Errorf("invalid field number %d", fieldNum)
			}
		}
		entry, err := readScalarQuantizedFieldEntry(meta, info)
		if err != nil {
			return fmt.Errorf("field %d: %w", fieldNum, err)
		}
		r.fields[int(fieldNum)] = entry
	}
	return nil
}

// readScalarQuantizedFieldEntry parses one .vemq field record. Mirrors the
// Java FieldEntry.create read order exactly: encoding ordinal, similarity
// ordinal, vint dimension, vlong offset, vlong length, vint size, then (when
// size>0) wire number, centroid floats and centroidDP, then the OrdToDoc
// stored-meta block. The field number is consumed by the caller.
func readScalarQuantizedFieldEntry(meta store.DataInput, info *index.FieldInfo) (*Lucene104ScalarQuantizedFieldEntry, error) {
	// readVectorEncoding / readSimilarityFunction are the statics of
	// org.apache.lucene.codecs.lucene99.Lucene99HnswVectorsReader that the
	// Java readField imports statically.
	enc, err := codecs.ReadVectorEncoding(meta)
	if err != nil {
		return nil, err
	}
	sim, err := codecs.ReadSimilarityFunction(meta)
	if err != nil {
		return nil, err
	}
	if info != nil {
		if sim != info.VectorSimilarityFunction() {
			return nil, fmt.Errorf("Inconsistent vector similarity function for field=%q; %s != %s",
				info.Name(), sim.ID(), info.VectorSimilarityFunction().ID())
		}
		// FieldEntry.create(input, vectorEncoding, info.getVectorSimilarityFunction())
		sim = info.VectorSimilarityFunction()
	}

	dimV, err := store.ReadVInt(meta)
	if err != nil {
		return nil, err
	}
	vectorDataOffset, err := meta.ReadVLong()
	if err != nil {
		return nil, err
	}
	vectorDataLength, err := meta.ReadVLong()
	if err != nil {
		return nil, err
	}
	size, err := store.ReadVInt(meta)
	if err != nil {
		return nil, err
	}

	entry := &Lucene104ScalarQuantizedFieldEntry{
		VectorEncoding:     enc,
		SimilarityFunction: sim,
		Dimension:          int(dimV),
		VectorDataOffset:   vectorDataOffset,
		VectorDataLength:   vectorDataLength,
		Size:               int(size),
		Encoding:           quantization.ScalarEncodingUnsignedByte,
	}

	if size > 0 {
		wireNumber, e := store.ReadVInt(meta)
		if e != nil {
			return nil, e
		}
		scalarEncoding, ok := quantization.ScalarEncodingFromWireNumber(int(wireNumber))
		if !ok {
			return nil, fmt.Errorf("Unknown scalar encoding wire number: %d", wireNumber)
		}
		entry.Encoding = scalarEncoding
		centroid := make([]float32, int(dimV))
		for i := range centroid {
			bits, e := meta.ReadInt()
			if e != nil {
				return nil, e
			}
			centroid[i] = math.Float32frombits(uint32(bits))
		}
		entry.Centroid = centroid
		dpBits, e := meta.ReadInt()
		if e != nil {
			return nil, e
		}
		entry.CentroidDP = math.Float32frombits(uint32(dpBits))
	}

	// OrdToDocDISIReaderConfiguration.fromStoredMeta.
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
	entry.DocsWithFieldOffset = docsWithFieldOffset
	entry.DocsWithFieldLength = docsWithFieldLength
	entry.JumpTableEntryCount = int(jumpTableEntryCount)
	entry.DenseRankPower = byte(denseRankPower)

	if docsWithFieldOffset > -1 {
		addressesOffset, e := meta.ReadLong()
		if e != nil {
			return nil, e
		}
		blockShift, e := store.ReadVInt(meta)
		if e != nil {
			return nil, e
		}
		ordToDocMeta, e := packed.LoadDirectMonotonicMeta(meta, int64(size), int(blockShift))
		if e != nil {
			return nil, fmt.Errorf("load ord-to-doc monotonic meta: %w", e)
		}
		addressesLength, e := meta.ReadLong()
		if e != nil {
			return nil, e
		}
		entry.AddressesOffset = addressesOffset
		entry.AddressesLength = addressesLength
		entry.OrdToDocMeta = ordToDocMeta
	}

	if info != nil {
		if sim != info.VectorSimilarityFunction() {
			return nil, fmt.Errorf("inconsistent similarity for field %q: %v != %v",
				info.Name(), sim, info.VectorSimilarityFunction())
		}
		if int(dimV) != info.VectorDimension() {
			return nil, fmt.Errorf("inconsistent dimension for field %q: %d != %d",
				info.Name(), dimV, info.VectorDimension())
		}
	}
	return entry, nil
}

// FieldEntry returns the parsed metadata for the named field, or an error if
// the field is unknown. Exposed for the round-trip read path.
func (r *Lucene104ScalarQuantizedVectorsReader) FieldEntry(field string) (*Lucene104ScalarQuantizedFieldEntry, error) {
	if r.fieldInfos == nil {
		return nil, errors.New("lucene104 sq: reader has no field infos")
	}
	info := r.fieldInfos.GetByName(field)
	if info == nil {
		return nil, fmt.Errorf("lucene104 sq: field %q not found", field)
	}
	entry, ok := r.fields[info.Number()]
	if !ok {
		return nil, fmt.Errorf("lucene104 sq: field %q has no vector entry", field)
	}
	return entry, nil
}

// VectorData returns the open .veq input. Exposed for the round-trip read path
// so callers can slice the quantized vector data for [Load].
func (r *Lucene104ScalarQuantizedVectorsReader) VectorData() store.IndexInput { return r.vectorData }

// CheckIntegrity verifies the .veq checksum end-to-end.
func (r *Lucene104ScalarQuantizedVectorsReader) CheckIntegrity() error {
	if r.closed {
		return errors.New("lucene104 sq: reader closed")
	}
	_, err := codecs.ChecksumEntireFile(r.vectorData)
	return err
}

// Close releases the .veq file handle. Idempotent.
func (r *Lucene104ScalarQuantizedVectorsReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	if r.vectorData != nil {
		return r.vectorData.Close()
	}
	return nil
}

// GetRandomVectorScorerFloat is a stub until rmp #134.
func (r *Lucene104ScalarQuantizedVectorsReader) GetRandomVectorScorerFloat(field string, target []float32) (utilhnsw.RandomVectorScorer, error) {
	return nil, errors.New("lucene104 sq: GetRandomVectorScorerFloat not implemented (rmp #134)")
}

// GetRandomVectorScorerByte is a stub until rmp #134.
func (r *Lucene104ScalarQuantizedVectorsReader) GetRandomVectorScorerByte(field string, target []byte) (utilhnsw.RandomVectorScorer, error) {
	return nil, errors.New("lucene104 sq: GetRandomVectorScorerByte not implemented (rmp #134)")
}

// Apache Lucene 10.5.0's Lucene104ScalarQuantizedVectorsReader overrides
// neither getMergeInstance() nor finishMerge(): both are inherited defaults
// of KnnVectorsReader (getMergeInstance returns this, finishMerge is empty).
// Gocene therefore declares neither here; the earlier GetMergeInstance had no
// Java counterpart in this class and has been reconciled away.
