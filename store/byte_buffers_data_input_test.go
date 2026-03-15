// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/test/org/apache/lucene/store/TestByteBuffersDataInput.java
// Purpose: Tests for ByteBuffersDataInput - position tracking, EOF exceptions,
// slice correctness, and large buffer handling.

package store

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/rand"
	"testing"
	"time"
)

// ByteBuffersDataInput is a DataInput implementation that reads from a list of byte buffers.
// This interface defines the expected behavior based on Lucene's ByteBuffersDataInput.
type ByteBuffersDataInput interface {
	DataInput
	// Length returns the total number of bytes available.
	Length() int64
	// Position returns the current position in the input.
	Position() int64
	// Seek sets the current position.
	Seek(pos int64) error
	// SkipBytes skips the specified number of bytes.
	SkipBytes(n int64) error
	// Slice returns a slice of this input starting at offset with the given length.
	Slice(offset int64, length int64) (ByteBuffersDataInput, error)
	// ReadByteAt reads a byte at the specified position without changing the current position.
	ReadByteAt(pos int64) (byte, error)
	// RamBytesUsed returns an estimate of the memory used by this input.
	RamBytesUsed() int64
}

// byteBuffersDataInputImpl is a mock implementation of ByteBuffersDataInput for testing.
// This will be replaced by the actual implementation when available.
type byteBuffersDataInputImpl struct {
	data   []byte
	pos    int64
	offset int64
}

// newByteBuffersDataInput creates a new ByteBuffersDataInput from the given data.
func newByteBuffersDataInput(data []byte) ByteBuffersDataInput {
	return &byteBuffersDataInputImpl{
		data:   data,
		pos:    0,
		offset: 0,
	}
}

func (in *byteBuffersDataInputImpl) ReadByte() (byte, error) {
	if in.pos >= int64(len(in.data)) {
		return 0, io.EOF
	}
	b := in.data[in.pos]
	in.pos++
	return b, nil
}

func (in *byteBuffersDataInputImpl) ReadBytes(b []byte) error {
	if in.pos+int64(len(b)) > int64(len(in.data)) {
		return io.EOF
	}
	copy(b, in.data[in.pos:in.pos+int64(len(b))])
	in.pos += int64(len(b))
	return nil
}

func (in *byteBuffersDataInputImpl) ReadBytesN(n int) ([]byte, error) {
	if in.pos+int64(n) > int64(len(in.data)) {
		return nil, io.EOF
	}
	result := make([]byte, n)
	copy(result, in.data[in.pos:in.pos+int64(n)])
	in.pos += int64(n)
	return result, nil
}

func (in *byteBuffersDataInputImpl) ReadShort() (int16, error) {
	buf := make([]byte, 2)
	if err := in.ReadBytes(buf); err != nil {
		return 0, err
	}
	return int16(binary.LittleEndian.Uint16(buf)), nil
}

func (in *byteBuffersDataInputImpl) ReadInt() (int32, error) {
	buf := make([]byte, 4)
	if err := in.ReadBytes(buf); err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(buf)), nil
}

func (in *byteBuffersDataInputImpl) ReadLong() (int64, error) {
	buf := make([]byte, 8)
	if err := in.ReadBytes(buf); err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(buf)), nil
}

