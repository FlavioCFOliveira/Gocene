// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktree

import (
	"io"
	"testing"
)

// byteSliceReader implements byteReader backed by a byte slice.
type byteSliceReader struct {
	data []byte
	pos  int
}

func (r *byteSliceReader) ReadByte() (byte, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	b := r.data[r.pos]
	r.pos++
	return b, nil
}

func TestReadMSBVLong_SingleByte(t *testing.T) {
	// 0x7f = 127, MSB = 0 (single byte)
	r := &byteSliceReader{data: []byte{0x7f}}
	v, err := readMSBVLong(r)
	if err != nil {
		t.Fatalf("readMSBVLong: %v", err)
	}
	if v != 127 {
		t.Fatalf("got %d, want 127", v)
	}
}

func TestReadMSBVLong_TwoBytes(t *testing.T) {
	// 0x81 0x00 = 128 (MSB set on first byte)
	r := &byteSliceReader{data: []byte{0x81, 0x00}}
	v, err := readMSBVLong(r)
	if err != nil {
		t.Fatalf("readMSBVLong: %v", err)
	}
	if v != 128 {
		t.Fatalf("got %d, want 128", v)
	}
}

func TestReadMSBVLong_MaxSingleByte(t *testing.T) {
	// 0x7f = max single-byte value
	r := &byteSliceReader{data: []byte{0x7f}}
	v, err := readMSBVLong(r)
	if err != nil {
		t.Fatalf("readMSBVLong: %v", err)
	}
	if v != 127 {
		t.Fatalf("got %d, want 127", v)
	}
}

func TestReadMSBVLong_Zero(t *testing.T) {
	r := &byteSliceReader{data: []byte{0x00}}
	v, err := readMSBVLong(r)
	if err != nil {
		t.Fatalf("readMSBVLong: %v", err)
	}
	if v != 0 {
		t.Fatalf("got %d, want 0", v)
	}
}

func TestReadMSBVLong_EOF(t *testing.T) {
	r := &byteSliceReader{data: []byte{}}
	_, err := readMSBVLong(r)
	if err == nil {
		t.Fatal("expected EOF on empty input")
	}
}

func TestReadVLongFromBytes_SingleByte(t *testing.T) {
	r := &byteSliceReader{data: []byte{0x7f}}
	v, err := readVLongFromBytes(r)
	if err != nil {
		t.Fatalf("readVLongFromBytes: %v", err)
	}
	if v != 127 {
		t.Fatalf("got %d, want 127", v)
	}
}

func TestReadVLongFromBytes_TwoBytes(t *testing.T) {
	// 0x80 | 0x01, 0x7f = ((1 & 0x7f) | ((127 & 0x7f) << 7)) = 1 | 16256 = 16257
	r := &byteSliceReader{data: []byte{0x81, 0x7f}}
	v, err := readVLongFromBytes(r)
	if err != nil {
		t.Fatalf("readVLongFromBytes: %v", err)
	}
	if v != 16257 {
		t.Fatalf("got %d, want 16257", v)
	}
}

func TestReadVLongFromBytes_Zero(t *testing.T) {
	r := &byteSliceReader{data: []byte{0x00}}
	v, err := readVLongFromBytes(r)
	if err != nil {
		t.Fatalf("readVLongFromBytes: %v", err)
	}
	if v != 0 {
		t.Fatalf("got %d, want 0", v)
	}
}

func TestReadVLongFromBytes_EOF(t *testing.T) {
	r := &byteSliceReader{data: []byte{}}
	_, err := readVLongFromBytes(r)
	if err == nil {
		t.Fatal("expected EOF on empty input")
	}
}

func TestReadVLongFromBytes_LargeValue(t *testing.T) {
	// Three bytes: 0x80 0x80 0x01 = (1<<14) + 0 = 16384
	r := &byteSliceReader{data: []byte{0x80, 0x80, 0x01}}
	v, err := readVLongFromBytes(r)
	if err != nil {
		t.Fatalf("readVLongFromBytes: %v", err)
	}
	if v != 16384 {
		t.Fatalf("got %d, want 16384", v)
	}
}

// TestVersionMSBVLongConstant verifies that the constant value is used to
// select the MSB VLong encoding path.
func TestVersionMSBVLongConstant(t *testing.T) {
	if versionMSBVLongOutput != 1 {
		t.Fatalf("versionMSBVLongOutput constant changed: got %d, want 1", versionMSBVLongOutput)
	}
}
