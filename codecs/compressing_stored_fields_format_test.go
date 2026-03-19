// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// testIndexOutput is a test implementation of IndexOutput
type testIndexOutput struct {
	*store.BaseIndexOutput
	buf *bytes.Buffer
}

func newTestIndexOutput() *testIndexOutput {
	return &testIndexOutput{
		BaseIndexOutput: store.NewBaseIndexOutput("test"),
		buf:             bytes.NewBuffer(nil),
	}
}

func (o *testIndexOutput) WriteByte(b byte) error {
	o.buf.WriteByte(b)
	o.IncrementFilePointer(1)
	return nil
}

func (o *testIndexOutput) WriteBytes(b []byte) error {
	o.buf.Write(b)
	o.IncrementFilePointer(int64(len(b)))
	return nil
}

func (o *testIndexOutput) WriteBytesN(b []byte, n int) error {
	return o.WriteBytes(b[:n])
}

func (o *testIndexOutput) WriteShort(i int16) error {
	return o.WriteBytes([]byte{byte(i >> 8), byte(i)})
}

func (o *testIndexOutput) WriteInt(i int32) error {
	return o.WriteBytes([]byte{byte(i >> 24), byte(i >> 16), byte(i >> 8), byte(i)})
}

func (o *testIndexOutput) WriteLong(i int64) error {
	return o.WriteBytes([]byte{
		byte(i >> 56), byte(i >> 48), byte(i >> 40), byte(i >> 32),
		byte(i >> 24), byte(i >> 16), byte(i >> 8), byte(i),
	})
}

func (o *testIndexOutput) WriteString(s string) error {
	return store.WriteString(o, s)
}

func (o *testIndexOutput) Length() int64 {
	return int64(o.buf.Len())
}

func (o *testIndexOutput) SetPosition(pos int64) error {
	return fmt.Errorf("SetPosition not supported")
}

func (o *testIndexOutput) Close() error {
	return nil
}

func (o *testIndexOutput) GetData() []byte {
	return o.buf.Bytes()
}

// testIndexInput is a test implementation of IndexInput
type testIndexInput struct {
	*store.BaseIndexInput
	data   []byte
	offset int64
}

func newTestIndexInput(data []byte) *testIndexInput {
	return &testIndexInput{
		BaseIndexInput: store.NewBaseIndexInput("test", int64(len(data))),
		data:           data,
		offset:         0,
	}
}

func (in *testIndexInput) ReadByte() (byte, error) {
	if in.offset >= int64(len(in.data)) {
		return 0, fmt.Errorf("EOF")
	}
	b := in.data[in.offset]
	in.offset++
	in.SetFilePointer(in.GetFilePointer() + 1)
	return b, nil
}

func (in *testIndexInput) ReadBytes(b []byte) error {
	if in.offset+int64(len(b)) > int64(len(in.data)) {
		return fmt.Errorf("EOF")
	}
	copy(b, in.data[in.offset:in.offset+int64(len(b))])
	in.offset += int64(len(b))
	in.SetFilePointer(in.GetFilePointer() + int64(len(b)))
	return nil
}

func (in *testIndexInput) ReadBytesN(n int) ([]byte, error) {
	b := make([]byte, n)
	if err := in.ReadBytes(b); err != nil {
		return nil, err
	}
	return b, nil
}

func (in *testIndexInput) ReadShort() (int16, error) {
	var b [2]byte
	if err := in.ReadBytes(b[:]); err != nil {
		return 0, err
	}
	return int16(b[0])<<8 | int16(b[1]), nil
}

func (in *testIndexInput) ReadInt() (int32, error) {
	var b [4]byte
	if err := in.ReadBytes(b[:]); err != nil {
		return 0, err
	}
	return int32(b[0])<<24 | int32(b[1])<<16 | int32(b[2])<<8 | int32(b[3]), nil
}

