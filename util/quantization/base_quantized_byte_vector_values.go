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

// BaseQuantizedByteVectorValues is the Go port of
// org.apache.lucene.util.quantization.BaseQuantizedByteVectorValues
// (Lucene 10.5.0): a [ByteVectorValues] for scalar quantization scores that
// also implements [HasIndexSlice].
//
// The Java abstract class carries two default bodies: scorer(float[]) throws
// UnsupportedOperationException and getSlice() returns null. A Go interface
// carries no bodies, so every implementer declares both members; one that
// does not override them in Lucene returns [ErrUnsupportedOperation] from
// ScorerFloat and nil from GetSlice.
//
// scorer(float[]) overloads ByteVectorValues.scorer(byte[]). Go has no
// overloading, so the float-query overload is named ScorerFloat, following
// the element-type suffix Gocene gives the other vector overloads
// (GetRandomVectorScorerFloat/Byte, SearchFloat/Byte).
type BaseQuantizedByteVectorValues interface {
	ByteVectorValues
	HasIndexSlice

	// ScorerFloat returns a [VectorScorer] for the given float query
	// vector, or nil. Mirrors scorer(float[] query).
	ScorerFloat(query []float32) (VectorScorer, error)
}
