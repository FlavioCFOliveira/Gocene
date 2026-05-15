// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package fst

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestPositiveIntOutputsSingleton(t *testing.T) {
	if PositiveIntOutputs() != PositiveIntOutputs() {
		t.Fatal("singleton identity broken")
	}
}

func TestPositiveIntOutputsAlgebra(t *testing.T) {
	o := PositiveIntOutputs()

	if got := o.Common(3, 5); got != 3 {
		t.Fatalf("Common(3,5)=%d", got)
	}
	if got := o.Common(0, 5); got != 0 {
		t.Fatalf("Common(0,5)=%d; want 0 (no output)", got)
	}
	if got := o.Subtract(5, 2); got != 3 {
		t.Fatalf("Subtract(5,2)=%d", got)
	}
	if got := o.Subtract(5, 5); got != 0 {
		t.Fatalf("Subtract equal => 0; got %d", got)
	}
	if got := o.Subtract(5, 0); got != 5 {
		t.Fatalf("Subtract by no-output => unchanged; got %d", got)
	}
	if got := o.Add(2, 3); got != 5 {
		t.Fatalf("Add(2,3)=%d", got)
	}
	if got := o.Add(0, 5); got != 5 {
		t.Fatalf("Add(no-output, x) should be x; got %d", got)
	}
}

func TestPositiveIntOutputsRoundTrip(t *testing.T) {
	o := PositiveIntOutputs()
	cases := []int64{0, 1, 127, 128, 0xFFFF, 0x12345678}
	for _, c := range cases {
		out := store.NewByteArrayDataOutput(16)
		if err := o.Write(c, out); err != nil {
			t.Fatalf("Write(%d): %v", c, err)
		}
		in := store.NewByteArrayDataInput(out.GetBytes())
		got, err := o.Read(in)
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		if got != c {
			t.Fatalf("round-trip: want %d got %d", c, got)
		}
	}
}

func TestPositiveIntOutputsByteFormat(t *testing.T) {
	o := PositiveIntOutputs()
	// 128 (0x80) requires 2 VInt bytes: [0x80, 0x01].
	out := store.NewByteArrayDataOutput(8)
	if err := o.Write(128, out); err != nil {
		t.Fatalf("Write: %v", err)
	}
	want := []byte{0x80, 0x01}
	if !bytes.Equal(out.GetBytes(), want) {
		t.Fatalf("byte format drift for 128: want % x got % x", want, out.GetBytes())
	}

	// 0 fits in 1 byte: [0x00].
	out2 := store.NewByteArrayDataOutput(2)
	if err := o.Write(0, out2); err != nil {
		t.Fatalf("Write 0: %v", err)
	}
	if !bytes.Equal(out2.GetBytes(), []byte{0x00}) {
		t.Fatalf("byte format drift for 0: got % x", out2.GetBytes())
	}
}

func TestPositiveIntOutputsToString(t *testing.T) {
	o := PositiveIntOutputs()
	cases := map[int64]string{0: "0", 1: "1", 127: "127", 1000000: "1000000"}
	for v, want := range cases {
		if got := o.OutputToString(v); got != want {
			t.Fatalf("OutputToString(%d)=%q; want %q", v, got, want)
		}
	}
}
