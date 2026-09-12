// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/FlavioCFOliveira/Gocene/spi"
)

const (
	// Default buffer size.
	DefaultBufferSize = 1024
	// Minimum buffer size allowed.
	MinBufferSize = 8
	// A buffer size for merges.
	MergeBufferSize = 4096
)

// bufferedInternal defines the operations that a buffered input must implement
// to provide the underlying data and seek capabilities.
type bufferedInternal interface {
	ReadInternal(b []byte, offset int64) (int, error)
	SeekInternal(pos int64) error
	Length() int64
	Description() string
	Clone() bufferedInternal
	Close() error
}

// BufferedIndexInput provides a buffered implementation of IndexInput.
// It is a faithful port of org.apache.lucene.store.BufferedIndexInput.
type BufferedIndexInput struct {
	spi.BaseDataInput
	impl bufferedInternal

	bufferSize     int
	buffer         []byte
	bufferStart    int64
	bufferPosition int
	bufferLength   int
}

// NewBufferedIndexInput creates a new BufferedIndexInput with the specified buffer size.
func NewBufferedIndexInput(impl bufferedInternal, bufferSize int) (*BufferedIndexInput, error) {
	if bufferSize < MinBufferSize {
		return nil, fmt.Errorf("bufferSize must be at least MinBufferSize (got %d)", bufferSize)
	}
	if bufferSize <= 0 {
		bufferSize = DefaultBufferSize
	}

	in := &BufferedIndexInput{
		impl:       impl,
		bufferSize: bufferSize,
		buffer:     make([]byte, bufferSize),
	}
	in.Core = in // Self-implementation of DataInputCore
	return in, nil
}

// getBufferSize returns the current buffer size.
func (in *BufferedIndexInput) getBufferSize() int {
	return in.bufferSize
}

// ReadByte reads a single byte, using the buffer when possible.
func (in *BufferedIndexInput) ReadByte() (byte, error) {
	if in.bufferPosition >= in.bufferLength {
		if err := in.refill(); err != nil {
			return 0, err
		}
		if in.bufferLength == 0 {
			return 0, io.EOF
		}
	}
	b := in.buffer[in.bufferPosition]
	in.bufferPosition++
	return b, nil
}

// ReadBytes reads len(b) bytes into b.
func (in *BufferedIndexInput) ReadBytes(b []byte, offset, len int) error {
	return in.readBytes(b, offset, len, true)
}

// readBytes is the internal implementation of reading bytes with buffering options.
func (in *BufferedIndexInput) readBytes(b []byte, offset, len int, useBuffer bool) error {
	available := in.bufferLength - in.bufferPosition
	if len <= available {
		// the buffer contains enough data to satisfy this request
		if len > 0 {
			copy(b[offset:], in.buffer[in.bufferPosition:in.bufferPosition+len])
			in.bufferPosition += len
		}
	} else {
		// the buffer does not have enough data. First serve all we've got.
		if available > 0 {
			copy(b[offset:], in.buffer[in.bufferPosition:in.bufferPosition+available])
			offset += available
			len -= available
			in.bufferPosition += available
		}
		// and now, read the remaining 'len' bytes:
		if useBuffer && len < in.bufferSize {
			// If the amount left to read is small enough, and
			// we are allowed to use our buffer, do it in the usual
			// buffered way: fill the buffer and copy from it:
			if err := in.refill(); err != nil {
				return err
			}
			if in.bufferLength < len {
				// Throw an exception when refill() could not read len bytes:
				copy(b[offset:], in.buffer[:in.bufferLength])
				return fmt.Errorf("read past EOF: %s", in.impl.Description())
			}
			copy(b[offset:], in.buffer[:len])
			in.bufferPosition = len
		} else {
			// The amount left to read is larger than the buffer
			// or we've been asked to not use our buffer -
			// there's no performance reason not to read it all
			// at once.
			after := in.bufferStart + int64(in.bufferPosition) + int64(len)
			if after > in.impl.Length() {
				return fmt.Errorf("read past EOF: %s", in.impl.Description())
			}

			// Read directly into the slice
			n, err := in.impl.ReadInternal(b[offset : offset+len], in.GetFilePointer())
			if err != nil {
				return err
			}
			if n < len {
				return io.EOF
			}

			in.bufferStart = after
			in.bufferLength = 0
			in.bufferPosition = 0
		}
	}
	return nil
}

