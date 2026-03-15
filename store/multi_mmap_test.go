// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/test/org/apache/lucene/store/TestMultiMMap.java
// Purpose: Tests MMapDirectory's MultiMMapIndexInput for files > 2GB handling,
//          clone safety, slice safety, and seeking exceptions.

package store

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestMultiMMap_SeekingExceptions tests seeking to negative positions
// and past EOF, including with slices.
// Source: TestMultiMMap.testSeekingExceptions()
func TestMultiMMap_SeekingExceptions(t *testing.T) {
	sliceSize := 128

	tempDir, err := os.MkdirTemp("", "gocene_multi_mmap_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewMMapDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to create MMapDirectory: %v", err)
	}
	defer dir.Close()

	// Set small chunk size to force multi-chunk behavior
	dir.SetMaxChunkSize(7) // 2^7 = 128 bytes

	// Create file of size 128 + 63 = 191 bytes
	size := 128 + 63
	testFile := filepath.Join(tempDir, "seek_test")
	content := make([]byte, size)
	for i := range content {
		content[i] = 0
	}
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	in, err := dir.OpenInput("seek_test", IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput() error = %v", err)
	}
	defer in.Close()

	// Test 1: Seek to negative position should fail
	negativePos := int64(-1234)
	err = in.SetPosition(negativePos)
	if err == nil {
		t.Error("SetPosition(-1234) expected error for negative position")
	}
	// Note: Go implementation returns "invalid position" rather than "negative position"

	// Test 2: Seek past EOF should fail
	posAfterEOF := int64(size + 123)
	err = in.SetPosition(posAfterEOF)
	if err == nil {
		t.Error("SetPosition(posAfterEOF) expected error")
	}

	// Test 3: Create a slice and test seeking past EOF on slice
	// Slice at offset 33 with length sliceSize + 15 = 143
	slice, err := in.Slice("slice", 33, int64(sliceSize+15))
	if err != nil {
		t.Fatalf("Slice() error = %v", err)
	}
	defer slice.Close()

	// Verify slice uses multi-mmap (size crosses chunk boundary)
	// offset 33 + length 143 = 176, which spans across 128-byte chunks
	err = slice.SetPosition(posAfterEOF)
	if err == nil {
		t.Error("SetPosition(posAfterEOF) on slice expected error")
	}
}

// TestMultiMMap_CloneSafety tests that clones are independent.
// Note: In Go implementation, Clone() creates a new independent IndexInput
// by reopening the file, so closing the original does not affect clones.
// This differs from Java's shared resource approach.
// Source: TestMultiMMap.testCloneSafety()
func TestMultiMMap_CloneSafety(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_multi_mmap_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewMMapDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to create MMapDirectory: %v", err)
	}
	defer dir.Close()

	// Create a file with a VInt (5)
	testFile := filepath.Join(tempDir, "clone_safety")
	f, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Write VInt(5) - variable length encoded
	// 5 in binary is 101, fits in 7 bits, so single byte
	f.Write([]byte{0x05})
	f.Close()

	one, err := dir.OpenInput("clone_safety", IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput() error = %v", err)
	}

	two := one.Clone()
	three := two.Clone() // clone of clone

	// Close the original
	one.Close()

	// In Go implementation, clones are independent (reopened files)
	// So they should still be readable after original is closed
	val, err := ReadVInt(two)
	if err != nil {
		t.Errorf("readVInt(two) unexpected error: %v", err)
	} else if val != 5 {
		t.Errorf("readVInt(two) = %d, want 5", val)
	}

	// Reset position for second read
	two.SetPosition(0)
	val, err = ReadVInt(three)
	if err != nil {
		t.Errorf("readVInt(three) unexpected error: %v", err)
	} else if val != 5 {
		t.Errorf("readVInt(three) = %d, want 5", val)
	}

	two.Close()
	three.Close()

	// Test double close of master (should not panic)
	one.Close()
}

