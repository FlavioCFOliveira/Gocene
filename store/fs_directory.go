// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FSDirectory is the abstract base class for Directory implementations
// that store the index in the file system.
//
// This is the Go port of Lucene's org.apache.lucene.store.FSDirectory.
// Subclasses must implement OpenInput() and CreateOutput() with specific
// I/O strategies (e.g., memory-mapped files, buffered I/O).
type FSDirectory struct {
	*BaseDirectory

	// directory is the path to the directory on the file system
	directory string

	// staleFiles tracks files that may have been deleted by the OS
	// but are still referenced by open IndexInputs
	staleFiles map[string]struct{}
}

// NewFSDirectory creates a new FSDirectory at the specified path.
// The directory must exist and be writable.
//
// This is an abstract base constructor - use Open() or specific
// implementations like MMapDirectory or NIOFSDirectory.
func NewFSDirectory(path string) (*FSDirectory, error) {
	// Validate the path exists and is a directory
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: directory does not exist: %s", ErrFileNotFound, path)
		}
		return nil, fmt.Errorf("failed to stat directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", path)
	}

	// Ensure the directory is writable
	testFile := filepath.Join(path, ".gocene_write_test")
	f, err := os.Create(testFile)
	if err != nil {
		return nil, fmt.Errorf("directory is not writable: %s", path)
	}
	f.Close()
	os.Remove(testFile)

	return &FSDirectory{
		BaseDirectory: NewBaseDirectory(nil),
		directory:     path,
		staleFiles:    make(map[string]struct{}),
	}, nil
}

// GetPath returns the directory path as a string.
func (d *FSDirectory) GetPath() string {
	return d.directory
}

// ListAll returns the names of all files in this directory.
func (d *FSDirectory) ListAll() ([]string, error) {
	if err := d.EnsureOpen(); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(d.directory)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		// Skip subdirectories
		if entry.IsDir() {
			continue
		}
		// Skip lock files and hidden files
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		// Skip lock files
		if strings.HasSuffix(name, ".lock") {
			continue
		}
		files = append(files, name)
	}

	sort.Strings(files)
	return files, nil
}

// FileExists returns true if a file with the given name exists in this directory.
func (d *FSDirectory) FileExists(name string) bool {
	if d.IsOpen() == false {
		return false
	}

	path := filepath.Join(d.directory, name)
	_, err := os.Stat(path)
	return err == nil
}

// FileLength returns the length of a file in the directory.
func (d *FSDirectory) FileLength(name string) (int64, error) {
	if err := d.EnsureOpen(); err != nil {
		return 0, err
	}

	path := filepath.Join(d.directory, name)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("%w: %s", ErrFileNotFound, name)
		}
		return 0, fmt.Errorf("failed to stat file: %w", err)
	}

	if info.IsDir() {
		return 0, fmt.Errorf("path is a directory, not a file: %s", name)
	}

	return info.Size(), nil
}

// OpenInput returns an IndexInput for reading an existing file.
// This method must be implemented by subclasses.
func (d *FSDirectory) OpenInput(name string, ctx IOContext) (IndexInput, error) {
	return nil, fmt.Errorf("OpenInput must be implemented by subclass")
}

// CreateOutput returns an IndexOutput for writing a new file.
// This method must be implemented by subclasses.
func (d *FSDirectory) CreateOutput(name string, ctx IOContext) (IndexOutput, error) {
	return nil, fmt.Errorf("CreateOutput must be implemented by subclass")
}

// DeleteFile deletes a file from the directory.
func (d *FSDirectory) DeleteFile(name string) error {
	if err := d.EnsureOpen(); err != nil {
		return err
	}

	if d.IsFileOpen(name) {
		return fmt.Errorf("%w: %s", ErrFileIsOpen, name)
	}

	path := filepath.Join(d.directory, name)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrFileNotFound, name)
		}
		return fmt.Errorf("failed to stat file: %w", err)
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// CreateTempOutput creates a temporary output file.
// The file is created with a unique name in the directory.
func (d *FSDirectory) CreateTempOutput(prefix string, suffix string, ctx IOContext) (IndexOutput, error) {
	if err := d.EnsureOpen(); err != nil {
		return nil, err
	}

	// Generate unique filename
	tempFile, err := os.CreateTemp(d.directory, prefix+"*"+suffix)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tempFile.Close()

	name := filepath.Base(tempFile.Name())

	// Delete the temp file - subclasses will create their own IndexOutput
	if err := os.Remove(tempFile.Name()); err != nil {
		return nil, fmt.Errorf("failed to remove temp file placeholder: %w", err)
	}

	// Call CreateOutput with the generated name
	return d.CreateOutput(name, ctx)
}

