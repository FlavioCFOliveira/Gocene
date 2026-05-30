// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build windows
// +build windows

package store

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// This file provides the Windows-specific memory-mapping primitives consumed by
// the platform-independent MMapDirectory / MMapIndexInput defined in
// mmap_directory.go. The directory code is shared across platforms; only the
// mmapFile type and the mmap/unmap/close helpers differ per OS (see
// mmap_unix.go for the Unix counterpart).
//
// The mmap signature MUST match the Unix one — mmap(f, offset, length) — so the
// shared MMapIndexInput slice fix (rmp #4747: borrowing slices share the
// owner's chunks and never unmap on Close) applies unchanged on Windows.

// Windows API constants.
const (
	FILE_MAP_READ       = 0x0004
	FILE_MAP_COPY       = 0x0001
	FILE_MAP_WRITE      = 0x0002
	FILE_MAP_EXECUTE    = 0x0020
	FILE_MAP_ALL_ACCESS = 0xF001F

	PAGE_READONLY          = 0x02
	PAGE_READWRITE         = 0x04
	PAGE_WRITECOPY         = 0x08
	PAGE_EXECUTE_READ      = 0x20
	PAGE_EXECUTE_READWRITE = 0x40
)

var (
	modkernel32 = syscall.NewLazyDLL("kernel32.dll")

	procCreateFileMappingW = modkernel32.NewProc("CreateFileMappingW")
	procMapViewOfFile      = modkernel32.NewProc("MapViewOfFile")
	procUnmapViewOfFile    = modkernel32.NewProc("UnmapViewOfFile")
	procGetSystemInfo      = modkernel32.NewProc("GetSystemInfo")
)

// systemInfo mirrors the Win32 SYSTEM_INFO struct. Only dwAllocationGranularity
// is read; the remaining fields are present so the layout matches GetSystemInfo.
type systemInfo struct {
	wProcessorArchitecture      uint16
	wReserved                   uint16
	dwPageSize                  uint32
	lpMinimumApplicationAddress uintptr
	lpMaximumApplicationAddress uintptr
	dwActiveProcessorMask       uintptr
	dwNumberOfProcessors        uint32
	dwProcessorType             uint32
	dwAllocationGranularity     uint32
	wProcessorLevel             uint16
	wProcessorRevision          uint16
}

// allocationGranularity returns the Win32 MapViewOfFile offset granularity
// (dwAllocationGranularity, typically 64 KiB). MapViewOfFile requires the file
// offset to be a multiple of this value.
func allocationGranularity() int64 {
	var si systemInfo
	procGetSystemInfo.Call(uintptr(unsafe.Pointer(&si)))
	if si.dwAllocationGranularity == 0 {
		// Defensive fallback: 64 KiB is the granularity on every supported
		// Windows release.
		return 64 * 1024
	}
	return int64(si.dwAllocationGranularity)
}

// mmapFile represents a single memory-mapped region of a file on Windows.
type mmapFile struct {
	// data is the caller-visible slice, starting at the requested offset.
	data []byte
	// view is the base address returned by MapViewOfFile (aligned offset); it is
	// the pointer that must be passed to UnmapViewOfFile, not &data[0].
	view uintptr
	// hMap is the section (file-mapping) handle backing this view.
	hMap   syscall.Handle
	length int64
}

