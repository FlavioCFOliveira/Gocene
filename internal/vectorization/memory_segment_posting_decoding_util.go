// Doc stub for Lucene 10.4.0
// org.apache.lucene.internal.vectorization.MemorySegmentPostingDecodingUtil
// (JDK21 source set).
//
// Upstream: lucene/core/src/java21/org/apache/lucene/internal/vectorization/
// MemorySegmentPostingDecodingUtil.java
//
// Upstream is a final, package-private subclass of PostingDecodingUtil that
// overrides splitInts to decode bit-packed posting blocks via Panama
// IntVector lanes loaded directly from a java.lang.foreign.MemorySegment
// (little-endian, INT_SPECIES width). The vectorised path right-aligns the
// tail by re-reading the trailing lane window and advances the backing
// IndexInput by count*Integer.BYTES.
//
// Gocene status: not ported. The Go runtime exposes no equivalent of the
// JDK Panama Vector API, and the existing PostingDecodingUtil stub in this
// package is a scalar placeholder consumed by codecs. A SIMD-aware
// MemorySegmentPostingDecodingUtil counterpart is deferred until a Go SIMD
// strategy is decided (likely golang.org/x/sys CPU feature gates plus
// hand-written intrinsics or assembly).
//
// This file is intentionally documentation-only; no declarations are
// emitted so the scalar PostingDecodingUtil in vectorization.go remains the
// single concrete type.

package vectorization
