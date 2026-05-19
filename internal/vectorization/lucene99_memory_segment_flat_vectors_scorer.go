// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package vectorization

// This file is the Gocene placeholder for Lucene's JDK-21-only
// org.apache.lucene.internal.vectorization.Lucene99MemorySegmentFlatVectorsScorer
// (lucene/core/src/java21/org/apache/lucene/internal/vectorization/
// Lucene99MemorySegmentFlatVectorsScorer.java).
//
// In Lucene, this class is the JDK-21 entry point that implements
// FlatVectorsScorer and acts as a dispatcher: when the supplied
// KnnVectorValues exposes an index slice via HasIndexSlice and that slice
// is backed by a MemorySegmentAccessInput, the scorer hands the request off
// to Lucene99MemorySegmentFloatVectorScorer / Lucene99MemorySegmentByteVectorScorer
// (or their *Supplier counterparts), which then run the SIMD-vectorised
// Panama kernels directly against the off-heap segment. When the slice is
// missing or the input does not expose a MemorySegment, it delegates to the
// portable DefaultFlatVectorScorer.INSTANCE. It owns a single
// publicly-exposed INSTANCE singleton wired with that default delegate.
//
// Go does not need this dispatcher. Panama/MemorySegment exists in Java to
// give the JVM Foreign-Memory + Vector API access to off-heap memory and to
// schedule SIMD intrinsics; Go's slices are already first-class views over
// raw memory, and SIMD (when needed) is reached through assembly or
// platform-specific Go intrinsic packages rather than through a Foreign-Memory
// indirection. Gocene therefore folds the entire role of this class directly
// into the portable flat-vectors scorer stack:
//
//   - codecs/hnsw DefaultFlatVectorScorer (see
//     codecs/hnsw/default_flat_vector_scorer.go) is the only FlatVectorsScorer
//     implementation, and it already operates on []float32 / []byte slices
//     backed by the same mmap-backed bytes that MMapDirectory hands out
//     (see store/memory_segment_index_input.go).
//   - The dispatch decisions Lucene makes at this level (per encoding, per
//     similarity, per slice availability) collapse into direct calls to the
//     portable scorer in Go, with no MemorySegment fast path to select.
//
// No type is defined here on purpose. The Lucene contract is fully realised
// by the portable DefaultFlatVectorScorer and the VectorUtil-style similarity
// kernels it calls.
//
// If a future task needs a dedicated dispatcher shape that mirrors the
// Lucene bulkScore fast path for performance reasons (for example, to wire a
// SIMD intrinsic path), it should be added on top of those existing types
// rather than reintroducing an Arena/MemorySegment abstraction.
