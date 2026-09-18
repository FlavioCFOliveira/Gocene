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
//	Lucene104ScalarQuantizedVectorsFormat.java (Lucene 10.4.0)
//
// This is the Go port of Lucene's Lucene104ScalarQuantizedVectorsFormat.
// It implements a scalar-quantized vector storage format that compresses
// float vectors into quantized byte representations, byte-for-byte
// compatible with Apache Lucene 10.4.0.
package lucene104

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/hnsw"
	"github.com/FlavioCFOliveira/Gocene/util/quantization"
)

const (
	// QuantizedVectorComponent is the info-stream component name.
	QuantizedVectorComponent = "QVEC"
	// Name is the canonical name of this format.
	Name = "Lucene104ScalarQuantizedVectorsFormat"

	// VersionStart is the initial format version.
	VersionStart = 0
	// VersionCurrent is the current format version.
	VersionCurrent = VersionStart
	// MetaCodecName is the name of the metadata codec.
	MetaCodecName = "Lucene104ScalarQuantizedVectorsFormatMeta"
	// VectorDataCodecName is the name of the vector data codec.
	VectorDataCodecName = "Lucene104ScalarQuantizedVectorsFormatData"
	// MetaExtension is the extension for metadata files.
	MetaExtension = "vemq"
	// VectorDataExtension is the extension for vector data files.
	VectorDataExtension = "veq"
	// DirectMonotonicBlockShift is the block shift used by the DirectMonotonicWriter.
	DirectMonotonicBlockShift = 16
)

// rawVectorFormat mirrors the Java private static final field
//
//	new Lucene99FlatVectorsFormat(FlatVectorScorerUtil.getLucene99FlatVectorsScorer())
var rawVectorFormat = codecs.NewLucene99FlatVectorsFormat(hnsw.GetLucene99FlatVectorsScorer())

// scorer mirrors the Java private static final field
//
//	new Lucene104ScalarQuantizedVectorScorer(FlatVectorScorerUtil.getLucene99FlatVectorsScorer())
var scorer = NewLucene104ScalarQuantizedVectorScorer(hnsw.GetLucene99FlatVectorsScorer())

// Lucene104ScalarQuantizedVectorsFormat implements per-vector optimized scalar
// quantization for vector storage. It compresses float vectors to quantized
// byte representations for efficient storage and fast approximate similarity
// computation, byte-for-byte compatible with Apache Lucene 10.4.0.
type Lucene104ScalarQuantizedVectorsFormat struct {
	*hnsw.BaseFlatVectorsFormat
	encoding quantization.ScalarEncoding
}

// NewLucene104ScalarQuantizedVectorsFormat creates a new
// Lucene104ScalarQuantizedVectorsFormat with the default encoding
// (UNSIGNED_BYTE). Mirrors the Java no-arg constructor.
func NewLucene104ScalarQuantizedVectorsFormat() *Lucene104ScalarQuantizedVectorsFormat {
	return NewLucene104ScalarQuantizedVectorsFormatWithEncoding(quantization.ScalarEncodingUnsignedByte)
}

// NewLucene104ScalarQuantizedVectorsFormatWithEncoding creates a new format
// with the specified encoding. Mirrors the Java single-argument constructor.
func NewLucene104ScalarQuantizedVectorsFormatWithEncoding(encoding quantization.ScalarEncoding) *Lucene104ScalarQuantizedVectorsFormat {
	return &Lucene104ScalarQuantizedVectorsFormat{
		BaseFlatVectorsFormat: hnsw.NewBaseFlatVectorsFormat(Name),
		encoding:              encoding,
	}
}

// Encoding returns the scalar encoding used by this format.
func (f *Lucene104ScalarQuantizedVectorsFormat) Encoding() quantization.ScalarEncoding {
	return f.encoding
}

// FlatFieldsWriter returns the byte-faithful writer for quantized vectors.
// Mirrors Java's fieldsWriter(SegmentWriteState).
func (f *Lucene104ScalarQuantizedVectorsFormat) FlatFieldsWriter(state *codecs.SegmentWriteState) (hnsw.FlatVectorsWriter, error) {
	rawVectorDelegate, err := rawVectorFormat.FlatFieldsWriter(state)
	if err != nil {
		return nil, err
	}
	return NewLucene104ScalarQuantizedVectorsWriter(state, f.encoding, rawVectorDelegate, scorer)
}

// FlatFieldsReader mirrors Java's fieldsReader(SegmentReadState), whose body
// is
//
//	new Lucene104ScalarQuantizedVectorsReader(
//	    state, rawVectorFormat.fieldsReader(state), scorer)
//
// This does not compile today, and deliberately so: the in-package
// [Lucene104ScalarQuantizedVectorsReader] is an incomplete port of
// org.apache.lucene.codecs.lucene104.Lucene104ScalarQuantizedVectorsReader —
// it reads and validates the .vemq/.veq framing and metadata but implements
// none of the value-access surface (getFloatVectorValues, getByteVectorValues,
// search, getFlatVectorScorer, ramBytesUsed, getOffHeapByteSize), so it is not
// a FlatVectorsReader. Completing it requires
// org.apache.lucene.codecs.lucene104.OffHeapScalarQuantizedVectorValues, which
// is not ported. Per CLAUDE.md § 2.1 and § 2.2 the resulting compile error is
// left standing rather than suppressed or satisfied with stubbed members.
func (f *Lucene104ScalarQuantizedVectorsFormat) FlatFieldsReader(state *codecs.SegmentReadState) (hnsw.FlatVectorsReader, error) {
	return NewLucene104ScalarQuantizedVectorsReader(state, f.encoding)
}

// GetMaxDimensions returns the largest vector dimensionality this
// format supports for the given field name. The Java base returns 1024
// unconditionally.
func (f *Lucene104ScalarQuantizedVectorsFormat) GetMaxDimensions(_ string) int {
	return 1024
}

// String mirrors toString().
func (f *Lucene104ScalarQuantizedVectorsFormat) String() string {
	return fmt.Sprintf("Lucene104ScalarQuantizedVectorsFormat(name=%s, encoding=%s, flatVectorScorer=%s, rawVectorFormat=%s)",
		Name, f.encoding, scorer, rawVectorFormat)
}
