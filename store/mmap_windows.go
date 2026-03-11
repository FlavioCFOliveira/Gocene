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

// Windows API constants
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
)

// mmapFile represents a memory-mapped file on Windows.
type mmapFile struct {
	data       []byte
	length     int64
	hFile      syscall.Handle
	hMap       syscall.Handle
	needsClose bool
}

// mmap maps the file into memory using Windows API.
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
			data:       nil,
			length:     0,
			hFile:      0,
			hMap:       0,
			needsClose: false,
		}, nil
	}

	// Get Windows file handle
	hFile := syscall.Handle(f.Fd())

	// Create file mapping
	hMap, err := createFileMapping(hFile, length)
	if err != nil {
		return nil, fmt.Errorf("failed to create file mapping: %w", err)
	}

	// Map view of file
	addr, err := mapViewOfFile(hMap, FILE_MAP_READ, length)
	if err != nil {
		syscall.CloseHandle(hMap)
		return nil, fmt.Errorf("failed to map view of file: %w", err)
	}

	// Create byte slice from mapped memory
	// This is safe because we're using read-only mapping
	data := unsafe.Slice((*byte)(unsafe.Pointer(addr)), length)

	return &mmapFile{
		data:       data,
		length:     length,
		hFile:      hFile,
		hMap:       hMap,
		needsClose: false, // The file handle is owned by os.File
	}, nil
}

// createFileMapping creates a file mapping object.
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

// mapViewOfFile maps a view of the file into the process's address space.
func mapViewOfFile(hMap syscall.Handle, access uint32, size int64) (uintptr, error) {
	offsetHigh := uint32(0)
	offsetLow := uint32(0)

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

// unmap unmaps the file from memory.
func (m *mmapFile) unmap() error {
	if m.data == nil {
		return nil
	}

	// Unmap the view
	if len(m.data) > 0 {
		ret, _, err := procUnmapViewOfFile.Call(uintptr(unsafe.Pointer(&m.data[0])))
		if ret == 0 {
			return err
		}
	}

	m.data = nil

	// Close the file mapping handle
	if m.hMap != 0 {
		syscall.CloseHandle(m.hMap)
		m.hMap = 0
	}

	// Close the file handle only if we opened it
	if m.needsClose && m.hFile != 0 {
		syscall.CloseHandle(m.hFile)
		m.hFile = 0
	}

	return nil
}

// close closes the underlying file.
func (m *mmapFile) close() error {
	// Nothing to do on Windows - the file is closed when unmapped
	return nil
}
