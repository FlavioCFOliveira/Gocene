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
	"github.com/FlavioCFOliveira/Gocene/store"
)

// BaseQuantizedByteVectorValues is the Go port of
// org.apache.lucene.util.quantization.BaseQuantizedByteVectorValues
// (Lucene 10.5.0).
//
// It provides the basic contract and default behavior for quantized
// byte vector values, specifically adding the ability to create a
// VectorScorer and expose the underlying IndexInput slice.
type BaseQuantizedByteVectorValues struct{}

// Scorer returns a [VectorScorer] for the given float32 query.
// Mirrors the Java default, which throws UnsupportedOperationException.
func (*BaseQuantizedByteVectorValues) Scorer(_ []float32) (VectorScorer, error) {
	return nil, ErrUnsupportedOperation
}

// GetSlice returns the [store.IndexInput] from which this instance's
// values are read, or nil if not available. Mirrors the Java default
// that returns null.
func (*BaseQuantizedByteVectorValues) GetSlice() store.IndexInput {
	return nil
}
