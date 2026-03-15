// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"fmt"
	"math"
	"testing"
)

// TestAlignOffset tests the AlignOffset function for correct alignment calculations.
// This is the Go port of Lucene's TestIndexOutputAlignment.testAlignmentCalculation().
// Source: lucene/core/src/test/org/apache/lucene/store/TestIndexOutputAlignment.java
func TestAlignOffset(t *testing.T) {
	tests := []struct {
		name           string
		offset         int64
		alignmentBytes int
		want           int64
	}{
		// Zero offset should always return 0
		{"zero offset with long alignment", 0, 8, 0},
		{"zero offset with int alignment", 0, 4, 0},
		{"zero offset with short alignment", 0, 2, 0},
		{"zero offset with byte alignment", 0, 1, 0},

		// Offset 1 should round up to next alignment boundary
		{"offset 1 with long alignment", 1, 8, 8},
		{"offset 1 with int alignment", 1, 4, 4},
		{"offset 1 with short alignment", 1, 2, 2},
		{"offset 1 with byte alignment", 1, 1, 1},

		// Offset 25 should round up to next alignment boundary
		{"offset 25 with long alignment", 25, 8, 32},
		{"offset 25 with int alignment", 25, 4, 28},
		{"offset 25 with short alignment", 25, 2, 26},
		{"offset 25 with byte alignment", 25, 1, 25},

		// Large value test: 1 << 48
		{"large value with long alignment", (1 << 48) - 1, 8, 1 << 48},
		{"large value with int alignment", (1 << 48) - 1, 4, 1 << 48},
		{"large value with short alignment", (1 << 48) - 1, 2, 1 << 48},
		// Byte alignment never changes anything
		{"large value with byte alignment", (1 << 48) - 1, 1, (1 << 48) - 1},

		// Max value with byte alignment should return max value
		{"max int64 with byte alignment", math.MaxInt64, 1, math.MaxInt64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AlignOffset(tt.offset, tt.alignmentBytes)
			if err != nil {
				t.Fatalf("AlignOffset(%d, %d) unexpected error: %v", tt.offset, tt.alignmentBytes, err)
			}
			if got != tt.want {
				t.Errorf("AlignOffset(%d, %d) = %d, want %d", tt.offset, tt.alignmentBytes, got, tt.want)
			}
		})
	}
}

// TestAlignOffsetInvalid tests that invalid alignments return errors.
// This is the Go port of Lucene's TestIndexOutputAlignment.testInvalidAlignments().
// Source: lucene/core/src/test/org/apache/lucene/store/TestIndexOutputAlignment.java
func TestAlignOffsetInvalid(t *testing.T) {
	tests := []struct {
		name           string
		offset         int64
		alignmentBytes int
	}{
		{"zero alignment", 1, 0},
		{"negative one alignment", 1, -1},
		{"negative two alignment", 1, -2},
		{"six alignment (not power of 2)", 1, 6},
		{"43 alignment (not power of 2)", 1, 43},
		{"min int alignment", 1, math.MinInt32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := AlignOffset(tt.offset, tt.alignmentBytes)
			if err == nil {
				t.Errorf("AlignOffset(%d, %d) expected error for invalid alignment", tt.offset, tt.alignmentBytes)
			}
		})
	}

	// Test negative offset error
	t.Run("negative offset", func(t *testing.T) {
		_, err := AlignOffset(-1, 1)
		if err == nil {
			t.Error("AlignOffset(-1, 1) expected error for negative offset")
		}
	})

	// Test overflow condition
	t.Run("overflow with max int64", func(t *testing.T) {
		_, err := AlignOffset(math.MaxInt64, 2)
		if err == nil {
			t.Error("AlignOffset(MaxInt64, 2) expected overflow error")
		}
	})
}

// TestAlignFilePointer tests the AlignFilePointer function.
// This is the Go port of Lucene's TestIndexOutputAlignment.testOutputAlignment().
// Source: lucene/core/src/test/org/apache/lucene/store/TestIndexOutputAlignment.java
func TestAlignFilePointer(t *testing.T) {
	alignments := []int{8, 4, 2, 1} // Long, Int, Short, Byte bytes

	for _, alignment := range alignments {
		t.Run(fmt.Sprintf("alignment_%d", alignment), func(t *testing.T) {
			testAlignFilePointerWithAlignment(t, alignment)
		})
	}
}

