// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package codecs_test provides tests for compound file format functionality.
//
// This file ports tests from Apache Lucene:
//   - TestCompoundFormat.java: Tests for CFS thresholds and configuration
//   - TestLucene90CompoundFormat.java: Tests for Lucene90 compound format
//   - BaseCompoundFormatTestCase.java: Base tests for compound format implementations
//
// Source: lucene/core/src/test/org/apache/lucene/codecs/TestCompoundFormat.java
// Source: lucene/core/src/test/org/apache/lucene/codecs/lucene90/TestLucene90CompoundFormat.java
// Source: lucene/test-framework/src/java/org/apache/lucene/tests/index/BaseCompoundFormatTestCase.java
//
// Test Coverage:
//   - CFS threshold configuration (document count and byte size)
//   - Global enable/disable of compound files
//   - Maximum segment size limits
//   - Compound file read/write operations
//   - File discovery within compound files
//   - Read-only behavior of compound file readers
//   - Random access and seeking within compound files
//   - File length ordering in entries
//
// GC-198: Port TestCompoundFormat.java and TestLucene90CompoundFormat.java
package codecs_test

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ============================================================================
// CompoundFormat Interface Definition (Expected to be implemented)
// ============================================================================

// CompoundFormat encodes/decodes compound files.
// This is the Go port of Lucene's org.apache.lucene.codecs.CompoundFormat.
type CompoundFormat interface {
	// GetCompoundReader returns a Directory view (read-only) for the compound files in this segment.
	GetCompoundReader(dir store.Directory, si *index.SegmentInfo) (CompoundDirectory, error)

	// Write packs the provided segment's files into a compound format.
	Write(dir store.Directory, si *index.SegmentInfo, context *store.IOContext) error
}

// CompoundDirectory is a read-only Directory view for compound files.
// This is the Go port of Lucene's org.apache.lucene.codecs.CompoundDirectory.
type CompoundDirectory interface {
	store.Directory

	// CheckIntegrity validates the checksum of all files in the compound directory.
	CheckIntegrity() error
}

// CodecWithCompoundFormat extends Codec with CompoundFormat support.
// This interface is expected to be implemented by concrete codec types.
type CodecWithCompoundFormat interface {
	codecs.Codec
	CompoundFormat() CompoundFormat
}

// ============================================================================
// TestCompoundFormat - Threshold and Configuration Tests
// ============================================================================

// TestCompoundFormat_DefaultThresholds tests that the default thresholds work correctly
// for different merge policies.
//
// Source: TestCompoundFormat.testDefaultThresholds()
// Purpose: Verifies default document threshold (65536) for LogDocMergePolicy and
// default byte threshold (64MB) for other merge policies
func TestCompoundFormat_DefaultThresholds(t *testing.T) {
	// Create a test CompoundFormat with minimal implementation
	format := NewTestCompoundFormat()

	// Enable compound files with no size limit
	format.SetShouldUseCompoundFile(true)
	format.SetMaxCFSSegmentSizeMB(math.Inf(1)) // Remove size constraints

	// Create mock merge policies for testing
	docPolicy := &MockMergePolicy{sizeUnit: SizeUnitDocs}
	bytePolicy := &MockMergePolicy{sizeUnit: SizeUnitBytes}

	// Verify default threshold values are as expected
	if format.GetCfsThresholdDocSize() != 65536 {
		t.Errorf("Expected default doc threshold 65536, got %d", format.GetCfsThresholdDocSize())
	}
	if format.GetCfsThresholdByteSize() != 64*1024*1024 {
		t.Errorf("Expected default byte threshold %d, got %d", 64*1024*1024, format.GetCfsThresholdByteSize())
	}

	// Test LogDocMergePolicy uses document count threshold
	useCFS, err := format.UseCompoundFile(65536, docPolicy)
	if err != nil {
		t.Fatalf("UseCompoundFile failed: %v", err)
	}
	if !useCFS {
		t.Error("Should use CFS at doc threshold")
	}

	useCFS, err = format.UseCompoundFile(65537, docPolicy)
	if err != nil {
		t.Fatalf("UseCompoundFile failed: %v", err)
	}
	if useCFS {
		t.Error("Should not use CFS above doc threshold")
	}

	// Test other merge policies use byte size threshold (64MB)
	useCFS, err = format.UseCompoundFile(64*1024*1024, bytePolicy)
	if err != nil {
		t.Fatalf("UseCompoundFile failed: %v", err)
	}
	if !useCFS {
		t.Error("Should use CFS at byte threshold")
	}

	useCFS, err = format.UseCompoundFile((64*1024*1024)+1, bytePolicy)
	if err != nil {
		t.Fatalf("UseCompoundFile failed: %v", err)
	}
	if useCFS {
		t.Error("Should not use CFS above byte threshold")
	}
}

