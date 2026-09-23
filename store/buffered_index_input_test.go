// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

// Port of lucene/core/src/test/org/apache/lucene/store/TestBufferedIndexInput.java
// (Apache Lucene 10.5.0).

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// testFileLength mirrors TestBufferedIndexInput.TEST_FILE_LENGTH.
const testFileLength = 100 * 1024

// newBufferedIndexInputTestRandom mirrors LuceneTestCase.random(): a seeded
// generator whose seed is logged so that a failure can be replayed.
func newBufferedIndexInputTestRandom(t *testing.T) *rand.Rand {
	t.Helper()
	seed := uint64(time.Now().UnixNano())
	t.Logf("random seed: %d", seed)
	return rand.New(rand.NewPCG(seed, 0x9E3779B97F4A7C15))
}

// testReadByte: call readByte() repeatedly, past the buffer boundary, and see
// that it is working as expected.
func TestBufferedIndexInput_ReadByte(t *testing.T) {
	input, _ := newMyBufferedIndexInput(t)
	for i := 0; i < DefaultBufferSize*10; i++ {
		b, err := input.ReadByte()
		if err != nil {
			t.Fatalf("readByte at %d: %v", i, err)
		}
		if b != byten(int64(i)) {
			t.Fatalf("readByte at %d: got %d, want %d", i, b, byten(int64(i)))
		}
	}
}

// testReadBytes: call readBytes() repeatedly, with various chunk sizes (from 1
// byte to larger than the buffer size), and see that it returns the bytes we
// expect.
func TestBufferedIndexInput_ReadBytes(t *testing.T) {
	input, _ := newMyBufferedIndexInput(t)
	runReadBytes(t, input, DefaultBufferSize, newBufferedIndexInputTestRandom(t))
}

func runReadBytes(t *testing.T, input IndexInput, bufferSize int, r *rand.Rand) {
	t.Helper()
	pos := 0
	// gradually increasing size:
	for size := 1; size < bufferSize*10; size = size + size/200 + 1 {
		mustCheckReadBytes(t, input, size, pos)
		pos += size
		if pos >= testFileLength {
			// wrap
			pos = 0
			if err := input.SetPosition(0); err != nil {
				t.Fatalf("seek(0): %v", err)
			}
		}
	}
	// wildly fluctuating size:
	for i := int64(0); i < 100; i++ {
		size := r.IntN(10000)
		mustCheckReadBytes(t, input, 1+size, pos)
		pos += 1 + size
		if pos >= testFileLength {
			// wrap
			pos = 0
			if err := input.SetPosition(0); err != nil {
				t.Fatalf("seek(0): %v", err)
			}
		}
	}
	// constant small size (7 bytes):
	for i := 0; i < bufferSize; i++ {
		mustCheckReadBytes(t, input, 7, pos)
		pos += 7
		if pos >= testFileLength {
			// wrap
			pos = 0
			if err := input.SetPosition(0); err != nil {
				t.Fatalf("seek(0): %v", err)
			}
		}
	}
}

// checkReadBytesBuffer mirrors the test class field `buffer`.
var checkReadBytesBuffer = make([]byte, 10)

// checkReadBytes mirrors TestBufferedIndexInput.checkReadBytes. Assertion
// failures stop the test; the error of readBytes is returned so that callers
// can assert on the IOException the Java method propagates.
func checkReadBytes(t *testing.T, input IndexInput, size, pos int) error {
	t.Helper()
	// Just to see that "offset" is treated properly in readBytes(), we
	// add an arbitrary offset at the beginning of the array
	offset := size % 10 // arbitrary
	checkReadBytesBuffer = util.GrowByte(checkReadBytesBuffer, offset+size)
	if got := input.GetFilePointer(); got != int64(pos) {
		t.Fatalf("getFilePointer: got %d, want %d", got, pos)
	}
	left := int64(testFileLength) - input.GetFilePointer()
	if left <= 0 {
		return nil
	} else if left < int64(size) {
		size = int(left)
	}
	if err := input.ReadBytes(checkReadBytesBuffer, offset, size); err != nil {
		return err
	}
	if got := input.GetFilePointer(); got != int64(pos+size) {
		t.Fatalf("getFilePointer after read: got %d, want %d", got, pos+size)
	}
	for i := 0; i < size; i++ {
		if want, got := byten(int64(pos+i)), checkReadBytesBuffer[offset+i]; want != got {
			t.Fatalf("pos=%d filepos=%d: got %d, want %d", i, pos+i, got, want)
		}
	}
	return nil
}

