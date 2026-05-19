// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

// This file is the Gocene placeholder for Lucene's JDK-21-only
// org.apache.lucene.store.MemorySegmentAccessInput
// (lucene/core/src/java21/org/apache/lucene/store/MemorySegmentAccessInput.java).
//
// In Lucene, MemorySegmentAccessInput is a tiny "expert API" interface that
// extends RandomAccessInput and Cloneable, exposing two operations on top of
// the Foreign Memory API:
//
//   - MemorySegment segmentSliceOrNull(long pos, long len)
//       Returns the backing MemorySegment for [pos, pos+len), or null when the
//       requested range straddles segment boundaries.
//   - MemorySegmentAccessInput clone()
//       Covariant clone narrowing IndexInput.clone() to this interface.
//
// Its sole purpose is to let advanced callers (mainly the vector and ANN
// codecs) reach through an IndexInput and obtain a zero-copy view of the
// underlying mmap-backed Arena slice when one is available.
//
// Go does not need this abstraction layer. The Foreign Memory API exists in
// Java because the language has no first-class access to raw native memory;
// Go does. In Gocene, the MMapDirectory implementation (see
// memory_segment_index_input.go for the companion rationale) hands back []byte
// windows directly from syscall.Mmap, and RandomAccessInput already gives
// callers random-access primitives without an Arena indirection. Any future
// caller that needs the "give me a zero-copy view of [pos, len) if it fits in
// one segment" semantics can take a []byte slice from the underlying
// MMapDirectory page directly.
//
// No type is defined here on purpose. The Lucene contract is fully realised
// by:
//
//   - RandomAccessInput      (store/random_access_input.go)
//   - MMapDirectory          (store/mmap_directory.go) and its platform glue
//     (store/mmap_unix.go, store/mmap_windows.go,
//     store/mmap_directory_windows.go), which expose the mapped pages as
//     []byte without an Arena/MemorySegment wrapper.
//
// If a future task needs an explicit MemorySegmentAccessInput-shaped type
// (for example, to expose segment-bounded zero-copy slicing as a public API),
// it should be added here and built on top of the existing MMapDirectory and
// RandomAccessInput primitives rather than reintroducing the Foreign Memory
// abstraction.
