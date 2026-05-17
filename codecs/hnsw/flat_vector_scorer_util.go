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

// FlatVectorScorerUtil is the Go port of
// org.apache.lucene.codecs.hnsw.FlatVectorScorerUtil (Lucene 10.4.0).
// It exposes two factory helpers that the Java reference uses to plug
// in platform-optimized scorers via the VectorizationProvider SPI; on
// platforms (or build profiles) without an optimized provider, both
// helpers fall back to the canonical [DefaultFlatVectorScorer].
//
// Gocene has not yet ported VectorizationProvider — the canonical
// `org.apache.lucene.internal.vectorization` SPI requires the Java
// Panama vector API and a service-loader bridge that does not
// translate cleanly to Go's compilation model. Until a Go-native
// vectorization SPI lands, both helpers return the singleton
// [DefaultFlatVectorScorerInstance], which is byte-for-byte
// equivalent to the Java fallback path on platforms where the
// optimized provider is unavailable.
//
// Mirrors the Java class as a package with two package-level
// functions; the Java class is `final` and its constructor is
// private, so it has no instantiable surface — the Go port preserves
// that by exposing only the two factory functions without a backing
// type.

// GetLucene99FlatVectorsScorer returns a [FlatVectorsScorer] suitable
// for the Lucene99 flat vectors format. Scorers retrieved through this
// helper may be optimized on certain platforms; until Gocene ports a
// vectorization SPI, the returned scorer is the singleton
// [DefaultFlatVectorScorerInstance].
//
// Mirrors the Java static method
// FlatVectorScorerUtil#getLucene99FlatVectorsScorer().
func GetLucene99FlatVectorsScorer() FlatVectorsScorer {
	return DefaultFlatVectorScorerInstance
}

// GetLucene99ScalarQuantizedVectorsScorer returns a [FlatVectorsScorer]
// suitable for the Lucene99 scalar-quantized flat vectors format.
// Until a vectorization SPI lands, the returned scorer wraps the
// default scorer in a [ScalarQuantizedVectorScorer]; the wrapper
// passes quantized vectors through the scalar-quantized similarity
// pipeline while non-quantized inputs are delegated to the underlying
// DefaultFlatVectorScorer.
//
// Mirrors the Java static method
// FlatVectorScorerUtil#getLucene99ScalarQuantizedVectorsScorer().
func GetLucene99ScalarQuantizedVectorsScorer() FlatVectorsScorer {
	return NewScalarQuantizedVectorScorer(DefaultFlatVectorScorerInstance)
}
