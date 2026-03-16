// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// NIOFSDirectory is a Directory implementation that uses buffered I/O
// for reading and writing files.
//
// This is the Go port of Lucene's org.apache.lucene.store.NIOFSDirectory.
// Unlike SimpleFSDirectory which uses raw file I/O, NIOFSDirectory uses
// buffered readers and writers to reduce system calls and improve performance.
//
// NIOFSDirectory is the recommended FSDirectory implementation for most
// use cases when MMapDirectory is not available or suitable.
type NIOFSDirectory struct {
	*FSDirectory
}

// NewNIOFSDirectory creates a new NIOFSDirectory at the specified path.
// The directory must exist and be writable.
func NewNIOFSDirectory(path string) (*NIOFSDirectory, error) {
	fsDir, err := NewFSDirectory(path)
	if err != nil {
		return nil, err
	}

	return &NIOFSDirectory{
		FSDirectory: fsDir,
	}, nil
}

// OpenInput returns an IndexInput for reading an existing file.
// This implementation uses buffered I/O for efficient reading.
func (d *NIOFSDirectory) OpenInput(name string, ctx IOContext) (IndexInput, error) {
	if err := d.EnsureOpen(); err != nil {
		return nil, err
	}

	path := filepath.Join(d.directory, name)
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrFileNotFound, name)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// Get file info for length
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	d.AddOpenFile(name)

	return &NIOFSIndexInput{
		file:           file,
		bufReader:      bufio.NewReaderSize(file, 8192), // 8KB buffer
		path:           path,
		name:           name,
		directory:      d,
		BaseIndexInput: NewBaseIndexInput(fmt.Sprintf("NIOFSIndexInput(path=\"%s\")", path), info.Size()),
	}, nil
}

// CreateOutput returns an IndexOutput for writing a new file.
// This implementation uses buffered I/O for efficient writing.
func (d *NIOFSDirectory) CreateOutput(name string, ctx IOContext) (IndexOutput, error) {
	if err := d.EnsureOpen(); err != nil {
		return nil, err
	}

	path := filepath.Join(d.directory, name)

	// Check if file already exists
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("%w: %s", ErrFileAlreadyExists, name)
	}

	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	d.AddOpenFile(name)

	return &NIOFSIndexOutput{
		file:            file,
		bufWriter:       bufio.NewWriterSize(file, 8192), // 8KB buffer
		path:            path,
		name:            name,
		directory:       d,
		BaseIndexOutput: NewBaseIndexOutput(name),
	}, nil
}

// NIOFSIndexInput is an IndexInput implementation for NIOFSDirectory.
// It uses buffered reading for improved I/O performance.
type NIOFSIndexInput struct {
	*BaseIndexInput
	file      *os.File
	bufReader *bufio.Reader
	path      string
	name      string
	directory *NIOFSDirectory
}

// ReadByte reads a single byte from the buffered reader.
func (in *NIOFSIndexInput) ReadByte() (byte, error) {
	if !in.directory.IsOpen() {
		return 0, ErrIllegalState
	}

	b, err := in.bufReader.ReadByte()
	if err != nil {
		if err == io.EOF {
			return 0, io.EOF
		}
		return 0, err
	}

	in.SetFilePointer(in.GetFilePointer() + 1)
	return b, nil
}

// ReadBytes reads len(b) bytes into b from the buffered reader.
func (in *NIOFSIndexInput) ReadBytes(b []byte) error {
	if !in.directory.IsOpen() {
		return ErrIllegalState
	}

	n, err := io.ReadFull(in.bufReader, b)
	if err != nil {
		return err
	}

	in.SetFilePointer(in.GetFilePointer() + int64(n))
	return nil
}

// ReadBytesN reads exactly n bytes and returns them.
func (in *NIOFSIndexInput) ReadBytesN(n int) ([]byte, error) {
	b := make([]byte, n)
	if err := in.ReadBytes(b); err != nil {
		return nil, err
	}
	return b, nil
}

