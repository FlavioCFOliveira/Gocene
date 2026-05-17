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
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// FlatVectorsScorer is the Go port of
// org.apache.lucene.codecs.hnsw.FlatVectorsScorer (Lucene 10.4.0). It
// provides mechanisms to score vectors stored in a flat file; the
// purpose of this interface is to give codec authors the flexibility to
// plug in optimized scorers (vectorized, off-heap, quantized, etc.)
// without changing the consumer-facing API.
//
// The three GetRandomVectorScorer overloads mirror the Java methods of
// the same name. Because Go has no method overloading, the byte-target
// overload is suffixed Byte; the float-target overload keeps the
// canonical name.
//
// Implementations return scorers from util/hnsw — the Java reference
// returns RandomVectorScorer and RandomVectorScorerSupplier from
// org.apache.lucene.util.hnsw, and Gocene preserves that boundary.
type FlatVectorsScorer interface {
	// GetRandomVectorScorerSupplier returns a
	// [hnsw.RandomVectorScorerSupplier] that can be used to score
	// vectors against an ordinal in vectorValues.
	GetRandomVectorScorerSupplier(
		similarityFunction index.VectorSimilarityFunction,
		vectorValues hnsw.KnnVectorValues,
	) (hnsw.RandomVectorScorerSupplier, error)

	// GetRandomVectorScorer returns a [hnsw.RandomVectorScorer] that
	// scores vectors in vectorValues against the supplied float32
	// target.
	GetRandomVectorScorer(
		similarityFunction index.VectorSimilarityFunction,
		vectorValues hnsw.KnnVectorValues,
		target []float32,
	) (hnsw.RandomVectorScorer, error)

	// GetRandomVectorScorerByte returns a [hnsw.RandomVectorScorer]
	// that scores vectors in vectorValues against the supplied byte
	// target. Mirrors the byte[] overload of getRandomVectorScorer in
	// the Java reference.
	GetRandomVectorScorerByte(
		similarityFunction index.VectorSimilarityFunction,
		vectorValues hnsw.KnnVectorValues,
		target []byte,
	) (hnsw.RandomVectorScorer, error)
}

// CheckDimensions mirrors the Java static helper
// FlatVectorsScorer#checkDimensions(int, int). It returns an error
// (rather than throwing IllegalArgumentException) when the supplied
// query length does not match the field length. The error text is
// byte-for-byte identical to the Java message so consumers comparing
// error strings across implementations stay aligned.
func CheckDimensions(queryLen, fieldLen int) error {
	if queryLen != fieldLen {
		return fmt.Errorf("vector query dimension: %d differs from field dimension: %d", queryLen, fieldLen)
	}
	return nil
}