// ReadBytesN reads n bytes into a new slice.
func (in *BufferedIndexInput) ReadBytesN(n int) ([]byte, error) {
	out := make([]byte, n)
	if err := in.ReadBytes(out, 0, n); err != nil {
		return nil, err
	}
	return out, nil
}

// ReadShort reads a 16-bit value.
func (in *BufferedIndexInput) ReadShort() (int16, error) {
	if 2 <= (in.bufferLength - in.bufferPosition) {
		v := binary.LittleEndian.Uint16(in.buffer[in.bufferPosition:])
		in.bufferPosition += 2
		return int16(v), nil
	}
	// Fallback to base implementation or direct read if buffer is insufficient
	// In Java, this calls super.readShort(). In Gocene, we can't call super.
	// We can use ReadBytes to fill the buffer or read directly.
	buf := make([]byte, 2)
	if err := in.ReadBytes(buf, 0, 2); err != nil {
		return 0, err
	}
	return int16(binary.LittleEndian.Uint16(buf)), nil
}

// ReadInt reads a 32-bit value.
func (in *BufferedIndexInput) ReadInt() (int32, error) {
	if 4 <= (in.bufferLength - in.bufferPosition) {
		v := binary.LittleEndian.Uint32(in.buffer[in.bufferPosition:])
		in.bufferPosition += 4
		return int32(v), nil
	}
	buf := make([]byte, 4)
	if err := in.ReadBytes(buf, 0, 4); err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(buf)), nil
}

// ReadLong reads a 64-bit value.
func (in *BufferedIndexInput) ReadLong() (int64, error) {
	if 8 <= (in.bufferLength - in.bufferPosition) {
		v := binary.LittleEndian.Uint64(in.buffer[in.bufferPosition:])
		in.bufferPosition += 8
		return int64(v), nil
	}
	buf := make([]byte, 8)
	if err := in.ReadBytes(buf, 0, 8); err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(buf)), nil
}

