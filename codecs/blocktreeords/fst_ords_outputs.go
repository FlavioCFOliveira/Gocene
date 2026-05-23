// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktreeords

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	gfst "github.com/FlavioCFOliveira/Gocene/util/fst"
)

// FSTOrdsOutput is the output type for FSTOrdsOutputs: a BytesRef prefix
// plus an ordinal range [StartOrd, EndOrd] (both inclusive).
//
// Port of org.apache.lucene.codecs.blocktreeords.FSTOrdsOutputs.Output
// (Java record, Lucene 10.4.0).
type FSTOrdsOutput struct {
	Bytes    *util.BytesRef
	StartOrd int64
	EndOrd   int64
}

// String returns a human-readable representation of the ordinal range.
func (o FSTOrdsOutput) String() string {
	var x int64
	if o.EndOrd > math.MaxInt64/2 {
		x = math.MaxInt64 - o.EndOrd
	} else {
		x = -o.EndOrd
	}
	return fmt.Sprintf("%d to %d", o.StartOrd, x)
}

// fstOrdsNoBytes is the shared empty BytesRef sentinel.
var fstOrdsNoBytes = util.NewBytesRefEmpty()

// fstOrdsNoOutput is the singleton "no output" value.  Pointer identity
// is used to detect it — callers must never construct a new zero-value
// FSTOrdsOutput and compare it against this; they must compare with ==.
var fstOrdsNoOutput = &FSTOrdsOutput{Bytes: fstOrdsNoBytes, StartOrd: 0, EndOrd: 0}

// FSTOrdsOutputs is the Outputs[*FSTOrdsOutput] implementation used by
// the BlockTreeOrds postings format.  Each value stores a BytesRef prefix
// together with a [startOrd, endOrd] ordinal range.
//
// Port of org.apache.lucene.codecs.blocktreeords.FSTOrdsOutputs
// (Lucene 10.4.0).
type FSTOrdsOutputs struct{}

// fstOrdsOutputsSingleton is the package-level singleton.
var fstOrdsOutputsSingleton = &FSTOrdsOutputs{}

// FSTOutputs returns the FSTOrdsOutputs singleton.
func FSTOutputs() gfst.Outputs[*FSTOrdsOutput] { return fstOrdsOutputsSingleton }

// newOutput returns a pooled no-output for the zero case or a fresh struct.
func (*FSTOrdsOutputs) newOutput(b *util.BytesRef, startOrd, endOrd int64) *FSTOrdsOutput {
	if b.Length == 0 && startOrd == 0 && endOrd == 0 {
		return fstOrdsNoOutput
	}
	return &FSTOrdsOutput{Bytes: b, StartOrd: startOrd, EndOrd: endOrd}
}

// Common implements Outputs: returns the longest common bytes prefix and
// the minimum ordinals of the two outputs.
func (o *FSTOrdsOutputs) Common(a, b *FSTOrdsOutput) *FSTOrdsOutput {
	b1, b2 := a.Bytes, b.Bytes

	pos1 := b1.Offset
	pos2 := b2.Offset
	stop := pos1 + min(b1.Length, b2.Length)
	for pos1 < stop {
		if b1.Bytes[pos1] != b2.Bytes[pos2] {
			break
		}
		pos1++
		pos2++
	}

	var prefix *util.BytesRef
	switch {
	case pos1 == b1.Offset:
		prefix = fstOrdsNoBytes
	case pos1 == b1.Offset+b1.Length:
		prefix = b1
	case pos2 == b2.Offset+b2.Length:
		prefix = b2
	default:
		prefix = &util.BytesRef{
			Bytes:  b1.Bytes,
			Offset: b1.Offset,
			Length: pos1 - b1.Offset,
		}
	}

	startOrd := a.StartOrd
	if b.StartOrd < startOrd {
		startOrd = b.StartOrd
	}
	endOrd := a.EndOrd
	if b.EndOrd < endOrd {
		endOrd = b.EndOrd
	}
	return o.newOutput(prefix, startOrd, endOrd)
}

// Subtract implements Outputs: removes the prefix inc from output.
func (o *FSTOrdsOutputs) Subtract(output, inc *FSTOrdsOutput) *FSTOrdsOutput {
	if inc == fstOrdsNoOutput {
		return output
	}
	incB := inc.Bytes
	outB := output.Bytes

	var suffix *util.BytesRef
	switch {
	case incB.Length == outB.Length:
		suffix = fstOrdsNoBytes
	case incB.Length == 0:
		suffix = outB
	default:
		suffix = &util.BytesRef{
			Bytes:  outB.Bytes,
			Offset: outB.Offset + incB.Length,
			Length: outB.Length - incB.Length,
		}
	}
	return o.newOutput(suffix, output.StartOrd-inc.StartOrd, output.EndOrd-inc.EndOrd)
}

