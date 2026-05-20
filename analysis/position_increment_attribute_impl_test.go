// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"testing"
)

func TestPositionIncrementAttributeImpl_DefaultState(t *testing.T) {
	p := NewPositionIncrementAttributeImpl()
	if p.GetPositionIncrement() != 1 {
		t.Fatalf("default position increment must be 1, got %d", p.GetPositionIncrement())
	}
}

func TestPositionIncrementAttributeImpl_SetGet(t *testing.T) {
	p := NewPositionIncrementAttributeImpl()
	p.SetPositionIncrement(5)
	if p.GetPositionIncrement() != 5 {
		t.Fatalf("want 5, got %d", p.GetPositionIncrement())
	}
}

func TestPositionIncrementAttributeImpl_SetZero(t *testing.T) {
	p := NewPositionIncrementAttributeImpl()
	p.SetPositionIncrement(0)
	if p.GetPositionIncrement() != 0 {
		t.Fatalf("want 0, got %d", p.GetPositionIncrement())
	}
}

func TestPositionIncrementAttributeImpl_SetNegative_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("SetPositionIncrement(-1) must panic")
		}
	}()
	NewPositionIncrementAttributeImpl().SetPositionIncrement(-1)
}

func TestPositionIncrementAttributeImpl_Clear(t *testing.T) {
	p := NewPositionIncrementAttributeImpl()
	p.SetPositionIncrement(7)
	p.Clear()
	if p.GetPositionIncrement() != 1 {
		t.Fatalf("after Clear want 1, got %d", p.GetPositionIncrement())
	}
}

func TestPositionIncrementAttributeImpl_End(t *testing.T) {
	p := NewPositionIncrementAttributeImpl()
	p.SetPositionIncrement(3)
	p.End()
	if p.GetPositionIncrement() != 0 {
		t.Fatalf("after End want 0, got %d", p.GetPositionIncrement())
	}
}

func TestPositionIncrementAttributeImpl_CopyTo(t *testing.T) {
	src := NewPositionIncrementAttributeImpl()
	src.SetPositionIncrement(4)
	dst := NewPositionIncrementAttributeImpl()
	src.CopyTo(dst)
	if !src.Equals(dst) {
		t.Fatal("CopyTo: destination does not equal source")
	}
}

func TestPositionIncrementAttributeImpl_Copy(t *testing.T) {
	src := NewPositionIncrementAttributeImpl()
	src.SetPositionIncrement(9)
	clone, ok := src.Copy().(*PositionIncrementAttributeImpl)
	if !ok {
		t.Fatal("Copy must return *PositionIncrementAttributeImpl")
	}
	if !src.Equals(clone) {
		t.Fatal("copy must equal original")
	}
}

func TestPositionIncrementAttributeImpl_CloneAttribute(t *testing.T) {
	src := NewPositionIncrementAttributeImpl()
	src.SetPositionIncrement(6)
	clone, ok := src.CloneAttribute().(*PositionIncrementAttributeImpl)
	if !ok {
		t.Fatal("CloneAttribute must return *PositionIncrementAttributeImpl")
	}
	if !src.Equals(clone) {
		t.Fatal("clone must equal original")
	}
}

func TestPositionIncrementAttributeImpl_Equals(t *testing.T) {
	a := NewPositionIncrementAttributeImpl()
	a.SetPositionIncrement(2)
	b := NewPositionIncrementAttributeImpl()
	b.SetPositionIncrement(2)
	c := NewPositionIncrementAttributeImpl()
	c.SetPositionIncrement(3)
	if !a.Equals(b) {
		t.Fatal("equal values must be equal")
	}
	if a.Equals(c) {
		t.Fatal("different values must not be equal")
	}
}

func TestPositionIncrementAttributeImpl_HashCode(t *testing.T) {
	p := NewPositionIncrementAttributeImpl()
	p.SetPositionIncrement(5)
	if p.HashCode() != 5 {
		t.Fatalf("hash must equal positionIncrement; want 5, got %d", p.HashCode())
	}
}

func TestPositionIncrementAttributeImpl_AttributeInterfaces(t *testing.T) {
	p := NewPositionIncrementAttributeImpl()
	ifaces := p.AttributeInterfaces()
	if len(ifaces) != 1 || ifaces[0] != PositionIncrementAttributeType {
		t.Fatalf("unexpected AttributeInterfaces: %v", ifaces)
	}
}

func TestPositionIncrementAttributeImpl_CopyTo_WrongTarget(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("CopyTo with wrong target type must panic")
		}
	}()
	src := NewPositionIncrementAttributeImpl()
	src.CopyTo(NewSentenceAttributeImpl())
}
