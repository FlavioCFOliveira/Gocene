// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly
// +build linux darwin freebsd netbsd openbsd dragonfly

package store

import (
	"errors"
	"testing"

	"golang.org/x/sys/unix"
)

// TestGetNativeAccess_ReturnsPosixSingletonOnPosix verifies that the
// dispatcher returns a non-nil singleton with the (impl, true) shape on
// every POSIX-tagged platform, mirroring upstream Optional.of(INSTANCE).
func TestGetNativeAccess_ReturnsPosixSingletonOnPosix(t *testing.T) {
	t.Parallel()
	na, ok := GetNativeAccess()
	if !ok {
		t.Fatal("GetNativeAccess returned ok=false on a POSIX build")
	}
	if na == nil {
		t.Fatal("GetNativeAccess returned ok=true but a nil NativeAccess")
	}
	if got, want := na.PageSize(), unix.Getpagesize(); got != want {
		t.Fatalf("PageSize: got %d, want %d", got, want)
	}
	// Singleton invariant: two calls return the same backing instance.
	na2, _ := GetNativeAccess()
	if na != na2 {
		t.Fatalf("GetNativeAccess must be a singleton, got distinct pointers")
	}
}

// TestPosixNativeAccess_MadviseEmptyIsNoOp guards the explicit zero-length
// short-circuit copied from the upstream class; an empty MemorySegment has
// no address and must not reach the syscall.
func TestPosixNativeAccess_MadviseEmptyIsNoOp(t *testing.T) {
	t.Parallel()
	p := &posixNativeAccess{pageSize: unix.Getpagesize()}
	if err := p.Madvise(nil, ReadAdviceNormal); err != nil {
		t.Fatalf("Madvise(nil): unexpected error: %v", err)
	}
	if err := p.Madvise([]byte{}, ReadAdviceSequential); err != nil {
		t.Fatalf("Madvise(empty): unexpected error: %v", err)
	}
	if err := p.MadviseWillNeed(nil); err != nil {
		t.Fatalf("MadviseWillNeed(nil): unexpected error: %v", err)
	}
}

// TestPosixNativeAccess_MadviseOnMmapped exercises the happy path against
// a real mmap region so the syscall actually fires under -race.
func TestPosixNativeAccess_MadviseOnMmapped(t *testing.T) {
	t.Parallel()
	p, ok := GetNativeAccess()
	if !ok {
		t.Skip("no NativeAccess on this platform")
	}
	const size = 1 << 16 // 64 KiB, comfortably above any page size we target.
	region, err := unix.Mmap(-1, 0, size,
		unix.PROT_READ|unix.PROT_WRITE,
		unix.MAP_ANON|unix.MAP_PRIVATE)
	if err != nil {
		t.Fatalf("Mmap: %v", err)
	}
	t.Cleanup(func() {
		if err := unix.Munmap(region); err != nil {
			t.Errorf("Munmap: %v", err)
		}
	})
	for _, adv := range []ReadAdvice{
		ReadAdviceNormal, ReadAdviceRandom, ReadAdviceSequential,
	} {
		if err := p.Madvise(region, adv); err != nil {
			t.Fatalf("Madvise(%s): %v", adv, err)
		}
	}
	if err := p.MadviseWillNeed(region); err != nil {
		t.Fatalf("MadviseWillNeed: %v", err)
	}
}

// TestPosixNativeAccess_MadviseUnknownAdviceRejected verifies that values
// outside the ReadAdvice enum surface as an error rather than silently
// mapping to MADV_NORMAL.
func TestPosixNativeAccess_MadviseUnknownAdviceRejected(t *testing.T) {
	t.Parallel()
	p := &posixNativeAccess{pageSize: unix.Getpagesize()}
	region, err := unix.Mmap(-1, 0, unix.Getpagesize(),
		unix.PROT_READ|unix.PROT_WRITE,
		unix.MAP_ANON|unix.MAP_PRIVATE)
	if err != nil {
		t.Fatalf("Mmap: %v", err)
	}
	t.Cleanup(func() { _ = unix.Munmap(region) })
	err = p.Madvise(region, ReadAdvice(99))
	if err == nil {
		t.Fatal("expected error for unknown ReadAdvice, got nil")
	}
	// The error path must not be a syscall errno (we never reached the
	// syscall): it is a plain validation error from mapReadAdvice.
	var errno unix.Errno
	if errors.As(err, &errno) {
		t.Fatalf("unexpected syscall errno from validation path: %v", err)
	}
}