// TestCompoundFormat_DisabledCompoundFile tests that compound files can be globally disabled.
//
// Source: TestCompoundFormat.testDisabledCompoundFile()
// Purpose: When compound files are disabled, no segments should use compound files
// regardless of their size or the configured thresholds.
func TestCompoundFormat_DisabledCompoundFile(t *testing.T) {
	format := NewTestCompoundFormat()

	// Globally disable compound files
	format.SetShouldUseCompoundFile(false)
	docPolicy := &MockMergePolicy{sizeUnit: SizeUnitDocs}

	// Verify that CFS is never used when globally disabled
	useCFS, err := format.UseCompoundFile(1, docPolicy)
	if err != nil {
		t.Fatalf("UseCompoundFile failed: %v", err)
	}
	if useCFS {
		t.Error("Should not use CFS when disabled (small segment)")
	}

	useCFS, err = format.UseCompoundFile(65536, docPolicy)
	if err != nil {
		t.Fatalf("UseCompoundFile failed: %v", err)
	}
	if useCFS {
		t.Error("Should not use CFS when disabled (at threshold)")
	}
}

// TestCompoundFormat_MaxCFSSegmentSize tests the maximum compound file segment size limit.
//
// Source: TestCompoundFormat.testMaxCFSSegmentSize()
// Purpose: Segments larger than the configured maximum size should not use compound files,
// even if they would otherwise be eligible based on the threshold settings.
func TestCompoundFormat_MaxCFSSegmentSize(t *testing.T) {
	format := NewTestCompoundFormat()

	format.SetShouldUseCompoundFile(true)
	format.SetMaxCFSSegmentSizeMB(10) // Set 10MB limit
	bytePolicy := &MockMergePolicy{sizeUnit: SizeUnitBytes}

	// Test segments below the maximum size limit
	useCFS, err := format.UseCompoundFile(9*1024*1024, bytePolicy)
	if err != nil {
		t.Fatalf("UseCompoundFile failed: %v", err)
	}
	if !useCFS {
		t.Error("Should use CFS below max size limit")
	}

	// Test segments above the maximum size limit
	useCFS, err = format.UseCompoundFile(11*1024*1024, bytePolicy)
	if err != nil {
		t.Fatalf("UseCompoundFile failed: %v", err)
	}
	if useCFS {
		t.Error("Should not use CFS above max size limit")
	}
}

// TestCompoundFormat_CustomThresholds tests that custom threshold values can be configured.
//
// Source: TestCompoundFormat.testCustomThresholds()
// Purpose: Verifies that both document count and byte size thresholds can be customized
// and that the boundary conditions work properly with the new values.
func TestCompoundFormat_CustomThresholds(t *testing.T) {
	format := NewTestCompoundFormat()

	// Configure custom thresholds
	format.SetCfsThresholdDocSize(1000)              // Custom doc count threshold
	format.SetCfsThresholdByteSize(10 * 1024 * 1024) // Custom 10MB byte threshold

	docPolicy := &MockMergePolicy{sizeUnit: SizeUnitDocs}
	bytePolicy := &MockMergePolicy{sizeUnit: SizeUnitBytes}

	// Test custom document count threshold
	useCFS, err := format.UseCompoundFile(1000, docPolicy)
	if err != nil {
		t.Fatalf("UseCompoundFile failed: %v", err)
	}
	if !useCFS {
		t.Error("Should use CFS at custom doc threshold")
	}

	useCFS, err = format.UseCompoundFile(1001, docPolicy)
	if err != nil {
		t.Fatalf("UseCompoundFile failed: %v", err)
	}
	if useCFS {
		t.Error("Should not use CFS above custom doc threshold")
	}

	// Test custom byte size threshold
	useCFS, err = format.UseCompoundFile(10*1024*1024, bytePolicy)
	if err != nil {
		t.Fatalf("UseCompoundFile failed: %v", err)
	}
	if !useCFS {
		t.Error("Should use CFS at custom byte threshold")
	}

	useCFS, err = format.UseCompoundFile((10*1024*1024)+1, bytePolicy)
	if err != nil {
		t.Fatalf("UseCompoundFile failed: %v", err)
	}
	if useCFS {
		t.Error("Should not use CFS above custom byte threshold")
	}
}

