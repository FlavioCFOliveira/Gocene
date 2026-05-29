// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"bytes"
	"fmt"
	"math"
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

// TestWriteVIntNegativeRegression guards against the arithmetic-right-shift
// bug (rmp #4745): WriteVInt/WriteVLong used a signed ">>= 7" which
// sign-extends negative inputs and loops forever. Java DataOutput.writeVInt
// uses an unsigned ">>>= 7" and emits exactly five bytes for negatives. This
// test would hang (and the suite would be SIGKILLed) under the old code.
func TestWriteVIntNegativeRegression(t *testing.T) {
	// Golden bytes for WriteVInt(-1) per Lucene 10.4.0 (uint32 logical shift):
	// 0xFFFFFFFF -> ff ff ff ff 0f.
	want := []byte{0xff, 0xff, 0xff, 0xff, 0x0f}

	t.Run("package WriteVInt(-1) golden bytes", func(t *testing.T) {
		out := NewByteArrayDataOutput(8)
		if err := WriteVInt(out, -1); err != nil {
			t.Fatalf("WriteVInt(-1): %v", err)
		}
		if got := out.GetBytes(); !bytes.Equal(got, want) {
			t.Fatalf("WriteVInt(-1) = %v, want %v", got, want)
		}
	})

	t.Run("method WriteVInt(-1) golden bytes", func(t *testing.T) {
		out := NewByteArrayDataOutput(8)
		if err := out.WriteVInt(-1); err != nil {
			t.Fatalf("(*ByteArrayDataOutput).WriteVInt(-1): %v", err)
		}
		if got := out.GetBytes(); !bytes.Equal(got, want) {
			t.Fatalf("method WriteVInt(-1) = %v, want %v", got, want)
		}
	})

	// VInt round-trips for the full int32 range, negatives included.
	vintCases := []int32{0, 1, 127, 128, 300, -1, -128, math.MinInt32, math.MaxInt32}
	t.Run("VInt round-trip", func(t *testing.T) {
		for _, v := range vintCases {
			out := NewByteArrayDataOutput(8)
			if err := WriteVInt(out, v); err != nil {
				t.Fatalf("WriteVInt(%d): %v", v, err)
			}
			in := NewByteArrayDataInput(out.GetBytes())
			got, err := in.ReadVInt()
			if err != nil {
				t.Fatalf("ReadVInt after WriteVInt(%d): %v", v, err)
			}
			if got != v {
				t.Errorf("VInt round-trip: wrote %d, read %d", v, got)
			}
		}
	})

	// VLong round-trips for the non-negative domain (Lucene rejects negative
	// vLong). For negatives we only require termination + deterministic output,
	// not round-trip: WriteVLong(-1) emits nine 0xff bytes then 0x01.
	vlongCases := []int64{0, 1, 127, 128, 1 << 40, math.MaxInt64}
	t.Run("VLong round-trip (non-negative)", func(t *testing.T) {
		for _, v := range vlongCases {
			out := NewByteArrayDataOutput(16)
			if err := WriteVLong(out, v); err != nil {
				t.Fatalf("WriteVLong(%d): %v", v, err)
			}
			in := NewByteArrayDataInput(out.GetBytes())
			got, err := in.ReadVLong()
			if err != nil {
				t.Fatalf("ReadVLong after WriteVLong(%d): %v", v, err)
			}
			if got != v {
				t.Errorf("VLong round-trip: wrote %d, read %d", v, got)
			}
		}
	})

	t.Run("WriteVLong(-1) terminates with deterministic bytes", func(t *testing.T) {
		out := NewByteArrayDataOutput(16)
		if err := WriteVLong(out, -1); err != nil {
			t.Fatalf("WriteVLong(-1): %v", err)
		}
		wantLong := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01}
		if got := out.GetBytes(); !bytes.Equal(got, wantLong) {
			t.Fatalf("WriteVLong(-1) = %v, want %v", got, wantLong)
		}
	})
}

// TestWriteMapOfStringsDeterministic exercises rmp #4784: WriteMapOfStrings
// must emit a multi-entry map in a stable (key-sorted) order so the same
// logical map serialises byte-identically across runs, matching the behaviour
// of WriteSetOfStrings/WriteMapOfIntToSetOfStrings.
func TestWriteMapOfStringsDeterministic(t *testing.T) {
	m := map[string]string{
		"os": "linux", "java": "21", "source": "flush",
		"lucene": "10.4.1", "timestamp": "0", "os.arch": "aarch64",
		"alpha": "a", "zeta": "z", "mid": "m",
	}

	encode := func() []byte {
		out := NewByteArrayDataOutput(256)
		if err := WriteMapOfStrings(out, m); err != nil {
			t.Fatalf("WriteMapOfStrings: %v", err)
		}
		return out.GetBytes()
	}

	first := encode()
	for i := 0; i < 32; i++ {
		if got := encode(); !bytes.Equal(first, got) {
			t.Fatalf("non-deterministic WriteMapOfStrings bytes on iteration %d", i)
		}
	}

	// Round-trip recovers the logical map regardless of serialisation order.
	in := NewByteArrayDataInput(first)
	got, err := ReadMapOfStrings(in)
	if err != nil {
		t.Fatalf("ReadMapOfStrings: %v", err)
	}
	if len(got) != len(m) {
		t.Fatalf("round-trip size: want %d, got %d", len(m), len(got))
	}
	for k, v := range m {
		if got[k] != v {
			t.Errorf("round-trip[%q]: want %q, got %q", k, v, got[k])
		}
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
func (m *mockIndexOutput) SetPosition(pos int64) error {
	if pos < 0 || pos > int64(len(m.data)) {
		return fmt.Errorf("invalid position: %d", pos)
	}
	m.pos = pos
	return nil
}
func (m *mockIndexOutput) Length() int64   { return int64(len(m.data)) }
func (m *mockIndexOutput) GetName() string { return m.name }
func (m *mockIndexOutput) Close() error    { return nil }

func (m *mockIndexOutput) WriteShort(i int16) error {
	m.data = append(m.data, byte(i>>8), byte(i))
	m.pos += 2
	return nil
}

func (m *mockIndexOutput) WriteInt(i int32) error {
	m.data = append(m.data, byte(i>>24), byte(i>>16), byte(i>>8), byte(i))
	m.pos += 4
	return nil
}

func (m *mockIndexOutput) WriteLong(i int64) error {
	m.data = append(m.data,
		byte(i>>56), byte(i>>48), byte(i>>40), byte(i>>32),
		byte(i>>24), byte(i>>16), byte(i>>8), byte(i))
	m.pos += 8
	return nil
}

func (m *mockIndexOutput) WriteString(s string) error {
	// VInt encoding for string length
	length := len(s)
	if length < 128 {
		m.data = append(m.data, byte(length))
		m.pos++
	} else if length < 16384 {
		m.data = append(m.data, byte((length>>7)|0x80), byte(length&0x7F))
		m.pos += 2
	} else {
		// Simplified: just use 2 bytes for now
		m.data = append(m.data, byte((length>>7)|0x80), byte(length&0x7F))
		m.pos += 2
	}
	m.data = append(m.data, s...)
	m.pos += int64(len(s))
	return nil
}
