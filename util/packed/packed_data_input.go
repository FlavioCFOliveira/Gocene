// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// PackedDataInput is a DataInput wrapper that decodes unaligned,
// variable-length packed integers written by PackedDataOutput.
//
// This API is much slower than the fixed-length PackedInts API but
// is useful when raw space matters and decoding speed does not.
type PackedDataInput struct {
	in            store.DataInput
	current       uint64
	remainingBits int
}

// NewPackedDataInput wraps the given DataInput. The wrapper is
// positioned at the start of the next byte boundary.
func NewPackedDataInput(in store.DataInput) *PackedDataInput {
	p := &PackedDataInput{in: in}
	p.SkipToNextByte()
	return p
}

// ReadLong reads the next unsigned value using exactly bitsPerValue
// bits. bitsPerValue must be in [1, 64].
func (p *PackedDataInput) ReadLong(bitsPerValue int) (int64, error) {
	if bitsPerValue < 1 || bitsPerValue > 64 {
		return 0, fmt.Errorf("packed: bitsPerValue out of range: %d", bitsPerValue)
	}
	var r uint64
	for bitsPerValue > 0 {
		if p.remainingBits == 0 {
			b, err := p.in.ReadByte()
			if err != nil {
				return 0, err
			}
			p.current = uint64(b) & 0xFF
			p.remainingBits = 8
		}
		bits := bitsPerValue
		if p.remainingBits < bits {
			bits = p.remainingBits
		}
		mask := (uint64(1) << uint(bits)) - 1
		shift := uint(p.remainingBits - bits)
		r = (r << uint(bits)) | ((p.current >> shift) & mask)
		bitsPerValue -= bits
		p.remainingBits -= bits
	}
	return int64(r), nil
}

// SkipToNextByte discards any pending bits (at most 7) so that the
// next ReadLong call starts at the next byte boundary.
func (p *PackedDataInput) SkipToNextByte() {
	p.remainingBits = 0
}
