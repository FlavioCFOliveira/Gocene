// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"testing"
)

// reflectorCapture collects every triple emitted by an AttributeReflectable
// for use by the parity assertions below.
type reflectorCapture struct {
	t reflect.Type
	k string
	v any
}

func captureReflect(r AttributeReflectable) []reflectorCapture {
	var got []reflectorCapture
	r.ReflectWith(func(attType reflect.Type, key string, value any) {
		got = append(got, reflectorCapture{attType, key, value})
	})
	return got
}

// TestTypeAttribute_Parity exercises the Sprint 12 additions for
// TypeAttribute: ReflectWith emits a single "type" triple, Equals
// matches on string content and HashCode uses the Java string-hash
// formula (h*31+ch). Sprint 54 Phase 3 promoted TypeAttribute to an
// interface+impl pair so Equals/HashCode are reached through the
// concrete *typeAttributeImpl receiver.
func TestTypeAttribute_Parity(t *testing.T) {
	a := NewTypeAttribute().(*typeAttributeImpl)
	a.SetType("foo")
	b := NewTypeAttribute().(*typeAttributeImpl)
	b.SetType("foo")

	if !a.Equals(b) {
		t.Fatal("equal types not equal")
	}
	if a.HashCode() != b.HashCode() {
		t.Fatalf("hash mismatch: %d vs %d", a.HashCode(), b.HashCode())
	}

	b.SetType("bar")
	if a.Equals(b) {
		t.Fatal("differing types compared equal")
	}
	if a.Equals(&MockAttribute{}) {
		t.Fatal("Equals against unrelated type returned true")
	}

	got := captureReflect(a)
	if len(got) != 1 || got[0].k != "type" || got[0].v != "foo" {
		t.Fatalf("ReflectWith=%v, want single type=foo triple", got)
	}
}

// TestPayloadAttribute_Parity covers nil vs populated equality (Lucene
// treats two nil payloads as equal) and the Java-style byte-array hash
// for non-nil values.
func TestPayloadAttribute_Parity(t *testing.T) {
	a := NewPayloadAttribute().(*payloadAttributeImpl)
	b := NewPayloadAttribute().(*payloadAttributeImpl)
	if !a.Equals(b) {
		t.Fatal("two empty payloads not equal")
	}
	if a.HashCode() != 0 || b.HashCode() != 0 {
		t.Fatalf("empty hash codes: a=%d b=%d, want 0/0", a.HashCode(), b.HashCode())
	}

	a.SetPayload([]byte{1, 2, 3})
	b.SetPayload([]byte{1, 2, 3})
	if !a.Equals(b) {
		t.Fatal("equal payloads not equal")
	}
	if a.HashCode() != b.HashCode() {
		t.Fatalf("hash mismatch: %d vs %d", a.HashCode(), b.HashCode())
	}

	b.SetPayload([]byte{1, 2, 4})
	if a.Equals(b) {
		t.Fatal("differing payloads compared equal")
	}

	got := captureReflect(a)
	if len(got) != 1 || got[0].k != "payload" {
		t.Fatalf("ReflectWith=%v, want single payload triple", got)
	}
}

// TestFlagsAttribute_Parity covers the Lucene flags hash (hash == flags).
func TestFlagsAttribute_Parity(t *testing.T) {
	a := NewFlagsAttribute().(*flagsAttributeImpl)
	a.SetFlags(0xABCD)
	b := NewFlagsAttribute().(*flagsAttributeImpl)
	b.SetFlags(0xABCD)

	if !a.Equals(b) {
		t.Fatal("equal flags not equal")
	}
	if a.HashCode() != 0xABCD {
		t.Fatalf("HashCode=%d, want %d", a.HashCode(), 0xABCD)
	}

	b.SetFlags(0)
	if a.Equals(b) {
		t.Fatal("differing flags compared equal")
	}

	got := captureReflect(a)
	if len(got) != 1 || got[0].k != "flags" || got[0].v != 0xABCD {
		t.Fatalf("ReflectWith=%v, want flags=0xABCD triple", got)
	}
}

