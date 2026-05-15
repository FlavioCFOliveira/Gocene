// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package fst

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestReverseBytesReaderSkipBytes verifies SkipBytes against the
// reverse-direction semantics documented on the BytesReader contract:
// positive n decrements pos; negative n increments pos.
func TestReverseBytesReaderSkipBytes(t *testing.T) {
	r := NewReverseBytesReader([]byte{0x10, 0x20, 0x30, 0x40, 0x50})
	// Initial position is len-1 = 4 (pointing at 0x50).
	if r.GetPosition() != 4 {
		t.Fatalf("initial pos: got %d want 4", r.GetPosition())
	}
	if err := r.SkipBytes(2); err != nil {
		t.Fatalf("SkipBytes(+2): %v", err)
	}
	if r.GetPosition() != 2 {
		t.Fatalf("pos after +2: got %d want 2", r.GetPosition())
	}
	b, err := r.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte: %v", err)
	}
	if b != 0x30 {
		t.Fatalf("ReadByte: got 0x%02x want 0x30", b)
	}
	// pos is now 1.
	if err := r.SkipBytes(-2); err != nil {
		t.Fatalf("SkipBytes(-2): %v", err)
	}
	if r.GetPosition() != 3 {
		t.Fatalf("pos after -2: got %d want 3", r.GetPosition())
	}
	b, err = r.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte: %v", err)
	}
	if b != 0x40 {
		t.Fatalf("ReadByte: got 0x%02x want 0x40", b)
	}
}

// TestReverseRandomAccessReaderSkipBytes mirrors the test above for
// the random-access reverse reader.
func TestReverseRandomAccessReaderSkipBytes(t *testing.T) {
	r := NewReverseRandomAccessReader(store.NewByteArrayRandomAccessInput([]byte{0x10, 0x20, 0x30, 0x40, 0x50}))
	r.SetPosition(4)
	if err := r.SkipBytes(2); err != nil {
		t.Fatalf("SkipBytes(+2): %v", err)
	}
	if r.GetPosition() != 2 {
		t.Fatalf("pos after +2: got %d want 2", r.GetPosition())
	}
	b, err := r.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte: %v", err)
	}
	if b != 0x30 {
		t.Fatalf("ReadByte: got 0x%02x want 0x30", b)
	}
	if err := r.SkipBytes(-2); err != nil {
		t.Fatalf("SkipBytes(-2): %v", err)
	}
	if r.GetPosition() != 3 {
		t.Fatalf("pos after -2: got %d want 3", r.GetPosition())
	}
	b, err = r.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte: %v", err)
	}
	if b != 0x40 {
		t.Fatalf("ReadByte: got 0x%02x want 0x40", b)
	}
}
