// RefCountedSharedArena (doc-only stub).
//
// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java21/org/apache/lucene/store/RefCountedSharedArena.java
//
// Purpose
//
// In Lucene, RefCountedSharedArena groups multiple mmapped MemorySegments
// (typically all files of the same index segment) under a single shared
// java.lang.foreign.Arena. The arena is closed only when the ref count drops
// to zero, amortising the relatively expensive cost of closing a shared
// Arena across many segment-scoped mappings.
//
// State packing
//
// A single 32-bit atomic word packs two counters:
//   - high 16 bits: monotonically decreasing remaining permits (max 0x7FFF;
//     default DefaultMaxPermits = 64).
//   - low  16 bits: current ref count.
//
// acquire() atomically subtracts (REMAINING_UNIT - 1) = 0xFFFF, which both
// decrements the remaining permits by one and increments the ref count by
// one. When remaining permits reach zero, acquire() returns false; no more
// references can ever be obtained from that instance (independent of the
// live ref count).
//
// release() decrements the low 16 bits; when the ref count transitions to
// zero, the onClose runnable is invoked and the underlying shared Arena is
// closed.
//
// Go porting status
//
// Not implemented in Gocene. Go has no direct equivalent of the JDK 21
// Foreign Function & Memory API; mmap-backed access in Gocene is handled
// via syscall.Mmap/Munmap and *os.File lifetimes rather than Arena scopes.
// The reference-counted grouping semantics may be useful for a future
// MMapDirectory port, but the JDK21-only Arena adapter itself is not
// portable.
//
// When/if a Go MMapDirectory is added, an equivalent SharedMmapGroup type
// can be modelled after this class:
//   - atomic.Uint32 state with the same 16/16 packing,
//   - Acquire() bool, Release(), Close() methods,
//   - onClose func() invoked exactly once on the 1 -> 0 transition.
//
// Cross-references: [[project-gocene]], [[feedback-gocene-store-bc]].

package store
