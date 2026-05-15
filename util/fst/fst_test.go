// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package fst

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestFSTGetFirstArcNoEmpty builds an FST whose metadata has no
// empty-output, and verifies that GetFirstArc produces an arc with
// BIT_LAST_ARC set (and BIT_FINAL_ARC clear).
func TestFSTGetFirstArcNoEmpty(t *testing.T) {
	m := NewFSTMetadata[*util.BytesRef](
		InputTypeByte1, ByteSequenceOutputs(), nil, false, 0, VERSION_CURRENT, 0,
	)
	store := NewOnHeapFSTStoreFromBytes(nil)
	f, err := NewFSTFromReader[*util.BytesRef](m, store)
	if err != nil {
		t.Fatalf("NewFSTFromReader: %v", err)
	}
	var arc Arc[*util.BytesRef]
	got := f.GetFirstArc(&arc)
	if got != &arc {
		t.Fatalf("GetFirstArc must return its argument")
	}
	if !got.IsLast() {
		t.Fatalf("IsLast: got false want true")
	}
	if got.IsFinal() {
		t.Fatalf("IsFinal: got true want false")
	}
	if got.Target() != 0 {
		t.Fatalf("Target: got %d want 0", got.Target())
	}
	if got.Output() != ByteSequenceOutputs().GetNoOutput() {
		t.Fatalf("Output must be NoOutput singleton")
	}
}

// TestFSTGetFirstArcWithEmptyOutput verifies the empty-output branch
// of GetFirstArc: both BIT_FINAL_ARC and BIT_LAST_ARC must be set, and
// BIT_ARC_HAS_FINAL_OUTPUT iff the empty output differs from NO_OUTPUT.
func TestFSTGetFirstArcWithEmptyOutput(t *testing.T) {
	empty := &util.BytesRef{Bytes: []byte{0x01, 0x02}, Offset: 0, Length: 2}
	m := NewFSTMetadata[*util.BytesRef](
		InputTypeByte1, ByteSequenceOutputs(), empty, true, 0, VERSION_CURRENT, 0,
	)
	storeImpl := NewOnHeapFSTStoreFromBytes(nil)
	f, err := NewFSTFromReader[*util.BytesRef](m, storeImpl)
	if err != nil {
		t.Fatalf("NewFSTFromReader: %v", err)
	}
	var arc Arc[*util.BytesRef]
	f.GetFirstArc(&arc)
	if !arc.IsFinal() {
		t.Fatalf("IsFinal: got false want true")
	}
	if !arc.IsLast() {
		t.Fatalf("IsLast: got false want true")
	}
	if !arc.flag(BIT_ARC_HAS_FINAL_OUTPUT) {
		t.Fatalf("BIT_ARC_HAS_FINAL_OUTPUT must be set when emptyOutput != NoOutput")
	}
	if arc.NextFinalOutput() != empty {
		t.Fatalf("NextFinalOutput must be the empty-output value")
	}
}

// TestFSTSaveRoundtrip emits an FST through Save and parses the
// metadata + byte stream back; the recovered FST must have the same
// metadata and bytes as the original.
func TestFSTSaveRoundtrip(t *testing.T) {
	body := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	m := NewFSTMetadata[int64](
		InputTypeByte1, PositiveIntOutputs(), 0, false, 3, VERSION_CURRENT, int64(len(body)),
	)
	s := NewOnHeapFSTStoreFromBytes(body)
	f, err := NewFSTFromReader[int64](m, s)
	if err != nil {
		t.Fatalf("NewFSTFromReader: %v", err)
	}

	meta := store.NewByteArrayDataOutput(64)
	bodyOut := store.NewByteArrayDataOutput(int(m.numBytes))
	if err := f.Save(meta, bodyOut); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Read metadata then body back.
	metaIn := store.NewByteArrayDataInput(meta.GetBytes())
	gotMeta, err := ReadMetadata[int64](metaIn, PositiveIntOutputs())
	if err != nil {
		t.Fatalf("ReadMetadata: %v", err)
	}
	if gotMeta.InputType() != InputTypeByte1 ||
		gotMeta.StartNode() != 3 ||
		gotMeta.NumBytes() != int64(len(body)) {
		t.Fatalf("metadata mismatch after roundtrip: %+v", gotMeta)
	}
	bodyIn := store.NewByteArrayDataInput(bodyOut.GetBytes())
	got, err := NewFSTFromDataInput[int64](gotMeta, bodyIn)
	if err != nil {
		t.Fatalf("NewFSTFromDataInput: %v", err)
	}
	if got.NumBytes() != int64(len(body)) {
		t.Fatalf("NumBytes mismatch")
	}
	r := got.GetBytesReader()
	// First reverse read returns the last byte of body.
	b, err := r.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte: %v", err)
	}
	if b != 0xEF {
		t.Fatalf("first reverse byte: got 0x%02x want 0xEF", b)
	}
}

