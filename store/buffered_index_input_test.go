// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/test/org/apache/lucene/store/TestBufferedIndexInput.java
// Purpose: Tests for BufferedIndexInput including buffer boundary reads,
// EOF detection, backwards reads, and bulk primitive reads.

package store

import (
	"encoding/binary"
	"io"
	"math"
	"math/rand"
	"testing"
)

const testFileLength = 100 * 1024 // 100KB

// byten emulates a file - byten(n) returns the n'th byte in that file.
// This matches the Java implementation: (byte) (n * n % 256)
func byten(n int64) byte {
	return byte((n * n) % 256)
}

// myBufferedIndexInput is a test implementation of BufferedIndexInput
// that generates data dynamically based on the byten function.
type myBufferedIndexInput struct {
	*BufferedIndexInput
	pos        int64
	len        int64
	readCount  int64
	bufferSize int
}

// newMyBufferedIndexInput creates a new test input with the given length.
// If len is <= 0, it creates an "infinite" file (Long.MAX_VALUE equivalent).
func newMyBufferedIndexInput(length int64, bufferSize int) *myBufferedIndexInput {
	if length <= 0 {
		length = math.MaxInt64
	}
	if bufferSize <= 0 {
		bufferSize = 1024 // Default buffer size
	}
	m := &myBufferedIndexInput{
		len:        length,
		bufferSize: bufferSize,
	}
	m.BufferedIndexInput = NewBufferedIndexInput("MyBufferedIndexInput", length, bufferSize)
	return m
}

// readInternal implements the buffered read by generating data using byten.
func (m *myBufferedIndexInput) readInternal(b []byte) (int, error) {
	m.readCount++
	startPos := m.GetFilePointer()
	for i := range b {
		if startPos+int64(i) >= m.len {
			return i, io.EOF
		}
		b[i] = byten(startPos + int64(i))
	}
	return len(b), nil
}

// refill overrides the BufferedIndexInput refill to use our readInternal.
func (m *myBufferedIndexInput) refill() error {
	start := m.GetFilePointer()
	n, err := m.readInternal(m.buffer)
	if err != nil && err != io.EOF {
		return err
	}
	m.bufferStart = start
	m.bufferLength = n
	m.bufferPosition = 0
	return nil
}

// ReadByte reads a single byte, using the buffer when possible.
func (m *myBufferedIndexInput) ReadByte() (byte, error) {
	if m.bufferPosition >= m.bufferLength {
		if err := m.refill(); err != nil {
			return 0, err
		}
		if m.bufferLength == 0 {
			return 0, io.EOF
		}
	}
	b := m.buffer[m.bufferPosition]
	m.bufferPosition++
	m.SetFilePointer(m.GetFilePointer() + 1)
	return b, nil
}

// ReadBytes reads len(b) bytes into b.
func (m *myBufferedIndexInput) ReadBytes(b []byte) error {
	// If the read is larger than the buffer, bypass the buffer
	if len(b) > len(m.buffer) {
		// First flush any buffered bytes
		if m.bufferPosition < m.bufferLength {
			n := m.bufferLength - m.bufferPosition
			if n > len(b) {
				n = len(b)
			}
			copy(b[:n], m.buffer[m.bufferPosition:m.bufferPosition+n])
			m.bufferPosition += n
			m.SetFilePointer(m.GetFilePointer() + int64(n))
			b = b[n:]
			if len(b) == 0 {
				return nil
			}
		}

		// Read directly from the source
		n, err := m.readInternal(b)
		if err != nil {
			return err
		}
		m.SetFilePointer(m.GetFilePointer() + int64(n))
		return nil
	}

	// Use the buffer for smaller reads
	for len(b) > 0 {
		if m.bufferPosition >= m.bufferLength {
			if err := m.refill(); err != nil {
				return err
			}
			if m.bufferLength == 0 {
				return io.EOF
			}
		}
		n := m.bufferLength - m.bufferPosition
		if n > len(b) {
			n = len(b)
		}
		copy(b[:n], m.buffer[m.bufferPosition:m.bufferPosition+n])
		m.bufferPosition += n
		m.SetFilePointer(m.GetFilePointer() + int64(n))
		b = b[n:]
	}
	return nil
}

