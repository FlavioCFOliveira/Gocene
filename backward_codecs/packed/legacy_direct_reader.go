// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

// Ported from Apache Lucene 10.5.0:
//
//	lucene/backward-codecs/src/java/org/apache/lucene/backward_codecs/packed/LegacyDirectReader.java

package packed

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// LegacyDirectReaderGetInstance retrieves an instance previously written by
// LegacyDirectWriter, reading from the start of slice.
//
// Mirrors {@code public static LongValues getInstance(RandomAccessInput slice,
// int bitsPerValue)}, whose body is
// {@code return getInstance(slice, bitsPerValue, 0);}.
//
// Java declares getInstance on the final class LegacyDirectReader, whose
// constructor is private ("no instances"). Go has no static methods, so the
// two overloads are package functions carrying the class name.
func LegacyDirectReaderGetInstance(slice store.RandomAccessInput, bitsPerValue int) (util.LongValues, error) {
	return LegacyDirectReaderGetInstanceAt(slice, bitsPerValue, 0)
}

// LegacyDirectReaderGetInstanceAt retrieves an instance previously written by
// LegacyDirectWriter, reading from the given byte offset of slice.
//
// Mirrors {@code public static LongValues getInstance(RandomAccessInput slice,
// int bitsPerValue, long offset)}.
//
// NOTE: unlike org.apache.lucene.util.packed.DirectReader, this legacy reader
// decodes the BIG-endian layout Lucene wrote before 9.0, which is why every
// accessor below shifts down rather than masking up. Callers supply a slice
// whose multi-byte reads are byte-swapped (see
// backward_codecs/store.EndiannessReverserRandomAccessInput).
func LegacyDirectReaderGetInstanceAt(slice store.RandomAccessInput, bitsPerValue int, offset int64) (util.LongValues, error) {
	switch bitsPerValue {
	case 1:
		return &legacyDirectPackedReader1{in: slice, offset: offset}, nil
	case 2:
		return &legacyDirectPackedReader2{in: slice, offset: offset}, nil
	case 4:
		return &legacyDirectPackedReader4{in: slice, offset: offset}, nil
	case 8:
		return &legacyDirectPackedReader8{in: slice, offset: offset}, nil
	case 12:
		return &legacyDirectPackedReader12{in: slice, offset: offset}, nil
	case 16:
		return &legacyDirectPackedReader16{in: slice, offset: offset}, nil
	case 20:
		return &legacyDirectPackedReader20{in: slice, offset: offset}, nil
	case 24:
		return &legacyDirectPackedReader24{in: slice, offset: offset}, nil
	case 28:
		return &legacyDirectPackedReader28{in: slice, offset: offset}, nil
	case 32:
		return &legacyDirectPackedReader32{in: slice, offset: offset}, nil
	case 40:
		return &legacyDirectPackedReader40{in: slice, offset: offset}, nil
	case 48:
		return &legacyDirectPackedReader48{in: slice, offset: offset}, nil
	case 56:
		return &legacyDirectPackedReader56{in: slice, offset: offset}, nil
	case 64:
		return &legacyDirectPackedReader64{in: slice, offset: offset}, nil
	default:
		return nil, fmt.Errorf("unsupported bitsPerValue: %d", bitsPerValue)
	}
}

// legacyDirectPackedReader1 mirrors the nested class
// LegacyDirectReader.DirectPackedReader1.
type legacyDirectPackedReader1 struct {
	in     store.RandomAccessInput
	offset int64
}

// Get reproduces
//
//	int shift = 7 - (int) (index & 7);
//	return (in.readByte(offset + (index >>> 3)) >>> shift) & 0x1;
func (r *legacyDirectPackedReader1) Get(index int64) (int64, error) {
	shift := uint(7 - (index & 7))
	b, err := r.in.ReadByteAt(r.offset + int64(uint64(index)>>3))
	if err != nil {
		return 0, err
	}
	return int64((uint64(b) >> shift) & 0x1), nil
}

