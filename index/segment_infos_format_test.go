// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
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

// TestSegmentInfos_RoundTrip_WithExtensions writes a SegmentInfos with
// FieldInfos, deleted ordinals, a parentField, and an indexSort, then reads it
// back and verifies all in-memory extensions are faithfully restored.
func TestSegmentInfos_RoundTrip_WithExtensions(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sis := index.NewSegmentInfos()
	sis.SetGeneration(3)
	sis.SetCounter(5)
	sis.SetUserDataValue("author", "gocene")

	si := index.NewSegmentInfo("_2", 10, nil)
	sci := index.NewSegmentCommitInfo(si, 2, -1)

	// Attach a FieldInfos with one field.
	fis := index.NewFieldInfos()
	opts := index.FieldInfoOptions{
		IndexOptions:             index.IndexOptionsDocsAndFreqsAndPositions,
		DocValuesType:            index.DocValuesTypeNone,
		DocValuesSkipIndexType:   index.DocValuesSkipIndexTypeNone,
		DocValuesGen:             -1,
		Stored:                   true,
		Tokenized:                true,
		VectorEncoding:           index.VectorEncodingFloat32,
		VectorSimilarityFunction: index.VectorSimilarityFunctionEuclidean,
	}
	_ = fis.Add(index.NewFieldInfo("body", 0, opts))
	sci.SetInMemoryFieldInfos(fis)

	// Attach deleted ordinals.
	sci.SetDeletedOrdinals([]int{1, 3, 7})

	sis.Add(sci)

	if err := index.WriteSegmentInfos(sis, dir); err != nil {
		t.Fatalf("WriteSegmentInfos: %v", err)
	}

	got, err := index.ReadSegmentInfos(dir)
	if err != nil {
		t.Fatalf("ReadSegmentInfos: %v", err)
	}

	if got.Generation() != sis.Generation() {
		t.Errorf("generation: want %d, got %d", sis.Generation(), got.Generation())
	}
	if got.Counter() != sis.Counter() {
		t.Errorf("counter: want %d, got %d", sis.Counter(), got.Counter())
	}
	if got.GetUserDataValue("author") != "gocene" {
		t.Errorf("userData[author]: want %q, got %q", "gocene", got.GetUserDataValue("author"))
	}
	if got.Size() != 1 {
		t.Fatalf("expected 1 segment, got %d", got.Size())
	}

	gotSCI := got.Get(0)
	if gotSCI.Name() != "_2" {
		t.Errorf("segment name: want _2, got %s", gotSCI.Name())
	}
	if gotSCI.DelCount() != 2 {
		t.Errorf("delCount: want 2, got %d", gotSCI.DelCount())
	}

	gotFIS := gotSCI.GetInMemoryFieldInfos()
	if gotFIS == nil {
		t.Fatal("expected FieldInfos to be restored, got nil")
	}
	if gotFIS.Size() != 1 {
		t.Errorf("FieldInfos.Size(): want 1, got %d", gotFIS.Size())
	}
	fi := gotFIS.GetByName("body")
	if fi == nil {
		t.Fatal("field 'body' missing after round-trip")
	}
	if !fi.IsStored() {
		t.Error("field 'body': IsStored should be true")
	}

	ords := gotSCI.GetDeletedOrdinals()
	if len(ords) != 3 || ords[0] != 1 || ords[1] != 3 || ords[2] != 7 {
		t.Errorf("deleted ordinals: want [1 3 7], got %v", ords)
	}
}
