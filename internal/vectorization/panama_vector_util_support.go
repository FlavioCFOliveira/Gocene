// Copyright 2026 Flavio Oliveira and the Gocene authors.
// Licensed under the Apache License, Version 2.0.

package vectorization

// PanamaVectorUtilSupport is intentionally not ported.
//
// The upstream class at
// lucene/core/src/java21/org/apache/lucene/internal/vectorization/PanamaVectorUtilSupport.java
// implements the [VectorUtilSupport] contract on top of the JDK 21 Panama
// Vector API (jdk.incubator.vector.*), providing SIMD-accelerated dot product,
// cosine, square distance, Hamming, and int4/binary vector kernels selected at
// runtime from the host CPU's preferred vector species (typically 128-, 256-,
// or 512-bit lanes on x86/AArch64).
//
// Go has no equivalent portable SIMD surface. A faithful port would require
// hand-written assembly (one variant per GOARCH/feature tier) generated via
// asm/bin/avo (https://github.com/mmcloughlin/avo) or the runtime's own
// internal/cpu feature detection, plus a registration layer in
// [VectorizationProvider] to select the asm-backed support when available and
// fall back to [DefaultVectorUtilSupport].
//
// That work is explicitly out of scope for this sprint and is tracked as a
// follow-up: the default scalar implementation already satisfies the
// VectorUtilSupport contract used by the rest of the port. When the SIMD
// backend is introduced it should live in build-tagged files
// (e.g. panama_amd64.go + panama_amd64.s, panama_arm64.go + panama_arm64.s)
// alongside this stub, and wire itself into VectorizationProvider via the
// same selection logic [DefaultVectorizationProvider] currently uses.
