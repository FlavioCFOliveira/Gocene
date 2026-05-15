// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package fst

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestNoOutputsSingletonIdentity(t *testing.T) {
	a := NoOutputs()
	b := NoOutputs()
	if a != b {
		t.Fatalf("NoOutputs() did not return the same singleton")
	}
	if NoOutputValue() != NoOutputValue() {
		t.Fatalf("NoOutputValue() did not return the same singleton")
	}
}

func TestNoOutputsAlgebraReturnsNoOutput(t *testing.T) {
	o := NoOutputs()
	no := o.GetNoOutput()

	if got := o.Common(no, no); got != no {
		t.Fatalf("Common should return NO_OUTPUT")
	}
	if got := o.Subtract(no, no); got != no {
		t.Fatalf("Subtract should return NO_OUTPUT")
	}
	if got := o.Add(no, no); got != no {
		t.Fatalf("Add should return NO_OUTPUT")
	}
	got, err := o.Merge(no, no)
	if err != nil {
		t.Fatalf("Merge unexpected err: %v", err)
	}
	if got != no {
		t.Fatalf("Merge should return NO_OUTPUT")
	}
}

func TestNoOutputsSerializationIsNoop(t *testing.T) {
	o := NoOutputs()
	out := store.NewByteArrayDataOutput(8)
	if err := o.Write(o.GetNoOutput(), out); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if len(out.GetBytes()) != 0 {
		t.Fatalf("expected zero bytes written, got %d", len(out.GetBytes()))
	}

	in := store.NewByteArrayDataInput([]byte{})
	got, err := o.Read(in)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != o.GetNoOutput() {
		t.Fatalf("Read should return NO_OUTPUT singleton")
	}
	if err := o.SkipOutput(in); err != nil {
		t.Fatalf("SkipOutput: %v", err)
	}
}

func TestNoOutputsToString(t *testing.T) {
	impl := NoOutputsImpl{}
	if got := impl.String(); got != "NoOutputs" {
		t.Fatalf("String() = %q; want \"NoOutputs\"", got)
	}
	if got := impl.OutputToString(NoOutputValue()); got != "" {
		t.Fatalf("OutputToString() = %q; want empty", got)
	}
	if got := impl.RAMBytesUsed(NoOutputValue()); got != 0 {
		t.Fatalf("RAMBytesUsed() = %d; want 0", got)
	}
}
