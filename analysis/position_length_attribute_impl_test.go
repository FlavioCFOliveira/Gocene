// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"testing"
)

func TestPositionLengthAttributeImpl_DefaultState(t *testing.T) {
	p := NewPositionLengthAttributeImpl()
	if p.GetPositionLength() != 1 {
		t.Fatalf("default position length must be 1, got %d", p.GetPositionLength())
	}
}

func TestPositionLengthAttributeImpl_SetGet(t *testing.T) {
	p := NewPositionLengthAttributeImpl()
	p.SetPositionLength(5)
	if p.GetPositionLength() != 5 {
		t.Fatalf("want 5, got %d", p.GetPositionLength())
	}
}

func TestPositionLengthAttributeImpl_SetPositionLengthValidated_Valid(t *testing.T) {
	p := NewPositionLengthAttributeImpl()
	p.SetPositionLengthValidated(3)
	if p.GetPositionLength() != 3 {
		t.Fatalf("want 3, got %d", p.GetPositionLength())
	}
}

func TestPositionLengthAttributeImpl_SetPositionLengthValidated_Invalid(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("SetPositionLengthValidated(0) must panic")
		}
	}()
	NewPositionLengthAttributeImpl().SetPositionLengthValidated(0)
}

func TestPositionLengthAttributeImpl_Clear(t *testing.T) {
	p := NewPositionLengthAttributeImpl()
	p.SetPositionLength(7)
	p.Clear()
	if p.GetPositionLength() != 1 {
		t.Fatalf("after Clear want 1, got %d", p.GetPositionLength())
	}
}

func TestPositionLengthAttributeImpl_End(t *testing.T) {
	p := NewPositionLengthAttributeImpl()
	p.SetPositionLength(4)
	p.End()
	if p.GetPositionLength() != 1 {
		t.Fatalf("after End want 1, got %d", p.GetPositionLength())
	}
}

func TestPositionLengthAttributeImpl_CopyTo(t *testing.T) {
	src := NewPositionLengthAttributeImpl()
	src.SetPositionLength(3)
	dst := NewPositionLengthAttributeImpl()
	src.CopyTo(dst)
	if !src.Equals(dst) {
		t.Fatal("CopyTo: destination does not equal source")
	}
	src.Clear()
	if dst.GetPositionLength() != 3 {
		t.Fatal("CopyTo: clearing source must not affect destination")
	}
}

func TestPositionLengthAttributeImpl_CloneAttribute(t *testing.T) {
	src := NewPositionLengthAttributeImpl()
	src.SetPositionLength(9)
	clone, ok := src.CloneAttribute().(*PositionLengthAttributeImpl)
	if !ok {
		t.Fatal("CloneAttribute must return *PositionLengthAttributeImpl")
	}
	if !src.Equals(clone) {
		t.Fatal("clone must equal original")
	}
}

func TestPositionLengthAttributeImpl_Equals(t *testing.T) {
	a := NewPositionLengthAttributeImpl()
	a.SetPositionLength(2)
	b := NewPositionLengthAttributeImpl()
	b.SetPositionLength(2)
	c := NewPositionLengthAttributeImpl()
	c.SetPositionLength(3)
	if !a.Equals(b) {
		t.Fatal("equal values must be equal")
	}
	if a.Equals(c) {
		t.Fatal("different values must not be equal")
	}
}

func TestPositionLengthAttributeImpl_HashCode(t *testing.T) {
	a := NewPositionLengthAttributeImpl()
	a.SetPositionLength(5)
	b := NewPositionLengthAttributeImpl()
	b.SetPositionLength(5)
	if a.HashCode() != b.HashCode() {
		t.Fatal("equal attributes must have equal hash codes")
	}
	if a.HashCode() != 5 {
		t.Fatalf("hash must equal positionLength; want 5, got %d", a.HashCode())
	}
}

func TestPositionLengthAttributeImpl_AttributeInterfaces(t *testing.T) {
	p := NewPositionLengthAttributeImpl()
	ifaces := p.AttributeInterfaces()
	if len(ifaces) != 1 || ifaces[0] != PositionLengthAttributeType {
		t.Fatalf("unexpected AttributeInterfaces: %v", ifaces)
	}
}
