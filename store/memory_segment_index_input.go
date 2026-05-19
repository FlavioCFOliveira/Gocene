// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

// This file is the Gocene placeholder for Lucene's JDK-21-only
// org.apache.lucene.store.MemorySegmentIndexInput
// (lucene/core/src/java21/org/apache/lucene/store/MemorySegmentIndexInput.java).
//
// In Lucene, MemorySegmentIndexInput is the JDK-21 Foreign Memory API (Arena +
// MemorySegment) backend behind MMapDirectory: it owns the array of segments
// produced by mmap(2) and exposes the IndexInput surface (slice, clone,
// random-access primitives, prefetch/madvise hints) on top of them.
//
// Go does not need this abstraction layer. The Foreign Memory API exists in
// Java because the language has no first-class access to raw native memory;
// Go does. Gocene therefore folds the entire MemorySegmentIndexInput role
// directly into MMapDirectory and its companion mmap_*.go files
// (mmap_unix.go, mmap_windows.go, mmap_directory.go), which:
//
//   - call syscall.Mmap (POSIX) / equivalent Windows APIs to map files,
//   - hand back []byte windows that the IndexInput implementation reads
//     from with zero copies,
//   - manage the lifecycle (munmap on Close) without an Arena indirection.
//
// No type is defined here on purpose. The Lucene contract is fully realised
// by:
//
//   - MMapDirectory          (store/mmap_directory.go)
//   - the platform mmap glue (store/mmap_unix.go, store/mmap_windows.go,
//     store/mmap_directory_windows.go)
//
// If a future task needs an explicit MemorySegmentIndexInput-shaped type
// (for example, to expose multi-segment slicing as a public API), it should
// be added here and built on top of the existing MMapDirectory primitives
// rather than reintroducing an Arena/MemorySegment abstraction.
