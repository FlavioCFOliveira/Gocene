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
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// FlatVectorsFormatMaxDimensions is the upper bound the abstract base
// returns from [FlatVectorsFormat.GetMaxDimensions]. Mirrors the constant
// 1024 hard-coded in
// org.apache.lucene.codecs.hnsw.FlatVectorsFormat#getMaxDimensions
// (Apache Lucene 10.5.0).
const FlatVectorsFormatMaxDimensions = 1024

// FlatVectorsFormat is the Go port of
// org.apache.lucene.codecs.hnsw.FlatVectorsFormat (Apache Lucene 10.5.0).
// It encodes/decodes per-document vectors and provides a scoring interface
// for the flat stored vectors.
//
// The Java reference is an abstract class extending KnnVectorsFormat whose
// fieldsWriter and fieldsReader narrow their return types to
// FlatVectorsWriter and FlatVectorsReader. A Go interface cannot redeclare
// the inherited [spi.KnnVectorsFormat] methods with covariant results, so the
// narrowed factories are rendered as FlatFieldsWriter and FlatFieldsReader;
// concrete formats implement FieldsWriter and FieldsReader by returning the
// same instances.
type FlatVectorsFormat interface {
	spi.KnnVectorsFormat

	// FlatFieldsWriter returns a [FlatVectorsWriter] to write the vectors to
	// the index. It is the covariant fieldsWriter(SegmentWriteState).
	FlatFieldsWriter(state *spi.SegmentWriteState) (FlatVectorsWriter, error)

	// FlatFieldsReader returns a [FlatVectorsReader] to read the vectors from
	// the index. It is the covariant fieldsReader(SegmentReadState).
	FlatFieldsReader(state *spi.SegmentReadState) (FlatVectorsReader, error)

	// GetMaxDimensions returns the maximum number of vector dimensions
	// supported for the given field name.
	GetMaxDimensions(fieldName string) int
}

// BaseFlatVectorsFormat carries the concrete members of the abstract Java
// class: the name handed to the protected constructor
// FlatVectorsFormat(String name) and getMaxDimensions, which returns 1024.
// Concrete formats embed *BaseFlatVectorsFormat and implement the field
// factories themselves.
type BaseFlatVectorsFormat struct {
	*spi.BaseKnnVectorsFormat
}

// NewBaseFlatVectorsFormat constructs a BaseFlatVectorsFormat with the
// given format name, matching the Java protected constructor
// FlatVectorsFormat(String name).
func NewBaseFlatVectorsFormat(name string) *BaseFlatVectorsFormat {
	return &BaseFlatVectorsFormat{
		BaseKnnVectorsFormat: spi.NewBaseKnnVectorsFormat(name),
	}
}

// GetMaxDimensions returns [FlatVectorsFormatMaxDimensions] regardless of
// the field name, as FlatVectorsFormat.getMaxDimensions does.
func (*BaseFlatVectorsFormat) GetMaxDimensions(_ string) int {
	return FlatVectorsFormatMaxDimensions
}