// legacyDirectPackedReader2 mirrors LegacyDirectReader.DirectPackedReader2.
type legacyDirectPackedReader2 struct {
	in     store.RandomAccessInput
	offset int64
}

// Get reproduces
//
//	int shift = (3 - (int) (index & 3)) << 1;
//	return (in.readByte(offset + (index >>> 2)) >>> shift) & 0x3;
func (r *legacyDirectPackedReader2) Get(index int64) (int64, error) {
	shift := uint((3 - (index & 3)) << 1)
	b, err := r.in.ReadByteAt(r.offset + int64(uint64(index)>>2))
	if err != nil {
		return 0, err
	}
	return int64((uint64(b) >> shift) & 0x3), nil
}

// legacyDirectPackedReader4 mirrors LegacyDirectReader.DirectPackedReader4.
type legacyDirectPackedReader4 struct {
	in     store.RandomAccessInput
	offset int64
}

// Get reproduces
//
//	int shift = (int) ((index + 1) & 1) << 2;
//	return (in.readByte(offset + (index >>> 1)) >>> shift) & 0xF;
func (r *legacyDirectPackedReader4) Get(index int64) (int64, error) {
	shift := uint(((index + 1) & 1) << 2)
	b, err := r.in.ReadByteAt(r.offset + int64(uint64(index)>>1))
	if err != nil {
		return 0, err
	}
	return int64((uint64(b) >> shift) & 0xF), nil
}

// legacyDirectPackedReader8 mirrors LegacyDirectReader.DirectPackedReader8.
type legacyDirectPackedReader8 struct {
	in     store.RandomAccessInput
	offset int64
}

// Get reproduces {@code return in.readByte(offset + index) & 0xFF;}.
func (r *legacyDirectPackedReader8) Get(index int64) (int64, error) {
	b, err := r.in.ReadByteAt(r.offset + index)
	if err != nil {
		return 0, err
	}
	return int64(b) & 0xFF, nil
}

// legacyDirectPackedReader12 mirrors LegacyDirectReader.DirectPackedReader12.
type legacyDirectPackedReader12 struct {
	in     store.RandomAccessInput
	offset int64
}

// Get reproduces
//
//	long offset = (index * 12) >>> 3;
//	int shift = (int) ((index + 1) & 1) << 2;
//	return (in.readShort(this.offset + offset) >>> shift) & 0xFFF;
func (r *legacyDirectPackedReader12) Get(index int64) (int64, error) {
	off := int64(uint64(index*12) >> 3)
	shift := uint(((index + 1) & 1) << 2)
	s, err := r.in.ReadShortAt(r.offset + off)
	if err != nil {
		return 0, err
	}
	return int64((uint64(uint16(s)) >> shift) & 0xFFF), nil
}

// legacyDirectPackedReader16 mirrors LegacyDirectReader.DirectPackedReader16.
type legacyDirectPackedReader16 struct {
	in     store.RandomAccessInput
	offset int64
}

// Get reproduces {@code return in.readShort(offset + (index << 1)) & 0xFFFF;}.
func (r *legacyDirectPackedReader16) Get(index int64) (int64, error) {
	s, err := r.in.ReadShortAt(r.offset + (index << 1))
	if err != nil {
		return 0, err
	}
	return int64(uint64(uint16(s))), nil
}

// legacyDirectPackedReader20 mirrors LegacyDirectReader.DirectPackedReader20.
type legacyDirectPackedReader20 struct {
	in     store.RandomAccessInput
	offset int64
}

// Get reproduces
//
//	long offset = (index * 20) >>> 3;
//	// TODO: clean this up...
//	int v = in.readInt(this.offset + offset) >>> 8;
//	int shift = (int) ((index + 1) & 1) << 2;
//	return (v >>> shift) & 0xFFFFF;
func (r *legacyDirectPackedReader20) Get(index int64) (int64, error) {
	off := int64(uint64(index*20) >> 3)
	i, err := r.in.ReadIntAt(r.offset + off)
	if err != nil {
		return 0, err
	}
	v := uint32(i) >> 8
	shift := uint(((index + 1) & 1) << 2)
	return int64((v >> shift) & 0xFFFFF), nil
}