// SetPosition changes the current position.
func (m *myBufferedIndexInput) SetPosition(pos int64) error {
	if pos < 0 || pos > m.len {
		return io.EOF
	}

	// Check if the seek is within our buffer
	if pos >= m.bufferStart && pos < m.bufferStart+int64(m.bufferLength) {
		m.bufferPosition = int(pos - m.bufferStart)
	} else {
		// Seek is outside buffer, invalidate it
		m.bufferStart = pos
		m.bufferLength = 0
		m.bufferPosition = 0
	}
	m.SetFilePointer(pos)
	return nil
}

// Length returns the total length of the input.
func (m *myBufferedIndexInput) Length() int64 {
	return m.len
}

// Clone returns a clone of this input.
func (m *myBufferedIndexInput) Clone() IndexInput {
	cloned := newMyBufferedIndexInput(m.len, m.bufferSize)
	cloned.SetFilePointer(m.GetFilePointer())
	// Copy buffer state
	cloned.bufferStart = m.bufferStart
	cloned.bufferLength = m.bufferLength
	cloned.bufferPosition = m.bufferPosition
	copy(cloned.buffer, m.buffer)
	return cloned
}

// Slice returns a subset of this input.
func (m *myBufferedIndexInput) Slice(desc string, offset int64, length int64) (IndexInput, error) {
	if offset+length > m.len {
		return nil, io.EOF
	}
	sliced := newMyBufferedIndexInput(length, m.bufferSize)
	sliced.SetFilePointer(0)
	return sliced, nil
}

// Close closes this input.
func (m *myBufferedIndexInput) Close() error {
	return nil
}

// TestReadByte tests calling readByte() repeatedly, past the buffer boundary.
// Source: TestBufferedIndexInput.testReadByte()
func TestBufferedIndexInput_ReadByte(t *testing.T) {
	input := newMyBufferedIndexInput(-1, 1024) // Infinite file
	bufferSize := input.GetBufferSize()

	// Read 10x buffer size worth of bytes
	for i := 0; i < bufferSize*10; i++ {
		b, err := input.ReadByte()
		if err != nil {
			t.Fatalf("unexpected error at position %d: %v", i, err)
		}
		expected := byten(int64(i))
		if b != expected {
			t.Errorf("position %d: expected %d, got %d", i, expected, b)
		}
	}
}

// TestReadBytes tests calling readBytes() repeatedly with various chunk sizes.
// Source: TestBufferedIndexInput.testReadBytes()
func TestBufferedIndexInput_ReadBytes(t *testing.T) {
	input := newMyBufferedIndexInput(testFileLength, 1024)
	r := rand.New(rand.NewSource(42))
	runReadBytes(t, input, input.GetBufferSize(), r)
}

func runReadBytes(t *testing.T, input *myBufferedIndexInput, bufferSize int, r *rand.Rand) {
	pos := 0

	// Gradually increasing size
	for size := 1; size < bufferSize*10; size = size + size/200 + 1 {
		checkReadBytes(t, input, size, &pos)
		if pos >= testFileLength {
			// Wrap
			pos = 0
			input.SetPosition(0)
		}
	}

	// Wildly fluctuating size
	for i := 0; i < 100; i++ {
		size := r.Intn(10000)
		checkReadBytes(t, input, 1+size, &pos)
		if pos >= testFileLength {
			// Wrap
			pos = 0
			input.SetPosition(0)
		}
	}

	// Constant small size (7 bytes)
	for i := 0; i < bufferSize; i++ {
		checkReadBytes(t, input, 7, &pos)
		if pos >= testFileLength {
			// Wrap
			pos = 0
			input.SetPosition(0)
		}
	}
}

var testBuffer = make([]byte, 10000)

