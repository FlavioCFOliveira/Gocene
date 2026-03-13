// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"testing"
)

// TestAttributeSource_AddAttribute tests adding attributes.
// Source: TestAttributeSource.testAddAttribute()
// Purpose: Tests attribute addition to source.
func TestAttributeSource_AddAttribute(t *testing.T) {
	as := NewAttributeSource()

	termAttr := NewCharTermAttribute()
	as.AddAttribute(termAttr)

	retrieved := as.GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
	if retrieved == nil {
		t.Error("Expected to retrieve added attribute")
	}

	if retrieved != termAttr {
		t.Error("Retrieved attribute should be the same instance")
	}
}

// TestAttributeSource_AddNullAttribute tests adding nil attribute.
// Source: TestAttributeSource.testAddNullAttribute()
// Purpose: Tests handling of nil attribute addition.
func TestAttributeSource_AddNullAttribute(t *testing.T) {
	as := NewAttributeSource()

	as.AddAttribute(nil)

	classes := as.GetAttributeClasses()
	if len(classes) != 0 {
		t.Errorf("Expected 0 attributes after adding nil, got %d", len(classes))
	}
}

// TestAttributeSource_GetAttribute tests retrieving attributes.
// Source: TestAttributeSource.testGetAttribute()
// Purpose: Tests attribute retrieval by type.
func TestAttributeSource_GetAttribute(t *testing.T) {
	as := NewAttributeSource()

	termAttr := NewCharTermAttribute()
	termAttr.SetValue("test")
	as.AddAttribute(termAttr)

	retrieved := as.GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
	if retrieved == nil {
		t.Fatal("Expected to retrieve attribute")
	}

	if termAttr, ok := retrieved.(CharTermAttribute); ok {
		if termAttr.String() != "test" {
			t.Errorf("Expected 'test', got '%s'", termAttr.String())
		}
	} else {
		t.Error("Retrieved attribute is not a CharTermAttribute")
	}
}

// TestAttributeSource_GetAttributeByName tests retrieving by name.
// Source: TestAttributeSource.testGetAttributeByName()
// Purpose: Tests attribute retrieval by name string.
func TestAttributeSource_GetAttributeByName(t *testing.T) {
	as := NewAttributeSource()

	termAttr := NewCharTermAttribute()
	as.AddAttribute(termAttr)

	retrieved := as.GetAttribute("CharTermAttribute")
	if retrieved == nil {
		t.Error("Expected to retrieve attribute by name")
	}
}

// TestAttributeSource_GetNonExistentAttribute tests non-existent attribute.
// Source: TestAttributeSource.testGetNonExistent()
// Purpose: Tests handling of non-existent attributes.
func TestAttributeSource_GetNonExistentAttribute(t *testing.T) {
	as := NewAttributeSource()

	retrieved := as.GetAttribute("NonExistent")
	if retrieved != nil {
		t.Error("Expected nil for non-existent attribute")
	}

	retrievedByType := as.GetAttributeByType(reflect.TypeOf(&offsetAttribute{}))
	if retrievedByType != nil {
		t.Error("Expected nil for non-existent attribute type")
	}
}

// TestAttributeSource_HasAttribute tests attribute existence check.
// Source: TestAttributeSource.testHasAttribute()
// Purpose: Tests attribute existence checking.
func TestAttributeSource_HasAttribute(t *testing.T) {
	as := NewAttributeSource()

	termAttr := NewCharTermAttribute()
	as.AddAttribute(termAttr)

	if !as.HasAttribute(reflect.TypeOf(&charTermAttribute{})) {
		t.Error("Expected HasAttribute to return true for existing attribute")
	}

	if as.HasAttribute(reflect.TypeOf(&offsetAttribute{})) {
		t.Error("Expected HasAttribute to return false for non-existent attribute")
	}
}

// TestAttributeSource_ClearAttributes tests attribute clearing.
// Source: TestAttributeSource.testClearAttributes()
// Purpose: Tests clearing all attributes.
func TestAttributeSource_ClearAttributes(t *testing.T) {
	as := NewAttributeSource()

	termAttr := NewCharTermAttribute()
	termAttr.SetValue("test")
	as.AddAttribute(termAttr)

	as.ClearAttributes()

	if termAttr.String() != "" {
		t.Errorf("Expected cleared attribute, got '%s'", termAttr.String())
	}
}

// TestAttributeSource_RemoveAttribute tests attribute removal.
// Source: TestAttributeSource.testRemoveAttribute()
// Purpose: Tests removing attributes from source.
func TestAttributeSource_RemoveAttribute(t *testing.T) {
	as := NewAttributeSource()

	termAttr := NewCharTermAttribute()
	as.AddAttribute(termAttr)

	if !as.HasAttribute(reflect.TypeOf(&charTermAttribute{})) {
		t.Error("Attribute should exist before removal")
	}

	as.RemoveAttribute(reflect.TypeOf(&charTermAttribute{}))

	if as.HasAttribute(reflect.TypeOf(&charTermAttribute{})) {
		t.Error("Attribute should not exist after removal")
	}
}

// TestAttributeSource_CaptureState tests state capture.
// Source: TestAttributeSource.testCaptureState()
// Purpose: Tests capturing attribute state.
func TestAttributeSource_CaptureState(t *testing.T) {
	as := NewAttributeSource()

	termAttr := NewCharTermAttribute()
	termAttr.SetValue("captured")
	as.AddAttribute(termAttr)

	state := as.CaptureState()
	if state == nil {
		t.Fatal("Expected non-nil state")
	}

	termAttr.SetValue("modified")

	as.RestoreState(state)

	if termAttr.String() != "captured" {
		t.Errorf("Expected 'captured' after restore, got '%s'", termAttr.String())
	}
}

