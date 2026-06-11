// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test verifies that the index detects files with mismatched
// checksums in the codec footer.
//
// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/index/TestAllFilesDetectMismatchedChecksum.java
//
// This is a simplified unit test. Instead of building a full index through
// RandomIndexWriter and verifying that OpenDirectoryReader catches the
// corruption (which requires reader-side CRC32 verification, not yet
// implemented), it writes a segments_N file through WriteSegmentInfos,
// corrupts a single byte in the footer-protected region of that file, and
// verifies that codecs.ChecksumEntireFile detects the mismatch.
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestAllFilesDetectMismatchedChecksum writes a segments_N file via
// WriteSegmentInfos, corrupts one byte inside its footer-protected region,
// and asserts that codecs.ChecksumEntireFile detects the corruption.
func TestAllFilesDetectMismatchedChecksum(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Write a minimal SegmentInfos.
	si := index.NewSegmentInfos()
	seg := index.NewSegmentInfo("_0", 100, dir)
	if err := seg.SetID(make([]byte, 16)); err != nil {
		t.Fatalf("SetID: %v", err)
	}
	sci := index.NewSegmentCommitInfo(seg, 0, -1)
	si.Add(sci)

	if err := index.WriteSegmentInfos(si, dir); err != nil {
		t.Fatalf("WriteSegmentInfos: %v", err)
	}

	segFile := index.GetSegmentFileName(si.Generation())

	// Verify the file is valid before corruption.
	in, err := dir.OpenInput(segFile, store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	if _, err := codecs.ChecksumEntireFile(in); err != nil {
		t.Fatalf("unexpected corruption before test: %v", err)
	}
	in.Close()

	// Corrupt one byte in the footer region and write to a new file.
	corruptDir := store.NewByteBuffersDirectory()
	defer corruptDir.Close()

	length, err := dir.FileLength(segFile)
	if err != nil {
		t.Fatalf("FileLength: %v", err)
	}
	if length <= int64(codecs.FooterLength()) {
		t.Fatalf("file too short (%d bytes) to have a footer", length)
	}

	// Flip a byte in [length-FooterLength, length-1] — the footer region.
	flipOffset := length - int64(codecs.FooterLength())

	readIn, err := dir.OpenInput(segFile, store.IOContextReadOnce)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	original, err := readIn.ReadBytesN(int(length))
	if err != nil {
		t.Fatalf("ReadBytesN: %v", err)
	}
	readIn.Close()

	corrupted := make([]byte, len(original))
	copy(corrupted, original)
	corrupted[flipOffset] ^= 0xFF // flip all bits

	out, err := corruptDir.CreateOutput(segFile, store.IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := out.WriteBytesN(corrupted, len(corrupted)); err != nil {
		t.Fatalf("WriteBytesN: %v", err)
	}
	out.Close()

	// Verify ChecksumEntireFile detects the corruption.
	corruptIn, err := corruptDir.OpenInput(segFile, store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer corruptIn.Close()

	if _, err := codecs.ChecksumEntireFile(corruptIn); err == nil {
		t.Fatal("mismatched checksum was NOT detected by ChecksumEntireFile")
	}
}
