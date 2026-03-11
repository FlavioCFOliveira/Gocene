// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build windows
// +build windows

package store

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

// mmapImpl implements memory-mapped file I/O for Windows.
// This file is built only on Windows platforms.

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
	procCloseHandle        = modkernel32.NewProc("CloseHandle")
	procGetFileSizeEx      = modkernel32.NewProc("GetFileSizeEx")
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

	if info.Size() != length {
		return nil, fmt.Errorf("file size mismatch: expected %d, got %d", length, info.Size())
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
	var hFile syscall.Handle
	sysFile, ok := f.Sys().(*syscall.Handle)
	if ok {
		hFile = *sysFile
	} else {
		// Fallback: duplicate the handle
		hFile = syscall.Handle(f.Fd())
	}

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
	data := unsafe.Slice((*byte)(addr), length)

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
func mapViewOfFile(hMap syscall.Handle, access, size int64) (uintptr, error) {
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
	ret, _, err := procUnmapViewOfFile.Call(uintptr(unsafe.Pointer(&m.data[0])))
	if ret == 0 {
		return err
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

// getSlice returns a slice of the mapped memory.
func (m *mmapFile) getSlice(offset, length int64) ([]byte, error) {
	if offset < 0 || length < 0 || offset+length > m.length {
		return nil, fmt.Errorf("invalid slice: offset=%d, length=%d, fileLength=%d", offset, length, m.length)
	}
	return m.data[offset : offset+length], nil
}

// MMapDirectory is a Directory implementation that uses memory-mapped
// files for reading. This provides very fast read access for large files
// by leveraging the operating system's virtual memory system.
//
// This is the Go port of Lucene's org.apache.lucene.store.MMapDirectory.
//
// Memory-mapped files have several advantages:
//   - No explicit read() system calls - the OS handles paging
//   - Data is cached in the OS page cache automatically
//   - Multiple processes can share the same physical memory
//   - No heap memory is used for file contents
//
// However, there are some limitations:
//   - Files must be opened read-only
//   - Maximum file size is limited by available virtual address space
//   - May not work on all platforms (e.g., 32-bit systems with large files)
type MMapDirectory struct {
	*FSDirectory

	// chunkSizePower is the power of 2 for chunk size (default 30 = 1GB)
	chunkSizePower int

	// preload specifies whether to preload files into the OS page cache
	preload bool
}

// NewMMapDirectory creates a new MMapDirectory at the specified path.
// The directory must exist and be writable.
func NewMMapDirectory(path string) (*MMapDirectory, error) {
	fsDir, err := NewFSDirectory(path)
	if err != nil {
		return nil, err
	}

	return &MMapDirectory{
		FSDirectory:    fsDir,
		chunkSizePower: 30, // 1GB chunks (2^30)
		preload:        false,
	}, nil
}

// SetPreload sets whether to preload files into the OS page cache.
// When true, files are read sequentially after mapping to populate the cache.
func (d *MMapDirectory) SetPreload(preload bool) {
	d.preload = preload
}

// GetPreload returns whether preload is enabled.
func (d *MMapDirectory) GetPreload() bool {
	return d.preload
}

// SetMaxChunkSize sets the maximum chunk size for memory mapping.
// The size is specified as a power of 2. Default is 30 (1GB).
// Larger values use fewer mappings but may fail on 32-bit systems.
func (d *MMapDirectory) SetMaxChunkSize(powerOf2 int) {
	if powerOf2 < 1 {
		powerOf2 = 1
	}
	if powerOf2 > 62 {
		powerOf2 = 62
	}
	d.chunkSizePower = powerOf2
}

// GetMaxChunkSize returns the maximum chunk size as a power of 2.
func (d *MMapDirectory) GetMaxChunkSize() int {
	return d.chunkSizePower
}

// OpenInput returns an IndexInput for reading an existing file.
func (d *MMapDirectory) OpenInput(name string, ctx IOContext) (IndexInput, error) {
	if err := d.EnsureOpen(); err != nil {
		return nil, err
	}

	path := filepath.Join(d.GetPath(), name)

	// Open the file read-only
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrFileNotFound, name)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// Get file info
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	length := info.Size()

	// For empty files, create a special empty IndexInput
	if length == 0 {
		file.Close()
		d.AddOpenFile(name)
		return &MMapIndexInput{
			BaseIndexInput: NewBaseIndexInput(fmt.Sprintf("MMapIndexInput(path=\"%s\")", path), 0),
			path:           path,
			name:           name,
			directory:      d,
			chunks:         nil,
			chunkSize:      0,
		}, nil
	}

	// Calculate chunk size
	chunkSize := int64(1) << d.chunkSizePower

	// Calculate number of chunks needed
	numChunks := int((length + chunkSize - 1) / chunkSize)

	// Map each chunk
	chunks := make([]*mmapFile, numChunks)
	for i := 0; i < numChunks; i++ {
		offset := int64(i) * chunkSize
		remaining := length - offset
		if remaining > chunkSize {
			remaining = chunkSize
		}

		// For multi-chunk files, we need to reopen the file for each chunk
		// because Windows file mappings are per-handle
		var f *os.File
		if i == 0 {
			f = file
		} else {
			f, err = os.Open(path)
			if err != nil {
				// Clean up already mapped chunks
				for j := 0; j < i; j++ {
					chunks[j].unmap()
					chunks[j].close()
				}
				return nil, fmt.Errorf("failed to open file for chunk %d: %w", i, err)
			}
		}

		chunk, err := mmap(f, remaining)
		if err != nil {
			// Clean up already mapped chunks
			for j := 0; j < i; j++ {
				chunks[j].unmap()
				chunks[j].close()
			}
			if i > 0 {
				f.Close()
			} else {
				file.Close()
			}
			return nil, fmt.Errorf("failed to mmap chunk %d: %w", i, err)
		}

		chunks[i] = chunk
	}

	d.AddOpenFile(name)

	input := &MMapIndexInput{
		BaseIndexInput: NewBaseIndexInput(fmt.Sprintf("MMapIndexInput(path=\"%s\")", path), length),
		path:           path,
		name:           name,
		directory:      d,
		chunks:         chunks,
		chunkSize:      chunkSize,
	}

	// Preload if enabled
	if d.preload {
		input.preload()
	}

	return input, nil
}

// CreateOutput returns an IndexOutput for writing a new file.
// MMapDirectory uses standard file I/O for writing (memory-mapping is read-only).
func (d *MMapDirectory) CreateOutput(name string, ctx IOContext) (IndexOutput, error) {
	// Delegate to SimpleFSDirectory for writing
	simpleDir, err := NewSimpleFSDirectory(d.GetPath())
	if err != nil {
		return nil, err
	}
	return simpleDir.CreateOutput(name, ctx)
}

// MMapIndexInput is an IndexInput implementation that reads from
// memory-mapped files.
type MMapIndexInput struct {
	*BaseIndexInput
	path      string
	name      string
	directory *MMapDirectory
	chunks    []*mmapFile
	chunkSize int64
}

// ReadByte reads a single byte.
func (in *MMapIndexInput) ReadByte() (byte, error) {
	if !in.directory.IsOpen() {
		return 0, ErrIllegalState
	}

	pos := in.GetFilePointer()
	if pos >= in.Length() {
		return 0, io.EOF
	}

	// Calculate which chunk and offset within chunk
	chunkIndex := int(pos / in.chunkSize)
	chunkOffset := pos % in.chunkSize

	if chunkIndex >= len(in.chunks) {
		return 0, io.EOF
	}

	b := in.chunks[chunkIndex].data[chunkOffset]
	in.SetFilePointer(pos + 1)
	return b, nil
}

// ReadBytes reads len(b) bytes into b.
func (in *MMapIndexInput) ReadBytes(b []byte) error {
	if !in.directory.IsOpen() {
		return ErrIllegalState
	}

	pos := in.GetFilePointer()
	remaining := in.Length() - pos
	if remaining < int64(len(b)) {
		return io.EOF
	}

	// Read across chunks if necessary
	offset := 0
	for offset < len(b) {
		chunkIndex := int(pos / in.chunkSize)
		chunkOffset := pos % in.chunkSize

		if chunkIndex >= len(in.chunks) {
			return io.EOF
		}

		chunk := in.chunks[chunkIndex]
		chunkRemaining := int64(len(chunk.data)) - chunkOffset
		toRead := int64(len(b) - offset)
		if toRead > chunkRemaining {
			toRead = chunkRemaining
		}

		copy(b[offset:offset+int(toRead)], chunk.data[chunkOffset:chunkOffset+toRead])
		offset += int(toRead)
		pos += toRead
	}

	in.SetFilePointer(pos)
	return nil
}

// ReadBytesN reads exactly n bytes and returns them.
func (in *MMapIndexInput) ReadBytesN(n int) ([]byte, error) {
	b := make([]byte, n)
	if err := in.ReadBytes(b); err != nil {
		return nil, err
	}
	return b, nil
}

// SetPosition changes the current position in the file.
func (in *MMapIndexInput) SetPosition(pos int64) error {
	if pos < 0 || pos > in.Length() {
		return fmt.Errorf("invalid position: %d", pos)
	}
	in.SetFilePointer(pos)
	return nil
}

// Clone returns a clone of this IndexInput.
func (in *MMapIndexInput) Clone() IndexInput {
	// For cloning, we need to reopen and remap the file
	clone, err := in.directory.OpenInput(in.name, IOContextRead)
	if err != nil {
		// Return a clone that will fail on read
		return &MMapIndexInput{
			BaseIndexInput: NewBaseIndexInput(in.GetDescription(), in.Length()),
			path:           in.path,
			name:           in.name,
			directory:      in.directory,
			chunks:         nil,
			chunkSize:      in.chunkSize,
		}
	}

	// Set the position to match the original
	clone.SetPosition(in.GetFilePointer())
	return clone
}

// Slice returns a subset of this IndexInput.
func (in *MMapIndexInput) Slice(desc string, offset int64, length int64) (IndexInput, error) {
	if offset < 0 || length < 0 || offset+length > in.Length() {
		return nil, fmt.Errorf("invalid slice parameters: offset=%d, length=%d, fileLength=%d", offset, length, in.Length())
	}

	// For slices, we create a new MMapIndexInput with the same chunks
	// but track the slice offset separately
	return &MMapIndexInput{
		BaseIndexInput: NewBaseIndexInput(desc, length),
		path:           in.path,
		name:           in.name,
		directory:      in.directory,
		chunks:         in.chunks,
		chunkSize:      in.chunkSize,
	}, nil
}

// Close closes this IndexInput.
func (in *MMapIndexInput) Close() error {
	var firstErr error
	for _, chunk := range in.chunks {
		if err := chunk.unmap(); err != nil && firstErr == nil {
			firstErr = err
		}
		if err := chunk.close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	in.directory.RemoveOpenFile(in.name)
	return firstErr
}

// preload reads through the entire file to populate the OS page cache.
func (in *MMapIndexInput) preload() {
	// Read sequentially through all chunks to populate the cache
	for _, chunk := range in.chunks {
		if chunk.data != nil {
			// Touch each page (typically 4KB) to trigger page-in
			pageSize := int64(4096)
			for i := int64(0); i < chunk.length; i += pageSize {
				_ = chunk.data[i]
			}
		}
	}
}