// TestAttributeSource_RestoreNullState tests restoring null state.
// Source: TestAttributeSource.testRestoreNullState()
// Purpose: Tests handling of nil state.
func TestAttributeSource_RestoreNullState(t *testing.T) {
	as := NewAttributeSource()

	termAttr := NewCharTermAttribute()
	termAttr.SetValue("test")
	as.AddAttribute(termAttr)

	as.RestoreState(nil)

	if termAttr.String() != "test" {
		t.Errorf("Expected 'test' after nil restore, got '%s'", termAttr.String())
	}
}

// TestAttributeSource_MultipleAttributes tests multiple attributes.
// Source: TestAttributeSource.testMultipleAttributes()
// Purpose: Tests handling of multiple attribute types.
func TestAttributeSource_MultipleAttributes(t *testing.T) {
	as := NewAttributeSource()

	termAttr := NewCharTermAttribute()
	offsetAttr := NewOffsetAttribute()
	posIncrAttr := NewPositionIncrementAttribute()

	as.AddAttribute(termAttr)
	as.AddAttribute(offsetAttr)
	as.AddAttribute(posIncrAttr)

	if !as.HasAttribute(reflect.TypeOf(&charTermAttribute{})) {
		t.Error("CharTermAttribute should exist")
	}
	if !as.HasAttribute(reflect.TypeOf(&offsetAttribute{})) {
		t.Error("OffsetAttribute should exist")
	}
	if !as.HasAttribute(reflect.TypeOf(&positionIncrementAttribute{})) {
		t.Error("PositionIncrementAttribute should exist")
	}

	classes := as.GetAttributeClasses()
	if len(classes) != 3 {
		t.Errorf("Expected 3 attribute classes, got %d", len(classes))
	}
}

// TestAttributeSource_Clone tests cloning.
// Source: TestAttributeSource.testClone()
// Purpose: Tests attribute source cloning.
func TestAttributeSource_Clone(t *testing.T) {
	original := NewAttributeSource()

	termAttr := NewCharTermAttribute()
	termAttr.SetValue("original")
	original.AddAttribute(termAttr)

	clone := original.Clone()
	if clone == nil {
		t.Fatal("Expected non-nil clone")
	}

	retrieved := clone.GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
	if retrieved == nil {
		t.Error("Clone should have the attribute")
	}

	termAttr.SetValue("modified")

	if termAttr.String() != "modified" {
		t.Error("Original should be modified")
	}
}

// TestAttributeSource_Factory tests factory registration.
// Source: TestAttributeSource.testFactory()
// Purpose: Tests attribute factory registration.
func TestAttributeSource_Factory(t *testing.T) {
	as := NewAttributeSource()

	factoryCalled := false
	as.RegisterFactory(reflect.TypeOf(&charTermAttribute{}), func() AttributeImpl {
		factoryCalled = true
		return NewCharTermAttribute()
	})

	attr := as.GetOrCreateAttribute(reflect.TypeOf(&charTermAttribute{}))
	if attr == nil {
		t.Error("Expected attribute from factory")
	}
	if !factoryCalled {
		t.Error("Factory should have been called")
	}

	factoryCalled = false
	attr2 := as.GetOrCreateAttribute(reflect.TypeOf(&charTermAttribute{}))
	if attr2 != attr {
		t.Error("Should return same instance")
	}
	if factoryCalled {
		t.Error("Factory should not be called for existing attribute")
	}
}

// TestAttributeSource_ConcurrentAccess tests concurrent access.
// Source: TestAttributeSource.testConcurrentAccess()
// Purpose: Tests thread safety of attribute source.
func TestAttributeSource_ConcurrentAccess(t *testing.T) {
	as := NewAttributeSource()

	termAttr := NewCharTermAttribute()
	as.AddAttribute(termAttr)

	done := make(chan bool, 10)

	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				as.HasAttribute(reflect.TypeOf(&charTermAttribute{}))
				as.GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
			}
			done <- true
		}(i)
	}

	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				attr := NewCharTermAttribute()
				as.AddAttribute(attr)
			}
			done <- true
		}(i + 5)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestAttributeSource_CopyTo tests attribute copying.
// Source: TestAttributeSource.testCopyTo()
// Purpose: Tests copying attributes to target.
func TestAttributeSource_CopyTo(t *testing.T) {
	source := NewAttributeSource()
	target := NewAttributeSource()

	termAttr := NewCharTermAttribute()
	termAttr.SetValue("source")
	source.AddAttribute(termAttr)

	targetAttr := NewCharTermAttribute()
	target.AddAttribute(targetAttr)

	state := source.CaptureState()
	target.RestoreState(state)

	if targetAttr.String() != "source" {
		t.Errorf("Expected 'source' in target, got '%s'", targetAttr.String())
	}
}

// TestAttributeSource_GetAttributeClasses tests getting all classes.
// Source: TestAttributeSource.testGetAttributeClasses()
// Purpose: Tests retrieval of all attribute classes.
func TestAttributeSource_GetAttributeClasses(t *testing.T) {
	as := NewAttributeSource()

	classes := as.GetAttributeClasses()
	if len(classes) != 0 {
		t.Errorf("Expected 0 classes initially, got %d", len(classes))
	}

	as.AddAttribute(NewCharTermAttribute())
	as.AddAttribute(NewOffsetAttribute())

	classes = as.GetAttributeClasses()
	if len(classes) != 2 {
		t.Errorf("Expected 2 classes, got %d", len(classes))
	}
}
