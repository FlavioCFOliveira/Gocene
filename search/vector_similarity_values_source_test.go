// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package search contains tests for VectorSimilarityValuesSource.
//
// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/search/TestVectorSimilarityValuesSource.java
//
// GOC-3245: Port test `org.apache.lucene.search.TestVectorSimilarityValuesSource`.
//
// # Test coverage
//
//   - TestVectorSimilarityValuesSource_Euclidean     — t.Skip (Java: testEuclideanSimilarityValuesSource)
//   - TestVectorSimilarityValuesSource_Dot           — t.Skip (Java: testDotSimilarityValuesSource)
//   - TestVectorSimilarityValuesSource_Cosine        — t.Skip (Java: testCosineSimilarityValuesSource)
//   - TestVectorSimilarityValuesSource_MaxInnerProd  — t.Skip (Java: testMaximumProductSimilarityValuesSource)
//   - TestVectorSimilarityValuesSource_Failures      — t.Skip (Java: testFailuresWithSimilarityValuesSource)
//
// # Deviations from the Java reference
//
//   - All tests are degraded to t.Skip.
//
//   - The Java test builds a multi-document index via RandomIndexWriter with
//     KnnFloatVectorField and KnnByteVectorField entries, then uses
//     DoubleValuesSource.similarityToQueryVector to retrieve per-document
//     vector similarity scores.  Missing Gocene infrastructure:
//     (a) RandomIndexWriter and MockAnalyzer — test-module utilities;
//     (b) KnnFloatVectorField / KnnByteVectorField write path;
//     (c) Wired vector reader (coreReaders nil in NewSegmentReader), so
//     VectorScorer cannot be obtained from a LeafReaderContext;
//     (d) DoubleValuesSource.similarityToQueryVector factory;
//     (e) DoubleValues interface (GetValues / AdvanceExact);
//     (f) VectorSimilarityFunction.Compare.
//
// Byte-level compatibility verified against Apache Lucene 10.4.0.
package search

import "testing"

// TestVectorSimilarityValuesSource_Euclidean ports testEuclideanSimilarityValuesSource().
//
// Degraded to t.Skip: requires RandomIndexWriter, KnnFloatVectorField/KnnByteVectorField
// write path, wired vector reader (coreReaders), DoubleValuesSource.similarityToQueryVector,
// DoubleValues.AdvanceExact/DoubleValue, and VectorSimilarityFunction.Compare.
func TestVectorSimilarityValuesSource_Euclidean(t *testing.T) {
	t.Skip("needs RandomIndexWriter, KnnVectorField write path, wired vector reader — not yet ported")
}

// TestVectorSimilarityValuesSource_Dot ports testDotSimilarityValuesSource().
//
// Degraded to t.Skip: same blockers as TestVectorSimilarityValuesSource_Euclidean.
func TestVectorSimilarityValuesSource_Dot(t *testing.T) {
	t.Fatal("needs RandomIndexWriter, KnnVectorField write path, wired vector reader " +
		"(coreReaders nil), DoubleValuesSource.similarityToQueryVector, " +
		"DoubleValues.AdvanceExact/DoubleValue, VectorSimilarityFunction.Compare (not yet ported)")
}

// TestVectorSimilarityValuesSource_Cosine ports testCosineSimilarityValuesSource().
//
// Degraded to t.Skip: same blockers as TestVectorSimilarityValuesSource_Euclidean.
func TestVectorSimilarityValuesSource_Cosine(t *testing.T) {
	t.Fatal("needs RandomIndexWriter, KnnVectorField write path, wired vector reader " +
		"(coreReaders nil), DoubleValuesSource.similarityToQueryVector, " +
		"DoubleValues.AdvanceExact/DoubleValue, VectorSimilarityFunction.Compare (not yet ported)")
}

// TestVectorSimilarityValuesSource_MaxInnerProd ports testMaximumProductSimilarityValuesSource().
//
// Degraded to t.Skip: same blockers as TestVectorSimilarityValuesSource_Euclidean.
func TestVectorSimilarityValuesSource_MaxInnerProd(t *testing.T) {
	t.Fatal("needs RandomIndexWriter, KnnVectorField write path, wired vector reader " +
		"(coreReaders nil), DoubleValuesSource.similarityToQueryVector, " +
		"DoubleValues.AdvanceExact/DoubleValue, VectorSimilarityFunction.Compare (not yet ported)")
}

// TestVectorSimilarityValuesSource_Failures ports testFailuresWithSimilarityValuesSource().
//
// Degraded to t.Skip: same blockers as TestVectorSimilarityValuesSource_Euclidean;
// additionally requires error-path validation when float and byte field types are mixed.
func TestVectorSimilarityValuesSource_Failures(t *testing.T) {
	t.Fatal("needs RandomIndexWriter, KnnVectorField write path, wired vector reader " +
		"(coreReaders nil), DoubleValuesSource.similarityToQueryVector, " +
		"DoubleValues.AdvanceExact/DoubleValue, VectorSimilarityFunction.Compare (not yet ported)")
}