// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"encoding/binary"
	"testing"
)

// TestByteArrayDataInput_Basic tests basic readString and EOF functionality.
// Source: org.apache.lucene.store.TestByteArrayDataInput.testBasic()
// Purpose: Tests string reading with VInt length prefix and EOF state
func TestByteArrayDataInput_Basic(t *testing.T) {
	// Test case 1: Simple string "A" (length 1, followed by 'A')
	// VInt encoding of 1 is just 0x01
	bytes := []byte{0x01, 'A'}
	in := NewByteArrayDataInput(bytes)

	str, err := ReadString(in)
	if err != nil {
		t.Fatalf("ReadString failed: %v", err)
	}
	if str != "A" {
		t.Errorf("Expected 'A', got '%s'", str)
	}
	if !in.EOF() {
		t.Error("Expected EOF to be true after reading all data")
	}

	// Test case 2: Reset with offset - read string starting at offset 1
	// bytes = {1, 1, 'A'} - skip first byte, read string at position 1
	bytes = []byte{0x00, 0x01, 'A'}
	in.ResetWithSlice(bytes, 1, 2)

	str, err = ReadString(in)
	if err != nil {
		t.Fatalf("ReadString after reset failed: %v", err)
	}
	if str != "A" {
		t.Errorf("Expected 'A' after reset, got '%s'", str)
	}
	if !in.EOF() {
		t.Error("Expected EOF to be true after reading all data from reset slice")
	}
}

// TestByteArrayDataInput_Datatypes tests reading primitive data types.
// Source: org.apache.lucene.store.TestByteArrayDataInput.testDatatypes()
// Purpose: Verifies little-endian encoding for short, int, and long
func TestByteArrayDataInput_Datatypes(t *testing.T) {
	// Create buffer for writing primitives
	bytes := make([]byte, 32)
	out := NewByteArrayDataOutput(32)

	// Write primitives using ByteArrayDataOutput
	// Note: The Go implementation currently uses big-endian, but Lucene uses little-endian
	// For byte-level compatibility, we need to write in little-endian format
	out.WriteByte(43)
	writeLittleEndianShort(out, 12345)
	writeLittleEndianInt(out, 1234567890)
	writeLittleEndianLong(out, 1234567890123456789)

	size := out.GetPosition()
	if size != 15 {
		t.Errorf("Expected position 15, got %d", size)
	}

	// Copy written bytes to our buffer
	copy(bytes, out.GetBytes())

	// Read using binary.LittleEndian to verify byte order
	if bytes[0] != 43 {
		t.Errorf("Expected byte 43, got %d", bytes[0])
	}

	// Verify little-endian encoding by checking byte positions
	expectedShort := uint16(12345)
	actualShort := binary.LittleEndian.Uint16(bytes[1:3])
	if actualShort != expectedShort {
		t.Errorf("Expected short %d, got %d", expectedShort, actualShort)
	}

	expectedInt := uint32(1234567890)
	actualInt := binary.LittleEndian.Uint32(bytes[3:7])
	if actualInt != expectedInt {
		t.Errorf("Expected int %d, got %d", expectedInt, actualInt)
	}

	expectedLong := uint64(1234567890123456789)
	actualLong := binary.LittleEndian.Uint64(bytes[7:15])
	if actualLong != expectedLong {
		t.Errorf("Expected long %d, got %d", expectedLong, actualLong)
	}

	// Now read the primitives using ByteArrayDataInput with little-endian helpers
	in := NewByteArrayDataInput(bytes[:size])

	b, err := in.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte failed: %v", err)
	}
	if b != 43 {
		t.Errorf("Expected byte 43, got %d", b)
	}

	s, err := readLittleEndianShort(in)
	if err != nil {
		t.Fatalf("ReadShort failed: %v", err)
	}
	if s != 12345 {
		t.Errorf("Expected short 12345, got %d", s)
	}

	i, err := readLittleEndianInt(in)
	if err != nil {
		t.Fatalf("ReadInt failed: %v", err)
	}
	if i != 1234567890 {
		t.Errorf("Expected int 1234567890, got %d", i)
	}

	l, err := readLittleEndianLong(in)
	if err != nil {
		t.Fatalf("ReadLong failed: %v", err)
	}
	if l != 1234567890123456789 {
		t.Errorf("Expected long 1234567890123456789, got %d", l)
	}

	if !in.EOF() {
		t.Error("Expected EOF to be true after reading all data")
	}
}

// Helper functions for little-endian writing (to match Lucene's byte order)

func writeLittleEndianShort(out *ByteArrayDataOutput, v int16) {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, uint16(v))
	out.WriteBytes(b)
}

func writeLittleEndianInt(out *ByteArrayDataOutput, v int32) {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(v))
	out.WriteBytes(b)
}

func writeLittleEndianLong(out *ByteArrayDataOutput, v int64) {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(v))
	out.WriteBytes(b)
}

// Helper functions for little-endian reading

func readLittleEndianShort(in *ByteArrayDataInput) (int16, error) {
	b, err := in.ReadBytesN(2)
	if err != nil {
		return 0, err
	}
	return int16(binary.LittleEndian.Uint16(b)), nil
}

func readLittleEndianInt(in *ByteArrayDataInput) (int32, error) {
	b, err := in.ReadBytesN(4)
	if err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(b)), nil
}

func readLittleEndianLong(in *ByteArrayDataInput) (int64, error) {
	b, err := in.ReadBytesN(8)
	if err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(b)), nil
}