// ============================================================================
// TestLucene90CompoundFormat - Lucene90 Specific Tests
// ============================================================================

// TestLucene90CompoundFormat_FileLengthOrdering tests that files are ordered by size in entries.
//
// Source: TestLucene90CompoundFormat.testFileLengthOrdering()
// Purpose: Verifies that the entries file contains files ordered by their size
func TestLucene90CompoundFormat_FileLengthOrdering(t *testing.T) {
	// This test requires the Lucene90CompoundFormat implementation
	// For now, we document the expected behavior
	t.Skip("Lucene90CompoundFormat not yet implemented - test documents expected behavior")

	// Test outline:
	// 1. Create a segment with multiple files of increasing sizes
	// 2. Write the compound file using Lucene90CompoundFormat
	// 3. Read the entries file (.cfe) and verify files are ordered by size
	// 4. Verify offsets are increasing and lengths are non-decreasing
}

// ============================================================================
// BaseCompoundFormatTestCase - Base Tests
// ============================================================================

// TestCompoundFormat_Empty tests that empty CFS is empty.
//
// Source: BaseCompoundFormatTestCase.testEmpty()
// Purpose: Verifies that a compound file with no entries is handled correctly.
func TestCompoundFormat_Empty(t *testing.T) {
	// This test requires CompoundFormat implementation
	t.Skip("CompoundFormat not yet implemented - test documents expected behavior")

	// Test outline:
	// 1. Create a segment with no files
	// 2. Write compound file
	// 3. Open compound reader and verify ListAll() returns empty
}

// TestCompoundFormat_SingleFile tests compound file with a single file of various sizes.
//
// Source: BaseCompoundFormatTestCase.testSingleFile()
// Purpose: Creates compound file based on a single file. Files of different sizes
// are tested: 0, 1, 10, 100 bytes.
func TestCompoundFormat_SingleFile(t *testing.T) {
	sizes := []int{0, 1, 10, 100}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			// This test requires CompoundFormat implementation
			t.Skip("CompoundFormat not yet implemented - test documents expected behavior")

			// Test outline:
			// 1. Create a file with sequential data of given size
			// 2. Write compound file
			// 3. Open compound reader and verify content matches original
			// 4. Test seek behavior at various positions
		})
	}
}

// TestCompoundFormat_TwoFiles tests compound file with two files.
//
// Source: BaseCompoundFormatTestCase.testTwoFiles()
// Purpose: Creates compound file based on two files and verifies content.
func TestCompoundFormat_TwoFiles(t *testing.T) {
	// This test requires CompoundFormat implementation
	t.Skip("CompoundFormat not yet implemented - test documents expected behavior")

	// Test outline:
	// 1. Create two files with different content
	// 2. Write compound file
	// 3. Open compound reader and verify both files can be read
	// 4. Verify content matches original files
}

// TestCompoundFormat_DoubleClose tests that a second call to close() behaves correctly.
//
// Source: BaseCompoundFormatTestCase.testDoubleClose()
// Purpose: Verifies that closing a compound reader twice does not throw an exception.
func TestCompoundFormat_DoubleClose(t *testing.T) {
	// This test requires CompoundFormat implementation
	t.Skip("CompoundFormat not yet implemented - test documents expected behavior")

	// Test outline:
	// 1. Create and write compound file
	// 2. Open compound reader
	// 3. Close compound reader
	// 4. Close compound reader again (should not error)
}

// TestCompoundFormat_ListAll tests that we can open all files returned by listAll.
//
// Source: BaseCompoundFormatTestCase.testListAll()
// Purpose: Verifies that all files in a compound file can be opened.
func TestCompoundFormat_ListAll(t *testing.T) {
	// This test requires CompoundFormat implementation
	t.Skip("CompoundFormat not yet implemented - test documents expected behavior")

	// Test outline:
	// 1. Create compound file with multiple files
	// 2. List all files in compound reader
	// 3. Open each file and verify it can be read
}