// TestKeywordAttribute_Parity covers the Lucene hash {31 if keyword
// else 37}.
func TestKeywordAttribute_Parity(t *testing.T) {
	on := NewKeywordAttributeWithValue(true).(*keywordAttributeImpl)
	off := NewKeywordAttributeWithValue(false).(*keywordAttributeImpl)
	if on.HashCode() != 31 {
		t.Fatalf("HashCode(true)=%d, want 31", on.HashCode())
	}
	if off.HashCode() != 37 {
		t.Fatalf("HashCode(false)=%d, want 37", off.HashCode())
	}
	if on.Equals(off) {
		t.Fatal("on/off compared equal")
	}

	got := captureReflect(on)
	if len(got) != 1 || got[0].k != "keyword" || got[0].v != true {
		t.Fatalf("ReflectWith=%v, want keyword=true triple", got)
	}
}

// TestPositionLengthAttribute_Parity covers Equals/HashCode and the
// validated setter that panics on length < 1.
func TestPositionLengthAttribute_Parity(t *testing.T) {
	a := NewPositionLengthAttribute().(*positionLengthAttributeImpl)
	a.SetPositionLengthValidated(5)
	if a.HashCode() != 5 {
		t.Fatalf("HashCode=%d, want 5", a.HashCode())
	}

	b := NewPositionLengthAttribute().(*positionLengthAttributeImpl)
	b.SetPositionLengthValidated(5)
	if !a.Equals(b) {
		t.Fatal("equal positionLength not equal")
	}

	got := captureReflect(a)
	if len(got) != 1 || got[0].k != "positionLength" || got[0].v != 5 {
		t.Fatalf("ReflectWith=%v, want positionLength=5 triple", got)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("SetPositionLengthValidated(0) did not panic")
		}
	}()
	NewPositionLengthAttribute().SetPositionLengthValidated(0)
}

// TestTermFrequencyAttribute_Parity covers Equals/HashCode, the
// validated setter (panic on freq < 1) and the End hook (which mirrors
// Clear by resetting to 1).
func TestTermFrequencyAttribute_Parity(t *testing.T) {
	a := NewTermFrequencyAttribute().(*termFrequencyAttributeImpl)
	a.SetTermFrequencyValidated(9)
	if a.HashCode() != 9 {
		t.Fatalf("HashCode=%d, want 9", a.HashCode())
	}

	b := NewTermFrequencyAttribute().(*termFrequencyAttributeImpl)
	b.SetTermFrequencyValidated(9)
	if !a.Equals(b) {
		t.Fatal("equal termFrequency not equal")
	}

	got := captureReflect(a)
	if len(got) != 1 || got[0].k != "termFrequency" || got[0].v != 9 {
		t.Fatalf("ReflectWith=%v, want termFrequency=9 triple", got)
	}

	// End resets to 1 (parity with Lucene's end() override).
	a.End()
	if a.GetTermFrequency() != 1 {
		t.Fatalf("End: termFrequency=%d, want 1", a.GetTermFrequency())
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("SetTermFrequencyValidated(0) did not panic")
		}
	}()
	NewTermFrequencyAttribute().SetTermFrequencyValidated(0)
}

// TestBareStructAttributes_AttributeImplCompliance verifies that the
// six interface+impl attribute pairs (post-Sprint 54 Phase 3) satisfy
// [AttributeImpl] and opt into [AttributeReflectable]. End() is exposed
// via [AttributeEnder] only for TermFrequencyAttribute (matches Lucene).
//
// The test name is preserved for git-history continuity even though
// the underlying impls are no longer bare structs.
func TestBareStructAttributes_AttributeImplCompliance(t *testing.T) {
	var (
		_ AttributeImpl        = (*typeAttributeImpl)(nil)
		_ AttributeImpl        = (*payloadAttributeImpl)(nil)
		_ AttributeImpl        = (*flagsAttributeImpl)(nil)
		_ AttributeImpl        = (*keywordAttributeImpl)(nil)
		_ AttributeImpl        = (*positionLengthAttributeImpl)(nil)
		_ AttributeImpl        = (*termFrequencyAttributeImpl)(nil)
		_ AttributeReflectable = (*typeAttributeImpl)(nil)
		_ AttributeReflectable = (*payloadAttributeImpl)(nil)
		_ AttributeReflectable = (*flagsAttributeImpl)(nil)
		_ AttributeReflectable = (*keywordAttributeImpl)(nil)
		_ AttributeReflectable = (*positionLengthAttributeImpl)(nil)
		_ AttributeReflectable = (*termFrequencyAttributeImpl)(nil)
		_ AttributeEnder       = (*termFrequencyAttributeImpl)(nil)
	)
}