func (in *byteBuffersDataInputImpl) ReadString() (string, error) {
	length, err := in.ReadInt()
	if err != nil {
		return "", err
	}
	buf := make([]byte, length)
	if err := in.ReadBytes(buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func (in *byteBuffersDataInputImpl) Length() int64 {
	return int64(len(in.data))
}

func (in *byteBuffersDataInputImpl) Position() int64 {
	return in.pos
}

func (in *byteBuffersDataInputImpl) Seek(pos int64) error {
	if pos > int64(len(in.data)) {
		in.pos = int64(len(in.data))
		return io.EOF
	}
	in.pos = pos
	return nil
}

func (in *byteBuffersDataInputImpl) SkipBytes(n int64) error {
	if n < 0 {
		return fmt.Errorf("numBytes must be >= 0, got %d", n)
	}
	return in.Seek(in.pos + n)
}

func (in *byteBuffersDataInputImpl) Slice(offset int64, length int64) (ByteBuffersDataInput, error) {
	if offset < 0 || length < 0 || offset+length > int64(len(in.data)) {
		return nil, fmt.Errorf("slice(offset=%d, length=%d) is out of bounds", offset, length)
	}
	return &byteBuffersDataInputImpl{
		data:   in.data[offset : offset+length],
		pos:    0,
		offset: offset,
	}, nil
}

func (in *byteBuffersDataInputImpl) ReadByteAt(pos int64) (byte, error) {
	if pos >= int64(len(in.data)) {
		return 0, io.EOF
	}
	return in.data[pos], nil
}

func (in *byteBuffersDataInputImpl) RamBytesUsed() int64 {
	return int64(len(in.data))
}

// toByteBuffersDataInput converts a DataInput to ByteBuffersDataInput.
// This is a helper function for testing.
func toByteBuffersDataInput(di DataInput) ByteBuffersDataInput {
	// For now, we read all data and create a new ByteBuffersDataInput
	// In the actual implementation, this would be a direct conversion
	if badi, ok := di.(*ByteArrayDataInput); ok {
		// Get the underlying data
		data := make([]byte, badi.Length())
		badi.SetPosition(0)
		for i := 0; i < len(data); i++ {
			b, _ := badi.ReadByte()
			data[i] = b
		}
		return newByteBuffersDataInput(data)
	}
	return nil
}

// TestByteBuffersDataInput_Sanity tests basic sanity checks for ByteBuffersDataInput.
// Source: TestByteBuffersDataInput.testSanity()
func TestByteBuffersDataInput_Sanity(t *testing.T) {
	out := NewByteBuffersDataOutput()
	o1 := toByteBuffersDataInput(out.ToDataInput())

	if o1.Length() != 0 {
		t.Errorf("expected length 0, got %d", o1.Length())
	}

	// Reading from empty should return EOF
	_, err := o1.ReadByte()
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}

	// Write a byte
	out.WriteByte(1)

	o2 := toByteBuffersDataInput(out.ToDataInput())
	if o2.Length() != 1 {
		t.Errorf("expected length 1, got %d", o2.Length())
	}
	if o2.Position() != 0 {
		t.Errorf("expected position 0, got %d", o2.Position())
	}
	if o1.Length() != 0 {
		t.Errorf("o1 length should still be 0, got %d", o1.Length())
	}

	if o2.RamBytesUsed() <= 0 {
		t.Error("expected ramBytesUsed > 0")
	}

	b, err := o2.ReadByte()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b != 1 {
		t.Errorf("expected byte 1, got %d", b)
	}
	if o2.Position() != 1 {
		t.Errorf("expected position 1, got %d", o2.Position())
	}

	// Read at specific position
	readAt, err := o2.ReadByteAt(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if readAt != 1 {
		t.Errorf("expected byte 1 at position 0, got %d", readAt)
	}

	// Reading past end should return EOF
	_, err = o2.ReadByte()
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}

	if o2.Position() != 1 {
		t.Errorf("position should still be 1, got %d", o2.Position())
	}
}

// TestByteBuffersDataInput_RandomReads tests random read operations.
// Source: TestByteBuffersDataInput.testRandomReads()
func TestByteBuffersDataInput_RandomReads(t *testing.T) {
	out := NewByteBuffersDataOutput()

	// Generate random data
	max := 1000
	if testing.Short() {
		max = 100
	}

	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	data := make([]byte, max)
	rnd.Read(data)
	out.WriteBytes(data)

	src := toByteBuffersDataInput(out.ToDataInput())
	readBuf := make([]byte, max)
	err := src.ReadBytes(readBuf)
	if err != nil {
		t.Fatalf("unexpected error reading bytes: %v", err)
	}

	if !bytes.Equal(readBuf, data) {
		t.Error("read data does not match written data")
	}

	// Reading past end should return EOF
	_, err = src.ReadByte()
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
}

// TestByteBuffersDataInput_RandomReadsOnSlices tests random reads on sliced inputs.
// Source: TestByteBuffersDataInput.testRandomReadsOnSlices()
func TestByteBuffersDataInput_RandomReadsOnSlices(t *testing.T) {
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	for reps := rnd.Intn(20) + 1; reps > 0; reps-- {
		out := NewByteBuffersDataOutput()

		// Write prefix
		prefixLen := rnd.Intn(1024 * 8)
		prefix := make([]byte, prefixLen)
		rnd.Read(prefix)
		out.WriteBytes(prefix)

		// Write main data
		max := 500
		if testing.Short() {
			max = 50
		}
		mainData := make([]byte, max)
		rnd.Read(mainData)
		out.WriteBytes(mainData)

		// Write suffix
		suffixLen := rnd.Intn(1024 * 8)
		suffix := make([]byte, suffixLen)
		rnd.Read(suffix)
		out.WriteBytes(suffix)

		totalSize := out.Size()
		sliceLen := totalSize - int64(prefixLen) - int64(suffixLen)

		src, err := toByteBuffersDataInput(out.ToDataInput()).Slice(int64(prefixLen), sliceLen)
		if err != nil {
			t.Fatalf("unexpected error creating slice: %v", err)
		}

		if src.Position() != 0 {
			t.Errorf("expected position 0, got %d", src.Position())
		}
		if src.Length() != sliceLen {
			t.Errorf("expected length %d, got %d", sliceLen, src.Length())
		}

		// Read all data from slice
		readBuf := make([]byte, sliceLen)
		err = src.ReadBytes(readBuf)
		if err != nil {
			t.Fatalf("unexpected error reading bytes: %v", err)
		}

		if !bytes.Equal(readBuf, mainData) {
			t.Error("read data does not match expected main data")
		}

		// Reading past end should return EOF
		_, err = src.ReadByte()
		if err != io.EOF {
			t.Errorf("expected EOF, got %v", err)
		}
	}
}

// TestByteBuffersDataInput_SeekEmpty tests seeking on an empty input.
// Source: TestByteBuffersDataInput.testSeekEmpty()
func TestByteBuffersDataInput_SeekEmpty(t *testing.T) {
	out := NewByteBuffersDataOutput()
	in := toByteBuffersDataInput(out.ToDataInput())

	err := in.Seek(0)
	if err != nil {
		t.Fatalf("unexpected error seeking to 0: %v", err)
	}

	// Seeking past end should return EOF
	err = in.Seek(1)
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}

	err = in.Seek(0)
	if err != nil {
		t.Fatalf("unexpected error seeking to 0: %v", err)
	}

	// Reading from empty should return EOF
	_, err = in.ReadByte()
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
}