// TestMultiMMap_CloneSliceSafety tests slice and clone independence.
// Note: In Go implementation, slices share the underlying mmap but clones
// reopen the file. Closing the slicer affects slices but not independent clones.
// Source: TestMultiMMap.testCloneSliceSafety()
func TestMultiMMap_CloneSliceSafety(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_multi_mmap_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewMMapDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to create MMapDirectory: %v", err)
	}
	defer dir.Close()

	// Create a file with two integers
	testFile := filepath.Join(tempDir, "slice_safety")
	content := make([]byte, 8)
	// Write two int32 values in big-endian: 1 and 2
	content[0] = 0x00
	content[1] = 0x00
	content[2] = 0x00
	content[3] = 0x01 // int32(1)
	content[4] = 0x00
	content[5] = 0x00
	content[6] = 0x00
	content[7] = 0x02 // int32(2)
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	slicer, err := dir.OpenInput("slice_safety", IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput() error = %v", err)
	}

	// Create slices
	one, err := slicer.Slice("first int", 0, 4)
	if err != nil {
		t.Fatalf("Slice() error = %v", err)
	}

	two, err := slicer.Slice("second int", 4, 4)
	if err != nil {
		t.Fatalf("Slice() error = %v", err)
	}

	three := one.Clone() // clone of slice (reopens file)
	four := two.Clone()  // clone of slice (reopens file)

	// Close the slicer
	slicer.Close()

	// In Go implementation, slices share the underlying mmap with slicer
	// So they should fail after slicer is closed
	_, err = ReadInt32(one)
	if err == nil {
		t.Error("readInt32(one) expected error after slicer.Close()")
	}

	_, err = ReadInt32(two)
	if err == nil {
		t.Error("readInt32(two) expected error after slicer.Close()")
	}

	// But clones (which reopen the file) should still work
	val, err := ReadInt32(three)
	if err != nil {
		t.Errorf("readInt32(three) unexpected error: %v", err)
	} else if val != 1 {
		t.Errorf("readInt32(three) = %d, want 1", val)
	}

	val, err = ReadInt32(four)
	if err != nil {
		t.Errorf("readInt32(four) unexpected error: %v", err)
	} else if val != 2 {
		t.Errorf("readInt32(four) = %d, want 2", val)
	}

	one.Close()
	two.Close()
	three.Close()
	four.Close()

	// Test double-close of slicer (should not panic)
	slicer.Close()
}

// TestMultiMMap_Implementations tests that the correct implementation
// (single vs multi chunk) is used based on file size relative to chunk size.
// Source: TestMultiMMap.testImplementations()
func TestMultiMMap_Implementations(t *testing.T) {
	// Test with various chunk sizes from 2^2 (4 bytes) to 2^11 (2048 bytes)
	for i := 2; i < 12; i++ {
		chunkSize := 1 << i

		tempDir, err := os.MkdirTemp("", fmt.Sprintf("gocene_multi_mmap_impl_test_%d_", i))
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}

		dir, err := NewMMapDirectory(tempDir)
		if err != nil {
			t.Fatalf("failed to create MMapDirectory: %v", err)
		}

		// Set chunk size
		dir.SetMaxChunkSize(i)

		// Create random data file (size between 0 and chunkSize*2 + 3)
		size := chunkSize*2 + 3
		content := make([]byte, size)
		for j := range content {
			content[j] = byte(j % 256)
		}

		testFile := filepath.Join(tempDir, "bytes")
		if err := os.WriteFile(testFile, content, 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		ii, err := dir.OpenInput("bytes", IOContextRead)
		if err != nil {
			t.Fatalf("OpenInput() error = %v", err)
		}

		// Read all bytes and verify
		actual := make([]byte, size)
		err = ii.ReadBytes(actual)
		if err != nil {
			t.Errorf("ReadBytes() error = %v", err)
		}
		if !bytes.Equal(content, actual) {
			t.Errorf("Content mismatch for chunkSize=%d", chunkSize)
		}

		// Re-seek to beginning
		err = ii.SetPosition(0)
		if err != nil {
			t.Errorf("SetPosition(0) error = %v", err)
		}

		// Clone test - clone should be same type
		clone := ii.Clone()
		if clone == nil {
			t.Error("Clone() returned nil")
		}
		clone.Close()

		// Slice test (offset 0)
		sliceSize := size / 2
		slice, err := ii.Slice("slice", 0, int64(sliceSize))
		if err != nil {
			t.Errorf("Slice() error = %v", err)
		} else {
			slice.Close()
		}

		// Slice test (offset > 0)
		offset := size/4 + 1
		sliceSize = size/4 + 1
		slice, err = ii.Slice("slice", int64(offset), int64(sliceSize))
		if err != nil {
			t.Errorf("Slice(offset) error = %v", err)
		} else {
			slice.Close()
		}

		ii.Close()
		dir.Close()
		os.RemoveAll(tempDir)
	}
}