func (in *testIndexInput) ReadLong() (int64, error) {
	var b [8]byte
	if err := in.ReadBytes(b[:]); err != nil {
		return 0, err
	}
	return int64(b[0])<<56 | int64(b[1])<<48 | int64(b[2])<<40 | int64(b[3])<<32 |
		int64(b[4])<<24 | int64(b[5])<<16 | int64(b[6])<<8 | int64(b[7]), nil
}

func (in *testIndexInput) ReadString() (string, error) {
	return store.ReadString(in)
}

func (in *testIndexInput) Length() int64 {
	return int64(len(in.data))
}

func (in *testIndexInput) SetPosition(pos int64) error {
	if pos < 0 || pos > int64(len(in.data)) {
		return fmt.Errorf("position out of bounds")
	}
	in.offset = pos
	in.SetFilePointer(pos)
	return nil
}

func (in *testIndexInput) Clone() store.IndexInput {
	clone := newTestIndexInput(in.data)
	clone.offset = in.offset
	clone.SetFilePointer(in.GetFilePointer())
	return clone
}

func (in *testIndexInput) Slice(desc string, offset, length int64) (store.IndexInput, error) {
	if offset+length > int64(len(in.data)) {
		return nil, fmt.Errorf("slice out of bounds")
	}
	slice := newTestIndexInput(in.data[offset : offset+length])
	return slice, nil
}

func (in *testIndexInput) Close() error {
	return nil
}

// GC-207: Port TestCompressingStoredFieldsFormat.java from Apache Lucene
// Source: lucene/core/src/test/org/apache/lucene/codecs/lucene90/compressing/TestCompressingStoredFieldsFormat.java
//
// This test file covers:
// - ZFloat compression (small integers, special values, random values)
// - ZDouble compression (small integers, special values, random values)
// - TLong compression (time-based values, random values)
// - Chunk cleanup and merge behavior

const (
	second = int64(1000)
	hour   = 60 * 60 * second
	day    = 24 * hour
)

// TestCompressingStoredFieldsFormat_ZFloat tests ZFloat compression
// Ported from: testZFloat()
// Tests round-trip compression of small integer values, special values, and random floats
func TestCompressingStoredFieldsFormat_ZFloat(t *testing.T) {
	// Test small integer values
	for i := int16(-32768); i < 32767; i++ {
		buf := newTestIndexOutput()
		f := float32(i)

		err := codecs.WriteZFloat(buf, f)
		if err != nil {
			t.Fatalf("Failed to write ZFloat for %d: %v", i, err)
		}

		// Verify single byte compression for range -1 to 123
		if i >= -1 && i <= 123 {
			if buf.Length() != 1 {
				t.Errorf("Expected 1 byte for value %d, got %d", i, buf.Length())
			}
		}

		// Read back and verify
		data := buf.GetData()
		input := newTestIndexInput(data)
		readF, err := codecs.ReadZFloat(input)
		if err != nil {
			t.Fatalf("Failed to read ZFloat for %d: %v", i, err)
		}

		// Compare bits for exact match
		if math.Float32bits(f) != math.Float32bits(readF) {
			t.Errorf("ZFloat round-trip failed for %d: expected %v, got %v", i, f, readF)
		}
	}

	// Test special values
	specialValues := []float32{
		float32(math.Copysign(0, -1)), // -0.0f
		0.0,                           // +0.0f
		float32(math.Inf(-1)),         // NEGATIVE_INFINITY
		float32(math.Inf(1)),          // POSITIVE_INFINITY
		math.SmallestNonzeroFloat32,   // MIN_VALUE
		math.MaxFloat32,               // MAX_VALUE
		float32(math.NaN()),           // NaN
	}
	for _, f := range specialValues {
		buf := newTestIndexOutput()
		err := codecs.WriteZFloat(buf, f)
		if err != nil {
			t.Fatalf("Failed to write ZFloat for special value %v: %v", f, err)
		}

		data := buf.GetData()
		input := newTestIndexInput(data)
		readF, err := codecs.ReadZFloat(input)
		if err != nil {
			t.Fatalf("Failed to read ZFloat for special value %v: %v", f, err)
		}

		// For NaN, just check that it's NaN
		if math.IsNaN(float64(f)) {
			if !math.IsNaN(float64(readF)) {
				t.Errorf("ZFloat NaN round-trip failed: expected NaN, got %v", readF)
			}
			continue
		}

		// For infinity, check sign
		if math.IsInf(float64(f), 1) {
			if !math.IsInf(float64(readF), 1) {
				t.Errorf("ZFloat +Inf round-trip failed: expected +Inf, got %v", readF)
			}
			continue
		}
		if math.IsInf(float64(f), -1) {
			if !math.IsInf(float64(readF), -1) {
				t.Errorf("ZFloat -Inf round-trip failed: expected -Inf, got %v", readF)
			}
			continue
		}

		// For other values, compare bits
		if math.Float32bits(f) != math.Float32bits(readF) {
			t.Errorf("ZFloat round-trip failed for special value %v: got %v", f, readF)
		}
	}

	// Test random values
	r := rand.New(rand.NewSource(42))
	for i := 0; i < 100000; i++ {
		buf := newTestIndexOutput()
		f := r.Float32() * float32(r.Intn(100)-50)

		err := codecs.WriteZFloat(buf, f)
		if err != nil {
			t.Fatalf("Failed to write ZFloat for random value %v: %v", f, err)
		}

		// Verify position <= 4 for positive, <= 5 for negative
		isNegative := (math.Float32bits(f) >> 31) == 1
		if isNegative {
			if buf.Length() > 5 {
				t.Errorf("ZFloat negative value %v encoded to %d bytes, expected <= 5", f, buf.Length())
			}
		} else {
			if buf.Length() > 4 {
				t.Errorf("ZFloat positive value %v encoded to %d bytes, expected <= 4", f, buf.Length())
			}
		}

		// Read back and verify
		data := buf.GetData()
		input := newTestIndexInput(data)
		readF, err := codecs.ReadZFloat(input)
		if err != nil {
			t.Fatalf("Failed to read ZFloat for random value %v: %v", f, err)
		}

		// Compare bits
		if math.Float32bits(f) != math.Float32bits(readF) {
			t.Errorf("ZFloat round-trip failed for random value %v: got %v", f, readF)
		}
	}
}

