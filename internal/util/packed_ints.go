// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"fmt"
)

// PackedIntsDecoder decodes packed integers from a byte slice.
type PackedIntsDecoder struct {
	data       []byte
	bitsPerVal int
	pos        int // bit position
}

// NewPackedIntsDecoder creates a new PackedIntsDecoder.
func NewPackedIntsDecoder(data []byte, bitsPerVal int) *PackedIntsDecoder {
	return &PackedIntsDecoder{
		data:       data,
		bitsPerVal: bitsPerVal,
	}
}

// Next decodes the next integer.
func (d *PackedIntsDecoder) Next() (int64, error) {
	if d.pos+d.bitsPerVal > len(d.data)*8 {
		return 0, fmt.Errorf("EOF")
	}

	var val uint64
	bitsRead := 0

	for bitsRead < d.bitsPerVal {
		bytePos := d.pos / 8
		bitOffset := d.pos % 8

		if bytePos >= len(d.data) {
			return 0, fmt.Errorf("EOF")
		}

		b := uint64(d.data[bytePos])
		// Mask out the bits we've already read from this byte
		b &= (0xFF >> bitOffset)

		// How many bits can we take from this byte?
		available := 8 - bitOffset
		take := d.bitsPerVal - bitsRead
		if take > available {
			take = available
		}

		// Shift the bits to the correct position in val
		// Lucene packs from MSB to LSB of the byte
		shift := uint(available - take)
		mask := uint64((1 << take) - 1)
		bits := (b >> shift) & mask

		val = (val << take) | bits

		bitsRead += take
		d.pos += take
	}

	return int64(val), nil
}
