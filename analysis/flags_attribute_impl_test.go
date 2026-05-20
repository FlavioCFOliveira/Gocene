// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"testing"
)

func TestFlagsAttributeImpl_DefaultState(t *testing.T) {
	f := NewFlagsAttributeImpl()
	if f.GetFlags() != 0 {
		t.Fatalf("default flags must be 0, got %d", f.GetFlags())
	}
}

func TestFlagsAttributeImpl_SetGet(t *testing.T) {
	f := NewFlagsAttributeImpl()
	f.SetFlags(0b1010)
	if f.GetFlags() != 0b1010 {
		t.Fatalf("want %b, got %b", 0b1010, f.GetFlags())
	}
}

func TestFlagsAttributeImpl_IsFlagSet(t *testing.T) {
	f := NewFlagsAttributeImpl()
	f.SetFlags(0b0110)
	if !f.IsFlagSet(0b0100) {
		t.Fatal("bit 2 must be set")
	}
	if f.IsFlagSet(0b0001) {
		t.Fatal("bit 0 must not be set")
	}
}

func TestFlagsAttributeImpl_SetFlag(t *testing.T) {
	f := NewFlagsAttributeImpl()
	f.SetFlag(0b0001, true)
	if !f.IsFlagSet(0b0001) {
		t.Fatal("SetFlag(true) must set the bit")
	}
	f.SetFlag(0b0001, false)
	if f.IsFlagSet(0b0001) {
		t.Fatal("SetFlag(false) must clear the bit")
	}
}

func TestFlagsAttributeImpl_Clear(t *testing.T) {
	f := NewFlagsAttributeImpl()
	f.SetFlags(0xFF)
	f.Clear()
	if f.GetFlags() != 0 {
		t.Fatalf("after Clear want 0, got %d", f.GetFlags())
	}
}

func TestFlagsAttributeImpl_End(t *testing.T) {
	f := NewFlagsAttributeImpl()
	f.SetFlags(5)
	f.End()
	if f.GetFlags() != 0 {
		t.Fatalf("after End want 0, got %d", f.GetFlags())
	}
}

func TestFlagsAttributeImpl_CopyTo(t *testing.T) {
	src := NewFlagsAttributeImpl()
	src.SetFlags(42)
	dst := NewFlagsAttributeImpl()
	src.CopyTo(dst)
	if !src.Equals(dst) {
		t.Fatal("CopyTo: destination does not equal source")
	}
}

func TestFlagsAttributeImpl_CloneAttribute(t *testing.T) {
	src := NewFlagsAttributeImpl()
	src.SetFlags(7)
	clone, ok := src.CloneAttribute().(*FlagsAttributeImpl)
	if !ok {
		t.Fatal("CloneAttribute must return *FlagsAttributeImpl")
	}
	if !src.Equals(clone) {
		t.Fatal("clone must equal original")
	}
}

func TestFlagsAttributeImpl_Equals(t *testing.T) {
	a := NewFlagsAttributeImpl()
	a.SetFlags(3)
	b := NewFlagsAttributeImpl()
	b.SetFlags(3)
	c := NewFlagsAttributeImpl()
	c.SetFlags(4)
	if !a.Equals(b) {
		t.Fatal("equal flags must be equal")
	}
	if a.Equals(c) {
		t.Fatal("different flags must not be equal")
	}
}

func TestFlagsAttributeImpl_HashCode(t *testing.T) {
	f := NewFlagsAttributeImpl()
	f.SetFlags(17)
	if f.HashCode() != 17 {
		t.Fatalf("hash must equal flags value; want 17, got %d", f.HashCode())
	}
}

func TestFlagsAttributeImpl_AttributeInterfaces(t *testing.T) {
	f := NewFlagsAttributeImpl()
	ifaces := f.AttributeInterfaces()
	if len(ifaces) != 1 || ifaces[0] != FlagsAttributeType {
		t.Fatalf("unexpected AttributeInterfaces: %v", ifaces)
	}
}

func TestFlagsAttributeImpl_CopyTo_WrongTarget(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("CopyTo with wrong target type must panic")
		}
	}()
	src := NewFlagsAttributeImpl()
	src.CopyTo(NewSentenceAttributeImpl())
}
