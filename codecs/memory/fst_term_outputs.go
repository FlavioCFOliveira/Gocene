// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package memory

import (
	"bytes"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	gfst "github.com/FlavioCFOliveira/Gocene/util/fst"
)

// TermData is the FST output type for FSTTermOutputs.  It stores optional
// per-term postings metadata bytes together with docFreq and totalTermFreq.
//
// Port of FSTTermOutputs.TermData (Lucene 10.4.0, inner static class).
type TermData struct {
	// Bytes holds optional codec-specific postings metadata.
	Bytes []byte
	// DocFreq is the document frequency (0 for no-output).
	DocFreq int
	// TotalTermFreq is the total term frequency (-1 for no-output / DOCS-only).
	TotalTermFreq int64
}

// fstTermNoOutput is the singleton "no output" sentinel.
var fstTermNoOutput = &TermData{Bytes: nil, DocFreq: 0, TotalTermFreq: -1}

// statsEqual returns true when two TermData values have equal stats.
func statsEqual(t1, t2 *TermData) bool {
	return t1.DocFreq == t2.DocFreq && t1.TotalTermFreq == t2.TotalTermFreq
}

// bytesEqual returns true when two TermData values have equal bytes.
func bytesEqual(t1, t2 *TermData) bool {
	if t1.Bytes == nil && t2.Bytes == nil {
		return true
	}
	return t1.Bytes != nil && t2.Bytes != nil && bytes.Equal(t1.Bytes, t2.Bytes)
}

// HashCode returns a hash of the TermData.
func (d *TermData) HashCode() int {
	h := 0
	for _, b := range d.Bytes {
		h += int(b)
	}
	h += d.DocFreq + int(d.TotalTermFreq)
	return h
}

// Equal returns true when d equals other.
func (d *TermData) Equal(other *TermData) bool {
	return other == d || (statsEqual(d, other) && bytesEqual(d, other))
}

// String implements fmt.Stringer.
func (d *TermData) String() string {
	return fmt.Sprintf("FSTTermOutputs$TermData bytes=%v docFreq=%d totalTermFreq=%d",
		d.Bytes, d.DocFreq, d.TotalTermFreq)
}

// FSTTermOutputs is the Outputs[*TermData] implementation used by
// FSTTermsWriter / FSTTermsReader.
//
// Port of org.apache.lucene.codecs.memory.FSTTermOutputs (Lucene 10.4.0).
type FSTTermOutputs struct {
	// hasPos is true when the field has positions (i.e. not DOCS-only).
	hasPos bool
}

// NewFSTTermOutputs constructs FSTTermOutputs for the given field.
func NewFSTTermOutputs(fieldInfo *index.FieldInfo) *FSTTermOutputs {
	return &FSTTermOutputs{
		hasPos: fieldInfo.IndexOptions() != index.IndexOptionsDocs,
	}
}

// GetNoOutput returns the singleton no-output sentinel.
func (*FSTTermOutputs) GetNoOutput() *TermData { return fstTermNoOutput }

// Common implements Outputs: returns the shared part of t1 and t2.
// Since TermData is not ordered, common is non-trivial only when the two
// values are identical; otherwise NO_OUTPUT is returned.
func (*FSTTermOutputs) Common(t1, t2 *TermData) *TermData {
	if t1 == fstTermNoOutput || t2 == fstTermNoOutput {
		return fstTermNoOutput
	}
	if statsEqual(t1, t2) && bytesEqual(t1, t2) {
		return t1
	}
	return fstTermNoOutput
}

// Subtract implements Outputs.
func (*FSTTermOutputs) Subtract(t1, t2 *TermData) *TermData {
	if t2 == fstTermNoOutput {
		return t1
	}
	if statsEqual(t1, t2) && bytesEqual(t1, t2) {
		return fstTermNoOutput
	}
	return &TermData{Bytes: t1.Bytes, DocFreq: t1.DocFreq, TotalTermFreq: t1.TotalTermFreq}
}

// Add implements Outputs.
func (*FSTTermOutputs) Add(t1, t2 *TermData) *TermData {
	if t1 == fstTermNoOutput {
		return t2
	}
	if t2 == fstTermNoOutput {
		return t1
	}
	if len(t2.Bytes) > 0 || t2.DocFreq > 0 {
		return &TermData{Bytes: t2.Bytes, DocFreq: t2.DocFreq, TotalTermFreq: t2.TotalTermFreq}
	}
	return &TermData{Bytes: t1.Bytes, DocFreq: t1.DocFreq, TotalTermFreq: t1.TotalTermFreq}
}