// Add implements Outputs: concatenates prefix and output.
func (o *FSTOrdsOutputs) Add(prefix, output *FSTOrdsOutput) *FSTOrdsOutput {
	if prefix == fstOrdsNoOutput {
		return output
	}
	if output == fstOrdsNoOutput {
		return prefix
	}
	pb, ob := prefix.Bytes, output.Bytes
	buf := make([]byte, pb.Length+ob.Length)
	copy(buf, pb.Bytes[pb.Offset:pb.Offset+pb.Length])
	copy(buf[pb.Length:], ob.Bytes[ob.Offset:ob.Offset+ob.Length])
	b := &util.BytesRef{Bytes: buf, Offset: 0, Length: len(buf)}
	return o.newOutput(b, prefix.StartOrd+output.StartOrd, prefix.EndOrd+output.EndOrd)
}

// Write implements Outputs: encodes as VInt(len)+bytes+VLong(startOrd)+VLong(endOrd).
func (*FSTOrdsOutputs) Write(v *FSTOrdsOutput, out store.DataOutput) error {
	vlo, ok := out.(store.VariableLengthOutput)
	if !ok {
		return fmt.Errorf("FSTOrdsOutputs.Write: DataOutput does not implement VariableLengthOutput")
	}
	if err := vlo.WriteVInt(int32(v.Bytes.Length)); err != nil {
		return err
	}
	if v.Bytes.Length > 0 {
		if err := out.WriteBytes(v.Bytes.Bytes[v.Bytes.Offset : v.Bytes.Offset+v.Bytes.Length]); err != nil {
			return err
		}
	}
	if err := vlo.WriteVLong(v.StartOrd); err != nil {
		return err
	}
	return vlo.WriteVLong(v.EndOrd)
}

// WriteFinalOutput delegates to Write (same encoding for final outputs).
func (o *FSTOrdsOutputs) WriteFinalOutput(v *FSTOrdsOutput, out store.DataOutput) error {
	return o.Write(v, out)
}

// Read implements Outputs.
func (o *FSTOrdsOutputs) Read(in store.DataInput) (*FSTOrdsOutput, error) {
	vli, ok := in.(store.VariableLengthInput)
	if !ok {
		return nil, fmt.Errorf("FSTOrdsOutputs.Read: DataInput does not implement VariableLengthInput")
	}
	lenI, err := vli.ReadVInt()
	if err != nil {
		return nil, err
	}
	n := int(lenI)
	var b *util.BytesRef
	if n == 0 {
		b = fstOrdsNoBytes
	} else {
		buf := make([]byte, n)
		if err := in.ReadBytes(buf); err != nil {
			return nil, err
		}
		b = &util.BytesRef{Bytes: buf, Offset: 0, Length: n}
	}
	startOrd, err := vli.ReadVLong()
	if err != nil {
		return nil, err
	}
	endOrd, err := vli.ReadVLong()
	if err != nil {
		return nil, err
	}
	return o.newOutput(b, startOrd, endOrd), nil
}

// ReadFinalOutput delegates to Read.
func (o *FSTOrdsOutputs) ReadFinalOutput(in store.DataInput) (*FSTOrdsOutput, error) {
	return o.Read(in)
}

// SkipOutput implements Outputs: discards VInt(len)+bytes+VLong+VLong.
func (*FSTOrdsOutputs) SkipOutput(in store.DataInput) error {
	vli, ok := in.(store.VariableLengthInput)
	if !ok {
		return fmt.Errorf("FSTOrdsOutputs.SkipOutput: DataInput does not implement VariableLengthInput")
	}
	n, err := vli.ReadVInt()
	if err != nil {
		return err
	}
	if n > 0 {
		scratch := make([]byte, n)
		if err := in.ReadBytes(scratch); err != nil {
			return err
		}
	}
	if _, err := vli.ReadVLong(); err != nil {
		return err
	}
	_, err = vli.ReadVLong()
	return err
}

// SkipFinalOutput delegates to SkipOutput.
func (o *FSTOrdsOutputs) SkipFinalOutput(in store.DataInput) error { return o.SkipOutput(in) }

// GetNoOutput returns the singleton no-output sentinel.
func (*FSTOrdsOutputs) GetNoOutput() *FSTOrdsOutput { return fstOrdsNoOutput }

// OutputToString implements Outputs.
func (*FSTOrdsOutputs) OutputToString(v *FSTOrdsOutput) string {
	if (v.EndOrd == 0 || v.EndOrd == math.MaxInt64) && v.StartOrd == 0 {
		return ""
	}
	return v.String()
}

// Merge implements Outputs. FSTOrdsOutputs does not support merge.
func (*FSTOrdsOutputs) Merge(_, _ *FSTOrdsOutput) (*FSTOrdsOutput, error) {
	return nil, gfst.ErrUnsupportedMerge
}

// RAMBytesUsed implements Outputs.
func (*FSTOrdsOutputs) RAMBytesUsed(v *FSTOrdsOutput) int64 {
	// 2× object header + 2× int64 + 2× pointer + BytesRef header + 2×int + bytes
	const base = 2*16 + 2*8 + 2*8 + 24 + 2*4
	return int64(base + v.Bytes.Length)
}

// compile-time assertion that FSTOrdsOutputs satisfies Outputs[*FSTOrdsOutput].
var _ gfst.Outputs[*FSTOrdsOutput] = (*FSTOrdsOutputs)(nil)
