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
//         Lucene104ScalarQuantizedVectorsFormat.java (Lucene 10.4.0)
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

// Lucene104ScalarQuantizedVectorsFormat implements per-vector optimized scalar
// quantization for vector storage. It compresses float vectors to quantized
// byte representations for efficient storage and fast approximate similarity
// computation, byte-for-byte compatible with Apache Lucene 10.4.0.
type Lucene104ScalarQuantizedVectorsFormat struct {
	*hnsw.BaseFlatVectorsFormat
	encoding codecs.ScalarEncoding
}

// NewLucene104ScalarQuantizedVectorsFormat creates a new
// Lucene104ScalarQuantizedVectorsFormat with the default encoding
// (UNSIGNED_BYTE). Mirrors the Java no-arg constructor.
func NewLucene104ScalarQuantizedVectorsFormat() *Lucene104ScalarQuantizedVectorsFormat {
	return NewLucene104ScalarQuantizedVectorsFormatWithEncoding(codecs.ScalarEncodingUnsignedByte)
}

// NewLucene104ScalarQuantizedVectorsFormatWithEncoding creates a new format
// with the specified encoding. Mirrors the Java single-argument constructor.
func NewLucene104ScalarQuantizedVectorsFormatWithEncoding(encoding codecs.ScalarEncoding) *Lucene104ScalarQuantizedVectorsFormat {
	return &Lucene104ScalarQuantizedVectorsFormat{
		BaseFlatVectorsFormat: hnsw.NewBaseFlatVectorsFormat(Name),
		encoding:              encoding,
	}
}

// Encoding returns the scalar encoding used by this format.
func (f *Lucene104ScalarQuantizedVectorsFormat) Encoding() codecs.ScalarEncoding {
	return f.encoding
}

// FlatFieldsWriter returns the byte-faithful writer for quantized vectors.
// Mirrors Java's fieldsWriter(SegmentWriteState).
func (f *Lucene104ScalarQuantizedVectorsFormat) FlatFieldsWriter(state *codecs.SegmentWriteState) (hnsw.FlatVectorsWriter, error) {
	return NewLucene104ScalarQuantizedVectorsWriter(state, f.encoding)
}

// FlatFieldsReader returns a reader that validates the CodecUtil framing and
// parses the per-field metadata. Mirrors Java's fieldsReader(SegmentReadState).
func (f *Lucene104ScalarQuantizedVectorsFormat) FlatFieldsReader(state *codecs.SegmentReadState) (hnsw.FlatVectorsReader, error) {
	return codecs.NewLucene104ScalarQuantizedVectorsReader(state, f.encoding)
}

// GetMaxDimensions returns the largest vector dimensionality this
// format supports for the given field name. The Java base returns 1024
// unconditionally.
func (f *Lucene104ScalarQuantizedVectorsFormat) GetMaxDimensions(_ string) int {
	return 1024
}

// String returns a string representation of this format.
func (f *Lucene104ScalarQuantizedVectorsFormat) String() string {
	return fmt.Sprintf("Lucene104ScalarQuantizedVectorsFormat(name=%s, encoding=%s)",
		Name, f.encoding.String())
}
