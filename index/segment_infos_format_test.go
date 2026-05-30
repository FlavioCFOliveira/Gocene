// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/internal/crossengine"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// lucene104SegmentsMagic is the 4-byte CodecUtil magic that must appear at the
// start of every segments_N file produced by WriteSegmentInfos.
var lucene104SegmentsMagic = [4]byte{0x3f, 0xd7, 0x6c, 0x17}

// TestIndexWriterCommit_SegmentInfosFormat creates an index, commits it, opens
// the resulting segments_N file, and asserts that the first 4 bytes are the
// Lucene 10.4.0 CodecUtil magic (0x3FD76C17).
func TestIndexWriterCommit_SegmentInfosFormat(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(nil)
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := document.NewDocument()
	field, err := document.NewTextField("body", "hello world", true)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(field)
	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Locate the segments_N file.
	files, err := dir.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	var segFile string
	for _, f := range files {
		if len(f) > 9 && f[:9] == "segments_" {
			segFile = f
			break
		}
	}
	if segFile == "" {
		t.Fatal("no segments_N file found after commit")
	}

	// Read and verify the leading 4 bytes.
	in, err := dir.OpenInput(segFile, store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput %q: %v", segFile, err)
	}
	defer in.Close()

	var hdr [4]byte
	b := make([]byte, 4)
	if err := in.ReadBytes(b); err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	copy(hdr[:], b)

	if hdr != lucene104SegmentsMagic {
		t.Errorf("segments file magic = %x, want %x", hdr, lucene104SegmentsMagic)
	}
}

// TestReadSegmentInfos_LuceneFixture verifies that ReadSegmentInfos can parse a
// segments_1 file produced by Apache Lucene 10.4.0.
func TestReadSegmentInfos_LuceneFixture(t *testing.T) {
	crossengine.SkipIfNoFixtures(t)
	dir := crossengine.OpenFixturesDir(t)

	sis, err := index.ReadSegmentInfos(dir)
	if err != nil {
		t.Fatalf("ReadSegmentInfos on Lucene 10.4.0 fixture: %v", err)
	}
	if sis.Generation() != 1 {
		t.Errorf("expected generation=1, got %d", sis.Generation())
	}
	// The fixture contains exactly one segment.
	if sis.Size() == 0 {
		t.Error("expected at least one segment in the fixture index")
	}
}

// TestSegmentInfos_RealOnDiskRoundTrip builds a genuine on-disk index with a
// real codec and verifies the rmp #4785 contract: per-segment docCount comes
// from the .si file and FieldInfos come from the .fnm file (inside the .cfs for
// a compound segment), with NO _gocene_* keys in the segments_N commitUserData.
// Real commit userData (the "author" key) still round-trips unchanged.
func TestSegmentInfos_RealOnDiskRoundTrip(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(createTestAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < 4; i++ {
		doc := document.NewDocument()
		f, ferr := document.NewTextField("body", "the quick brown fox", true)
		if ferr != nil {
			t.Fatalf("NewTextField: %v", ferr)
		}
		doc.Add(f)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
	}
	w.SetLiveCommitData(map[string]string{"author": "gocene"})
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// The segments_N userData must contain NO _gocene_* keys.
	sis, err := index.ReadSegmentInfos(dir)
	if err != nil {
		t.Fatalf("ReadSegmentInfos: %v", err)
	}
	for k := range sis.GetUserData() {
		if strings.HasPrefix(k, "_gocene_") {
			t.Errorf("segments_N userData still carries Gocene extension key %q", k)
		}
	}
	if got := sis.GetUserDataValue("author"); got != "gocene" {
		t.Errorf("userData[author]: want %q, got %q", "gocene", got)
	}
	if sis.Size() != 1 {
		t.Fatalf("expected 1 segment, got %d", sis.Size())
	}
	// docCount is recovered from the real .si file (not userData).
	if dc := sis.Get(0).SegmentInfo().DocCount(); dc != 4 {
		t.Errorf("segment docCount from .si: want 4, got %d", dc)
	}

	// FieldInfos are recovered from the real .fnm file (inside the .cfs), and
	// the reopened reader sees the indexed field.
	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	if r.NumDocs() != 4 {
		t.Errorf("NumDocs: want 4, got %d", r.NumDocs())
	}
	fis := r.GetFieldInfos()
	if fis.Size() != 1 {
		t.Fatalf("merged FieldInfos.Size(): want 1, got %d", fis.Size())
	}
	if fis.GetByName("body") == nil {
		t.Error("field 'body' missing after on-disk round-trip")
	}
}
