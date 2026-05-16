// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import "testing"

// TestRandomAccessInput_InterfaceParity verifies that the canonical
// ByteArrayRandomAccessInput implementation satisfies the extended
// RandomAccessInput interface (Length + ReadByteAt + ReadShortAt + ReadIntAt
// + ReadLongAt) that mirrors Lucene 10.4.0.
func TestRandomAccessInput_InterfaceParity(t *testing.T) {
	var _ RandomAccessInput = (*ByteArrayRandomAccessInput)(nil)
}

func TestRandomAccessInput_ReadsAllSizes(t *testing.T) {
	// Layout: byte=0xAB, short=0xCDEF (LE), int=0x01020304 (LE), long=0x0506070809000102 (LE)
	bytes := []byte{
		0xAB,
		0xEF, 0xCD,
		0x04, 0x03, 0x02, 0x01,
		0x02, 0x01, 0x00, 0x09, 0x08, 0x07, 0x06, 0x05,
	}
	in := NewByteArrayRandomAccessInput(bytes)

	b, err := in.ReadByteAt(0)
	if err != nil || b != 0xAB {
		t.Fatalf("ReadByteAt(0) = (%#x, %v), want (0xAB, nil)", b, err)
	}
	s, err := in.ReadShortAt(1)
	if err != nil || s != int16(-0x3211) /* 0xCDEF as signed */ {
		t.Fatalf("ReadShortAt(1) = (%#x, %v)", s, err)
	}
	i, err := in.ReadIntAt(3)
	if err != nil || i != int32(0x01020304) {
		t.Fatalf("ReadIntAt(3) = (%#x, %v)", i, err)
	}
	l, err := in.ReadLongAt(7)
	if err != nil || l != int64(0x0506070809000102) {
		t.Fatalf("ReadLongAt(7) = (%#x, %v)", l, err)
	}
	if got, want := in.Length(), int64(len(bytes)); got != want {
		t.Fatalf("Length() = %d, want %d", got, want)
	}
}