// Write implements Outputs: encodes data into out.
// Format: byte (flags + short-length), [VInt if long-length], [bytes], [VInt stats].
func (o *FSTTermOutputs) Write(data *TermData, out store.DataOutput) error {
	vlo, ok := out.(store.VariableLengthOutput)
	if !ok {
		return fmt.Errorf("FSTTermOutputs.Write: DataOutput does not implement VariableLengthOutput")
	}

	bit0 := 0
	if len(data.Bytes) > 0 {
		bit0 = 1
	}
	bit1 := 0
	if data.DocFreq > 0 {
		bit1 = 2
	}
	bits := bit0 | bit1

	if bit0 > 0 {
		n := len(data.Bytes)
		if n < 32 {
			bits |= n << 2
			if err := out.WriteByte(byte(bits)); err != nil {
				return err
			}
		} else {
			if err := out.WriteByte(byte(bits)); err != nil {
				return err
			}
			if err := vlo.WriteVInt(int32(n)); err != nil {
				return err
			}
		}
		if err := out.WriteBytes(data.Bytes); err != nil {
			return err
		}
	} else {
		if err := out.WriteByte(byte(bits)); err != nil {
			return err
		}
	}

	if bit1 > 0 {
		if o.hasPos {
			if data.DocFreq == int(data.TotalTermFreq) {
				if err := vlo.WriteVInt(int32(data.DocFreq<<1) | 1); err != nil {
					return err
				}
			} else {
				if err := vlo.WriteVInt(int32(data.DocFreq << 1)); err != nil {
					return err
				}
				if err := vlo.WriteVLong(data.TotalTermFreq - int64(data.DocFreq)); err != nil {
					return err
				}
			}
		} else {
			if err := vlo.WriteVInt(int32(data.DocFreq)); err != nil {
				return err
			}
		}
	}
	return nil
}

// WriteFinalOutput delegates to Write.
func (o *FSTTermOutputs) WriteFinalOutput(data *TermData, out store.DataOutput) error {
	return o.Write(data, out)
}

// Read implements Outputs.
func (o *FSTTermOutputs) Read(in store.DataInput) (*TermData, error) {
	vli, ok := in.(store.VariableLengthInput)
	if !ok {
		return nil, fmt.Errorf("FSTTermOutputs.Read: DataInput does not implement VariableLengthInput")
	}

	b, err := in.ReadByte()
	if err != nil {
		return nil, err
	}
	bits := int(b) & 0xff
	bit0 := bits & 1
	bit1 := bits & 2
	bytesSize := bits >> 2

	if bit0 > 0 && bytesSize == 0 {
		n, err := vli.ReadVInt()
		if err != nil {
			return nil, err
		}
		bytesSize = int(n)
	}

	var termBytes []byte
	if bit0 > 0 {
		termBytes = make([]byte, bytesSize)
		if err := in.ReadBytes(termBytes); err != nil {
			return nil, err
		}
	}

	docFreq := 0
	totalTermFreq := int64(-1)
	if bit1 > 0 {
		code, err := vli.ReadVInt()
		if err != nil {
			return nil, err
		}
		if o.hasPos {
			totalTermFreq = int64(code >> 1)
			docFreq = int(totalTermFreq)
			if (code & 1) == 0 {
				delta, err := vli.ReadVLong()
				if err != nil {
					return nil, err
				}
				totalTermFreq += delta
			}
		} else {
			docFreq = int(code)
		}
	}
	return &TermData{Bytes: termBytes, DocFreq: docFreq, TotalTermFreq: totalTermFreq}, nil
}

// ReadFinalOutput delegates to Read.
func (o *FSTTermOutputs) ReadFinalOutput(in store.DataInput) (*TermData, error) {
	return o.Read(in)
}

// SkipOutput implements Outputs: discards the encoded value.
func (o *FSTTermOutputs) SkipOutput(in store.DataInput) error {
	vli, ok := in.(store.VariableLengthInput)
	if !ok {
		return fmt.Errorf("FSTTermOutputs.SkipOutput: DataInput does not implement VariableLengthInput")
	}
	b, err := in.ReadByte()
	if err != nil {
		return err
	}
	bits := int(b) & 0xff
	bit0 := bits & 1
	bit1 := bits & 2
	bytesSize := bits >> 2

	if bit0 > 0 && bytesSize == 0 {
		n, err := vli.ReadVInt()
		if err != nil {
			return err
		}
		bytesSize = int(n)
	}
	if bit0 > 0 {
		scratch := make([]byte, bytesSize)
		if err := in.ReadBytes(scratch); err != nil {
			return err
		}
	}
	if bit1 > 0 {
		code, err := vli.ReadVInt()
		if err != nil {
			return err
		}
		if o.hasPos && (code&1) == 0 {
			if _, err := vli.ReadVLong(); err != nil {
				return err
			}
		}
	}
	return nil
}

// SkipFinalOutput delegates to SkipOutput.
func (o *FSTTermOutputs) SkipFinalOutput(in store.DataInput) error { return o.SkipOutput(in) }

// OutputToString implements Outputs.
func (*FSTTermOutputs) OutputToString(data *TermData) string { return data.String() }

// Merge implements Outputs.  FSTTermOutputs does not support merge.
func (*FSTTermOutputs) Merge(_, _ *TermData) (*TermData, error) {
	return nil, gfst.ErrUnsupportedMerge
}

// RAMBytesUsed implements Outputs.
func (*FSTTermOutputs) RAMBytesUsed(data *TermData) int64 {
	// shallow struct + slice header + bytes content
	const base = 32
	return int64(base + len(data.Bytes))
}

// compile-time assertion.
var _ gfst.Outputs[*TermData] = (*FSTTermOutputs)(nil)
