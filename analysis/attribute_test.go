// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"testing"
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
