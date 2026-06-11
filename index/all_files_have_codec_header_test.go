// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test verifies that every index file starts with a codec header.
//
// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/index/TestAllFilesHaveCodecHeader.java
//
// This is a simplified unit test: instead of verifying every file in every
// segment (which would require RandomIndexWriter, LineFileDocs, compound-file
// reader, and CodecUtil.checkIndexHeaderID), it verifies that the segments_N
// file produced by WriteSegmentInfos starts with CODEC_MAGIC and can be
// read back with ReadSegmentInfos.
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestAllFilesHaveCodecHeader verifies that WriteSegmentInfos produces a
// file that starts with CODEC_MAGIC and can be round-tripped through
// ReadSegmentInfos.
func TestAllFilesHaveCodecHeader(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create a SegmentInfos with one segment so the file has meaningful content.
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

	// Verify the file starts with CODEC_MAGIC.
	in, err := dir.OpenInput(segFile, store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput(%q): %v", segFile, err)
	}

	magic, err := store.ReadInt32(in)
	if err != nil {
		t.Fatalf("ReadInt32: %v", err)
	}
	in.Close()

	if magic != codecs.CODEC_MAGIC {
		t.Fatalf("file %q starts with 0x%08x, want 0x%08x (CODEC_MAGIC)", segFile, uint32(magic), uint32(codecs.CODEC_MAGIC))
	}

	// Verify ReadSegmentInfos can read the file back.
	readSI, err := index.ReadSegmentInfos(dir)
	if err != nil {
		t.Fatalf("ReadSegmentInfos: %v", err)
	}
	if readSI == nil {
		t.Fatal("ReadSegmentInfos returned nil")
	}
	if readSI.Size() != 1 {
		t.Fatalf("expected 1 segment, got %d", readSI.Size())
	}
}
