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
)

// mmapFile represents a memory-mapped file on Unix systems.
type mmapFile struct {
	data   []byte
	length int64
	file   *os.File
}

// mmap maps the file into memory using Unix syscalls.
func mmap(f *os.File, length int64) (*mmapFile, error) {
	// Get file info to check size
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	if info.Size() < length {
		return nil, fmt.Errorf("file size too small: expected at least %d, got %d", length, info.Size())
	}

	// Handle empty file
	if length == 0 {
		return &mmapFile{
			data:   nil,
			length: 0,
			file:   f,
		}, nil
	}

	// Memory map the file
	// PROT_READ: pages may be read
	// MAP_SHARED: share this mapping
	data, err := syscall.Mmap(int(f.Fd()), 0, int(length), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return nil, fmt.Errorf("failed to mmap file: %w", err)
	}

	return &mmapFile{
		data:   data,
		length: length,
		file:   f,
	}, nil
}

// unmap unmaps the file from memory.
func (m *mmapFile) unmap() error {
	if m.data == nil {
		return nil
	}

	err := syscall.Munmap(m.data)
	m.data = nil
	return err
}

// close closes the underlying file.
func (m *mmapFile) close() error {
	if m.file != nil {
		return m.file.Close()
	}
	return nil
}