// ReadShort reads a 16-bit value.
func (in *NIOFSIndexInput) ReadShort() (int16, error) {
	b, err := in.ReadBytesN(2)
	if err != nil {
		return 0, err
	}
	return int16(b[0])<<8 | int16(b[1]), nil
}

// ReadInt reads a 32-bit value.
func (in *NIOFSIndexInput) ReadInt() (int32, error) {
	b, err := in.ReadBytesN(4)
	if err != nil {
		return 0, err
	}
	return int32(b[0])<<24 | int32(b[1])<<16 | int32(b[2])<<8 | int32(b[3]), nil
}

// ReadLong reads a 64-bit value.
func (in *NIOFSIndexInput) ReadLong() (int64, error) {
	b, err := in.ReadBytesN(8)
	if err != nil {
		return 0, err
	}
	return int64(b[0])<<56 | int64(b[1])<<48 | int64(b[2])<<40 | int64(b[3])<<32 |
		int64(b[4])<<24 | int64(b[5])<<16 | int64(b[6])<<8 | int64(b[7]), nil
}

// ReadString reads a string.
func (in *NIOFSIndexInput) ReadString() (string, error) {
	return ReadString(in)
}

// SetPosition changes the current position in the file.
// This operation discards the buffer and repositions the underlying file.
func (in *NIOFSIndexInput) SetPosition(pos int64) error {
	if pos < 0 || pos > in.Length() {
		return fmt.Errorf("invalid position: %d", pos)
	}

	// Seek in the underlying file
	_, err := in.file.Seek(pos, io.SeekStart)
	if err != nil {
		return err
	}

	// Reset the buffered reader
	in.bufReader.Reset(in.file)
	in.SetFilePointer(pos)
	return nil
}

// Clone returns a clone of this IndexInput.
// The clone shares the same underlying file but has independent buffering.
func (in *NIOFSIndexInput) Clone() IndexInput {
	// Open a new file handle for the clone
	file, err := os.Open(in.path)
	if err != nil {
		// Return a clone that will fail on read
		return &NIOFSIndexInput{
			BaseIndexInput: NewBaseIndexInput(in.GetDescription(), in.Length()),
			file:           nil,
			bufReader:      nil,
			path:           in.path,
			name:           in.name,
			directory:      in.directory,
		}
	}

	// Position the new file at the current position
	currentPos := in.GetFilePointer()
	if _, err := file.Seek(currentPos, io.SeekStart); err != nil {
		file.Close()
		return &NIOFSIndexInput{
			BaseIndexInput: NewBaseIndexInput(in.GetDescription(), in.Length()),
			file:           nil,
			bufReader:      nil,
			path:           in.path,
			name:           in.name,
			directory:      in.directory,
		}
	}

	in.directory.AddOpenFile(in.name)

	clone := &NIOFSIndexInput{
		BaseIndexInput: NewBaseIndexInput(in.GetDescription(), in.Length()),
		file:           file,
		bufReader:      bufio.NewReaderSize(file, 8192),
		path:           in.path,
		name:           in.name,
		directory:      in.directory,
	}
	// Set the file pointer to match the original
	clone.SetFilePointer(currentPos)
	return clone
}

// Slice returns a subset of this IndexInput.
func (in *NIOFSIndexInput) Slice(desc string, offset int64, length int64) (IndexInput, error) {
	if offset < 0 || length < 0 || offset+length > in.Length() {
		return nil, fmt.Errorf("invalid slice parameters: offset=%d, length=%d, fileLength=%d", offset, length, in.Length())
	}

	// Open a new file handle for the slice
	file, err := os.Open(in.path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file for slice: %w", err)
	}

	// Seek to the offset
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to seek to offset: %w", err)
	}

	in.directory.AddOpenFile(in.name)

	return &NIOFSIndexInput{
		BaseIndexInput: NewBaseIndexInput(desc, length),
		file:           file,
		bufReader:      bufio.NewReaderSize(file, 8192),
		path:           in.path,
		name:           in.name,
		directory:      in.directory,
	}, nil
}

