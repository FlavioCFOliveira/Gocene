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

	"github.com/FlavioCFOliveira/Gocene/util"
)

// QuantizedVectorsReader is the reader interface for quantized
// vectors. Port of org.apache.lucene.util.quantization.QuantizedVectorsReader
// (Lucene 10.4.0).
//
// Composes io.Closer (Java Closeable) and util.Accountable.
type QuantizedVectorsReader interface {
	io.Closer
	util.Accountable

	// GetQuantizedVectorValues returns the QuantizedByteVectorValues
	// for the given field. Returns an error for IO failures (Java
	// IOException).
	GetQuantizedVectorValues(fieldName string) (QuantizedByteVectorValues, error)

	// GetQuantizationState returns the ScalarQuantizer used for the
	// given field's quantization (or nil if the field is not
	// quantized).
	GetQuantizationState(fieldName string) ScalarQuantizer
}
