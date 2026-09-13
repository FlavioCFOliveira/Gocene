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

	"github.com/FlavioCFOliveira/Gocene/spi"
)

// DefaultMaxChunkSize is the default maximum chunk size for memory mapping.
// For 64-bit platforms, this is typically 16 GiB.
const DefaultMaxChunkSize int64 = 1 << 34

// MMapDirectory is a Directory implementation that uses mmap for reading,
// and SimpleFSDirectory for writing.
//
// This is the Go port of Lucene's org.apache.lucene.store.MMapDirectory.
type MMapDirectory struct {
	*FSDirectory

	// readAdvice configures the read advice based on the filename and IOContext.
	readAdvice func(string, IOContext) *ReadAdvice

	// preload configures whether files should be preloaded upon opening.
	preload func(string, IOContext) bool

	// groupingFunction configures the grouping of files.
	groupingFunction func(string) (string, bool)

	// chunkSizePower is the power of 2 for the maximum chunk size.
	chunkSizePower int
}

// NewMMapDirectory creates a new MMapDirectory for the named location.
func NewMMapDirectory(path string) (*MMapDirectory, error) {
	return NewMMapDirectoryWithChunkSize(path, DefaultMaxChunkSize)
}

// NewMMapDirectoryWithChunkSize creates a new MMapDirectory for the named location,
// specifying the maximum chunk size used for memory mapping.
func NewMMapDirectoryWithChunkSize(path string, maxChunkSize int64) (*MMapDirectory, error) {
	fsDir, err := NewFSDirectory(path)
	if err != nil {
		return nil, err
	}

	if maxChunkSize <= 0 {
		return nil, fmt.Errorf("maximum chunk size for mmap must be >0")
	}

	// Round down to the nearest power of 2, as Lucene does.
	power := 0
	for (int64(1) << (power + 1)) <= maxChunkSize {
		power++
	}

	d := &MMapDirectory{
		FSDirectory:      fsDir,
		chunkSizePower:   power,
		readAdvice:       nil, // Default: no advice
		preload:          func(string, IOContext) bool { return false },
		groupingFunction: groupBySegment,
	}

	return d, nil
}

// SetPreload configures which files to preload in physical memory upon opening.
func (d *MMapDirectory) SetPreload(preload func(string, IOContext) bool) {
	d.preload = preload
}

// SetReadAdvice configures ReadAdvice for certain files.
func (d *MMapDirectory) SetReadAdvice(toReadAdvice func(string, IOContext) *ReadAdvice) {
	d.readAdvice = toReadAdvice
}

// SetGroupingFunction configures a grouping function for files.
func (d *MMapDirectory) SetGroupingFunction(groupingFunction func(string) (string, bool)) {
	d.groupingFunction = groupingFunction
}

// GetMaxChunkSize returns the current mmap chunk size.
func (d *MMapDirectory) GetMaxChunkSize() int64 {
	return int64(1) << d.chunkSizePower
}

// OpenInput creates an IndexInput for the file with the given name.
func (d *MMapDirectory) OpenInput(name string, ctx IOContext) (IndexInput, error) {
	if err := d.EnsureOpen(); err != nil {
		return nil, err
	}
	if err := validateFileName(name); err != nil {
		return nil, err
	}

	path := filepath.Join(d.GetPath(), name)

	// Determine read advice
	advice := ReadAdviceNormal
	if d.readAdvice != nil {
		if a := d.readAdvice(name, ctx); a != nil {
			advice = *a
		}
	}

	// Use a provider-like logic to open the input.
	return d.openMMapInput(path, name, ctx, advice)
}