// legacyDirectPackedReader24 mirrors LegacyDirectReader.DirectPackedReader24.
type legacyDirectPackedReader24 struct {
	in     store.RandomAccessInput
	offset int64
}

// Get reproduces {@code return in.readInt(offset + index * 3) >>> 8;}.
func (r *legacyDirectPackedReader24) Get(index int64) (int64, error) {
	i, err := r.in.ReadIntAt(r.offset + index*3)
	if err != nil {
		return 0, err
	}
	return int64(uint32(i) >> 8), nil
}

// legacyDirectPackedReader28 mirrors LegacyDirectReader.DirectPackedReader28.
type legacyDirectPackedReader28 struct {
	in     store.RandomAccessInput
	offset int64
}

// Get reproduces
//
//	long offset = (index * 28) >>> 3;
//	int shift = (int) ((index + 1) & 1) << 2;
//	return (in.readInt(this.offset + offset) >>> shift) & 0xFFFFFFFL;
func (r *legacyDirectPackedReader28) Get(index int64) (int64, error) {
	off := int64(uint64(index*28) >> 3)
	shift := uint(((index + 1) & 1) << 2)
	i, err := r.in.ReadIntAt(r.offset + off)
	if err != nil {
		return 0, err
	}
	return int64(uint32(i)>>shift) & 0xFFFFFFF, nil
}

// legacyDirectPackedReader32 mirrors LegacyDirectReader.DirectPackedReader32.
type legacyDirectPackedReader32 struct {
	in     store.RandomAccessInput
	offset int64
}

// Get reproduces
// {@code return in.readInt(this.offset + (index << 2)) & 0xFFFFFFFFL;}.
func (r *legacyDirectPackedReader32) Get(index int64) (int64, error) {
	i, err := r.in.ReadIntAt(r.offset + (index << 2))
	if err != nil {
		return 0, err
	}
	return int64(uint32(i)), nil
}

// legacyDirectPackedReader40 mirrors LegacyDirectReader.DirectPackedReader40.
type legacyDirectPackedReader40 struct {
	in     store.RandomAccessInput
	offset int64
}

// Get reproduces {@code return in.readLong(this.offset + index * 5) >>> 24;}.
func (r *legacyDirectPackedReader40) Get(index int64) (int64, error) {
	l, err := r.in.ReadLongAt(r.offset + index*5)
	if err != nil {
		return 0, err
	}
	return int64(uint64(l) >> 24), nil
}

// legacyDirectPackedReader48 mirrors LegacyDirectReader.DirectPackedReader48.
type legacyDirectPackedReader48 struct {
	in     store.RandomAccessInput
	offset int64
}

// Get reproduces {@code return in.readLong(this.offset + index * 6) >>> 16;}.
func (r *legacyDirectPackedReader48) Get(index int64) (int64, error) {
	l, err := r.in.ReadLongAt(r.offset + index*6)
	if err != nil {
		return 0, err
	}
	return int64(uint64(l) >> 16), nil
}

// legacyDirectPackedReader56 mirrors LegacyDirectReader.DirectPackedReader56.
type legacyDirectPackedReader56 struct {
	in     store.RandomAccessInput
	offset int64
}

// Get reproduces {@code return in.readLong(this.offset + index * 7) >>> 8;}.
func (r *legacyDirectPackedReader56) Get(index int64) (int64, error) {
	l, err := r.in.ReadLongAt(r.offset + index*7)
	if err != nil {
		return 0, err
	}
	return int64(uint64(l) >> 8), nil
}

// legacyDirectPackedReader64 mirrors LegacyDirectReader.DirectPackedReader64.
type legacyDirectPackedReader64 struct {
	in     store.RandomAccessInput
	offset int64
}

// Get reproduces {@code return in.readLong(offset + (index << 3));}.
func (r *legacyDirectPackedReader64) Get(index int64) (int64, error) {
	return r.in.ReadLongAt(r.offset + (index << 3))
}
