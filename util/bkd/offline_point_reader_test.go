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
)

// writePointsFile produces a synthetic OfflinePointWriter-compatible
// file inside an in-memory directory. Each point is encoded as
// packedValue || docID (big-endian uint32), exactly mirroring the
// layout produced by OfflinePointWriter.append; a 16-byte codec footer
// closes the file.
func writePointsFile(t *testing.T, cfg BKDConfig, dir *store.ByteBuffersDirectory, name string, packedValues [][]byte, docIDs []int32) {
	t.Helper()
	if len(packedValues) != len(docIDs) {
		t.Fatalf("test setup: %d packed values vs %d docIDs", len(packedValues), len(docIDs))
	}
	raw, err := dir.CreateOutput(name, store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	out := store.NewChecksumIndexOutput(raw)
	packedBytes := cfg.PackedBytesLength()
	for i, packed := range packedValues {
		if len(packed) != packedBytes {
			t.Fatalf("packedValues[%d]: len=%d want=%d", i, len(packed), packedBytes)
		}
		if err := out.WriteBytes(packed); err != nil {
			t.Fatalf("WriteBytes (packed[%d]): %v", i, err)
		}
		// docID is stored as 4 big-endian bytes; emit them via a raw
		// byte write to bypass any endianness divergence between
		// IndexOutput implementations.
		var docBuf [4]byte
		binary.BigEndian.PutUint32(docBuf[:], uint32(docIDs[i]))
		if err := out.WriteBytes(docBuf[:]); err != nil {
			t.Fatalf("WriteBytes (docID[%d]): %v", i, err)
		}
	}
	if err := codecs.WriteFooter(out); err != nil {
		t.Fatalf("WriteFooter: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close output: %v", err)
	}
}

func makePackedValues(cfg BKDConfig, n int, seed byte) [][]byte {
	values := make([][]byte, n)
	pb := cfg.PackedBytesLength()
	for i := 0; i < n; i++ {
		b := make([]byte, pb)
		for j := 0; j < pb; j++ {
			b[j] = byte(i+1)*seed + byte(j)
		}
		values[i] = b
	}
	return values
}

func makeDocIDs(n int) []int32 {
	ids := make([]int32, n)
	for i := 0; i < n; i++ {
		ids[i] = int32(i * 7)
	}
	return ids
}

// TestOfflinePointReader_ReadsAllPointsWithChecksum exercises the
// happy path: a full-file read uses ChecksumIndexInput and verifies
// the footer on Close.
func TestOfflinePointReader_ReadsAllPointsWithChecksum(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	const n = 8
	packed := makePackedValues(cfg, n, 13)
	docIDs := makeDocIDs(n)
	writePointsFile(t, cfg, dir, "points.bkd", packed, docIDs)

	buf := make([]byte, cfg.BytesPerDoc()*3) // forces multiple refills
	r, err := NewOfflinePointReader(cfg, dir, "points.bkd", 0, int64(n), buf)
	if err != nil {
		t.Fatalf("NewOfflinePointReader: %v", err)
	}
	if r.checksumIn == nil {
		t.Fatalf("expected ChecksumIndexInput for full-file read")
	}

	gotIDs := make([]int32, 0, n)
	gotPacked := make([][]byte, 0, n)
	for {
		ok, err := r.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if !ok {
			break
		}
		pv := r.PointValue()
		gotIDs = append(gotIDs, int32(pv.DocID()))
		ref := pv.PackedValue()
		copied := make([]byte, ref.Length)
		copy(copied, ref.Bytes[ref.Offset:ref.Offset+ref.Length])
		gotPacked = append(gotPacked, copied)
	}
	if len(gotIDs) != n {
		t.Fatalf("iterated %d points; want %d", len(gotIDs), n)
	}
	for i := 0; i < n; i++ {
		if gotIDs[i] != docIDs[i] {
			t.Errorf("docID[%d]: got %d want %d", i, gotIDs[i], docIDs[i])
		}
		if string(gotPacked[i]) != string(packed[i]) {
			t.Errorf("packed[%d]: got %x want %x", i, gotPacked[i], packed[i])
		}
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !r.checked {
		t.Fatalf("expected checksum to have been verified on Close")
	}
}

// TestOfflinePointReader_PartialRangeBypassesChecksum confirms that a
// non-full read opens via plain IndexInput and never engages the
// checksum path.
func TestOfflinePointReader_PartialRangeBypassesChecksum(t *testing.T) {
	cfg, err := Of(2, 2, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	const n = 10
	packed := makePackedValues(cfg, n, 5)
	docIDs := makeDocIDs(n)
	writePointsFile(t, cfg, dir, "points.bkd", packed, docIDs)

	// Read the middle five points [3, 8).
	const start = int64(3)
	const length = int64(5)
	buf := make([]byte, cfg.BytesPerDoc())
	r, err := NewOfflinePointReader(cfg, dir, "points.bkd", start, length, buf)
	if err != nil {
		t.Fatalf("NewOfflinePointReader: %v", err)
	}
	if r.checksumIn != nil {
		t.Fatalf("partial range should not engage ChecksumIndexInput")
	}

	gotIDs := make([]int32, 0, length)
	for {
		ok, err := r.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if !ok {
			break
		}
		gotIDs = append(gotIDs, int32(r.PointValue().DocID()))
	}
	wantIDs := docIDs[start : start+length]
	if len(gotIDs) != len(wantIDs) {
		t.Fatalf("iterated %d; want %d", len(gotIDs), len(wantIDs))
	}
	for i, id := range gotIDs {
		if id != wantIDs[i] {
			t.Errorf("docID[%d]: got %d want %d", i, id, wantIDs[i])
		}
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if r.checked {
		t.Fatalf("partial range should never set checked=true")
	}
}

// TestOfflinePointReader_EmptyRange ensures that an explicitly empty
// range terminates immediately without reading any bytes.
func TestOfflinePointReader_EmptyRange(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	writePointsFile(t, cfg, dir, "points.bkd",
		makePackedValues(cfg, 4, 1), makeDocIDs(4))

	buf := make([]byte, cfg.BytesPerDoc())
	r, err := NewOfflinePointReader(cfg, dir, "points.bkd", 2, 0, buf)
	if err != nil {
		t.Fatalf("NewOfflinePointReader: %v", err)
	}
	ok, err := r.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if ok {
		t.Fatalf("Next on empty range: got true want false")
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestOfflinePointReader_BufferSizingControlsRefill verifies that a
// buffer holding a single point still drives the multi-refill code
// path correctly.
func TestOfflinePointReader_BufferSizingControlsRefill(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	const n = 6
	packed := makePackedValues(cfg, n, 17)
	docIDs := makeDocIDs(n)
	writePointsFile(t, cfg, dir, "points.bkd", packed, docIDs)

	// Buffer holds exactly one point.
	buf := make([]byte, cfg.BytesPerDoc())
	r, err := NewOfflinePointReader(cfg, dir, "points.bkd", 0, int64(n), buf)
	if err != nil {
		t.Fatalf("NewOfflinePointReader: %v", err)
	}
	defer r.Close()

	for i := 0; i < n; i++ {
		ok, err := r.Next()
		if err != nil {
			t.Fatalf("Next(%d): %v", i, err)
		}
		if !ok {
			t.Fatalf("Next(%d): unexpected end of iteration", i)
		}
		if got := int32(r.PointValue().DocID()); got != docIDs[i] {
			t.Errorf("docID[%d]: got %d want %d", i, got, docIDs[i])
		}
	}
	ok, err := r.Next()
	if err != nil {
		t.Fatalf("trailing Next: %v", err)
	}
	if ok {
		t.Fatalf("trailing Next: got true want false")
	}
}

// TestOfflinePointReader_RejectsSliceBeyondFile ensures the
// constructor enforces the bounds check against the actual file
// length.
func TestOfflinePointReader_RejectsSliceBeyondFile(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	const n = 4
	writePointsFile(t, cfg, dir, "points.bkd",
		makePackedValues(cfg, n, 1), makeDocIDs(n))

	buf := make([]byte, cfg.BytesPerDoc())
	_, err = NewOfflinePointReader(cfg, dir, "points.bkd", 0, int64(n+1), buf)
	if err == nil {
		t.Fatalf("expected error for slice beyond file length")
	}
	if !strings.Contains(err.Error(), "beyond the length of this file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestOfflinePointReader_RejectsNilBuffer covers the
// reusableBuffer == null contract.
func TestOfflinePointReader_RejectsNilBuffer(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	writePointsFile(t, cfg, dir, "points.bkd",
		makePackedValues(cfg, 2, 1), makeDocIDs(2))

	_, err = NewOfflinePointReader(cfg, dir, "points.bkd", 0, 2, nil)
	if err == nil {
		t.Fatalf("expected error for nil reusableBuffer")
	}
	if !strings.Contains(err.Error(), "cannot be null") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestOfflinePointReader_RejectsUndersizedBuffer covers the contract
// that the reusable buffer must accommodate at least one point.
func TestOfflinePointReader_RejectsUndersizedBuffer(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	writePointsFile(t, cfg, dir, "points.bkd",
		makePackedValues(cfg, 2, 1), makeDocIDs(2))

	tooSmall := make([]byte, cfg.BytesPerDoc()-1)
	_, err = NewOfflinePointReader(cfg, dir, "points.bkd", 0, 2, tooSmall)
	if err == nil {
		t.Fatalf("expected error for undersized buffer")
	}
	if !strings.Contains(err.Error(), "must be bigger than") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestOfflinePointReader_RejectsNegativeRange enforces non-negative
// start / length.
func TestOfflinePointReader_RejectsNegativeRange(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	writePointsFile(t, cfg, dir, "points.bkd",
		makePackedValues(cfg, 2, 1), makeDocIDs(2))

	buf := make([]byte, cfg.BytesPerDoc())
	if _, err := NewOfflinePointReader(cfg, dir, "points.bkd", -1, 2, buf); err == nil {
		t.Fatalf("expected error for negative start")
	}
	if _, err := NewOfflinePointReader(cfg, dir, "points.bkd", 0, -1, buf); err == nil {
		t.Fatalf("expected error for negative length")
	}
}

// TestOfflinePointReader_CloseRejectsReuse ensures that Next after
// Close surfaces ErrOfflinePointReaderClosed and that Close is
// idempotent.
func TestOfflinePointReader_CloseRejectsReuse(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	writePointsFile(t, cfg, dir, "points.bkd",
		makePackedValues(cfg, 2, 1), makeDocIDs(2))

	buf := make([]byte, cfg.BytesPerDoc())
	r, err := NewOfflinePointReader(cfg, dir, "points.bkd", 0, 2, buf)
	if err != nil {
		t.Fatalf("NewOfflinePointReader: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Second Close is a no-op.
	if err := r.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	ok, err := r.Next()
	if ok {
		t.Fatalf("Next after Close: got true want false")
	}
	if !errors.Is(err, ErrOfflinePointReaderClosed) {
		t.Fatalf("Next after Close: err=%v want %v", err, ErrOfflinePointReaderClosed)
	}
}

// TestOfflinePointReader_PartialRangeSkipsChecksumOnEarlyClose
// verifies that closing before draining a full-file read does not
// engage the footer verification (countLeft != 0 path).
func TestOfflinePointReader_PartialRangeSkipsChecksumOnEarlyClose(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	const n = 4
	writePointsFile(t, cfg, dir, "points.bkd",
		makePackedValues(cfg, n, 11), makeDocIDs(n))

	buf := make([]byte, cfg.BytesPerDoc())
	r, err := NewOfflinePointReader(cfg, dir, "points.bkd", 0, int64(n), buf)
	if err != nil {
		t.Fatalf("NewOfflinePointReader: %v", err)
	}
	if r.checksumIn == nil {
		t.Fatalf("expected ChecksumIndexInput for full-file read")
	}
	// Read just one point and bail.
	ok, err := r.Next()
	if err != nil || !ok {
		t.Fatalf("Next: ok=%v err=%v", ok, err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if r.checked {
		t.Fatalf("Close must not verify checksum when iterator was not drained")
	}
}

// TestOfflinePointReader_ReturnsErrorOnMissingFile ensures the
// constructor surfaces directory errors.
func TestOfflinePointReader_ReturnsErrorOnMissingFile(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	buf := make([]byte, cfg.BytesPerDoc())
	_, err = NewOfflinePointReader(cfg, dir, "missing.bkd", 0, 1, buf)
	if err == nil {
		t.Fatalf("expected error for missing file")
	}
}

// TestOfflinePointReader_DetectsFooterCorruption verifies that the
// footer-verification path on Close propagates a checksum mismatch
// when the file is tampered with after writing.
func TestOfflinePointReader_DetectsFooterCorruption(t *testing.T) {
	cfg, err := Of(1, 1, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	const n = 3
	packed := makePackedValues(cfg, n, 9)
	docIDs := makeDocIDs(n)
	writePointsFile(t, cfg, dir, "points.bkd", packed, docIDs)

	// Tamper with the very first point byte. Re-create the file with
	// the corrupted prefix so the in-memory file content is updated.
	corrupted := make([]byte, 0, cfg.BytesPerDoc()*n+16)
	for i := 0; i < n; i++ {
		buf := make([]byte, cfg.BytesPerDoc())
		copy(buf, packed[i])
		binary.BigEndian.PutUint32(buf[cfg.PackedBytesLength():], uint32(docIDs[i]))
		corrupted = append(corrupted, buf...)
	}
	// Flip a single bit in the first packed value.
	corrupted[0] ^= 0xFF

	// Re-write the file with the corrupted bytes but a valid (over
	// the now-corrupted bytes) checksum footer. The checksum captured
	// by the writer matches the corrupted bytes, so reading back must
	// succeed on the data path but fail when we deliberately corrupt
	// the checksum itself.
	if err := dir.DeleteFile("points.bkd"); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	raw, err := dir.CreateOutput("points.bkd", store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	out := store.NewChecksumIndexOutput(raw)
	if err := out.WriteBytes(corrupted); err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}
	if err := codecs.WriteFooter(out); err != nil {
		t.Fatalf("WriteFooter: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close output: %v", err)
	}
	// Surgical bit flip in the trailing CRC bytes (the last byte of
	// the file) so the footer no longer matches its content.
	fileLength, err := dir.FileLength("points.bkd")
	if err != nil {
		t.Fatalf("FileLength: %v", err)
	}
	in, err := dir.OpenInput("points.bkd", store.IOContextReadOnce)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	contents := make([]byte, fileLength)
	if err := in.ReadBytes(contents); err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	if err := in.Close(); err != nil {
		t.Fatalf("Close input: %v", err)
	}
	contents[fileLength-1] ^= 0x01
	if err := dir.DeleteFile("points.bkd"); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	out2raw, err := dir.CreateOutput("points.bkd", store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := out2raw.WriteBytes(contents); err != nil {
		t.Fatalf("WriteBytes (corrupted): %v", err)
	}
	if err := out2raw.Close(); err != nil {
		t.Fatalf("Close (corrupted): %v", err)
	}

	buf := make([]byte, cfg.BytesPerDoc())
	r, err := NewOfflinePointReader(cfg, dir, "points.bkd", 0, int64(n), buf)
	if err != nil {
		t.Fatalf("NewOfflinePointReader: %v", err)
	}
	// Drain the iterator.
	for {
		ok, err := r.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if !ok {
			break
		}
		_ = r.PointValue()
	}
	closeErr := r.Close()
	if closeErr == nil {
		t.Fatalf("Close: expected checksum error, got nil")
	}
}

// TestOfflinePointReader_PointValueViewIsReusable confirms that the
// PointValue handle is the same pointer across iterations and that
// PackedValueDocIDBytes spans BytesPerDoc bytes.
func TestOfflinePointReader_PointValueViewIsReusable(t *testing.T) {
	cfg, err := Of(2, 2, 4, DefaultMaxPointsInLeafNode)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	const n = 4
	packed := makePackedValues(cfg, n, 3)
	docIDs := makeDocIDs(n)
	writePointsFile(t, cfg, dir, "points.bkd", packed, docIDs)

	buf := make([]byte, cfg.BytesPerDoc()*2)
	r, err := NewOfflinePointReader(cfg, dir, "points.bkd", 0, int64(n), buf)
	if err != nil {
		t.Fatalf("NewOfflinePointReader: %v", err)
	}
	defer r.Close()

	var firstPV PointValue
	for i := 0; i < n; i++ {
		ok, err := r.Next()
		if err != nil || !ok {
			t.Fatalf("Next(%d): ok=%v err=%v", i, ok, err)
		}
		pv := r.PointValue()
		if i == 0 {
			firstPV = pv
		} else if pv != firstPV {
			t.Fatalf("PointValue is not reused across iterations")
		}
		combined := pv.PackedValueDocIDBytes()
		if combined.Length != cfg.BytesPerDoc() {
			t.Errorf("combined.Length=%d want %d", combined.Length, cfg.BytesPerDoc())
		}
		// Reassemble what we expect to find in the combined slice.
		want := make([]byte, cfg.BytesPerDoc())
		copy(want, packed[i])
		binary.BigEndian.PutUint32(want[cfg.PackedBytesLength():], uint32(docIDs[i]))
		got := combined.Bytes[combined.Offset : combined.Offset+combined.Length]
		if string(got) != string(want) {
			t.Errorf("combined[%d]: got %x want %x", i, got, want)
		}
	}
}
