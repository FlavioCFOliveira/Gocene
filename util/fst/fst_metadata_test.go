// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package fst

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestFSTConstants pins the on-disk constant values to the Lucene
// 10.4.0 reference, guarding against accidental drift.
func TestFSTConstants(t *testing.T) {
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"BIT_FINAL_ARC", BIT_FINAL_ARC, 1},
		{"BIT_LAST_ARC", BIT_LAST_ARC, 2},
		{"BIT_TARGET_NEXT", BIT_TARGET_NEXT, 4},
		{"BIT_STOP_NODE", BIT_STOP_NODE, 8},
		{"BIT_ARC_HAS_OUTPUT", BIT_ARC_HAS_OUTPUT, 16},
		{"BIT_ARC_HAS_FINAL_OUTPUT", BIT_ARC_HAS_FINAL_OUTPUT, 32},
		{"ARCS_FOR_BINARY_SEARCH", int(ARCS_FOR_BINARY_SEARCH), 32},
		{"ARCS_FOR_DIRECT_ADDRESSING", int(ARCS_FOR_DIRECT_ADDRESSING), 64},
		{"ARCS_FOR_CONTINUOUS", int(ARCS_FOR_CONTINUOUS), 96},
		{"VERSION_START", VERSION_START, 6},
		{"VERSION_CONTINUOUS_ARCS", VERSION_CONTINUOUS_ARCS, 9},
		{"VERSION_CURRENT", VERSION_CURRENT, 9},
		{"VERSION_90", VERSION_90, 8},
		{"END_LABEL", END_LABEL, -1},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %d want %d", c.name, c.got, c.want)
		}
	}
	if FinalEndNode != -1 {
		t.Errorf("FinalEndNode: got %d want -1", FinalEndNode)
	}
	if NonFinalEndNode != 0 {
		t.Errorf("NonFinalEndNode: got %d want 0", NonFinalEndNode)
	}
}

// TestFSTMetadataInputTypeRoundTrip exercises Save / ReadMetadata for
// each INPUT_TYPE, with no empty output. The serialized form is
// reproduced verbatim and compared byte-for-byte.
func TestFSTMetadataInputTypeRoundTrip(t *testing.T) {
	for _, it := range []InputType{InputTypeByte1, InputTypeByte2, InputTypeByte4} {
		t.Run(it.String(), func(t *testing.T) {
			m := NewFSTMetadata[*util.BytesRef](
				it, ByteSequenceOutputs(), nil, false, 17, VERSION_CURRENT, 1234,
			)
			buf := store.NewByteArrayDataOutput(64)
			if err := m.Save(buf); err != nil {
				t.Fatalf("Save: %v", err)
			}
			raw := buf.GetBytes()
			// Expected layout: 4 BE magic, VInt(3)+"FST", 4 BE version,
			// 1 byte zero (no empty), 1 byte input type, VLong startNode,
			// VLong numBytes.
			want := []byte{
				0x3F, 0xD7, 0x6C, 0x17, // magic
				0x03, 'F', 'S', 'T', // VInt(3) + name
				0x00, 0x00, 0x00, 0x09, // BE version 9
				0x00, // no empty output
			}
			switch it {
			case InputTypeByte1:
				want = append(want, 0x00)
			case InputTypeByte2:
				want = append(want, 0x01)
			case InputTypeByte4:
				want = append(want, 0x02)
			}
			// startNode = 17 fits in a single VLong byte.
			want = append(want, 17)
			// numBytes = 1234 in VLong: 1234 = 0b10011010010
			// → 1234 & 0x7F = 0x52, then 1234>>7 = 9 → 0x09 (no continuation).
			// First byte: 0x52 | 0x80 = 0xD2, second byte: 0x09.
			want = append(want, 0xD2, 0x09)
			if !bytes.Equal(raw, want) {
				t.Fatalf("serialized form: got % x want % x", raw, want)
			}

			// Round-trip through ReadMetadata.
			in := store.NewByteArrayDataInput(raw)
			got, err := ReadMetadata[*util.BytesRef](in, ByteSequenceOutputs())
			if err != nil {
				t.Fatalf("ReadMetadata: %v", err)
			}
			if got.InputType() != it {
				t.Fatalf("InputType: got %v want %v", got.InputType(), it)
			}
			if got.StartNode() != 17 {
				t.Fatalf("StartNode: got %d want 17", got.StartNode())
			}
			if got.NumBytes() != 1234 {
				t.Fatalf("NumBytes: got %d want 1234", got.NumBytes())
			}
			if got.HasEmptyOutput() {
				t.Fatalf("HasEmptyOutput: got true want false")
			}
			if got.Version() != VERSION_CURRENT {
				t.Fatalf("Version: got %d want %d", got.Version(), VERSION_CURRENT)
			}
		})
	}
}

// TestFSTMetadataWithEmptyOutputRoundTrip serialises a metadata block
// that has a non-empty empty-output value, then parses it back and
// compares the recovered value.
func TestFSTMetadataWithEmptyOutputRoundTrip(t *testing.T) {
	emptyOut := &util.BytesRef{Bytes: []byte("EOF"), Offset: 0, Length: 3}
	m := NewFSTMetadata[*util.BytesRef](
		InputTypeByte1, ByteSequenceOutputs(), emptyOut, true, 0, VERSION_CURRENT, 0,
	)
	buf := store.NewByteArrayDataOutput(64)
	if err := m.Save(buf); err != nil {
		t.Fatalf("Save: %v", err)
	}
	in := store.NewByteArrayDataInput(buf.GetBytes())
	got, err := ReadMetadata[*util.BytesRef](in, ByteSequenceOutputs())
	if err != nil {
		t.Fatalf("ReadMetadata: %v", err)
	}
	if !got.HasEmptyOutput() {
		t.Fatalf("HasEmptyOutput: got false want true")
	}
	rec := got.EmptyOutput()
	if rec.Length != 3 || string(rec.ValidBytes()) != "EOF" {
		t.Fatalf("EmptyOutput: got %q want %q", rec.String(), "EOF")
	}
}

// TestFSTMetadataRejectsBadMagic checks that ReadMetadata refuses a
// stream whose magic does not match CODEC_MAGIC.
func TestFSTMetadataRejectsBadMagic(t *testing.T) {
	in := store.NewByteArrayDataInput([]byte{0, 0, 0, 0, 1, 'F', 0, 0, 0, 9})
	if _, err := ReadMetadata[*util.BytesRef](in, ByteSequenceOutputs()); err == nil {
		t.Fatal("expected an error for invalid codec magic")
	}
}

// TestFSTMetadataRejectsBadVersion verifies that a too-new version is
// refused.
func TestFSTMetadataRejectsBadVersion(t *testing.T) {
	buf := store.NewByteArrayDataOutput(32)
	// Hand-write the header with version 99 (out of range).
	if err := writeBEInt32(buf, codecMagic); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteString(buf, "FST"); err != nil {
		t.Fatal(err)
	}
	if err := writeBEInt32(buf, 99); err != nil {
		t.Fatal(err)
	}
	in := store.NewByteArrayDataInput(buf.GetBytes())
	if _, err := ReadMetadata[*util.BytesRef](in, ByteSequenceOutputs()); err == nil {
		t.Fatal("expected an error for unsupported version")
	}
}
