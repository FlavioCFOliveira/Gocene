// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"io"
	"testing"
)

func TestByteArrayDataInput(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "read byte",
			fn: func(t *testing.T) {
				data := []byte{0x01, 0x02, 0x03}
				in := NewByteArrayDataInput(data)

				b, err := in.ReadByte()
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if b != 0x01 {
					t.Errorf("expected 0x01, got 0x%02x", b)
				}
			},
		},
		{
			name: "read bytes",
			fn: func(t *testing.T) {
				data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
				in := NewByteArrayDataInput(data)

				buf := make([]byte, 3)
				if err := in.ReadBytes(buf); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				expected := []byte{0x01, 0x02, 0x03}
				for i, b := range buf {
					if b != expected[i] {
						t.Errorf("byte %d: expected 0x%02x, got 0x%02x", i, expected[i], b)
					}
				}
			},
		},
		{
			name: "read bytes n",
			fn: func(t *testing.T) {
				data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
				in := NewByteArrayDataInput(data)

				result, err := in.ReadBytesN(3)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if len(result) != 3 {
					t.Errorf("expected length 3, got %d", len(result))
				}
			},
		},
		{
			name: "read past end returns EOF",
			fn: func(t *testing.T) {
				data := []byte{0x01}
				in := NewByteArrayDataInput(data)

				_, err := in.ReadByte()
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				_, err = in.ReadByte()
				if err != io.EOF {
					t.Errorf("expected EOF, got %v", err)
				}
			},
		},
		{
			name: "get and set position",
			fn: func(t *testing.T) {
				data := []byte{0x01, 0x02, 0x03}
				in := NewByteArrayDataInput(data)

				if in.GetPosition() != 0 {
					t.Errorf("expected position 0, got %d", in.GetPosition())
				}

				if err := in.SetPosition(2); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if in.GetPosition() != 2 {
					t.Errorf("expected position 2, got %d", in.GetPosition())
				}
			},
		},
		{
			name: "set position out of range returns error",
			fn: func(t *testing.T) {
				data := []byte{0x01, 0x02}
				in := NewByteArrayDataInput(data)

				if err := in.SetPosition(10); err == nil {
					t.Error("expected error for out of range position")
				}
			},
		},
		{
			name: "length returns correct value",
			fn: func(t *testing.T) {
				data := []byte{0x01, 0x02, 0x03}
				in := NewByteArrayDataInput(data)

				if in.Length() != 3 {
					t.Errorf("expected length 3, got %d", in.Length())
				}
			},
		},
		{
			name: "reset",
			fn: func(t *testing.T) {
				data := []byte{0x01, 0x02, 0x03}
				in := NewByteArrayDataInput(data)

				in.ReadByte()
				in.ReadByte()

				newData := []byte{0x0A, 0x0B}
				in.Reset(newData)

				if in.GetPosition() != 0 {
					t.Errorf("expected position 0 after reset, got %d", in.GetPosition())
				}

				if in.Length() != 2 {
					t.Errorf("expected length 2 after reset, got %d", in.Length())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}

func TestBaseIndexInput(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "new base index input has correct initial state",
			fn: func(t *testing.T) {
				bi := NewBaseIndexInput("test", 100)

				if bi.GetDescription() != "test" {
					t.Errorf("expected description 'test', got '%s'", bi.GetDescription())
				}

				if bi.Length() != 100 {
					t.Errorf("expected length 100, got %d", bi.Length())
				}

				if bi.GetFilePointer() != 0 {
					t.Errorf("expected file pointer 0, got %d", bi.GetFilePointer())
				}
			},
		},
		{
			name: "set file pointer",
			fn: func(t *testing.T) {
				bi := NewBaseIndexInput("test", 100)
				bi.SetFilePointer(50)

				if bi.GetFilePointer() != 50 {
					t.Errorf("expected file pointer 50, got %d", bi.GetFilePointer())
				}
			},
		},
		{
			name: "validate seek with negative position",
			fn: func(t *testing.T) {
				bi := NewBaseIndexInput("test", 100)

				if err := bi.ValidateSeek(-1); err == nil {
					t.Error("expected error for negative position")
				}
			},
		},
		{
			name: "validate seek with position exceeding length",
			fn: func(t *testing.T) {
				bi := NewBaseIndexInput("test", 100)

				if err := bi.ValidateSeek(101); err == nil {
					t.Error("expected error for position exceeding length")
				}
			},
		},
		{
			name: "validate seek with valid position",
			fn: func(t *testing.T) {
				bi := NewBaseIndexInput("test", 100)

				if err := bi.ValidateSeek(50); err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			},
		},
		{
			name: "validate slice with negative offset",
			fn: func(t *testing.T) {
				bi := NewBaseIndexInput("test", 100)

				if err := bi.ValidateSlice(-1, 10); err == nil {
					t.Error("expected error for negative offset")
				}
			},
		},
		{
			name: "validate slice with negative length",
			fn: func(t *testing.T) {
				bi := NewBaseIndexInput("test", 100)

				if err := bi.ValidateSlice(0, -1); err == nil {
					t.Error("expected error for negative length")
				}
			},
		},
		{
			name: "validate slice exceeding length",
			fn: func(t *testing.T) {
				bi := NewBaseIndexInput("test", 100)

				if err := bi.ValidateSlice(50, 60); err == nil {
					t.Error("expected error for slice exceeding length")
				}
			},
		},
		{
			name: "skip bytes",
			fn: func(t *testing.T) {
				bi := NewBaseIndexInput("test", 100)
				bi.SetFilePointer(10)

				if err := bi.SkipBytes(20); err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				if bi.GetFilePointer() != 30 {
					t.Errorf("expected file pointer 30, got %d", bi.GetFilePointer())
				}
			},
		},
		{
			name: "skip bytes with negative count",
			fn: func(t *testing.T) {
				bi := NewBaseIndexInput("test", 100)

				if err := bi.SkipBytes(-1); err == nil {
					t.Error("expected error for negative skip")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}

func TestDataInputHelpers(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "read uint16",
			fn: func(t *testing.T) {
				// Big-endian: 0x01 0x02 = 0x0102 = 258
				data := []byte{0x01, 0x02}
				in := NewByteArrayDataInput(data)

				v, err := ReadUint16(in)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if v != 0x0102 {
					t.Errorf("expected 0x0102, got 0x%04x", v)
				}
			},
		},
		{
			name: "read uint32",
			fn: func(t *testing.T) {
				data := []byte{0x01, 0x02, 0x03, 0x04}
				in := NewByteArrayDataInput(data)

				v, err := ReadUint32(in)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if v != 0x01020304 {
					t.Errorf("expected 0x01020304, got 0x%08x", v)
				}
			},
		},
		{
			name: "read uint64",
			fn: func(t *testing.T) {
				data := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
				in := NewByteArrayDataInput(data)

				v, err := ReadUint64(in)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if v != 0x0102030405060708 {
					t.Errorf("expected 0x0102030405060708, got 0x%016x", v)
				}
			},
		},
		{
			name: "read vint small value",
			fn: func(t *testing.T) {
				// Single byte for values < 128
				data := []byte{0x64} // 100
				in := NewByteArrayDataInput(data)

				v, err := ReadVInt(in)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if v != 100 {
					t.Errorf("expected 100, got %d", v)
				}
			},
		},
		{
			name: "read vint large value",
			fn: func(t *testing.T) {
				// Multi-byte for larger values
				// 0x80 + high bit set, 0x02 = (0x00 << 7) | 0x02 = 2
				// Actually let's use a simpler case: 128 = 0x80 0x01
				// 0x80 means high bit set, continue
				// 0x01 = 1, so total = (1 << 7) | 0 = 128
				data := []byte{0x80, 0x01}
				in := NewByteArrayDataInput(data)

				v, err := ReadVInt(in)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if v != 128 {
					t.Errorf("expected 128, got %d", v)
				}
			},
		},
		{
			name: "read string",
			fn: func(t *testing.T) {
				// Length (vint) + bytes
				// "hi" = length 2 (0x02) + 'h' 'i'
				data := []byte{0x02, 'h', 'i'}
				in := NewByteArrayDataInput(data)

				s, err := ReadString(in)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if s != "hi" {
					t.Errorf("expected 'hi', got '%s'", s)
				}
			},
		},
		{
			name: "read empty string",
			fn: func(t *testing.T) {
				data := []byte{0x00}
				in := NewByteArrayDataInput(data)

				s, err := ReadString(in)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if s != "" {
					t.Errorf("expected empty string, got '%s'", s)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}
