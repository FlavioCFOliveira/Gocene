// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"testing"
)

func TestNewBufferedIndexOutput(t *testing.T) {
	// Test with default buffer size
	out := NewBufferedIndexOutput("test", 0)
	if out == nil {
		t.Fatal("NewBufferedIndexOutput() returned nil")
	}
	if out.GetBufferSize() != 1024 {
		t.Errorf("GetBufferSize() = %d, want 1024", out.GetBufferSize())
	}

	// Test with custom buffer size
	out2 := NewBufferedIndexOutput("test2", 2048)
	if out2.GetBufferSize() != 2048 {
		t.Errorf("GetBufferSize() = %d, want 2048", out2.GetBufferSize())
	}
}

func TestBufferedIndexOutput_GetBufferSize(t *testing.T) {
	out := NewBufferedIndexOutput("test", 2048)

	if out.GetBufferSize() != 2048 {
		t.Errorf("GetBufferSize() = %d, want 2048", out.GetBufferSize())
	}
}

func TestBufferedIndexOutput_SetBufferSize_Invalid(t *testing.T) {
	out := NewBufferedIndexOutput("test", 1024)

	// Set to invalid size (0 or negative) should default to 1024
	// Note: SetBufferSize calls Flush which will fail since writeInternal is not implemented
	// but the buffer size should still be set to the default
	_ = out.SetBufferSize(0)

	if out.GetBufferSize() != 1024 {
		t.Errorf("GetBufferSize() = %d, want 1024 after setting to 0", out.GetBufferSize())
	}
}

func TestBufferedIndexOutput_WriteBytesN_InvalidLength(t *testing.T) {
	out := NewBufferedIndexOutput("test", 1024)

	data := []byte("Hello")
	// Try to write more bytes than available
	err := out.WriteBytesN(data, 10)
	if err == nil {
		t.Error("WriteBytesN() with invalid length should return error")
	}
}

func TestBufferedIndexOutput_WriteBytesN_NegativeLength(t *testing.T) {
	out := NewBufferedIndexOutput("test", 1024)

	data := []byte("Hello")
	// Try to write negative bytes
	err := out.WriteBytesN(data, -1)
	if err == nil {
		t.Error("WriteBytesN() with negative length should return error")
	}
}

func TestBufferedIndexOutput_BufferedOutputInterface(t *testing.T) {
	// Verify BufferedIndexOutput implements BufferedOutput interface
	var _ BufferedOutput = (*BufferedIndexOutput)(nil)
}

func TestBufferedIndexOutput_DataOutputInterface(t *testing.T) {
	// Verify BufferedIndexOutput implements DataOutput interface
	var _ DataOutput = (*BufferedIndexOutput)(nil)
}
