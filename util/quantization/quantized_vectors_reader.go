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

package quantization

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// QuantizedVectorsReader is the quantized vector reader. Port of
// org.apache.lucene.util.quantization.QuantizedVectorsReader
// (Lucene 10.5.0).
//
// Composes io.Closer (Java Closeable) and util.Accountable.
type QuantizedVectorsReader interface {
	io.Closer
	util.Accountable

	// GetQuantizedVectorValues returns the quantized vector values of the
	// given field. Mirrors getQuantizedVectorValues(String).
	GetQuantizedVectorValues(fieldName string) (BaseQuantizedByteVectorValues, error)

	// GetQuantizationState returns the ScalarQuantizer used for the given
	// field's quantization, or nil. Mirrors getQuantizationState(String).
	GetQuantizationState(fieldName string) *ScalarQuantizer

	// GetRandomVectorScorerSupplierForMerge provides a scorer for merging
	// this quantized vector reader, so that any additional merging logic can
	// be implemented by the user of this type. The returned supplier scores
	// over the newly merged flat vectors and must be closed, as it may hold
	// temporary file handles to read over auxiliary data structures. Mirrors
	// getRandomVectorScorerSupplierForMerge(FieldInfo, SegmentWriteState).
	GetRandomVectorScorerSupplierForMerge(
		fieldInfo *spi.FieldInfo,
		segmentWriteState *spi.SegmentWriteState,
	) (hnsw.CloseableRandomVectorScorerSupplier, error)
}