func testAlignFilePointerWithAlignment(t *testing.T, alignment int) {
	// Create a test IndexOutput using ByteArrayDataOutput as the underlying implementation
	// We wrap it to provide IndexOutput interface
	out := &testIndexOutputForAlignment{
		name:         "test_output",
		dataOutput:   NewByteArrayDataOutput(8192),
		filePointer:  0,
	}

	// Run multiple iterations with random byte writes
	// Using a fixed seed for reproducibility in tests
	for i := 0; i < 100; i++ {
		// Write some random bytes (0-31 bytes)
		length := i % 32
		if length > 0 {
			data := make([]byte, length)
			for j := range data {
				data[j] = byte(j % 256)
			}
			if err := out.WriteBytes(data); err != nil {
				t.Fatalf("WriteBytes failed: %v", err)
			}
		}

		origPos := out.GetFilePointer()

		// Align to next boundary
		newPos, err := AlignFilePointer(out, alignment)
		if err != nil {
			t.Fatalf("AlignFilePointer failed: %v", err)
		}

		// Verify the file pointer matches returned position
		if out.GetFilePointer() != newPos {
			t.Errorf("file pointer %d != returned position %d", out.GetFilePointer(), newPos)
		}

		// Verify alignment
		if newPos%int64(alignment) != 0 {
			t.Errorf("position %d not aligned to %d", newPos, alignment)
		}

		// Verify new position >= original position
		if newPos < origPos {
			t.Errorf("new position %d < original position %d", newPos, origPos)
		}

		// Verify padding added is less than alignment
		padding := newPos - origPos
		if padding >= int64(alignment) {
			t.Errorf("padding %d >= alignment %d", padding, alignment)
		}

		// Verify padding bytes are zeros
		if padding > 0 {
			written := out.dataOutput.GetBytes()
			for j := origPos; j < newPos; j++ {
				if written[j] != 0 {
					t.Errorf("padding byte at position %d is not zero: %d", j, written[j])
				}
			}
		}
	}
}

// testIndexOutputForAlignment is a test implementation of IndexOutput for alignment tests.
type testIndexOutputForAlignment struct {
	name        string
	dataOutput  *ByteArrayDataOutput
	filePointer int64
}

func (o *testIndexOutputForAlignment) WriteByte(b byte) error {
	if err := o.dataOutput.WriteByte(b); err != nil {
		return err
	}
	o.filePointer++
	return nil
}

func (o *testIndexOutputForAlignment) WriteBytes(b []byte) error {
	if err := o.dataOutput.WriteBytes(b); err != nil {
		return err
	}
	o.filePointer += int64(len(b))
	return nil
}

func (o *testIndexOutputForAlignment) WriteBytesN(b []byte, n int) error {
	if err := o.dataOutput.WriteBytesN(b, n); err != nil {
		return err
	}
	o.filePointer += int64(n)
	return nil
}

func (o *testIndexOutputForAlignment) GetFilePointer() int64 {
	return o.filePointer
}

func (o *testIndexOutputForAlignment) Length() int64 {
	return o.filePointer
}

func (o *testIndexOutputForAlignment) GetName() string {
	return o.name
}

func (o *testIndexOutputForAlignment) Close() error {
	return nil
}

func (o *testIndexOutputForAlignment) WriteShort(i int16) error {
	if err := o.dataOutput.WriteShort(i); err != nil {
		return err
	}
	o.filePointer += 2
	return nil
}

func (o *testIndexOutputForAlignment) WriteInt(i int32) error {
	if err := o.dataOutput.WriteInt(i); err != nil {
		return err
	}
	o.filePointer += 4
	return nil
}

func (o *testIndexOutputForAlignment) WriteLong(i int64) error {
	if err := o.dataOutput.WriteLong(i); err != nil {
		return err
	}
	o.filePointer += 8
	return nil
}

func (o *testIndexOutputForAlignment) WriteString(s string) error {
	if err := o.dataOutput.WriteString(s); err != nil {
		return err
	}
	// String length is variable, so we need to calculate the actual bytes written
	// VInt encoding: 1-5 bytes for length + string bytes
	lengthBytes := 1
	if len(s) > 127 {
		lengthBytes = 2
	}
	if len(s) > 16383 {
		lengthBytes = 3
	}
	if len(s) > 2097151 {
		lengthBytes = 4
	}
	if len(s) > 268435455 {
		lengthBytes = 5
	}
	o.filePointer += int64(lengthBytes + len(s))
	return nil
}

// TestAlignOffsetEdgeCases tests additional edge cases for AlignOffset.
func TestAlignOffsetEdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		offset         int64
		alignmentBytes int
		want           int64
		wantErr        bool
	}{
		// Already aligned offsets should return the same value
		{"already aligned to 8", 16, 8, 16, false},
		{"already aligned to 4", 20, 4, 20, false},
		{"already aligned to 2", 10, 2, 10, false},
		{"already aligned to 1", 5, 1, 5, false},

		// One less than alignment boundary
		{"one less than 8", 7, 8, 8, false},
		{"one less than 4", 3, 4, 4, false},
		{"one less than 2", 1, 2, 2, false},

		// Large alignments
		{"large alignment 1024", 1000, 1024, 1024, false},
		{"large alignment 4096", 4000, 4096, 4096, false},
		{"large alignment 65536", 65000, 65536, 65536, false},

		// Edge case: offset 0 with various alignments
		{"offset 0 alignment 1024", 0, 1024, 0, false},
		{"offset 0 alignment 4096", 0, 4096, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AlignOffset(tt.offset, tt.alignmentBytes)
			if (err != nil) != tt.wantErr {
				t.Fatalf("AlignOffset(%d, %d) error = %v, wantErr %v", tt.offset, tt.alignmentBytes, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("AlignOffset(%d, %d) = %d, want %d", tt.offset, tt.alignmentBytes, got, tt.want)
			}
		})
	}
}