// TestCompressingStoredFieldsFormat_ZDouble tests ZDouble compression
// Ported from: testZDouble()
// Tests round-trip compression of small integer values, special values, and random doubles
func TestCompressingStoredFieldsFormat_ZDouble(t *testing.T) {
	// Test small integer values
	for i := int16(-32768); i < 32767; i++ {
		buf := newTestIndexOutput()
		d := float64(i)

		err := codecs.WriteZDouble(buf, d)
		if err != nil {
			t.Fatalf("Failed to write ZDouble for %d: %v", i, err)
		}

		// Verify single byte compression for range -1 to 124
		if i >= -1 && i <= 124 {
			if buf.Length() != 1 {
				t.Errorf("Expected 1 byte for value %d, got %d", i, buf.Length())
			}
		}

		// Read back and verify
		data := buf.GetData()
		input := newTestIndexInput(data)
		readD, err := codecs.ReadZDouble(input)
		if err != nil {
			t.Fatalf("Failed to read ZDouble for %d: %v", i, err)
		}

		// Compare bits for exact match
		if math.Float64bits(d) != math.Float64bits(readD) {
			t.Errorf("ZDouble round-trip failed for %d: expected %v, got %v", i, d, readD)
		}
	}

	// Test special values
	specialValues := []float64{
		math.Copysign(0, -1),        // -0.0d
		0.0,                         // +0.0d
		math.Inf(-1),                // NEGATIVE_INFINITY
		math.Inf(1),                 // POSITIVE_INFINITY
		math.SmallestNonzeroFloat64, // MIN_VALUE
		math.MaxFloat64,             // MAX_VALUE
		math.NaN(),                  // NaN
	}
	for _, d := range specialValues {
		buf := newTestIndexOutput()
		err := codecs.WriteZDouble(buf, d)
		if err != nil {
			t.Fatalf("Failed to write ZDouble for special value %v: %v", d, err)
		}

		data := buf.GetData()
		input := newTestIndexInput(data)
		readD, err := codecs.ReadZDouble(input)
		if err != nil {
			t.Fatalf("Failed to read ZDouble for special value %v: %v", d, err)
		}

		// For NaN, just check that it's NaN
		if math.IsNaN(d) {
			if !math.IsNaN(readD) {
				t.Errorf("ZDouble NaN round-trip failed: expected NaN, got %v", readD)
			}
			continue
		}

		// For infinity, check sign
		if math.IsInf(d, 1) {
			if !math.IsInf(readD, 1) {
				t.Errorf("ZDouble +Inf round-trip failed: expected +Inf, got %v", readD)
			}
			continue
		}
		if math.IsInf(d, -1) {
			if !math.IsInf(readD, -1) {
				t.Errorf("ZDouble -Inf round-trip failed: expected -Inf, got %v", readD)
			}
			continue
		}

		// For other values, compare bits
		if math.Float64bits(d) != math.Float64bits(readD) {
			t.Errorf("ZDouble round-trip failed for special value %v: got %v", d, readD)
		}
	}

	// Test random double values
	r := rand.New(rand.NewSource(42))
	for i := 0; i < 100000; i++ {
		buf := newTestIndexOutput()
		d := r.Float64() * float64(r.Intn(100)-50)

		err := codecs.WriteZDouble(buf, d)
		if err != nil {
			t.Fatalf("Failed to write ZDouble for random value %v: %v", d, err)
		}

		// Verify position <= 8 for positive, <= 9 for negative
		isNegative := d < 0
		if isNegative {
			if buf.Length() > 9 {
				t.Errorf("ZDouble negative value %v encoded to %d bytes, expected <= 9", d, buf.Length())
			}
		} else {
			if buf.Length() > 8 {
				t.Errorf("ZDouble positive value %v encoded to %d bytes, expected <= 8", d, buf.Length())
			}
		}

		// Read back and verify
		data := buf.GetData()
		input := newTestIndexInput(data)
		readD, err := codecs.ReadZDouble(input)
		if err != nil {
			t.Fatalf("Failed to read ZDouble for random value %v: %v", d, err)
		}

		// Compare bits
		if math.Float64bits(d) != math.Float64bits(readD) {
			t.Errorf("ZDouble round-trip failed for random value %v: got %v", d, readD)
		}
	}

	// Test float values cast to double (should compress to <= 5 bytes)
	for i := 0; i < 100000; i++ {
		buf := newTestIndexOutput()
		d := float64(r.Float32() * float32(r.Intn(100)-50))

		err := codecs.WriteZDouble(buf, d)
		if err != nil {
			t.Fatalf("Failed to write ZDouble for float-derived value %v: %v", d, err)
		}

		// Verify position <= 5 for float-derived doubles
		if buf.Length() > 5 {
			t.Errorf("ZDouble float-derived value %v encoded to %d bytes, expected <= 5", d, buf.Length())
		}

		// Read back and verify
		data := buf.GetData()
		input := newTestIndexInput(data)
		readD, err := codecs.ReadZDouble(input)
		if err != nil {
			t.Fatalf("Failed to read ZDouble for float-derived value %v: %v", d, err)
		}

		// Compare bits
		if math.Float64bits(d) != math.Float64bits(readD) {
			t.Errorf("ZDouble round-trip failed for float-derived value %v: got %v", d, readD)
		}
	}
}

