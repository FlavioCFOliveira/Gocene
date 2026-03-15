// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"
)

// TestOutputStreamIndexOutput_DataTypes tests primitive data type writing
// with little-endian encoding and file pointer tracking.
//
// Source: org.apache.lucene.store.TestOutputStreamIndexOutput.testDataTypes()
// Purpose: Verifies that OutputStreamIndexOutput correctly writes primitives
// in little-endian format and tracks file pointer position.
func TestOutputStreamIndexOutput_DataTypes(t *testing.T) {
	// Test with offsets 0-11 (matching Java's for i < 12)
	for offset := 0; offset < 12; offset++ {
		t.Run(fmt.Sprintf("offset_%d", offset), func(t *testing.T) {
			doTestDataTypes(t, offset)
		})
	}
}

// doTestDataTypes performs the actual test for a given offset.
// It writes offset bytes, then writes a short, int, and long,
// and verifies the output is correctly encoded in little-endian.
func doTestDataTypes(t *testing.T, offset int) {
	buf := &bytes.Buffer{}
	out := NewOutputStreamIndexOutput("test"+string(rune('0'+offset)), "test", buf, 12)

	// Write offset bytes
	for i := 0; i < offset; i++ {
		if err := out.WriteByte(byte(i)); err != nil {
			t.Fatalf("failed to write byte at offset %d: %v", i, err)
		}
	}

	// Write primitives
	if err := out.WriteShort(12345); err != nil {
		t.Fatalf("failed to write short: %v", err)
	}
	if err := out.WriteInt(1234567890); err != nil {
		t.Fatalf("failed to write int: %v", err)
	}
	if err := out.WriteLong(1234567890123456789); err != nil {
		t.Fatalf("failed to write long: %v", err)
	}

	// Verify file pointer: offset + 2 (short) + 4 (int) + 8 (long) = offset + 14
	expectedFilePointer := int64(offset + 14)
	if out.GetFilePointer() != expectedFilePointer {
		t.Errorf("expected file pointer %d, got %d", expectedFilePointer, out.GetFilePointer())
	}

	// Close the output
	if err := out.Close(); err != nil {
		t.Fatalf("failed to close output: %v", err)
	}

	// Read the primitives using binary.LittleEndian to ensure encoding is LE
	result := buf.Bytes()
	if len(result) != offset+14 {
		t.Errorf("expected %d bytes, got %d", offset+14, len(result))
	}

	// Verify offset bytes
	for i := 0; i < offset; i++ {
		if result[i] != byte(i) {
			t.Errorf("expected byte %d at position %d, got %d", i, i, result[i])
		}
	}

	// Read and verify short (little-endian)
	pos := offset
	shortVal := binary.LittleEndian.Uint16(result[pos : pos+2])
	if shortVal != 12345 {
		t.Errorf("expected short 12345, got %d", shortVal)
	}
	pos += 2

	// Read and verify int (little-endian)
	intVal := binary.LittleEndian.Uint32(result[pos : pos+4])
	if intVal != 1234567890 {
		t.Errorf("expected int 1234567890, got %d", intVal)
	}
	pos += 4

	// Read and verify long (little-endian)
	longVal := binary.LittleEndian.Uint64(result[pos : pos+8])
	if longVal != 1234567890123456789 {
		t.Errorf("expected long 1234567890123456789, got %d", longVal)
	}
	pos += 8

	// Verify no remaining bytes
	if pos != len(result) {
		t.Errorf("expected to read all %d bytes, but read %d", len(result), pos)
	}
}

// TestOutputStreamIndexOutput_WriteByte tests individual byte writing.
func TestOutputStreamIndexOutput_WriteByte(t *testing.T) {
	buf := &bytes.Buffer{}
	out := NewOutputStreamIndexOutput("test", "test", buf, 1024)

	// Write several bytes
	testBytes := []byte{0x00, 0x01, 0xFF, 0x7F, 0x80}
	for _, b := range testBytes {
		if err := out.WriteByte(b); err != nil {
			t.Fatalf("failed to write byte %x: %v", b, err)
		}
	}

	// Verify file pointer
	if out.GetFilePointer() != int64(len(testBytes)) {
		t.Errorf("expected file pointer %d, got %d", len(testBytes), out.GetFilePointer())
	}

	// Verify written bytes
	result := buf.Bytes()
	if !bytes.Equal(result, testBytes) {
		t.Errorf("expected %v, got %v", testBytes, result)
	}
}

// TestOutputStreamIndexOutput_WriteBytes tests batch byte writing.
func TestOutputStreamIndexOutput_WriteBytes(t *testing.T) {
	buf := &bytes.Buffer{}
	out := NewOutputStreamIndexOutput("test", "test", buf, 1024)

	// Write bytes in batch
	testBytes := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	if err := out.WriteBytes(testBytes); err != nil {
		t.Fatalf("failed to write bytes: %v", err)
	}

	// Verify file pointer
	if out.GetFilePointer() != int64(len(testBytes)) {
		t.Errorf("expected file pointer %d, got %d", len(testBytes), out.GetFilePointer())
	}

	// Verify written bytes
	result := buf.Bytes()
	if !bytes.Equal(result, testBytes) {
		t.Errorf("expected %v, got %v", testBytes, result)
	}
}