func (d *MMapDirectory) openMMapInput(path, name string, ctx IOContext, advice ReadAdvice) (IndexInput, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrFileNotFound, name)
		}
		return nil, fmt.Errorf("failed to open file for mmap: %w", err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	length := info.Size()
	if length == 0 {
		file.Close()
		d.AddOpenFile(name)
		in := &MMapIndexInput{
			BaseIndexInput: NewBaseIndexInput(fmt.Sprintf("MMapIndexInput(path=\"%s\")", path), 0),
			path:           path,
			name:           name,
			directory:      d,
			chunks:         nil,
			chunkSize:      0,
		}
		in.Core = in
		return in, nil
	}

	chunkSize := int64(1) << d.chunkSizePower
	numChunks := int((length + chunkSize - 1) / chunkSize)

	chunks := make([]*mmapFile, numChunks)
	for i := 0; i < numChunks; i++ {
		offset := int64(i) * chunkSize
		remaining := length - offset
		if remaining > chunkSize {
			remaining = chunkSize
		}

		chunk, err := mmap(file, offset, remaining)
		if err != nil {
			for j := 0; j < i; j++ {
				chunks[j].unmap()
			}
			file.Close()
			return nil, fmt.Errorf("failed to mmap chunk %d: %w", i, err)
		}
		chunks[i] = chunk
	}

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

	if d.preload != nil && d.preload(name, ctx) {
		input.preload()
	}

	input.Core = input
		return input, nil
}

// CreateOutput returns an IndexOutput for writing a new file.
// Memory-mapping is read-only, so we delegate writes to a SimpleFSDirectory.
func (d *MMapDirectory) CreateOutput(name string, ctx IOContext) (IndexOutput, error) {
	if err := d.EnsureOpen(); err != nil {
		return nil, err
	}

	// We create a temporary SimpleFSDirectory to handle the output.
	// In Lucene, this is cached.
	delegate, err := NewSimpleFSDirectory(d.GetPath())
	if err != nil {
		return nil, err
	}
	// Note: SimpleFSDirectory.CreateOutput handles filename validation.
	return delegate.CreateOutput(name, ctx)
}

// Close releases all resources associated with this directory.
func (d *MMapDirectory) Close() error {
	if !d.IsOpen() {
		return nil
	}
	d.MarkClosed()
	return d.FSDirectory.Close()
}

// groupBySegment is the default grouping function.
func groupBySegment(filename string) (string, bool) {
	// Simplification of Lucene's GROUP_BY_SEGMENT.
	// We use store.ParseSegmentName to get the segment ID.
	seg := ParseSegmentName(filename)
	if seg == "" {
		return "", false
	}
	// Remove leading underscore if present
	if len(seg) > 0 && seg[0] == '_' {
		seg = seg[1:]
	}
	return seg, true
}

// MMapIndexInput is an IndexInput implementation that reads from memory-mapped files.
type MMapIndexInput struct {
	*BaseIndexInput
	spi.BaseDataInput
	path      string
	name      string
	directory *MMapDirectory
	chunks    []*mmapFile
	chunkSize int64
	mu        sync.RWMutex
	sliceOffset int64
	isSlice     bool
}

func (in *MMapIndexInput) ReadByte() (byte, error) {
	in.mu.RLock()
	defer in.mu.RUnlock()

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

	actualPos := in.sliceOffset + pos
	chunkIndex := int(actualPos / in.chunkSize)
	chunkOffset := actualPos % in.chunkSize

	if chunkIndex >= len(in.chunks) {
		return 0, io.EOF
	}

	chunk := in.chunks[chunkIndex]
	if chunk.data == nil {
		return 0, ErrIllegalState
	}

	b := chunk.data[chunkOffset]
	in.SetFilePointer(pos + 1)
	return b, nil
}

func (in *MMapIndexInput) ReadBytes(b []byte, offset, length int) error {
	in.mu.RLock()
	defer in.mu.RUnlock()

	if err := in.ensureChunksOpen(); err != nil {
		return err
	}
	if !in.directory.IsOpen() {
		return ErrIllegalState
	}

	pos := in.GetFilePointer()
	remaining := in.Length() - pos
	if remaining < int64(length) {
		return io.EOF
	}

	actualPos := in.sliceOffset + pos
	currentOffset := offset
	for currentOffset < length {
		chunkIndex := int(actualPos / in.chunkSize)
		chunkOffset := actualPos % in.chunkSize

		if chunkIndex >= len(in.chunks) {
			return io.EOF
		}

		chunk := in.chunks[chunkIndex]
		if chunk.data == nil {
			return ErrIllegalState
		}
		chunkRemaining := int64(len(chunk.data)) - chunkOffset
		if chunkRemaining <= 0 {
			return io.EOF
		}
		toRead := int64(length - currentOffset)
		if toRead > chunkRemaining {
			toRead = chunkRemaining
		}

		copy(b[currentOffset:currentOffset+int(toRead)], chunk.data[chunkOffset:chunkOffset+toRead])
		currentOffset += int(toRead)
		actualPos += toRead
	}

	in.SetFilePointer(pos + int64(length))
	return nil
}

