// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"testing"
)

// TestSentenceAttributeImpl_Default verifies the default sentence
// index is 0 (matching the no-arg Lucene constructor).
func TestSentenceAttributeImpl_Default(t *testing.T) {
	impl := NewSentenceAttributeImpl()
	if got := impl.GetSentenceIndex(); got != 0 {
		t.Fatalf("default index=%d, want 0", got)
	}
}

// TestSentenceAttributeImpl_SetGet verifies the set/get round-trip.
func TestSentenceAttributeImpl_SetGet(t *testing.T) {
	impl := NewSentenceAttributeImpl()
	impl.SetSentenceIndex(42)
	if got := impl.GetSentenceIndex(); got != 42 {
		t.Fatalf("after set: index=%d, want 42", got)
	}
}

// TestSentenceAttributeImpl_Clear verifies Clear resets to 0.
func TestSentenceAttributeImpl_Clear(t *testing.T) {
	impl := NewSentenceAttributeImpl()
	impl.SetSentenceIndex(7)
	impl.Clear()
	if got := impl.GetSentenceIndex(); got != 0 {
		t.Fatalf("after clear: index=%d, want 0", got)
	}
}

// TestSentenceAttributeImpl_CopyTo verifies the deep-copy contract
// (assertCopyIsEqual analogue from Lucene's
// LuceneTestCase#assertCopyIsEqual).
func TestSentenceAttributeImpl_CopyTo(t *testing.T) {
	src := NewSentenceAttributeImpl()
	src.SetSentenceIndex(11)
	dst := NewSentenceAttributeImpl()
	src.CopyTo(dst)

	if !src.Equals(dst) {
		t.Fatalf("copy not equal: src=%d dst=%d", src.GetSentenceIndex(), dst.GetSentenceIndex())
	}
	if src.HashCode() != dst.HashCode() {
		t.Fatalf("hash mismatch: %d vs %d", src.HashCode(), dst.HashCode())
	}
}

// TestSentenceAttributeImpl_CopyTo_WrongTarget verifies the cast
// contract: an unrelated target panics with an explanatory message.
func TestSentenceAttributeImpl_CopyTo_WrongTarget(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("CopyTo with wrong target type did not panic")
		}
	}()
	NewSentenceAttributeImpl().CopyTo(&MockAttribute{})
}

// TestSentenceAttributeImpl_Copy verifies Copy returns an independent
// clone (mutation of the clone does not affect the source).
func TestSentenceAttributeImpl_Copy(t *testing.T) {
	src := NewSentenceAttributeImpl()
	src.SetSentenceIndex(5)
	clone := src.Copy().(*SentenceAttributeImpl)
	clone.SetSentenceIndex(99)
	if src.GetSentenceIndex() != 5 {
		t.Fatalf("Copy not independent: src=%d, want 5", src.GetSentenceIndex())
	}
}

// TestSentenceAttributeImpl_Equals_NotSentence verifies that an
// unrelated type compares unequal, matching Lucene's instance-of
// guard.
func TestSentenceAttributeImpl_Equals_NotSentence(t *testing.T) {
	impl := NewSentenceAttributeImpl()
	if impl.Equals(&MockAttribute{}) {
		t.Fatal("Equals against unrelated type returned true")
	}
}

// TestSentenceAttributeImpl_ReflectWith verifies the single triple
// emitted matches the Lucene reflectWith output for this attribute
// (note the plural "sentences" key, preserved from the reference).
func TestSentenceAttributeImpl_ReflectWith(t *testing.T) {
	impl := NewSentenceAttributeImpl()
	impl.SetSentenceIndex(3)

	var got []struct {
		t reflect.Type
		k string
		v any
	}
	impl.ReflectWith(func(attType reflect.Type, key string, value any) {
		got = append(got, struct {
			t reflect.Type
			k string
			v any
		}{attType, key, value})
	})

	if len(got) != 1 {
		t.Fatalf("emitted %d triples, want 1", len(got))
	}
	wantType := reflect.TypeOf((*SentenceAttribute)(nil)).Elem()
	if got[0].t != wantType || got[0].k != "sentences" || got[0].v != 3 {
		t.Fatalf("triple=%+v, want {%v sentences 3}", got[0], wantType)
	}
}