func mustCheckReadBytes(t *testing.T, input IndexInput, size, pos int) {
	t.Helper()
	if err := checkReadBytes(t, input, size, pos); err != nil {
		t.Fatalf("readBytes(size=%d, pos=%d): %v", size, pos, err)
	}
}

// testEOF: attempts to readBytes() past an EOF will fail, while reads up to
// the EOF will succeed. The EOF is determined by the BufferedIndexInput's
// arbitrary length() value.
func TestBufferedIndexInput_EOF(t *testing.T) {
	input, _ := newMyBufferedIndexInputWithLength(t, 1024)
	// see that we can read all the bytes at one go:
	mustCheckReadBytes(t, input, int(input.Length()), 0)
	// go back and see that we can't read more than that, for small and
	// large overflows:
	pos := int(input.Length()) - 10
	mustSeek(t, input, int64(pos))
	mustCheckReadBytes(t, input, 10, pos)
	mustSeek(t, input, int64(pos))
	// block read past end of file
	if err := checkReadBytes(t, input, 11, pos); err == nil {
		t.Fatal("expected IOException reading 11 bytes past EOF")
	}

	mustSeek(t, input, int64(pos))

	// block read past end of file
	if err := checkReadBytes(t, input, 50, pos); err == nil {
		t.Fatal("expected IOException reading 50 bytes past EOF")
	}

	mustSeek(t, input, int64(pos))

	// block read past end of file
	if err := checkReadBytes(t, input, 100000, pos); err == nil {
		t.Fatal("expected IOException reading 100000 bytes past EOF")
	}
}

func mustSeek(t *testing.T, input IndexInput, pos int64) {
	t.Helper()
	if err := input.SetPosition(pos); err != nil {
		t.Fatalf("seek(%d): %v", pos, err)
	}
}

// testBackwardsByteReads: when reading backwards, we page backwards rather
// than refilling on every call.
func TestBufferedIndexInput_BackwardsByteReads(t *testing.T) {
	r := newBufferedIndexInputTestRandom(t)
	input, impl := newMyBufferedIndexInputWithLength(t, 1024*8)
	for i := 2048; i > 0; i -= r.IntN(16) {
		got, err := input.ReadByteAt(int64(i))
		if err != nil {
			t.Fatalf("readByte(%d): %v", i, err)
		}
		if want := byten(int64(i)); got != want {
			t.Fatalf("readByte(%d): got %d, want %d", i, got, want)
		}
	}
	if impl.readCount != 3 {
		t.Fatalf("readCount: got %d, want 3", impl.readCount)
	}
}

func TestBufferedIndexInput_BackwardsShortReads(t *testing.T) {
	r := newBufferedIndexInputTestRandom(t)
	input, impl := newMyBufferedIndexInputWithLength(t, 1024*8)
	bb := make([]byte, 2)
	for i := 2048; i > 0; i -= r.IntN(16) + 1 {
		bb[0] = byten(int64(i))
		bb[1] = byten(int64(i + 1))
		got, err := input.ReadShortAt(int64(i))
		if err != nil {
			t.Fatalf("readShort(%d): %v", i, err)
		}
		if want := int16(binary.LittleEndian.Uint16(bb)); got != want {
			t.Fatalf("readShort(%d): got %d, want %d", i, got, want)
		}
	}
	// readCount can be three or four, depending on whether or not we had to
	// adjust the bufferStart to include a whole short
	if impl.readCount != 4 && impl.readCount != 3 {
		t.Fatalf("Expected 4 or 3, got %d", impl.readCount)
	}
}

