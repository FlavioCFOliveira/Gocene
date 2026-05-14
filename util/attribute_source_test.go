// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"reflect"
	"testing"
)

// Lucene-shaped fake attributes used in the AttributeSource tests.

type asTermAttribute interface {
	Attribute
	Term() string
	SetTerm(string)
}

type asTypeAttribute interface {
	Attribute
	Type() string
	SetType(string)
}

type asFlagsAttribute interface {
	Attribute
	Flags() int
	SetFlags(int)
}

type asTermAttributeImpl struct {
	BaseAttributeImpl
	term string
}

func (a *asTermAttributeImpl) Term() string                  { return a.term }
func (a *asTermAttributeImpl) SetTerm(s string)              { a.term = s }
func (a *asTermAttributeImpl) Clear()                        { a.term = "" }
func (a *asTermAttributeImpl) End()                          { a.Clear() }
func (a *asTermAttributeImpl) CloneAttribute() AttributeImpl { c := *a; return &c }
func (a *asTermAttributeImpl) CopyTo(target AttributeImpl) {
	t, ok := target.(*asTermAttributeImpl)
	if !ok {
		panic("CopyTo target must be *asTermAttributeImpl")
	}
	t.term = a.term
}
func (a *asTermAttributeImpl) ReflectWith(r AttributeReflector) {
	r(asTermAttributeType, "term", a.term)
}

type asTypeAttributeImpl struct {
	BaseAttributeImpl
	typeStr string
}

func (a *asTypeAttributeImpl) Type() string                  { return a.typeStr }
func (a *asTypeAttributeImpl) SetType(s string)              { a.typeStr = s }
func (a *asTypeAttributeImpl) Clear()                        { a.typeStr = "" }
func (a *asTypeAttributeImpl) End()                          { a.Clear() }
func (a *asTypeAttributeImpl) CloneAttribute() AttributeImpl { c := *a; return &c }
func (a *asTypeAttributeImpl) CopyTo(target AttributeImpl) {
	t, ok := target.(*asTypeAttributeImpl)
	if !ok {
		panic("CopyTo target must be *asTypeAttributeImpl")
	}
	t.typeStr = a.typeStr
}
func (a *asTypeAttributeImpl) ReflectWith(r AttributeReflector) {
	r(asTypeAttributeType, "type", a.typeStr)
}

type asFlagsAttributeImpl struct {
	BaseAttributeImpl
	flags int
}

func (a *asFlagsAttributeImpl) Flags() int                    { return a.flags }
func (a *asFlagsAttributeImpl) SetFlags(f int)                { a.flags = f }
func (a *asFlagsAttributeImpl) Clear()                        { a.flags = 0 }
func (a *asFlagsAttributeImpl) End()                          { a.Clear() }
func (a *asFlagsAttributeImpl) CloneAttribute() AttributeImpl { c := *a; return &c }
func (a *asFlagsAttributeImpl) CopyTo(target AttributeImpl) {
	t, ok := target.(*asFlagsAttributeImpl)
	if !ok {
		panic("CopyTo target must be *asFlagsAttributeImpl")
	}
	t.flags = a.flags
}
func (a *asFlagsAttributeImpl) ReflectWith(r AttributeReflector) {
	r(asFlagsAttributeType, "flags", a.flags)
}

var (
	asTermAttributeType  = reflect.TypeOf((*asTermAttribute)(nil)).Elem()
	asTypeAttributeType  = reflect.TypeOf((*asTypeAttribute)(nil)).Elem()
	asFlagsAttributeType = reflect.TypeOf((*asFlagsAttribute)(nil)).Elem()
)

func init() {
	RegisterAttributeImpl(asTermAttributeType, func() AttributeImpl { return &asTermAttributeImpl{} })
	RegisterAttributeImpl(asTypeAttributeType, func() AttributeImpl { return &asTypeAttributeImpl{} })
	RegisterAttributeImpl(asFlagsAttributeType, func() AttributeImpl { return &asFlagsAttributeImpl{} })
}

