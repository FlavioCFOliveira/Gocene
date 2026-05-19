// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package vectorization

// This file is the Gocene placeholder for Lucene's JDK-21-only
// org.apache.lucene.internal.vectorization.Lucene99MemorySegmentFloatVectorScorer
// (lucene/core/src/java21/org/apache/lucene/internal/vectorization/
// Lucene99MemorySegmentFloatVectorScorer.java).
//
// In Lucene, this sealed class is the JDK-21 Panama/MemorySegment fast path
// for HNSW float-vector scoring of a single query vector, including the
// four-way bulkScore kernel. It is selected only when the underlying
// IndexInput exposes a contiguous MemorySegment (via MemorySegmentAccessInput)
// and a single segment can cover the entire input, then dispatches per
// VectorSimilarityFunction to one of four subclasses (CosineScorer,
// DotProductScorer, EuclideanScorer, MaxInnerProductScorer). Each subclass
// delegates the per-node score path to VectorSimilarityFunction.compare
// (on-heap copy) and the bulk path to MemorySegmentBulkVectorOps singletons
// (Cosine, DotProduct, SqrDistance), processing four nodes at a time and
// normalising raw scores via VectorUtil.normalizeToUnitInterval,
// normalizeDistanceToUnitInterval, or scaleMaxInnerProductScore.
//
// Go does not need this abstraction. Panama/MemorySegment exists in Java to
// give the JVM Foreign-Memory + Vector API access to off-heap memory and to
// schedule SIMD intrinsics; Go's slices are already first-class views over
// raw memory, and SIMD (when needed) is reached through assembly or
// platform-specific Go intrinsic packages rather than through a Foreign-Memory
// indirection. Gocene therefore folds the entire role of this class directly
// into the portable HNSW scorer stack:
//
//   - util/hnsw RandomVectorScorer implementations for float vectors, which
//     operate on []float32 backed by the same mmap-backed bytes that
//     MMapDirectory hands out (see store/memory_segment_index_input.go) and
//     apply the same canonical float-similarity normalisations as Lucene.
//   - util/vector_util float-similarity kernels (cosine, dot product,
//     Euclidean, max inner product), which provide the per-pair and bulk
//     score computations that MemorySegmentBulkVectorOps provides on the
//     JVM side.
//
// No type is defined here on purpose. The Lucene contract is fully realised
// by the portable RandomVectorScorer implementations and the VectorUtil-style
// similarity kernels they call.
//
// If a future task needs a dedicated scorer shape that mirrors the Lucene
// four-way bulkScore kernel for performance reasons (for example, to wire a
// SIMD intrinsic path for float vectors), it should be added on top of those
// existing types rather than reintroducing an Arena/MemorySegment
// abstraction.