func TestBufferedIndexInput_BackwardsIntReads(t *testing.T) {
	r := newBufferedIndexInputTestRandom(t)
	input, impl := newMyBufferedIndexInputWithLength(t, 1024*8)
	bb := make([]byte, 4)
	for i := 2048; i > 0; i -= r.IntN(16) + 3 {
		for k := 0; k < 4; k++ {
			bb[k] = byten(int64(i + k))
		}
		got, err := input.ReadIntAt(int64(i))
		if err != nil {
			t.Fatalf("readInt(%d): %v", i, err)
		}
		if want := int32(binary.LittleEndian.Uint32(bb)); got != want {
			t.Fatalf("readInt(%d): got %d, want %d", i, got, want)
		}
	}
	// readCount can be three or four, depending on whether or not we had to
	// adjust the bufferStart to include a whole int
	if impl.readCount != 4 && impl.readCount != 3 {
		t.Fatalf("Expected 4 or 3, got %d", impl.readCount)
	}
}

func TestBufferedIndexInput_BackwardsLongReads(t *testing.T) {
	r := newBufferedIndexInputTestRandom(t)
	input, impl := newMyBufferedIndexInputWithLength(t, 1024*8)
	bb := make([]byte, 8)
	for i := 2048; i > 0; i -= r.IntN(16) + 7 {
		for k := 0; k < 8; k++ {
			bb[k] = byten(int64(i + k))
		}
		got, err := input.ReadLongAt(int64(i))
		if err != nil {
			t.Fatalf("readLong(%d): %v", i, err)
		}
		if want := int64(binary.LittleEndian.Uint64(bb)); got != want {
			t.Fatalf("readLong(%d): got %d, want %d", i, got, want)
		}
	}
	// readCount can be three or four, depending on whether or not we had to
	// adjust the bufferStart to include a whole long
	if impl.readCount != 4 && impl.readCount != 3 {
		t.Fatalf("Expected 4 or 3, got %d", impl.readCount)
	}
}

func TestBufferedIndexInput_ReadFloats(t *testing.T) {
	r := newBufferedIndexInputTestRandom(t)
	const length = 1024 * 8
	input, _ := newMyBufferedIndexInputWithLength(t, length)
	bb := make([]byte, 4)
	const bufferLength = 128
	floatBuffer := make([]float32, bufferLength)

	for alignment := 0; alignment < 4; alignment++ {
		mustSeek(t, input, 0)
		for i := 0; i < alignment; i++ {
			if _, err := input.ReadByte(); err != nil {
				t.Fatalf("readByte: %v", err)
			}
		}
		bulkReads := length/(bufferLength*4) - 1
		for i := 0; i < bulkReads; i++ {
			pos := alignment + i*bufferLength*4
			floatOffset := r.IntN(3)
			if err := input.SkipBytes(int64(floatOffset * 4)); err != nil {
				t.Fatalf("skipBytes: %v", err)
			}
			if err := input.ReadFloats(floatBuffer, floatOffset, bufferLength-floatOffset); err != nil {
				t.Fatalf("readFloats: %v", err)
			}
			for idx := floatOffset; idx < bufferLength; idx++ {
				offset := pos + idx*4
				for k := 0; k < 4; k++ {
					bb[k] = byten(int64(offset + k))
				}
				want := binary.LittleEndian.Uint32(bb)
				if got := math.Float32bits(floatBuffer[idx]); got != want {
					t.Fatalf("alignment=%d idx=%d: got bits %#x, want %#x", alignment, idx, got, want)
				}
			}
		}
	}
}

func TestBufferedIndexInput_ReadInts(t *testing.T) {
	r := newBufferedIndexInputTestRandom(t)
	const length = 1024 * 8
	input, _ := newMyBufferedIndexInputWithLength(t, length)
	bb := make([]byte, 4)
	const bufferLength = 128
	intBuffer := make([]int32, bufferLength)

	for alignment := 0; alignment < 4; alignment++ {
		mustSeek(t, input, 0)
		for i := 0; i < alignment; i++ {
			if _, err := input.ReadByte(); err != nil {
				t.Fatalf("readByte: %v", err)
			}
		}
		bulkReads := length/(bufferLength*4) - 1
		for i := 0; i < bulkReads; i++ {
			pos := alignment + i*bufferLength*4
			intOffset := r.IntN(3)
			if err := input.SkipBytes(int64(intOffset * 4)); err != nil {
				t.Fatalf("skipBytes: %v", err)
			}
			if err := input.ReadInts(intBuffer, intOffset, bufferLength-intOffset); err != nil {
				t.Fatalf("readInts: %v", err)
			}
			for idx := intOffset; idx < bufferLength; idx++ {
				offset := pos + idx*4
				for k := 0; k < 4; k++ {
					bb[k] = byten(int64(offset + k))
				}
				if want := int32(binary.LittleEndian.Uint32(bb)); intBuffer[idx] != want {
					t.Fatalf("alignment=%d idx=%d: got %d, want %d", alignment, idx, intBuffer[idx], want)
				}
			}
		}
	}
}