// TestAttributeSource_AddAndGet mirrors testDefaultAttributeFactory's
// roundtrip: AddAttribute returns an instance of the registered
// concrete impl, and HasAttribute reflects insertion.
func TestAttributeSource_AddAndGet(t *testing.T) {
	src := NewAttributeSource()
	if src.HasAttributes() {
		t.Fatal("freshly created source must report HasAttributes() == false")
	}

	termAtt := src.AddAttribute(asTermAttributeType).(asTermAttribute)
	termAtt.SetTerm("hello")

	if !src.HasAttribute(asTermAttributeType) {
		t.Error("HasAttribute(asTermAttributeType) returned false after AddAttribute")
	}
	if !src.HasAttributes() {
		t.Error("HasAttributes() returned false after AddAttribute")
	}
	got := src.GetAttribute(asTermAttributeType).(asTermAttribute)
	if got.Term() != "hello" {
		t.Errorf("GetAttribute term = %q, want %q", got.Term(), "hello")
	}
}

// TestAttributeSource_AddAttributeIdempotent ensures repeated AddAttribute
// returns the same instance.
func TestAttributeSource_AddAttributeIdempotent(t *testing.T) {
	src := NewAttributeSource()
	a := src.AddAttribute(asTermAttributeType)
	b := src.AddAttribute(asTermAttributeType)
	if a != b {
		t.Error("AddAttribute should return the same instance on repeated calls")
	}
}

// TestAttributeSource_CaptureRestoreState mirrors testCaptureState: a
// captured state must survive subsequent mutation and be restorable.
func TestAttributeSource_CaptureRestoreState(t *testing.T) {
	src := NewAttributeSource()
	termAtt := src.AddAttribute(asTermAttributeType).(asTermAttribute)
	typeAtt := src.AddAttribute(asTypeAttributeType).(asTypeAttribute)
	termAtt.SetTerm("TestTerm")
	typeAtt.SetType("TestType")
	originalHash := src.HashCode()

	state := src.CaptureState()

	termAtt.SetTerm("AnotherTestTerm")
	typeAtt.SetType("AnotherTestType")
	if src.HashCode() == originalHash {
		t.Error("HashCode should change after mutation")
	}

	src.RestoreState(state)
	if termAtt.Term() != "TestTerm" || typeAtt.Type() != "TestType" {
		t.Errorf("after RestoreState got term=%q type=%q", termAtt.Term(), typeAtt.Type())
	}
	if src.HashCode() != originalHash {
		t.Error("HashCode should match original after RestoreState")
	}

	// Restoring into another source with the same impls.
	copySrc := NewAttributeSource()
	copySrc.AddAttribute(asTermAttributeType)
	copySrc.AddAttribute(asTypeAttributeType)
	copySrc.RestoreState(state)
	if !copySrc.Equals(src) {
		t.Error("after RestoreState into copy, copy must Equals src")
	}
	if copySrc.HashCode() != src.HashCode() {
		t.Error("after RestoreState into copy, hash codes must match")
	}
}

// TestAttributeSource_RestoreState_MissingImplPanics mirrors the
// testCaptureState IllegalArgumentException expectation.
func TestAttributeSource_RestoreState_MissingImplPanics(t *testing.T) {
	src := NewAttributeSource()
	src.AddAttribute(asTermAttributeType)
	src.AddAttribute(asTypeAttributeType)
	state := src.CaptureState()

	other := NewAttributeSource()
	other.AddAttribute(asTermAttributeType) // missing typeAtt

	defer func() {
		if r := recover(); r == nil {
			t.Error("RestoreState with missing target impl should panic")
		}
	}()
	other.RestoreState(state)
}

