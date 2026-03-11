// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"testing"
)

func TestByteArrayDataOutput(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "write byte",
			fn: func(t *testing.T) {
				out := NewByteArrayDataOutput(10)

				if err := out.WriteByte(0x01); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				bytes := out.GetBytes()
				if len(bytes) != 1 || bytes[0] != 0x01 {
					t.Errorf("expected [0x01], got %v", bytes)
				}
			},
		},
		{
			name: "write bytes",
			fn: func(t *testing.T) {
				out := NewByteArrayDataOutput(10)

				data := []byte{0x01, 0x02, 0x03}
				if err := out.WriteBytes(data); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				bytes := out.GetBytes()
				if len(bytes) != 3 {
					t.Errorf("expected length 3, got %d", len(bytes))
				}
			},
		},
		{
			name: "write bytes n",
			fn: func(t *testing.T) {
				out := NewByteArrayDataOutput(10)

				data := []byte{0x01, 0x02, 0x03, 0x04}
				if err := out.WriteBytesN(data, 2); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				bytes := out.GetBytes()
				if len(bytes) != 2 || bytes[0] != 0x01 || bytes[1] != 0x02 {
					t.Errorf("expected [0x01 0x02], got %v", bytes)
				}
			},
		},
		{
			name: "write bytes n with invalid length",
			fn: func(t *testing.T) {
				out := NewByteArrayDataOutput(10)

				data := []byte{0x01, 0x02}
				if err := out.WriteBytesN(data, 5); err == nil {
					t.Error("expected error for length exceeding slice")
				}
			},
		},
		{
			name: "get position",
			fn: func(t *testing.T) {
				out := NewByteArrayDataOutput(10)

				if out.GetPosition() != 0 {
					t.Errorf("expected position 0, got %d", out.GetPosition())
				}

				out.WriteByte(0x01)
				if out.GetPosition() != 1 {
					t.Errorf("expected position 1, got %d", out.GetPosition())
				}
			},
		},
		{
			name: "reset",
			fn: func(t *testing.T) {
				out := NewByteArrayDataOutput(10)

				out.WriteBytes([]byte{0x01, 0x02, 0x03})
				out.Reset()

				if out.GetPosition() != 0 {
					t.Errorf("expected position 0 after reset, got %d", out.GetPosition())
				}

				if len(out.GetBytes()) != 0 {
					t.Errorf("expected empty bytes after reset")
				}
			},
		},
		{
			name: "length",
			fn: func(t *testing.T) {
				out := NewByteArrayDataOutput(10)

				if out.Length() != 0 {
					t.Errorf("expected length 0, got %d", out.Length())
				}

				out.WriteBytes([]byte{0x01, 0x02, 0x03})
				if out.Length() != 3 {
					t.Errorf("expected length 3, got %d", out.Length())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}

func TestBaseIndexOutput(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "new base index output has correct initial state",
			fn: func(t *testing.T) {
				out := NewBaseIndexOutput("test.txt")

				if out.GetName() != "test.txt" {
					t.Errorf("expected name 'test.txt', got '%s'", out.GetName())
				}

				if out.GetFilePointer() != 0 {
					t.Errorf("expected file pointer 0, got %d", out.GetFilePointer())
				}
			},
		},
		{
			name: "set file pointer",
			fn: func(t *testing.T) {
				out := NewBaseIndexOutput("test.txt")
				out.SetFilePointer(50)

				if out.GetFilePointer() != 50 {
					t.Errorf("expected file pointer 50, got %d", out.GetFilePointer())
				}
			},
		},
		{
			name: "increment file pointer",
			fn: func(t *testing.T) {
				out := NewBaseIndexOutput("test.txt")
				out.IncrementFilePointer(25)

				if out.GetFilePointer() != 25 {
					t.Errorf("expected file pointer 25, got %d", out.GetFilePointer())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}

func TestDataOutputHelpers(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "write uint16",
			fn: func(t *testing.T) {
				out := NewByteArrayDataOutput(10)

				if err := WriteUint16(out, 0x0102); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				bytes := out.GetBytes()
				if len(bytes) != 2 || bytes[0] != 0x01 || bytes[1] != 0x02 {
					t.Errorf("expected [0x01 0x02], got %v", bytes)
				}
			},
		},
		{
			name: "write uint32",
			fn: func(t *testing.T) {
				out := NewByteArrayDataOutput(10)

				if err := WriteUint32(out, 0x01020304); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				bytes := out.GetBytes()
				if len(bytes) != 4 {
					t.Errorf("expected length 4, got %d", len(bytes))
				}
			},
		},
		{
			name: "write uint64",
			fn: func(t *testing.T) {
				out := NewByteArrayDataOutput(10)

				if err := WriteUint64(out, 0x0102030405060708); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				bytes := out.GetBytes()
				if len(bytes) != 8 {
					t.Errorf("expected length 8, got %d", len(bytes))
				}
			},
		},
		{
			name: "write vint small value",
			fn: func(t *testing.T) {
				out := NewByteArrayDataOutput(10)

				if err := WriteVInt(out, 100); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				bytes := out.GetBytes()
				if len(bytes) != 1 || bytes[0] != 100 {
					t.Errorf("expected [100], got %v", bytes)
				}
			},
		},
		{
			name: "write vint large value",
			fn: func(t *testing.T) {
				out := NewByteArrayDataOutput(10)

				if err := WriteVInt(out, 128); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				bytes := out.GetBytes()
				// 128 = 0x80 0x01
				if len(bytes) != 2 || bytes[0] != 0x80 || bytes[1] != 0x01 {
					t.Errorf("expected [0x80 0x01], got %v", bytes)
				}
			},
		},
		{
			name: "write vlong",
			fn: func(t *testing.T) {
				out := NewByteArrayDataOutput(10)

				if err := WriteVLong(out, 128); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				bytes := out.GetBytes()
				if len(bytes) != 2 || bytes[0] != 0x80 || bytes[1] != 0x01 {
					t.Errorf("expected [0x80 0x01], got %v", bytes)
				}
			},
		},
		{
			name: "write string",
			fn: func(t *testing.T) {
				out := NewByteArrayDataOutput(10)

				if err := WriteString(out, "hi"); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				bytes := out.GetBytes()
				// length 2 + 'h' 'i'
				if len(bytes) != 3 || bytes[0] != 2 || bytes[1] != 'h' || bytes[2] != 'i' {
					t.Errorf("expected [2 'h' 'i'], got %v", bytes)
				}
			},
		},
		{
			name: "write empty string",
			fn: func(t *testing.T) {
				out := NewByteArrayDataOutput(10)

				if err := WriteString(out, ""); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				bytes := out.GetBytes()
				if len(bytes) != 1 || bytes[0] != 0 {
					t.Errorf("expected [0], got %v", bytes)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}

func TestIndexOutputWithDigest(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "digest is computed",
			fn: func(t *testing.T) {
				base := &mockIndexOutput{name: "test"}
				out := NewIndexOutputWithDigest(base)

				out.WriteByte(0x01)
				out.WriteByte(0x02)

				digest1 := out.GetDigest()
				if digest1 == 0 {
					t.Error("expected non-zero digest")
				}

				out.WriteByte(0x03)

				digest2 := out.GetDigest()
				if digest1 == digest2 {
					t.Error("expected different digest after more writes")
				}
			},
		},
		{
			name: "reset digest",
			fn: func(t *testing.T) {
				base := &mockIndexOutput{name: "test"}
				out := NewIndexOutputWithDigest(base)

				out.WriteBytes([]byte{0x01, 0x02, 0x03})
				digest1 := out.GetDigest()

				out.ResetDigest()
				digest2 := out.GetDigest()

				if digest1 == digest2 {
					t.Error("expected different digest after reset")
				}
				if digest2 != 1 { // Adler-32 initial value
					t.Errorf("expected initial digest value, got %d", digest2)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}

// mockIndexOutput is a mock implementation for testing
type mockIndexOutput struct {
	name string
	data []byte
	pos  int64
}

func (m *mockIndexOutput) WriteByte(b byte) error {
	m.data = append(m.data, b)
	m.pos++
	return nil
}

func (m *mockIndexOutput) WriteBytes(b []byte) error {
	m.data = append(m.data, b...)
	m.pos += int64(len(b))
	return nil
}

func (m *mockIndexOutput) WriteBytesN(b []byte, n int) error {
	m.data = append(m.data, b[:n]...)
	m.pos += int64(n)
	return nil
}

func (m *mockIndexOutput) GetFilePointer() int64 { return m.pos }
func (m *mockIndexOutput) Length() int64         { return int64(len(m.data)) }
func (m *mockIndexOutput) GetName() string       { return m.name }