func TestBufferedIndexInput_ReadLongs(t *testing.T) {
	r := newBufferedIndexInputTestRandom(t)
	const length = 1024 * 8
	input, _ := newMyBufferedIndexInputWithLength(t, length)
	bb := make([]byte, 8)
	const bufferLength = 128
	longBuffer := make([]int64, bufferLength)

	for alignment := 0; alignment < 8; alignment++ {
		mustSeek(t, input, 0)
		for i := 0; i < alignment; i++ {
			if _, err := input.ReadByte(); err != nil {
				t.Fatalf("readByte: %v", err)
			}
		}
		bulkReads := length/(bufferLength*8) - 1
		for i := 0; i < bulkReads; i++ {
			pos := alignment + i*bufferLength*8
			longOffset := r.IntN(3)
			if err := input.SkipBytes(int64(longOffset * 8)); err != nil {
				t.Fatalf("skipBytes: %v", err)
			}
			if err := input.ReadLongs(longBuffer, longOffset, bufferLength-longOffset); err != nil {
				t.Fatalf("readLongs: %v", err)
			}
			for idx := longOffset; idx < bufferLength; idx++ {
				offset := pos + idx*8
				for k := 0; k < 8; k++ {
					bb[k] = byten(int64(offset + k))
				}
				if want := int64(binary.LittleEndian.Uint64(bb)); longBuffer[idx] != want {
					t.Fatalf("alignment=%d idx=%d: got %d, want %d", alignment, idx, longBuffer[idx], want)
				}
			}
		}
	}
}

// byten emulates a file - byten(n) returns the n'th byte in that file.
// MyBufferedIndexInput reads this "file".
func byten(n int64) byte {
	return byte(n * n % 256)
}

// myBufferedIndexInput is the port of the test class MyBufferedIndexInput.
// Gocene renders the abstract BufferedIndexInput as a concrete type that
// delegates readInternal/seekInternal to a bufferedInternal implementation;
// this type is that implementation. Under that contract readInternal receives
// the file position explicitly, so the reader starts at that position.
type myBufferedIndexInput struct {
	pos       int64
	len       int64
	readCount int64
}

// newMyBufferedIndexInputWithLength mirrors MyBufferedIndexInput(long len).
func newMyBufferedIndexInputWithLength(t *testing.T, length int64) (*BufferedIndexInput, *myBufferedIndexInput) {
	t.Helper()
	impl := &myBufferedIndexInput{len: length, pos: 0}
	in, err := NewBufferedIndexInput(impl, DefaultBufferSize)
	if err != nil {
		t.Fatalf("NewBufferedIndexInput: %v", err)
	}
	return in, impl
}

// newMyBufferedIndexInput mirrors MyBufferedIndexInput(): an infinite file.
func newMyBufferedIndexInput(t *testing.T) (*BufferedIndexInput, *myBufferedIndexInput) {
	t.Helper()
	return newMyBufferedIndexInputWithLength(t, math.MaxInt64)
}

func (m *myBufferedIndexInput) ReadInternal(b []byte, offset int64) (int, error) {
	m.readCount++
	m.pos = offset
	for i := range b {
		b[i] = byten(m.pos)
		m.pos++
	}
	return len(b), nil
}

func (m *myBufferedIndexInput) SeekInternal(pos int64) error {
	m.pos = pos
	return nil
}

func (m *myBufferedIndexInput) Close() error { return nil }

func (m *myBufferedIndexInput) Length() int64 { return m.len }

func (m *myBufferedIndexInput) Description() string {
	return fmt.Sprintf("MyBufferedIndexInput(len=%d)", m.len)
}

func (m *myBufferedIndexInput) Clone() bufferedInternal {
	c := *m
	return &c
}