// ReadFloats reads len floats into dst.
func (in *BufferedIndexInput) ReadFloats(dst []float32, offset, len int) error {
	remainingDst := len
	for remainingDst > 0 {
		cnt := (in.bufferLength - in.bufferPosition) / 4
		if cnt > remainingDst {
			cnt = remainingDst
		}
		if cnt > 0 {
			for i := 0; i < cnt; i++ {
				bits := binary.LittleEndian.Uint32(in.buffer[in.bufferPosition:])
				dst[offset+len-remainingDst] = math.Float32frombits(bits)
				in.bufferPosition += 4
			}
			remainingDst -= cnt
		}
		if remainingDst > 0 {
			if in.bufferPosition < in.bufferLength {
				v, err := in.ReadInt()
				if err != nil {
					return err
				}
				dst[offset+len-remainingDst] = math.Float32frombits(uint32(v))
				remainingDst--
			} else {
				if err := in.refill(); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// ReadLongs reads len longs into dst.
func (in *BufferedIndexInput) ReadLongs(dst []int64, offset, len int) error {
	remainingDst := len
	for remainingDst > 0 {
		cnt := (in.bufferLength - in.bufferPosition) / 8
		if cnt > remainingDst {
			cnt = remainingDst
		}
		if cnt > 0 {
			for i := 0; i < cnt; i++ {
				v := binary.LittleEndian.Uint64(in.buffer[in.bufferPosition:])
				dst[offset+len-remainingDst] = int64(v)
				in.bufferPosition += 8
			}
			remainingDst -= cnt
		}
		if remainingDst > 0 {
			if in.bufferPosition < in.bufferLength {
				v, err := in.ReadLong()
				if err != nil {
					return err
				}
				dst[offset+len-remainingDst] = v
				remainingDst--
			} else {
				if err := in.refill(); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// SkipBytes skips n bytes forward.
func (in *BufferedIndexInput) SkipBytes(n int64) error {
	if n < 0 {
		return fmt.Errorf("cannot skip negative bytes: %d", n)
	}
	// If we are skipping a small amount and it's within the current buffer, just move the pointer.
	if in.bufferPosition+int(n) <= in.bufferLength {
		in.bufferPosition += int(n)
		return nil
	}

	// Otherwise, we can either read through the buffer (to maintain buffering)
	// or seek the underlying implementation.
	// To keep it simple and consistent with Lucene, we seek the impl.
	in.bufferLength = 0 // invalidate buffer
	in.bufferPosition = 0
	newPos := in.GetFilePointer() + n
	return in.impl.SeekInternal(newPos)
}

// Close closes the underlying input.
func (in *BufferedIndexInput) Close() error {
	return in.impl.Close()
}

// ReadInts reads len ints into dst.
func (in *BufferedIndexInput) ReadInts(dst []int32, offset, len int) error {
	remainingDst := len
	for remainingDst > 0 {
		cnt := (in.bufferLength - in.bufferPosition) / 4
		if cnt > remainingDst {
			cnt = remainingDst
		}
		if cnt > 0 {
			for i := 0; i < cnt; i++ {
				v := binary.LittleEndian.Uint32(in.buffer[in.bufferPosition:])
				dst[offset+len-remainingDst] = int32(v)
				in.bufferPosition += 4
			}
			remainingDst -= cnt
		}
		if remainingDst > 0 {
			if in.bufferPosition < in.bufferLength {
				v, err := in.ReadInt()
				if err != nil {
					return err
				}
				dst[offset+len-remainingDst] = v
				remainingDst--
			} else {
				if err := in.refill(); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// ReadVInt reads a variable-length integer.
func (in *BufferedIndexInput) ReadVInt() (int32, error) {
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

// ReadVLong reads a variable-length long.
func (in *BufferedIndexInput) ReadVLong() (int64, error) {
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

// resolvePositionInBuffer computes an offset into the current buffer from an absolute position.
func (in *BufferedIndexInput) resolvePositionInBuffer(pos int64, width int) (int, error) {
	index := pos - in.bufferStart
	if index >= 0 && index <= int64(in.bufferLength-width) {
		return int(index), nil
	}
	if index < 0 {
		// if we're moving backwards, try and fill up the previous page
		in.bufferStart = in.bufferStart - int64(in.bufferSize)
		if in.bufferStart < pos+int64(width)-int64(in.bufferSize) {
			in.bufferStart = pos + int64(width) - int64(in.bufferSize)
		}
		if in.bufferStart < 0 {
			in.bufferStart = 0
		}
		if in.bufferStart > pos {
			in.bufferStart = pos
		}
	} else {
		// moving forwards, reset buffer to start at pos
		in.bufferStart = pos
	}
	in.bufferLength = 0 // trigger refill()
	if err := in.impl.SeekInternal(in.bufferStart); err != nil {
		return 0, err
	}
	if err := in.refill(); err != nil {
		return 0, err
	}
	return int(pos - in.bufferStart), nil
}

// ReadByteAt reads a byte at the given absolute position.
func (in *BufferedIndexInput) ReadByteAt(pos int64) (byte, error) {
	index, err := in.resolvePositionInBuffer(pos, 1)
	if err != nil {
		return 0, err
	}
	return in.buffer[index], nil
}

// ReadBytesAt reads len bytes at the given absolute position into bytes.
func (in *BufferedIndexInput) ReadBytesAt(pos int64, bytes []byte, offset, len int) error {
	if len <= in.bufferSize {
		if len > 0 {
			index, err := in.resolvePositionInBuffer(pos, len)
			if err != nil {
				return err
			}
			copy(bytes[offset:], in.buffer[index:index+len])
		}
	} else {
		for len > in.bufferSize {
			index, err := in.resolvePositionInBuffer(pos, in.bufferSize)
			if err != nil {
				return err
			}
			copy(bytes[offset:], in.buffer[index:index+in.bufferSize])
			len -= in.bufferSize
			offset += in.bufferSize
			pos += int64(in.bufferSize)
		}
		index, err := in.resolvePositionInBuffer(pos, len)
		if err != nil {
			return err
		}
		copy(bytes[offset:], in.buffer[index:index+len])
	}
	return nil
}

// ReadShortAt reads a 16-bit little-endian value at the given absolute position.
func (in *BufferedIndexInput) ReadShortAt(pos int64) (int16, error) {
	index, err := in.resolvePositionInBuffer(pos, 2)
	if err != nil {
		return 0, err
	}
	return int16(binary.LittleEndian.Uint16(in.buffer[index:])), nil
}

// ReadIntAt reads a 32-bit little-endian value at the given absolute position.
func (in *BufferedIndexInput) ReadIntAt(pos int64) (int32, error) {
	index, err := in.resolvePositionInBuffer(pos, 4)
	if err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(in.buffer[index:])), nil
}

// ReadLongAt reads a 64-bit little-endian value at the given absolute position.
func (in *BufferedIndexInput) ReadLongAt(pos int64) (int64, error) {
	index, err := in.resolvePositionInBuffer(pos, 8)
	if err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(in.buffer[index:])), nil
}

// refill fills the buffer from the underlying source.
func (in *BufferedIndexInput) refill() error {
	start := in.bufferStart + int64(in.bufferPosition)
	end := start + int64(in.bufferSize)
	if end > in.impl.Length() {
		end = in.impl.Length()
	}
	newLength := int(end - start)
	if newLength <= 0 {
		return fmt.Errorf("read past EOF: %s", in.impl.Description())
	}

	n, err := in.impl.ReadInternal(in.buffer, start)
	if err != nil {
		return err
	}
	if n < newLength {
		return io.EOF
	}

	in.bufferStart = start
	in.bufferLength = n
	in.bufferPosition = 0
	return nil
}

// GetFilePointer returns the current position in the input.
func (in *BufferedIndexInput) GetFilePointer() int64 {
	return in.bufferStart + int64(in.bufferPosition)
}

// Length returns the total length of the input.
func (in *BufferedIndexInput) Length() int64 {
	return in.impl.Length()
}

// SetPosition changes the current position.
func (in *BufferedIndexInput) SetPosition(pos int64) error {
	if pos >= in.bufferStart && pos < (in.bufferStart+int64(in.bufferLength)) {
		in.bufferPosition = int(pos - in.bufferStart)
	} else {
		in.bufferStart = pos
		in.bufferLength = 0
		in.bufferPosition = 0
	}
	return in.impl.SeekInternal(pos)
}

// Clone returns a clone of this BufferedIndexInput.
func (in *BufferedIndexInput) Clone() IndexInput {
	implClone := in.impl.Clone()
	clone, err := NewBufferedIndexInput(implClone, in.bufferSize)
	if err != nil {
		// This should not happen with valid bufferSize
		return nil
	}
	clone.bufferStart = in.bufferStart
	clone.bufferPosition = in.bufferPosition
	clone.bufferLength = in.bufferLength
	copy(clone.buffer, in.buffer)
	return clone
}

// Slice returns a subset of this IndexInput.
func (in *BufferedIndexInput) Slice(desc string, offset int64, length int64) (IndexInput, error) {
	// Wraps a portion of another IndexInput with buffering.
	return NewSlicedBufferedIndexInput(desc, in, offset, length)
}

// NewSlicedBufferedIndexInput creates a SlicedIndexInput.
func NewSlicedBufferedIndexInput(desc string, base IndexInput, offset int64, length int64) (IndexInput, error) {
	if (length|offset) < 0 || length > base.Length()-offset {
		return nil, fmt.Errorf("slice() %s out of bounds", desc)
	}

	sliced := &slicedBufferedInternal{
		base:       base.Clone(),
		fileOffset: offset,
		length:     length,
	}

	return NewBufferedIndexInput(sliced, DefaultBufferSize)
}

type slicedBufferedInternal struct {
	base       IndexInput
	fileOffset int64
	length     int64
}

func (s *slicedBufferedInternal) ReadInternal(b []byte, offset int64) (int, error) {
	if offset+int64(len(b)) > s.length {
		return 0, fmt.Errorf("read past EOF: %s", s.Description())
	}
	s.base.SetPosition(s.fileOffset + offset)
	err := s.base.ReadBytes(b, 0, len(b))
	if err != nil {
		return 0, err
	}
	return len(b), nil
}

func (s *slicedBufferedInternal) SeekInternal(pos int64) error {
	return nil // Seek is handled by the wrapper's buffer management
}

func (s *slicedBufferedInternal) Length() int64 {
	return s.length
}

func (s *slicedBufferedInternal) Description() string {
	return "sliced buffered input"
}

func (s *slicedBufferedInternal) Close() error {
	return s.base.Close()
}

func (s *slicedBufferedInternal) Clone() bufferedInternal {
	return &slicedBufferedInternal{
		base:       s.base.Clone(),
		fileOffset: s.fileOffset,
		length:     s.length,
	}
}