// TestAlignFilePointerEdgeCases tests edge cases for AlignFilePointer.
func TestAlignFilePointerEdgeCases(t *testing.T) {
	t.Run("align at position 0", func(t *testing.T) {
		out := &testIndexOutputForAlignment{
			name:        "test",
			dataOutput:  NewByteArrayDataOutput(100),
			filePointer: 0,
		}

		newPos, err := AlignFilePointer(out, 8)
		if err != nil {
			t.Fatalf("AlignFilePointer failed: %v", err)
		}
		if newPos != 0 {
			t.Errorf("expected position 0, got %d", newPos)
		}
	})

	t.Run("align when already aligned", func(t *testing.T) {
		out := &testIndexOutputForAlignment{
			name:        "test",
			dataOutput:  NewByteArrayDataOutput(100),
			filePointer: 0,
		}

		// Write 8 bytes to get to position 8 (already aligned)
		out.WriteBytes([]byte{1, 2, 3, 4, 5, 6, 7, 8})

		newPos, err := AlignFilePointer(out, 8)
		if err != nil {
			t.Fatalf("AlignFilePointer failed: %v", err)
		}
		if newPos != 8 {
			t.Errorf("expected position 8, got %d", newPos)
		}

		// Verify no extra bytes were written
		if out.GetFilePointer() != 8 {
			t.Errorf("expected file pointer 8, got %d", out.GetFilePointer())
		}
	})

	t.Run("verify padding bytes are zero", func(t *testing.T) {
		out := &testIndexOutputForAlignment{
			name:        "test",
			dataOutput:  NewByteArrayDataOutput(100),
			filePointer: 0,
		}

		// Write 5 bytes
		out.WriteBytes([]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE})

		// Align to 8 bytes
		AlignFilePointer(out, 8)

		// Check the padding bytes
		written := out.dataOutput.GetBytes()
		if len(written) != 8 {
			t.Fatalf("expected 8 bytes written, got %d", len(written))
		}

		// First 5 bytes should be the data
		for i := 0; i < 5; i++ {
			if written[i] == 0 {
				t.Errorf("data byte at position %d is zero", i)
			}
		}

		// Last 3 bytes should be padding (zeros)
		for i := 5; i < 8; i++ {
			if written[i] != 0 {
				t.Errorf("padding byte at position %d is not zero: %d", i, written[i])
			}
		}
	})
}

// TestAlignOffsetMemoryMappedOptimization documents that alignment is used for mmap optimization.
// This test verifies the specific alignments used for memory-mapped I/O.
func TestAlignOffsetMemoryMappedOptimization(t *testing.T) {
	// Common alignment values used for memory-mapped file optimization
	mmapAlignments := []int{
		1,   // Byte alignment (no alignment)
		2,   // Short alignment
		4,   // Int alignment (common for 32-bit values)
		8,   // Long alignment (common for 64-bit values)
		16,  // Cache line alignment
		32,  // Larger cache line
		64,  // Common cache line size
		128, // Large alignment for vector operations
		4096, // Page alignment (common for mmap)
	}

	for _, alignment := range mmapAlignments {
		t.Run(fmt.Sprintf("mmap_alignment_%d", alignment), func(t *testing.T) {
			// Test various offsets
			for offset := int64(0); offset < int64(alignment*3); offset += int64(alignment/2 + 1) {
				aligned, err := AlignOffset(offset, alignment)
				if err != nil {
					t.Fatalf("AlignOffset(%d, %d) failed: %v", offset, alignment, err)
				}

				// Verify alignment
				if aligned%int64(alignment) != 0 {
					t.Errorf("AlignOffset(%d, %d) = %d, not aligned", offset, alignment, aligned)
				}

				// Verify aligned >= offset
				if aligned < offset {
					t.Errorf("AlignOffset(%d, %d) = %d < offset", offset, alignment, aligned)
				}

				// Verify padding is minimal
				if aligned-offset >= int64(alignment) {
					t.Errorf("AlignOffset(%d, %d) added too much padding: %d", offset, alignment, aligned-offset)
				}
			}
		})
	}
}

// BenchmarkAlignOffset benchmarks the AlignOffset function.
func BenchmarkAlignOffset(b *testing.B) {
	for i := 0; i < b.N; i++ {
		AlignOffset(int64(i%1000), 8)
	}
}

// BenchmarkAlignFilePointer benchmarks the AlignFilePointer function.
func BenchmarkAlignFilePointer(b *testing.B) {
	out := &testIndexOutputForAlignment{
		name:        "bench",
		dataOutput:  NewByteArrayDataOutput(8192),
		filePointer: 0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%10 == 0 {
			// Reset periodically to avoid buffer overflow
			out.filePointer = 0
			out.dataOutput.Reset()
		}
		out.WriteByte(byte(i))
		AlignFilePointer(out, 8)
	}
}
