// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"bytes"
	"testing"
)

func TestNewGrowableByteArrayDataOutput(t *testing.T) {
	g := NewGrowableByteArrayDataOutput(100)

	if g.Length() != 0 {
		t.Errorf("expected length 0, got %d", g.Length())
	}

	if g.Capacity() != 100 {
		t.Errorf("expected capacity 100, got %d", g.Capacity())
	}

	// Test default size
	g2 := NewGrowableByteArrayDataOutput(0)
	if g2.Capacity() != 1024 {
		t.Errorf("expected default capacity 1024, got %d", g2.Capacity())
	}
}

func TestGrowableByteArrayDataOutput_WriteByte(t *testing.T) {
	g := NewGrowableByteArrayDataOutput(10)

	// Write some bytes
	for i := 0; i < 5; i++ {
		if err := g.WriteByte(byte('a' + i)); err != nil {
			t.Fatalf("failed to write byte: %v", err)
		}
	}

	if g.Length() != 5 {
		t.Errorf("expected length 5, got %d", g.Length())
	}

	result := g.GetBytes()
	if len(result) != 5 {
		t.Errorf("expected 5 bytes, got %d", len(result))
	}

	expected := []byte{'a', 'b', 'c', 'd', 'e'}
	if !bytes.Equal(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestGrowableByteArrayDataOutput_Grow(t *testing.T) {
	g := NewGrowableByteArrayDataOutput(4)

	// Write more bytes than initial capacity
	data := []byte("hello world this is a test")
	for _, b := range data {
		if err := g.WriteByte(b); err != nil {
			t.Fatalf("failed to write byte: %v", err)
		}
	}

	if g.Length() != len(data) {
		t.Errorf("expected length %d, got %d", len(data), g.Length())
	}

	result := g.GetBytes()
	if !bytes.Equal(result, data) {
		t.Errorf("expected %v, got %v", data, result)
	}

	// Capacity should have grown
	if g.Capacity() < len(data) {
		t.Error("expected capacity to grow")
	}
}

func TestGrowableByteArrayDataOutput_WriteBytes(t *testing.T) {
	g := NewGrowableByteArrayDataOutput(10)

	data := []byte("hello world")
	if err := g.WriteBytes(data); err != nil {
		t.Fatalf("failed to write bytes: %v", err)
	}

	if g.Length() != len(data) {
		t.Errorf("expected length %d, got %d", len(data), g.Length())
	}

	result := g.GetBytes()
	if !bytes.Equal(result, data) {
		t.Errorf("expected %v, got %v", data, result)
	}
}

func TestGrowableByteArrayDataOutput_WriteString(t *testing.T) {
	g := NewGrowableByteArrayDataOutput(10)

	if err := g.WriteString("hello"); err != nil {
		t.Fatalf("failed to write string: %v", err)
	}

	if g.String() != "hello" {
		t.Errorf("expected 'hello', got %q", g.String())
	}
}

func TestGrowableByteArrayDataOutput_Reset(t *testing.T) {
	g := NewGrowableByteArrayDataOutput(10)

	g.WriteBytes([]byte("hello"))
	if g.Length() != 5 {
		t.Errorf("expected length 5, got %d", g.Length())
	}

	g.Reset()
	if g.Length() != 0 {
		t.Errorf("expected length 0 after reset, got %d", g.Length())
	}

	// Capacity should remain
	if g.Capacity() != 10 {
		t.Errorf("expected capacity 10, got %d", g.Capacity())
	}
}

func TestGrowableByteArrayDataOutput_WriteInt32(t *testing.T) {
	g := NewGrowableByteArrayDataOutput(10)

	if err := g.WriteInt32(0x12345678); err != nil {
		t.Fatalf("failed to write int32: %v", err)
	}

	if g.Length() != 4 {
		t.Errorf("expected length 4, got %d", g.Length())
	}

	result := g.GetBytes()
	expected := []byte{0x12, 0x34, 0x56, 0x78}
	if !bytes.Equal(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestGrowableByteArrayDataOutput_WriteInt64(t *testing.T) {
	g := NewGrowableByteArrayDataOutput(10)

	if err := g.WriteInt64(0x123456789ABCDEF0); err != nil {
		t.Fatalf("failed to write int64: %v", err)
	}

	if g.Length() != 8 {
		t.Errorf("expected length 8, got %d", g.Length())
	}

	result := g.GetBytes()
	expected := []byte{0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC, 0xDE, 0xF0}
	if !bytes.Equal(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestGrowableByteArrayDataOutput_GetBytesRef(t *testing.T) {
	g := NewGrowableByteArrayDataOutput(10)
	g.WriteBytes([]byte("test"))

	ref := g.GetBytesRef()
	if ref == nil {
		t.Fatal("expected BytesRef to be returned")
	}

	if !bytes.Equal(ref.Bytes, []byte("test")) {
		t.Errorf("expected 'test', got %v", ref.Bytes)
	}
}