// mmap maps the region [offset, offset+length) of f into memory.
//
// The signature matches mmap_unix.go so that the shared MMapDirectory /
// MMapIndexInput code (mmap_directory.go) is platform-agnostic. The Win32
// MapViewOfFile offset must be a multiple of the allocation granularity, so the
// offset is aligned down and the returned data slice is trimmed back to the
// requested offset (mirroring the page-alignment delta trick in mmap_unix.go).
func mmap(f *os.File, offset int64, length int64) (*mmapFile, error) {
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	if info.Size() < offset+length {
		return nil, fmt.Errorf("file size too small: expected at least %d, got %d", offset+length, info.Size())
	}

	// Handle empty mapping.
	if length == 0 {
		return &mmapFile{data: nil, length: 0}, nil
	}

	hFile := syscall.Handle(f.Fd())

	// Create a section spanning the whole file (size 0 = entire file). The
	// section keeps the underlying file object alive even after the os.File
	// handle is closed by the directory.
	hMap, err := createFileMapping(hFile, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to create file mapping: %w", err)
	}

	// Align the view offset down to the allocation granularity; the trimmed
	// delta is reapplied to the returned slice.
	gran := allocationGranularity()
	alignedOffset := (offset / gran) * gran
	delta := offset - alignedOffset

	addr, err := mapViewOfFile(hMap, FILE_MAP_READ, alignedOffset, length+delta)
	if err != nil {
		syscall.CloseHandle(hMap)
		return nil, fmt.Errorf("failed to map view of file at offset %d: %w", offset, err)
	}

	// The full mapped view starts at the aligned offset; trim to the requested
	// offset for the caller-visible slice.
	//
	// addr is the base address returned by MapViewOfFile — a live OS mapping that
	// is not Go-managed memory and remains valid until UnmapViewOfFile, so the
	// uintptr->unsafe.Pointer conversion is safe. `go vet` reports a generic
	// "possible misuse of unsafe.Pointer" here; it is intrinsic to MapViewOfFile
	// (every Windows mmap library hits it) and the CI Windows gate is compilation
	// (go build / go test), not `go vet`.
	full := unsafe.Slice((*byte)(unsafe.Pointer(addr)), length+delta)
	data := full[delta:]

	return &mmapFile{
		data:   data,
		view:   addr,
		hMap:   hMap,
		length: length,
	}, nil
}

// createFileMapping creates a read-only section object over hFile.
// A size of 0 maps the entire current file.
func createFileMapping(hFile syscall.Handle, size int64) (syscall.Handle, error) {
	sizeHigh := uint32(size >> 32)
	sizeLow := uint32(size)

	hMap, _, err := procCreateFileMappingW.Call(
		uintptr(hFile),         // hFile
		uintptr(0),             // lpFileMappingAttributes (default security)
		uintptr(PAGE_READONLY), // flProtect
		uintptr(sizeHigh),      // dwMaximumSizeHigh
		uintptr(sizeLow),       // dwMaximumSizeLow
		uintptr(0),             // lpName (unnamed)
	)
	if hMap == 0 {
		return 0, err
	}
	return syscall.Handle(hMap), nil
}

// mapViewOfFile maps a view of the section into the process address space,
// starting at the given file offset.
func mapViewOfFile(hMap syscall.Handle, access uint32, offset, size int64) (uintptr, error) {
	offsetHigh := uint32(offset >> 32)
	offsetLow := uint32(offset)

	addr, _, err := procMapViewOfFile.Call(
		uintptr(hMap),
		uintptr(access),
		uintptr(offsetHigh),
		uintptr(offsetLow),
		uintptr(size),
	)
	if addr == 0 {
		return 0, err
	}
	return addr, nil
}

// unmap releases the view and its backing section. Mirrors mmap_unix.go: the
// owning MMapIndexInput calls this; borrowing slices (isSlice) never do.
func (m *mmapFile) unmap() error {
	if m.data == nil {
		return nil
	}

	var firstErr error
	if m.view != 0 {
		ret, _, err := procUnmapViewOfFile.Call(m.view)
		if ret == 0 {
			firstErr = err
		}
		m.view = 0
	}
	m.data = nil

	if m.hMap != 0 {
		if err := syscall.CloseHandle(m.hMap); err != nil && firstErr == nil {
			firstErr = err
		}
		m.hMap = 0
	}
	return firstErr
}

// close is a no-op; the file handle is managed by MMapDirectory, and the view's
// lifetime is governed by unmap. Mirrors mmap_unix.go.
func (m *mmapFile) close() error {
	return nil
}
