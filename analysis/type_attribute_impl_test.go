// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"testing"
)

func TestTypeAttributeImpl_DefaultState(t *testing.T) {
	ty := NewTypeAttributeImpl()
	if ty.GetType() != DefaultTypeAttributeValue {
		t.Fatalf("default type must be %q, got %q", DefaultTypeAttributeValue, ty.GetType())
	}
}

func TestTypeAttributeImpl_SetGet(t *testing.T) {
	ty := NewTypeAttributeImpl()
	ty.SetType("<ALPHANUM>")
	if ty.GetType() != "<ALPHANUM>" {
		t.Fatalf("want %q, got %q", "<ALPHANUM>", ty.GetType())
	}
}

func TestTypeAttributeImpl_WithType(t *testing.T) {
	ty := NewTypeAttributeImplWithType("acronym")
	if ty.GetType() != "acronym" {
		t.Fatalf("want %q, got %q", "acronym", ty.GetType())
	}
}

func TestTypeAttributeImpl_Clear(t *testing.T) {
	ty := NewTypeAttributeImpl()
	ty.SetType("custom")
	ty.Clear()
	if ty.GetType() != DefaultTypeAttributeValue {
		t.Fatalf("after Clear want %q, got %q", DefaultTypeAttributeValue, ty.GetType())
	}
}

func TestTypeAttributeImpl_End(t *testing.T) {
	ty := NewTypeAttributeImpl()
	ty.SetType("custom")
	ty.End()
	if ty.GetType() != DefaultTypeAttributeValue {
		t.Fatalf("after End want %q, got %q", DefaultTypeAttributeValue, ty.GetType())
	}
}

func TestTypeAttributeImpl_CopyTo(t *testing.T) {
	src := NewTypeAttributeImplWithType("test")
	dst := NewTypeAttributeImpl()
	src.CopyTo(dst)
	if !src.Equals(dst) {
		t.Fatal("CopyTo: destination does not equal source")
	}
}

func TestTypeAttributeImpl_CloneAttribute(t *testing.T) {
	src := NewTypeAttributeImplWithType("cloned")
	clone, ok := src.CloneAttribute().(*TypeAttributeImpl)
	if !ok {
		t.Fatal("CloneAttribute must return *TypeAttributeImpl")
	}
	if !src.Equals(clone) {
		t.Fatal("clone must equal original")
	}
}

func TestTypeAttributeImpl_Equals(t *testing.T) {
	a := NewTypeAttributeImplWithType("word")
	b := NewTypeAttributeImplWithType("word")
	c := NewTypeAttributeImplWithType("other")
	if !a.Equals(b) {
		t.Fatal("equal types must be equal")
	}
	if a.Equals(c) {
		t.Fatal("different types must not be equal")
	}
}

func TestTypeAttributeImpl_HashCode(t *testing.T) {
	a := NewTypeAttributeImplWithType("word")
	b := NewTypeAttributeImplWithType("word")
	if a.HashCode() != b.HashCode() {
		t.Fatal("equal types must have equal hash codes")
	}
	// The existing typeAttributeImpl uses the same javaStringHash; verify
	// the values align.
	c := NewTypeAttributeImplWithType("")
	if c.HashCode() != 0 {
		t.Fatalf("empty string hash must be 0, got %d", c.HashCode())
	}
}

func TestTypeAttributeImpl_AttributeInterfaces(t *testing.T) {
	ty := NewTypeAttributeImpl()
	ifaces := ty.AttributeInterfaces()
	if len(ifaces) != 1 || ifaces[0] != TypeAttributeType {
		t.Fatalf("unexpected AttributeInterfaces: %v", ifaces)
	}
}

func TestTypeAttributeImpl_CopyTo_WrongTarget(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("CopyTo with wrong target type must panic")
		}
	}()
	src := NewTypeAttributeImpl()
	src.CopyTo(NewSentenceAttributeImpl())
}

func TestTypeAttributeImpl_DistinctFromPrivateImpl(t *testing.T) {
	// Verify that TypeAttributeImpl and the private typeAttributeImpl
	// are independent types (the exported type does NOT satisfy the
	// Equals of the private type and vice-versa).
	pub := NewTypeAttributeImpl()
	priv := NewTypeAttribute() // returns *typeAttributeImpl
	// pub.Equals only matches *TypeAttributeImpl, not *typeAttributeImpl.
	if pub.Equals(priv) {
		t.Fatal("exported TypeAttributeImpl must not equal private typeAttributeImpl")
	}
}
