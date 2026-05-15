// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package fst

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestOnHeapFSTStoreFromBytesAndReadback(t *testing.T) {
	raw := []byte{0xAA, 0xBB, 0xCC, 0xDD}
	s := NewOnHeapFSTStoreFromBytes(raw)
	if s.Size() != int64(len(raw)) {
		t.Fatalf("Size: got %d want %d", s.Size(), len(raw))
	}

	out := store.NewByteArrayDataOutput(8)
	if err := s.WriteTo(out); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	if !bytes.Equal(out.GetBytes(), raw) {
		t.Fatalf("WriteTo: got % x want % x", out.GetBytes(), raw)
	}

	r := s.GetReverseBytesReader()
	// First byte read in reverse is the last byte (DD).
	b, err := r.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte: %v", err)
	}
	if b != 0xDD {
		t.Fatalf("first reverse byte: got 0x%02X want 0xDD", b)
	}

	// SetPosition / GetPosition.
	r.SetPosition(0)
	if got := r.GetPosition(); got != 0 {
		t.Fatalf("GetPosition: got %d want 0", got)
	}
	b, err = r.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte at pos 0: %v", err)
	}
	if b != 0xAA {
		t.Fatalf("byte at pos 0: got 0x%02X want 0xAA", b)
	}
}

func TestOnHeapFSTStoreFromDataInput(t *testing.T) {
	raw := []byte{1, 2, 3, 4, 5, 6}
	in := store.NewByteArrayDataInput(raw)
	s, err := NewOnHeapFSTStoreFromDataInput(15, in, int64(len(raw)))
	if err != nil {
		t.Fatalf("NewOnHeapFSTStoreFromDataInput: %v", err)
	}
	if s.Size() != int64(len(raw)) {
		t.Fatalf("Size mismatch")
	}
	if s.RAMBytesUsed() <= 0 {
		t.Fatalf("RAMBytesUsed should be positive")
	}
}

func TestOnHeapFSTStoreRejectsBadBlockBits(t *testing.T) {
	in := store.NewByteArrayDataInput(nil)
	if _, err := NewOnHeapFSTStoreFromDataInput(0, in, 0); err == nil {
		t.Fatal("expected error for maxBlockBits=0")
	}
	if _, err := NewOnHeapFSTStoreFromDataInput(31, in, 0); err == nil {
		t.Fatal("expected error for maxBlockBits=31")
	}
}