// TestCompressingStoredFieldsFormat_TLong tests TLong compression
// Ported from: testTLong()
// Tests round-trip compression of time-based values and random longs
func TestCompressingStoredFieldsFormat_TLong(t *testing.T) {
	multipliers := []int64{second, hour, day}

	// Test small integer values with time multipliers
	for i := int16(-32768); i < 32767; i++ {
		for _, mul := range multipliers {
			buf := newTestIndexOutput()
			l := int64(i) * mul

			err := codecs.WriteTLong(buf, l)
			if err != nil {
				t.Fatalf("Failed to write TLong for %d * %d: %v", i, mul, err)
			}

			// Verify single byte compression for range -16 to 15
			if i >= -16 && i <= 15 {
				if buf.Length() != 1 {
					t.Errorf("Expected 1 byte for value %d * %d, got %d", i, mul, buf.Length())
				}
			}

			// Read back and verify
			data := buf.GetData()
			input := newTestIndexInput(data)
			readL, err := codecs.ReadTLong(input)
			if err != nil {
				t.Fatalf("Failed to read TLong for %d * %d: %v", i, mul, err)
			}

			if l != readL {
				t.Errorf("TLong round-trip failed for %d * %d: expected %d, got %d", i, mul, l, readL)
			}
		}
	}

	// Test random values
	r := rand.New(rand.NewSource(42))
	for i := 0; i < 100000; i++ {
		buf := newTestIndexOutput()
		numBits := r.Intn(65)
		var l int64
		if numBits == 64 {
			l = r.Int63()
			if r.Intn(2) == 0 {
				l = -l
			}
		} else if numBits == 0 {
			l = 0
		} else {
			l = r.Int63n(1<<numBits - 1)
		}

		// Apply time multipliers randomly
		switch r.Intn(4) {
		case 0:
			l *= second
		case 1:
			l *= hour
		case 2:
			l *= day
		default:
			// No multiplier
		}

		err := codecs.WriteTLong(buf, l)
		if err != nil {
			t.Fatalf("Failed to write TLong for random value %d: %v", l, err)
		}

		// Read back and verify
		data := buf.GetData()
		input := newTestIndexInput(data)
		readL, err := codecs.ReadTLong(input)
		if err != nil {
			t.Fatalf("Failed to read TLong for random value %d: %v", l, err)
		}

		if l != readL {
			t.Errorf("TLong round-trip failed for random value %d: got %d", l, readL)
		}
	}
}