// TestByteBuffersDataInput_SeekAndSkip tests seeking and skipping operations.
// Source: TestByteBuffersDataInput.testSeekAndSkip()
func TestByteBuffersDataInput_SeekAndSkip(t *testing.T) {
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	for reps := rnd.Intn(200) + 1; reps > 0; reps-- {
		out := NewByteBuffersDataOutput()

		// Optional prefix
		var prefixLen int
		if rnd.Intn(2) == 1 {
			prefixLen = rnd.Intn(1024*8) + 1
			prefix := make([]byte, prefixLen)
			rnd.Read(prefix)
			out.WriteBytes(prefix)
		}

		// Write main data
		max := 1000
		if testing.Short() {
			max = 100
		}
		mainData := make([]byte, max)
		rnd.Read(mainData)
		out.WriteBytes(mainData)

		totalSize := out.Size()
		sliceLen := totalSize - int64(prefixLen)

		in, err := toByteBuffersDataInput(out.ToDataInput()).Slice(int64(prefixLen), sliceLen)
		if err != nil {
			t.Fatalf("unexpected error creating slice: %v", err)
		}

		// Test seeking to 0 and reading all
		err = in.Seek(0)
		if err != nil {
			t.Fatalf("unexpected error seeking to 0: %v", err)
		}

		readBuf := make([]byte, sliceLen)
		err = in.ReadBytes(readBuf)
		if err != nil {
			t.Fatalf("unexpected error reading bytes: %v", err)
		}

		if !bytes.Equal(readBuf, mainData) {
			t.Error("read data does not match expected data")
		}

		// Test seeking to 0 again
		err = in.Seek(0)
		if err != nil {
			t.Fatalf("unexpected error seeking to 0: %v", err)
		}

		// Test random seeks
		for i := 0; i < 100 && i < int(sliceLen); i++ {
			offs := rnd.Int63n(sliceLen)
			err = in.Seek(offs)
			if err != nil {
				t.Fatalf("unexpected error seeking to %d: %v", offs, err)
			}
			if in.Position() != offs {
				t.Errorf("expected position %d, got %d", offs, in.Position())
			}

			b, err := in.ReadByte()
			if err != nil {
				t.Fatalf("unexpected error reading byte at %d: %v", offs, err)
			}
			if b != mainData[offs] {
				t.Errorf("expected byte %d at position %d, got %d", mainData[offs], offs, b)
			}
		}

		// Test skipping
		err = in.Seek(0)
		if err != nil {
			t.Fatalf("unexpected error seeking to 0: %v", err)
		}

		maxSkipTo := sliceLen - 1
		var curr int64 = 0
		for curr < maxSkipTo {
			skipTo := rnd.Int63n(maxSkipTo-curr) + curr
			step := skipTo - curr
			err = in.SkipBytes(step)
			if err != nil {
				t.Fatalf("unexpected error skipping %d bytes: %v", step, err)
			}

			b, err := in.ReadByte()
			if err != nil {
				t.Fatalf("unexpected error reading byte: %v", err)
			}
			if b != mainData[skipTo] {
				t.Errorf("expected byte %d at position %d, got %d", mainData[skipTo], skipTo, b)
			}
			curr = skipTo + 1
		}

		// Seek to end and verify EOF
		err = in.Seek(in.Length())
		if err != nil {
			t.Fatalf("unexpected error seeking to end: %v", err)
		}
		if in.Position() != in.Length() {
			t.Errorf("expected position %d, got %d", in.Length(), in.Position())
		}

		_, err = in.ReadByte()
		if err != io.EOF {
			t.Errorf("expected EOF, got %v", err)
		}
	}
}

