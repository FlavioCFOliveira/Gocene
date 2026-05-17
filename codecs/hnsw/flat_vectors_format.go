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

package hnsw

import (
	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// FlatVectorsFormatMaxDimensions is the upper bound the abstract base
// returns from [FlatVectorsFormat.GetMaxDimensions]. Mirrors the constant
// 1024 hard-coded in
// org.apache.lucene.codecs.hnsw.FlatVectorsFormat#getMaxDimensions
// (Lucene 10.4.0).
const FlatVectorsFormatMaxDimensions = 1024

// FlatVectorsFormat is the Go port of
// org.apache.lucene.codecs.hnsw.FlatVectorsFormat (Lucene 10.4.0). It
// encodes/decodes per-document vectors and exposes a scoring interface
// for flat (sequential, graph-less) stored vectors.
//
// The Java reference is an abstract class extending KnnVectorsFormat
// with two abstract methods (fieldsWriter / fieldsReader) and a
// concrete getMaxDimensions returning the constant 1024. In Go the
// class is encoded as an interface plus a struct base:
//   - FlatVectorsFormat (interface) captures the abstract surface:
//     callers receive [FlatVectorsWriter] / [FlatVectorsReader] back
//     from the field factories, narrower than the parent
//     KnnVectorsFormat which returns the wider KnnVectorsWriter /
//     KnnVectorsReader contracts.
//   - [BaseFlatVectorsFormat] embeds the parent codec base and
//     supplies the concrete GetMaxDimensions default. Concrete formats
//     embed *BaseFlatVectorsFormat to inherit Name + GetMaxDimensions
//     and then implement FieldsWriter / FieldsReader themselves.
//
// The interface intentionally also satisfies
// [codecs.KnnVectorsFormat]: an HNSW codec can store a FlatVectorsFormat
// wherever it expects a KnnVectorsFormat and still construct
// FlatVectorsReader/Writer instances through the narrower methods when
// it needs flat-vector-specific operations.
type FlatVectorsFormat interface {
	codecs.KnnVectorsFormat

	// FlatFieldsWriter returns a [FlatVectorsWriter] for the segment
	// described by state. The narrower return type lets HNSW codecs
	// recover the flat-vector-only contract from a FlatVectorsFormat
	// instance.
	FlatFieldsWriter(state *codecs.SegmentWriteState) (FlatVectorsWriter, error)

	// FlatFieldsReader returns a [FlatVectorsReader] for the segment
	// described by state, mirroring FlatFieldsWriter on the read path.
	FlatFieldsReader(state *codecs.SegmentReadState) (FlatVectorsReader, error)

	// GetMaxDimensions returns the largest vector dimensionality this
	// format supports for the given field name. The
	// [BaseFlatVectorsFormat] default returns
	// [FlatVectorsFormatMaxDimensions] regardless of fieldName, matching
	// the Java reference.
	GetMaxDimensions(fieldName string) int
}

// BaseFlatVectorsFormat is the canonical zero-state of a
// [FlatVectorsFormat]. Concrete formats embed *BaseFlatVectorsFormat
// to inherit the Name + GetMaxDimensions defaults expected by the
// abstract Java base, then provide their own FieldsWriter /
// FieldsReader / FlatFieldsWriter / FlatFieldsReader implementations.
//
// BaseFlatVectorsFormat composes [codecs.BaseKnnVectorsFormat] so the
// parent codec interface is satisfied for free. The two narrower
// FlatFields* methods are not implemented here because the abstract
// class promises no default behavior for them; concrete embedders
// must override them.
type BaseFlatVectorsFormat struct {
	*codecs.BaseKnnVectorsFormat
}

// NewBaseFlatVectorsFormat constructs a BaseFlatVectorsFormat with the
// given format name, matching the Java protected constructor
// FlatVectorsFormat(String name).
func NewBaseFlatVectorsFormat(name string) *BaseFlatVectorsFormat {
	return &BaseFlatVectorsFormat{
		BaseKnnVectorsFormat: codecs.NewBaseKnnVectorsFormat(name),
	}
}

// GetMaxDimensions returns [FlatVectorsFormatMaxDimensions] regardless
// of the field name. Concrete formats may override; the Java base
// itself returns 1024 unconditionally.
func (*BaseFlatVectorsFormat) GetMaxDimensions(_ string) int {
	return FlatVectorsFormatMaxDimensions
}
