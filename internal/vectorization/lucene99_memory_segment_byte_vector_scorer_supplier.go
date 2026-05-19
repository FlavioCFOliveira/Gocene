// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package vectorization

// This file is the Gocene placeholder for Lucene's JDK-21-only
// org.apache.lucene.internal.vectorization.Lucene99MemorySegmentByteVectorScorerSupplier
// (lucene/core/src/java21/org/apache/lucene/internal/vectorization/
// Lucene99MemorySegmentByteVectorScorerSupplier.java).
//
// In Lucene, this sealed class is the JDK-21 Panama/MemorySegment fast path
// for HNSW byte-vector scoring. It is selected only when the underlying
// IndexInput exposes a contiguous MemorySegment (via MemorySegmentAccessInput),
// and it dispatches per VectorSimilarityFunction to one of four subclasses
// (CosineSupplier, DotProductSupplier, EuclideanSupplier, MaxInnerProductSupplier),
// each delegating to PanamaVectorUtilSupport for SIMD-vectorised kernels
// (cosine, dotProduct, squareDistance) over the raw segment bytes, with the
// canonical Lucene byte-similarity normalisations applied on top of the raw
// integer score (e.g. (1+raw)/2 for cosine, 0.5 + raw/(dim * 2^15) for dot
// product, 1/(1+raw) for Euclidean, raw+1 or 1/(1-raw) for max inner product).
//
// Go does not need this abstraction. Panama/MemorySegment exists in Java to
// give the JVM Foreign-Memory + Vector API access to off-heap memory and to
// schedule SIMD intrinsics; Go's slices are already first-class views over
// raw memory, and SIMD (when needed) is reached through assembly or
// platform-specific Go intrinsic packages rather than through a Foreign-Memory
// indirection. Gocene therefore folds the entire role of this class directly
// into the portable HNSW scorer stack:
//
//   - util/hnsw/random_vector_scorer_supplier.go (RandomVectorScorerSupplier)
//   - util/hnsw and util/vector_util kernels (cosine, dot product, Euclidean,
//     max inner product) for byte vectors, which operate on []byte backed by
//     the same mmap-backed bytes that MMapDirectory hands out (see
//     store/memory_segment_index_input.go), applying the same canonical
//     byte-similarity normalisations.
//
// No type is defined here on purpose. The Lucene contract is fully realised
// by the portable RandomVectorScorerSupplier implementations and the
// VectorUtil-style similarity kernels they call.
//
// If a future task needs a dedicated supplier shape that mirrors the Lucene
// four-way bulkScore kernel for performance reasons (for example, to wire a
// SIMD intrinsic path for byte vectors), it should be added on top of those
// existing types rather than reintroducing an Arena/MemorySegment abstraction.