// TestAttributeSource_CloneAttributes mirrors testCloneAttributes:
// clone preserves order, contains deep copies, and copyTo round-trips.
func TestAttributeSource_CloneAttributes(t *testing.T) {
	src := NewAttributeSource()
	flagsAtt := src.AddAttribute(asFlagsAttributeType).(asFlagsAttribute)
	typeAtt := src.AddAttribute(asTypeAttributeType).(asTypeAttribute)
	flagsAtt.SetFlags(1234)
	typeAtt.SetType("TestType")

	clone := src.CloneAttributes()
	classes := clone.GetAttributeClassesIterator()
	if len(classes) != 2 {
		t.Fatalf("clone has %d classes, want 2", len(classes))
	}
	if classes[0] != asFlagsAttributeType || classes[1] != asTypeAttributeType {
		t.Errorf("clone order = %v, want [flags, type]", classes)
	}

	flagsAtt2 := clone.GetAttribute(asFlagsAttributeType).(asFlagsAttribute)
	typeAtt2 := clone.GetAttribute(asTypeAttributeType).(asTypeAttribute)
	if flagsAtt2 == flagsAtt {
		t.Error("clone flags impl should be a different instance")
	}
	if typeAtt2 == typeAtt {
		t.Error("clone type impl should be a different instance")
	}
	if flagsAtt2.Flags() != 1234 || typeAtt2.Type() != "TestType" {
		t.Error("clone should carry equal state")
	}

	// Mutate the clone and copy back.
	flagsAtt2.SetFlags(4711)
	typeAtt2.SetType("OtherType")
	clone.CopyTo(src)
	if flagsAtt.Flags() != 4711 {
		t.Errorf("after copyTo flags = %d, want 4711", flagsAtt.Flags())
	}
	if typeAtt.Type() != "OtherType" {
		t.Errorf("after copyTo type = %q, want OtherType", typeAtt.Type())
	}
}

// TestAttributeSource_RemoveAllAttributes mirrors testRemoveAllAttributes.
func TestAttributeSource_RemoveAllAttributes(t *testing.T) {
	src := NewAttributeSource()
	types := []reflect.Type{asTermAttributeType, asTypeAttributeType, asFlagsAttributeType}
	for _, tp := range types {
		src.AddAttribute(tp)
		if !src.HasAttribute(tp) {
			t.Errorf("missing added attribute %v", tp)
		}
	}
	src.RemoveAllAttributes()
	for _, tp := range types {
		if src.HasAttribute(tp) {
			t.Errorf("attribute %v still present after RemoveAllAttributes", tp)
		}
	}
	if src.HasAttributes() {
		t.Error("HasAttributes should be false after RemoveAllAttributes")
	}
}

// TestAttributeSource_AddAttributeRejectsNonInterface mirrors
// testInvalidArguments.
func TestAttributeSource_AddAttributeRejectsNonInterface(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("AddAttribute with non-interface type should panic")
		}
	}()
	src := NewAttributeSource()
	src.AddAttribute(reflect.TypeOf(0))
}

// TestAttributeSource_Lucene3042 mirrors testLUCENE_3042: a source
// built from another source preserves the cached state of the input
// and observes subsequent mutations.
func TestAttributeSource_Lucene3042(t *testing.T) {
	src1 := NewAttributeSource()
	src1.AddAttribute(asTermAttributeType).(asTermAttribute).SetTerm("foo")
	hash1 := src1.HashCode() // primes the cached state

	src2 := NewAttributeSourceFrom(src1)
	src2.AddAttribute(asTypeAttributeType).(asTypeAttribute).SetType("bar")

	if hash1 == src1.HashCode() {
		t.Error("hash1 must differ from src1.HashCode() after src2 added a new attribute")
	}
	if src2.HashCode() != src1.HashCode() {
		t.Error("src1 and src2 share state, hashes must be equal")
	}
}

// TestAttributeSource_ClearAttributes verifies ClearAttributes resets
// every impl to its default.
func TestAttributeSource_ClearAttributes(t *testing.T) {
	src := NewAttributeSource()
	src.AddAttribute(asTermAttributeType).(asTermAttribute).SetTerm("x")
	src.AddAttribute(asFlagsAttributeType).(asFlagsAttribute).SetFlags(99)

	src.ClearAttributes()

	if got := src.GetAttribute(asTermAttributeType).(asTermAttribute).Term(); got != "" {
		t.Errorf("term not cleared: %q", got)
	}
	if got := src.GetAttribute(asFlagsAttributeType).(asFlagsAttribute).Flags(); got != 0 {
		t.Errorf("flags not cleared: %d", got)
	}
}

// TestAttributeSource_ReflectAsString verifies the deterministic
// reflect output (no class prefix).
func TestAttributeSource_ReflectAsString(t *testing.T) {
	src := NewAttributeSource()
	src.AddAttribute(asTermAttributeType).(asTermAttribute).SetTerm("hello")
	src.AddAttribute(asFlagsAttributeType).(asFlagsAttribute).SetFlags(7)

	got := src.ReflectAsString(false)
	want := "term=hello,flags=7"
	if got != want {
		t.Errorf("ReflectAsString(false) = %q, want %q", got, want)
	}
}
