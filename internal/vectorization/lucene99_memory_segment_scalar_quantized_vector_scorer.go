// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package vectorization

// This file is the Gocene placeholder for Lucene's JDK-21-only
// org.apache.lucene.internal.vectorization.Lucene99MemorySegmentScalarQuantizedVectorScorer
// (lucene/core/src/java21/org/apache/lucene/internal/vectorization/
// Lucene99MemorySegmentScalarQuantizedVectorScorer.java).
//
// In Lucene, this class is the JDK-21 Panama/MemorySegment fast path for HNSW
// scalar-quantized vector scoring. It implements FlatVectorsScorer and is
// selected only when the QuantizedByteVectorValues' slice is a
// MemorySegmentAccessInput, in which case it builds a RandomVectorScorer or
// RandomVectorScorerSupplier that:
//
//   - Reads each (vector || float offset) node directly off the MemorySegment
//     (segmentSliceOrNull fast path with a heap byte[] fallback), decoding the
//     trailing per-vector quantization offset as a little-endian unaligned int
//     and reinterpreting it as a float.
//   - Dispatches per VectorSimilarityFunction (and per ScalarQuantizer bit
//     width / compression: full 8-bit, packed int4, single-packed int4, or
//     both-packed int4 for the updateable supplier variant) to
//     PanamaVectorUtilSupport SIMD kernels: uint8SquareDistance,
//     int4SquareDistance(SinglePacked|BothPacked), uint8DotProduct,
//     int4DotProduct(SinglePacked|BothPacked).
//   - Applies the canonical scalar-quantized score combiner
//     scaler(rawScore * constantMultiplier + nodeOffset + queryOffset), where
//     scaler is VectorUtil.normalizeDistanceToUnitInterval for Euclidean,
//     normalizeToUnitInterval for dot product / cosine, and
//     scaleMaxInnerProductScore for max inner product.
//   - For float-target queries, it quantizes the target up front via
//     ScalarQuantizedVectorScorer.quantizeQuery; for byte-target queries it
//     delegates straight back to Lucene99ScalarQuantizedVectorScorer
//     (the non-Panama path).
//   - Falls back to Lucene99ScalarQuantizedVectorScorer wrapping
//     DefaultFlatVectorScorer when the input is not a MemorySegment slice.
//
// Go does not need this abstraction. Panama/MemorySegment exists in Java to
// give the JVM Foreign-Memory + Vector API access to off-heap memory and to
// schedule SIMD intrinsics; Go's slices are already first-class views over
// raw memory, and SIMD (when needed) is reached through assembly or
// platform-specific Go intrinsic packages rather than through a Foreign-Memory
// indirection. Gocene therefore folds the entire role of this class directly
// into the portable scalar-quantized HNSW scorer stack:
//
//   - codecs/hnsw/scalar_quantized_vector_scorer.go and the related
//     FlatVectorsScorer / RandomVectorScorer(Supplier) implementations
//     produced by codecs/lucene99 for the scalar-quantized format.
//   - util/quantization (ScalarQuantizer.quantizeQuery, constantMultiplier,
//     bit-width / compression branching) and util/vector_util kernels
//     (uint8SquareDistance, int4SquareDistance / int4SquareDistance(Single|Both)Packed,
//     uint8DotProduct, int4DotProduct / int4DotProduct(Single|Both)Packed),
//     which operate on []byte backed by the same mmap-backed bytes that
//     MMapDirectory hands out (see store/memory_segment_index_input.go),
//     applying the canonical scaler(raw*constMultiplier + nodeOffset + queryOffset)
//     combiner per VectorSimilarityFunction.
//
// No type is defined here on purpose. The Lucene contract is fully realised
// by the portable scalar-quantized FlatVectorsScorer in codecs/hnsw and the
// VectorUtil-style similarity kernels it calls.
//
// If a future task needs a dedicated scorer shape that mirrors the Lucene
// MemorySegment fast path for performance reasons (for example, to wire a
// SIMD intrinsic path for the int4 single/both-packed kernels over mmap-backed
// bytes), it should be added on top of those existing types rather than
// reintroducing an Arena/MemorySegment abstraction.
