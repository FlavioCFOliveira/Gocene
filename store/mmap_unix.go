// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build (linux || darwin) && !windows
// +build linux darwin
// +build !windows

package store

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// mmapFile represents a memory-mapped file on Unix systems.
type mmapFile struct {
	// data is the slice visible to callers, starting at the requested offset.
	data []byte
	// raw is the full mmap allocation from the page-aligned offset; used for Munmap.
	raw    []byte
	length int64
	file   *os.File
}

// mmap maps a region of the file into memory using Unix syscalls.
// Parameters:
//   - f: the file to map
//   - offset: the offset in the file to start mapping from
//   - length: the number of bytes to map
func mmap(f *os.File, offset int64, length int64) (*mmapFile, error) {
	// Get file info to check size
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	if info.Size() < offset+length {
		return nil, fmt.Errorf("file size too small: expected at least %d, got %d", offset+length, info.Size())
	}

	// Handle empty file
	if length == 0 {
		return &mmapFile{
			data:   nil,
			length: 0,
			file:   f,
		}, nil
	}

	// mmap requires the offset to be a multiple of the system page size.
	// Align the offset down to the nearest page boundary, then adjust the
	// returned slice so that callers see data starting at the original offset.
	pageSize := int64(os.Getpagesize())
	alignedOffset := (offset / pageSize) * pageSize
	delta := offset - alignedOffset // bytes between aligned offset and requested offset

	raw, err := syscall.Mmap(int(f.Fd()), alignedOffset, int(length+delta), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return nil, fmt.Errorf("failed to mmap file at offset %d: %w", offset, err)
	}

	// data is the caller-visible slice starting at the requested offset.
	data := raw[delta:]

	// Advise the kernel about sequential access pattern
	// This helps with read-ahead optimization for Lucene's typically sequential access
	// Note: We ignore errors from madvise as it's a hint, not a requirement
	madviseSequential(raw)

	return &mmapFile{
		data:   data,
		raw:    raw,
		length: length,
		file:   f,
	}, nil
}

// unmap unmaps the file from memory.
// Munmap must be called with the pointer returned by Mmap, which is raw
// (the page-aligned allocation), not data (the trimmed caller-visible slice).
func (m *mmapFile) unmap() error {
	if m.raw == nil {
		return nil
	}

	err := syscall.Munmap(m.raw)
	m.raw = nil
	m.data = nil
	return err
}

// close is a no-op since the file handle is now shared and managed externally.
// The file is closed by MMapDirectory after all chunks are mapped.
func (m *mmapFile) close() error {
	return nil
}

// madviseSequential advises the kernel that the mapped memory will be accessed sequentially.
// This is a hint that can improve read-ahead behavior for Lucene's typical access patterns.
func madviseSequential(data []byte) {
	if len(data) == 0 {
		return
	}
	// MADV_SEQUENTIAL = 2 - Expect sequential page references
	// This tells the kernel to aggressively read ahead
	const MADV_SEQUENTIAL = 2
	syscall.Syscall(syscall.SYS_MADVISE, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), uintptr(MADV_SEQUENTIAL))
}
