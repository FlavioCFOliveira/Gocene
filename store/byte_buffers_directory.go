// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/spi"
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
	chunkSize   int // 0 means monolithic (single buffer), >0 means chunked
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
		chunkSize:     0,
	}
}

// NewByteBuffersDirectoryWithChunkSize creates a new in-memory directory where
// each file is exposed as a sequence of chunks of at most chunkSize bytes.
// This is useful for testing cross-boundary read behaviour.
func NewByteBuffersDirectoryWithChunkSize(chunkSize int) *ByteBuffersDirectory {
	if chunkSize < 1 {
		panic("chunkSize must be >= 1")
	}
	return &ByteBuffersDirectory{
		BaseDirectory: NewBaseDirectory(NewSingleInstanceLockFactory()),
		files:         make(map[string]*byteBufferFile),
		chunkSize:     chunkSize,
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

	out := &ByteBuffersIndexOutput{
		BaseIndexOutput: spi.NewBaseIndexOutput(name),
		file:            file,
		directory:       d,
	}
	// Java's ByteBuffersIndexOutput forwards every derived writer to a
	// ByteBuffersDataOutput delegate, which inherits them from DataOutput.
	// Gocene collapses that delegate into this type, so the same bodies are
	// supplied by the embedded BaseDataOutput and reach the buffer through
	// this type's own WriteByte/WriteBytes.
	out.BaseDataOutput = *NewBaseDataOutput(out)
	return out, nil
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

	var chunks [][]byte
	if d.chunkSize > 0 {
		chunks = splitIntoChunks(contentCopy, d.chunkSize)
	} else {
		chunks = [][]byte{contentCopy}
	}

	d.AddOpenFile(name)

	return newByteBuffersIndexInput(chunks, fmt.Sprintf("ByteBuffersIndexInput(name=\"%s\")", name), file, d, name), nil
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
// It supports reading from either a monolithic byte slice or a chunked set of
// slices, enabling cross-boundary reads when the directory is configured with
// a chunk size.
type ByteBuffersIndexInput struct {
	*BaseIndexInput
	spi.BaseDataInput
	chunks    [][]byte
	cumLens   []int64
	position  int64
	file      *byteBufferFile
	directory *ByteBuffersDirectory
	name      string
}

// newByteBuffersIndexInput creates a new ByteBuffersIndexInput backed by the
// given chunks. The chunks slice is owned by the caller; newByteBuffersIndexInput
// does not copy it.
func newByteBuffersIndexInput(chunks [][]byte, desc string, file *byteBufferFile, directory *ByteBuffersDirectory, name string) *ByteBuffersIndexInput {
	cumLens := make([]int64, len(chunks))
	var total int64
	for i, chunk := range chunks {
		cumLens[i] = total
		total += int64(len(chunk))
	}
	in := &ByteBuffersIndexInput{
		BaseIndexInput: NewBaseIndexInput(desc, total),
		chunks:         chunks,
		cumLens:        cumLens,
		file:           file,
		directory:      directory,
		name:           name,
	}
	in.Core = in
	return in
}

// findChunk returns the chunk index and offset within that chunk for the given
// absolute position. pos must be within [0, Length).
func (in *ByteBuffersIndexInput) findChunk(pos int64) (int, int64) {
	idx := sort.Search(len(in.cumLens), func(i int) bool {
		return in.cumLens[i] > pos
	}) - 1
	if idx < 0 {
		idx = 0
	}
	offset := pos - in.cumLens[idx]
	return idx, offset
}

// readByteAt reads a single byte at the given absolute position without
// changing the current position.
func (in *ByteBuffersIndexInput) readByteAt(pos int64) (byte, error) {
	if pos < 0 || pos >= in.Length() {
		return 0, fmt.Errorf("position %d out of range [0, %d)", pos, in.Length())
	}
	idx, offset := in.findChunk(pos)
	return in.chunks[idx][offset], nil
}

// readBytesAt reads len(b) bytes at the given absolute position without
// changing the current position.
func (in *ByteBuffersIndexInput) readBytesAt(pos int64, b []byte) error {
	if pos+int64(len(b)) > in.Length() {
		return fmt.Errorf("not enough data available at position %d", pos)
	}
	n := len(b)
	for n > 0 {
		idx, offset := in.findChunk(pos)
		chunk := in.chunks[idx]
		avail := int64(len(chunk)) - offset
		toRead := int64(n)
		if toRead > avail {
			toRead = avail
		}
		copy(b[:toRead], chunk[offset:offset+toRead])
		pos += toRead
		n -= int(toRead)
		b = b[toRead:]
	}
	return nil
}

// ReadFloats reads len floats into dst.
func (in *ByteBuffersIndexInput) ReadFloats(dst []float32, offset, length int) error {
	for i := 0; i < length; i++ {
		v, err := in.ReadInt()
		if err != nil {
			return err
		}
		dst[offset+i] = math.Float32frombits(uint32(v))
	}
	return nil
}

// ReadInts reads len ints into dst.
func (in *ByteBuffersIndexInput) ReadInts(dst []int32, offset, length int) error {
	for i := 0; i < length; i++ {
		v, err := in.ReadInt()
		if err != nil {
			return err
		}
		dst[offset+i] = v
	}
	return nil
}

// ReadLongs reads a specified number of longs into dst.
func (in *ByteBuffersIndexInput) ReadLongs(dst []int64, offset, length int) error {
	for i := 0; i < length; i++ {
		v, err := in.ReadLong()
		if err != nil {
			return err
		}
		dst[offset+i] = v
	}
	return nil
}

// ReadByte reads a single byte.
func (in *ByteBuffersIndexInput) ReadByte() (byte, error) {
	if !in.directory.IsOpen() {
		return 0, ErrIllegalState
	}
	if in.position >= in.Length() {
		return 0, fmt.Errorf("EOF")
	}
	b, err := in.readByteAt(in.position)
	if err != nil {
		return 0, err
	}
	in.position++
	in.SetFilePointer(in.position)
	return b, nil
}

// ReadBytes reads len(b) bytes into b.
func (in *ByteBuffersIndexInput) ReadBytes(b []byte, offset, length int) error {
	if !in.directory.IsOpen() {
		return ErrIllegalState
	}
	if in.position+int64(length) > in.Length() {
		return fmt.Errorf("not enough data available")
	}
	if err := in.readBytesAt(in.position, b[offset:offset+length]); err != nil {
		return err
	}
	in.position += int64(length)
	in.SetFilePointer(in.position)
	return nil
}

// ReadBytesN reads exactly n bytes and returns them.
func (in *ByteBuffersIndexInput) ReadBytesN(n int) ([]byte, error) {
	b := make([]byte, n)
	if err := in.ReadBytes(b, 0, n); err != nil {
		return nil, err
	}
	return b, nil
}

// ReadShort reads a 16-bit value.
func (in *ByteBuffersIndexInput) ReadShort() (int16, error) {
	buf := make([]byte, 2)
	if err := in.ReadBytes(buf, 0, 2); err != nil {
		return 0, err
	}
	return int16(binary.LittleEndian.Uint16(buf)), nil
}

// ReadInt reads a 32-bit value.
func (in *ByteBuffersIndexInput) ReadInt() (int32, error) {
	buf := make([]byte, 4)
	if err := in.ReadBytes(buf, 0, 4); err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(buf)), nil
}

// ReadLong reads a 64-bit value.
func (in *ByteBuffersIndexInput) ReadLong() (int64, error) {
	buf := make([]byte, 8)
	if err := in.ReadBytes(buf, 0, 8); err != nil {
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
	if err := in.ReadBytes(buf, 0, int(length)); err != nil {
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
func (in *ByteBuffersIndexInput) ReadVLong() (int64, error) {
	var result int64
	shift := 0
	for {
		b, err := in.ReadByte()
		if err != nil {
			return 0, err
		}
		result |= int64(b&0x7F) << shift
		if (b & 0x80) == 0 {
			break
		}
		shift += 7
		if shift >= 64 {
			return 0, fmt.Errorf("corrupted VLong")
		}
	}
	return result, nil
}

// SetPosition changes the current position.
func (in *ByteBuffersIndexInput) SetPosition(pos int64) error {
	if pos < 0 || pos > in.Length() {
		return fmt.Errorf("invalid position: %d", pos)
	}
	in.position = pos
	in.SetFilePointer(pos)
	return nil
}

// SkipBytes skips n bytes forward in the input.
func (in *ByteBuffersIndexInput) SkipBytes(n int64) error {
	return in.BaseIndexInput.SkipBytes(n)
}

// Clone returns a clone of this IndexInput.
func (in *ByteBuffersIndexInput) Clone() IndexInput {
	in.directory.AddOpenFile(in.name)
	clone := newByteBuffersIndexInput(in.chunks, in.GetDescription(), in.file, in.directory, in.name)
	clone.position = in.position
	clone.SetFilePointer(in.position)
	return clone
}

// Slice returns a subset of this IndexInput. If the parent is chunked, the
// returned input preserves chunking over the sliced range.
func (in *ByteBuffersIndexInput) Slice(desc string, offset int64, length int64) (IndexInput, error) {
	if offset < 0 || length < 0 || offset+length > in.Length() {
		return nil, fmt.Errorf("invalid slice parameters: offset=%d, length=%d, contentLength=%d", offset, length, in.Length())
	}

	in.directory.AddOpenFile(in.name)

	newChunks := buildChunksFromSlice(in.chunks, in.cumLens, offset, length)
	sliced := newByteBuffersIndexInput(newChunks, desc, in.file, in.directory, in.name)
	return sliced, nil
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
	return in.readByteAt(pos)
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
	if pos < 0 || pos+8 > in.Length() {
		return 0, fmt.Errorf("position %d out of range for 8-byte read [0, %d)", pos, in.Length())
	}
	buf := make([]byte, 8)
	if err := in.readBytesAt(pos, buf); err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(buf)), nil
}

// buildChunksFromSlice creates a new chunk slice covering the byte range
// [offset, offset+length) from the given chunks. The returned chunks are slices
// into the original data (no copying).
func buildChunksFromSlice(chunks [][]byte, cumLens []int64, offset int64, length int64) [][]byte {
	if length == 0 {
		return [][]byte{{}}
	}
	if len(chunks) == 0 {
		return [][]byte{{}}
	}

	startIdx := sort.Search(len(cumLens), func(i int) bool {
		return cumLens[i] > offset
	}) - 1
	if startIdx < 0 {
		startIdx = 0
	}
	startOff := offset - cumLens[startIdx]

	endPos := offset + length - 1
	endIdx := sort.Search(len(cumLens), func(i int) bool {
		return cumLens[i] > endPos
	}) - 1
	if endIdx < 0 {
		endIdx = 0
	}
	endOff := endPos - cumLens[endIdx]

	newChunks := make([][]byte, 0, endIdx-startIdx+1)
	for i := startIdx; i <= endIdx; i++ {
		chunk := chunks[i]
		start := 0
		if i == startIdx {
			start = int(startOff)
		}
		end := len(chunk)
		if i == endIdx {
			end = int(endOff) + 1
		}
		newChunks = append(newChunks, chunk[start:end])
	}
	return newChunks
}

// splitIntoChunks splits data into chunks of at most chunkSize bytes.
// If data is empty, it returns a single empty chunk.
func splitIntoChunks(data []byte, chunkSize int) [][]byte {
	if len(data) == 0 {
		return [][]byte{{}}
	}
	n := (len(data) + chunkSize - 1) / chunkSize
	chunks := make([][]byte, n)
	for i := 0; i < n; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(data) {
			end = len(data)
		}
		chunks[i] = data[start:end]
	}
	return chunks
}

// ByteBuffersIndexOutput is an IndexOutput implementation for ByteBuffersDirectory
// with random-access write support via SetPosition.
type ByteBuffersIndexOutput struct {
	*spi.BaseIndexOutput
	BaseDataOutput
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
func (out *ByteBuffersIndexOutput) WriteBytes(b []byte, offset, length int) error {
	if !out.directory.IsOpen() {
		return ErrIllegalState
	}

	n := int64(length)
	pos := out.GetFilePointer()
	end := pos + n
	if end > int64(len(out.data)) {
		out.data = append(out.data, make([]byte, end-int64(len(out.data)))...)
	}
	copy(out.data[pos:end], b[offset:offset+length])
	out.IncrementFilePointer(n)
	return nil
}

// WriteBytesN writes exactly n bytes from b.
func (out *ByteBuffersIndexOutput) WriteBytesN(b []byte, n int) error {
	if n > len(b) {
		return fmt.Errorf("n exceeds buffer length")
	}
	return out.WriteBytes(b, 0, n)
}

// WriteShort writes a 16-bit value as little-endian to match Lucene 10.x
// DataOutput.writeShort (low byte first) and ByteBuffersIndexInput.ReadShort
// (already LE). See rmp #4786.
func (out *ByteBuffersIndexOutput) WriteShort(i int16) error {
	b := []byte{byte(i), byte(i >> 8)}
	return out.WriteBytes(b, 0, len(b))
}

// WriteInt writes a 32-bit value as little-endian to match Lucene 10.x
// DataOutput.writeInt (low byte first) and ByteBuffersIndexInput.ReadInt
// (already LE). See rmp #4786.
func (out *ByteBuffersIndexOutput) WriteInt(i int32) error {
	b := []byte{byte(i), byte(i >> 8), byte(i >> 16), byte(i >> 24)}
	return out.WriteBytes(b, 0, len(b))
}

// WriteLong writes a 64-bit value as little-endian to match Lucene 10.x
// DataOutput.writeLong (low byte first) and ByteBuffersIndexInput.ReadLong
// (already LE). See rmp #4786.
func (out *ByteBuffersIndexOutput) WriteLong(i int64) error {
	b := []byte{
		byte(i), byte(i >> 8), byte(i >> 16), byte(i >> 24),
		byte(i >> 32), byte(i >> 40), byte(i >> 48), byte(i >> 56),
	}
	return out.WriteBytes(b, 0, len(b))
}

// CopyBytes copies bytes from the given input into this output.
func (out *ByteBuffersIndexOutput) CopyBytes(input DataInput, numBytes int64) error {
	buf := make([]byte, 8192)
	remaining := numBytes
	for remaining > 0 {
		toRead := int(remaining)
		if toRead > len(buf) {
			toRead = len(buf)
		}
		if err := input.ReadBytes(buf, 0, toRead); err != nil {
			return err
		}
		if err := out.WriteBytes(buf, 0, toRead); err != nil {
			return err
		}
		remaining -= int64(toRead)
	}
	return nil
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
