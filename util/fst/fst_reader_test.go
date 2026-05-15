// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package fst

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// fakeBytesReader is a minimal BytesReader used to validate that the
// interface contract compiles and that downstream code can hold an
// FSTReader without needing the full FST type yet.
type fakeBytesReader struct {
	store.DataInput
	store.VariableLengthInput
	pos int64
}

func (f *fakeBytesReader) GetPosition() int64      { return f.pos }
func (f *fakeBytesReader) SetPosition(pos int64)   { f.pos = pos }
func (f *fakeBytesReader) SkipBytes(n int64) error { f.pos -= n; return nil }

// fakeFSTReader exercises the FSTReader contract.
type fakeFSTReader struct {
	bytes []byte
}

func (f *fakeFSTReader) GetReverseBytesReader() BytesReader {
	// Wrap a ByteArrayDataInput in our fakeBytesReader. The real
	// implementation reverses the byte order; this fake just exposes
	// the raw bytes and is sufficient for an interface smoke test.
	in := store.NewByteArrayDataInput(f.bytes)
	return &fakeBytesReader{DataInput: in, VariableLengthInput: in}
}

func (f *fakeFSTReader) WriteTo(out store.DataOutput) error {
	return out.WriteBytesN(f.bytes, len(f.bytes))
}

func (f *fakeFSTReader) RAMBytesUsed() int64 { return int64(len(f.bytes)) }

func TestFSTReaderInterfaceContract(t *testing.T) {
	var r FSTReader = &fakeFSTReader{bytes: []byte{0xDE, 0xAD}}
	if r.RAMBytesUsed() != 2 {
		t.Fatalf("RAMBytesUsed: got %d want 2", r.RAMBytesUsed())
	}
	rev := r.GetReverseBytesReader()
	rev.SetPosition(1)
	if rev.GetPosition() != 1 {
		t.Fatalf("GetPosition mismatch")
	}

	out := store.NewByteArrayDataOutput(2)
	if err := r.WriteTo(out); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	if len(out.GetBytes()) != 2 {
		t.Fatalf("WriteTo wrote %d bytes; want 2", len(out.GetBytes()))
	}
}