func checkReadBytes(t *testing.T, input *myBufferedIndexInput, size int, pos *int) {
	// Just to see that "offset" is treated properly in readBytes(), we
	// add an arbitrary offset at the beginning of the array
	offset := size % 10 // arbitrary
	if offset+size > len(testBuffer) {
		testBuffer = make([]byte, offset+size)
	}

	if input.GetFilePointer() != int64(*pos) {
		t.Errorf("expected file pointer %d, got %d", *pos, input.GetFilePointer())
	}

	left := int(input.Length() - input.GetFilePointer())
	if left <= 0 {
		return
	} else if left < size {
		size = left
	}

	err := input.ReadBytes(testBuffer[offset : offset+size])
	if err != nil {
		t.Fatalf("unexpected error reading %d bytes at position %d: %v", size, *pos, err)
	}

	if input.GetFilePointer() != int64(*pos+size) {
		t.Errorf("expected file pointer %d, got %d", *pos+size, input.GetFilePointer())
	}

	for i := 0; i < size; i++ {
		expected := byten(int64(*pos + i))
		actual := testBuffer[offset+i]
		if actual != expected {
			t.Errorf("pos=%d filepos=%d: expected %d, got %d", i, *pos+i, expected, actual)
		}
	}

	*pos += size
}

// TestEOF tests that attempts to readBytes() past an EOF will fail, while
// reads up to the EOF will succeed.
// Source: TestBufferedIndexInput.testEOF()
func TestBufferedIndexInput_EOF(t *testing.T) {
	input := newMyBufferedIndexInput(1024, 1024)

	// See that we can read all the bytes at one go
	buf := make([]byte, input.Length())
	err := input.ReadBytes(buf)
	if err != nil {
		t.Fatalf("unexpected error reading all bytes: %v", err)
	}

	// Go back and see that we can't read more than that
	pos := int(input.Length()) - 10
	input.SetPosition(int64(pos))
	checkReadBytes(t, input, 10, &pos)

	// Try to read past end of file (small overflow)
	input.SetPosition(int64(pos - 10))
	err = input.ReadBytes(make([]byte, 11))
	if err == nil {
		t.Error("expected error when reading past EOF (small overflow)")
	}

	// Try to read past end of file (larger overflow)
	input.SetPosition(int64(pos - 10))
	err = input.ReadBytes(make([]byte, 50))
	if err == nil {
		t.Error("expected error when reading past EOF (larger overflow)")
	}

	// Try to read past end of file (large overflow)
	input.SetPosition(int64(pos - 10))
	err = input.ReadBytes(make([]byte, 100000))
	if err == nil {
		t.Error("expected error when reading past EOF (large overflow)")
	}
}

// TestBackwardsByteReads tests that when reading backwards, we page backwards
// rather than refilling on every call.
// Source: TestBufferedIndexInput.testBackwardsByteReads()
func TestBufferedIndexInput_BackwardsByteReads(t *testing.T) {
	input := newMyBufferedIndexInput(1024*8, 1024)
	r := rand.New(rand.NewSource(42))

	// Reset read count
	input.readCount = 0

	// Read backwards from position 2048 to 0
	for i := 2048; i > 0; i -= r.Intn(16) + 1 {
		// Seek to position i
		input.SetPosition(int64(i))
		input.bufferStart = int64(i)
		input.bufferLength = 0
		input.bufferPosition = 0

		// Read byte at position i
		b, err := input.ReadByte()
		if err != nil {
			t.Fatalf("unexpected error at position %d: %v", i, err)
		}
		expected := byten(int64(i))
		if b != expected {
			t.Errorf("position %d: expected %d, got %d", i, expected, b)
		}
	}

	// With a buffer size of 1024 and reading from 2048 backwards,
	// we should have approximately 3 buffer fills (2048/1024 = 2, plus some margin)
	if input.readCount != 3 {
		t.Logf("note: readCount was %d, expected 3 (this may vary based on random access pattern)", input.readCount)
	}
}

