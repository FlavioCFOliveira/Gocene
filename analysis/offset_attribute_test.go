// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"testing"
)

// TestOffsetAttribute_Basic tests basic OffsetAttribute operations.
// Source: TestOffsetAttribute.java
// Purpose: Tests that offset attributes can be set and retrieved.
func TestOffsetAttribute_Basic(t *testing.T) {
	attr := NewOffsetAttribute()

	// Test initial values
	if attr.StartOffset() != 0 {
		t.Errorf("Initial start offset should be 0, got %d", attr.StartOffset())
	}
	if attr.EndOffset() != 0 {
		t.Errorf("Initial end offset should be 0, got %d", attr.EndOffset())
	}

	// Test SetOffset
	attr.SetStartOffset(10)
	attr.SetEndOffset(15)
	if attr.StartOffset() != 10 {
		t.Errorf("Start offset should be 10, got %d", attr.StartOffset())
	}
	if attr.EndOffset() != 15 {
		t.Errorf("End offset should be 15, got %d", attr.EndOffset())
	}
}

// TestOffsetAttribute_Clear tests clearing the offset attribute.
// Source: TestOffsetAttribute.java
// Purpose: Tests that offsets are reset when cleared.
func TestOffsetAttribute_Clear(t *testing.T) {
	attr := NewOffsetAttribute()
	attr.SetStartOffset(10)
	attr.SetEndOffset(15)
	attr.Clear()

	if attr.StartOffset() != 0 {
		t.Errorf("After Clear(), start offset should be 0, got %d", attr.StartOffset())
	}
	if attr.EndOffset() != 0 {
		t.Errorf("After Clear(), end offset should be 0, got %d", attr.EndOffset())
	}
}

// TestOffsetAttribute_CopyTo tests copying to another attribute.
// Source: TestOffsetAttribute.java
// Purpose: Tests that offsets can be copied to another instance.
func TestOffsetAttribute_CopyTo(t *testing.T) {
	source := NewOffsetAttribute()
	source.SetStartOffset(5)
	source.SetEndOffset(10)

	target := NewOffsetAttribute()
	source.CopyTo(target)

	if target.StartOffset() != 5 {
		t.Errorf("Target start offset should be 5, got %d", target.StartOffset())
	}
	if target.EndOffset() != 10 {
		t.Errorf("Target end offset should be 10, got %d", target.EndOffset())
	}
}

// TestOffsetAttribute_Copy tests creating a copy.
// Source: TestOffsetAttribute.java
// Purpose: Tests that a deep copy can be created.
func TestOffsetAttribute_Copy(t *testing.T) {
	original := NewOffsetAttribute()
	original.SetStartOffset(20)
	original.SetEndOffset(25)

	copy := original.Copy()
	if copy == nil {
		t.Fatal("Copy() should return a non-nil attribute")
	}

	if offsetAttr, ok := copy.(OffsetAttribute); ok {
		if offsetAttr.StartOffset() != 20 {
			t.Errorf("Copy start offset should be 20, got %d", offsetAttr.StartOffset())
		}
		if offsetAttr.EndOffset() != 25 {
			t.Errorf("Copy end offset should be 25, got %d", offsetAttr.EndOffset())
		}
	} else {
		t.Error("Copy() should return *OffsetAttribute")
	}
}

// TestOffsetAttribute_Interface tests interface compliance.
func TestOffsetAttribute_Interface(t *testing.T) {
	var _ Attribute = NewOffsetAttribute()
	var _ AttributeImpl = NewOffsetAttribute()
}
