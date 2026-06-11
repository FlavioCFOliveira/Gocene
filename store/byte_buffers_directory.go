// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"encoding/binary"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
)

// ByteBuffersDirectory is an in-memory Directory implementation using byte slices.
// This is useful for testing, temporary indexes, and situations where
// disk persistence is not required.
//
// This is the Go port of Lucene's org.apache.lucene.store.ByteBuffersDirectory.
// All data is stored in memory and is lost when the directory is closed.
type ByteBuffersDirectory struct {
	*BaseDirectory
	files       map[string]*byteBufferFile
	mu          sync.RWMutex
	tempCounter atomic.Uint64
}

// byteBufferFile represents a file stored in memory.
type byteBufferFile struct {
	name    string
	content []byte
	mu      sync.RWMutex
}

// NewByteBuffersDirectory creates a new in-memory directory.
// Uses SingleInstanceLockFactory so that locks are correctly shared within
// the same process, matching Java's ByteBuffersDirectory semantics.
func NewByteBuffersDirectory() *ByteBuffersDirectory {
	return &ByteBuffersDirectory{
		BaseDirectory: NewBaseDirectory(NewSingleInstanceLockFactory()),
		files:         make(map[string]*byteBufferFile),
	}
}

// ListAll returns the names of all files in this directory.
func (d *ByteBuffersDirectory) ListAll() ([]string, error) {
	if err := d.EnsureOpen(); err != nil {
		return nil, err
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	names := make([]string, 0, len(d.files))
	for name := range d.files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// FileExists returns true if a file with the given name exists.
func (d *ByteBuffersDirectory) FileExists(name string) bool {
	if !d.IsOpen() {
		return false
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	_, exists := d.files[name]
	return exists
}

// FileLength returns the length of a file in bytes.
func (d *ByteBuffersDirectory) FileLength(name string) (int64, error) {
	if err := d.EnsureOpen(); err != nil {
		return 0, err
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	file, exists := d.files[name]
	if !exists {
		return 0, fmt.Errorf("%w: %s", ErrFileNotFound, name)
	}

	file.mu.RLock()
	defer file.mu.RUnlock()

	return int64(len(file.content)), nil
}

// DeleteFile deletes a file from the directory.
// Unlike some implementations, deleting an open file is allowed: the file
// is removed from the directory index immediately, but open handles remain
// valid until they are closed. This matches Java's ByteBuffersDirectory
// semantics where reference-counting keeps data alive past deletion.
func (d *ByteBuffersDirectory) DeleteFile(name string) error {
	if err := d.EnsureOpen(); err != nil {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.files[name]; !exists {
		return fmt.Errorf("%w: %s", ErrFileNotFound, name)
	}

	delete(d.files, name)
	return nil
}

// CreateOutput returns an IndexOutput for writing a new file.
func (d *ByteBuffersDirectory) CreateOutput(name string, ctx IOContext) (IndexOutput, error) {
	if err := d.EnsureOpen(); err != nil {
		return nil, err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.files[name]; exists {
		return nil, fmt.Errorf("%w: %s", ErrFileAlreadyExists, name)
	}

	// Create new file
	file := &byteBufferFile{
		name:    name,
		content: make([]byte, 0),
	}
	d.files[name] = file
	d.AddOpenFile(name)

	return &ByteBuffersIndexOutput{
		BaseIndexOutput: NewBaseIndexOutput(name),
		file:            file,
		directory:       d,
	}, nil
}

// OpenInput returns an IndexInput for reading an existing file.
func (d *ByteBuffersDirectory) OpenInput(name string, ctx IOContext) (IndexInput, error) {
	if err := d.EnsureOpen(); err != nil {
		return nil, err
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	file, exists := d.files[name]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrFileNotFound, name)
	}

	file.mu.RLock()
	defer file.mu.RUnlock()

	// Make a copy of the content for reading
	contentCopy := make([]byte, len(file.content))
	copy(contentCopy, file.content)

	d.AddOpenFile(name)

	return &ByteBuffersIndexInput{
		BaseIndexInput: NewBaseIndexInput(fmt.Sprintf("ByteBuffersIndexInput(name=\"%s\")", name), int64(len(contentCopy))),
		content:        contentCopy,
		file:           file,
		directory:      d,
		name:           name,
	}, nil
}

// CreateTempOutput creates a temporary output file with a unique name.
func (d *ByteBuffersDirectory) CreateTempOutput(prefix string, suffix string, ctx IOContext) (IndexOutput, error) {
	if err := d.EnsureOpen(); err != nil {
		return nil, err
	}

	// Generate unique name using atomic counter
	name := d.generateTempFileName(prefix, suffix)
	return d.CreateOutput(name, ctx)
}

// generateTempFileName generates a unique temporary file name.
// Uses an atomic counter to avoid lock acquisition, which would deadlock
// when called from CreateTempOutput (since CreateOutput also acquires d.mu).
func (d *ByteBuffersDirectory) generateTempFileName(prefix, suffix string) string {
	counter := d.tempCounter.Add(1)
	return fmt.Sprintf("%s_%d%s", prefix, counter, suffix)
}

// Sync is a no-op for ByteBuffersDirectory as data is already in memory.
func (d *ByteBuffersDirectory) Sync(names []string) error {
	if err := d.EnsureOpen(); err != nil {
		return err
	}
	// No-op for in-memory directory
	return nil
}

// Rename renames a file.
func (d *ByteBuffersDirectory) Rename(source string, dest string) error {
	if err := d.EnsureOpen(); err != nil {
		return err
	}

	if d.IsFileOpen(source) {
		return fmt.Errorf("%w: %s", ErrFileIsOpen, source)
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	file, exists := d.files[source]
	if !exists {
		return fmt.Errorf("%w: %s", ErrFileNotFound, source)
	}

	if _, exists := d.files[dest]; exists {
		return fmt.Errorf("%w: %s", ErrFileAlreadyExists, dest)
	}

	// Rename the file
	delete(d.files, source)
	file.name = dest
	d.files[dest] = file

	return nil
}

// ObtainLock returns a lock for the given name.
// For ByteBuffersDirectory, locks are in-memory only.
func (d *ByteBuffersDirectory) ObtainLock(name string) (Lock, error) {
	if err := d.EnsureOpen(); err != nil {
		return nil, err
	}
	return d.BaseDirectory.ObtainLock(name)
}

// Close releases all resources associated with this directory.
func (d *ByteBuffersDirectory) Close() error {
	if !d.IsOpen() {
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Clear all files
	d.files = make(map[string]*byteBufferFile)
	d.BaseDirectory.Close()

	return nil
}

// ByteBuffersIndexInput is an IndexInput implementation for ByteBuffersDirectory.
type ByteBuffersIndexInput struct {
	*BaseIndexInput
	content   []byte
	position  int64
	file      *byteBufferFile
	directory *ByteBuffersDirectory
	name      string
}

// ReadByte reads a single byte.
func (in *ByteBuffersIndexInput) ReadByte() (byte, error) {
	if !in.directory.IsOpen() {
		return 0, ErrIllegalState
	}

	if in.position >= int64(len(in.content)) {
		return 0, fmt.Errorf("EOF")
	}

	b := in.content[in.position]
	in.position++
	in.SetFilePointer(in.position)
	return b, nil
}

// ReadBytes reads len(b) bytes into b.
func (in *ByteBuffersIndexInput) ReadBytes(b []byte) error {
	if !in.directory.IsOpen() {
		return ErrIllegalState
	}

	if in.position+int64(len(b)) > int64(len(in.content)) {
		return fmt.Errorf("not enough data available")
	}

	copy(b, in.content[in.position:in.position+int64(len(b))])
	in.position += int64(len(b))
	in.SetFilePointer(in.position)
	return nil
}

// ReadBytesN reads exactly n bytes and returns them.
func (in *ByteBuffersIndexInput) ReadBytesN(n int) ([]byte, error) {
	b := make([]byte, n)
	if err := in.ReadBytes(b); err != nil {
		return nil, err
	}
	return b, nil
}

// ReadShort reads a 16-bit value.
func (in *ByteBuffersIndexInput) ReadShort() (int16, error) {
	buf := make([]byte, 2)
	if err := in.ReadBytes(buf); err != nil {
		return 0, err
	}
	return int16(binary.LittleEndian.Uint16(buf)), nil
}

// ReadInt reads a 32-bit value.
func (in *ByteBuffersIndexInput) ReadInt() (int32, error) {
	buf := make([]byte, 4)
	if err := in.ReadBytes(buf); err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(buf)), nil
}

// ReadLong reads a 64-bit value.
func (in *ByteBuffersIndexInput) ReadLong() (int64, error) {
	buf := make([]byte, 8)
	if err := in.ReadBytes(buf); err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(buf)), nil
}

// ReadString reads a string.
func (in *ByteBuffersIndexInput) ReadString() (string, error) {
	length, err := in.ReadVInt()
	if err != nil {
		return "", err
	}
	buf := make([]byte, length)
	if err := in.ReadBytes(buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

// ReadVInt reads a variable-length integer.
func (in *ByteBuffersIndexInput) ReadVInt() (int32, error) {
	var result int32
	shift := 0
	for {
		b, err := in.ReadByte()
		if err != nil {
			return 0, err
		}
		result |= int32(b&0x7F) << shift
		if (b & 0x80) == 0 {
			break
		}
		shift += 7
		if shift >= 32 {
			return 0, fmt.Errorf("corrupted VInt")
		}
	}
	return result, nil
}

// ReadVLong reads a variable-length long, completing the
// [VariableLengthInput] surface alongside [ByteBuffersIndexInput.ReadVInt].
// Delegates to the package-level [ReadVLong] so the decoding is identical to
// every other DataInput in the package.
func (in *ByteBuffersIndexInput) ReadVLong() (int64, error) {
	return ReadVLong(in)
}

// SetPosition changes the current position.
func (in *ByteBuffersIndexInput) SetPosition(pos int64) error {
	if pos < 0 || pos > int64(len(in.content)) {
		return fmt.Errorf("invalid position: %d", pos)
	}
	in.position = pos
	in.SetFilePointer(pos)
	return nil
}

// Clone returns a clone of this IndexInput.
func (in *ByteBuffersIndexInput) Clone() IndexInput {
	in.directory.AddOpenFile(in.name)
	clone := &ByteBuffersIndexInput{
		BaseIndexInput: NewBaseIndexInput(in.GetDescription(), in.Length()),
		content:        in.content,
		position:       in.position,
		file:           in.file,
		directory:      in.directory,
		name:           in.name,
	}
	clone.SetFilePointer(in.position)
	return clone
}

// Slice returns a subset of this IndexInput.
func (in *ByteBuffersIndexInput) Slice(desc string, offset int64, length int64) (IndexInput, error) {
	if offset < 0 || length < 0 || offset+length > int64(len(in.content)) {
		return nil, fmt.Errorf("invalid slice parameters: offset=%d, length=%d, contentLength=%d", offset, length, len(in.content))
	}

	in.directory.AddOpenFile(in.name)

	return &ByteBuffersIndexInput{
		BaseIndexInput: NewBaseIndexInput(desc, length),
		content:        in.content[offset : offset+length],
		position:       0,
		file:           in.file,
		directory:      in.directory,
		name:           in.name,
	}, nil
}

// Close releases resources for this IndexInput.
func (in *ByteBuffersIndexInput) Close() error {
	in.directory.RemoveOpenFile(in.name)
	return nil
}

// ReadByteAt reads a single byte at the given position.
// This implements the RandomAccessInput interface.
func (in *ByteBuffersIndexInput) ReadByteAt(pos int64) (byte, error) {
	if !in.directory.IsOpen() {
		return 0, ErrIllegalState
	}
	if pos < 0 || pos >= int64(len(in.content)) {
		return 0, fmt.Errorf("position %d out of range [0, %d]", pos, len(in.content))
	}
	return in.content[pos], nil
}

// ReadLongAt reads a 64-bit value at the given position in little-endian
// format, as required by the RandomAccessInput contract (see
// random_access_input.go) and matching Lucene 10.4.0, whose
// ByteBuffersDataInput / RandomAccessInput are little-endian. It must agree
// with the sequential ReadLong above; readers such as the doc-values jump
// table, the lucene103 trie, and packed DirectReader all assume little-endian.
func (in *ByteBuffersIndexInput) ReadLongAt(pos int64) (int64, error) {
	if !in.directory.IsOpen() {
		return 0, ErrIllegalState
	}
	if pos < 0 || pos+8 > int64(len(in.content)) {
		return 0, fmt.Errorf("position %d out of range for 8-byte read [0, %d]", pos, len(in.content))
	}
	return int64(binary.LittleEndian.Uint64(in.content[pos : pos+8])), nil
}

// ByteBuffersIndexOutput is an IndexOutput implementation for ByteBuffersDirectory
// with random-access write support via SetPosition.
type ByteBuffersIndexOutput struct {
	*BaseIndexOutput
	file      *byteBufferFile
	directory *ByteBuffersDirectory
	data      []byte
}

// WriteByte writes a single byte at the current write position.
func (out *ByteBuffersIndexOutput) WriteByte(b byte) error {
	if !out.directory.IsOpen() {
		return ErrIllegalState
	}

	pos := out.GetFilePointer()
	if pos >= int64(len(out.data)) {
		// Extend with zeros up to and including this byte
		out.data = append(out.data, make([]byte, pos+1-int64(len(out.data)))...)
	}
	out.data[pos] = b
	out.IncrementFilePointer(1)
	return nil
}

// WriteBytes writes all bytes from b at the current write position.
func (out *ByteBuffersIndexOutput) WriteBytes(b []byte) error {
	if !out.directory.IsOpen() {
		return ErrIllegalState
	}

	n := int64(len(b))
	pos := out.GetFilePointer()
	end := pos + n
	if end > int64(len(out.data)) {
		out.data = append(out.data, make([]byte, end-int64(len(out.data)))...)
	}
	copy(out.data[pos:end], b)
	out.IncrementFilePointer(n)
	return nil
}

// WriteBytesN writes exactly n bytes from b.
func (out *ByteBuffersIndexOutput) WriteBytesN(b []byte, n int) error {
	if n > len(b) {
		return fmt.Errorf("n exceeds buffer length")
	}
	return out.WriteBytes(b[:n])
}

// WriteShort writes a 16-bit value as little-endian to match Lucene 10.x
// DataOutput.writeShort (low byte first) and ByteBuffersIndexInput.ReadShort
// (already LE). See rmp #4786.
func (out *ByteBuffersIndexOutput) WriteShort(i int16) error {
	b := []byte{byte(i), byte(i >> 8)}
	return out.WriteBytes(b)
}

// WriteInt writes a 32-bit value as little-endian to match Lucene 10.x
// DataOutput.writeInt (low byte first) and ByteBuffersIndexInput.ReadInt
// (already LE). See rmp #4786.
func (out *ByteBuffersIndexOutput) WriteInt(i int32) error {
	b := []byte{byte(i), byte(i >> 8), byte(i >> 16), byte(i >> 24)}
	return out.WriteBytes(b)
}

// WriteLong writes a 64-bit value as little-endian to match Lucene 10.x
// DataOutput.writeLong (low byte first) and ByteBuffersIndexInput.ReadLong
// (already LE). See rmp #4786.
func (out *ByteBuffersIndexOutput) WriteLong(i int64) error {
	b := []byte{
		byte(i), byte(i >> 8), byte(i >> 16), byte(i >> 24),
		byte(i >> 32), byte(i >> 40), byte(i >> 48), byte(i >> 56),
	}
	return out.WriteBytes(b)
}

// WriteString writes a string.
func (out *ByteBuffersIndexOutput) WriteString(s string) error {
	return WriteString(out, s)
}

// Length returns the current length of the file being written.
func (out *ByteBuffersIndexOutput) Length() int64 {
	return int64(len(out.data))
}

// SetPosition sets the current write position.
// Seeking past the end extends the buffer with zero bytes.
func (out *ByteBuffersIndexOutput) SetPosition(pos int64) error {
	if pos < 0 {
		return fmt.Errorf("negative position: %d", pos)
	}
	if pos > int64(len(out.data)) {
		out.data = append(out.data, make([]byte, pos-int64(len(out.data)))...)
	}
	out.SetFilePointer(pos)
	return nil
}

// Close finalizes the file and stores it in the directory.
func (out *ByteBuffersIndexOutput) Close() error {
	if out.file == nil {
		return nil
	}

	// Store the final content
	out.file.mu.Lock()
	out.file.content = out.data
	out.file.mu.Unlock()

	out.directory.RemoveOpenFile(out.file.name)
	return nil
}