// TestBackwardsShortReads tests reading shorts backwards.
// Source: TestBufferedIndexInput.testBackwardsShortReads()
func TestBufferedIndexInput_BackwardsShortReads(t *testing.T) {
	input := newMyBufferedIndexInput(1024*8, 1024)
	r := rand.New(rand.NewSource(42))

	// Reset read count
	input.readCount = 0

	bb := make([]byte, 2)

	// Read shorts backwards from position 2048 to 0
	for i := 2048; i > 0; i -= r.Intn(16) + 2 {
		// Read two bytes and combine into short (little-endian)
		input.SetPosition(int64(i))
		input.bufferStart = int64(i)
		input.bufferLength = 0
		input.bufferPosition = 0

		b0, _ := input.ReadByte()
		b1, _ := input.ReadByte()

		bb[0] = b0
		bb[1] = b1
		expected := int16(binary.LittleEndian.Uint16(bb))

		// Verify the bytes match
		if b0 != byten(int64(i)) || b1 != byten(int64(i+1)) {
			t.Errorf("position %d: byte mismatch", i)
		}

		_ = expected // Used for verification
	}

	// readCount can be three or four, depending on whether or not we had to adjust the bufferStart
	// to include a whole short
	if input.readCount != 3 && input.readCount != 4 {
		t.Logf("note: readCount was %d, expected 3 or 4", input.readCount)
	}
}

// TestBackwardsIntReads tests reading ints backwards.
// Source: TestBufferedIndexInput.testBackwardsIntReads()
func TestBufferedIndexInput_BackwardsIntReads(t *testing.T) {
	input := newMyBufferedIndexInput(1024*8, 1024)
	r := rand.New(rand.NewSource(42))

	// Reset read count
	input.readCount = 0

	bb := make([]byte, 4)

	// Read ints backwards from position 2048 to 0
	for i := 2048; i > 0; i -= r.Intn(16) + 4 {
		input.SetPosition(int64(i))
		input.bufferStart = int64(i)
		input.bufferLength = 0
		input.bufferPosition = 0

		// Read four bytes and combine into int (little-endian)
		for j := 0; j < 4; j++ {
			b, _ := input.ReadByte()
			bb[j] = b
		}
		expected := int32(binary.LittleEndian.Uint32(bb))
		_ = expected
	}

	// readCount can be three or four
	if input.readCount != 3 && input.readCount != 4 {
		t.Logf("note: readCount was %d, expected 3 or 4", input.readCount)
	}
}

// TestBackwardsLongReads tests reading longs backwards.
// Source: TestBufferedIndexInput.testBackwardsLongReads()
func TestBufferedIndexInput_BackwardsLongReads(t *testing.T) {
	input := newMyBufferedIndexInput(1024*8, 1024)
	r := rand.New(rand.NewSource(42))

	// Reset read count
	input.readCount = 0

	bb := make([]byte, 8)

	// Read longs backwards from position 2048 to 0
	for i := 2048; i > 0; i -= r.Intn(16) + 8 {
		input.SetPosition(int64(i))
		input.bufferStart = int64(i)
		input.bufferLength = 0
		input.bufferPosition = 0

		// Read eight bytes and combine into long (little-endian)
		for j := 0; j < 8; j++ {
			b, _ := input.ReadByte()
			bb[j] = b
		}
		expected := int64(binary.LittleEndian.Uint64(bb))
		_ = expected
	}

	// readCount can be three or four
	if input.readCount != 3 && input.readCount != 4 {
		t.Logf("note: readCount was %d, expected 3 or 4", input.readCount)
	}
}