// TestFromFSTReaderNilMetadata mirrors Lucene's FST.fromFSTReader
// returning null for nil metadata.
func TestFromFSTReaderNilMetadata(t *testing.T) {
	got, err := FromFSTReader[*util.BytesRef](nil, NewOnHeapFSTStoreFromBytes(nil))
	if err != nil {
		t.Fatalf("FromFSTReader: %v", err)
	}
	if got != nil {
		t.Fatalf("FromFSTReader(nil, ...): got %v want nil", got)
	}
}

// TestFromFSTReaderNilReader checks that a nil reader is rejected.
func TestFromFSTReaderNilReader(t *testing.T) {
	m := NewFSTMetadata[*util.BytesRef](
		InputTypeByte1, ByteSequenceOutputs(), nil, false, 0, VERSION_CURRENT, 0,
	)
	if _, err := FromFSTReader[*util.BytesRef](m, nil); err == nil {
		t.Fatal("FromFSTReader: expected error for nil reader")
	}
}

// TestFSTNewFromDataInputNumBytesValidation: NewFSTFromDataInput must
// reject a metadata whose numBytes exceeds what the DataInput can
// supply.
func TestFSTNewFromDataInputNumBytesValidation(t *testing.T) {
	m := NewFSTMetadata[*util.BytesRef](
		InputTypeByte1, ByteSequenceOutputs(), nil, false, 0, VERSION_CURRENT, 16,
	)
	in := store.NewByteArrayDataInput([]byte{1, 2, 3})
	if _, err := NewFSTFromDataInput[*util.BytesRef](m, in); err == nil {
		t.Fatal("expected an error when DataInput is shorter than numBytes")
	}
}

// TestFSTReadLabelByte1 / Byte4: ReadLabel correctness for both
// single-byte and VInt-encoded labels.
func TestFSTReadLabelByte1(t *testing.T) {
	m := NewFSTMetadata[*util.BytesRef](
		InputTypeByte1, ByteSequenceOutputs(), nil, false, 0, VERSION_CURRENT, 0,
	)
	f, _ := NewFSTFromReader[*util.BytesRef](m, NewOnHeapFSTStoreFromBytes(nil))
	in := store.NewByteArrayDataInput([]byte{0x7F, 0xFF})
	for _, want := range []int{0x7F, 0xFF} {
		got, err := f.ReadLabel(in)
		if err != nil {
			t.Fatalf("ReadLabel: %v", err)
		}
		if got != want {
			t.Fatalf("ReadLabel: got 0x%X want 0x%X", got, want)
		}
	}
}

func TestFSTReadLabelByte4(t *testing.T) {
	m := NewFSTMetadata[*util.BytesRef](
		InputTypeByte4, ByteSequenceOutputs(), nil, false, 0, VERSION_CURRENT, 0,
	)
	f, _ := NewFSTFromReader[*util.BytesRef](m, NewOnHeapFSTStoreFromBytes(nil))
	// VInt 0x1234 = 0xB4 0x24.
	buf := store.NewByteArrayDataOutput(8)
	if err := store.WriteVInt(buf, 0x1234); err != nil {
		t.Fatal(err)
	}
	in := store.NewByteArrayDataInput(buf.GetBytes())
	got, err := f.ReadLabel(in)
	if err != nil {
		t.Fatalf("ReadLabel: %v", err)
	}
	if got != 0x1234 {
		t.Fatalf("ReadLabel: got 0x%X want 0x1234", got)
	}
}

// TestTargetHasArcs covers the small static helper used by readers.
func TestTargetHasArcs(t *testing.T) {
	var a Arc[*util.BytesRef]
	a.target = 0
	if TargetHasArcs(&a) {
		t.Fatalf("target==0: want false")
	}
	a.target = 5
	if !TargetHasArcs(&a) {
		t.Fatalf("target>0: want true")
	}
	a.target = -1 // FinalEndNode
	if TargetHasArcs(&a) {
		t.Fatalf("FinalEndNode: want false")
	}
}

// TestGetNumPresenceBytes covers the static helper from FST.java.
func TestGetNumPresenceBytes(t *testing.T) {
	cases := []struct {
		labelRange, want int
	}{
		{0, 0}, {1, 1}, {7, 1}, {8, 1}, {9, 2},
		{15, 2}, {16, 2}, {17, 3}, {64, 8}, {65, 9},
	}
	for _, c := range cases {
		if got := getNumPresenceBytes(c.labelRange); got != c.want {
			t.Errorf("labelRange=%d: got %d want %d", c.labelRange, got, c.want)
		}
	}
}
