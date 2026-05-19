// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

// This file is the Gocene placeholder for Lucene's JDK-21-only
// org.apache.lucene.store.MemorySegmentIndexInputProvider
// (lucene/core/src/java21/org/apache/lucene/store/MemorySegmentIndexInputProvider.java).
//
// In Lucene, MemorySegmentIndexInputProvider is the JDK-21 SPI implementation
// of MMapDirectory.MMapIndexInputProvider. It bridges MMapDirectory to the
// Foreign Memory API: it opens a FileChannel, slices the file into a power-of-
// two array of MemorySegments via FileChannel.map(..., Arena), applies
// preload/madvise hints through NativeAccess, manages confined vs shared
// (RefCountedSharedArena) Arenas keyed by segment group, and finally hands the
// MemorySegment[] to MemorySegmentIndexInput.newInstance to produce an
// IndexInput. It also publishes the JDK-21 defaults: 16 GiB chunk size on
// 64-bit JREs, 256 MiB on 32-bit, and madvise support gated on NativeAccess.
//
// Go does not need this provider abstraction. The SPI exists in Lucene to
// switch between a legacy NIO-MappedByteBuffer backend and the JDK-21
// Foreign Memory API backend at runtime; Gocene has a single, idiomatic mmap
// path. The provider's responsibilities are folded directly into
// MMapDirectory and the platform mmap glue:
//
//   - chunked mmap of the file:        store/mmap_unix.go, store/mmap_windows.go
//   - lifecycle (munmap on Close):     same files (no Arena indirection)
//   - madvise / preload hints:         store/mmap_directory.go via syscall.Madvise
//                                       and MAP_POPULATE/Mlock equivalents
//   - default max chunk size:          store/mmap_directory.go (DefaultMaxChunkSize)
//   - shared-vs-confined arenas:       n/a; Go's GC + explicit Close on
//                                       IndexInput clones removes the need
//                                       for RefCountedSharedArena bookkeeping
//
// No type is defined here on purpose. The Lucene contract is fully realised
// by MMapDirectory (store/mmap_directory.go) together with the platform mmap
// glue (store/mmap_unix.go, store/mmap_windows.go,
// store/mmap_directory_windows.go) and the documentation-only peer
// store/memory_segment_index_input.go.
//
// If a future task needs an explicit provider-shaped type (for example, to
// expose pluggable mmap backends), it should be added here and built on top
// of the existing MMapDirectory primitives rather than reintroducing an
// Arena / MemorySegment SPI.
