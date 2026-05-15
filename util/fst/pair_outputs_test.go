// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package fst

import (
	"bytes"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

func TestPairOutputsAlgebra(t *testing.T) {
	p := NewPairOutputs[int64, int64](
		PositiveIntOutputs(),
		PositiveIntOutputs(),
	)

	a := p.NewPair(3, 10)
	b := p.NewPair(5, 4)

	if got := p.Common(a, b); got.Output1 != 3 || got.Output2 != 4 {
		t.Fatalf("Common: want (3,4) got (%d,%d)", got.Output1, got.Output2)
	}

	if got := p.Subtract(a, p.NewPair(1, 2)); got.Output1 != 2 || got.Output2 != 8 {
		t.Fatalf("Subtract: want (2,8) got (%d,%d)", got.Output1, got.Output2)
	}

	if got := p.Add(p.NewPair(1, 2), p.NewPair(3, 4)); got.Output1 != 4 || got.Output2 != 6 {
		t.Fatalf("Add: want (4,6) got (%d,%d)", got.Output1, got.Output2)
	}
}

func TestPairOutputsNoOutputIdentity(t *testing.T) {
	p := NewPairOutputs[int64, int64](
		PositiveIntOutputs(),
		PositiveIntOutputs(),
	)
	if p.GetNoOutput() != p.GetNoOutput() {
		t.Fatalf("PairOutputs NO_OUTPUT identity broken")
	}
	zero := p.NewPair(0, 0)
	if zero != p.GetNoOutput() {
		t.Fatalf("NewPair(0,0) should return the NO_OUTPUT singleton")
	}
}

func TestPairOutputsByteSequenceCombo(t *testing.T) {
	p := NewPairOutputs[*util.BytesRef, int64](
		ByteSequenceOutputs(),
		PositiveIntOutputs(),
	)

	// Round-trip with non-trivial values.
	bs := &util.BytesRef{Bytes: []byte("hi"), Offset: 0, Length: 2}
	in := p.NewPair(bs, int64(7))

	out := store.NewByteArrayDataOutput(16)
	if err := p.Write(in, out); err != nil {
		t.Fatalf("Write: %v", err)
	}
	di := store.NewByteArrayDataInput(out.GetBytes())
	got, err := p.Read(di)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(got.Output1.ValidBytes(), []byte("hi")) {
		t.Fatalf("Output1 mismatch")
	}
	if got.Output2 != 7 {
		t.Fatalf("Output2 mismatch: %d", got.Output2)
	}
}

func TestPairOutputsByteFormat(t *testing.T) {
	// Write Pair(BytesRef{0xCA,0xFE}, int64(2)) and assert the wire
	// layout is [vint(2), 0xCA, 0xFE, vlong(2)] = [0x02 0xCA 0xFE 0x02].
	p := NewPairOutputs[*util.BytesRef, int64](
		ByteSequenceOutputs(),
		PositiveIntOutputs(),
	)
	out := store.NewByteArrayDataOutput(8)
	pair := p.NewPair(&util.BytesRef{Bytes: []byte{0xCA, 0xFE}, Offset: 0, Length: 2}, int64(2))
	if err := p.Write(pair, out); err != nil {
		t.Fatalf("Write: %v", err)
	}
	want := []byte{0x02, 0xCA, 0xFE, 0x02}
	if !bytes.Equal(out.GetBytes(), want) {
		t.Fatalf("byte format drift: want % x got % x", want, out.GetBytes())
	}
}

func TestPairOutputsToString(t *testing.T) {
	p := NewPairOutputs[int64, int64](PositiveIntOutputs(), PositiveIntOutputs())
	got := p.OutputToString(p.NewPair(1, 2))
	if !strings.Contains(got, "<pair:") || !strings.Contains(got, "1") || !strings.Contains(got, "2") {
		t.Fatalf("OutputToString: %q", got)
	}
}