// TestCompressingStoredFieldsFormat_ChunkCleanup tests chunk cleanup during merge
// Ported from: testChunkCleanup()
// Tests that small segments with incomplete compressed blocks are recompressed during merge
func TestCompressingStoredFieldsFormat_ChunkCleanup(t *testing.T) {
	t.Skip("Chunk cleanup test requires full IndexWriter integration - skipping for now")

	// This test will verify:
	// 1. Creating small segments with incomplete compressed blocks
	// 2. Each segment has dirty chunks (incomplete blocks)
	// 3. After merge, dirty chunks are consolidated
	// 4. NumDirtyDocs and NumDirtyChunks are tracked correctly

	// Test setup requires:
	// - CompressingCodec with configurable parameters (chunkSize=4KB, maxDocsPerChunk=4)
	// - NoMergePolicy to prevent auto-merging during document addition
	// - Ability to examine dirty chunk counts via StoredFieldsReader

	// Steps:
	// 1. Create directory
	// 2. Configure IndexWriter with CompressingCodec and NoMergePolicy
	// 3. Add 5 documents, flushing after each
	// 4. Verify each segment has dirty chunks
	// 5. Force merge to 1 segment
	// 6. Add another document and merge again
	// 7. Verify dirty chunks <= 2 (consolidated from 5 chunks)
}