// TestByteBuffersDataInput_SlicingWindow tests slicing with various window sizes.
// Source: TestByteBuffersDataInput.testSlicingWindow()
func TestByteBuffersDataInput_SlicingWindow(t *testing.T) {
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	out := NewByteBuffersDataOutput()

	// Empty slice should have length 0
	slice, err := toByteBuffersDataInput(out.ToDataInput()).Slice(0, 0)
	if err != nil {
		t.Fatalf("unexpected error creating empty slice: %v", err)
	}
	if slice.Length() != 0 {
		t.Errorf("expected length 0, got %d", slice.Length())
	}

	// Write some data
	dataLen := 1024 * 8
	data := make([]byte, dataLen)
	rnd.Read(data)
	out.WriteBytes(data)

	in := toByteBuffersDataInput(out.ToDataInput())
	max := out.Size()

	for offset := int64(0); offset < max; offset++ {
		slice0, err := in.Slice(offset, 0)
		if err != nil {
			t.Fatalf("unexpected error creating slice at offset %d: %v", offset, err)
		}
		if slice0.Length() != 0 {
			t.Errorf("expected length 0 for slice at offset %d, got %d", offset, slice0.Length())
		}

		slice1, err := in.Slice(offset, 1)
		if err != nil {
			t.Fatalf("unexpected error creating slice at offset %d: %v", offset, err)
		}
		if slice1.Length() != 1 {
			t.Errorf("expected length 1 for slice at offset %d, got %d", offset, slice1.Length())
		}

		window := int64(1024)
		if max-offset < window {
			window = max - offset
		}
		sliceWindow, err := in.Slice(offset, window)
		if err != nil {
			t.Fatalf("unexpected error creating slice at offset %d: %v", offset, err)
		}
		if sliceWindow.Length() != window {
			t.Errorf("expected length %d for slice at offset %d, got %d", window, offset, sliceWindow.Length())
		}
	}

	// Slice at end with length 0
	sliceEnd, err := in.Slice(max, 0)
	if err != nil {
		t.Fatalf("unexpected error creating slice at end: %v", err)
	}
	if sliceEnd.Length() != 0 {
		t.Errorf("expected length 0 for slice at end, got %d", sliceEnd.Length())
	}
}

