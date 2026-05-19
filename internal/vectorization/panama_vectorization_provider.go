// Copyright 2026 Flavio Oliveira and the Gocene authors.
// Licensed under the Apache License, Version 2.0.

package vectorization

// PanamaVectorizationProvider is intentionally not ported.
//
// The upstream class at
// lucene/core/src/java21/org/apache/lucene/internal/vectorization/PanamaVectorizationProvider.java
// is a package-private VectorizationProvider variant selected at runtime when
// the JVM exposes the JDK 21 Panama Vector API (jdk.incubator.vector.*) with a
// preferred species of at least 128 bits. Its constructor probes the API (a
// workaround for JDK-8309727), then wires three Panama-specific collaborators:
//
//   - PanamaVectorUtilSupport — SIMD VectorUtilSupport kernels;
//   - Lucene99MemorySegmentFlatVectorsScorer.INSTANCE — MemorySegment-backed
//     flat-vectors scorer;
//   - Lucene99MemorySegmentScalarQuantizedVectorScorer.INSTANCE — the int8
//     quantized counterpart;
//
// and overrides newPostingDecodingUtil() to return MemorySegmentPostingDecodingUtil
// when the IndexInput exposes a MemorySegmentAccessInput slice and HAS_FAST_INTEGER_VECTORS
// is true; otherwise it falls back to the default PostingDecodingUtil.
//
// Go has no equivalent portable SIMD surface. A faithful port would require
// hand-written assembly (one variant per GOARCH/feature tier) generated via
// asm/bin/avo (https://github.com/mmcloughlin/avo) or the runtime's own
// internal/cpu feature detection, plus a registration layer in
// [VectorizationProvider] to select the asm-backed support when available and
// fall back to [DefaultVectorizationProvider].
//
// That work is explicitly out of scope for this sprint and is tracked as a
// follow-up: the scalar [DefaultVectorizationProvider] already satisfies the
// VectorizationProvider contract used by the rest of the port. When the SIMD
// backend is introduced it should live in build-tagged files
// (e.g. panama_amd64.go + panama_amd64.s, panama_arm64.go + panama_arm64.s)
// alongside this stub and register itself in [VectorizationProvider] via the
// same selection logic [DefaultVectorizationProvider] currently uses, mirroring
// the sibling [PanamaVectorUtilSupport] stub.
