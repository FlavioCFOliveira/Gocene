// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

// NativeAccess is the Go port of the package-private JDK-21 abstract class
// org.apache.lucene.store.NativeAccess.
//
// It exposes a thin, OS-flavoured surface for paging-cache advisories on
// memory-mapped segments. Upstream Lucene drives these through the Java
// foreign-memory API; in Gocene the surface is intentionally generic so
// platform implementations can wire syscall.Madvise / posix_madvise (Linux,
// macOS) without leaking unsafe.Pointer types into the public API.
//
// Implementations must be safe for concurrent use: the same NativeAccess
// instance is shared across all mmap'd files in a Directory.
//
// The Go counterpart of the upstream MemorySegment is the byte slice handed
// to Madvise: it must alias the mapped region (typically a slice produced by
// syscall.Mmap), be page-aligned at its starting address, and remain valid
// for the duration of the call.
type NativeAccess interface {
	// Madvise issues the platform's madvise syscall for the given mapped
	// region, translating the high-level ReadAdvice into the appropriate
	// MADV_* / POSIX_MADV_* constant. A nil or empty segment is a no-op.
	Madvise(segment []byte, advice ReadAdvice) error

	// MadviseWillNeed issues madvise with MADV_WILLNEED for the given mapped
	// region, hinting the kernel to pre-fault the pages.
	MadviseWillNeed(segment []byte) error

	// PageSize returns the native virtual-memory page size in bytes, as
	// reported by the kernel (e.g. sysconf(_SC_PAGESIZE) on POSIX).
	PageSize() int
}

// GetNativeAccess returns the NativeAccess implementation for the current
// platform, mirroring NativeAccess.getImplementation() in Lucene 10.4.0.
//
// The boolean ok is the Go-idiomatic stand-in for java.util.Optional: it is
// true only when a concrete implementation is wired for the build target.
// This stub returns (nil, false) on every platform; the real Linux/macOS
// wiring lands with GOC-3478 (PosixNativeAccess) and the platform-specific
// build-tag files that follow it.
func GetNativeAccess() (NativeAccess, bool) {
	return nil, false
}