// Sync forces any buffered writes to be written to the file system.
func (d *FSDirectory) Sync(names []string) error {
	if err := d.EnsureOpen(); err != nil {
		return err
	}

	for _, name := range names {
		path := filepath.Join(d.directory, name)
		f, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("%w: %s", ErrFileNotFound, name)
			}
			return fmt.Errorf("failed to open file for sync: %w", err)
		}

		if err := f.Sync(); err != nil {
			f.Close()
			return fmt.Errorf("failed to sync file %s: %w", name, err)
		}
		f.Close()
	}

	return nil
}

// Rename renames a source file to a target file.
func (d *FSDirectory) Rename(source string, dest string) error {
	if err := d.EnsureOpen(); err != nil {
		return err
	}

	sourcePath := filepath.Join(d.directory, source)
	destPath := filepath.Join(d.directory, dest)

	// Check if source exists
	if _, err := os.Stat(sourcePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrFileNotFound, source)
		}
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	// Check if destination exists
	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("%w: %s", ErrFileAlreadyExists, dest)
	}

	if err := os.Rename(sourcePath, destPath); err != nil {
		return fmt.Errorf("failed to rename file: %w", err)
	}

	return nil
}

// ObtainLock attempts to obtain a lock for the specified name.
// For FSDirectory, this creates a lock file in the directory.
func (d *FSDirectory) ObtainLock(name string) (Lock, error) {
	if err := d.EnsureOpen(); err != nil {
		return nil, err
	}

	// Use the parent's LockFactory if configured, passing d (FSDirectory) instead of BaseDirectory
	if d.GetLockFactory() != nil {
		return d.GetLockFactory().ObtainLock(d, name)
	}

	// Default: create a file-based lock
	lockFile := filepath.Join(d.directory, name+".lock")
	return d.obtainFSLock(lockFile)
}

// obtainFSLock creates a simple file-based lock.
func (d *FSDirectory) obtainFSLock(lockFile string) (Lock, error) {
	// Try to create the lock file exclusively
	f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("lock already held: %s", lockFile)
		}
		return nil, fmt.Errorf("failed to create lock file: %w", err)
	}

	// Write process ID to lock file for debugging
	pid := os.Getpid()
	fmt.Fprintf(f, "%d\n", pid)
	f.Close()

	return &FSLock{
		BaseLock: NewBaseLock(),
		path:     lockFile,
	}, nil
}

// Close releases all resources associated with this directory.
func (d *FSDirectory) Close() error {
	if !d.IsOpen() {
		return nil
	}

	// Close all open files
	d.BaseDirectory.Close()
	return nil
}

// FSLock is a file-system based lock implementation.
type FSLock struct {
	*BaseLock
	path string
}

// Close releases the lock by deleting the lock file.
func (l *FSLock) Close() error {
	if !l.IsLocked() {
		return nil
	}

	if err := os.Remove(l.path); err != nil {
		// Lock file may have been already removed - that's ok
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove lock file: %w", err)
		}
	}

	l.MarkReleased()
	return nil
}

// EnsureValid returns an error if the lock is no longer valid.
// For file-based locks, we check if the lock file still exists.
func (l *FSLock) EnsureValid() error {
	if !l.IsLocked() {
		return fmt.Errorf("lock is not held")
	}

	if _, err := os.Stat(l.path); err != nil {
		if os.IsNotExist(err) {
			l.MarkReleased()
			return fmt.Errorf("lock file was removed externally")
		}
		return fmt.Errorf("failed to stat lock file: %w", err)
	}

	return nil
}

// Open opens a directory at the specified path.
// This attempts to choose the best FSDirectory implementation for the platform.
// Currently returns an FSDirectory base; subclasses should override.
func Open(path string) (*FSDirectory, error) {
	// For now, return the base FSDirectory
	// In a full implementation, this would choose between MMapDirectory
	// and NIOFSDirectory based on platform capabilities
	return NewFSDirectory(path)
}

