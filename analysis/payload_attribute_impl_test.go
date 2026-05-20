// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"testing"
)

func TestPayloadAttributeImpl_DefaultState(t *testing.T) {
	p := NewPayloadAttributeImpl()
	if p.GetPayload() != nil {
		t.Fatalf("expected nil payload, got %v", p.GetPayload())
	}
	if p.HasPayload() {
		t.Fatal("expected HasPayload=false for empty attribute")
	}
}

func TestPayloadAttributeImpl_SetGet(t *testing.T) {
	p := NewPayloadAttributeImpl()
	payload := []byte{1, 2, 3}
	p.SetPayload(payload)
	got := p.GetPayload()
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("unexpected payload %v", got)
	}
	// Verify deep copy: mutating original does not affect attribute.
	payload[0] = 99
	if p.GetPayload()[0] == 99 {
		t.Fatal("SetPayload must deep-copy; original slice shares backing array")
	}
}

func TestPayloadAttributeImpl_Clear(t *testing.T) {
	p := NewPayloadAttributeImplWithPayload([]byte{7, 8})
	p.Clear()
	if p.GetPayload() != nil {
		t.Fatalf("expected nil after Clear, got %v", p.GetPayload())
	}
}

func TestPayloadAttributeImpl_End(t *testing.T) {
	p := NewPayloadAttributeImplWithPayload([]byte{1})
	p.End()
	if p.GetPayload() != nil {
		t.Fatal("End must clear payload")
	}
}

func TestPayloadAttributeImpl_CopyTo(t *testing.T) {
	src := NewPayloadAttributeImplWithPayload([]byte{10, 20, 30})
	dst := NewPayloadAttributeImpl()
	src.CopyTo(dst)
	if !dst.Equals(src) {
		t.Fatal("CopyTo: destination does not equal source after copy")
	}
	// Verify isolation.
	src.Clear()
	if dst.GetPayload() == nil {
		t.Fatal("CopyTo: clearing source must not affect destination")
	}
}

func TestPayloadAttributeImpl_CopyTo_NilPayload(t *testing.T) {
	src := NewPayloadAttributeImpl()
	dst := NewPayloadAttributeImplWithPayload([]byte{1})
	src.CopyTo(dst)
	if dst.GetPayload() != nil {
		t.Fatal("CopyTo with nil payload must clear destination")
	}
}

func TestPayloadAttributeImpl_CloneAttribute(t *testing.T) {
	src := NewPayloadAttributeImplWithPayload([]byte{5, 6, 7})
	clone, ok := src.CloneAttribute().(*PayloadAttributeImpl)
	if !ok {
		t.Fatal("CloneAttribute must return *PayloadAttributeImpl")
	}
	if !src.Equals(clone) {
		t.Fatal("clone must equal original")
	}
	// Isolation check.
	src.Clear()
	if clone.GetPayload() == nil {
		t.Fatal("clone must be independent of original")
	}
}

func TestPayloadAttributeImpl_Equals(t *testing.T) {
	a := NewPayloadAttributeImplWithPayload([]byte{1, 2})
	b := NewPayloadAttributeImplWithPayload([]byte{1, 2})
	c := NewPayloadAttributeImplWithPayload([]byte{1, 3})
	nilA := NewPayloadAttributeImpl()
	nilB := NewPayloadAttributeImpl()

	if !a.Equals(b) {
		t.Fatal("equal payloads must be equal")
	}
	if a.Equals(c) {
		t.Fatal("different payloads must not be equal")
	}
	if !nilA.Equals(nilB) {
		t.Fatal("two nil payloads must be equal")
	}
	if a.Equals(nilA) {
		t.Fatal("non-nil must not equal nil")
	}
}

func TestPayloadAttributeImpl_HashCode(t *testing.T) {
	a := NewPayloadAttributeImplWithPayload([]byte{1, 2})
	b := NewPayloadAttributeImplWithPayload([]byte{1, 2})
	if a.HashCode() != b.HashCode() {
		t.Fatal("equal attributes must have equal hash codes")
	}
	nilP := NewPayloadAttributeImpl()
	if nilP.HashCode() != 0 {
		t.Fatalf("nil payload hashcode must be 0, got %d", nilP.HashCode())
	}
}

func TestPayloadAttributeImpl_CopyTo_WrongTarget(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("CopyTo with wrong target type must panic")
		}
	}()
	src := NewPayloadAttributeImplWithPayload([]byte{1})
	src.CopyTo(NewSentenceAttributeImpl())
}

func TestPayloadAttributeImpl_AttributeInterfaces(t *testing.T) {
	p := NewPayloadAttributeImpl()
	ifaces := p.AttributeInterfaces()
	if len(ifaces) != 1 || ifaces[0] != PayloadAttributeType {
		t.Fatalf("unexpected AttributeInterfaces: %v", ifaces)
	}
}