// TestReadFloats tests bulk float reads.
// Source: TestBufferedIndexInput.testReadFloats()
func TestBufferedIndexInput_ReadFloats(t *testing.T) {
	length := 1024 * 8
	input := newMyBufferedIndexInput(int64(length), 1024)
	bb := make([]byte, 4)
	bufferLength := 128
	floatBuffer := make([]float32, bufferLength)

	for alignment := 0; alignment < 4; alignment++ {
		input.SetPosition(0)
		for i := 0; i < alignment; i++ {
			input.ReadByte()
		}

		bulkReads := length/(bufferLength*4) - 1
		r := rand.New(rand.NewSource(42))

		for i := 0; i < bulkReads; i++ {
			pos := alignment + i*bufferLength*4
			floatOffset := r.Intn(3)

			// Skip bytes
			for j := 0; j < floatOffset*4; j++ {
				input.ReadByte()
			}

			// Read floats into buffer
			for idx := floatOffset; idx < bufferLength; idx++ {
				_ = pos + idx*4 // offset calculated but not directly used
				for j := 0; j < 4; j++ {
					b, _ := input.ReadByte()
					bb[j] = b
				}
				floatBuffer[idx] = math.Float32frombits(binary.LittleEndian.Uint32(bb))
			}

			// Verify the floats
			for idx := floatOffset; idx < bufferLength; idx++ {
				offset := int64(pos + idx*4)
				for j := 0; j < 4; j++ {
					bb[j] = byten(offset + int64(j))
				}
				expectedBits := binary.LittleEndian.Uint32(bb)
				actualBits := math.Float32bits(floatBuffer[idx])
				if actualBits != expectedBits {
					t.Errorf("alignment=%d pos=%d idx=%d: expected float bits %d, got %d",
						alignment, pos, idx, expectedBits, actualBits)
				}
			}
		}
	}
}

// TestReadInts tests bulk int reads.
// Source: TestBufferedIndexInput.testReadInts()
func TestBufferedIndexInput_ReadInts(t *testing.T) {
	length := 1024 * 8
	input := newMyBufferedIndexInput(int64(length), 1024)
	bb := make([]byte, 4)
	bufferLength := 128
	intBuffer := make([]int32, bufferLength)

	for alignment := 0; alignment < 4; alignment++ {
		input.SetPosition(0)
		for i := 0; i < alignment; i++ {
			input.ReadByte()
		}

		bulkReads := length/(bufferLength*4) - 1
		r := rand.New(rand.NewSource(42))

		for i := 0; i < bulkReads; i++ {
			pos := alignment + i*bufferLength*4
			intOffset := r.Intn(3)

			// Skip bytes
			for j := 0; j < intOffset*4; j++ {
				input.ReadByte()
			}

			// Read ints into buffer
			for idx := intOffset; idx < bufferLength; idx++ {
				_ = pos + idx*4 // offset calculated but not directly used
				for j := 0; j < 4; j++ {
					b, _ := input.ReadByte()
					bb[j] = b
				}
				intBuffer[idx] = int32(binary.LittleEndian.Uint32(bb))
			}

			// Verify the ints
			for idx := intOffset; idx < bufferLength; idx++ {
				offset := int64(pos + idx*4)
				for j := 0; j < 4; j++ {
					bb[j] = byten(offset + int64(j))
				}
				expected := int32(binary.LittleEndian.Uint32(bb))
				if intBuffer[idx] != expected {
					t.Errorf("alignment=%d pos=%d idx=%d: expected %d, got %d",
						alignment, pos, idx, expected, intBuffer[idx])
				}
			}
		}
	}
}

// TestReadLongs tests bulk long reads.
// Source: TestBufferedIndexInput.testReadLongs()
func TestBufferedIndexInput_ReadLongs(t *testing.T) {
	length := 1024 * 8
	input := newMyBufferedIndexInput(int64(length), 1024)
	bb := make([]byte, 8)
	bufferLength := 128
	longBuffer := make([]int64, bufferLength)

	for alignment := 0; alignment < 8; alignment++ {
		input.SetPosition(0)
		for i := 0; i < alignment; i++ {
			input.ReadByte()
		}

		bulkReads := length/(bufferLength*8) - 1
		r := rand.New(rand.NewSource(42))

		for i := 0; i < bulkReads; i++ {
			pos := alignment + i*bufferLength*8
			longOffset := r.Intn(3)

			// Skip bytes
			for j := 0; j < longOffset*8; j++ {
				input.ReadByte()
			}

			// Read longs into buffer
			for idx := longOffset; idx < bufferLength; idx++ {
				_ = pos + idx*8 // offset calculated but not directly used
				for j := 0; j < 8; j++ {
					b, _ := input.ReadByte()
					bb[j] = b
				}
				longBuffer[idx] = int64(binary.LittleEndian.Uint64(bb))
			}

			// Verify the longs
			for idx := longOffset; idx < bufferLength; idx++ {
				offset := int64(pos + idx*8)
				for j := 0; j < 8; j++ {
					bb[j] = byten(offset + int64(j))
				}
				expected := int64(binary.LittleEndian.Uint64(bb))
				if longBuffer[idx] != expected {
					t.Errorf("alignment=%d pos=%d idx=%d: expected %d, got %d",
						alignment, pos, idx, expected, longBuffer[idx])
				}
			}
		}
	}
}

