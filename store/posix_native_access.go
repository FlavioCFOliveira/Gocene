// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly
// +build linux darwin freebsd netbsd openbsd dragonfly

package store

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// posixNativeAccess is the Go port of the package-private JDK-21 class
// org.apache.lucene.store.PosixNativeAccess from Apache Lucene 10.4.0.
//
// Upstream Lucene reaches posix_madvise via the Java foreign-memory API
// (Linker / SymbolLookup / MethodHandle). In Gocene the equivalent is the
// already-portable golang.org/x/sys/unix.Madvise wrapper, which on Linux
// calls the madvise(2) syscall directly and on BSD/Darwin reaches the libc
// posix_madvise symbol through cgo-less syscall shims. The set of advice
// constants exposed by x/sys/unix matches the POSIX_MADV_* values used by
// the upstream class (NORMAL=0, RANDOM=1, SEQUENTIAL=2, WILLNEED=3).
//
// The receiver is stateless and the package-level singleton is constructed
// once in init; getInstance returns it via the cross-platform dispatcher
// installed in native_access_posix.go.
type posixNativeAccess struct {
	pageSize int
}

// posixInstance is the lazily-validated singleton, mirroring the
// Optional<NativeAccess> INSTANCE field on the upstream class. It is set in
// init and treated as immutable thereafter.
var posixInstance *posixNativeAccess

func init() {
	posixInstance = &posixNativeAccess{pageSize: unix.Getpagesize()}
}

// Madvise issues posix_madvise for the given mapped region, translating the
// high-level ReadAdvice into the matching POSIX_MADV_* constant. A nil or
// empty segment is treated as a no-op, matching the upstream guard that
// short-circuits zero-length MemorySegments because they may have no
// address at all.
func (p *posixNativeAccess) Madvise(segment []byte, advice ReadAdvice) error {
	if len(segment) == 0 {
		return nil
	}
	adv, err := mapReadAdvice(advice)
	if err != nil {
		return err
	}
	return p.madvise(segment, adv)
}

// MadviseWillNeed issues madvise with MADV_WILLNEED, asking the kernel to
// pre-fault the pages backing the segment. The empty-segment guard mirrors
// the one in Madvise above.
func (p *posixNativeAccess) MadviseWillNeed(segment []byte) error {
	if len(segment) == 0 {
		return nil
	}
	return p.madvise(segment, unix.MADV_WILLNEED)
}

// PageSize returns the native virtual-memory page size, captured once in
// init via syscall.Getpagesize. The upstream class caches the value of
// libc's getpagesize() in the static PAGE_SIZE field for the same reason.
func (p *posixNativeAccess) PageSize() int {
	return p.pageSize
}

// madvise is the private helper that performs the actual syscall and wraps
// any non-zero return value in a descriptive error, mirroring the upstream
// IOException with the same format string semantics (address + byteSize +
// return code). x/sys/unix.Madvise already converts the C return value
// into a syscall.Errno error, so we only need to attach context.
func (p *posixNativeAccess) madvise(segment []byte, advice int) error {
	if err := unix.Madvise(segment, advice); err != nil {
		return fmt.Errorf(
			"call to posix_madvise with byteSize=%d failed: %w",
			len(segment), err,
		)
	}
	return nil
}

// mapReadAdvice translates the cross-platform ReadAdvice enum into the
// POSIX_MADV_* integer expected by the syscall. It mirrors the upstream
// switch expression mapReadAdvice. Unknown values surface as an error
// rather than silently degrading to MADV_NORMAL.
func mapReadAdvice(advice ReadAdvice) (int, error) {
	switch advice {
	case ReadAdviceNormal:
		return unix.MADV_NORMAL, nil
	case ReadAdviceRandom:
		return unix.MADV_RANDOM, nil
	case ReadAdviceSequential:
		return unix.MADV_SEQUENTIAL, nil
	default:
		return 0, fmt.Errorf("unknown ReadAdvice value: %d", advice)
	}
}