// SimpleFSDirectory is a simple file-system based directory using standard file I/O.
// This is a concrete implementation that can be used directly.
type SimpleFSDirectory struct {
	*FSDirectory
}

// NewSimpleFSDirectory creates a new SimpleFSDirectory at the specified path.
func NewSimpleFSDirectory(path string) (*SimpleFSDirectory, error) {
	fsDir, err := NewFSDirectory(path)
	if err != nil {
		return nil, err
	}

	return &SimpleFSDirectory{
		FSDirectory: fsDir,
	}, nil
}

// OpenInput returns an IndexInput for reading an existing file.
func (d *SimpleFSDirectory) OpenInput(name string, ctx IOContext) (IndexInput, error) {
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

	return &SimpleFSIndexInput{
		file:           file,
		path:           path,
		name:           name,
		directory:      d,
		BaseIndexInput: NewBaseIndexInput(fmt.Sprintf("SimpleFSIndexInput(path=\"%s\")", path), info.Size()),
	}, nil
}

// CreateOutput returns an IndexOutput for writing a new file.
func (d *SimpleFSDirectory) CreateOutput(name string, ctx IOContext) (IndexOutput, error) {
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

	return &SimpleFSIndexOutput{
		file:            file,
		path:            path,
		name:            name,
		directory:       d,
		BaseIndexOutput: NewBaseIndexOutput(name),
	}, nil
}

// SimpleFSIndexInput is an IndexInput implementation for SimpleFSDirectory.
type SimpleFSIndexInput struct {
	*BaseIndexInput
	file      *os.File
	path      string
	name      string
	directory *SimpleFSDirectory
}

// ReadByte reads a single byte.
func (in *SimpleFSIndexInput) ReadByte() (byte, error) {
	if !in.directory.IsOpen() {
		return 0, ErrIllegalState
	}

	b := make([]byte, 1)
	_, err := in.file.Read(b)
	if err != nil {
		if err == io.EOF {
			return 0, io.EOF
		}
		return 0, err
	}

	in.SetFilePointer(in.GetFilePointer() + 1)
	return b[0], nil
}

// ReadBytes reads len(b) bytes into b.
func (in *SimpleFSIndexInput) ReadBytes(b []byte) error {
	if !in.directory.IsOpen() {
		return ErrIllegalState
	}

	n, err := io.ReadFull(in.file, b)
	if err != nil {
		return err
	}

	in.SetFilePointer(in.GetFilePointer() + int64(n))
	return nil
}

// ReadBytesN reads exactly n bytes and returns them.
func (in *SimpleFSIndexInput) ReadBytesN(n int) ([]byte, error) {
	b := make([]byte, n)
	if err := in.ReadBytes(b); err != nil {
		return nil, err
	}
	return b, nil
}

// ReadShort reads a 16-bit value.
func (in *SimpleFSIndexInput) ReadShort() (int16, error) {
	b, err := in.ReadBytesN(2)
	if err != nil {
		return 0, err
	}
	return int16(b[0])<<8 | int16(b[1]), nil
}

// ReadInt reads a 32-bit value.
func (in *SimpleFSIndexInput) ReadInt() (int32, error) {
	b, err := in.ReadBytesN(4)
	if err != nil {
		return 0, err
	}
	return int32(b[0])<<24 | int32(b[1])<<16 | int32(b[2])<<8 | int32(b[3]), nil
}

// ReadLong reads a 64-bit value.
func (in *SimpleFSIndexInput) ReadLong() (int64, error) {
	b, err := in.ReadBytesN(8)
	if err != nil {
		return 0, err
	}
	return int64(b[0])<<56 | int64(b[1])<<48 | int64(b[2])<<40 | int64(b[3])<<32 |
		int64(b[4])<<24 | int64(b[5])<<16 | int64(b[6])<<8 | int64(b[7]), nil
}

// ReadString reads a string.
func (in *SimpleFSIndexInput) ReadString() (string, error) {
	return ReadString(in)
}

// SetPosition changes the current position in the file.
func (in *SimpleFSIndexInput) SetPosition(pos int64) error {
	if pos < 0 || pos > in.Length() {
		return fmt.Errorf("invalid position: %d", pos)
	}
	_, err := in.file.Seek(pos, io.SeekStart)
	if err != nil {
		return err
	}
	in.SetFilePointer(pos)
	return nil
}