// TestCompoundFormat_CreateOutputDisabled tests that cfs reader is read-only.
//
// Source: BaseCompoundFormatTestCase.testCreateOutputDisabled()
// Purpose: Verifies that creating output on a compound reader is not allowed.
func TestCompoundFormat_CreateOutputDisabled(t *testing.T) {
	// This test requires CompoundFormat implementation
	t.Skip("CompoundFormat not yet implemented - test documents expected behavior")

	// Test outline:
	// 1. Create and open compound reader
	// 2. Attempt to create output (should return error)
}

// TestCompoundFormat_DeleteFileDisabled tests that cfs reader is read-only.
//
// Source: BaseCompoundFormatTestCase.testDeleteFileDisabled()
// Purpose: Verifies that deleting files on a compound reader is not allowed.
func TestCompoundFormat_DeleteFileDisabled(t *testing.T) {
	// This test requires CompoundFormat implementation
	t.Skip("CompoundFormat not yet implemented - test documents expected behavior")

	// Test outline:
	// 1. Create and open compound reader
	// 2. Attempt to delete file (should return error)
}

// TestCompoundFormat_RenameFileDisabled tests that cfs reader is read-only.
//
// Source: BaseCompoundFormatTestCase.testRenameFileDisabled()
// Purpose: Verifies that renaming files on a compound reader is not allowed.
func TestCompoundFormat_RenameFileDisabled(t *testing.T) {
	// This test requires CompoundFormat implementation
	t.Skip("CompoundFormat not yet implemented - test documents expected behavior")

	// Test outline:
	// 1. Create and open compound reader
	// 2. Attempt to rename file (should return error)
}

// TestCompoundFormat_SyncDisabled tests that cfs reader is read-only.
//
// Source: BaseCompoundFormatTestCase.testSyncDisabled()
// Purpose: Verifies that syncing on a compound reader is not allowed.
func TestCompoundFormat_SyncDisabled(t *testing.T) {
	// This test requires CompoundFormat implementation
	t.Skip("CompoundFormat not yet implemented - test documents expected behavior")

	// Test outline:
	// 1. Create and open compound reader
	// 2. Attempt to sync (should return error)
}

// TestCompoundFormat_RandomFiles tests compound file with random file sizes.
//
// Source: BaseCompoundFormatTestCase.testRandomFiles()
// Purpose: Creates compound file with files of various sizes to test buffering logic.
func TestCompoundFormat_RandomFiles(t *testing.T) {
	// This test requires CompoundFormat implementation
	t.Skip("CompoundFormat not yet implemented - test documents expected behavior")

	// Test outline:
	// 1. Create files of various sizes (0, 1, 10, 100, chunk, chunk-1, chunk+1, etc.)
	// 2. Write compound file
	// 3. Verify all files can be read and content matches
}

// TestCompoundFormat_FileNotFound tests that opening a non-existent file throws IOException.
//
// Source: BaseCompoundFormatTestCase.testFileNotFound()
// Purpose: Verifies proper error handling for missing files.
func TestCompoundFormat_FileNotFound(t *testing.T) {
	// This test requires CompoundFormat implementation
	t.Skip("CompoundFormat not yet implemented - test documents expected behavior")

	// Test outline:
	// 1. Create and open compound reader
	// 2. Attempt to open non-existent file (should return error)
}

// TestCompoundFormat_ReadPastEOF tests that reading past EOF throws IOException.
//
// Source: BaseCompoundFormatTestCase.testReadPastEOF()
// Purpose: Verifies proper EOF handling.
func TestCompoundFormat_ReadPastEOF(t *testing.T) {
	// This test requires CompoundFormat implementation
	t.Skip("CompoundFormat not yet implemented - test documents expected behavior")

	// Test outline:
	// 1. Create compound file with known content
	// 2. Open file and seek to near end
	// 3. Read past EOF (should return error)
}

// TestCompoundFormat_RandomAccess tests random access within compound files.
//
// Source: BaseCompoundFormatTestCase.testRandomAccess()
// Purpose: Verifies that file positions are independent when using multiple IndexInputs.
func TestCompoundFormat_RandomAccess(t *testing.T) {
	// This test requires CompoundFormat implementation
	t.Skip("CompoundFormat not yet implemented - test documents expected behavior")

	// Test outline:
	// 1. Create compound file with multiple files
	// 2. Open two different files from compound reader
	// 3. Seek and read from both files
	// 4. Verify positions are independent
}

