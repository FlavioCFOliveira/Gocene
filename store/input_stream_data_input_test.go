// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"bytes"
	"io"
	"math/rand"
	"testing"
)

// TestInputStreamDataInput_SkipBytes tests skipping bytes and verifying position by reading.
// Source: TestInputStreamDataInput.testSkipBytes()
// Purpose: Tests that skipBytes correctly advances position and data can be read after skipping.
func TestInputStreamDataInput_SkipBytes(t *testing.T) {
	// Generate random test data (at least 100 bytes)
	randomData := make([]byte, 100)
	rand.Read(randomData)

	// Create InputStreamDataInput with the random data
	in := NewInputStreamDataInput(bytes.NewReader(randomData))
	defer in.Close()

	maxSkipTo := len(randomData) - 1
	curr := 0

	// Skip chunks of bytes until exhausted
	for curr < maxSkipTo {
		skipTo := curr + rand.Intn(maxSkipTo-curr+1)
		step := skipTo - curr

		err := in.SkipBytes(int64(step))
		if err != nil {
			t.Fatalf("SkipBytes failed at position %d: %v", curr, err)
		}

		// Read and verify the byte at the skipped-to position
		b, err := in.ReadByte()
		if err != nil {
			t.Fatalf("ReadByte failed at position %d: %v", skipTo, err)
		}

		if b != randomData[skipTo] {
			t.Errorf("Expected byte 0x%02x at position %d, got 0x%02x", randomData[skipTo], skipTo, b)
		}

		curr = skipTo + 1 // +1 for the read byte
	}
}

// TestInputStreamDataInput_NoReadWhenSkipping tests that skipBytes doesn't invoke read operations.
// Source: TestInputStreamDataInput.testNoReadWhenSkipping()
// Purpose: Ensures skipBytes uses the underlying stream's skip method rather than reading and discarding.
func TestInputStreamDataInput_NoReadWhenSkipping(t *testing.T) {
	// Generate random test data (at least 100 bytes)
	randomData := make([]byte, 100)
	rand.Read(randomData)

	// Use NoReadInputStreamDataInput to ensure skipBytes doesn't call read
	in := NewNoReadInputStreamDataInput(bytes.NewReader(randomData))
	defer in.Close()

	maxSkipTo := len(randomData) - 1
	curr := 0

	// Skip chunks of bytes until exhausted
	for curr < maxSkipTo {
		step := rand.Intn(maxSkipTo - curr + 1)
		err := in.SkipBytes(int64(step))
		if err != nil {
			t.Fatalf("SkipBytes failed: %v", err)
		}
		curr += step
	}
}

// TestInputStreamDataInput_FullSkip tests skipping all bytes in the stream.
// Source: TestInputStreamDataInput.testFullSkip()
// Purpose: Tests that skipBytes can skip the entire stream without error.
func TestInputStreamDataInput_FullSkip(t *testing.T) {
	// Generate random test data (at least 100 bytes)
	randomData := make([]byte, 100)
	rand.Read(randomData)

	// Use NoReadInputStreamDataInput to ensure skipBytes doesn't call read
	in := NewNoReadInputStreamDataInput(bytes.NewReader(randomData))
	defer in.Close()

	// Skip all bytes
	err := in.SkipBytes(int64(len(randomData)))
	if err != nil {
		t.Fatalf("Full skip failed: %v", err)
	}
}

// TestInputStreamDataInput_SkipOffEnd tests that skipping past the end of stream returns EOF.
// Source: TestInputStreamDataInput.testSkipOffEnd()
// Purpose: Tests that skipBytes returns EOF when attempting to skip more bytes than available.
func TestInputStreamDataInput_SkipOffEnd(t *testing.T) {
	// Generate random test data (at least 100 bytes)
	randomData := make([]byte, 100)
	rand.Read(randomData)

	// Use NoReadInputStreamDataInput to ensure skipBytes doesn't call read
	in := NewNoReadInputStreamDataInput(bytes.NewReader(randomData))
	defer in.Close()

	// Attempt to skip past the end of the stream
	err := in.SkipBytes(int64(len(randomData) + 1))
	if err != io.EOF {
		t.Errorf("Expected io.EOF when skipping past end, got %v", err)
	}
}

// noReadInputStreamDataInput wraps InputStreamDataInput and throws errors on read operations.
// This is used to verify that skipBytes doesn't invoke read methods.
// Source: TestInputStreamDataInput.NoReadInputStreamDataInput (inner class)
type noReadInputStreamDataInput struct {
	*InputStreamDataInput
}

// NewNoReadInputStreamDataInput creates a new noReadInputStreamDataInput.
func NewNoReadInputStreamDataInput(r io.Reader) *noReadInputStreamDataInput {
	return &noReadInputStreamDataInput{
		InputStreamDataInput: NewInputStreamDataInput(r),
	}
}

// ReadBytes overrides the parent method to throw UnsupportedOperationException.
func (n *noReadInputStreamDataInput) ReadBytes(b []byte) error {
	panic("ReadBytes should not be called during skipBytes")
}

// ReadByte overrides the parent method to throw UnsupportedOperationException.
func (n *noReadInputStreamDataInput) ReadByte() (byte, error) {
	panic("ReadByte should not be called during skipBytes")
}