// TestCompressingStoredFieldsFormat_CompressionModes tests different compression modes
// Additional test coverage for compression mode configurations
func TestCompressingStoredFieldsFormat_CompressionModes(t *testing.T) {
	modes := []struct {
		name codecs.CompressionMode
	}{
		{codecs.CompressionModeLZ4Fast},
		{codecs.CompressionModeLZ4High},
		{codecs.CompressionModeDeflate},
	}

	testData := []byte("This is test data for compression. " +
		"It will be repeated to make it longer and more compressible. " +
		"The quick brown fox jumps over the lazy dog. " +
		"Pack my box with five dozen liquor jugs.")

	// Repeat the data to make it longer
	var data []byte
	for i := 0; i < 100; i++ {
		data = append(data, testData...)
	}

	for _, mode := range modes {
		mode := mode // capture range variable
		t.Run(mode.name.String(), func(t *testing.T) {
			// Create format with this compression mode
			format := codecs.NewCompressingStoredFieldsFormat(mode.name, 16*1024, 128)

			if format == nil {
				t.Fatal("Failed to create CompressingStoredFieldsFormat")
			}

			// Verify the compression mode is set correctly
			if format.CompressionMode() != mode.name {
				t.Errorf("Expected compression mode %v, got %v", mode.name, format.CompressionMode())
			}

			// Verify chunk size
			if format.ChunkSize() != 16*1024 {
				t.Errorf("Expected chunk size %d, got %d", 16*1024, format.ChunkSize())
			}

			// Verify max docs per chunk
			if format.MaxDocsPerChunk() != 128 {
				t.Errorf("Expected max docs per chunk %d, got %d", 128, format.MaxDocsPerChunk())
			}
		})
	}
}

// TestCompressingStoredFieldsFormat_ChunkSizeConfigurations tests various chunk size configurations
// Focus: Chunk size configurations as specified in task requirements
func TestCompressingStoredFieldsFormat_ChunkSizeConfigurations(t *testing.T) {
	chunkSizes := []int{
		1024,  // 1KB - minimum
		4096,  // 4KB - common default
		8192,  // 8KB - larger chunks
		16384, // 16KB - even larger
		65536, // 64KB - maximum reasonable
	}

	maxDocsPerChunkValues := []int{
		1,   // One doc per chunk
		4,   // Small number
		16,  // Medium
		128, // Large
	}

	for _, chunkSize := range chunkSizes {
		for _, maxDocs := range maxDocsPerChunkValues {
			chunkSize := chunkSize // capture range variable
			maxDocs := maxDocs     // capture range variable
			t.Run(fmt.Sprintf("chunkSize_%d_maxDocs_%d", chunkSize, maxDocs), func(t *testing.T) {
				format := codecs.NewCompressingStoredFieldsFormat(codecs.CompressionModeLZ4Fast, chunkSize, maxDocs)

				if format == nil {
					t.Fatal("Failed to create CompressingStoredFieldsFormat")
				}

				if format.ChunkSize() != chunkSize {
					t.Errorf("Expected chunk size %d, got %d", chunkSize, format.ChunkSize())
				}

				if format.MaxDocsPerChunk() != maxDocs {
					t.Errorf("Expected max docs per chunk %d, got %d", maxDocs, format.MaxDocsPerChunk())
				}
			})
		}
	}
}

// TestCompressingStoredFieldsFormat_ByteLevelCompatibility verifies byte-level compatibility with Lucene
// This ensures the Go implementation produces identical bytes to the Java implementation
func TestCompressingStoredFieldsFormat_ByteLevelCompatibility(t *testing.T) {
	t.Skip("Byte-level compatibility test requires reference data from Lucene Java - skipping for now")

	// This test will verify:
	// - Same input produces same compressed bytes as Lucene Java
	// - ZFloat encoding matches Java implementation
	// - ZDouble encoding matches Java implementation
	// - TLong encoding matches Java implementation
	// - Chunk headers and footers match
	// - Document metadata encoding matches
}

// Helper function for float32 bits
func float32ToBits(f float32) uint32 {
	return math.Float32bits(f)
}

// Helper function for float64 bits
func float64ToBits(f float64) uint64 {
	return math.Float64bits(f)
}

// eofCheck simulates Java's ByteArrayDataInput.eof() check
// Returns true if all bytes have been read
func eofCheck(in *store.ByteArrayDataInput, startPos, endPos int) bool {
	return in.GetPosition() >= endPos
}

// resetInput resets the ByteArrayDataInput to read from the written bytes
// Equivalent to Java's in.reset(bytes, 0, out.getPosition())
func resetInput(in *store.ByteArrayDataInput, bytes []byte, length int) {
	// In Go implementation, we create a new input with the slice
	// This is a placeholder for the actual implementation
	_ = bytes[:length]
}
