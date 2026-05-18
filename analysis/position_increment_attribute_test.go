// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
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
	var _ util.Attribute = NewPositionIncrementAttribute()
	var _ util.AttributeImpl = NewPositionIncrementAttribute()
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

// TestPositionIncrementAttribute_SetNegative_Panics verifies that the
// Lucene-faithful validation rejects negative values with a panic
// (mirroring IllegalArgumentException).
func TestPositionIncrementAttribute_SetNegative_Panics(t *testing.T) {
	attr := NewPositionIncrementAttribute()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("SetPositionIncrement(-1) did not panic")
		}
	}()
	attr.SetPositionIncrement(-1)
}

// TestPositionIncrementAttribute_End verifies that End sets the
// increment to 0, distinct from Clear which resets to 1 (Lucene
// reference: PositionIncrementutil.AttributeImpl#end).
func TestPositionIncrementAttribute_End(t *testing.T) {
	attr := NewPositionIncrementAttribute().(*positionIncrementAttribute)
	attr.SetPositionIncrement(4)
	attr.End()
	if got := attr.GetPositionIncrement(); got != 0 {
		t.Fatalf("End: positionIncrement=%d, want 0", got)
	}
}

// TestPositionIncrementAttribute_ReflectWith verifies the single
// (PositionIncrementAttribute, "positionIncrement", value) triple
// expected by the Lucene reference.
func TestPositionIncrementAttribute_ReflectWith(t *testing.T) {
	attr := NewPositionIncrementAttribute().(*positionIncrementAttribute)
	attr.SetPositionIncrement(2)

	var got []struct {
		k string
		v int
		t reflect.Type
	}
	attr.ReflectWith(func(attType reflect.Type, key string, value any) {
		got = append(got, struct {
			k string
			v int
			t reflect.Type
		}{key, value.(int), attType})
	})

	if len(got) != 1 {
		t.Fatalf("emitted %d triples, want 1", len(got))
	}
	wantType := reflect.TypeOf((*PositionIncrementAttribute)(nil)).Elem()
	if got[0].k != "positionIncrement" || got[0].v != 2 || got[0].t != wantType {
		t.Fatalf("triple=%+v, want {positionIncrement 2 %v}", got[0], wantType)
	}
}

// TestPositionIncrementAttribute_EqualsHashCode verifies the
// equals/hashCode contract: equal positionIncrement => equal, hash
// equals positionIncrement.
func TestPositionIncrementAttribute_EqualsHashCode(t *testing.T) {
	a := NewPositionIncrementAttribute().(*positionIncrementAttribute)
	b := NewPositionIncrementAttribute().(*positionIncrementAttribute)
	a.SetPositionIncrement(3)
	b.SetPositionIncrement(3)

	if !a.Equals(b) {
		t.Fatal("equal increments not equal")
	}
	if a.HashCode() != 3 {
		t.Fatalf("HashCode=%d, want 3", a.HashCode())
	}

	b.SetPositionIncrement(4)
	if a.Equals(b) {
		t.Fatal("differing increments compared equal")
	}

	if a.Equals(&MockAttribute{}) {
		t.Fatal("Equals against unrelated type returned true")
	}
}