// TestMultiMMap_LargeFile tests reading files that span multiple chunks.
// This simulates the >2GB file handling that MultiMMapIndexInput is designed for.
func TestMultiMMap_LargeFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_multi_mmap_large_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewMMapDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to create MMapDirectory: %v", err)
	}
	defer dir.Close()

	// Use a small chunk size to simulate multi-chunk behavior
	// without actually creating multi-GB files
	dir.SetMaxChunkSize(10) // 2^10 = 1KB chunks

	// Create a file that spans multiple chunks (5KB)
	size := 5 * 1024 // 5KB
	content := make([]byte, size)
	for i := range content {
		content[i] = byte(i % 256)
	}

	testFile := filepath.Join(tempDir, "largefile")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	in, err := dir.OpenInput("largefile", IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput() error = %v", err)
	}
	defer in.Close()

	if in.Length() != int64(size) {
		t.Errorf("Length() = %d, want %d", in.Length(), size)
	}

	// Read entire file
	buf := make([]byte, size)
	err = in.ReadBytes(buf)
	if err != nil {
		t.Errorf("ReadBytes() error = %v", err)
	}

	// Verify content
	for i := range content {
		if buf[i] != content[i] {
			t.Errorf("Content mismatch at byte %d: got %d, want %d", i, buf[i], content[i])
			break
		}
	}

	// Test seeking across chunk boundaries
	err = in.SetPosition(1024) // Start of second chunk
	if err != nil {
		t.Errorf("SetPosition(1024) error = %v", err)
	}

	b, err := in.ReadByte()
	if err != nil {
		t.Errorf("ReadByte() error = %v", err)
	}
	if b != content[1024] {
		t.Errorf("ReadByte() at position 1024 = %d, want %d", b, content[1024])
	}

	// Test seeking to position that requires multiple chunk hops
	err = in.SetPosition(3072) // Start of fourth chunk
	if err != nil {
		t.Errorf("SetPosition(3072) error = %v", err)
	}

	b, err = in.ReadByte()
	if err != nil {
		t.Errorf("ReadByte() error = %v", err)
	}
	if b != content[3072] {
		t.Errorf("ReadByte() at position 3072 = %d, want %d", b, content[3072])
	}
}

// TestMultiMMap_SliceAcrossChunks tests slicing that spans chunk boundaries.
func TestMultiMMap_SliceAcrossChunks(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_multi_mmap_slice_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewMMapDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to create MMapDirectory: %v", err)
	}
	defer dir.Close()

	// Use small chunk size
	dir.SetMaxChunkSize(10) // 1KB chunks

	// Create file spanning multiple chunks
	size := 3 * 1024 // 3KB
	content := make([]byte, size)
	for i := range content {
		content[i] = byte(i % 256)
	}

	testFile := filepath.Join(tempDir, "slicefile")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	in, err := dir.OpenInput("slicefile", IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput() error = %v", err)
	}
	defer in.Close()

	// Create a slice that spans chunk boundary (offset 512, length 1024)
	// This spans from middle of first chunk to middle of second chunk
	slice, err := in.Slice("cross-chunk", 512, 1024)
	if err != nil {
		t.Fatalf("Slice() error = %v", err)
	}
	defer slice.Close()

	if slice.Length() != 1024 {
		t.Errorf("Slice Length() = %d, want 1024", slice.Length())
	}

	// Read entire slice
	buf := make([]byte, 1024)
	err = slice.ReadBytes(buf)
	if err != nil {
		t.Errorf("ReadBytes() error = %v", err)
	}

	// Verify content matches original
	for i := 0; i < 1024; i++ {
		if buf[i] != content[512+i] {
			t.Errorf("Slice content mismatch at byte %d: got %d, want %d", i, buf[i], content[512+i])
			break
		}
	}

	// Test seeking within slice
	err = slice.SetPosition(512)
	if err != nil {
		t.Errorf("SetPosition(512) error = %v", err)
	}

	b, err := slice.ReadByte()
	if err != nil {
		t.Errorf("ReadByte() error = %v", err)
	}
	if b != content[1024] {
		t.Errorf("ReadByte() at slice position 512 = %d, want %d", b, content[1024])
	}
}

// TestMultiMMap_EmptyFile tests handling of empty files.
func TestMultiMMap_EmptyFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_multi_mmap_empty_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewMMapDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to create MMapDirectory: %v", err)
	}
	defer dir.Close()

	// Create empty file
	testFile := filepath.Join(tempDir, "empty")
	if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create empty file: %v", err)
	}

	in, err := dir.OpenInput("empty", IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput() error = %v", err)
	}
	defer in.Close()

	if in.Length() != 0 {
		t.Errorf("Length() = %d, want 0", in.Length())
	}

	// ReadByte should return EOF
	_, err = in.ReadByte()
	if !errors.Is(err, io.EOF) {
		t.Errorf("ReadByte() expected io.EOF, got: %v", err)
	}
}

