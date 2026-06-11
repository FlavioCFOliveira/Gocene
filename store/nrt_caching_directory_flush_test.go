// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"testing"
)

// TestNRTCachingDirectory_FlushNonAlignedFileSize verifies that the unCache
// method correctly writes the final partial chunk for files whose size is
// not a multiple of the 8192-byte copy buffer. Before the fix, the last
// partial chunk was silently dropped because the read loop broke on EOF
// without writing the bytes that were previously read into the buffer.
func TestNRTCachingDirectory_FlushNonAlignedFileSize(t *testing.T) {
	sizes := []int64{
		1,       // single byte
		8191,    // one less than buffer size
		8192,    // exactly buffer size (aligned)
		8193,    // one more than buffer size
		16383,   // just under 2x buffer
		16384,   // exactly 2x buffer
		16385,   // just over 2x buffer
		100,     // arbitrary small
		5000,    // arbitrary medium
		20000,   // arbitrary larger
	}

	for _, size := range sizes {
		t.Run("", func(t *testing.T) {
			testNRTCachingFlushSize(t, size)
		})
	}
}

// testNRTCachingFlushSize verifies byte-identical round-trip of a file
// flushed from NRTCachingDirectory cache to its delegate.
func testNRTCachingFlushSize(t *testing.T, size int64) {
	t.Helper()

	delegateDir := NewByteBuffersDirectory()
	// maxMergeSizeMB=100 ensures all our test files go through the cache.
	nrtDir := NewNRTCachingDirectory(delegateDir, 100.0, 100.0)

	// Generate deterministic content.
	content := make([]byte, size)
	for i := int64(0); i < size; i++ {
		content[i] = byte((i * 17) ^ 0x55)
	}

	// Write file through NRT directory (goes to cache).
	// Use a flush context so doCacheWrite returns true (plain IOContextDefault
	// skips the cache because it has no FlushInfo/MergeInfo).
	flushCtx := IOContext{
		Context: ContextFlush,
		FlushInfo: &FlushInfo{
			NumDocs:             1,
			EstimatedSegmentSize: size,
		},
	}
	out, err := nrtDir.CreateOutput("testfile", flushCtx)
	if err != nil {
		nrtDir.Close()
		t.Fatalf("CreateOutput failed: %v", err)
	}
	if err := out.WriteBytes(content); err != nil {
		out.Close()
		nrtDir.Close()
		t.Fatalf("WriteBytes failed: %v", err)
	}
	if err := out.Close(); err != nil {
		nrtDir.Close()
		t.Fatalf("Close output failed: %v", err)
	}

	// Verify file is currently in the cache, not in delegate.
	if !nrtDir.cacheDir.FileExists("testfile") {
		nrtDir.Close()
		t.Fatal("File should be in cache before flush")
	}

	// Use Rename to trigger unCache (rename from cache to delegate).
	// Rename calls unCache(source) then delegate.Rename(source, dest).
	// We rename "testfile" -> "testfile_flushed" to avoid "already exists".
	if err := nrtDir.Rename("testfile", "testfile_flushed"); err != nil {
		nrtDir.Close()
		t.Fatalf("Rename (unCache trigger) failed: %v", err)
	}

	// Now read back from delegate and verify.
	readBack, err := readFileContent(delegateDir, "testfile_flushed", size)
	nrtDir.Close()
	if err != nil {
		t.Fatalf("Read back failed: %v", err)
	}

	if int64(len(readBack)) != size {
		t.Fatalf("Length mismatch: expected %d, got %d. Last partial chunk was likely dropped during unCache.",
			size, len(readBack))
	}

	for i := int64(0); i < size; i++ {
		if readBack[i] != content[i] {
			t.Fatalf("Byte mismatch at offset %d: expected 0x%02x, got 0x%02x",
				i, content[i], readBack[i])
		}
	}
}

// readFileContent opens a file from the given directory, reads exactly
// expectedSize bytes, and returns them.
func readFileContent(dir Directory, name string, expectedSize int64) ([]byte, error) {
	in, err := dir.OpenInput(name, IOContextRead)
	if err != nil {
		return nil, err
	}
	defer in.Close()
	return in.ReadBytesN(int(expectedSize))
}
