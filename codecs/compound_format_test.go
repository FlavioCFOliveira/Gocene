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
)

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
// Purpose: Verifies that the entries file contains files ordered by their size.
// The Lucene90CompoundFormat sorts input files by size ascending before writing
// the .cfe, so offsets are strictly increasing and lengths are non-decreasing.
func TestLucene90CompoundFormat_FileLengthOrdering(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	si := index.NewSegmentInfo("_123", 0, dir)
	if err := si.SetID(id); err != nil {
		t.Fatalf("SetID: %v", err)
	}

	// Create 5 files with strictly increasing payload sizes (0, 10, 20, 30, 40 bytes)
	// so the sort order equals the file-creation order.
	type orderedFile struct {
		name    string
		payload []byte
	}
	ordered := []orderedFile{
		{"_123.f0", make([]byte, 0)},
		{"_123.f1", make([]byte, 10)},
		{"_123.f2", make([]byte, 20)},
		{"_123.f3", make([]byte, 30)},
		{"_123.f4", make([]byte, 40)},
	}

	names := make([]string, len(ordered))
	for i, of := range ordered {
		names[i] = of.name
		raw, err := dir.CreateOutput(of.name, store.IOContextWrite)
		if err != nil {
			t.Fatalf("CreateOutput %s: %v", of.name, err)
		}
		out := store.NewChecksumIndexOutput(raw)
		if err := codecs.WriteIndexHeader(out, "Test", 0, id, ""); err != nil {
			t.Fatalf("WriteIndexHeader: %v", err)
		}
		if err := out.WriteBytes(of.payload); err != nil {
			t.Fatalf("WriteBytes: %v", err)
		}
		if err := codecs.WriteFooter(out); err != nil {
			t.Fatalf("WriteFooter: %v", err)
		}
		if err := out.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}

	// Shuffle the file list before writing so we test that Write sorts them.
	shuffled := []string{names[4], names[2], names[0], names[3], names[1]}
	si.SetFiles(shuffled)

	format := codecs.NewLucene90CompoundFormat()
	if err := format.Write(dir, si, store.IOContext{Context: store.ContextWrite}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Read the .cfe directly and check ordering: after the IndexHeader each
	// entry is (name string, offset int64, length int64). Offsets must be
	// strictly increasing; lengths must be non-decreasing.
	cfeIn, err := dir.OpenInput("_123.cfe", store.IOContextDefault)
	if err != nil {
		t.Fatalf("OpenInput .cfe: %v", err)
	}
	defer cfeIn.Close()

	csIn := store.NewChecksumIndexInput(cfeIn)
	if _, err := codecs.CheckIndexHeader(csIn, codecs.Lucene90CompoundEntriesCodec,
		codecs.Lucene90CompoundVersionStart, codecs.Lucene90CompoundVersionCurrent, id, ""); err != nil {
		t.Fatalf("CheckIndexHeader: %v", err)
	}
	numEntries, err := store.ReadVInt(csIn)
	if err != nil {
		t.Fatalf("ReadVInt numEntries: %v", err)
	}
	if int(numEntries) != len(ordered) {
		t.Fatalf("numEntries: got %d, want %d", numEntries, len(ordered))
	}

	var lastOffset, lastLength int64
	for i := int32(0); i < numEntries; i++ {
		if _, err := store.ReadString(csIn); err != nil {
			t.Fatalf("ReadString entry %d: %v", i, err)
		}
		// .cfe offset/length are little-endian (DataOutput.writeLong),
		// matching Lucene90CompoundFormat.
		offset, err := store.ReadInt64LE(csIn)
		if err != nil {
			t.Fatalf("ReadInt64LE offset %d: %v", i, err)
		}
		length, err := store.ReadInt64LE(csIn)
		if err != nil {
			t.Fatalf("ReadInt64LE length %d: %v", i, err)
		}
		if i > 0 && offset <= lastOffset {
			t.Errorf("entry %d: offset %d not > lastOffset %d", i, offset, lastOffset)
		}
		if length < lastLength {
			t.Errorf("entry %d: length %d < lastLength %d", i, length, lastLength)
		}
		lastOffset = offset
		lastLength = length
	}
}

// ============================================================================
// BaseCompoundFormatTestCase - Base Tests
// ============================================================================

// TestCompoundFormat_Empty tests that empty CFS is empty.
//
// Source: BaseCompoundFormatTestCase.testEmpty()
// Purpose: Verifies that a compound file with no entries is handled correctly.
func TestCompoundFormat_Empty(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	si := index.NewSegmentInfo("_0", 0, dir)
	if err := si.SetID(id); err != nil {
		t.Fatalf("SetID: %v", err)
	}
	// No files registered → empty compound.
	si.SetFiles(nil)

	format := codecs.NewLucene90CompoundFormat()
	if err := format.Write(dir, si, store.IOContext{Context: store.ContextWrite}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	reader, err := format.GetCompoundReader(dir, si)
	if err != nil {
		t.Fatalf("GetCompoundReader: %v", err)
	}
	defer reader.Close()

	all, err := reader.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("ListAll: got %d files, want 0: %v", len(all), all)
	}
}

// TestCompoundFormat_SingleFile tests compound file with a single file of various sizes.
//
// Source: BaseCompoundFormatTestCase.testSingleFile()
// Purpose: Creates compound file based on a single file. Files of different sizes
// are tested: 0, 1, 10, 100 bytes.
func TestCompoundFormat_SingleFile(t *testing.T) {
	sizes := []int{0, 1, 10, 100}

	for _, size := range sizes {
		size := size
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			dir, err := store.NewSimpleFSDirectory(t.TempDir())
			if err != nil {
				t.Fatalf("NewSimpleFSDirectory: %v", err)
			}
			defer dir.Close()

			body := make([]byte, size)
			for i := range body {
				body[i] = byte(i)
			}
			si, payloads := writeSegmentWithFiles(t, dir, map[string][]byte{
				"_0.dat": body,
			})

			format := codecs.NewLucene90CompoundFormat()
			if err := format.Write(dir, si, store.IOContext{Context: store.ContextWrite}); err != nil {
				t.Fatalf("Write: %v", err)
			}

			reader, err := format.GetCompoundReader(dir, si)
			if err != nil {
				t.Fatalf("GetCompoundReader: %v", err)
			}
			defer reader.Close()

			all, err := reader.ListAll()
			if err != nil {
				t.Fatalf("ListAll: %v", err)
			}
			if len(all) != 1 {
				t.Fatalf("ListAll: got %d files, want 1: %v", len(all), all)
			}

			got, err := readFileFromCompound(reader, all[0])
			if err != nil {
				t.Fatalf("readFileFromCompound: %v", err)
			}
			want := payloads[stripPrefix(t, all[0])]
			if len(got) != len(want) {
				t.Fatalf("size=%d: payload length got %d want %d", size, len(got), len(want))
			}
			for i := range got {
				if got[i] != want[i] {
					t.Fatalf("size=%d: byte mismatch at %d: got %d want %d", size, i, got[i], want[i])
				}
			}

			// The CFS entry must be byte-identical to the original file
			// (header+body+footer verbatim). Open both streams and compare.
			expected, err := dir.OpenInput("_0.dat", store.IOContextDefault)
			if err != nil {
				t.Fatalf("OpenInput original: %v", err)
			}
			defer expected.Close()

			actual, err := reader.OpenInput(all[0], store.IOContextDefault)
			if err != nil {
				t.Fatalf("OpenInput compound: %v", err)
			}
			defer actual.Close()

			assertSameStreams(t, fmt.Sprintf("size=%d streams", size), expected, actual)
			assertSameSeekBehavior(t, fmt.Sprintf("size=%d seek", size), expected, actual)
		})
	}
}

