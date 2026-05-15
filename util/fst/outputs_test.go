// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package fst

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// minimalOutputs is a smoke-test implementation of the Outputs[T]
// interface used to assert that the generic contract compiles and
// behaves as documented. It is not a production Outputs.
type minimalOutputs struct{ noOutput int64 }

func (m *minimalOutputs) Common(a, b int64) int64     { return min(a, b) }
func (m *minimalOutputs) Subtract(o, inc int64) int64 { return o - inc }
func (m *minimalOutputs) Add(p, o int64) int64        { return p + o }
func (m *minimalOutputs) Write(o int64, out store.DataOutput) error {
	return out.(store.VariableLengthOutput).WriteVLong(o)
}
func (m *minimalOutputs) WriteFinalOutput(o int64, out store.DataOutput) error {
	return m.Write(o, out)
}
func (m *minimalOutputs) Read(in store.DataInput) (int64, error) {
	return in.(store.VariableLengthInput).ReadVLong()
}
func (m *minimalOutputs) SkipOutput(in store.DataInput) error {
	_, err := m.Read(in)
	return err
}
func (m *minimalOutputs) ReadFinalOutput(in store.DataInput) (int64, error) { return m.Read(in) }
func (m *minimalOutputs) SkipFinalOutput(in store.DataInput) error          { return m.SkipOutput(in) }
func (m *minimalOutputs) GetNoOutput() int64                                { return m.noOutput }
func (m *minimalOutputs) OutputToString(o int64) string                     { return "" }
func (m *minimalOutputs) Merge(a, b int64) (int64, error)                   { return 0, ErrUnsupportedMerge }
func (m *minimalOutputs) RAMBytesUsed(int64) int64                          { return 8 }

func TestOutputsInterfaceContract(t *testing.T) {
	// Compile-time check: minimalOutputs satisfies Outputs[int64].
	var o Outputs[int64] = &minimalOutputs{noOutput: 0}

	if got := o.Common(3, 5); got != 3 {
		t.Fatalf("Common(3,5) = %d; want 3", got)
	}
	if got := o.Subtract(5, 2); got != 3 {
		t.Fatalf("Subtract(5,2) = %d; want 3", got)
	}
	if got := o.Add(2, 3); got != 5 {
		t.Fatalf("Add(2,3) = %d; want 5", got)
	}
	if got := o.GetNoOutput(); got != 0 {
		t.Fatalf("GetNoOutput() = %d; want 0", got)
	}

	if _, err := o.Merge(1, 2); !errors.Is(err, ErrUnsupportedMerge) {
		t.Fatalf("Merge: expected ErrUnsupportedMerge, got %v", err)
	}
}

func TestOutputsSerializationRoundTrip(t *testing.T) {
	o := &minimalOutputs{}
	out := store.NewByteArrayDataOutput(16)
	if err := o.Write(int64(0x12345678), out); err != nil {
		t.Fatalf("Write: %v", err)
	}
	in := store.NewByteArrayDataInput(out.GetBytes())
	got, err := o.Read(in)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != 0x12345678 {
		t.Fatalf("round-trip = %x; want %x", got, 0x12345678)
	}
}
