// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test verifies the Lucene99SegmentInfo .si writer.
package index_test

import (
	"encoding/binary"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// codecMagic is the expected first four bytes of every Lucene codec file.
// Mirrors org.apache.lucene.codecs.CodecUtil.MAGIC = 0x3FD76C17.
const siCodecMagic uint32 = 0x3FD76C17

// TestWriteSegmentInfo_Magic commits an index through IndexWriter and
// verifies that the resulting .si file starts with the Lucene codec magic
// 0x3FD76C17 (big-endian Int32 per Lucene's DataOutput.writeInt).
//
// Acceptance criteria:
//   - .si file exists in the directory after Commit
//   - First 4 bytes equal 0x3f 0xd7 0x6c 0x17
//   - No error from Open or Read
func TestWriteSegmentInfo_Magic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// AddDocument to ensure at least one segment is created.
	doc := &testDocument{fields: []interface{}{}}
	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Discover the .si file that was written.
	files, err := dir.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}

	var siFile string
	for _, f := range files {
		if len(f) > 3 && f[len(f)-3:] == ".si" {
			siFile = f
			break
		}
	}
	if siFile == "" {
		t.Fatalf("no .si file found; directory contains: %v", files)
	}

	in, err := dir.OpenInput(siFile, store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput(%s): %v", siFile, err)
	}
	defer in.Close()

	header, err := in.ReadBytesN(4)
	if err != nil {
		t.Fatalf("ReadBytesN(4) from %s: %v", siFile, err)
	}

	got := binary.BigEndian.Uint32(header)
	if got != siCodecMagic {
		t.Errorf(".si magic = 0x%08x, want 0x%08x (bytes: % x)", got, siCodecMagic, header)
	}
}

// TestWriteSegmentInfo_AfterForceMerge verifies that ForceMerge also
// writes a .si file with the Lucene codec magic header.
func TestWriteSegmentInfo_AfterForceMerge(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	for i := 0; i < 3; i++ {
		doc := &testDocument{fields: []interface{}{}}
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
		if err := w.Commit(); err != nil {
			t.Fatalf("Commit %d: %v", i, err)
		}
	}

	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit after ForceMerge: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	files, err := dir.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}

	found := 0
	for _, f := range files {
		if len(f) > 3 && f[len(f)-3:] == ".si" {
			found++
			in, err := dir.OpenInput(f, store.IOContextRead)
			if err != nil {
				t.Fatalf("OpenInput(%s): %v", f, err)
			}
			header, err := in.ReadBytesN(4)
			_ = in.Close()
			if err != nil {
				t.Fatalf("ReadBytesN(4) from %s: %v", f, err)
			}
			got := binary.BigEndian.Uint32(header)
			if got != siCodecMagic {
				t.Errorf("%s magic = 0x%08x, want 0x%08x", f, got, siCodecMagic)
			}
		}
	}
	if found == 0 {
		t.Fatalf("no .si files found after ForceMerge; directory: %v", files)
	}
}
