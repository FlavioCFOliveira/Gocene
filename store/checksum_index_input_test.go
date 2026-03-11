// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"hash/crc32"
	"testing"
)

// createTestInput creates a test IndexInput with the given data using ByteBuffersDirectory
func createTestInput(t *testing.T, data []byte) IndexInput {
	t.Helper()
	dir := NewByteBuffersDirectory()
	out, err := dir.CreateOutput("test", IOContext{})
	if err != nil {
		t.Fatalf("Failed to create output: %v", err)
	}
	if err := out.WriteBytes(data); err != nil {
		t.Fatalf("Failed to write: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	in, err := dir.OpenInput("test", IOContext{})
	if err != nil {
		t.Fatalf("Failed to open input: %v", err)
	}
	return in
}

// createTestOutput creates a test IndexOutput using ByteBuffersDirectory
func createTestOutput(t *testing.T) (IndexOutput, *ByteBuffersDirectory) {
	t.Helper()
	dir := NewByteBuffersDirectory()
	out, err := dir.CreateOutput("test", IOContext{})
	if err != nil {
		t.Fatalf("Failed to create output: %v", err)
	}
	return out, dir
}

func TestNewChecksumIndexInput(t *testing.T) {
	testData := []byte("Hello, Checksum!")
	baseInput := createTestInput(t, testData)
	defer baseInput.Close()

	checksumInput := NewChecksumIndexInput(baseInput)
	defer checksumInput.Close()

	if checksumInput == nil {
		t.Fatal("Expected non-nil ChecksumIndexInput")
	}

	if checksumInput.GetChecksumType() != ChecksumCRC32 {
		t.Errorf("Expected CRC32 checksum type, got %v", checksumInput.GetChecksumType())
	}

	if checksumInput.Length() != int64(len(testData)) {
		t.Errorf("Expected length %d, got %d", len(testData), checksumInput.Length())
	}
}

func TestNewChecksumIndexInputWithType(t *testing.T) {
	testData := []byte("Test data")
	baseInput := createTestInput(t, testData)
	defer baseInput.Close()

	adlerInput := NewChecksumIndexInputWithType(baseInput, ChecksumAdler32)
	defer adlerInput.Close()

	if adlerInput.GetChecksumType() != ChecksumAdler32 {
		t.Errorf("Expected Adler32 checksum type, got %v", adlerInput.GetChecksumType())
	}

	baseInput2 := createTestInput(t, testData)
	defer baseInput2.Close()
	crcInput := NewChecksumIndexInputWithType(baseInput2, ChecksumCRC32)
	defer crcInput.Close()

	if crcInput.GetChecksumType() != ChecksumCRC32 {
		t.Errorf("Expected CRC32 checksum type, got %v", crcInput.GetChecksumType())
	}
}

func TestChecksumIndexInput_ReadByte(t *testing.T) {
	testData := []byte("ABC")
	baseInput := createTestInput(t, testData)
	defer baseInput.Close()
	checksumInput := NewChecksumIndexInput(baseInput)
	defer checksumInput.Close()

	for i, expected := range testData {
		b, err := checksumInput.ReadByte()
		if err != nil {
			t.Fatalf("Failed to read byte at position %d: %v", i, err)
		}
		if b != expected {
			t.Errorf("Expected byte %d at position %d, got %d", expected, i, b)
		}
	}

	expectedChecksum := crc32.ChecksumIEEE(testData)
	if checksumInput.GetChecksum() != expectedChecksum {
		t.Errorf("Expected checksum %d, got %d", expectedChecksum, checksumInput.GetChecksum())
	}
}

func TestChecksumIndexInput_ReadBytes(t *testing.T) {
	testData := []byte("Hello, World!")
	baseInput := createTestInput(t, testData)
	defer baseInput.Close()
	checksumInput := NewChecksumIndexInput(baseInput)
	defer checksumInput.Close()

	buf := make([]byte, len(testData))
	if err := checksumInput.ReadBytes(buf); err != nil {
		t.Fatalf("Failed to read bytes: %v", err)
	}

	if string(buf) != string(testData) {
		t.Errorf("Expected %q, got %q", testData, buf)
	}

	expectedChecksum := crc32.ChecksumIEEE(testData)
	if checksumInput.GetChecksum() != expectedChecksum {
		t.Errorf("Expected checksum %d, got %d", expectedChecksum, checksumInput.GetChecksum())
	}
}

func TestChecksumIndexInput_ReadBytesN(t *testing.T) {
	testData := []byte("Test data for ReadBytesN")
	baseInput := createTestInput(t, testData)
	defer baseInput.Close()
	checksumInput := NewChecksumIndexInput(baseInput)
	defer checksumInput.Close()

	result, err := checksumInput.ReadBytesN(8)
	if err != nil {
		t.Fatalf("Failed to read bytes: %v", err)
	}
	if string(result) != "Test dat" {
		t.Errorf("Expected 'Test dat', got %q", result)
	}
}

func TestChecksumIndexInput_Seek(t *testing.T) {
	testData := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	baseInput := createTestInput(t, testData)
	defer baseInput.Close()
	checksumInput := NewChecksumIndexInput(baseInput)
	defer checksumInput.Close()

	buf := make([]byte, 5)
	if err := checksumInput.ReadBytes(buf); err != nil {
		t.Fatalf("Failed to read: %v", err)
	}
	oldChecksum := checksumInput.GetChecksum()

	if err := checksumInput.SetPosition(10); err != nil {
		t.Fatalf("Failed to seek: %v", err)
	}

	if checksumInput.GetChecksum() == oldChecksum {
		t.Error("Checksum should be reset after seek")
	}

	b, err := checksumInput.ReadByte()
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}
	if b != testData[10] {
		t.Errorf("Expected byte %c at position 10, got %c", testData[10], b)
	}
}

func TestChecksumIndexInput_Clone(t *testing.T) {
	testData := []byte("Clone test data")
	baseInput := createTestInput(t, testData)
	defer baseInput.Close()
	checksumInput := NewChecksumIndexInput(baseInput)
	defer checksumInput.Close()

	buf := make([]byte, 5)
	if err := checksumInput.ReadBytes(buf); err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	cloned := checksumInput.Clone()
	defer cloned.Close()

	if checksumInput.GetFilePointer() != 5 {
		t.Errorf("Original should be at position 5, got %d", checksumInput.GetFilePointer())
	}
}

func TestChecksumIndexInput_Slice(t *testing.T) {
	testData := []byte("Slice test data here")
	baseInput := createTestInput(t, testData)
	defer baseInput.Close()
	checksumInput := NewChecksumIndexInput(baseInput)
	defer checksumInput.Close()

	slice, err := checksumInput.Slice("slice desc", 6, 4)
	if err != nil {
		t.Fatalf("Failed to create slice: %v", err)
	}
	defer slice.Close()

	sliceChecksum := slice.(*ChecksumIndexInput)

	buf := make([]byte, 4)
	if err := sliceChecksum.ReadBytes(buf); err != nil {
		t.Fatalf("Failed to read from slice: %v", err)
	}
	if string(buf) != "test" {
		t.Errorf("Expected 'test', got %q", buf)
	}
}

func TestChecksumIndexInput_VerifyChecksum(t *testing.T) {
	testData := []byte("Verify checksum test")
	baseInput := createTestInput(t, testData)
	defer baseInput.Close()
	checksumInput := NewChecksumIndexInput(baseInput)
	defer checksumInput.Close()

	buf := make([]byte, len(testData))
	if err := checksumInput.ReadBytes(buf); err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	computedChecksum := checksumInput.GetChecksum()

	if err := checksumInput.VerifyChecksum(computedChecksum); err != nil {
		t.Errorf("Verification should succeed with correct checksum: %v", err)
	}

	wrongChecksum := computedChecksum + 1
	if err := checksumInput.VerifyChecksum(wrongChecksum); err == nil {
		t.Error("Verification should fail with wrong checksum")
	}
}

func TestChecksumIndexInput_GetWrappedInput(t *testing.T) {
	testData := []byte("Wrapped input test")
	baseInput := createTestInput(t, testData)
	defer baseInput.Close()
	checksumInput := NewChecksumIndexInput(baseInput)
	defer checksumInput.Close()

	wrapped := checksumInput.GetWrappedInput()
	if wrapped != baseInput {
		t.Error("GetWrappedInput should return the original input")
	}
}

func TestChecksumIndexInput_ChecksumTypeString(t *testing.T) {
	if ChecksumAdler32.String() != "Adler32" {
		t.Errorf("Expected 'Adler32', got %s", ChecksumAdler32.String())
	}
	if ChecksumCRC32.String() != "CRC32" {
		t.Errorf("Expected 'CRC32', got %s", ChecksumCRC32.String())
	}
	if ChecksumType(99).String() != "Unknown" {
		t.Errorf("Expected 'Unknown' for invalid type, got %s", ChecksumType(99).String())
	}
}

func TestNewChecksumIndexOutput(t *testing.T) {
	baseOutput, _ := createTestOutput(t)

	checksumOutput := NewChecksumIndexOutput(baseOutput)
	defer checksumOutput.Close()

	if checksumOutput == nil {
		t.Fatal("Expected non-nil ChecksumIndexOutput")
	}

	if checksumOutput.GetChecksumType() != ChecksumCRC32 {
		t.Errorf("Expected CRC32 checksum type, got %v", checksumOutput.GetChecksumType())
	}
}

func TestChecksumIndexOutput_WriteByte(t *testing.T) {
	baseOutput, _ := createTestOutput(t)
	checksumOutput := NewChecksumIndexOutput(baseOutput)
	defer checksumOutput.Close()

	testData := []byte("ABC")
	for _, b := range testData {
		if err := checksumOutput.WriteByte(b); err != nil {
			t.Fatalf("Failed to write byte: %v", err)
		}
	}

	expectedChecksum := crc32.ChecksumIEEE(testData)
	if checksumOutput.GetChecksum() != expectedChecksum {
		t.Errorf("Expected checksum %d, got %d", expectedChecksum, checksumOutput.GetChecksum())
	}
}

func TestChecksumIndexOutput_WriteBytes(t *testing.T) {
	baseOutput, _ := createTestOutput(t)
	checksumOutput := NewChecksumIndexOutput(baseOutput)
	defer checksumOutput.Close()

	testData := []byte("Hello, World!")
	if err := checksumOutput.WriteBytes(testData); err != nil {
		t.Fatalf("Failed to write bytes: %v", err)
	}

	expectedChecksum := crc32.ChecksumIEEE(testData)
	if checksumOutput.GetChecksum() != expectedChecksum {
		t.Errorf("Expected checksum %d, got %d", expectedChecksum, checksumOutput.GetChecksum())
	}
}

func TestChecksumIndexOutput_WriteBytesN(t *testing.T) {
	baseOutput, _ := createTestOutput(t)
	checksumOutput := NewChecksumIndexOutput(baseOutput)
	defer checksumOutput.Close()

	testData := []byte("Test data for WriteBytesN")
	if err := checksumOutput.WriteBytesN(testData, 8); err != nil {
		t.Fatalf("Failed to write bytes: %v", err)
	}

	if checksumOutput.Length() != 8 {
		t.Errorf("Expected length 8, got %d", checksumOutput.Length())
	}
}

func TestChecksumIndexOutput_WriteBytesN_Invalid(t *testing.T) {
	baseOutput, _ := createTestOutput(t)
	checksumOutput := NewChecksumIndexOutput(baseOutput)
	defer checksumOutput.Close()

	testData := []byte("short")
	if err := checksumOutput.WriteBytesN(testData, 10); err == nil {
		t.Error("Expected error when n exceeds buffer length")
	}
}

func TestChecksumIndexOutput_Length(t *testing.T) {
	baseOutput, _ := createTestOutput(t)
	checksumOutput := NewChecksumIndexOutput(baseOutput)
	defer checksumOutput.Close()

	if checksumOutput.Length() != 0 {
		t.Errorf("Expected initial length 0, got %d", checksumOutput.Length())
	}

	testData := []byte("Length test")
	if err := checksumOutput.WriteBytes(testData); err != nil {
		t.Fatalf("Failed to write: %v", err)
	}

	if checksumOutput.Length() != int64(len(testData)) {
		t.Errorf("Expected length %d, got %d", len(testData), checksumOutput.Length())
	}
}

func TestChecksumIndexOutput_GetWrappedOutput(t *testing.T) {
	baseOutput, _ := createTestOutput(t)
	checksumOutput := NewChecksumIndexOutput(baseOutput)
	defer checksumOutput.Close()

	wrapped := checksumOutput.GetWrappedOutput()
	if wrapped != baseOutput {
		t.Error("GetWrappedOutput should return the original output")
	}
}

func TestChecksumIndexOutput_Adler32(t *testing.T) {
	baseOutput, _ := createTestOutput(t)
	checksumOutput := NewChecksumIndexOutputWithType(baseOutput, ChecksumAdler32)
	defer checksumOutput.Close()

	testData := []byte("Adler32 test")
	if err := checksumOutput.WriteBytes(testData); err != nil {
		t.Fatalf("Failed to write: %v", err)
	}

	if checksumOutput.GetChecksumType() != ChecksumAdler32 {
		t.Error("Expected Adler32 checksum type")
	}

	if checksumOutput.GetChecksum() == 0 {
		t.Error("Expected non-zero Adler32 checksum")
	}
}

func TestChecksumException(t *testing.T) {
	exception := NewChecksumException(12345, 67890)

	if exception.Computed != 12345 {
		t.Errorf("Expected computed checksum 12345, got %d", exception.Computed)
	}
	if exception.Expected != 67890 {
		t.Errorf("Expected expected checksum 67890, got %d", exception.Expected)
	}
	if exception.Error() != "checksum verification failed" {
		t.Errorf("Unexpected error message: %s", exception.Error())
	}
}

func TestChecksumError(t *testing.T) {
	err := NewChecksumError("test error")
	if err.Error() != "test error" {
		t.Errorf("Expected 'test error', got %s", err.Error())
	}
}

func TestChecksumIndexOutput_ByteByByteVsBulk(t *testing.T) {
	testData := []byte("Checksum comparison test")

	baseOutput1, _ := createTestOutput(t)
	checksumOutput1 := NewChecksumIndexOutput(baseOutput1)
	for _, b := range testData {
		if err := checksumOutput1.WriteByte(b); err != nil {
			t.Fatalf("Failed to write byte: %v", err)
		}
	}
	checksum1 := checksumOutput1.GetChecksum()
	checksumOutput1.Close()

	baseOutput2, _ := createTestOutput(t)
	checksumOutput2 := NewChecksumIndexOutput(baseOutput2)
	if err := checksumOutput2.WriteBytes(testData); err != nil {
		t.Fatalf("Failed to write bytes: %v", err)
	}
	checksum2 := checksumOutput2.GetChecksum()
	checksumOutput2.Close()

	if checksum1 != checksum2 {
		t.Errorf("Checksum mismatch: byte-by-byte=%d, bulk=%d", checksum1, checksum2)
	}
}
