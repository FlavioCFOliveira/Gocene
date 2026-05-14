// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// PackedDataOutput is a DataOutput wrapper that encodes unaligned,
// variable-length packed integers consumed by PackedDataInput.
type PackedDataOutput struct {
	out           store.DataOutput
	current       uint64
	remainingBits int
}

// NewPackedDataOutput wraps the given DataOutput.
func NewPackedDataOutput(out store.DataOutput) *PackedDataOutput {
	return &PackedDataOutput{
		out:           out,
		remainingBits: 8,
	}
}

// WriteLong writes value using exactly bitsPerValue bits. The value
// must be non-negative and fit in bitsPerValue bits (or
// bitsPerValue must equal 64 for unrestricted 64-bit values).
func (p *PackedDataOutput) WriteLong(value int64, bitsPerValue int) error {
	if bitsPerValue < 1 || bitsPerValue > 64 {
		return fmt.Errorf("packed: bitsPerValue out of range: %d", bitsPerValue)
	}
	if bitsPerValue != 64 {
		if value < 0 || uint64(value) > uint64(MaxValue(bitsPerValue)) {
			return fmt.Errorf("packed: value %d does not fit in %d bits", value, bitsPerValue)
		}
	}
	v := uint64(value)
	for bitsPerValue > 0 {
		if p.remainingBits == 0 {
			if err := p.out.WriteByte(byte(p.current)); err != nil {
				return err
			}
			p.current = 0
			p.remainingBits = 8
		}
		bits := p.remainingBits
		if bitsPerValue < bits {
			bits = bitsPerValue
		}
		mask := (uint64(1) << uint(bits)) - 1
		p.current |= ((v >> uint(bitsPerValue-bits)) & mask) << uint(p.remainingBits-bits)
		bitsPerValue -= bits
		p.remainingBits -= bits
	}
	return nil
}

// Flush emits any pending bits to the underlying DataOutput, padding
// the trailing byte with zero bits when necessary.
func (p *PackedDataOutput) Flush() error {
	if p.remainingBits < 8 {
		if err := p.out.WriteByte(byte(p.current)); err != nil {
			return err
		}
	}
	p.remainingBits = 8
	p.current = 0
	return nil
}