// Close closes this IndexInput and releases resources.
func (in *NIOFSIndexInput) Close() error {
	if in.file == nil {
		return nil
	}
	in.directory.RemoveOpenFile(in.name)
	return in.file.Close()
}

// NIOFSIndexOutput is an IndexOutput implementation for NIOFSDirectory.
// It uses buffered writing for improved I/O performance.
type NIOFSIndexOutput struct {
	*BaseIndexOutput
	file      *os.File
	bufWriter *bufio.Writer
	path      string
	name      string
	directory *NIOFSDirectory
}

// WriteByte writes a single byte to the buffered writer.
func (out *NIOFSIndexOutput) WriteByte(b byte) error {
	if !out.directory.IsOpen() {
		return ErrIllegalState
	}

	if err := out.bufWriter.WriteByte(b); err != nil {
		return err
	}

	out.IncrementFilePointer(1)
	return nil
}

// WriteBytes writes all bytes from b to the buffered writer.
func (out *NIOFSIndexOutput) WriteBytes(b []byte) error {
	if !out.directory.IsOpen() {
		return ErrIllegalState
	}

	if _, err := out.bufWriter.Write(b); err != nil {
		return err
	}

	out.IncrementFilePointer(int64(len(b)))
	return nil
}

// WriteBytesN writes exactly n bytes from b.
func (out *NIOFSIndexOutput) WriteBytesN(b []byte, n int) error {
	if n > len(b) {
		return fmt.Errorf("n exceeds buffer length")
	}
	return out.WriteBytes(b[:n])
}

// WriteShort writes a 16-bit value.
func (out *NIOFSIndexOutput) WriteShort(i int16) error {
	b := []byte{byte(i >> 8), byte(i)}
	return out.WriteBytes(b)
}

// WriteInt writes a 32-bit value.
func (out *NIOFSIndexOutput) WriteInt(i int32) error {
	b := []byte{byte(i >> 24), byte(i >> 16), byte(i >> 8), byte(i)}
	return out.WriteBytes(b)
}

// WriteLong writes a 64-bit value.
func (out *NIOFSIndexOutput) WriteLong(i int64) error {
	b := []byte{
		byte(i >> 56), byte(i >> 48), byte(i >> 40), byte(i >> 32),
		byte(i >> 24), byte(i >> 16), byte(i >> 8), byte(i),
	}
	return out.WriteBytes(b)
}

// WriteString writes a string.
func (out *NIOFSIndexOutput) WriteString(s string) error {
	return WriteString(out, s)
}

// Length returns the total length of the file written so far.
func (out *NIOFSIndexOutput) Length() int64 {
	// Flush buffer to ensure accurate length
	_ = out.bufWriter.Flush()

	info, err := out.file.Stat()
	if err != nil {
		return out.GetFilePointer()
	}
	return info.Size()
}

// SetPosition sets the current position for writing using file seek.
func (out *NIOFSIndexOutput) SetPosition(pos int64) error {
	if !out.directory.IsOpen() {
		return ErrIllegalState
	}
	// Flush any buffered data before seeking
	if err := out.bufWriter.Flush(); err != nil {
		return fmt.Errorf("failed to flush buffer: %w", err)
	}
	_, err := out.file.Seek(pos, 0)
	if err != nil {
		return fmt.Errorf("failed to seek to position %d: %w", pos, err)
	}
	out.SetFilePointer(pos)
	return nil
}

// Close flushes any buffered data, syncs to disk, and closes the file.
func (out *NIOFSIndexOutput) Close() error {
	if out.file == nil {
		return nil
	}

	// Flush the buffer
	if err := out.bufWriter.Flush(); err != nil {
		return fmt.Errorf("failed to flush buffer: %w", err)
	}

	// Sync the file to disk
	if err := out.file.Sync(); err != nil {
		// Log error but continue closing
		_ = err
	}

	out.directory.RemoveOpenFile(out.name)
	return out.file.Close()
}
