// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

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

	// writeMu guards lazy creation of writeDelegate.
	writeMu sync.Mutex

	// writeDelegate is a single SimpleFSDirectory created lazily on the first
	// CreateOutput call and reused for every subsequent write. Memory mapping
	// is read-only in Lucene's MMapDirectory, so writes are delegated to a
	// plain file-I/O directory. Caching a single delegate avoids leaking a
	// fresh SimpleFSDirectory (each with its own openFiles map) per call.
	writeDelegate *SimpleFSDirectory
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
	if err := validateFileName(name); err != nil {
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
	// Note: We reuse the same file handle for all chunks, which is safe
	// because each chunk maps a different non-overlapping region.
	// This optimization reduces file descriptor usage.
	chunks := make([]*mmapFile, numChunks)
	for i := 0; i < numChunks; i++ {
		offset := int64(i) * chunkSize
		remaining := length - offset
		if remaining > chunkSize {
			remaining = chunkSize
		}

		chunk, err := mmap(file, offset, remaining)
		if err != nil {
			// Clean up already mapped chunks
			for j := 0; j < i; j++ {
				chunks[j].unmap()
			}
			file.Close()
			return nil, fmt.Errorf("failed to mmap chunk %d: %w", i, err)
		}

		chunks[i] = chunk
	}

	// Close the file handle - the mmap keeps the file open internally
	// This is safe because the kernel keeps the file open until all mappings are unmapped
	file.Close()

	d.AddOpenFile(name)

	input := &MMapIndexInput{
		BaseIndexInput: NewBaseIndexInput(fmt.Sprintf("MMapIndexInput(path=\"%s\")", path), length),
		path:           path,
		name:           name,
		directory:      d,
		chunks:         chunks,
		chunkSize:      chunkSize,
		sliceOffset:    0,
	}

	// Preload if enabled
	if d.preload {
		input.preload()
	}

	return input, nil
}

// writeDelegateLocked returns the cached SimpleFSDirectory used for writes,
// creating it on first use. Callers must ensure the directory is open before
// invoking it. The delegate is shared across all CreateOutput calls so that a
// single openFiles map tracks every output handle.
func (d *MMapDirectory) writeDelegateLocked() (*SimpleFSDirectory, error) {
	d.writeMu.Lock()
	defer d.writeMu.Unlock()

	if d.writeDelegate == nil {
		simpleDir, err := NewSimpleFSDirectory(d.GetPath())
		if err != nil {
			return nil, err
		}
		d.writeDelegate = simpleDir
	}
	return d.writeDelegate, nil
}

// CreateOutput returns an IndexOutput for writing a new file.
// MMapDirectory uses standard file I/O for writing (memory-mapping is read-only),
// delegating to a single cached SimpleFSDirectory reused across all calls.
func (d *MMapDirectory) CreateOutput(name string, ctx IOContext) (IndexOutput, error) {
	if err := d.EnsureOpen(); err != nil {
		return nil, err
	}

	delegate, err := d.writeDelegateLocked()
	if err != nil {
		return nil, err
	}
	// SimpleFSDirectory.CreateOutput validates the file name (path-traversal
	// guard from rmp #4719), so it is intentionally not duplicated here.
	return delegate.CreateOutput(name, ctx)
}

// Close releases all resources associated with this directory, including the
// cached write delegate created lazily by CreateOutput.
func (d *MMapDirectory) Close() error {
	if !d.IsOpen() {
		return nil
	}

	d.writeMu.Lock()
	delegate := d.writeDelegate
	d.writeDelegate = nil
	d.writeMu.Unlock()

	var firstErr error
	if delegate != nil {
		if err := delegate.Close(); err != nil {
			firstErr = err
		}
	}

	if err := d.FSDirectory.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
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
	// sliceOffset is the file-absolute start of this view.
	sliceOffset int64
	// isSlice marks a borrowing view created by Slice: it shares the owner's
	// mmap chunks and must NOT unmap them on Close — only the owning input (the
	// one returned by OpenInput, or a Clone) unmaps. Without this, closing a
	// sub-file slice of a compound (.cfs) file would munmap the shared mapping
	// and corrupt every other live slice of it (rmp #4747). SimpleFS/NIOFS avoid
	// this by reopening the file per slice; MMap shares one mapping, so the
	// owner/borrower distinction is required.
	isSlice bool
}

