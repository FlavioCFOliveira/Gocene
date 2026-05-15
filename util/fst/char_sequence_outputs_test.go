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

func crOf(s string) *util.CharsRef {
	r := []rune(s)
	return &util.CharsRef{Chars: r, Offset: 0, Length: len(r)}
}

func runeSliceEq(a *util.CharsRef, want string) bool {
	got := string(a.Chars[a.Offset : a.Offset+a.Length])
	return got == want
}

func TestCharSequenceOutputsSingleton(t *testing.T) {
	if CharSequenceOutputs() != CharSequenceOutputs() {
		t.Fatal("singleton identity broken")
	}
}

func TestCharSequenceOutputsCommon(t *testing.T) {
	o := CharSequenceOutputs()
	if got := o.Common(crOf("foobar"), crOf("food")); !runeSliceEq(got, "foo") {
		t.Fatalf("Common: want 'foo', got %q", got.String())
	}
	if got := o.Common(crOf("abc"), crOf("xyz")); got != o.GetNoOutput() {
		t.Fatalf("Common: no common prefix should return NO_OUTPUT singleton")
	}
}

func TestCharSequenceOutputsAddSubtract(t *testing.T) {
	o := CharSequenceOutputs()
	if got := o.Add(crOf("foo"), crOf("bar")); !runeSliceEq(got, "foobar") {
		t.Fatalf("Add: want 'foobar', got %q", got.String())
	}
	if got := o.Subtract(crOf("foobar"), crOf("foo")); !runeSliceEq(got, "bar") {
		t.Fatalf("Subtract: want 'bar', got %q", got.String())
	}
}

func TestCharSequenceOutputsRoundTrip(t *testing.T) {
	o := CharSequenceOutputs()
	cases := []string{"", "x", "hello", "abc-def-ghi"}
	for _, s := range cases {
		out := store.NewByteArrayDataOutput(64)
		if err := o.Write(crOf(s), out); err != nil {
			t.Fatalf("Write(%q): %v", s, err)
		}
		in := store.NewByteArrayDataInput(out.GetBytes())
		got, err := o.Read(in)
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		if !runeSliceEq(got, s) {
			t.Fatalf("round-trip: want %q got %q", s, got.String())
		}
	}
}

func TestCharSequenceOutputsByteFormat(t *testing.T) {
	// Encode "abc": length 3 then 3 VInts: 0x61, 0x62, 0x63.
	o := CharSequenceOutputs()
	out := store.NewByteArrayDataOutput(8)
	if err := o.Write(crOf("abc"), out); err != nil {
		t.Fatalf("Write: %v", err)
	}
	want := []byte{0x03, 0x61, 0x62, 0x63}
	if !bytes.Equal(out.GetBytes(), want) {
		t.Fatalf("byte format drift: want % x got % x", want, out.GetBytes())
	}
}

func TestCharSequenceOutputsSkip(t *testing.T) {
	o := CharSequenceOutputs()
	out := store.NewByteArrayDataOutput(16)
	if err := o.Write(crOf("payload"), out); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := out.WriteByte(0x55); err != nil {
		t.Fatalf("WriteByte: %v", err)
	}
	in := store.NewByteArrayDataInput(out.GetBytes())
	if err := o.SkipOutput(in); err != nil {
		t.Fatalf("SkipOutput: %v", err)
	}
	b, err := in.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte after skip: %v", err)
	}
	if b != 0x55 {
		t.Fatalf("post-skip: got 0x%02X want 0x55", b)
	}
}