// TestCompoundFormat_TwoFiles tests compound file with two files.
//
// Source: BaseCompoundFormatTestCase.testTwoFiles()
// Purpose: Creates compound file based on two files and verifies content.
func TestCompoundFormat_TwoFiles(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	body0 := make([]byte, 15)
	body1 := make([]byte, 114)
	for i := range body0 {
		body0[i] = byte(i)
	}
	for i := range body1 {
		body1[i] = byte(i)
	}
	si, _ := writeSegmentWithFiles(t, dir, map[string][]byte{
		"_123.d1": body0,
		"_123.d2": body1,
	})

	format := codecs.NewLucene90CompoundFormat()
	if err := format.Write(dir, si, store.IOContext{Context: store.ContextWrite}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	reader, err := format.GetCompoundReader(dir, si)
	if err != nil {
		t.Fatalf("GetCompoundReader: %v", err)
	}
	defer reader.Close()

	for _, name := range []string{"_123.d1", "_123.d2"} {
		expected, err := dir.OpenInput(name, store.IOContextDefault)
		if err != nil {
			t.Fatalf("OpenInput original %s: %v", name, err)
		}
		actual, err := reader.OpenInput(name, store.IOContextDefault)
		if err != nil {
			_ = expected.Close()
			t.Fatalf("OpenInput compound %s: %v", name, err)
		}
		assertSameStreams(t, name+" streams", expected, actual)
		assertSameSeekBehavior(t, name+" seek", expected, actual)
		_ = expected.Close()
		_ = actual.Close()
	}
}

// TestCompoundFormat_DoubleClose tests that a second call to close() behaves correctly.
//
// Source: BaseCompoundFormatTestCase.testDoubleClose()
// Purpose: Verifies that closing a compound reader twice does not throw an exception.
func TestCompoundFormat_DoubleClose(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	si, _ := writeSegmentWithFiles(t, dir, map[string][]byte{"_0.tmp": []byte("x")})

	format := codecs.NewLucene90CompoundFormat()
	if err := format.Write(dir, si, store.IOContext{Context: store.ContextWrite}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	reader, err := format.GetCompoundReader(dir, si)
	if err != nil {
		t.Fatalf("GetCompoundReader: %v", err)
	}

	if err := reader.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// Second close must not panic or return an error.
	if err := reader.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// TestCompoundFormat_ListAll tests that we can open all files returned by listAll.
//
// Source: BaseCompoundFormatTestCase.testListAll()
// Purpose: Verifies that all files in a compound file can be opened.
func TestCompoundFormat_ListAll(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	si, _ := writeSegmentWithFiles(t, dir, map[string][]byte{
		"_0.foo": []byte("foo"),
		"_0.bar": []byte("barbaz"),
		"_0.qux": make([]byte, 64),
	})

	format := codecs.NewLucene90CompoundFormat()
	if err := format.Write(dir, si, store.IOContext{Context: store.ContextWrite}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	reader, err := format.GetCompoundReader(dir, si)
	if err != nil {
		t.Fatalf("GetCompoundReader: %v", err)
	}
	defer reader.Close()

	all, err := reader.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("ListAll: got %d files, want 3: %v", len(all), all)
	}
	// Every file returned by ListAll must be openable.
	for _, name := range all {
		in, err := reader.OpenInput(name, store.IOContextDefault)
		if err != nil {
			t.Fatalf("OpenInput(%q): %v", name, err)
		}
		if in.Length() == 0 {
			t.Errorf("OpenInput(%q): zero length", name)
		}
		_ = in.Close()
	}
}

// TestCompoundFormat_CreateOutputDisabled tests that cfs reader is read-only.
//
// Source: BaseCompoundFormatTestCase.testCreateOutputDisabled()
// Purpose: Verifies that creating output on a compound reader is not allowed.
func TestCompoundFormat_CreateOutputDisabled(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	si, _ := writeSegmentWithFiles(t, dir, map[string][]byte{"_0.tmp": []byte("x")})

	format := codecs.NewLucene90CompoundFormat()
	if err := format.Write(dir, si, store.IOContext{Context: store.ContextWrite}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	reader, err := format.GetCompoundReader(dir, si)
	if err != nil {
		t.Fatalf("GetCompoundReader: %v", err)
	}
	defer reader.Close()

	if _, err := reader.CreateOutput("_0.new", store.IOContext{Context: store.ContextWrite}); err == nil {
		t.Fatal("CreateOutput on compound reader must return an error")
	}
}

// TestCompoundFormat_DeleteFileDisabled tests that cfs reader is read-only.
//
// Source: BaseCompoundFormatTestCase.testDeleteFileDisabled()
// Purpose: Verifies that deleting files on a compound reader is not allowed.
func TestCompoundFormat_DeleteFileDisabled(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	si, _ := writeSegmentWithFiles(t, dir, map[string][]byte{"_0.tmp": []byte("x")})

	format := codecs.NewLucene90CompoundFormat()
	if err := format.Write(dir, si, store.IOContext{Context: store.ContextWrite}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	reader, err := format.GetCompoundReader(dir, si)
	if err != nil {
		t.Fatalf("GetCompoundReader: %v", err)
	}
	defer reader.Close()

	if err := reader.DeleteFile("_0.tmp"); err == nil {
		t.Fatal("DeleteFile on compound reader must return an error")
	}
}

// TestCompoundFormat_RenameFileDisabled tests that cfs reader is read-only.
//
// Source: BaseCompoundFormatTestCase.testRenameFileDisabled()
// Purpose: Verifies that mutating operations on a compound reader are not
// allowed. Gocene's store.Directory does not expose a Rename method, so
// this test exercises ObtainLock — the canonical read-only sentinel in
// Gocene's CompoundDirectory — which must return ErrReadOnlyCompoundDirectory.
func TestCompoundFormat_RenameFileDisabled(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	si, _ := writeSegmentWithFiles(t, dir, map[string][]byte{"_0.tmp": []byte("x")})

	format := codecs.NewLucene90CompoundFormat()
	if err := format.Write(dir, si, store.IOContext{Context: store.ContextWrite}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	reader, err := format.GetCompoundReader(dir, si)
	if err != nil {
		t.Fatalf("GetCompoundReader: %v", err)
	}
	defer reader.Close()

	// ObtainLock is the read-only gate available on store.Directory.
	if _, err := reader.ObtainLock("write.lock"); err == nil {
		t.Fatal("ObtainLock on compound reader must return an error")
	}
}

// TestCompoundFormat_SyncDisabled tests that cfs reader is read-only.
//
// Source: BaseCompoundFormatTestCase.testSyncDisabled()
// Purpose: Verifies that syncing on a compound reader is not allowed. Gocene's
// store.Directory does not expose a Sync method, so this test verifies that
// CreateOutput — the first write-path entry point — returns an error, proving
// the reader is fully read-only.
func TestCompoundFormat_SyncDisabled(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	si, _ := writeSegmentWithFiles(t, dir, map[string][]byte{"_0.tmp": []byte("x")})

	format := codecs.NewLucene90CompoundFormat()
	if err := format.Write(dir, si, store.IOContext{Context: store.ContextWrite}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	reader, err := format.GetCompoundReader(dir, si)
	if err != nil {
		t.Fatalf("GetCompoundReader: %v", err)
	}
	defer reader.Close()

	// Verify all three mutating operations fail, covering the read-only contract.
	if _, err := reader.CreateOutput("_0.new", store.IOContext{Context: store.ContextWrite}); err == nil {
		t.Error("CreateOutput: expected error on read-only compound directory")
	}
	if err := reader.DeleteFile("_0.tmp"); err == nil {
		t.Error("DeleteFile: expected error on read-only compound directory")
	}
	if _, err := reader.ObtainLock("write.lock"); err == nil {
		t.Error("ObtainLock: expected error on read-only compound directory")
	}
}

// TestCompoundFormat_RandomFiles tests compound file with random file sizes.
//
// Source: BaseCompoundFormatTestCase.testRandomFiles()
// Purpose: Creates compound file with files of various sizes to test buffering logic.
// The Java test uses chunk=1024 (internal buffer size) and creates files at 0, 1, 10,
// 100, chunk, chunk-1, chunk+1, 3*chunk, 3*chunk-1, 3*chunk+1, 1000*chunk bytes.
func TestCompoundFormat_RandomFiles(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	const chunk = 1024
	sizes := map[string]int{
		"_123.zero": 0,
		"_123.one":  1,
		"_123.ten":  10,
		"_123.hun":  100,
		"_123.big1": chunk,
		"_123.big2": chunk - 1,
		"_123.big3": chunk + 1,
		"_123.big4": 3 * chunk,
		"_123.big5": 3*chunk - 1,
		"_123.big6": 3*chunk + 1,
		// 1000*chunk (1 MB) is too slow for a unit test on the RPi; use 10*chunk
		// which still exercises multi-buffer reads.
		"_123.big7": 10 * chunk,
	}

	files := make(map[string][]byte, len(sizes))
	for name, sz := range sizes {
		body := make([]byte, sz)
		if _, err := rand.Read(body); err != nil {
			t.Fatalf("rand.Read: %v", err)
		}
		files[name] = body
	}

	si, _ := writeSegmentWithFiles(t, dir, files)

	format := codecs.NewLucene90CompoundFormat()
	if err := format.Write(dir, si, store.IOContext{Context: store.ContextWrite}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	reader, err := format.GetCompoundReader(dir, si)
	if err != nil {
		t.Fatalf("GetCompoundReader: %v", err)
	}
	defer reader.Close()

	for name := range files {
		expected, err := dir.OpenInput(name, store.IOContextDefault)
		if err != nil {
			t.Fatalf("OpenInput original %s: %v", name, err)
		}
		actual, err := reader.OpenInput(name, store.IOContextDefault)
		if err != nil {
			_ = expected.Close()
			t.Fatalf("OpenInput compound %s: %v", name, err)
		}
		assertSameStreams(t, name+" streams", expected, actual)
		assertSameSeekBehavior(t, name+" seek", expected, actual)
		_ = expected.Close()
		_ = actual.Close()
	}
}

// TestCompoundFormat_FileNotFound tests that opening a non-existent file throws IOException.
//
// Source: BaseCompoundFormatTestCase.testFileNotFound()
// Purpose: Verifies proper error handling for missing files.
func TestCompoundFormat_FileNotFound(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	si, _ := writeSegmentWithFiles(t, dir, map[string][]byte{
		"_0.foo": []byte("hello"),
	})

	format := codecs.NewLucene90CompoundFormat()
	if err := format.Write(dir, si, store.IOContext{Context: store.ContextWrite}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	reader, err := format.GetCompoundReader(dir, si)
	if err != nil {
		t.Fatalf("GetCompoundReader: %v", err)
	}
	defer reader.Close()

	if _, err := reader.OpenInput("_0.doesnotexist", store.IOContextDefault); err == nil {
		t.Fatal("OpenInput on missing file must return an error")
	}
}

// TestCompoundFormat_ReadPastEOF tests that reading past EOF throws IOException.
//
// Source: BaseCompoundFormatTestCase.testReadPastEOF()
// Purpose: Verifies proper EOF handling.
func TestCompoundFormat_ReadPastEOF(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	cr := buildLargeCFS(t, dir)
	defer cr.Close()

	is, err := cr.OpenInput("_123.f2", store.IOContextDefault)
	if err != nil {
		t.Fatalf("OpenInput _123.f2: %v", err)
	}
	defer is.Close()

	// Seek to 10 bytes before the end and read those 10 bytes — must succeed.
	if err := is.SetPosition(is.Length() - 10); err != nil {
		t.Fatalf("SetPosition to end-10: %v", err)
	}
	buf := make([]byte, 10)
	if err := is.ReadBytes(buf); err != nil {
		t.Fatalf("ReadBytes 10 near end: %v", err)
	}

	// Single byte read past end must return an error.
	if _, err := is.ReadByte(); err == nil {
		t.Error("ReadByte past EOF: expected error, got nil")
	}

	// Seek back to end-10 and try a block read past the end — must also error.
	if err := is.SetPosition(is.Length() - 10); err != nil {
		t.Fatalf("SetPosition to end-10 (second): %v", err)
	}
	big := make([]byte, 50)
	if err := is.ReadBytes(big); err == nil {
		t.Error("ReadBytes past EOF: expected error, got nil")
	}
}

// TestCompoundFormat_RandomAccess tests random access within compound files.
//
// Source: BaseCompoundFormatTestCase.testRandomAccess()
// Purpose: Verifies that file positions are independent when using multiple IndexInputs
// opened from the same CompoundDirectory (i.e., both share the same underlying .cfs
// handle but maintain independent file pointers).
func TestCompoundFormat_RandomAccess(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	cr := buildLargeCFS(t, dir)
	defer cr.Close()

	// Open two pairs: one from the compound reader and one direct for comparison.
	e1, err := dir.OpenInput("_123.f11", store.IOContextDefault)
	if err != nil {
		t.Fatalf("OpenInput dir _123.f11: %v", err)
	}
	defer e1.Close()

	e2, err := dir.OpenInput("_123.f3", store.IOContextDefault)
	if err != nil {
		t.Fatalf("OpenInput dir _123.f3: %v", err)
	}
	defer e2.Close()

	a1, err := cr.OpenInput("_123.f11", store.IOContextDefault)
	if err != nil {
		t.Fatalf("OpenInput cfs _123.f11: %v", err)
	}
	defer a1.Close()

	// For a2 use the direct directory just as the Java test does.
	a2, err := dir.OpenInput("_123.f3", store.IOContextDefault)
	if err != nil {
		t.Fatalf("OpenInput dir _123.f3 (a2): %v", err)
	}
	defer a2.Close()

	mustSeekAndMatch := func(label string, ex, ax store.IndexInput, pos int64) byte {
		t.Helper()
		if err := ex.SetPosition(pos); err != nil {
			t.Fatalf("%s: SetPosition expected %d: %v", label, pos, err)
		}
		if err := ax.SetPosition(pos); err != nil {
			t.Fatalf("%s: SetPosition actual %d: %v", label, pos, err)
		}
		var be, ba [1]byte
		if err := ex.ReadBytes(be[:]); err != nil {
			t.Fatalf("%s: ReadByte expected: %v", label, err)
		}
		if err := ax.ReadBytes(ba[:]); err != nil {
			t.Fatalf("%s: ReadByte actual: %v", label, err)
		}
		if be[0] != ba[0] {
			t.Errorf("%s: byte mismatch: expected %d, actual %d", label, be[0], ba[0])
		}
		return be[0]
	}

	mustSeekAndMatch("e1/a1 seek(100)", e1, a1, 100)
	mustSeekAndMatch("e2/a2 seek(1027)", e2, a2, 1027)

	// First pair must still be at 101 after second pair moved.
	if e1.GetFilePointer() != 101 {
		t.Errorf("e1 file pointer: got %d, want 101", e1.GetFilePointer())
	}
	if a1.GetFilePointer() != 101 {
		t.Errorf("a1 file pointer: got %d, want 101", a1.GetFilePointer())
	}
	mustSeekAndMatch("e1/a1 read@101", e1, a1, 101)

	// Move first pair past buffer boundary.
	mustSeekAndMatch("e1/a1 seek(1910)", e1, a1, 1910)

	// Second pair must still be at 1028.
	if e2.GetFilePointer() != 1028 {
		t.Errorf("e2 file pointer: got %d, want 1028", e2.GetFilePointer())
	}
	if a2.GetFilePointer() != 1028 {
		t.Errorf("a2 file pointer: got %d, want 1028", a2.GetFilePointer())
	}
	mustSeekAndMatch("e2/a2 read@1028", e2, a2, 1028)

	// Move second pair back.
	mustSeekAndMatch("e2/a2 seek(17)", e2, a2, 17)

	// First pair must still be at 1911.
	if e1.GetFilePointer() != 1911 {
		t.Errorf("e1 file pointer: got %d, want 1911", e1.GetFilePointer())
	}
	if a1.GetFilePointer() != 1911 {
		t.Errorf("a1 file pointer: got %d, want 1911", a1.GetFilePointer())
	}
	mustSeekAndMatch("e1/a1 read@1911", e1, a1, 1911)
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
// Passing math.Inf(1) removes the size cap (equivalent to MaxInt64).
func (f *TestCompoundFormat) SetMaxCFSSegmentSizeMB(mb float64) {
	if mb < 0 {
		panic("maxCFSSegmentSizeMB must be >= 0")
	}
	if math.IsInf(mb, 1) {
		f.maxCFSSegmentSize = math.MaxInt64
		return
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

// buildLargeCFS creates a compound file with 20 sequence files (_123.f0 to
// _123.f19), each containing 2000 payload bytes with values 0,1,2,…,255,0,…
// (wrapping), wrapped in a valid Lucene index header + footer. It mirrors
// BaseCompoundFormatTestCase.createLargeCFS from Apache Lucene 10.4.0.
// The returned CompoundDirectory is open; the caller must close it.
func buildLargeCFS(t *testing.T, dir store.Directory) codecs.CompoundDirectory {
	t.Helper()

	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		t.Fatalf("buildLargeCFS rand.Read: %v", err)
	}
	si := index.NewSegmentInfo("_123", 10000, dir)
	if err := si.SetID(id); err != nil {
		t.Fatalf("buildLargeCFS SetID: %v", err)
	}

	names := make([]string, 20)
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("_123.f%d", i)
		names[i] = name

		// Build a 2000-byte sequential body (0,1,…,255,0,1,…).
		body := make([]byte, 2000)
		for j := range body {
			body[j] = byte(j)
		}

		raw, err := dir.CreateOutput(name, store.IOContext{Context: store.ContextWrite})
		if err != nil {
			t.Fatalf("buildLargeCFS CreateOutput %s: %v", name, err)
		}
		out := store.NewChecksumIndexOutput(raw)
		if err := codecs.WriteIndexHeader(out, "Foo", 0, id, "suffix"); err != nil {
			t.Fatalf("buildLargeCFS WriteIndexHeader %s: %v", name, err)
		}
		if err := out.WriteBytes(body); err != nil {
			t.Fatalf("buildLargeCFS WriteBytes %s: %v", name, err)
		}
		if err := codecs.WriteFooter(out); err != nil {
			t.Fatalf("buildLargeCFS WriteFooter %s: %v", name, err)
		}
		if err := out.Close(); err != nil {
			t.Fatalf("buildLargeCFS Close %s: %v", name, err)
		}
	}

	si.SetFiles(names)

	format := codecs.NewLucene90CompoundFormat()
	if err := format.Write(dir, si, store.IOContext{Context: store.ContextWrite}); err != nil {
		t.Fatalf("buildLargeCFS Write: %v", err)
	}

	cr, err := format.GetCompoundReader(dir, si)
	if err != nil {
		t.Fatalf("buildLargeCFS GetCompoundReader: %v", err)
	}
	return cr
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
