// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"testing"
)

func TestKeywordAttributeImpl_DefaultState(t *testing.T) {
	k := NewKeywordAttributeImpl()
	if k.IsKeywordToken() {
		t.Fatal("default keyword flag must be false")
	}
}

func TestKeywordAttributeImpl_SetGet(t *testing.T) {
	k := NewKeywordAttributeImpl()
	k.SetKeyword(true)
	if !k.IsKeywordToken() {
		t.Fatal("IsKeywordToken must return true after SetKeyword(true)")
	}
	k.SetKeyword(false)
	if k.IsKeywordToken() {
		t.Fatal("IsKeywordToken must return false after SetKeyword(false)")
	}
}

func TestKeywordAttributeImpl_Clear(t *testing.T) {
	k := NewKeywordAttributeImpl()
	k.SetKeyword(true)
	k.Clear()
	if k.IsKeywordToken() {
		t.Fatal("after Clear keyword must be false")
	}
}

func TestKeywordAttributeImpl_End(t *testing.T) {
	k := NewKeywordAttributeImpl()
	k.SetKeyword(true)
	k.End()
	if k.IsKeywordToken() {
		t.Fatal("after End keyword must be false")
	}
}

func TestKeywordAttributeImpl_CopyTo(t *testing.T) {
	src := NewKeywordAttributeImpl()
	src.SetKeyword(true)
	dst := NewKeywordAttributeImpl()
	src.CopyTo(dst)
	if !dst.IsKeywordToken() {
		t.Fatal("CopyTo: destination keyword must be true")
	}
}

func TestKeywordAttributeImpl_CloneAttribute(t *testing.T) {
	src := NewKeywordAttributeImpl()
	src.SetKeyword(true)
	clone, ok := src.CloneAttribute().(*KeywordAttributeImpl)
	if !ok {
		t.Fatal("CloneAttribute must return *KeywordAttributeImpl")
	}
	if !src.Equals(clone) {
		t.Fatal("clone must equal original")
	}
}

func TestKeywordAttributeImpl_Equals(t *testing.T) {
	a := NewKeywordAttributeImpl()
	a.SetKeyword(true)
	b := NewKeywordAttributeImpl()
	b.SetKeyword(true)
	c := NewKeywordAttributeImpl()
	c.SetKeyword(false)
	if !a.Equals(b) {
		t.Fatal("equal flags must be equal")
	}
	if a.Equals(c) {
		t.Fatal("different flags must not be equal")
	}
}

func TestKeywordAttributeImpl_HashCode(t *testing.T) {
	trueK := NewKeywordAttributeImpl()
	trueK.SetKeyword(true)
	falseK := NewKeywordAttributeImpl()

	if trueK.HashCode() != 31 {
		t.Fatalf("keyword=true hash must be 31, got %d", trueK.HashCode())
	}
	if falseK.HashCode() != 37 {
		t.Fatalf("keyword=false hash must be 37, got %d", falseK.HashCode())
	}
}

func TestKeywordAttributeImpl_AttributeInterfaces(t *testing.T) {
	k := NewKeywordAttributeImpl()
	ifaces := k.AttributeInterfaces()
	if len(ifaces) != 1 || ifaces[0] != KeywordAttributeType {
		t.Fatalf("unexpected AttributeInterfaces: %v", ifaces)
	}
}

func TestKeywordAttributeImpl_CopyTo_WrongTarget(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("CopyTo with wrong target type must panic")
		}
	}()
	src := NewKeywordAttributeImpl()
	src.CopyTo(NewSentenceAttributeImpl())
}
