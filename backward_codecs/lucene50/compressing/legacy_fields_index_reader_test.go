// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compressing

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// buildIndexPayload constructs the raw bytes that NewLegacyFieldsIndexReader
// expects to consume.  It encodes one or more blocks and terminates with a
// zero numChunks sentinel.
//
// blockSpec describes each block:
//   - docBase       int   — absolute first doc in block
//   - avgChunkDocs  int   — average docs per chunk
//   - startPointer  int64 — absolute start pointer of first chunk in block
//   - avgChunkSize  int64 — average bytes per chunk
//   - docDeltas     []int64 — zig-zag–encoded doc-base deltas per chunk
//   - spDeltas      []int64 — zig-zag–encoded start-pointer deltas per chunk
//   - bitsPerDocBase      int
//   - bitsPerStartPointer int
func buildIndexPayload(t *testing.T, blocks []blockSpec) store.IndexInput {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })

	out, err := dir.CreateOutput("idx.dat", store.IOContext{})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}

	// packed-ints version
	if err := store.WriteVInt(out, int32(packed.VersionCurrent)); err != nil {
		t.Fatalf("WriteVInt packedIntsVersion: %v", err)
	}

	for _, b := range blocks {
		numChunks := len(b.docDeltas)

		// numChunks
		if err := store.WriteVInt(out, int32(numChunks)); err != nil {
			t.Fatalf("WriteVInt numChunks: %v", err)
		}

		// doc bases section
		if err := store.WriteVInt(out, int32(b.docBase)); err != nil {
			t.Fatalf("WriteVInt docBase: %v", err)
		}
		if err := store.WriteVInt(out, int32(b.avgChunkDocs)); err != nil {
			t.Fatalf("WriteVInt avgChunkDocs: %v", err)
		}
		if err := store.WriteVInt(out, int32(b.bitsPerDocBase)); err != nil {
			t.Fatalf("WriteVInt bitsPerDocBase: %v", err)
		}
		if err := writePackedNoHeader(t, out, b.docDeltas, b.bitsPerDocBase); err != nil {
			t.Fatalf("write docDeltas: %v", err)
		}

		// start pointers section
		if err := store.WriteVLong(out, b.startPointer); err != nil {
			t.Fatalf("WriteVLong startPointer: %v", err)
		}
		if err := store.WriteVLong(out, b.avgChunkSize); err != nil {
			t.Fatalf("WriteVLong avgChunkSize: %v", err)
		}
		if err := store.WriteVInt(out, int32(b.bitsPerStartPointer)); err != nil {
			t.Fatalf("WriteVInt bitsPerStartPointer: %v", err)
		}
		if err := writePackedNoHeader(t, out, b.spDeltas, b.bitsPerStartPointer); err != nil {
			t.Fatalf("write spDeltas: %v", err)
		}
	}

	// terminating zero
	if err := store.WriteVInt(out, 0); err != nil {
		t.Fatalf("WriteVInt terminator: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close output: %v", err)
	}

	in, err := dir.OpenInput("idx.dat", store.IOContext{})
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	t.Cleanup(func() { _ = in.Close() })
	return in
}

// writePackedNoHeader writes the PACKED-format data for values into out.
func writePackedNoHeader(t *testing.T, out store.IndexOutput, values []int64, bitsPerValue int) error {
	t.Helper()
	if bitsPerValue == 0 {
		return nil // zero-width: no bytes to write
	}
	w, err := packed.GetWriterNoHeader(out, packed.FormatPacked, len(values), bitsPerValue, 256)
	if err != nil {
		return err
	}
	for _, v := range values {
		if err := w.Add(v); err != nil {
			return err
		}
	}
	return w.Finish()
}

// blockSpec describes one block in a test index payload.
type blockSpec struct {
	docBase             int
	avgChunkDocs        int
	startPointer        int64
	avgChunkSize        int64
	bitsPerDocBase      int
	bitsPerStartPointer int
	docDeltas           []int64 // zig-zag encoded
	spDeltas            []int64 // zig-zag encoded
}

// makeSI constructs a minimal SegmentInfo with the given doc count.
func makeSI(t *testing.T, docCount int) *index.SegmentInfo {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })
	si := index.NewSegmentInfo("_0", docCount, dir)
	if err := si.SetID(make([]byte, 16)); err != nil {
		t.Fatalf("SetID: %v", err)
	}
	si.SetDocCount(docCount)
	return si
}

// zigzagEncode mirrors util.ZigZagEncodeInt64 for constructing test data.
func zigzagEncode(v int64) int64 {
	return (v >> 63) ^ (v << 1)
}

// ─────────────────────────────────────────────────────────────────────────────

