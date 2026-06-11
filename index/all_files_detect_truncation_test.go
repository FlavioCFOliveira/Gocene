// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test verifies that the index detects truncated files.
//
// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/index/TestAllFilesDetectTruncation.java
//
// This is a simplified unit test. Instead of building a full index through
// RandomIndexWriter and verifying that OpenDirectoryReader catches
// truncation (which requires reader-side CRC32/footer verification, not
// yet implemented), it writes a segments_N file through WriteSegmentInfos,
// truncates it, and verifies that codecs.ChecksumEntireFile detects the
// truncation.
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestAllFilesDetectTruncation writes a segments_N file via
// WriteSegmentInfos, truncates it by removing the last few bytes,
// and asserts that codecs.ChecksumEntireFile detects the truncation.
func TestAllFilesDetectTruncation(t *testing.T) {
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

	// Verify the file is valid before truncation.
	in, err := dir.OpenInput(segFile, store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	if _, err := codecs.ChecksumEntireFile(in); err != nil {
		t.Fatalf("unexpected corruption before test: %v", err)
	}
	in.Close()

	// Truncate the file by removing half the footer bytes.
	length, err := dir.FileLength(segFile)
	if err != nil {
		t.Fatalf("FileLength: %v", err)
	}
	truncLen := length - int64(codecs.FooterLength())/2
	if truncLen < 0 {
		truncLen = 0
	}

	truncDir := store.NewByteBuffersDirectory()
	defer truncDir.Close()

	readIn, err := dir.OpenInput(segFile, store.IOContextReadOnce)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	original, err := readIn.ReadBytesN(int(length))
	if err != nil {
		t.Fatalf("ReadBytesN: %v", err)
	}
	readIn.Close()

	out, err := truncDir.CreateOutput(segFile, store.IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := out.WriteBytesN(original[:truncLen], int(truncLen)); err != nil {
		t.Fatalf("WriteBytesN: %v", err)
	}
	out.Close()

	// Verify ChecksumEntireFile detects the truncation.
	truncIn, err := truncDir.OpenInput(segFile, store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer truncIn.Close()

	if _, err := codecs.ChecksumEntireFile(truncIn); err == nil {
		t.Fatal("truncation was NOT detected by ChecksumEntireFile")
	}
}
