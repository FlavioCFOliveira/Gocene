// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bkd

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// makePointWriter is a tiny helper used by every test to keep them
// short.
func makePointWriter(t *testing.T, numDims, numIndexDims, bytesPerDim, maxPoints, size int) (*HeapPointWriter, BKDConfig) {
	t.Helper()
	cfg, err := NewBKDConfig(numDims, numIndexDims, bytesPerDim, maxPoints)
	if err != nil {
		t.Fatalf("NewBKDConfig: %v", err)
	}
	return NewHeapPointWriter(cfg, size), cfg
}

// packedFor returns a synthetic packed value where dimension d gets the
// byte pattern d+1 over bytesPerDim bytes; useful as a deterministic
// fixture.
func packedFor(point int, cfg BKDConfig) []byte {
	out := make([]byte, cfg.PackedBytesLength())
	for d := 0; d < cfg.NumDims(); d++ {
		for b := 0; b < cfg.BytesPerDim(); b++ {
			out[d*cfg.BytesPerDim()+b] = byte((point*10 + d*7 + b) & 0xff)
		}
	}
	return out
}

func TestHeapPointWriter_AppendStoresPackedValueAndDocID(t *testing.T) {
	w, cfg := makePointWriter(t, 2, 2, 4, 64, 4)

	// Append 3 distinct points.
	for i := 0; i < 3; i++ {
		if err := w.Append(packedFor(i, cfg), 1000+i); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
	if w.Count() != 3 {
		t.Fatalf("Count: got %d want 3", w.Count())
	}

	// The reusable PointValue must surface what we stored.
	for i := 0; i < 3; i++ {
		pv := w.GetPackedValueSlice(i)
		wantPacked := packedFor(i, cfg)
		if !bytes.Equal(pv.PackedValue().ValidBytes(), wantPacked) {
			t.Fatalf("point %d packed mismatch: got %x want %x", i, pv.PackedValue().ValidBytes(), wantPacked)
		}
		if pv.DocID() != 1000+i {
			t.Fatalf("point %d docID: got %d want %d", i, pv.DocID(), 1000+i)
		}
		combo := pv.PackedValueDocIDBytes()
		if combo.Length != cfg.BytesPerDoc() {
			t.Fatalf("point %d combo.Length: got %d want %d", i, combo.Length, cfg.BytesPerDoc())
		}
		// The docID trailing bytes must be big-endian.
		gotDoc := int32(binary.BigEndian.Uint32(
			combo.Bytes[combo.Offset+cfg.PackedBytesLength() : combo.Offset+cfg.BytesPerDoc()],
		))
		if int(gotDoc) != 1000+i {
			t.Fatalf("point %d BE docID: got %d want %d", i, gotDoc, 1000+i)
		}
	}
}

func TestHeapPointWriter_AppendRejectsWrongLength(t *testing.T) {
	w, cfg := makePointWriter(t, 2, 2, 4, 64, 2)
	short := make([]byte, cfg.PackedBytesLength()-1)
	if err := w.Append(short, 0); err == nil {
		t.Fatalf("expected error for short packedValue, got nil")
	}
	long := make([]byte, cfg.PackedBytesLength()+1)
	if err := w.Append(long, 0); err == nil {
		t.Fatalf("expected error for long packedValue, got nil")
	}
	if w.Count() != 0 {
		t.Fatalf("Count: got %d want 0 (rejected appends must not advance)", w.Count())
	}
}

func TestHeapPointWriter_AppendRejectsOverflow(t *testing.T) {
	w, cfg := makePointWriter(t, 1, 1, 4, 64, 2)
	if err := w.Append(packedFor(0, cfg), 1); err != nil {
		t.Fatalf("Append 0: %v", err)
	}
	if err := w.Append(packedFor(1, cfg), 2); err != nil {
		t.Fatalf("Append 1: %v", err)
	}
	if err := w.Append(packedFor(2, cfg), 3); err == nil {
		t.Fatalf("expected overflow error, got nil")
	}
	if w.Count() != 2 {
		t.Fatalf("Count: got %d want 2", w.Count())
	}
}

func TestHeapPointWriter_AppendPointValueRoundTrip(t *testing.T) {
	src, cfg := makePointWriter(t, 3, 2, 3, 64, 3)
	for i := 0; i < 3; i++ {
		if err := src.Append(packedFor(i, cfg), 7000+i); err != nil {
			t.Fatalf("source Append %d: %v", i, err)
		}
	}
	dst := NewHeapPointWriter(cfg, 3)
	for i := 0; i < 3; i++ {
		if err := dst.AppendPointValue(src.GetPackedValueSlice(i)); err != nil {
			t.Fatalf("AppendPointValue %d: %v", i, err)
		}
	}
	for i := 0; i < 3; i++ {
		got := dst.GetPackedValueSlice(i)
		if got.DocID() != 7000+i {
			t.Fatalf("docID %d: got %d want %d", i, got.DocID(), 7000+i)
		}
		if !bytes.Equal(got.PackedValue().ValidBytes(), packedFor(i, cfg)) {
			t.Fatalf("packed value mismatch at %d", i)
		}
	}
}

func TestHeapPointWriter_AppendPointValueRejectsWrongLength(t *testing.T) {
	w, cfg := makePointWriter(t, 2, 2, 4, 64, 2)
	bogus := &stubPointValue{
		packed: &util.BytesRef{Bytes: make([]byte, cfg.PackedBytesLength()), Length: cfg.PackedBytesLength()},
		id:     1,
		combo:  &util.BytesRef{Bytes: make([]byte, cfg.BytesPerDoc()-1), Length: cfg.BytesPerDoc() - 1},
	}
	if err := w.AppendPointValue(bogus); err == nil {
		t.Fatalf("expected error for wrong combo length, got nil")
	}
	if w.Count() != 0 {
		t.Fatalf("Count: got %d want 0", w.Count())
	}
}

func TestHeapPointWriter_AppendAfterCloseReturnsError(t *testing.T) {
	w, cfg := makePointWriter(t, 1, 1, 4, 64, 4)
	if err := w.Append(packedFor(0, cfg), 1); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !w.IsClosed() {
		t.Fatalf("IsClosed: got false want true")
	}
	if err := w.Append(packedFor(1, cfg), 2); !errors.Is(err, ErrPointWriterClosed) {
		t.Fatalf("Append after close: got %v want ErrPointWriterClosed", err)
	}
	other := NewHeapPointWriter(cfg, 1)
	if err := other.Append(packedFor(0, cfg), 9); err != nil {
		t.Fatalf("seed other: %v", err)
	}
	if err := w.AppendPointValue(other.GetPackedValueSlice(0)); !errors.Is(err, ErrPointWriterClosed) {
		t.Fatalf("AppendPointValue after close: got %v want ErrPointWriterClosed", err)
	}
}

func TestHeapPointWriter_GetReaderRequiresClose(t *testing.T) {
	w, cfg := makePointWriter(t, 1, 1, 4, 64, 4)
	for i := 0; i < 4; i++ {
		if err := w.Append(packedFor(i, cfg), 100+i); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
	if _, err := w.GetReader(0, 4); err == nil {
		t.Fatalf("GetReader on open writer: expected error")
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	r, err := w.GetReader(1, 2)
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	var ids []int
	for {
		ok, err := r.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if !ok {
			break
		}
		ids = append(ids, r.PointValue().DocID())
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close reader: %v", err)
	}
	want := []int{101, 102}
	if len(ids) != len(want) {
		t.Fatalf("ids=%v want %v", ids, want)
	}
	for i := range ids {
		if ids[i] != want[i] {
			t.Fatalf("ids[%d]=%d want %d", i, ids[i], want[i])
		}
	}
}

func TestHeapPointWriter_GetReaderRejectsOutOfRange(t *testing.T) {
	w, cfg := makePointWriter(t, 1, 1, 4, 64, 3)
	for i := 0; i < 2; i++ {
		if err := w.Append(packedFor(i, cfg), 100+i); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// start+length > nextWrite => error.
	if _, err := w.GetReader(0, 3); err == nil {
		t.Fatalf("expected error when range > nextWrite")
	}
	// start+length > size => error.
	if _, err := w.GetReader(0, 4); err == nil {
		t.Fatalf("expected error when range > size")
	}
	// Negative inputs.
	if _, err := w.GetReader(-1, 1); err == nil {
		t.Fatalf("expected error for negative start")
	}
}

func TestHeapPointWriter_SwapExchangesSlots(t *testing.T) {
	w, cfg := makePointWriter(t, 2, 2, 3, 64, 4)
	for i := 0; i < 4; i++ {
		if err := w.Append(packedFor(i, cfg), 500+i); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	wantAt1 := append([]byte(nil), packedFor(1, cfg)...)
	wantAt3 := append([]byte(nil), packedFor(3, cfg)...)

	w.swap(1, 3)

	if got := w.GetPackedValueSlice(1); !bytes.Equal(got.PackedValue().ValidBytes(), wantAt3) {
		t.Fatalf("after swap, slot 1 packed: got %x want %x", got.PackedValue().ValidBytes(), wantAt3)
	}
	if got := w.GetPackedValueSlice(1); got.DocID() != 503 {
		t.Fatalf("after swap, slot 1 docID: got %d want 503", got.DocID())
	}
	if got := w.GetPackedValueSlice(3); !bytes.Equal(got.PackedValue().ValidBytes(), wantAt1) {
		t.Fatalf("after swap, slot 3 packed: got %x want %x", got.PackedValue().ValidBytes(), wantAt1)
	}
	if got := w.GetPackedValueSlice(3); got.DocID() != 501 {
		t.Fatalf("after swap, slot 3 docID: got %d want 501", got.DocID())
	}
	// Swap with itself must be a no-op.
	w.swap(2, 2)
	if got := w.GetPackedValueSlice(2); !bytes.Equal(got.PackedValue().ValidBytes(), packedFor(2, cfg)) {
		t.Fatalf("self-swap mutated slot 2")
	}
}

func TestHeapPointWriter_ByteAtAndCopyDim(t *testing.T) {
	w, cfg := makePointWriter(t, 2, 2, 3, 64, 2)
	for i := 0; i < 2; i++ {
		if err := w.Append(packedFor(i, cfg), 9000+i); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	expected := packedFor(1, cfg)
	for k := 0; k < cfg.PackedBytesLength(); k++ {
		got := w.byteAt(1, k)
		if got != int(expected[k]) {
			t.Fatalf("byteAt(1, %d): got %d want %d", k, got, expected[k])
		}
	}
	// copyDim: copy dim 1 of point 0 into a buffer.
	buf := make([]byte, cfg.BytesPerDim())
	w.copyDim(0, cfg.BytesPerDim() /* dim 1 starts here */, buf, 0)
	wantSlice := packedFor(0, cfg)[cfg.BytesPerDim() : 2*cfg.BytesPerDim()]
	if !bytes.Equal(buf, wantSlice) {
		t.Fatalf("copyDim mismatch: got %x want %x", buf, wantSlice)
	}
}

func TestHeapPointWriter_CopyDataDimsAndDoc(t *testing.T) {
	// 3 stored dims, 2 indexed: data-dim region = dim 2, then 4 docID bytes.
	w, cfg := makePointWriter(t, 3, 2, 2, 64, 1)
	packed := packedFor(0, cfg)
	if err := w.Append(packed, 0x01020304); err != nil {
		t.Fatalf("Append: %v", err)
	}
	dataDimsAndDocLength := cfg.BytesPerDoc() - cfg.PackedIndexBytesLength()
	if got := w.dataDimsAndDocLength; got != dataDimsAndDocLength {
		t.Fatalf("dataDimsAndDocLength: got %d want %d", got, dataDimsAndDocLength)
	}
	buf := make([]byte, dataDimsAndDocLength)
	w.copyDataDimsAndDoc(0, buf, 0)
	want := make([]byte, dataDimsAndDocLength)
	copy(want, packed[cfg.PackedIndexBytesLength():])
	binary.BigEndian.PutUint32(want[cfg.BytesPerDim():], 0x01020304)
	if !bytes.Equal(buf, want) {
		t.Fatalf("copyDataDimsAndDoc mismatch:\n got %x\nwant %x", buf, want)
	}
}

func TestHeapPointWriter_CompareDim(t *testing.T) {
	w, cfg := makePointWriter(t, 2, 2, 2, 64, 3)
	// Point 0: dim0=[01 02], dim1=[10 11]
	// Point 1: dim0=[01 02], dim1=[20 21] => dim0 equal, dim1 less for 0
	// Point 2: dim0=[00 02], dim1=[10 11] => dim0 less for 2
	pts := [][]byte{
		{0x01, 0x02, 0x10, 0x11},
		{0x01, 0x02, 0x20, 0x21},
		{0x00, 0x02, 0x10, 0x11},
	}
	for i, p := range pts {
		if err := w.Append(p, i); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	if got := w.compareDim(0, 1, 0); got != 0 {
		t.Fatalf("compareDim(0,1, dim0)=%d want 0", got)
	}
	if got := w.compareDim(0, 1, cfg.BytesPerDim()); got >= 0 {
		t.Fatalf("compareDim(0,1, dim1)=%d want <0", got)
	}
	if got := w.compareDim(2, 0, 0); got >= 0 {
		t.Fatalf("compareDim(2,0, dim0)=%d want <0", got)
	}
	// compareDimWithValue: compare a provided value to point 0 dim 0.
	val := []byte{0x99, 0x01, 0x02, 0x09}
	if got := w.compareDimWithValue(0, val, 1, 0); got != 0 {
		t.Fatalf("compareDimWithValue at offset 1 vs (0,dim0) = %d want 0", got)
	}
	bigger := []byte{0x02, 0x00}
	if got := w.compareDimWithValue(0, bigger, 0, 0); got <= 0 {
		t.Fatalf("compareDimWithValue bigger vs (0,dim0) = %d want >0", got)
	}
}

func TestHeapPointWriter_CompareDataDimsAndDoc(t *testing.T) {
	// numDims=3, numIndexDims=2, bytesPerDim=2 => data-dim region starts at byte 4.
	w, cfg := makePointWriter(t, 3, 2, 2, 64, 2)
	// Point 0: indexed dims + data-dim 0x10,0x11 + docID 0x00,0x00,0x00,0x07
	// Point 1: indexed dims + data-dim 0x10,0x11 + docID 0x00,0x00,0x00,0x08 (greater)
	p0 := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0x10, 0x11}
	p1 := []byte{0x00, 0x00, 0x00, 0x00, 0x10, 0x11}
	if err := w.Append(p0, 7); err != nil {
		t.Fatalf("Append p0: %v", err)
	}
	if err := w.Append(p1, 8); err != nil {
		t.Fatalf("Append p1: %v", err)
	}
	// Point 0 vs point 1: data-dim equal, docID 7 < 8 => negative.
	if got := w.compareDataDimsAndDoc(0, 1); got >= 0 {
		t.Fatalf("compareDataDimsAndDoc(0,1)=%d want <0", got)
	}
	if got := w.compareDataDimsAndDoc(1, 0); got <= 0 {
		t.Fatalf("compareDataDimsAndDoc(1,0)=%d want >0", got)
	}
	// compareDataDimsAndDocWithValue: build a matching reference and confirm zero.
	ref := make([]byte, cfg.BytesPerDoc()-cfg.PackedIndexBytesLength())
	copy(ref, p0[cfg.PackedIndexBytesLength():])
	binary.BigEndian.PutUint32(ref[cfg.BytesPerDim():], 7)
	if got := w.compareDataDimsAndDocWithValue(0, ref, 0); got != 0 {
		t.Fatalf("compareDataDimsAndDocWithValue: got %d want 0", got)
	}
}

func TestHeapPointWriter_ComputeCardinality(t *testing.T) {
	w, cfg := makePointWriter(t, 2, 2, 2, 64, 5)
	// Two identical points then three more variations.
	same := []byte{0x01, 0x02, 0x10, 0x20}
	pts := [][]byte{
		same,
		append([]byte(nil), same...),
		{0x01, 0x02, 0x10, 0x21}, // changes dim 1 last byte
		{0x01, 0x03, 0x10, 0x21}, // changes dim 0 last byte
		{0x01, 0x03, 0x10, 0x21}, // duplicate of previous
	}
	for i, p := range pts {
		if err := w.Append(p, i); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	commonPrefix := make([]int, cfg.NumDims())
	got := w.ComputeCardinality(0, 5, commonPrefix)
	if got != 3 {
		t.Fatalf("ComputeCardinality: got %d want 3", got)
	}
	// Subrange [0,2): the two equal points => cardinality 1.
	if g := w.ComputeCardinality(0, 2, commonPrefix); g != 1 {
		t.Fatalf("ComputeCardinality(0,2): got %d want 1", g)
	}
	// commonPrefix that covers the differing bytes => collapses cardinality.
	wide := []int{cfg.BytesPerDim(), cfg.BytesPerDim()}
	if g := w.ComputeCardinality(0, 5, wide); g != 1 {
		t.Fatalf("ComputeCardinality with full prefix: got %d want 1", g)
	}
}

func TestHeapPointWriter_PointValueIsReused(t *testing.T) {
	w, cfg := makePointWriter(t, 1, 1, 4, 64, 2)
	if err := w.Append(packedFor(0, cfg), 1); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := w.Append(packedFor(1, cfg), 2); err != nil {
		t.Fatalf("Append: %v", err)
	}
	a := w.GetPackedValueSlice(0)
	b := w.GetPackedValueSlice(1)
	// Lucene returns the same reusable instance — Go must mirror that.
	if a != b {
		t.Fatalf("GetPackedValueSlice must return the same reusable instance across calls")
	}
}

func TestHeapPointWriter_DestroyIsNoop(t *testing.T) {
	w, _ := makePointWriter(t, 1, 1, 4, 64, 1)
	if err := w.Destroy(); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := w.Destroy(); err != nil {
		t.Fatalf("Destroy after close: %v", err)
	}
}

func TestHeapPointWriter_ZeroSize(t *testing.T) {
	cfg, err := NewBKDConfig(1, 1, 4, 64)
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	w := NewHeapPointWriter(cfg, 0)
	if w.Count() != 0 {
		t.Fatalf("Count: got %d want 0", w.Count())
	}
	if err := w.Append(packedFor(0, cfg), 1); err == nil {
		t.Fatalf("Append on zero-size writer: expected error")
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	r, err := w.GetReader(0, 0)
	if err != nil {
		t.Fatalf("GetReader(0,0): %v", err)
	}
	if ok, _ := r.Next(); ok {
		t.Fatalf("empty reader Next: expected false")
	}
}

func TestHeapPointWriter_GetPackedValueSlicePanicsOutOfRange(t *testing.T) {
	w, cfg := makePointWriter(t, 1, 1, 4, 64, 4)
	if err := w.Append(packedFor(0, cfg), 0); err != nil {
		t.Fatalf("Append: %v", err)
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic for out-of-range GetPackedValueSlice")
		}
	}()
	_ = w.GetPackedValueSlice(1)
}

func TestHeapPointWriter_String(t *testing.T) {
	w, cfg := makePointWriter(t, 1, 1, 4, 64, 2)
	if err := w.Append(packedFor(0, cfg), 7); err != nil {
		t.Fatalf("Append: %v", err)
	}
	want := fmt.Sprintf("HeapPointWriter(count=%d size=%d)", 1, 2)
	if got := w.String(); got != want {
		t.Fatalf("String: got %q want %q", got, want)
	}
}

func TestHeapPointWriter_ImplementsPointWriter(t *testing.T) {
	cfg, err := NewBKDConfig(1, 1, 4, 64)
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	var _ PointWriter = NewHeapPointWriter(cfg, 1)
}

func TestHeapPointWriter_DocIDByteOrderMatchesLucene(t *testing.T) {
	// Lucene encodes docID as VH_BE_INT (big-endian 4 bytes). Verify
	// the raw byte slab carries that order, since BKD on-disk codec
	// downstream depends on it.
	w, cfg := makePointWriter(t, 1, 1, 4, 64, 1)
	const docID = 0x01020304
	if err := w.Append(packedFor(0, cfg), docID); err != nil {
		t.Fatalf("Append: %v", err)
	}
	combo := w.GetPackedValueSlice(0).PackedValueDocIDBytes()
	tail := combo.Bytes[combo.Offset+cfg.PackedBytesLength() : combo.Offset+cfg.BytesPerDoc()]
	want := []byte{0x01, 0x02, 0x03, 0x04}
	if !bytes.Equal(tail, want) {
		t.Fatalf("docID bytes: got %x want %x", tail, want)
	}
}
