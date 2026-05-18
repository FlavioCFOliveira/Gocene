// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
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
	var _ util.Attribute = NewOffsetAttribute()
	var _ util.AttributeImpl = NewOffsetAttribute()
}

// TestOffsetAttribute_SetOffset_Validation verifies the Lucene-faithful
// combined setter rejects illegal inputs with a panic, matching the
// IllegalArgumentException thrown by OffsetAttributeImpl#setOffset.
func TestOffsetAttribute_SetOffset_Validation(t *testing.T) {
	cases := []struct {
		name       string
		start, end int
		wantPanic  bool
	}{
		{"valid_zero", 0, 0, false},
		{"valid", 3, 5, false},
		{"start_equals_end", 4, 4, false},
		{"negative_start", -1, 5, true},
		{"end_before_start", 5, 3, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			attr := NewOffsetAttribute()
			defer func() {
				r := recover()
				if (r != nil) != tc.wantPanic {
					t.Fatalf("panic=%v, want panic=%v", r, tc.wantPanic)
				}
			}()
			attr.SetOffset(tc.start, tc.end)
			if !tc.wantPanic {
				if attr.StartOffset() != tc.start || attr.EndOffset() != tc.end {
					t.Fatalf("start=%d end=%d, want %d/%d",
						attr.StartOffset(), attr.EndOffset(), tc.start, tc.end)
				}
			}
		})
	}
}

// TestOffsetAttribute_ReflectWith verifies that the two parity triples
// expected by the Lucene reference (startOffset and endOffset under
// the OffsetAttribute key) are emitted in order.
func TestOffsetAttribute_ReflectWith(t *testing.T) {
	attr := NewOffsetAttribute().(*offsetAttribute)
	attr.SetOffset(3, 7)

	var keys []string
	var values []int
	attr.ReflectWith(func(_ reflect.Type, key string, value any) {
		keys = append(keys, key)
		values = append(values, value.(int))
	})
	if len(keys) != 2 || keys[0] != "startOffset" || keys[1] != "endOffset" {
		t.Fatalf("emitted keys=%v, want [startOffset endOffset]", keys)
	}
	if values[0] != 3 || values[1] != 7 {
		t.Fatalf("emitted values=%v, want [3 7]", values)
	}
}

// TestOffsetAttribute_EqualsHashCode verifies the equals/hashCode
// contract: identical offsets => equal & same hash; differing offsets
// => not equal.
func TestOffsetAttribute_EqualsHashCode(t *testing.T) {
	a := NewOffsetAttribute().(*offsetAttribute)
	b := NewOffsetAttribute().(*offsetAttribute)
	a.SetOffset(2, 9)
	b.SetOffset(2, 9)

	if !a.Equals(b) {
		t.Fatal("equal offsets not equal")
	}
	if a.HashCode() != b.HashCode() {
		t.Fatalf("hash mismatch: %d vs %d", a.HashCode(), b.HashCode())
	}

	b.SetOffset(2, 10)
	if a.Equals(b) {
		t.Fatal("differing offsets compared equal")
	}

	if a.Equals(&MockAttribute{}) {
		t.Fatal("Equals against unrelated type returned true")
	}
}