// TestLegacyFieldsIndexReader_CompileTimeAssertion verifies the interface
// compliance at compile time.
func TestLegacyFieldsIndexReader_CompileTimeAssertion(t *testing.T) {
	var _ LegacyFieldsIndex = (*LegacyFieldsIndexReader)(nil)
}

// TestLegacyFieldsIndexReader_EmptyIndex verifies that an index with no blocks
// (only the terminating sentinel) can be constructed.
func TestLegacyFieldsIndexReader_EmptyIndex(t *testing.T) {
	in := buildIndexPayload(t, nil)
	si := makeSI(t, 0)

	r, err := NewLegacyFieldsIndexReader(in, si)
	if err != nil {
		t.Fatalf("NewLegacyFieldsIndexReader: %v", err)
	}
	if len(r.docBases) != 0 {
		t.Errorf("blocks: got %d want 0", len(r.docBases))
	}
}

// TestLegacyFieldsIndexReader_NoopMethods verifies CheckIntegrity, Clone, Close.
func TestLegacyFieldsIndexReader_NoopMethods(t *testing.T) {
	in := buildIndexPayload(t, nil)
	si := makeSI(t, 0)

	r, err := NewLegacyFieldsIndexReader(in, si)
	if err != nil {
		t.Fatalf("NewLegacyFieldsIndexReader: %v", err)
	}

	if err := r.CheckIntegrity(); err != nil {
		t.Errorf("CheckIntegrity: %v", err)
	}
	if r.Clone() != r {
		t.Error("Clone should return self")
	}
	if err := r.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// TestLegacyFieldsIndexReader_String verifies String returns non-empty.
func TestLegacyFieldsIndexReader_String(t *testing.T) {
	in := buildIndexPayload(t, nil)
	si := makeSI(t, 0)

	r, err := NewLegacyFieldsIndexReader(in, si)
	if err != nil {
		t.Fatalf("NewLegacyFieldsIndexReader: %v", err)
	}
	if s := r.String(); s == "" {
		t.Error("String(): expected non-empty")
	}
}

// TestLegacyFieldsIndexReader_GetStartPointer_OutOfRange verifies error for
// out-of-range docIDs.
func TestLegacyFieldsIndexReader_GetStartPointer_OutOfRange(t *testing.T) {
	in := buildIndexPayload(t, nil)
	si := makeSI(t, 10)

	r, err := NewLegacyFieldsIndexReader(in, si)
	if err != nil {
		t.Fatalf("NewLegacyFieldsIndexReader: %v", err)
	}

	if _, err := r.GetStartPointer(-1); err == nil {
		t.Error("GetStartPointer(-1): expected error")
	}
	if _, err := r.GetStartPointer(10); err == nil {
		t.Error("GetStartPointer(10): expected error")
	}
}

// TestLegacyFieldsIndexReader_SingleBlock verifies that docIDs map to the
// correct start-pointer when there is one block with two chunks.
//
// Block layout:
//   - docBase=0, avgChunkDocs=5
//   - chunk 0: covers docs 0-4  (relativeDoc=0)
//   - chunk 1: covers docs 5-9  (relativeDoc=5)
//
// The deltas are all zero (every chunk is exactly average), so
// bitsPerDocBase=1 with all-zero packed values works.
func TestLegacyFieldsIndexReader_SingleBlock(t *testing.T) {
	// Two chunks: relativeDocBase(0)=0, relativeDocBase(1)=5.
	// docDeltas: zig-zag(0)=0 for both chunks.
	// spDeltas:  zig-zag(0)=0 for both.
	// startPointer = 100, avgChunkSize = 200.
	// chunk 0 startPointer = 100 + 0 = 100
	// chunk 1 startPointer = 100 + 200 = 300

	block := blockSpec{
		docBase:             0,
		avgChunkDocs:        5,
		startPointer:        100,
		avgChunkSize:        200,
		bitsPerDocBase:      1, // minimum non-zero width
		bitsPerStartPointer: 1,
		docDeltas:           []int64{0, 0},
		spDeltas:            []int64{0, 0},
	}
	in := buildIndexPayload(t, []blockSpec{block})
	si := makeSI(t, 10)

	r, err := NewLegacyFieldsIndexReader(in, si)
	if err != nil {
		t.Fatalf("NewLuckyFieldsIndexReader: %v", err)
	}
	if len(r.docBases) != 1 {
		t.Fatalf("blocks: got %d want 1", len(r.docBases))
	}

	tests := []struct {
		docID int
		want  int64
	}{
		{0, 100},
		{1, 100},
		{4, 100},
		{5, 300},
		{9, 300},
	}
	for _, tc := range tests {
		got, err := r.GetStartPointer(tc.docID)
		if err != nil {
			t.Errorf("GetStartPointer(%d): %v", tc.docID, err)
			continue
		}
		if got != tc.want {
			t.Errorf("GetStartPointer(%d): got %d want %d", tc.docID, got, tc.want)
		}
	}
}

// TestLegacyFieldsIndexReader_NonZeroDeltas verifies GetStartPointer when
// deltas are non-zero.
//
// One block, one chunk:
//   - docBase=0, avgChunkDocs=10
//   - docDelta for chunk 0 = zig-zag(2) = 4 (doc base is 0 + 10*0 + 2 = 2)
//     so chunk 0 covers docs starting at docBase=2? No — relativeDocBase is
//     the offset from docBases[block], so chunk 0 covers docs [docBases+2, ...)
//     For docID=3: block(3)=0, relativeDoc=3-0=3, relativeChunk: search for
//     largest relativeDocBase ≤ 3. chunk 0 → relativeDocBase=2 ≤ 3 → hi=0.
//   - spDelta for chunk 0 = zig-zag(-50) = 99 (negative delta)
//     startPointer = 1000 + 200*0 + (-50) = 950.
func TestLegacyFieldsIndexReader_NonZeroDeltas(t *testing.T) {
	block := blockSpec{
		docBase:             0,
		avgChunkDocs:        10,
		startPointer:        1000,
		avgChunkSize:        200,
		bitsPerDocBase:      3,
		bitsPerStartPointer: 7,
		docDeltas:           []int64{zigzagEncode(2)},   // relativeDocBase(0,0) = 0 + 2 = 2
		spDeltas:            []int64{zigzagEncode(-50)}, // relativeStartPointer = 0 + (-50) = -50
	}
	in := buildIndexPayload(t, []blockSpec{block})
	si := makeSI(t, 15)

	r, err := NewLegacyFieldsIndexReader(in, si)
	if err != nil {
		t.Fatalf("NewLegacyFieldsIndexReader: %v", err)
	}

	// docID=5: block=0, relativeDoc=5-0=5, relativeChunk=0 (only chunk), sp=1000-50=950
	got, err := r.GetStartPointer(5)
	if err != nil {
		t.Fatalf("GetStartPointer(5): %v", err)
	}
	if got != 950 {
		t.Errorf("GetStartPointer(5): got %d want 950", got)
	}
}

// TestLegacyFieldsIndexReader_CorruptBitsPerDocBase verifies that an out-of-range
// bitsPerDocBase (>32) returns an error.
func TestLegacyFieldsIndexReader_CorruptBitsPerDocBase(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })

	out, err := dir.CreateOutput("bad.dat", store.IOContext{})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	// packedIntsVersion
	_ = store.WriteVInt(out, int32(packed.VersionCurrent))
	// numChunks = 1 (non-zero)
	_ = store.WriteVInt(out, 1)
	// docBase, avgChunkDocs
	_ = store.WriteVInt(out, 0)
	_ = store.WriteVInt(out, 1)
	// bitsPerDocBase = 33 (invalid)
	_ = store.WriteVInt(out, 33)
	_ = out.Close()

	in, _ := dir.OpenInput("bad.dat", store.IOContext{})
	t.Cleanup(func() { _ = in.Close() })

	si := makeSI(t, 10)
	if _, err := NewLegacyFieldsIndexReader(in, si); err == nil {
		t.Error("expected error for bitsPerDocBase=33")
	}
}

// TestLegacyFieldsIndexReader_CorruptBitsPerStartPointer verifies that an
// out-of-range bitsPerStartPointer (>64) returns an error.
func TestLegacyFieldsIndexReader_CorruptBitsPerStartPointer(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })

	out, err := dir.CreateOutput("bad2.dat", store.IOContext{})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	_ = store.WriteVInt(out, int32(packed.VersionCurrent))
	_ = store.WriteVInt(out, 1) // numChunks
	_ = store.WriteVInt(out, 0) // docBase
	_ = store.WriteVInt(out, 1) // avgChunkDocs
	_ = store.WriteVInt(out, 0) // bitsPerDocBase=0 (zero-width)
	// no packed data for zero-width
	_ = store.WriteVLong(out, 0) // startPointer
	_ = store.WriteVLong(out, 0) // avgChunkSize
	_ = store.WriteVInt(out, 65) // bitsPerStartPointer=65 (invalid)
	_ = out.Close()

	in, _ := dir.OpenInput("bad2.dat", store.IOContext{})
	t.Cleanup(func() { _ = in.Close() })

	si := makeSI(t, 10)
	if _, err := NewLegacyFieldsIndexReader(in, si); err == nil {
		t.Error("expected error for bitsPerStartPointer=65")
	}
}