// TestByteBuffersDataInput_EofOnArrayReadPastBufferSize tests EOF when reading past buffer size.
// Source: TestByteBuffersDataInput.testEofOnArrayReadPastBufferSize()
func TestByteBuffersDataInput_EofOnArrayReadPastBufferSize(t *testing.T) {
	out := NewByteBuffersDataOutput()
	out.WriteBytes(make([]byte, 10))

	// Try to read more bytes than available
	in := toByteBuffersDataInput(out.ToDataInput())
	buf := make([]byte, 100)
	err := in.ReadBytes(buf)
	if err != io.EOF {
		t.Errorf("expected EOF when reading past buffer, got %v", err)
	}
}

// TestByteBuffersDataInput_SlicingLargeBuffers tests slicing with large buffers (> 4GB simulation).
// Source: TestByteBuffersDataInput.testSlicingLargeBuffers()
func TestByteBuffersDataInput_SlicingLargeBuffers(t *testing.T) {
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Simulate a "large" (> 4GB) input by using a smaller page
	MB := 1024 * 1024
	pageBytes := make([]byte, 4*MB)
	rnd.Read(pageBytes)

	// Add some head shift
	shift := rnd.Intn(len(pageBytes) / 2)

	// Simulate large length (we'll use a smaller one for testing)
	simulatedLength := int64(rnd.Intn(2018)) + 4*int64(math.MaxInt32)
	if testing.Short() {
		simulatedLength = int64(rnd.Intn(100)) + int64(4*MB)
	}

	// Create input with repeated page content
	var data []byte
	remaining := simulatedLength + int64(shift)
	for remaining > 0 {
		chunk := int64(len(pageBytes))
		if chunk > remaining {
			chunk = remaining
		}
		data = append(data, pageBytes[:chunk]...)
		remaining -= chunk
	}
	data = data[shift:]

	out := NewByteBuffersDataOutput()
	out.WriteBytes(data)

	in := toByteBuffersDataInput(out.ToDataInput())
	if in.Length() != simulatedLength {
		t.Errorf("expected length %d, got %d", simulatedLength, in.Length())
	}

	max := in.Length()
	var offset int64
	for offset < max {
		chunkSize := int64(rnd.Intn(3*MB) + MB)
		if chunkSize > max-offset {
			chunkSize = max - offset
		}

		slice0, err := in.Slice(offset, 0)
		if err != nil {
			t.Fatalf("unexpected error creating slice at offset %d: %v", offset, err)
		}
		if slice0.Length() != 0 {
			t.Errorf("expected length 0, got %d", slice0.Length())
		}

		slice1, err := in.Slice(offset, 1)
		if err != nil {
			t.Fatalf("unexpected error creating slice at offset %d: %v", offset, err)
		}
		if slice1.Length() != 1 {
			t.Errorf("expected length 1, got %d", slice1.Length())
		}

		window := int64(1024)
		if max-offset < window {
			window = max - offset
		}
		slice, err := in.Slice(offset, window)
		if err != nil {
			t.Fatalf("unexpected error creating slice at offset %d: %v", offset, err)
		}
		if slice.Length() != window {
			t.Errorf("expected length %d, got %d", window, slice.Length())
		}

		// Sanity check content
		for i := int64(0); i < window && i < 100; i++ {
			expected := pageBytes[(int64(shift)+offset+i)%int64(len(pageBytes))]
			actual, err := slice.ReadByteAt(i)
			if err != nil {
				t.Fatalf("unexpected error reading byte at %d: %v", i, err)
			}
			if actual != expected {
				t.Errorf("expected byte %d at position %d, got %d", expected, i, actual)
			}
		}

		offset += chunkSize
	}
}

