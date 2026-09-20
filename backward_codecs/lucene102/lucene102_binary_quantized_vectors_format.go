// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.5.0:
//
//	Licensed to the Apache Software Foundation (ASF) under one or more
//	contributor license agreements. See the NOTICE file distributed with
//	this work for additional information regarding copyright ownership.
//	The ASF licenses this file to You under the Apache License, Version 2.0
//	(the "License"); you may not use this file except in compliance with
//	the License. You may obtain a copy of the License at
//
//	    http://www.apache.org/licenses/LICENSE-2.0

package lucene102

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/hnsw"
)

// Lucene102BinaryQuantizedVectorsFormat wire-level constants. Mirror the
// static definitions of Lucene102BinaryQuantizedVectorsFormat (Apache Lucene
// 10.5.0).
const (
	lucene102QueryBits byte = 4
	lucene102IndexBits byte = 1

	lucene102BinarizedVectorComponent         = "BVEC"
	lucene102BinaryQuantizedVectorsFormatName = "Lucene102BinaryQuantizedVectorsFormat"

	lucene102VersionStart   int32 = 0
	lucene102VersionCurrent       = lucene102VersionStart

	lucene102MetaCodecName             = "Lucene102BinaryQuantizedVectorsFormatMeta"
	lucene102VectorDataCodecName       = "Lucene102BinaryQuantizedVectorsFormatData"
	lucene102MetaExtension             = "vemb"
	lucene102VectorDataExtension       = "veb"
	lucene102DirectMonotonicBlockShift = 16
)

// errOldCodecReadOnly mirrors the UnsupportedOperationException thrown by the
// field writer factories of the backward-compatibility formats.
var errOldCodecReadOnly = errors.New("UnsupportedOperationException: Old codecs may only be used for reading")

// lucene102RawVectorFormat mirrors the protected static final rawVectorFormat:
// the raw (unquantized) vector format used to read the original vectors.
var lucene102RawVectorFormat = codecs.NewLucene99FlatVectorsFormat(hnsw.GetLucene99FlatVectorsScorer())

// lucene102BinaryFlatVectorsScorer mirrors the private static final scorer.
var lucene102BinaryFlatVectorsScorer = NewLucene102BinaryFlatVectorsScorer(hnsw.GetLucene99FlatVectorsScorer())

// Lucene102BinaryQuantizedVectorsFormat is the Go port of
// org.apache.lucene.backward_codecs.lucene102.Lucene102BinaryQuantizedVectorsFormat
// (Apache Lucene 10.5.0). The binary quantization format is a per-vector
// optimized scalar quantization stored in two files:
//
//   - .veb (vector data): for each vector, the binary quantized values (each
//     byte holds 8 bits), the optimized quantiles and an additional
//     similarity dependent corrective factor as floats, and the sum of the
//     quantized components as a short; after the vectors, the sparse vector
//     information keeping track of monotonic blocks.
//   - .vemb (vector metadata): the field number, vector encoding ordinal and
//     vector similarity ordinal as ints, the dimension as a vint, the offset
//     and length of the vector data as vlongs, the number of vectors as a
//     vint, the centroid as floats and the centroid square magnitude as a
//     float, then the sparse vector information mapping ordinals to doc IDs.
type Lucene102BinaryQuantizedVectorsFormat struct {
	*hnsw.BaseFlatVectorsFormat
}

// NewLucene102BinaryQuantizedVectorsFormat creates a new instance. Mirrors
// Lucene102BinaryQuantizedVectorsFormat().
func NewLucene102BinaryQuantizedVectorsFormat() *Lucene102BinaryQuantizedVectorsFormat {
	return &Lucene102BinaryQuantizedVectorsFormat{
		BaseFlatVectorsFormat: hnsw.NewBaseFlatVectorsFormat(lucene102BinaryQuantizedVectorsFormatName),
	}
}

// FlatFieldsWriter mirrors fieldsWriter(SegmentWriteState), which throws
// UnsupportedOperationException: old codecs may only be used for reading.
func (f *Lucene102BinaryQuantizedVectorsFormat) FlatFieldsWriter(_ *codecs.SegmentWriteState) (hnsw.FlatVectorsWriter, error) {
	return nil, errOldCodecReadOnly
}

// FlatFieldsReader mirrors fieldsReader(SegmentReadState):
// new Lucene102BinaryQuantizedVectorsReader(state, rawVectorFormat.fieldsReader(state), scorer).
func (f *Lucene102BinaryQuantizedVectorsFormat) FlatFieldsReader(state *codecs.SegmentReadState) (hnsw.FlatVectorsReader, error) {
	rawVectorsReader, err := lucene102RawVectorFormat.FlatFieldsReader(state)
	if err != nil {
		return nil, err
	}
	reader, err := NewLucene102BinaryQuantizedVectorsReader(state, rawVectorsReader, lucene102BinaryFlatVectorsScorer)
	if err != nil {
		return nil, err
	}
	return reader, nil
}

// FieldsWriter returns the result of FlatFieldsWriter typed as the inherited
// KnnVectorsFormat factory.
func (f *Lucene102BinaryQuantizedVectorsFormat) FieldsWriter(state *codecs.SegmentWriteState) (codecs.KnnVectorsWriter, error) {
	return f.FlatFieldsWriter(state)
}

// FieldsReader returns the result of FlatFieldsReader typed as the inherited
// KnnVectorsFormat factory.
func (f *Lucene102BinaryQuantizedVectorsFormat) FieldsReader(state *codecs.SegmentReadState) (codecs.KnnVectorsReader, error) {
	return f.FlatFieldsReader(state)
}

// GetMaxDimensions mirrors getMaxDimensions(String), which returns 1024.
func (f *Lucene102BinaryQuantizedVectorsFormat) GetMaxDimensions(_ string) int {
	return 1024
}

// String mirrors toString().
func (f *Lucene102BinaryQuantizedVectorsFormat) String() string {
	return fmt.Sprintf("Lucene102BinaryQuantizedVectorsFormat(name=%s, flatVectorScorer=%v, rawVectorFormat=%v)",
		lucene102BinaryQuantizedVectorsFormatName, lucene102BinaryFlatVectorsScorer, lucene102RawVectorFormat)
}

var _ hnsw.FlatVectorsFormat = (*Lucene102BinaryQuantizedVectorsFormat)(nil)
