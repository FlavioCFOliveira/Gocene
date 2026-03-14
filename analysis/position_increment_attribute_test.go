// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"testing"
)

// TestPositionIncrementAttribute_Basic tests basic PositionIncrementAttribute operations.
// Source: TestPositionIncrementAttribute.java
// Purpose: Tests that position increment can be set and retrieved.
func TestPositionIncrementAttribute_Basic(t *testing.T) {
	attr := NewPositionIncrementAttribute()

	// Test initial value
	if attr.GetPositionIncrement() != 1 {
		t.Errorf("Initial position increment should be 1, got %d", attr.GetPositionIncrement())
	}

	// Test SetPositionIncrement
	attr.SetPositionIncrement(5)
	if attr.GetPositionIncrement() != 5 {
		t.Errorf("Position increment should be 5, got %d", attr.GetPositionIncrement())
	}
}

// TestPositionIncrementAttribute_Clear tests clearing the position increment attribute.
// Source: TestPositionIncrementAttribute.java
// Purpose: Tests that position increment is reset when cleared.
func TestPositionIncrementAttribute_Clear(t *testing.T) {
	attr := NewPositionIncrementAttribute()
	attr.SetPositionIncrement(10)
	attr.Clear()

	if attr.GetPositionIncrement() != 1 {
		t.Errorf("After Clear(), position increment should be 1, got %d", attr.GetPositionIncrement())
	}
}

// TestPositionIncrementAttribute_CopyTo tests copying to another attribute.
// Source: TestPositionIncrementAttribute.java
// Purpose: Tests that position increment can be copied to another instance.
func TestPositionIncrementAttribute_CopyTo(t *testing.T) {
	source := NewPositionIncrementAttribute()
	source.SetPositionIncrement(3)

	target := NewPositionIncrementAttribute()
	source.CopyTo(target)

	if target.GetPositionIncrement() != 3 {
		t.Errorf("Target position increment should be 3, got %d", target.GetPositionIncrement())
	}
}

// TestPositionIncrementAttribute_Copy tests creating a copy.
// Source: TestPositionIncrementAttribute.java
// Purpose: Tests that a deep copy can be created.
func TestPositionIncrementAttribute_Copy(t *testing.T) {
	original := NewPositionIncrementAttribute()
	original.SetPositionIncrement(7)

	copy := original.Copy()
	if copy == nil {
		t.Fatal("Copy() should return a non-nil attribute")
	}

	if posAttr, ok := copy.(PositionIncrementAttribute); ok {
		if posAttr.GetPositionIncrement() != 7 {
			t.Errorf("Copy position increment should be 7, got %d", posAttr.GetPositionIncrement())
		}
	} else {
		t.Error("Copy() should return *PositionIncrementAttribute")
	}
}

// TestPositionIncrementAttribute_Interface tests interface compliance.
func TestPositionIncrementAttribute_Interface(t *testing.T) {
	var _ Attribute = NewPositionIncrementAttribute()
	var _ AttributeImpl = NewPositionIncrementAttribute()
}

// TestPositionIncrementAttribute_Zero tests zero position increment.
// This is valid in Lucene for overlapping tokens.
func TestPositionIncrementAttribute_Zero(t *testing.T) {
	attr := NewPositionIncrementAttribute()
	attr.SetPositionIncrement(0)

	if attr.GetPositionIncrement() != 0 {
		t.Errorf("Position increment should be 0, got %d", attr.GetPositionIncrement())
	}
}