// TestOutputStreamIndexOutput_WriteBytesN tests writing n bytes from slice.
func TestOutputStreamIndexOutput_WriteBytesN(t *testing.T) {
	buf := &bytes.Buffer{}
	out := NewOutputStreamIndexOutput("test", "test", buf, 1024)

	// Write only first 3 bytes of a 5-byte slice
	testBytes := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	if err := out.WriteBytesN(testBytes, 3); err != nil {
		t.Fatalf("failed to write bytes: %v", err)
	}

	// Verify file pointer
	if out.GetFilePointer() != 3 {
		t.Errorf("expected file pointer 3, got %d", out.GetFilePointer())
	}

	// Verify written bytes
	result := buf.Bytes()
	expected := []byte{0x01, 0x02, 0x03}
	if !bytes.Equal(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

// TestOutputStreamIndexOutput_FilePointerTracking tests file pointer tracking.
func TestOutputStreamIndexOutput_FilePointerTracking(t *testing.T) {
	buf := &bytes.Buffer{}
	out := NewOutputStreamIndexOutput("test", "test", buf, 1024)

	// Initial file pointer should be 0
	if out.GetFilePointer() != 0 {
		t.Errorf("expected initial file pointer 0, got %d", out.GetFilePointer())
	}

	// Write 1 byte
	out.WriteByte(0x01)
	if out.GetFilePointer() != 1 {
		t.Errorf("expected file pointer 1, got %d", out.GetFilePointer())
	}

	// Write 2 bytes (short)
	out.WriteShort(0x1234)
	if out.GetFilePointer() != 3 {
		t.Errorf("expected file pointer 3, got %d", out.GetFilePointer())
	}

	// Write 4 bytes (int)
	out.WriteInt(0x12345678)
	if out.GetFilePointer() != 7 {
		t.Errorf("expected file pointer 7, got %d", out.GetFilePointer())
	}

	// Write 8 bytes (long)
	out.WriteLong(0x1234567890ABCDEF)
	if out.GetFilePointer() != 15 {
		t.Errorf("expected file pointer 15, got %d", out.GetFilePointer())
	}

	// Length should equal file pointer
	if out.Length() != out.GetFilePointer() {
		t.Errorf("expected length %d to equal file pointer %d", out.Length(), out.GetFilePointer())
	}
}

// TestOutputStreamIndexOutput_LittleEndianShort tests short encoding is little-endian.
func TestOutputStreamIndexOutput_LittleEndianShort(t *testing.T) {
	buf := &bytes.Buffer{}
	out := NewOutputStreamIndexOutput("test", "test", buf, 1024)

	// Write a short value
	out.WriteShort(0x1234)

	// In little-endian, 0x1234 should be encoded as [0x34, 0x12]
	result := buf.Bytes()
	expected := []byte{0x34, 0x12}
	if !bytes.Equal(result, expected) {
		t.Errorf("expected little-endian encoding %v, got %v", expected, result)
	}
}

// TestOutputStreamIndexOutput_LittleEndianInt tests int encoding is little-endian.
func TestOutputStreamIndexOutput_LittleEndianInt(t *testing.T) {
	buf := &bytes.Buffer{}
	out := NewOutputStreamIndexOutput("test", "test", buf, 1024)

	// Write an int value
	out.WriteInt(0x12345678)

	// In little-endian, 0x12345678 should be encoded as [0x78, 0x56, 0x34, 0x12]
	result := buf.Bytes()
	expected := []byte{0x78, 0x56, 0x34, 0x12}
	if !bytes.Equal(result, expected) {
		t.Errorf("expected little-endian encoding %v, got %v", expected, result)
	}
}

// TestOutputStreamIndexOutput_LittleEndianLong tests long encoding is little-endian.
func TestOutputStreamIndexOutput_LittleEndianLong(t *testing.T) {
	buf := &bytes.Buffer{}
	out := NewOutputStreamIndexOutput("test", "test", buf, 1024)

	// Write a long value
	out.WriteLong(0x1234567890ABCDEF)

	// In little-endian, should be encoded as bytes in reverse order
	result := buf.Bytes()
	expected := []byte{0xEF, 0xCD, 0xAB, 0x90, 0x78, 0x56, 0x34, 0x12}
	if !bytes.Equal(result, expected) {
		t.Errorf("expected little-endian encoding %v, got %v", expected, result)
	}
}

// TestOutputStreamIndexOutput_Name tests the name accessor.
func TestOutputStreamIndexOutput_Name(t *testing.T) {
	buf := &bytes.Buffer{}
	out := NewOutputStreamIndexOutput("resource", "testfile.dat", buf, 1024)

	if out.GetName() != "testfile.dat" {
		t.Errorf("expected name 'testfile.dat', got '%s'", out.GetName())
	}
}

// TestOutputStreamIndexOutput_Close tests closing the output.
func TestOutputStreamIndexOutput_Close(t *testing.T) {
	buf := &bytes.Buffer{}
	out := NewOutputStreamIndexOutput("test", "test", buf, 1024)

	// Write some data
	out.WriteByte(0x01)

	// Close should succeed
	if err := out.Close(); err != nil {
		t.Errorf("close failed: %v", err)
	}
}