// Clone returns a clone of this IndexInput.
func (in *SimpleFSIndexInput) Clone() IndexInput {
	// Open a new file handle for the clone
	file, err := os.Open(in.path)
	if err != nil {
		// In a real implementation, we'd handle this better
		// For now, return a clone that will fail on read
		return &SimpleFSIndexInput{
			BaseIndexInput: NewBaseIndexInput(in.GetDescription(), in.Length()),
			file:           nil,
			path:           in.path,
			name:           in.name,
			directory:      in.directory,
		}
	}

	in.directory.AddOpenFile(in.name)

	return &SimpleFSIndexInput{
		BaseIndexInput: NewBaseIndexInput(in.GetDescription(), in.Length()),
		file:           file,
		path:           in.path,
		name:           in.name,
		directory:      in.directory,
	}
}

// Slice returns a subset of this IndexInput.
func (in *SimpleFSIndexInput) Slice(desc string, offset int64, length int64) (IndexInput, error) {
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

	return &SimpleFSIndexInput{
		BaseIndexInput: NewBaseIndexInput(desc, length),
		file:           file,
		path:           in.path,
		name:           in.name,
		directory:      in.directory,
	}, nil
}

// Close closes this IndexInput.
func (in *SimpleFSIndexInput) Close() error {
	if in.file == nil {
		return nil
	}
	in.directory.RemoveOpenFile(in.name)
	return in.file.Close()
}

// SimpleFSIndexOutput is an IndexOutput implementation for SimpleFSDirectory.
type SimpleFSIndexOutput struct {
	*BaseIndexOutput
	file      *os.File
	path      string
	name      string
	directory *SimpleFSDirectory
}

// WriteByte writes a single byte.
func (out *SimpleFSIndexOutput) WriteByte(b byte) error {
	if !out.directory.IsOpen() {
		return ErrIllegalState
	}

	if _, err := out.file.Write([]byte{b}); err != nil {
		return err
	}

	out.IncrementFilePointer(1)
	return nil
}

// WriteBytes writes all bytes from b.
func (out *SimpleFSIndexOutput) WriteBytes(b []byte) error {
	if !out.directory.IsOpen() {
		return ErrIllegalState
	}

	if _, err := out.file.Write(b); err != nil {
		return err
	}

	out.IncrementFilePointer(int64(len(b)))
	return nil
}

// WriteBytesN writes exactly n bytes from b.
func (out *SimpleFSIndexOutput) WriteBytesN(b []byte, n int) error {
	if n > len(b) {
		return fmt.Errorf("n exceeds buffer length")
	}
	return out.WriteBytes(b[:n])
}

// WriteShort writes a 16-bit value.
func (out *SimpleFSIndexOutput) WriteShort(i int16) error {
	b := []byte{byte(i >> 8), byte(i)}
	return out.WriteBytes(b)
}

// WriteInt writes a 32-bit value.
func (out *SimpleFSIndexOutput) WriteInt(i int32) error {
	b := []byte{byte(i >> 24), byte(i >> 16), byte(i >> 8), byte(i)}
	return out.WriteBytes(b)
}

// WriteLong writes a 64-bit value.
func (out *SimpleFSIndexOutput) WriteLong(i int64) error {
	b := []byte{
		byte(i >> 56), byte(i >> 48), byte(i >> 40), byte(i >> 32),
		byte(i >> 24), byte(i >> 16), byte(i >> 8), byte(i),
	}
	return out.WriteBytes(b)
}

// WriteString writes a string.
func (out *SimpleFSIndexOutput) WriteString(s string) error {
	return WriteString(out, s)
}

// Length returns the total length of the file written so far.
func (out *SimpleFSIndexOutput) Length() int64 {
	info, err := out.file.Stat()
	if err != nil {
		return out.GetFilePointer()
	}
	return info.Size()
}

// Close closes this IndexOutput.
func (out *SimpleFSIndexOutput) Close() error {
	if out.file == nil {
		return nil
	}

	// Sync the file before closing
	if err := out.file.Sync(); err != nil {
		// Log error but continue closing
		_ = err
	}

	out.directory.RemoveOpenFile(out.name)
	return out.file.Close()
}
