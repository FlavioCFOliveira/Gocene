// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bkd

import (
	"encoding/binary"
	"errors"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// newOfflineWriterFixture builds a writer atop a fresh
// ByteBuffersDirectory and returns both so the test can verify
// directory state (file presence, length, etc.) afterwards.
func newOfflineWriterFixture(t *testing.T, cfg BKDConfig, expectedCount int64) (*OfflinePointWriter, *store.ByteBuffersDirectory) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	w, err := NewOfflinePointWriter(cfg, dir, "tmp", "test", expectedCount)
	if err != nil {
		t.Fatalf("NewOfflinePointWriter: %v", err)
	}
	return w, dir
}

// buildPackedValue produces a deterministic packed value of the
// requested length. The encoding is irrelevant to the writer; the
// tests only need uniqueness so they can detect cross-talk between
// slots.
func buildPackedValue(packedBytes int, seed byte) []byte {
	out := make([]byte, packedBytes)
	for i := range out {
		out[i] = byte(i) + seed
	}
	return out
}

// TestOfflinePointWriter_AppendByteSequenceMatchesWireFormat verifies
// that a single Append call produces exactly packedValue || BE(docID)
// followed by the codec footer once the writer is closed.
func TestOfflinePointWriter_AppendByteSequenceMatchesWireFormat(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	w, dir := newOfflineWriterFixture(t, cfg, 0)
	defer dir.Close()

	packed := buildPackedValue(cfg.PackedBytesLength(), 0xA5)
	const docID int = 0x01020304

	if err := w.Append(packed, docID); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if got := w.Count(); got != 1 {
		t.Fatalf("Count after one Append: got %d want 1", got)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	in, err := dir.OpenInput(w.Name(), store.IOContextReadOnce)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()

	wantLen := int64(cfg.BytesPerDoc() + codecs.FooterLength())
	if in.Length() != wantLen {
		t.Fatalf("file length: got %d want %d", in.Length(), wantLen)
	}

	got := make([]byte, cfg.BytesPerDoc())
	if err := in.ReadBytes(got); err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	wantPoint := make([]byte, cfg.BytesPerDoc())
	copy(wantPoint, packed)
	binary.BigEndian.PutUint32(wantPoint[cfg.PackedBytesLength():], uint32(docID))
	if string(got) != string(wantPoint) {
		t.Fatalf("point bytes: got %x want %x", got, wantPoint)
	}
}

// TestOfflinePointWriter_RoundTripViaOfflinePointReader confirms a
// full Append loop followed by Close produces a file that the
// existing OfflinePointReader can iterate, recovering every point in
// order, with a valid codec footer.
func TestOfflinePointWriter_RoundTripViaOfflinePointReader(t *testing.T) {
	cfg, err := Of(2, 2, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	w, dir := newOfflineWriterFixture(t, cfg, 0)
	defer dir.Close()

	const n = 11
	wantPacked := make([][]byte, n)
	wantDocIDs := make([]int32, n)
	for i := 0; i < n; i++ {
		wantPacked[i] = buildPackedValue(cfg.PackedBytesLength(), byte(i*3+1))
		wantDocIDs[i] = int32(i*17 - 5)
		if err := w.Append(wantPacked[i], int(wantDocIDs[i])); err != nil {
			t.Fatalf("Append(%d): %v", i, err)
		}
	}
	if got := w.Count(); got != int64(n) {
		t.Fatalf("Count: got %d want %d", got, n)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	pr, err := w.GetReader(0, int64(n))
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	defer pr.Close()
	r, ok := pr.(*OfflinePointReader)
	if !ok {
		t.Fatalf("GetReader returned %T; want *OfflinePointReader", pr)
	}
	if r.checksumIn == nil {
		t.Fatalf("expected ChecksumIndexInput for full-file read")
	}

	gotPacked := make([][]byte, 0, n)
	gotDocIDs := make([]int32, 0, n)
	for {
		ok, err := r.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if !ok {
			break
		}
		pv := r.PointValue()
		ref := pv.PackedValue()
		buf := make([]byte, ref.Length)
		copy(buf, ref.Bytes[ref.Offset:ref.Offset+ref.Length])
		gotPacked = append(gotPacked, buf)
		gotDocIDs = append(gotDocIDs, int32(pv.DocID()))
	}
	if len(gotPacked) != n {
		t.Fatalf("iterated %d points; want %d", len(gotPacked), n)
	}
	for i := 0; i < n; i++ {
		if string(gotPacked[i]) != string(wantPacked[i]) {
			t.Errorf("packed[%d]: got %x want %x", i, gotPacked[i], wantPacked[i])
		}
		if gotDocIDs[i] != wantDocIDs[i] {
			t.Errorf("docID[%d]: got %d want %d", i, gotDocIDs[i], wantDocIDs[i])
		}
	}
	if err := r.Close(); err != nil {
		t.Fatalf("reader Close: %v", err)
	}
	if !r.checked {
		t.Fatalf("expected checksum to have been verified on Close")
	}
}

// TestOfflinePointWriter_AppendPointValueRoundTrip checks that the
// PointValue overload produces the same bytes as the byte/int form
// by sourcing input from a HeapPointWriter slot.
func TestOfflinePointWriter_AppendPointValueRoundTrip(t *testing.T) {
	cfg, err := Of(1, 1, 8, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}

	// Pre-stage three points in a HeapPointWriter so we can replay
	// them through AppendPointValue.
	const n = 3
	heap := NewHeapPointWriter(cfg, n)
	wantPacked := make([][]byte, n)
	wantDocIDs := []int32{42, -1, 7}
	for i := 0; i < n; i++ {
		wantPacked[i] = buildPackedValue(cfg.PackedBytesLength(), byte(i+5))
		if err := heap.Append(wantPacked[i], int(wantDocIDs[i])); err != nil {
			t.Fatalf("heap.Append(%d): %v", i, err)
		}
	}
	if err := heap.Close(); err != nil {
		t.Fatalf("heap.Close: %v", err)
	}

	w, dir := newOfflineWriterFixture(t, cfg, 0)
	defer dir.Close()
	for i := 0; i < n; i++ {
		if err := w.AppendPointValue(heap.GetPackedValueSlice(i)); err != nil {
			t.Fatalf("AppendPointValue(%d): %v", i, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	pr, err := w.GetReader(0, int64(n))
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	defer pr.Close()
	for i := 0; i < n; i++ {
		ok, err := pr.Next()
		if err != nil || !ok {
			t.Fatalf("Next(%d): ok=%v err=%v", i, ok, err)
		}
		pv := pr.PointValue()
		ref := pv.PackedValue()
		got := ref.Bytes[ref.Offset : ref.Offset+ref.Length]
		if string(got) != string(wantPacked[i]) {
			t.Errorf("packed[%d]: got %x want %x", i, got, wantPacked[i])
		}
		if int32(pv.DocID()) != wantDocIDs[i] {
			t.Errorf("docID[%d]: got %d want %d", i, pv.DocID(), wantDocIDs[i])
		}
	}
}

// TestOfflinePointWriter_RoundTripPartialSlice exercises GetReader
// with a non-trivial start/length pair to confirm the reader can
// iterate a sub-range of the file written by the writer.
func TestOfflinePointWriter_RoundTripPartialSlice(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	w, dir := newOfflineWriterFixture(t, cfg, 0)
	defer dir.Close()

	const n = 8
	packed := make([][]byte, n)
	docIDs := make([]int32, n)
	for i := 0; i < n; i++ {
		packed[i] = buildPackedValue(cfg.PackedBytesLength(), byte(i+1))
		docIDs[i] = int32(i * 11)
		if err := w.Append(packed[i], int(docIDs[i])); err != nil {
			t.Fatalf("Append(%d): %v", i, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	const start, length = int64(2), int64(4)
	pr, err := w.GetReader(start, length)
	if err != nil {
		t.Fatalf("GetReader(start=%d length=%d): %v", start, length, err)
	}
	defer pr.Close()

	gotIDs := make([]int32, 0, length)
	for {
		ok, err := pr.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if !ok {
			break
		}
		gotIDs = append(gotIDs, int32(pr.PointValue().DocID()))
	}
	wantIDs := docIDs[start : start+length]
	if len(gotIDs) != len(wantIDs) {
		t.Fatalf("iterated %d points; want %d", len(gotIDs), len(wantIDs))
	}
	for i := range gotIDs {
		if gotIDs[i] != wantIDs[i] {
			t.Errorf("docID[%d]: got %d want %d", i, gotIDs[i], wantIDs[i])
		}
	}
}

// TestOfflinePointWriter_CountTracksAppends covers Count() before /
// during / after a sequence of Append + AppendPointValue calls.
func TestOfflinePointWriter_CountTracksAppends(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	w, dir := newOfflineWriterFixture(t, cfg, 0)
	defer dir.Close()
	defer w.Close()

	if got := w.Count(); got != 0 {
		t.Fatalf("initial Count: got %d want 0", got)
	}

	// Stage two points then replay via AppendPointValue.
	heap := NewHeapPointWriter(cfg, 2)
	for i := 0; i < 2; i++ {
		if err := heap.Append(buildPackedValue(cfg.PackedBytesLength(), byte(i+1)), i); err != nil {
			t.Fatalf("heap.Append(%d): %v", i, err)
		}
	}
	if err := heap.Close(); err != nil {
		t.Fatalf("heap.Close: %v", err)
	}

	if err := w.Append(buildPackedValue(cfg.PackedBytesLength(), 9), 99); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if got := w.Count(); got != 1 {
		t.Fatalf("Count after 1 Append: got %d want 1", got)
	}
	if err := w.AppendPointValue(heap.GetPackedValueSlice(0)); err != nil {
		t.Fatalf("AppendPointValue: %v", err)
	}
	if err := w.AppendPointValue(heap.GetPackedValueSlice(1)); err != nil {
		t.Fatalf("AppendPointValue: %v", err)
	}
	if got := w.Count(); got != 3 {
		t.Fatalf("Count after 3 appends: got %d want 3", got)
	}
}

// TestOfflinePointWriter_DestroyRemovesFile verifies Destroy
// successfully removes the temp file from the directory.
func TestOfflinePointWriter_DestroyRemovesFile(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	w, dir := newOfflineWriterFixture(t, cfg, 0)
	defer dir.Close()

	if err := w.Append(buildPackedValue(cfg.PackedBytesLength(), 1), 1); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	name := w.Name()
	if !dir.FileExists(name) {
		t.Fatalf("expected file %q to exist before Destroy", name)
	}
	if err := w.Destroy(); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if dir.FileExists(name) {
		t.Fatalf("file %q still present after Destroy", name)
	}
}

// TestOfflinePointWriter_CloseIsIdempotent exercises calling Close
// twice — the second call must be a no-op.
func TestOfflinePointWriter_CloseIsIdempotent(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	w, dir := newOfflineWriterFixture(t, cfg, 0)
	defer dir.Close()

	if err := w.Append(buildPackedValue(cfg.PackedBytesLength(), 1), 1); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close (first): %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close (second): %v", err)
	}
	if !w.IsClosed() {
		t.Fatalf("IsClosed: got false want true")
	}
}

// TestOfflinePointWriter_PostCloseRejection enforces that Append /
// AppendPointValue return ErrOfflinePointWriterClosed after Close.
func TestOfflinePointWriter_PostCloseRejection(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	w, dir := newOfflineWriterFixture(t, cfg, 0)
	defer dir.Close()

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	appendErr := w.Append(buildPackedValue(cfg.PackedBytesLength(), 1), 1)
	if !errors.Is(appendErr, ErrOfflinePointWriterClosed) {
		t.Fatalf("Append after Close: err=%v want %v", appendErr, ErrOfflinePointWriterClosed)
	}

	// Build a PointValue we can hand to AppendPointValue after Close.
	heap := NewHeapPointWriter(cfg, 1)
	if err := heap.Append(buildPackedValue(cfg.PackedBytesLength(), 1), 0); err != nil {
		t.Fatalf("heap.Append: %v", err)
	}
	if err := heap.Close(); err != nil {
		t.Fatalf("heap.Close: %v", err)
	}
	pvErr := w.AppendPointValue(heap.GetPackedValueSlice(0))
	if !errors.Is(pvErr, ErrOfflinePointWriterClosed) {
		t.Fatalf("AppendPointValue after Close: err=%v want %v", pvErr, ErrOfflinePointWriterClosed)
	}
}

// TestOfflinePointWriter_GetReaderRejectsBeforeClose covers the
// open-writer guard on GetReader.
func TestOfflinePointWriter_GetReaderRejectsBeforeClose(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	w, dir := newOfflineWriterFixture(t, cfg, 0)
	defer dir.Close()

	if err := w.Append(buildPackedValue(cfg.PackedBytesLength(), 1), 1); err != nil {
		t.Fatalf("Append: %v", err)
	}
	_, err = w.GetReader(0, 1)
	if !errors.Is(err, ErrOfflinePointWriterOpen) {
		t.Fatalf("GetReader before Close: err=%v want %v", err, ErrOfflinePointWriterOpen)
	}
	// Now close and try again — must succeed.
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	r, err := w.GetReader(0, 1)
	if err != nil {
		t.Fatalf("GetReader after Close: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("reader Close: %v", err)
	}
}

// TestOfflinePointWriter_GetReaderRejectsOutOfRange covers the
// start/length bounds checks on GetReader.
func TestOfflinePointWriter_GetReaderRejectsOutOfRange(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	w, dir := newOfflineWriterFixture(t, cfg, 0)
	defer dir.Close()

	const n = 3
	for i := 0; i < n; i++ {
		if err := w.Append(buildPackedValue(cfg.PackedBytesLength(), byte(i+1)), i); err != nil {
			t.Fatalf("Append(%d): %v", i, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := w.GetReader(-1, 1); err == nil {
		t.Fatalf("expected error for negative start")
	}
	if _, err := w.GetReader(0, -1); err == nil {
		t.Fatalf("expected error for negative length")
	}
	if _, err := w.GetReader(0, n+1); err == nil {
		t.Fatalf("expected error for start+length > count")
	}
}

// TestOfflinePointWriter_AppendRejectsBadPackedLength enforces the
// packed-value length contract.
func TestOfflinePointWriter_AppendRejectsBadPackedLength(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	w, dir := newOfflineWriterFixture(t, cfg, 0)
	defer dir.Close()
	defer w.Close()

	short := make([]byte, cfg.PackedBytesLength()-1)
	if err := w.Append(short, 1); err == nil {
		t.Fatalf("expected error for undersized packedValue")
	} else if !strings.Contains(err.Error(), "must have length") {
		t.Fatalf("unexpected error: %v", err)
	}

	long := make([]byte, cfg.PackedBytesLength()+1)
	if err := w.Append(long, 1); err == nil {
		t.Fatalf("expected error for oversized packedValue")
	}
}

// TestOfflinePointWriter_AppendPointValueRejectsBadLength enforces
// the BytesPerDoc length contract on the PointValue overload.
func TestOfflinePointWriter_AppendPointValueRejectsBadLength(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	w, dir := newOfflineWriterFixture(t, cfg, 0)
	defer dir.Close()
	defer w.Close()

	badLen := cfg.BytesPerDoc() - 1
	bad := &stubPointValue{combo: &util.BytesRef{Bytes: make([]byte, badLen), Offset: 0, Length: badLen}}
	if err := w.AppendPointValue(bad); err == nil {
		t.Fatalf("expected error for undersized PackedValueDocIDBytes")
	} else if !strings.Contains(err.Error(), "must have length") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestOfflinePointWriter_ExpectedCountEnforced verifies the
// expectedCount upper bound is honoured when non-zero.
func TestOfflinePointWriter_ExpectedCountEnforced(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	w, dir := newOfflineWriterFixture(t, cfg, 2)
	defer dir.Close()
	defer w.Close()

	for i := 0; i < 2; i++ {
		if err := w.Append(buildPackedValue(cfg.PackedBytesLength(), byte(i+1)), i); err != nil {
			t.Fatalf("Append(%d): %v", i, err)
		}
	}
	err = w.Append(buildPackedValue(cfg.PackedBytesLength(), 99), 99)
	if err == nil {
		t.Fatalf("expected error for Append beyond expectedCount")
	}
	if !strings.Contains(err.Error(), "expectedCount") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestOfflinePointWriter_ExpectedCountZeroDisablesCheck confirms
// expectedCount==0 leaves the upper bound disabled.
func TestOfflinePointWriter_ExpectedCountZeroDisablesCheck(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	w, dir := newOfflineWriterFixture(t, cfg, 0)
	defer dir.Close()
	defer w.Close()

	for i := 0; i < 5; i++ {
		if err := w.Append(buildPackedValue(cfg.PackedBytesLength(), byte(i+1)), i); err != nil {
			t.Fatalf("Append(%d): %v", i, err)
		}
	}
	if got := w.Count(); got != 5 {
		t.Fatalf("Count: got %d want 5", got)
	}
}

// TestOfflinePointWriter_RejectsNegativeExpectedCount covers the
// constructor validation.
func TestOfflinePointWriter_RejectsNegativeExpectedCount(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	if _, err := NewOfflinePointWriter(cfg, dir, "tmp", "test", -1); err == nil {
		t.Fatalf("expected error for negative expectedCount")
	}
}

// TestOfflinePointWriter_RejectsNilDirectory covers the constructor
// nil-directory guard.
func TestOfflinePointWriter_RejectsNilDirectory(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	if _, err := NewOfflinePointWriter(cfg, nil, "tmp", "test", 0); err == nil {
		t.Fatalf("expected error for nil tempDir")
	}
}

// TestOfflinePointWriter_RejectsDirectoryWithoutTempOutput covers
// the typed-error path when the directory cannot create temp files.
func TestOfflinePointWriter_RejectsDirectoryWithoutTempOutput(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	dir := &noTempDirectory{}
	_, err = NewOfflinePointWriter(cfg, dir, "tmp", "test", 0)
	if err == nil {
		t.Fatalf("expected error for directory without CreateTempOutput")
	}
	if !strings.Contains(err.Error(), "CreateTempOutput") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestOfflinePointWriter_StringIncludesCountAndName covers the
// debug representation shape.
func TestOfflinePointWriter_StringIncludesCountAndName(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	w, dir := newOfflineWriterFixture(t, cfg, 0)
	defer dir.Close()
	defer w.Close()

	if err := w.Append(buildPackedValue(cfg.PackedBytesLength(), 1), 1); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got := w.String()
	if !strings.Contains(got, "count=1") {
		t.Errorf("String missing count=1: %q", got)
	}
	if !strings.Contains(got, w.Name()) {
		t.Errorf("String missing temp file name: %q", got)
	}
}

// TestOfflinePointWriter_DocIDEdgeCases covers the encoding of
// boundary docID values (zero, max int32, min int32 / -1) by
// round-tripping each via OfflinePointReader.
func TestOfflinePointWriter_DocIDEdgeCases(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	w, dir := newOfflineWriterFixture(t, cfg, 0)
	defer dir.Close()

	docIDs := []int32{0, 1, -1, 0x7FFFFFFF, -0x80000000}
	packed := make([][]byte, len(docIDs))
	for i, id := range docIDs {
		packed[i] = buildPackedValue(cfg.PackedBytesLength(), byte(i+1))
		if err := w.Append(packed[i], int(id)); err != nil {
			t.Fatalf("Append(%d, %d): %v", i, id, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	pr, err := w.GetReader(0, int64(len(docIDs)))
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	defer pr.Close()

	for i, want := range docIDs {
		ok, err := pr.Next()
		if err != nil || !ok {
			t.Fatalf("Next(%d): ok=%v err=%v", i, ok, err)
		}
		if got := int32(pr.PointValue().DocID()); got != want {
			t.Errorf("docID[%d]: got %d want %d", i, got, want)
		}
	}
}

// noTempDirectory is a stub Directory implementation that
// intentionally omits CreateTempOutput. Only the methods used by
// NewOfflinePointWriter need real behaviour; the rest panic if the
// writer ever calls them, which surfaces accidental dependencies.
type noTempDirectory struct{}

func (noTempDirectory) ListAll() ([]string, error)       { return nil, nil }
func (noTempDirectory) FileExists(string) bool           { return false }
func (noTempDirectory) FileLength(string) (int64, error) { return 0, nil }
func (noTempDirectory) OpenInput(string, store.IOContext) (store.IndexInput, error) {
	return nil, errors.New("OpenInput not supported")
}
func (noTempDirectory) CreateOutput(string, store.IOContext) (store.IndexOutput, error) {
	return nil, errors.New("CreateOutput not supported")
}
func (noTempDirectory) DeleteFile(string) error { return nil }
func (noTempDirectory) ObtainLock(string) (store.Lock, error) {
	return nil, errors.New("locking not supported")
}
func (noTempDirectory) Close() error                  { return nil }
func (noTempDirectory) GetDirectory() store.Directory { return nil }
