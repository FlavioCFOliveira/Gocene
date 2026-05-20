// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"testing"
)

func TestTermFrequencyAttributeImpl_DefaultState(t *testing.T) {
	tf := NewTermFrequencyAttributeImpl()
	if tf.GetTermFrequency() != 1 {
		t.Fatalf("default term frequency must be 1, got %d", tf.GetTermFrequency())
	}
}

func TestTermFrequencyAttributeImpl_SetGet(t *testing.T) {
	tf := NewTermFrequencyAttributeImpl()
	tf.SetTermFrequency(5)
	if tf.GetTermFrequency() != 5 {
		t.Fatalf("want 5, got %d", tf.GetTermFrequency())
	}
}

func TestTermFrequencyAttributeImpl_SetTermFrequencyValidated_Valid(t *testing.T) {
	tf := NewTermFrequencyAttributeImpl()
	tf.SetTermFrequencyValidated(3)
	if tf.GetTermFrequency() != 3 {
		t.Fatalf("want 3, got %d", tf.GetTermFrequency())
	}
}

func TestTermFrequencyAttributeImpl_SetTermFrequencyValidated_Invalid(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("SetTermFrequencyValidated(0) must panic")
		}
	}()
	NewTermFrequencyAttributeImpl().SetTermFrequencyValidated(0)
}

func TestTermFrequencyAttributeImpl_Clear(t *testing.T) {
	tf := NewTermFrequencyAttributeImpl()
	tf.SetTermFrequency(7)
	tf.Clear()
	if tf.GetTermFrequency() != 1 {
		t.Fatalf("after Clear want 1, got %d", tf.GetTermFrequency())
	}
}

func TestTermFrequencyAttributeImpl_End(t *testing.T) {
	tf := NewTermFrequencyAttributeImpl()
	tf.SetTermFrequency(4)
	tf.End()
	if tf.GetTermFrequency() != 1 {
		t.Fatalf("after End want 1, got %d", tf.GetTermFrequency())
	}
}

func TestTermFrequencyAttributeImpl_CopyTo(t *testing.T) {
	src := NewTermFrequencyAttributeImpl()
	src.SetTermFrequency(9)
	dst := NewTermFrequencyAttributeImpl()
	src.CopyTo(dst)
	if !src.Equals(dst) {
		t.Fatal("CopyTo: destination does not equal source")
	}
}

func TestTermFrequencyAttributeImpl_CloneAttribute(t *testing.T) {
	src := NewTermFrequencyAttributeImpl()
	src.SetTermFrequency(6)
	clone, ok := src.CloneAttribute().(*TermFrequencyAttributeImpl)
	if !ok {
		t.Fatal("CloneAttribute must return *TermFrequencyAttributeImpl")
	}
	if !src.Equals(clone) {
		t.Fatal("clone must equal original")
	}
}

func TestTermFrequencyAttributeImpl_Equals(t *testing.T) {
	a := NewTermFrequencyAttributeImpl()
	a.SetTermFrequency(2)
	b := NewTermFrequencyAttributeImpl()
	b.SetTermFrequency(2)
	c := NewTermFrequencyAttributeImpl()
	c.SetTermFrequency(3)
	if !a.Equals(b) {
		t.Fatal("equal values must be equal")
	}
	if a.Equals(c) {
		t.Fatal("different values must not be equal")
	}
}

func TestTermFrequencyAttributeImpl_HashCode(t *testing.T) {
	a := NewTermFrequencyAttributeImpl()
	a.SetTermFrequency(5)
	if a.HashCode() != 5 {
		t.Fatalf("hash must equal termFrequency; want 5, got %d", a.HashCode())
	}
}

func TestTermFrequencyAttributeImpl_AttributeInterfaces(t *testing.T) {
	tf := NewTermFrequencyAttributeImpl()
	ifaces := tf.AttributeInterfaces()
	if len(ifaces) != 1 || ifaces[0] != TermFrequencyAttributeType {
		t.Fatalf("unexpected AttributeInterfaces: %v", ifaces)
	}
}

func TestTermFrequencyAttributeImpl_CopyTo_WrongTarget(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("CopyTo with wrong target type must panic")
		}
	}()
	src := NewTermFrequencyAttributeImpl()
	src.CopyTo(NewSentenceAttributeImpl())
}
