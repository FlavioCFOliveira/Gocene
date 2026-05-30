// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// rmp #4 acceptance tests: SegmentCoreReaders must always wire the codec
// DocValuesProducer for a codec-backed segment that owns its data files, so
// SegmentReader exposes DocValues for every codec-written field; and a
// core-readers wiring failure must surface as an explicit error rather than a
// silent data-less reader, EXCEPT for benign metadata-only segments (a
// not-yet-data-merged ForceMerge result), which still reopen structurally.
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestSegmentCoreReaders_AlwaysWireDocValues verifies that a flushed,
// codec-backed segment carrying all five DocValues types reopens with non-nil
// core readers, a non-nil DocValuesProducer, and a working read-back for every
// field. This locks in the "always construct the codec DocValuesProducer for
// codec-backed segments" guarantee (rmp #4).
func TestSegmentCoreReaders_AlwaysWireDocValues(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	const numDocs = 4
	for i := 0; i < numDocs; i++ {
		if err := writer.AddDocument(dvTestDoc(i)); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	segs := reader.GetSegmentReaders()
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segs))
	}
	r := segs[0]

	core := r.GetCoreReaders()
	if core == nil {
		t.Fatal("GetCoreReaders() = nil; codec-backed segment took the silent fallback (rmp #4)")
	}
	if core.GetDocValuesProducer() == nil {
		t.Fatal("GetDocValuesProducer() = nil; DocValuesProducer was not wired (rmp #4)")
	}

	// Every codec-written DocValues field must be exposed and non-nil.
	if dv, err := r.GetNumericDocValues("ndv"); err != nil || dv == nil {
		t.Fatalf("GetNumericDocValues(ndv): dv=%v err=%v", dv, err)
	}
	if dv, err := r.GetBinaryDocValues("bdv"); err != nil || dv == nil {
		t.Fatalf("GetBinaryDocValues(bdv): dv=%v err=%v", dv, err)
	}
	if dv, err := r.GetSortedDocValues("sdv"); err != nil || dv == nil {
		t.Fatalf("GetSortedDocValues(sdv): dv=%v err=%v", dv, err)
	}
	if dv, err := r.GetSortedNumericDocValues("sndv"); err != nil || dv == nil {
		t.Fatalf("GetSortedNumericDocValues(sndv): dv=%v err=%v", dv, err)
	}
	if dv, err := r.GetSortedSetDocValues("ssdv"); err != nil || dv == nil {
		t.Fatalf("GetSortedSetDocValues(ssdv): dv=%v err=%v", dv, err)
	}
}

// TestForceMergeMetadataOnlySegmentReopens verifies that reopening an index
// whose only segment is a not-yet-data-merged ForceMerge result (a metadata-
// only segment carrying just .si/.fnm) still succeeds structurally rather than
// erroring. The real merge that writes the merged segment's data files —
// including index-sorted DocValues read-back, which the GOC-4136 order
// verifications depend on — is tracked as a separate task. When that lands this
// test should be upgraded to assert per-document read-back.
func TestForceMergeMetadataOnlySegmentReopens(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for _, v := range []int64{18, -1, 7} {
		doc := document.NewDocument()
		f, _ := document.NewNumericDocValuesField("foo", v)
		doc.Add(f)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit: %v", err)
		}
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader on force-merged index must not error (rmp #4 graceful metadata-only fallback): %v", err)
	}
	defer reader.Close()

	if got := reader.MaxDoc(); got != 3 {
		t.Fatalf("merged MaxDoc = %d, want 3", got)
	}
}
