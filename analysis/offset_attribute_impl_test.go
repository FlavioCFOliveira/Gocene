// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"testing"
)

func TestOffsetAttributeImpl_DefaultState(t *testing.T) {
	o := NewOffsetAttributeImpl()
	if o.StartOffset() != 0 || o.EndOffset() != 0 {
		t.Fatalf("default offsets must be 0, got start=%d end=%d",
			o.StartOffset(), o.EndOffset())
	}
}

func TestOffsetAttributeImpl_SetOffset(t *testing.T) {
	o := NewOffsetAttributeImpl()
	o.SetOffset(3, 7)
	if o.StartOffset() != 3 || o.EndOffset() != 7 {
		t.Fatalf("want start=3 end=7, got start=%d end=%d",
			o.StartOffset(), o.EndOffset())
	}
}

func TestOffsetAttributeImpl_SetOffset_Equal(t *testing.T) {
	// startOffset == endOffset is valid (zero-length token).
	o := NewOffsetAttributeImpl()
	o.SetOffset(5, 5)
	if o.StartOffset() != 5 || o.EndOffset() != 5 {
		t.Fatalf("want start=5 end=5, got start=%d end=%d",
			o.StartOffset(), o.EndOffset())
	}
}

func TestOffsetAttributeImpl_SetOffset_NegativeStart_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("SetOffset(-1, 0) must panic")
		}
	}()
	NewOffsetAttributeImpl().SetOffset(-1, 0)
}

func TestOffsetAttributeImpl_SetOffset_EndBeforeStart_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("SetOffset(5, 3) must panic")
		}
	}()
	NewOffsetAttributeImpl().SetOffset(5, 3)
}

func TestOffsetAttributeImpl_SetStartEndOffset(t *testing.T) {
	o := NewOffsetAttributeImpl()
	o.SetStartOffset(2)
	o.SetEndOffset(8)
	if o.StartOffset() != 2 || o.EndOffset() != 8 {
		t.Fatalf("want start=2 end=8, got start=%d end=%d",
			o.StartOffset(), o.EndOffset())
	}
}

func TestOffsetAttributeImpl_Clear(t *testing.T) {
	o := NewOffsetAttributeImpl()
	o.SetOffset(10, 20)
	o.Clear()
	if o.StartOffset() != 0 || o.EndOffset() != 0 {
		t.Fatalf("after Clear want 0/0, got %d/%d",
			o.StartOffset(), o.EndOffset())
	}
}

func TestOffsetAttributeImpl_End(t *testing.T) {
	o := NewOffsetAttributeImpl()
	o.SetOffset(5, 10)
	o.End()
	if o.StartOffset() != 0 || o.EndOffset() != 0 {
		t.Fatalf("after End want 0/0, got %d/%d",
			o.StartOffset(), o.EndOffset())
	}
}

func TestOffsetAttributeImpl_CopyTo(t *testing.T) {
	src := NewOffsetAttributeImpl()
	src.SetOffset(1, 9)
	dst := NewOffsetAttributeImpl()
	src.CopyTo(dst)
	if !src.Equals(dst) {
		t.Fatal("CopyTo: destination does not equal source")
	}
}

func TestOffsetAttributeImpl_Copy(t *testing.T) {
	src := NewOffsetAttributeImpl()
	src.SetOffset(3, 7)
	clone, ok := src.Copy().(*OffsetAttributeImpl)
	if !ok {
		t.Fatal("Copy must return *OffsetAttributeImpl")
	}
	if !src.Equals(clone) {
		t.Fatal("copy must equal original")
	}
}

func TestOffsetAttributeImpl_CloneAttribute(t *testing.T) {
	src := NewOffsetAttributeImpl()
	src.SetOffset(4, 8)
	clone, ok := src.CloneAttribute().(*OffsetAttributeImpl)
	if !ok {
		t.Fatal("CloneAttribute must return *OffsetAttributeImpl")
	}
	if !src.Equals(clone) {
		t.Fatal("clone must equal original")
	}
}

func TestOffsetAttributeImpl_Equals(t *testing.T) {
	a := NewOffsetAttributeImpl()
	a.SetOffset(1, 5)
	b := NewOffsetAttributeImpl()
	b.SetOffset(1, 5)
	c := NewOffsetAttributeImpl()
	c.SetOffset(2, 5)
	if !a.Equals(b) {
		t.Fatal("equal offsets must be equal")
	}
	if a.Equals(c) {
		t.Fatal("different offsets must not be equal")
	}
}

func TestOffsetAttributeImpl_HashCode(t *testing.T) {
	a := NewOffsetAttributeImpl()
	a.SetOffset(3, 7)
	// hash = startOffset*31 + endOffset = 3*31+7 = 100
	if a.HashCode() != 100 {
		t.Fatalf("want 100, got %d", a.HashCode())
	}
	b := NewOffsetAttributeImpl()
	b.SetOffset(3, 7)
	if a.HashCode() != b.HashCode() {
		t.Fatal("equal attributes must have equal hash codes")
	}
}

func TestOffsetAttributeImpl_AttributeInterfaces(t *testing.T) {
	o := NewOffsetAttributeImpl()
	ifaces := o.AttributeInterfaces()
	if len(ifaces) != 1 || ifaces[0] != OffsetAttributeType {
		t.Fatalf("unexpected AttributeInterfaces: %v", ifaces)
	}
}

func TestOffsetAttributeImpl_CopyTo_WrongTarget(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("CopyTo with wrong target type must panic")
		}
	}()
	src := NewOffsetAttributeImpl()
	src.CopyTo(NewSentenceAttributeImpl())
}
