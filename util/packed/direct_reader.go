// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"fmt"
)

// RandomAccessInput is the subset of store.RandomAccessInput used by
// DirectReader / DirectMonotonicReader. It is declared here instead
// of importing store directly so that callers can supply any
// little-endian random-access source.
type RandomAccessInput interface {
	ReadByteAt(pos int64) (byte, error)
	ReadShortAt(pos int64) (int16, error)
	ReadIntAt(pos int64) (int32, error)
	ReadLongAt(pos int64) (int64, error)
}

// LongValues is the read-only function-style view of a long array
// used by DirectReader and PackedLongValues.
//
// Implementations must be safe for repeated calls and may perform
// arbitrary I/O.
type LongValues interface {
	// Get returns the long at the given index.
	Get(index int64) int64
}

// GetDirectReader returns a LongValues backed by the bytes of the
// DirectWriter output located in slice starting at offset = 0.
func GetDirectReader(slice RandomAccessInput, bitsPerValue int) (LongValues, error) {
	return GetDirectReaderAt(slice, bitsPerValue, 0)
}

// GetDirectReaderAt returns a LongValues whose first value is read
// from slice at the given byte offset.
func GetDirectReaderAt(slice RandomAccessInput, bitsPerValue int, offset int64) (LongValues, error) {
	switch bitsPerValue {
	case 1:
		return &directReader1{in: slice, offset: offset}, nil
	case 2:
		return &directReader2{in: slice, offset: offset}, nil
	case 4:
		return &directReader4{in: slice, offset: offset}, nil
	case 8:
		return &directReader8{in: slice, offset: offset}, nil
	case 12:
		return &directReader12{in: slice, offset: offset}, nil
	case 16:
		return &directReader16{in: slice, offset: offset}, nil
	case 20:
		return &directReader20{in: slice, offset: offset}, nil
	case 24:
		return &directReader24{in: slice, offset: offset}, nil
	case 28:
		return &directReader28{in: slice, offset: offset}, nil
	case 32:
		return &directReader32{in: slice, offset: offset}, nil
	case 40:
		return &directReader40{in: slice, offset: offset}, nil
	case 48:
		return &directReader48{in: slice, offset: offset}, nil
	case 56:
		return &directReader56{in: slice, offset: offset}, nil
	case 64:
		return &directReader64{in: slice, offset: offset}, nil
	default:
		return nil, fmt.Errorf("packed: unsupported bitsPerValue: %d", bitsPerValue)
	}
}

// directReader1 reads 1-bit values.
type directReader1 struct {
	in     RandomAccessInput
	offset int64
}

func (r *directReader1) Get(index int64) int64 {
	shift := uint(index & 7)
	b, err := r.in.ReadByteAt(r.offset + index>>3)
	if err != nil {
		panic(err)
	}
	return int64((uint64(b) >> shift) & 0x1)
}

type directReader2 struct {
	in     RandomAccessInput
	offset int64
}

func (r *directReader2) Get(index int64) int64 {
	shift := uint((index & 3) << 1)
	b, err := r.in.ReadByteAt(r.offset + index>>2)
	if err != nil {
		panic(err)
	}
	return int64((uint64(b) >> shift) & 0x3)
}

type directReader4 struct {
	in     RandomAccessInput
	offset int64
}

func (r *directReader4) Get(index int64) int64 {
	shift := uint((index & 1) << 2)
	b, err := r.in.ReadByteAt(r.offset + index>>1)
	if err != nil {
		panic(err)
	}
	return int64((uint64(b) >> shift) & 0xF)
}

type directReader8 struct {
	in     RandomAccessInput
	offset int64
}

func (r *directReader8) Get(index int64) int64 {
	b, err := r.in.ReadByteAt(r.offset + index)
	if err != nil {
		panic(err)
	}
	return int64(uint64(b) & 0xFF)
}

type directReader12 struct {
	in     RandomAccessInput
	offset int64
}

func (r *directReader12) Get(index int64) int64 {
	off := (index * 12) >> 3
	shift := uint((index & 1) << 2)
	s, err := r.in.ReadShortAt(r.offset + off)
	if err != nil {
		panic(err)
	}
	return int64((uint64(uint16(s)) >> shift) & 0xFFF)
}

type directReader16 struct {
	in     RandomAccessInput
	offset int64
}

func (r *directReader16) Get(index int64) int64 {
	s, err := r.in.ReadShortAt(r.offset + index<<1)
	if err != nil {
		panic(err)
	}
	return int64(uint64(uint16(s)) & 0xFFFF)
}

type directReader20 struct {
	in     RandomAccessInput
	offset int64
}

func (r *directReader20) Get(index int64) int64 {
	off := (index * 20) >> 3
	shift := uint((index & 1) << 2)
	i, err := r.in.ReadIntAt(r.offset + off)
	if err != nil {
		panic(err)
	}
	return int64((uint64(uint32(i)) >> shift) & 0xFFFFF)
}

type directReader24 struct {
	in     RandomAccessInput
	offset int64
}

func (r *directReader24) Get(index int64) int64 {
	i, err := r.in.ReadIntAt(r.offset + index*3)
	if err != nil {
		panic(err)
	}
	return int64(uint64(uint32(i)) & 0xFFFFFF)
}

type directReader28 struct {
	in     RandomAccessInput
	offset int64
}

func (r *directReader28) Get(index int64) int64 {
	off := (index * 28) >> 3
	shift := uint((index & 1) << 2)
	i, err := r.in.ReadIntAt(r.offset + off)
	if err != nil {
		panic(err)
	}
	return int64((uint64(uint32(i)) >> shift) & 0xFFFFFFF)
}

type directReader32 struct {
	in     RandomAccessInput
	offset int64
}

func (r *directReader32) Get(index int64) int64 {
	i, err := r.in.ReadIntAt(r.offset + index<<2)
	if err != nil {
		panic(err)
	}
	return int64(uint64(uint32(i)) & 0xFFFFFFFF)
}

type directReader40 struct {
	in     RandomAccessInput
	offset int64
}

func (r *directReader40) Get(index int64) int64 {
	l, err := r.in.ReadLongAt(r.offset + index*5)
	if err != nil {
		panic(err)
	}
	return int64(uint64(l) & 0xFFFFFFFFFF)
}

type directReader48 struct {
	in     RandomAccessInput
	offset int64
}

func (r *directReader48) Get(index int64) int64 {
	l, err := r.in.ReadLongAt(r.offset + index*6)
	if err != nil {
		panic(err)
	}
	return int64(uint64(l) & 0xFFFFFFFFFFFF)
}

type directReader56 struct {
	in     RandomAccessInput
	offset int64
}

func (r *directReader56) Get(index int64) int64 {
	l, err := r.in.ReadLongAt(r.offset + index*7)
	if err != nil {
		panic(err)
	}
	return int64(uint64(l) & 0xFFFFFFFFFFFFFF)
}

type directReader64 struct {
	in     RandomAccessInput
	offset int64
}

func (r *directReader64) Get(index int64) int64 {
	l, err := r.in.ReadLongAt(r.offset + index<<3)
	if err != nil {
		panic(err)
	}
	return l
}
