// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package fst

import (
	"errors"
	"io"
	"testing"
)

// stubRandomAccessInput is a minimal RandomAccessInput backed by a
// fixed byte slice; sufficient to exercise the OffHeapFSTStore
// reverse reader contract without dragging in a real IndexInput.
type stubRandomAccessInput struct {
	bytes []byte
}

func (s *stubRandomAccessInput) ReadByteAt(pos int64) (byte, error) {
	if pos < 0 || pos >= int64(len(s.bytes)) {
		return 0, io.EOF
	}
	return s.bytes[pos], nil
}

func (s *stubRandomAccessInput) ReadLongAt(pos int64) (int64, error) {
	return 0, errors.New("stubRandomAccessInput: ReadLongAt not implemented")
}

func TestOffHeapFSTStoreReverseRead(t *testing.T) {
	raw := []byte{0x10, 0x20, 0x30, 0x40}
	in := &stubRandomAccessInput{bytes: raw}
	s, err := NewOffHeapFSTStore(in, 0, int64(len(raw)))
	if err != nil {
		t.Fatalf("NewOffHeapFSTStore: %v", err)
	}
	if s.Size() != int64(len(raw)) {
		t.Fatalf("Size mismatch")
	}

	r := s.GetReverseBytesReader()
	if got := r.GetPosition(); got != int64(len(raw)-1) {
		t.Fatalf("initial reverse position: got %d want %d", got, len(raw)-1)
	}
	b, err := r.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte: %v", err)
	}
	if b != 0x40 {
		t.Fatalf("first reverse byte: got 0x%02X want 0x40", b)
	}
	b, _ = r.ReadByte()
	if b != 0x30 {
		t.Fatalf("second reverse byte: got 0x%02X want 0x30", b)
	}
}

func TestOffHeapFSTStoreWriteToUnsupported(t *testing.T) {
	in := &stubRandomAccessInput{bytes: []byte{1, 2}}
	s, err := NewOffHeapFSTStore(in, 0, 2)
	if err != nil {
		t.Fatalf("NewOffHeapFSTStore: %v", err)
	}
	if err := s.WriteTo(nil); err == nil {
		t.Fatal("expected WriteTo to error")
	}
}

func TestOffHeapFSTStoreRAMBytesUsedConstant(t *testing.T) {
	in := &stubRandomAccessInput{bytes: make([]byte, 1024)}
	s, _ := NewOffHeapFSTStore(in, 0, 1024)
	if s.RAMBytesUsed() <= 0 {
		t.Fatalf("RAMBytesUsed should be positive")
	}
}

func TestOffHeapFSTStoreValidatesArgs(t *testing.T) {
	if _, err := NewOffHeapFSTStore(nil, 0, 0); err == nil {
		t.Fatal("expected error for nil input")
	}
	in := &stubRandomAccessInput{}
	if _, err := NewOffHeapFSTStore(in, -1, 0); err == nil {
		t.Fatal("expected error for negative offset")
	}
	if _, err := NewOffHeapFSTStore(in, 0, -1); err == nil {
		t.Fatal("expected error for negative numBytes")
	}
}
