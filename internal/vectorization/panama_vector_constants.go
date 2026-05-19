// This file is a documentation-only stub for the Java reference type
// org.apache.lucene.internal.vectorization.PanamaVectorConstants
// from Apache Lucene 10.4.0
// (core/src/java21/org/apache/lucene/internal/vectorization/PanamaVectorConstants.java).
//
// The Java class holds shared constants for implementations that exploit the
// JDK Panama Vector API:
//
//   - PREFERRED_VECTOR_BITSIZE   — platform preferred vector bit-size, with
//     an opt-in test override via VectorizationProvider.TESTS_VECTOR_SIZE.
//   - HAS_FAST_INTEGER_VECTORS   — whether integer vectors are trustworthy
//     on the current CPU (HotSpot misses some SSE intrinsics on AMD64 with
//     vectors narrower than 256 bits).
//   - PRERERRED_LONG_SPECIES,
//     PRERERRED_INT_SPECIES,
//     PREFERRED_DOUBLE_SPECIES — VectorSpecies handles for the preferred
//     long/int/double element widths at the preferred bit-size.
//
// Gocene does not bind to the Panama Vector API. The Go runtime exposes SIMD
// only via stdlib intrinsics and compiler vectorization; there is no
// VectorSpecies analogue and no preferred-bit-size negotiation between the
// codec and an external SIMD layer. Concrete SIMD-accelerated paths live
// inside the codec packages themselves (see DefaultVectorUtilSupport and the
// memory-segment scorer suppliers in this directory), with the VectorUtil
// helpers operating directly on []byte / []float32 slices.
//
// Consequently this port intentionally publishes no symbols: there is no
// caller in the Go tree that needs PREFERRED_VECTOR_BITSIZE or a
// VectorSpecies, and exposing a value that does not drive any code path
// would be misleading. If a future Gocene SIMD backend requires a tunable
// preferred-width knob, this file is the canonical home for it and the
// corresponding rmp task should reopen the contract decision.

package vectorization
