// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/test/org/apache/lucene/store/* (OutputStreamDataOutput
// has no dedicated test peer; behaviour is exercised indirectly via callers
// that serialise primitives. The tests below validate the contract directly).

package store

import (
	"bytes"
	"errors"
	"testing"
)

func TestOutputStreamDataOutput_Roundtrip(t *testing.T) {
	var sink bytes.Buffer
	o := NewOutputStreamDataOutput(&sink)
	if err := o.WriteByte(0x42); err != nil {
		t.Fatalf("WriteByte: %v", err)
	}
	if err := o.WriteShort(int16(-12345)); err != nil {
		t.Fatalf("WriteShort: %v", err)
	}
	if err := o.WriteInt(int32(0x01020304)); err != nil {
		t.Fatalf("WriteInt: %v", err)
	}
	if err := o.WriteLong(int64(0x05060708090A0B0C)); err != nil {
		t.Fatalf("WriteLong: %v", err)
	}
	if err := o.WriteVInt(int32(300)); err != nil {
		t.Fatalf("WriteVInt: %v", err)
	}
	if err := o.WriteVLong(int64(1 << 35)); err != nil {
		t.Fatalf("WriteVLong: %v", err)
	}
	if err := o.WriteString("hello"); err != nil {
		t.Fatalf("WriteString: %v", err)
	}

	in := NewByteArrayDataInput(sink.Bytes())
	if got, _ := in.ReadByte(); got != 0x42 {
		t.Fatalf("ReadByte = %#x", got)
	}
	if got, _ := in.ReadShort(); got != int16(-12345) {
		t.Fatalf("ReadShort = %d", got)
	}
	if got, _ := in.ReadInt(); got != int32(0x01020304) {
		t.Fatalf("ReadInt = %#x", got)
	}
	if got, _ := in.ReadLong(); got != int64(0x05060708090A0B0C) {
		t.Fatalf("ReadLong = %#x", got)
	}
	if got, _ := in.ReadVInt(); got != 300 {
		t.Fatalf("ReadVInt = %d", got)
	}
	if got, _ := in.ReadVLong(); got != int64(1<<35) {
		t.Fatalf("ReadVLong = %d", got)
	}
	if got, _ := in.ReadString(); got != "hello" {
		t.Fatalf("ReadString = %q", got)
	}
}

type closingWriter struct {
	bytes.Buffer
	closed bool
}

func (c *closingWriter) Close() error {
	c.closed = true
	return nil
}

func TestOutputStreamDataOutput_CloseForwards(t *testing.T) {
	cw := &closingWriter{}
	o := NewOutputStreamDataOutput(cw)
	if err := o.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !cw.closed {
		t.Fatalf("close was not forwarded to wrapped writer")
	}
}

func TestOutputStreamDataOutput_CloseNoCloser(t *testing.T) {
	var sink bytes.Buffer
	o := NewOutputStreamDataOutput(&sink)
	if err := o.Close(); err != nil {
		t.Fatalf("Close on non-closer should be a no-op, got %v", err)
	}
}

type erroringWriter struct{}

func (erroringWriter) Write([]byte) (int, error) { return 0, errors.New("boom") }

func TestOutputStreamDataOutput_PropagatesError(t *testing.T) {
	o := NewOutputStreamDataOutput(erroringWriter{})
	if err := o.WriteByte(1); err == nil {
		t.Fatalf("expected error from underlying writer")
	}
}
