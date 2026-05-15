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

func irOf(vals ...int) *util.IntsRef {
	cp := append([]int(nil), vals...)
	return &util.IntsRef{Ints: cp, Offset: 0, Length: len(cp)}
}

func intSliceEq(r *util.IntsRef, want ...int) bool {
	if r.Length != len(want) {
		return false
	}
	for i, v := range want {
		if r.Ints[r.Offset+i] != v {
			return false
		}
	}
	return true
}

func TestIntSequenceOutputsSingleton(t *testing.T) {
	if IntSequenceOutputs() != IntSequenceOutputs() {
		t.Fatal("singleton identity broken")
	}
}

func TestIntSequenceOutputsAlgebra(t *testing.T) {
	o := IntSequenceOutputs()
	if got := o.Common(irOf(1, 2, 3, 4), irOf(1, 2, 5)); !intSliceEq(got, 1, 2) {
		t.Fatalf("Common: want [1 2], got %v", got.Ints[got.Offset:got.Offset+got.Length])
	}
	if got := o.Add(irOf(1, 2), irOf(3, 4)); !intSliceEq(got, 1, 2, 3, 4) {
		t.Fatalf("Add: want [1 2 3 4], got %v", got.Ints[got.Offset:got.Offset+got.Length])
	}
	if got := o.Subtract(irOf(1, 2, 3, 4), irOf(1, 2)); !intSliceEq(got, 3, 4) {
		t.Fatalf("Subtract: want [3 4], got %v", got.Ints[got.Offset:got.Offset+got.Length])
	}
}

func TestIntSequenceOutputsRoundTrip(t *testing.T) {
	o := IntSequenceOutputs()
	cases := [][]int{{}, {7}, {1, 2, 3}, {0, 0, 0xFF, 0x1234}}
	for _, c := range cases {
		out := store.NewByteArrayDataOutput(64)
		ir := &util.IntsRef{Ints: append([]int(nil), c...), Offset: 0, Length: len(c)}
		if err := o.Write(ir, out); err != nil {
			t.Fatalf("Write: %v", err)
		}
		in := store.NewByteArrayDataInput(out.GetBytes())
		got, err := o.Read(in)
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		if !intSliceEq(got, c...) {
			t.Fatalf("round-trip: want %v got %v", c, got.Ints[got.Offset:got.Offset+got.Length])
		}
	}
}

func TestIntSequenceOutputsByteFormat(t *testing.T) {
	o := IntSequenceOutputs()
	out := store.NewByteArrayDataOutput(8)
	if err := o.Write(irOf(1, 2, 3), out); err != nil {
		t.Fatalf("Write: %v", err)
	}
	want := []byte{0x03, 0x01, 0x02, 0x03}
	if !bytes.Equal(out.GetBytes(), want) {
		t.Fatalf("byte format drift: want % x got % x", want, out.GetBytes())
	}
}