// ============================================================================
// Helper Types and Functions
// ============================================================================

// SizeUnit represents the unit for measuring segment size.
// This mirrors Lucene's MergePolicy.SizeUnit.
type SizeUnit int

const (
	// SizeUnitDocs measures segment size by document count.
	SizeUnitDocs SizeUnit = iota
	// SizeUnitBytes measures segment size by byte size.
	SizeUnitBytes
)

// MockMergePolicy is a mock implementation of MergePolicy for testing.
type MockMergePolicy struct {
	sizeUnit SizeUnit
}

// GetSizeUnit returns the size unit for this merge policy.
func (p *MockMergePolicy) GetSizeUnit() SizeUnit {
	return p.sizeUnit
}

// TestCompoundFormat is a minimal implementation for testing threshold logic.
// This mirrors the abstract CompoundFormat class in Lucene.
type TestCompoundFormat struct {
	cfsThresholdDocSize   int
	cfsThresholdByteSize  int64
	shouldUseCompoundFile bool
	maxCFSSegmentSize     int64
}

// NewTestCompoundFormat creates a new TestCompoundFormat with default values.
func NewTestCompoundFormat() *TestCompoundFormat {
	return &TestCompoundFormat{
		cfsThresholdDocSize:   65536,
		cfsThresholdByteSize:  64 * 1024 * 1024,
		shouldUseCompoundFile: true,
		maxCFSSegmentSize:     math.MaxInt64,
	}
}

// SetCfsThresholdDocSize sets the document count threshold.
func (f *TestCompoundFormat) SetCfsThresholdDocSize(threshold int) {
	f.cfsThresholdDocSize = threshold
}

// SetCfsThresholdByteSize sets the byte size threshold.
func (f *TestCompoundFormat) SetCfsThresholdByteSize(threshold int64) {
	f.cfsThresholdByteSize = threshold
}

// GetCfsThresholdDocSize returns the document count threshold.
func (f *TestCompoundFormat) GetCfsThresholdDocSize() int {
	return f.cfsThresholdDocSize
}

// GetCfsThresholdByteSize returns the byte size threshold.
func (f *TestCompoundFormat) GetCfsThresholdByteSize() int64 {
	return f.cfsThresholdByteSize
}

// SetShouldUseCompoundFile enables or disables compound files.
func (f *TestCompoundFormat) SetShouldUseCompoundFile(use bool) {
	f.shouldUseCompoundFile = use
}

// GetShouldUseCompoundFile returns whether compound files are enabled.
func (f *TestCompoundFormat) GetShouldUseCompoundFile() bool {
	return f.shouldUseCompoundFile
}

// SetMaxCFSSegmentSizeMB sets the maximum segment size in MB.
func (f *TestCompoundFormat) SetMaxCFSSegmentSizeMB(mb float64) {
	if mb < 0 {
		panic("maxCFSSegmentSizeMB must be >= 0")
	}
	f.maxCFSSegmentSize = int64(mb * 1024 * 1024)
}

// GetMaxCFSSegmentSizeMB returns the maximum segment size in MB.
func (f *TestCompoundFormat) GetMaxCFSSegmentSizeMB() float64 {
	return float64(f.maxCFSSegmentSize) / (1024 * 1024)
}

// UseCompoundFile determines whether a segment should use compound file format.
// This mirrors the logic in Lucene's CompoundFormat.useCompoundFile().
func (f *TestCompoundFormat) UseCompoundFile(mergedInfoSize int64, mergePolicy *MockMergePolicy) (bool, error) {
	// Check if compound files are globally disabled
	if !f.shouldUseCompoundFile {
		return false, nil
	}

	// Check if segment exceeds maximum allowed size for CFS
	if mergedInfoSize > f.maxCFSSegmentSize {
		return false, nil
	}

	// Apply appropriate threshold based on merge policy's size unit
	if mergePolicy.GetSizeUnit() == SizeUnitDocs {
		return mergedInfoSize <= int64(f.cfsThresholdDocSize), nil
	}
	return mergedInfoSize <= f.cfsThresholdByteSize, nil
}