// ReadByte reads a single byte.
func (in *MMapIndexInput) ReadByte() (byte, error) {
	if err := in.ensureChunksOpen(); err != nil {
		return 0, err
	}
	if !in.directory.IsOpen() {
		return 0, ErrIllegalState
	}

	pos := in.GetFilePointer()
	if pos >= in.Length() {
		return 0, io.EOF
	}

	// Add slice offset to get the actual position in the underlying file
	actualPos := in.sliceOffset + pos

	// Calculate which chunk and offset within chunk
	chunkIndex := int(actualPos / in.chunkSize)
	chunkOffset := actualPos % in.chunkSize

	if chunkIndex >= len(in.chunks) {
		return 0, io.EOF
	}

	chunk := in.chunks[chunkIndex]
	if chunk.data == nil {
		// Chunk has been unmapped (e.g. the owning input was closed).
		return 0, ErrIllegalState
	}

	b := chunk.data[chunkOffset]
	in.SetFilePointer(pos + 1)
	return b, nil
}

// ReadBytes reads len(b) bytes into b.
func (in *MMapIndexInput) ReadBytes(b []byte) error {
	if err := in.ensureChunksOpen(); err != nil {
		return err
	}
	if !in.directory.IsOpen() {
		return ErrIllegalState
	}

	pos := in.GetFilePointer()
	remaining := in.Length() - pos
	if remaining < int64(len(b)) {
		return io.EOF
	}

	// Add slice offset to get the actual position in the underlying file
	actualPos := in.sliceOffset + pos

	// Read across chunks if necessary
	offset := 0
	for offset < len(b) {
		chunkIndex := int(actualPos / in.chunkSize)
		chunkOffset := actualPos % in.chunkSize

		if chunkIndex >= len(in.chunks) {
			return io.EOF
		}

		chunk := in.chunks[chunkIndex]
		if chunk.data == nil {
			// Chunk has been unmapped (e.g. the owning input was closed).
			return ErrIllegalState
		}
		chunkRemaining := int64(len(chunk.data)) - chunkOffset
		if chunkRemaining <= 0 {
			return io.EOF
		}
		toRead := int64(len(b) - offset)
		if toRead > chunkRemaining {
			toRead = chunkRemaining
		}

		copy(b[offset:offset+int(toRead)], chunk.data[chunkOffset:chunkOffset+toRead])
		offset += int(toRead)
		actualPos += toRead
	}

	in.SetFilePointer(pos + int64(len(b)))
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

// ReadShort reads a 16-bit little-endian value to match Lucene 10.x
// DataInput.readShort (low byte first). See rmp #4786.
func (in *MMapIndexInput) ReadShort() (int16, error) {
	b, err := in.ReadBytesN(2)
	if err != nil {
		return 0, err
	}
	return int16(uint16(b[0]) | uint16(b[1])<<8), nil
}

// ReadInt reads a 32-bit little-endian value to match Lucene 10.x
// DataInput.readInt (low byte first). See rmp #4786.
func (in *MMapIndexInput) ReadInt() (int32, error) {
	b, err := in.ReadBytesN(4)
	if err != nil {
		return 0, err
	}
	return int32(uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24), nil
}

// ReadLong reads a 64-bit little-endian value to match Lucene 10.x
// DataInput.readLong (low byte first). See rmp #4786.
func (in *MMapIndexInput) ReadLong() (int64, error) {
	b, err := in.ReadBytesN(8)
	if err != nil {
		return 0, err
	}
	return int64(uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
		uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56), nil
}

// ReadString reads a string.
func (in *MMapIndexInput) ReadString() (string, error) {
	return ReadString(in)
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
// The clone starts at position 0 and is independent of the original.
// If this input is a slice, the clone has the same slice bounds.
func (in *MMapIndexInput) Clone() IndexInput {
	// Reopen the file so the clone is fully independent.
	full, err := in.directory.OpenInput(in.name, IOContextRead)
	if err != nil {
		// Return a clone that will fail on read.
		return &MMapIndexInput{
			BaseIndexInput: NewBaseIndexInput(in.GetDescription(), in.Length()),
			path:           in.path,
			name:           in.name,
			directory:      in.directory,
			chunks:         nil,
			chunkSize:      in.chunkSize,
			sliceOffset:    in.sliceOffset,
		}
	}

	// If this input is a slice, restrict the clone to the same region so that
	// reads start at the correct logical offset within the file.
	if in.sliceOffset == 0 {
		return full
	}
	// The clone is independent and is the sole holder of full's freshly-reopened
	// mmap, so it OWNS that mapping (isSlice=false) and views it at this input's
	// absolute slice offset. We must not return a borrowing Slice here: borrows
	// never unmap, which would leak full's mapping (rmp #4747).
	fm, ok := full.(*MMapIndexInput)
	if !ok {
		full.Close()
		return &MMapIndexInput{
			BaseIndexInput: NewBaseIndexInput(in.GetDescription(), in.Length()),
			path:           in.path,
			name:           in.name,
			directory:      in.directory,
			chunks:         nil,
			chunkSize:      in.chunkSize,
			sliceOffset:    in.sliceOffset,
		}
	}
	return &MMapIndexInput{
		BaseIndexInput: NewBaseIndexInput(in.GetDescription(), in.Length()),
		path:           fm.path,
		name:           fm.name,
		directory:      fm.directory,
		chunks:         fm.chunks,
		chunkSize:      fm.chunkSize,
		sliceOffset:    in.sliceOffset,
		isSlice:        false,
	}
}

// Slice returns a subset of this IndexInput.
func (in *MMapIndexInput) Slice(desc string, offset int64, length int64) (IndexInput, error) {
	if offset < 0 || length < 0 || offset+length > in.Length() {
		return nil, fmt.Errorf("invalid slice parameters: offset=%d, length=%d, fileLength=%d", offset, length, in.Length())
	}

	// A slice BORROWS the owner's mmap chunks (shared by pointer) and tracks its
	// own absolute offset. It is marked isSlice so Close does not unmap the
	// shared mapping — only the owning input does (rmp #4747).
	return &MMapIndexInput{
		BaseIndexInput: NewBaseIndexInput(desc, length),
		path:           in.path,
		name:           in.name,
		directory:      in.directory,
		chunks:         in.chunks,
		chunkSize:      in.chunkSize,
		sliceOffset:    in.sliceOffset + offset,
		isSlice:        true,
	}, nil
}

// ensureChunksOpen returns an error if the chunks slice is nil, which can happen
// when Clone() fails to open the file. Without this guard, read methods would
// panic on nil slice access.
func (in *MMapIndexInput) ensureChunksOpen() error {
	if in.chunks == nil {
		return fmt.Errorf("MMapIndexInput: chunks are nil (clone of %q failed to open): %w", in.name, ErrIllegalState)
	}
	return nil
}

// Close closes this IndexInput. A borrowing slice (isSlice) shares the owner's
// mmap chunks and must NOT unmap them — doing so would corrupt the owner and
// every sibling slice (rmp #4747); only the owning input unmaps.
func (in *MMapIndexInput) Close() error {
	if in.chunks == nil {
		in.directory.RemoveOpenFile(in.name)
		return nil
	}
	if in.isSlice {
		in.directory.RemoveOpenFile(in.name)
		return nil
	}
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