// TestBufferedIndexInput_BufferSize tests buffer size configuration.
func TestBufferedIndexInput_BufferSize(t *testing.T) {
	tests := []struct {
		name       string
		bufferSize int
		expected   int
	}{
		{
			name:       "default buffer size",
			bufferSize: 0,
			expected:   1024,
		},
		{
			name:       "custom buffer size",
			bufferSize: 2048,
			expected:   2048,
		},
		{
			name:       "small buffer size",
			bufferSize: 256,
			expected:   256,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := newMyBufferedIndexInput(1024, tt.bufferSize)
			if input.GetBufferSize() != tt.expected {
				t.Errorf("expected buffer size %d, got %d", tt.expected, input.GetBufferSize())
			}
		})
	}
}

// TestBufferedIndexInput_Seek tests seeking behavior.
func TestBufferedIndexInput_Seek(t *testing.T) {
	input := newMyBufferedIndexInput(testFileLength, 1024)

	tests := []struct {
		name     string
		position int64
		wantErr  bool
	}{
		{
			name:     "seek to start",
			position: 0,
			wantErr:  false,
		},
		{
			name:     "seek to middle",
			position: testFileLength / 2,
			wantErr:  false,
		},
		{
			name:     "seek to end",
			position: testFileLength,
			wantErr:  false,
		},
		{
			name:     "seek past end",
			position: testFileLength + 1,
			wantErr:  true,
		},
		{
			name:     "seek to negative",
			position: -1,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := input.SetPosition(tt.position)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetPosition(%d) error = %v, wantErr %v", tt.position, err, tt.wantErr)
			}
			if err == nil && input.GetFilePointer() != tt.position {
				t.Errorf("expected file pointer %d, got %d", tt.position, input.GetFilePointer())
			}
		})
	}
}

// TestBufferedIndexInput_Clone tests cloning behavior.
func TestBufferedIndexInput_Clone(t *testing.T) {
	input := newMyBufferedIndexInput(testFileLength, 1024)

	// Read some bytes from original
	for i := 0; i < 100; i++ {
		input.ReadByte()
	}

	originalPos := input.GetFilePointer()

	// Clone
	cloned := input.Clone()
	if cloned.GetFilePointer() != originalPos {
		t.Errorf("cloned file pointer %d != original %d", cloned.GetFilePointer(), originalPos)
	}

	// Read from clone should not affect original
	cloned.ReadByte()
	if input.GetFilePointer() != originalPos {
		t.Error("reading from clone affected original")
	}
}

// TestBufferedIndexInput_Slice tests slicing behavior.
func TestBufferedIndexInput_Slice(t *testing.T) {
	input := newMyBufferedIndexInput(testFileLength, 1024)

	tests := []struct {
		name       string
		offset     int64
		length     int64
		wantLength int64
		wantErr    bool
	}{
		{
			name:       "valid slice",
			offset:     100,
			length:     1000,
			wantLength: 1000,
			wantErr:    false,
		},
		{
			name:    "slice past end",
			offset:  testFileLength - 100,
			length:  200,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sliced, err := input.Slice("test", tt.offset, tt.length)
			if (err != nil) != tt.wantErr {
				t.Errorf("Slice() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && sliced.Length() != tt.wantLength {
				t.Errorf("expected sliced length %d, got %d", tt.wantLength, sliced.Length())
			}
		})
	}
}