func (in *MMapIndexInput) ReadBytesN(n int) ([]byte, error) {
	b := make([]byte, n)
	if err := in.ReadBytes(b, 0, n); err != nil {
		return nil, err
	}
	return b, nil
}

func (in *MMapIndexInput) ReadShort() (int16, error) {
	b, err := in.ReadBytesN(2)
	if err != nil {
		return 0, err
	}
	return int16(uint16(b[0]) | uint16(b[1])<<8), nil
}

func (in *MMapIndexInput) ReadInt() (int32, error) {
	b, err := in.ReadBytesN(4)
	if err != nil {
		return 0, err
	}
	return int32(uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24), nil
}

func (in *MMapIndexInput) ReadLong() (int64, error) {
	b, err := in.ReadBytesN(8)
	if err != nil {
		return 0, err
	}
	return int64(uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
		uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56), nil
}

func (in *MMapIndexInput) SetPosition(pos int64) error {
	if pos < 0 || pos > in.Length() {
		return fmt.Errorf("invalid position: %d", pos)
	}
	in.SetFilePointer(pos)
	return nil
}

func (in *MMapIndexInput) Clone() IndexInput {
	full, err := in.directory.OpenInput(in.name, IOContextRead)
	if err != nil {
		broken := &MMapIndexInput{
			BaseIndexInput: NewBaseIndexInput(in.GetDescription(), in.Length()),
			path:           in.path,
			name:           in.name,
			directory:      in.directory,
			chunks:         nil,
			chunkSize:      in.chunkSize,
			sliceOffset:    in.sliceOffset,
		}
		broken.Core = broken
		return broken
	}

	if in.sliceOffset == 0 {
		return full
	}

	fm, ok := full.(*MMapIndexInput)
	if !ok {
		full.Close()
		broken := &MMapIndexInput{
			BaseIndexInput: NewBaseIndexInput(in.GetDescription(), in.Length()),
			path:           in.path,
			name:           in.name,
			directory:      in.directory,
			chunks:         nil,
			chunkSize:      in.chunkSize,
			sliceOffset:    in.sliceOffset,
		}
		broken.Core = broken
		return broken
	}
	// Every DataInput-derived reader (readVInt, readString, ...) dispatches
	// through Core; a clone whose Core is unset would nil-panic on the first one.
	clone := &MMapIndexInput{
		BaseIndexInput: NewBaseIndexInput(in.GetDescription(), in.Length()),
		path:           fm.path,
		name:           fm.name,
		directory:      fm.directory,
		chunks:         fm.chunks,
		chunkSize:      fm.chunkSize,
		sliceOffset:    in.sliceOffset,
		isSlice:        false,
	}
	clone.Core = clone
	return clone
}

func (in *MMapIndexInput) Slice(desc string, offset int64, length int64) (IndexInput, error) {
	if offset < 0 || length < 0 || offset+length > in.Length() {
		return nil, fmt.Errorf("invalid slice parameters: offset=%d, length=%d, fileLength=%d", offset, length, in.Length())
	}

	slice := &MMapIndexInput{
		BaseIndexInput: NewBaseIndexInput(desc, length),
		path:           in.path,
		name:           in.name,
		directory:      in.directory,
		chunks:         in.chunks,
		chunkSize:      in.chunkSize,
		sliceOffset:    in.sliceOffset + offset,
		isSlice:        true,
	}
	slice.Core = slice
	return slice, nil
}

func (in *MMapIndexInput) ensureChunksOpen() error {
	if in.chunks == nil && in.Length() > 0 {
		return fmt.Errorf("MMapIndexInput: chunks are nil (clone of %q failed to open): %w", in.name, ErrIllegalState)
	}
	return nil
}

func (in *MMapIndexInput) SkipBytes(n int64) error {
	return in.BaseIndexInput.SkipBytes(n)
}

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

func (in *MMapIndexInput) preload() {
	for _, chunk := range in.chunks {
		if chunk.data != nil {
			pageSize := int64(4096)
			for i := int64(0); i < int64(len(chunk.data)); i += pageSize {
				_ = chunk.data[i]
			}
		}
	}
}