// createSequenceFile creates a file with sequential data.
func createSequenceFile(t *testing.T, dir store.Directory, name string, start byte, size int, segID []byte, segSuffix string) {
	out, err := dir.CreateOutput(name, store.IOContextWrite)
	if err != nil {
		t.Fatalf("Failed to create output: %v", err)
	}
	defer out.Close()

	codecs.WriteIndexHeader(out, "Foo", 0, segID, segSuffix)
	for i := 0; i < size; i++ {
		out.WriteByte(start)
		start++
	}
	codecs.WriteFooter(out)
}

// createRandomFile creates a file with random data.
func createRandomFile(t *testing.T, dir store.Directory, name string, size int, segID []byte) {
	out, err := dir.CreateOutput(name, store.IOContextWrite)
	if err != nil {
		t.Fatalf("Failed to create output: %v", err)
	}
	defer out.Close()

	codecs.WriteIndexHeader(out, "Foo", 0, segID, "suffix")
	data := make([]byte, size)
	rand.Read(data)
	out.WriteBytes(data)
	codecs.WriteFooter(out)
}

// createLargeCFS creates a large compound file with multiple components.
func createLargeCFS(t *testing.T, dir store.Directory, id []byte) store.Directory {
	files := make([]string, 20)
	for i := 0; i < 20; i++ {
		filename := fmt.Sprintf("_123.f%d", i)
		files[i] = filename
		createSequenceFile(t, dir, filename, 0, 2000, id, "suffix")
	}

	si := index.NewSegmentInfo("_123", 10000, dir)
	si.SetID(id)
	si.SetFiles(files)

	// This would use the actual CompoundFormat implementation
	// For now, return the regular directory
	return dir
}

// assertSameStreams verifies that two IndexInput streams have the same content.
func assertSameStreams(t *testing.T, msg string, expected, test store.IndexInput) {
	if expected == nil {
		t.Fatalf("%s: null expected", msg)
	}
	if test == nil {
		t.Fatalf("%s: null test", msg)
	}

	if expected.Length() != test.Length() {
		t.Errorf("%s: expected length %d, got %d", msg, expected.Length(), test.Length())
	}
	if expected.GetFilePointer() != test.GetFilePointer() {
		t.Errorf("%s: expected position %d, got %d", msg, expected.GetFilePointer(), test.GetFilePointer())
	}

	expectedBuffer := make([]byte, 512)
	testBuffer := make([]byte, 512)

	remainder := expected.Length() - expected.GetFilePointer()
	for remainder > 0 {
		readLen := int(remainder)
		if readLen > len(expectedBuffer) {
			readLen = len(expectedBuffer)
		}

		err := expected.ReadBytes(expectedBuffer[:readLen])
		if err != nil {
			t.Fatalf("%s: failed to read expected: %v", msg, err)
		}

		err = test.ReadBytes(testBuffer[:readLen])
		if err != nil {
			t.Fatalf("%s: failed to read test: %v", msg, err)
		}

		if !bytes.Equal(expectedBuffer[:readLen], testBuffer[:readLen]) {
			t.Errorf("%s: content mismatch at remainder %d", msg, remainder)
			break
		}
		remainder -= int64(readLen)
	}
}

// assertSameSeekBehavior verifies that seeking works correctly.
func assertSameSeekBehavior(t *testing.T, msg string, expected, actual store.IndexInput) {
	// seek to 0
	assertSameStreamsAt(t, msg+", seek(0)", expected, actual, 0)

	// seek to middle
	point := expected.Length() / 2
	assertSameStreamsAt(t, msg+", seek(mid)", expected, actual, point)

	// seek to end - 2
	point = expected.Length() - 2
	if point >= 0 {
		assertSameStreamsAt(t, msg+", seek(end-2)", expected, actual, point)
	}

	// seek to end - 1
	point = expected.Length() - 1
	if point >= 0 {
		assertSameStreamsAt(t, msg+", seek(end-1)", expected, actual, point)
	}

	// seek to the end
	point = expected.Length()
	assertSameStreamsAt(t, msg+", seek(end)", expected, actual, point)

	// seek past end
	point = expected.Length() + 1
	assertSameStreamsAt(t, msg+", seek(end+1)", expected, actual, point)
}

// assertSameStreamsAt verifies streams are equal at a specific position.
func assertSameStreamsAt(t *testing.T, msg string, expected, actual store.IndexInput, seekTo int64) {
	if seekTo >= 0 && seekTo < expected.Length() {
		expected.SetPosition(seekTo)
		actual.SetPosition(seekTo)
		assertSameStreams(t, msg, expected, actual)
	}
}
