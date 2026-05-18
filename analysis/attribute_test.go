// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// MockAttribute is a mock implementation of AttributeImpl for testing.
type MockAttribute struct {
	value string
}

func (m *MockAttribute) Clear() {
	m.value = ""
}

func (m *MockAttribute) CopyTo(target AttributeImpl) {
	if mock, ok := target.(*MockAttribute); ok {
		mock.value = m.value
	}
}

func (m *MockAttribute) Copy() AttributeImpl {
	return &MockAttribute{value: m.value}
}

// End satisfies the Sprint 54 Phase 2 util.AttributeImpl surface. The
// default (Lucene-faithful) behaviour is to delegate to Clear; this
// mirrors the Java {@code AttributeImpl#end() -> clear()} contract.
func (m *MockAttribute) End() { m.Clear() }

// ReflectWith is a no-op for the bare MockAttribute, matching the
// Sprint 12 contract where a mock that does not opt into reflection
// emits no triples. [reflectableMockAttribute] overrides this method to
// exercise the opt-in path.
func (m *MockAttribute) ReflectWith(reflector AttributeReflector) {}

// CloneAttribute satisfies util.AttributeImpl.CloneAttribute by
// delegating to the existing Copy().
func (m *MockAttribute) CloneAttribute() util.AttributeImpl { return m.Copy() }

// TestAttributeImpl_Clear tests the Clear method.
// Source: TestAttributeImpl.java
// Purpose: Tests that attributes can be cleared.
func TestAttributeImpl_Clear(t *testing.T) {
	attr := &MockAttribute{value: "test"}
	attr.Clear()
	if attr.value != "" {
		t.Error("Clear() should reset the attribute value")
	}
}

// TestAttributeImpl_CopyTo tests the CopyTo method.
// Source: TestAttributeImpl.java
// Purpose: Tests that attributes can be copied to another instance.
func TestAttributeImpl_CopyTo(t *testing.T) {
	source := &MockAttribute{value: "source"}
	target := &MockAttribute{value: "target"}

	source.CopyTo(target)

	if target.value != "source" {
		t.Error("CopyTo() should copy the value to target")
	}
}

// TestAttributeImpl_Copy tests the Copy method.
// Source: TestAttributeImpl.java
// Purpose: Tests that attributes can be deep copied.
func TestAttributeImpl_Copy(t *testing.T) {
	original := &MockAttribute{value: "original"}
	copy := original.Copy()

	if copy == nil {
		t.Fatal("Copy() should return a non-nil attribute")
	}

	if mock, ok := copy.(*MockAttribute); ok {
		if mock.value != "original" {
			t.Error("Copy() should copy the value")
		}

		// Modify copy and verify original is unchanged
		mock.value = "modified"
		if original.value != "original" {
			t.Error("Copy() should create an independent copy")
		}
	} else {
		t.Error("Copy() should return the correct type")
	}
}

// TestAttributeFactory tests AttributeFactory implementations.
func TestAttributeFactory(t *testing.T) {
	factory := &MockAttributeFactory{}

	attr := factory.CreateAttributeInstance("MockAttribute")
	if attr == nil {
		t.Error("CreateAttributeInstance() should return a non-nil attribute")
	}
}

// MockAttributeFactory is a mock implementation of AttributeFactory.
type MockAttributeFactory struct{}

func (f *MockAttributeFactory) CreateAttributeInstance(attribType string) AttributeImpl {
	if attribType == "MockAttribute" {
		return &MockAttribute{}
	}
	return nil
}

// TestAttributeInterface tests that Attribute interface is properly defined.
func TestAttributeInterface(t *testing.T) {
	// Verify that MockAttribute implements Attribute
	var _ Attribute = (*MockAttribute)(nil)

	// Verify that MockAttribute implements AttributeImpl
	var _ AttributeImpl = (*MockAttribute)(nil)
}

// reflectableMockAttribute is a MockAttribute variant that opts into
// AttributeReflectable, used to exercise the Sprint 12 reflection
// helpers in [TestReflectWith_OptIn] and [TestReflectAsString_Format].
type reflectableMockAttribute struct {
	MockAttribute
}

func (r *reflectableMockAttribute) ReflectWith(reflector AttributeReflector) {
	reflector(reflect.TypeOf((*Attribute)(nil)).Elem(), "value", r.value)
}

// endingMockAttribute is a MockAttribute variant that opts into
// AttributeEnder with a distinct end-of-field state.
type endingMockAttribute struct {
	MockAttribute
	ended bool
}

func (e *endingMockAttribute) End() {
	e.value = ""
	e.ended = true
}

// TestReflectWith_OptIn verifies that the [ReflectWith] helper invokes
// an impl's ReflectWith hook when it opts into [AttributeReflectable]
// and is a no-op otherwise. Mirrors the Lucene contract that the
// default {@code reflectWith} emits nothing.
func TestReflectWith_OptIn(t *testing.T) {
	got := make(map[string]any)
	collector := func(_ reflect.Type, key string, value any) {
		got[key] = value
	}

	// Bare AttributeImpl does not opt in: helper is a no-op.
	ReflectWith(&MockAttribute{value: "ignored"}, collector)
	if len(got) != 0 {
		t.Fatalf("ReflectWith on non-Reflectable impl emitted %d triples, want 0", len(got))
	}

	// Opting in dispatches the hook.
	ReflectWith(&reflectableMockAttribute{MockAttribute{value: "v"}}, collector)
	if v, ok := got["value"]; !ok || v != "v" {
		t.Fatalf("ReflectWith on Reflectable impl: got %#v, want value=v", got)
	}
}

// TestEnd_DefaultsToClear verifies that the [End] helper falls back to
// Clear when the impl does not opt into [AttributeEnder], matching the
// Java default {@code AttributeImpl#end() -> clear()}, and dispatches
// the override when the impl opts in.
func TestEnd_DefaultsToClear(t *testing.T) {
	// Default: End delegates to Clear.
	plain := &MockAttribute{value: "stale"}
	End(plain)
	if plain.value != "" {
		t.Fatalf("End fallback: value=%q, want %q", plain.value, "")
	}

	// Opt-in: End override is invoked.
	custom := &endingMockAttribute{MockAttribute: MockAttribute{value: "stale"}}
	End(custom)
	if !custom.ended {
		t.Fatal("End on AttributeEnder did not invoke override")
	}
	if custom.value != "" {
		t.Fatalf("End on AttributeEnder: value=%q, want %q", custom.value, "")
	}
}

// TestReflectAsString_Format verifies both rendering modes documented
// by [ReflectAsString], byte-for-byte against the Lucene reference for
// a single-key impl.
func TestReflectAsString_Format(t *testing.T) {
	attr := &reflectableMockAttribute{MockAttribute{value: "v"}}

	got := ReflectAsString(attr, false)
	if want := "value=v"; got != want {
		t.Fatalf("ReflectAsString(false)=%q, want %q", got, want)
	}

	got = ReflectAsString(attr, true)
	want := "analysis.Attribute#value=v"
	if got != want {
		t.Fatalf("ReflectAsString(true)=%q, want %q", got, want)
	}

	// Empty impl renders to the empty string.
	if got := ReflectAsString(&MockAttribute{}, true); got != "" {
		t.Fatalf("ReflectAsString on non-Reflectable impl: %q, want empty", got)
	}
}
