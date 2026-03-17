// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"encoding/binary"
	"testing"
)

func TestNewByteArrayRandomAccessInput(t *testing.T) {
	bytes := []byte{0x01, 0x02, 0x03, 0x04}
	input := NewByteArrayRandomAccessInput(bytes)

	if input == nil {
		t.Fatal("NewByteArrayRandomAccessInput() returned nil")
	}

	if input.Length() != 4 {
		t.Errorf("Length() = %d, want 4", input.Length())
	}
}

func TestByteArrayRandomAccessInput_ReadByteAt(t *testing.T) {
	bytes := []byte{0x01, 0x02, 0x03, 0x04}
	input := NewByteArrayRandomAccessInput(bytes)

	tests := []struct {
		pos     int64
		want    byte
		wantErr bool
	}{
		{0, 0x01, false},
		{1, 0x02, false},
		{2, 0x03, false},
		{3, 0x04, false},
		{4, 0, true},  // out of bounds
		{-1, 0, true}, // negative position
	}

	for _, tt := range tests {
		got, err := input.ReadByteAt(tt.pos)
		if (err != nil) != tt.wantErr {
			t.Errorf("ReadByteAt(%d) error = %v, wantErr %v", tt.pos, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("ReadByteAt(%d) = 0x%02x, want 0x%02x", tt.pos, got, tt.want)
		}
	}
}

func TestByteArrayRandomAccessInput_ReadShortAt(t *testing.T) {
	// Create byte slice with a short value (little-endian: 0x1234)
	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint16(bytes[0:], 0x1234)
	binary.LittleEndian.PutUint16(bytes[2:], 0x5678)

	input := NewByteArrayRandomAccessInput(bytes)

	tests := []struct {
		pos     int64
		want    int16
		wantErr bool
	}{
		{0, 0x1234, false},
		{2, 0x5678, false},
		{3, 0, true}, // not enough bytes
		{-1, 0, true},
	}

	for _, tt := range tests {
		got, err := input.ReadShortAt(tt.pos)
		if (err != nil) != tt.wantErr {
			t.Errorf("ReadShortAt(%d) error = %v, wantErr %v", tt.pos, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("ReadShortAt(%d) = 0x%04x, want 0x%04x", tt.pos, got, tt.want)
		}
	}
}

func TestByteArrayRandomAccessInput_ReadIntAt(t *testing.T) {
	// Create byte slice with int values (little-endian)
	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint32(bytes[0:], 0x12345678)
	binary.LittleEndian.PutUint32(bytes[4:], 0x0FEDCBA9)

	input := NewByteArrayRandomAccessInput(bytes)

	tests := []struct {
		pos     int64
		want    int32
		wantErr bool
	}{
		{0, 0x12345678, false},
		{4, 0x0FEDCBA9, false},
		{5, 0, true}, // not enough bytes
		{-1, 0, true},
	}

	for _, tt := range tests {
		got, err := input.ReadIntAt(tt.pos)
		if (err != nil) != tt.wantErr {
			t.Errorf("ReadIntAt(%d) error = %v, wantErr %v", tt.pos, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("ReadIntAt(%d) = 0x%08x, want 0x%08x", tt.pos, got, tt.want)
		}
	}
}

func TestByteArrayRandomAccessInput_ReadLongAt(t *testing.T) {
	// Create byte slice with long values (little-endian)
	bytes := make([]byte, 16)
	binary.LittleEndian.PutUint64(bytes[0:], 0x123456789ABCDEF0)
	binary.LittleEndian.PutUint64(bytes[8:], 0x0FEDCBA987654321)

	input := NewByteArrayRandomAccessInput(bytes)

	tests := []struct {
		pos     int64
		want    int64
		wantErr bool
	}{
		{0, 0x123456789ABCDEF0, false},
		{8, 0x0FEDCBA987654321, false},
		{9, 0, true}, // not enough bytes
		{-1, 0, true},
	}

	for _, tt := range tests {
		got, err := input.ReadLongAt(tt.pos)
		if (err != nil) != tt.wantErr {
			t.Errorf("ReadLongAt(%d) error = %v, wantErr %v", tt.pos, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("ReadLongAt(%d) = 0x%016x, want 0x%016x", tt.pos, got, tt.want)
		}
	}
}

func TestByteArrayRandomAccessInput_Slice(t *testing.T) {
	bytes := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	input := NewByteArrayRandomAccessInput(bytes)

	tests := []struct {
		offset  int64
		length  int64
		wantLen int64
		wantErr bool
	}{
		{0, 4, 4, false},
		{2, 4, 4, false},
		{4, 4, 4, false},
		{0, 8, 8, false},
		{8, 0, 0, false},  // empty slice at end
		{-1, 4, 0, true},  // negative offset
		{0, 9, 0, true},   // length exceeds
		{4, 5, 0, true},   // offset + length exceeds
	}

	for _, tt := range tests {
		sliced, err := input.Slice(tt.offset, tt.length)
		if (err != nil) != tt.wantErr {
			t.Errorf("Slice(%d, %d) error = %v, wantErr %v", tt.offset, tt.length, err, tt.wantErr)
			continue
		}
		if tt.wantErr {
			continue
		}
		if sliced.Length() != tt.wantLen {
			t.Errorf("Slice(%d, %d).Length() = %d, want %d", tt.offset, tt.length, sliced.Length(), tt.wantLen)
		}

		// Verify the sliced data
		if tt.length > 0 {
			firstByte, err := sliced.ReadByteAt(0)
			if err != nil {
				t.Errorf("Slice(%d, %d).ReadByteAt(0) error = %v", tt.offset, tt.length, err)
				continue
			}
			if firstByte != bytes[tt.offset] {
				t.Errorf("Slice(%d, %d).ReadByteAt(0) = 0x%02x, want 0x%02x", tt.offset, tt.length, firstByte, bytes[tt.offset])
			}
		}
	}
}

func TestByteArrayRandomAccessInput_Slice_Independent(t *testing.T) {
	bytes := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	input := NewByteArrayRandomAccessInput(bytes)

	// Create a slice
	sliced, err := input.Slice(2, 4)
	if err != nil {
		t.Fatalf("Slice(2, 4) error = %v", err)
	}

	// Verify slice has correct data
	for i := int64(0); i < 4; i++ {
		b, err := sliced.ReadByteAt(i)
		if err != nil {
			t.Errorf("ReadByteAt(%d) error = %v", i, err)
			continue
		}
		if b != bytes[2+i] {
			t.Errorf("ReadByteAt(%d) = 0x%02x, want 0x%02x", i, b, bytes[2+i])
		}
	}
}

func TestByteArrayRandomAccessInput_Empty(t *testing.T) {
	input := NewByteArrayRandomAccessInput([]byte{})

	if input.Length() != 0 {
		t.Errorf("Length() = %d, want 0", input.Length())
	}

	_, err := input.ReadByteAt(0)
	if err == nil {
		t.Error("ReadByteAt(0) on empty input should return error")
	}
}

func TestByteArrayRandomAccessInput_LargeValues(t *testing.T) {
	// Test with larger data
	bytes := make([]byte, 1024)
	for i := range bytes {
		bytes[i] = byte(i % 256)
	}

	input := NewByteArrayRandomAccessInput(bytes)

	// Read at various positions
	positions := []int64{0, 1, 255, 256, 511, 512, 1023}
	for _, pos := range positions {
		b, err := input.ReadByteAt(pos)
		if err != nil {
			t.Errorf("ReadByteAt(%d) error = %v", pos, err)
			continue
		}
		if b != bytes[pos] {
			t.Errorf("ReadByteAt(%d) = 0x%02x, want 0x%02x", pos, b, bytes[pos])
		}
	}
}
