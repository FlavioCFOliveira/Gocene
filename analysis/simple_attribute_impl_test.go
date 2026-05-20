// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/analysis/tokenattributes/TestSimpleAttributeImpl.java
//
// Deviation: the Java test uses TestUtil.assertAttributeReflection, a
// reflection-based helper from LuceneTestCase. In Go the defaults are
// verified directly against each attribute impl's getter methods.

package analysis

import "testing"

// TestSimpleAttributeImpl_Attributes mirrors testAttributes (Lucene 10.4.0).
// It verifies the out-of-the-box default values of every simple attribute
// implementation, matching the Java assertAttributeReflection assertions.
func TestSimpleAttributeImpl_Attributes(t *testing.T) {
	t.Run("PositionIncrementAttributeImpl", func(t *testing.T) {
		a := NewPositionIncrementAttributeImpl()
		if got := a.GetPositionIncrement(); got != 1 {
			t.Errorf("default positionIncrement: got %d, want 1", got)
		}
	})

	t.Run("PositionLengthAttributeImpl", func(t *testing.T) {
		a := NewPositionLengthAttributeImpl()
		if got := a.GetPositionLength(); got != 1 {
			t.Errorf("default positionLength: got %d, want 1", got)
		}
	})

	t.Run("FlagsAttributeImpl", func(t *testing.T) {
		a := NewFlagsAttributeImpl()
		if got := a.GetFlags(); got != 0 {
			t.Errorf("default flags: got %d, want 0", got)
		}
	})

	t.Run("TypeAttributeImpl", func(t *testing.T) {
		a := NewTypeAttributeImpl()
		if got := a.GetType(); got != DefaultTypeAttributeValue {
			t.Errorf("default type: got %q, want %q", got, DefaultTypeAttributeValue)
		}
	})

	t.Run("PayloadAttributeImpl", func(t *testing.T) {
		a := NewPayloadAttributeImpl()
		if got := a.GetPayload(); got != nil {
			t.Errorf("default payload: got %v, want nil", got)
		}
	})

	t.Run("KeywordAttributeImpl", func(t *testing.T) {
		a := NewKeywordAttributeImpl()
		if got := a.IsKeywordToken(); got {
			t.Errorf("default keyword: got true, want false")
		}
	})

	t.Run("OffsetAttributeImpl", func(t *testing.T) {
		a := NewOffsetAttributeImpl()
		if got := a.StartOffset(); got != 0 {
			t.Errorf("default startOffset: got %d, want 0", got)
		}
		if got := a.EndOffset(); got != 0 {
			t.Errorf("default endOffset: got %d, want 0", got)
		}
	})
}
