// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test verifies that the index detects broken codec headers.
//
// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/index/TestAllFilesCheckIndexHeader.java
//
// This is a simplified unit test. Instead of building a full index and
// verifying that OpenDirectoryReader catches corrupted headers on open
// (which requires reader-side header validation, not yet implemented),
// it writes a segments_N file through WriteSegmentInfos, corrupts the
// leading header bytes, and verifies that codecs.CheckIndexHeader detects
// the corruption.
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestAllFilesCheckIndexHeader writes a segments_N file via
// WriteSegmentInfos, corrupts the leading codec header bytes, and
// asserts that codecs.CheckIndexHeader detects the corruption.
func TestAllFilesCheckIndexHeader(t *testing.T) {
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

	// Read the file and corrupt the leading header bytes.
	length, err := dir.FileLength(segFile)
	if err != nil {
		t.Fatalf("FileLength: %v", err)
	}
	readIn, err := dir.OpenInput(segFile, store.IOContextReadOnce)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	original, err := readIn.ReadBytesN(int(length))
	if err != nil {
		t.Fatalf("ReadBytesN: %v", err)
	}
	readIn.Close()

	// Flip every bit in the first 8 bytes (covering CODEC_MAGIC + part of
	// the codec name length byte), guaranteeing CheckHeader fails.
	corrupted := make([]byte, len(original))
	copy(corrupted, original)
	corruptBytes := 8
	if len(original) < corruptBytes {
		corruptBytes = len(original)
	}
	for i := 0; i < corruptBytes; i++ {
		corrupted[i] = ^original[i]
	}

	corruptDir := store.NewByteBuffersDirectory()
	defer corruptDir.Close()

	out, err := corruptDir.CreateOutput(segFile, store.IOContextDefault)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := out.WriteBytesN(corrupted, len(corrupted)); err != nil {
		t.Fatalf("WriteBytesN: %v", err)
	}
	out.Close()

	// Verify CheckIndexHeader detects the corruption.
	in, err := corruptDir.OpenInput(segFile, store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()

	// Use CheckIndexHeader with the expected parameters from WriteSegmentInfos.
	// The codec name is "segments" and version is 10.
	if _, err := codecs.CheckIndexHeader(in, "segments", 10, 10, nil, ""); err == nil {
		t.Fatal("corrupted header was NOT detected by CheckIndexHeader")

	}
}
