// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestNewBinaryRangeDocValuesField(t *testing.T) {
	packed := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
	f, err := NewBinaryRangeDocValuesField("range", packed, 1, 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.FieldName() != "range" {
		t.Errorf("FieldName = %q, want %q", f.FieldName(), "range")
	}
	if f.Name() != "range" {
		t.Errorf("Name = %q, want %q", f.Name(), "range")
	}
	if f.NumDims() != 1 {
		t.Errorf("NumDims = %d, want 1", f.NumDims())
	}
	if f.NumBytesPerDimension() != 4 {
		t.Errorf("NumBytesPerDimension = %d, want 4", f.NumBytesPerDimension())
	}
	if !bytes.Equal(f.PackedValue(), packed) {
		t.Errorf("PackedValue = %v, want %v", f.PackedValue(), packed)
	}
	if !bytes.Equal(f.BinaryValue(), packed) {
		t.Errorf("BinaryValue = %v, want %v", f.BinaryValue(), packed)
	}
}

func TestBinaryRangeDocValuesFieldDefensiveCopy(t *testing.T) {
	packed := []byte{0xAA, 0xBB, 0xCC, 0xDD}
	f, err := NewBinaryRangeDocValuesField("r", packed, 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Mutate caller-owned slice after construction; the stored value must
	// not observe the mutation.
	packed[0] = 0x00
	packed[3] = 0x00
	got := f.PackedValue()
	want := []byte{0xAA, 0xBB, 0xCC, 0xDD}
	if !bytes.Equal(got, want) {
		t.Errorf("PackedValue after caller mutation = %v, want %v", got, want)
	}
	if !bytes.Equal(f.BinaryValue(), want) {
		t.Errorf("BinaryValue after caller mutation = %v, want %v", f.BinaryValue(), want)
	}
}

func TestBinaryRangeDocValuesFieldType(t *testing.T) {
	f, err := NewBinaryRangeDocValuesField("r", []byte{0, 0}, 1, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ft := f.FieldType()
	if ft.Indexed {
		t.Error("range field must not be indexed")
	}
	if ft.Stored {
		t.Error("range field must not be stored")
	}
	if ft.DocValuesType != index.DocValuesTypeBinary {
		t.Errorf("DocValuesType = %v, want %v", ft.DocValuesType, index.DocValuesTypeBinary)
	}
}

func TestBinaryRangeDocValuesFieldMultiDim(t *testing.T) {
	// 2 dimensions, 8 bytes per dim, [min, max] pair => 32-byte payload.
	packed := make([]byte, 2*2*8)
	for i := range packed {
		packed[i] = byte(i)
	}
	f, err := NewBinaryRangeDocValuesField("multi", packed, 2, 8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.NumDims() != 2 {
		t.Errorf("NumDims = %d, want 2", f.NumDims())
	}
	if f.NumBytesPerDimension() != 8 {
		t.Errorf("NumBytesPerDimension = %d, want 8", f.NumBytesPerDimension())
	}
	if !bytes.Equal(f.PackedValue(), packed) {
		t.Errorf("PackedValue round-trip mismatch")
	}
}

func TestBinaryRangeDocValuesFieldEmpty(t *testing.T) {
	f, err := NewBinaryRangeDocValuesField("e", []byte{}, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.PackedValue()) != 0 {
		t.Errorf("PackedValue length = %d, want 0", len(f.PackedValue()))
	}
}