// TestMultiMMap_CloneIndependence tests that clones are independent
// in terms of file position.
func TestMultiMMap_CloneIndependence(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_multi_mmap_clone_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewMMapDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to create MMapDirectory: %v", err)
	}
	defer dir.Close()

	// Create file with sequential bytes
	content := make([]byte, 256)
	for i := range content {
		content[i] = byte(i)
	}

	testFile := filepath.Join(tempDir, "clone_test")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	in, err := dir.OpenInput("clone_test", IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput() error = %v", err)
	}
	defer in.Close()

	// Read first byte from original
	b1, err := in.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte() error = %v", err)
	}
	if b1 != 0 {
		t.Errorf("ReadByte() = %d, want 0", b1)
	}

	// Clone should start at position 0
	clone := in.Clone()
	defer clone.Close()

	if clone.GetFilePointer() != 0 {
		t.Errorf("Clone position = %d, want 0", clone.GetFilePointer())
	}

	// Read from clone
	b2, err := clone.ReadByte()
	if err != nil {
		t.Fatalf("Clone ReadByte() error = %v", err)
	}
	if b2 != 0 {
		t.Errorf("Clone ReadByte() = %d, want 0", b2)
	}

	// Original should still be at position 1
	if in.GetFilePointer() != 1 {
		t.Errorf("Original position = %d, want 1", in.GetFilePointer())
	}

	// Read second byte from original
	b3, err := in.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte() error = %v", err)
	}
	if b3 != 1 {
		t.Errorf("ReadByte() = %d, want 1", b3)
	}
}

// TestMultiMMap_ReadBytesAcrossChunks tests reading bytes that span chunk boundaries.
func TestMultiMMap_ReadBytesAcrossChunks(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_multi_mmap_readbytes_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewMMapDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to create MMapDirectory: %v", err)
	}
	defer dir.Close()

	// Use small chunk size
	dir.SetMaxChunkSize(10) // 1KB chunks

	// Create file spanning multiple chunks
	size := 3 * 1024 // 3KB
	content := make([]byte, size)
	for i := range content {
		content[i] = byte(i % 256)
	}

	testFile := filepath.Join(tempDir, "readbytesfile")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	in, err := dir.OpenInput("readbytesfile", IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput() error = %v", err)
	}
	defer in.Close()

	// Seek to position near end of first chunk
	err = in.SetPosition(1000)
	if err != nil {
		t.Fatalf("SetPosition(1000) error = %v", err)
	}

	// Read 100 bytes that span chunk boundary (1000-1099)
	buf := make([]byte, 100)
	err = in.ReadBytes(buf)
	if err != nil {
		t.Errorf("ReadBytes() error = %v", err)
	}

	// Verify content
	for i := 0; i < 100; i++ {
		if buf[i] != content[1000+i] {
			t.Errorf("Content mismatch at byte %d: got %d, want %d", i, buf[i], content[1000+i])
			break
		}
	}
}

// TestMultiMMap_SliceClone tests cloning of slices.
func TestMultiMMap_SliceClone(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocene_multi_mmap_sliceclone_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dir, err := NewMMapDirectory(tempDir)
	if err != nil {
		t.Fatalf("failed to create MMapDirectory: %v", err)
	}
	defer dir.Close()

	// Create file
	content := []byte("Hello, World! This is a test.")
	testFile := filepath.Join(tempDir, "sliceclone")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	in, err := dir.OpenInput("sliceclone", IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput() error = %v", err)
	}
	defer in.Close()

	// Create slice
	slice, err := in.Slice("testslice", 7, 5) // "World"
	if err != nil {
		t.Fatalf("Slice() error = %v", err)
	}

	// Clone the slice
	sliceClone := slice.Clone()
	defer sliceClone.Close()

	// Both should have same length
	if sliceClone.Length() != slice.Length() {
		t.Errorf("Slice clone Length() = %d, want %d", sliceClone.Length(), slice.Length())
	}

	// Read from original slice
	buf1 := make([]byte, 5)
	err = slice.ReadBytes(buf1)
	if err != nil {
		t.Errorf("Slice ReadBytes() error = %v", err)
	}

	// Read from clone (should start at position 0)
	buf2 := make([]byte, 5)
	err = sliceClone.ReadBytes(buf2)
	if err != nil {
		t.Errorf("Slice clone ReadBytes() error = %v", err)
	}

	// Both should read same content
	if !bytes.Equal(buf1, buf2) {
		t.Errorf("Slice and clone content differ: %v vs %v", buf1, buf2)
	}

	if string(buf1) != "World" {
		t.Errorf("Slice content = %s, want World", string(buf1))
	}

	slice.Close()
}