// TestByteBuffersDataInput_PositionTracking tests position tracking functionality.
func TestByteBuffersDataInput_PositionTracking(t *testing.T) {
	out := NewByteBuffersDataOutput()
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	out.WriteBytes(data)

	in := toByteBuffersDataInput(out.ToDataInput())

	if in.Position() != 0 {
		t.Errorf("expected initial position 0, got %d", in.Position())
	}

	// Read one byte
	_, err := in.ReadByte()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if in.Position() != 1 {
		t.Errorf("expected position 1, got %d", in.Position())
	}

	// Read multiple bytes
	buf := make([]byte, 2)
	err = in.ReadBytes(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if in.Position() != 3 {
		t.Errorf("expected position 3, got %d", in.Position())
	}

	// Seek
	err = in.Seek(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if in.Position() != 1 {
		t.Errorf("expected position 1 after seek, got %d", in.Position())
	}
}

// TestByteBuffersDataInput_ReadByteAt tests reading at specific positions.
func TestByteBuffersDataInput_ReadByteAt(t *testing.T) {
	out := NewByteBuffersDataOutput()
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	out.WriteBytes(data)

	in := toByteBuffersDataInput(out.ToDataInput())

	for i, expected := range data {
		actual, err := in.ReadByteAt(int64(i))
		if err != nil {
			t.Fatalf("unexpected error at position %d: %v", i, err)
		}
		if actual != expected {
			t.Errorf("expected byte %d at position %d, got %d", expected, i, actual)
		}
	}

	// Position should not change
	if in.Position() != 0 {
		t.Errorf("position should still be 0, got %d", in.Position())
	}

	// Read past end should return EOF
	_, err := in.ReadByteAt(int64(len(data)))
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
}

// TestByteBuffersDataInput_SliceBounds tests slice boundary conditions.
func TestByteBuffersDataInput_SliceBounds(t *testing.T) {
	out := NewByteBuffersDataOutput()
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A}
	out.WriteBytes(data)

	in := toByteBuffersDataInput(out.ToDataInput())

	// Valid slice
	slice, err := in.Slice(2, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if slice.Length() != 5 {
		t.Errorf("expected length 5, got %d", slice.Length())
	}

	// Read from slice
	buf := make([]byte, 5)
	err = slice.ReadBytes(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []byte{0x03, 0x04, 0x05, 0x06, 0x07}
	if !bytes.Equal(buf, expected) {
		t.Errorf("expected %v, got %v", expected, buf)
	}

	// Negative offset should error
	_, err = in.Slice(-1, 5)
	if err == nil {
		t.Error("expected error for negative offset")
	}

	// Negative length should error
	_, err = in.Slice(0, -1)
	if err == nil {
		t.Error("expected error for negative length")
	}

	// Slice past end should error
	_, err = in.Slice(5, 10)
	if err == nil {
		t.Error("expected error for slice past end")
	}
}

// TestByteBuffersDataInput_MultiByteReads tests reading multi-byte values.
func TestByteBuffersDataInput_MultiByteReads(t *testing.T) {
	out := NewByteBuffersDataOutput()

	// Write various multi-byte values
	out.WriteShort(0x1234)
	out.WriteInt(0x12345678)
	out.WriteLong(0x123456789ABCDEF0)

	in := toByteBuffersDataInput(out.ToDataInput())

	// Read short
	shortVal, err := in.ReadShort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shortVal != 0x1234 {
		t.Errorf("expected 0x1234, got 0x%04x", shortVal)
	}

	// Read int
	intVal, err := in.ReadInt()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if intVal != 0x12345678 {
		t.Errorf("expected 0x12345678, got 0x%08x", intVal)
	}

	// Read long
	longVal, err := in.ReadLong()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if longVal != 0x123456789ABCDEF0 {
		t.Errorf("expected 0x123456789ABCDEF0, got 0x%016x", longVal)
	}
}

// TestByteBuffersDataInput_EmptyInput tests behavior with empty input.
func TestByteBuffersDataInput_EmptyInput(t *testing.T) {
	out := NewByteBuffersDataOutput()
	in := toByteBuffersDataInput(out.ToDataInput())

	if in.Length() != 0 {
		t.Errorf("expected length 0, got %d", in.Length())
	}

	if in.Position() != 0 {
		t.Errorf("expected position 0, got %d", in.Position())
	}

	// All read operations should return EOF
	_, err := in.ReadByte()
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}

	err = in.ReadBytes(make([]byte, 1))
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}

	// Seek to 0 should work
	err = in.Seek(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Seek past end should return EOF
	err = in.Seek(1)
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
}

// TestByteBuffersDataInput_SingleByte tests reading a single byte.
func TestByteBuffersDataInput_SingleByte(t *testing.T) {
	out := NewByteBuffersDataOutput()
	out.WriteByte(0x42)

	in := toByteBuffersDataInput(out.ToDataInput())

	b, err := in.ReadByte()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b != 0x42 {
		t.Errorf("expected 0x42, got 0x%02x", b)
	}

	// Second read should return EOF
	_, err = in.ReadByte()
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
}

// TestByteBuffersDataInput_RamBytesUsed tests RAM usage estimation.
func TestByteBuffersDataInput_RamBytesUsed(t *testing.T) {
	out := NewByteBuffersDataOutput()
	in := toByteBuffersDataInput(out.ToDataInput())

	// Empty input should use minimal RAM
	if in.RamBytesUsed() != 0 {
		t.Errorf("expected 0 ram bytes for empty input, got %d", in.RamBytesUsed())
	}

	// Write some data
	out.WriteBytes(make([]byte, 1000))
	in = toByteBuffersDataInput(out.ToDataInput())

	if in.RamBytesUsed() <= 0 {
		t.Error("expected ramBytesUsed > 0 for non-empty input")
	}

	if in.RamBytesUsed() < 1000 {
		t.Errorf("expected ramBytesUsed >= 1000, got %d", in.RamBytesUsed())
	}
}

// TestByteBuffersDataInput_SkipBytes tests the SkipBytes functionality.
func TestByteBuffersDataInput_SkipBytes(t *testing.T) {
	out := NewByteBuffersDataOutput()
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	out.WriteBytes(data)

	in := toByteBuffersDataInput(out.ToDataInput())

	// Skip 2 bytes
	err := in.SkipBytes(2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if in.Position() != 2 {
		t.Errorf("expected position 2, got %d", in.Position())
	}

	// Read next byte
	b, err := in.ReadByte()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b != 0x03 {
		t.Errorf("expected 0x03, got 0x%02x", b)
	}

	// Skip past end should return EOF
	err = in.SkipBytes(10)
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}

	// Negative skip should error
	in.Seek(0)
	err = in.SkipBytes(-1)
	if err == nil {
		t.Error("expected error for negative skip")
	}
}

// TestByteBuffersDataInput_ReadString tests reading strings.
func TestByteBuffersDataInput_ReadString(t *testing.T) {
	out := NewByteBuffersDataOutput()
	out.WriteString("Hello, World!")

	in := toByteBuffersDataInput(out.ToDataInput())

	str, err := in.ReadString()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if str != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got '%s'", str)
	}
}
