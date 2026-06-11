// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package intervals

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Term source identity
// ---------------------------------------------------------------------------

func TestTermSource_Equals(t *testing.T) {
	a := Term("hello")
	b := Term("hello")
	c := Term("world")
	if !a.Equals(b) {
		t.Error("Equal terms should compare equal")
	}
	if a.Equals(c) {
		t.Error("Different terms should not compare equal")
	}
}

// ---------------------------------------------------------------------------
// Phrase
// ---------------------------------------------------------------------------

func TestPhrase_MultiTerm(t *testing.T) {
	result := Phrase("one", "two", "three")
	single := Term("one")
	if result.Equals(single) {
		t.Error("Phrase with multiple terms should not equal single term")
	}
}

// ---------------------------------------------------------------------------
// Unordered wrapping
// ---------------------------------------------------------------------------

func TestUnordered_SingleMatchesIdentity(t *testing.T) {
	result := Unordered(NoIntervals("none"))
	single := NoIntervals("none")
	if !result.Equals(single) {
		t.Errorf("Unordered(noIntervals) = %s; want %s", result, single)
	}
}

// ---------------------------------------------------------------------------
// Ordered function
// ---------------------------------------------------------------------------

func TestOrdered_TwoSources(t *testing.T) {
	a := Ordered(Term("a"), Term("b"))
	b := Ordered(Term("a"), Term("b"))
	if !a.Equals(b) {
		t.Error("Equal Ordered sources should compare equal")
	}
}

// ---------------------------------------------------------------------------
// Or operations
// ---------------------------------------------------------------------------

func TestOr_SingleClause(t *testing.T) {
	a := Term("a")
	result := Or(a)
	if !result.Equals(a) {
		t.Errorf("Or(single) = %s; want %s", result, a)
	}
}

func TestOr_TwoClausesCommutative(t *testing.T) {
	result := Or(Term("a"), Term("b"))
	same := Or(Term("b"), Term("a"))
	if !result.Equals(same) {
		t.Error("Or should be commutative")
	}
}

// ---------------------------------------------------------------------------
// MaxWidth constraint
// ---------------------------------------------------------------------------

func TestMaxWidth_Applied(t *testing.T) {
	result := MaxWidth(5, Term("a"))
	if result == nil {
		t.Fatal("MaxWidth returned nil")
	}
}

func TestMaxWidth_NotEquals(t *testing.T) {
	a := MaxWidth(3, Term("x"))
	b := MaxWidth(5, Term("x"))
	if a.Equals(b) {
		t.Error("MaxWidth with different widths should not compare equal")
	}
}

// ---------------------------------------------------------------------------
// MaxGaps constraint
// ---------------------------------------------------------------------------

func TestMaxGaps_Applied(t *testing.T) {
	result := MaxGaps(2, Term("a"))
	if result == nil {
		t.Fatal("MaxGaps returned nil")
	}
}

func TestMaxGaps_NotEquals(t *testing.T) {
	a := MaxGaps(3, Term("x"))
	b := MaxGaps(7, Term("x"))
	if a.Equals(b) {
		t.Error("MaxGaps with different gaps should not compare equal")
	}
}

// ---------------------------------------------------------------------------
// NoIntervals
// ---------------------------------------------------------------------------

func TestNoIntervals_Identity(t *testing.T) {
	a := NoIntervals("reason")
	b := NoIntervals("reason")
	if !a.Equals(b) {
		t.Error("NoIntervals should equal itself")
	}
}

func TestNoIntervals_NotEqualsTerm(t *testing.T) {
	a := NoIntervals("reason")
	b := Term("a")
	if a.Equals(b) {
		t.Error("NoIntervals should not equal a Term")
	}
}

// ---------------------------------------------------------------------------
// PhraseOf
// ---------------------------------------------------------------------------

func TestPhraseOf_Single(t *testing.T) {
	result := PhraseOf(Term("x"))
	expected := Term("x")
	if !result.Equals(expected) {
		t.Errorf("PhraseOf(single) = %s; want %s", result, expected)
	}
}

func TestPhraseOf_Multiple(t *testing.T) {
	result := PhraseOf(Term("x"), Term("y"))
	single := Term("x")
	if result.Equals(single) {
		t.Error("PhraseOf with multiple sources should not equal single term")
	}
}

// ---------------------------------------------------------------------------
// OrNoRewrite — preserves structure
// ---------------------------------------------------------------------------

func TestOrNoRewrite_PreservesList(t *testing.T) {
	a := OrNoRewrite(Term("a"), Term("b"))
	b := OrNoRewrite(Term("a"), Term("b"))
	if !a.Equals(b) {
		t.Error("Equal OrNoRewrite sources should compare equal")
	}
}

// ---------------------------------------------------------------------------
// UnorderedNoOverlaps
// ---------------------------------------------------------------------------

func TestUnorderedNoOverlaps_Applied(t *testing.T) {
	result := UnorderedNoOverlaps(Term("a"), Term("b"))
	if result == nil {
		t.Fatal("UnorderedNoOverlaps returned nil")
	}
}

// ---------------------------------------------------------------------------
// FixField
// ---------------------------------------------------------------------------

func TestFixField_Identity(t *testing.T) {
	a := FixField("test_field", Term("val"))
	b := FixField("test_field", Term("val"))
	if !a.Equals(b) {
		t.Error("FixField with same field and source should compare equal")
	}
}

func TestFixField_DifferentField(t *testing.T) {
	a := FixField("f1", Term("val"))
	b := FixField("f2", Term("val"))
	if a.Equals(b) {
		t.Error("FixField with different fields should not compare equal")
	}
}

// ---------------------------------------------------------------------------
// TermWithPayloadFilter
// ---------------------------------------------------------------------------

func TestTermWithPayloadFilter_Applied(t *testing.T) {
	result := TermWithPayloadFilter("term", func(payload []byte) bool {
		return len(payload) > 0
	})
	if result == nil {
		t.Fatal("TermWithPayloadFilter returned nil")
	}
}

// ---------------------------------------------------------------------------
// Disjunction operations
// ---------------------------------------------------------------------------

func TestBasicDisjunction_Identity(t *testing.T) {
	a := Term("a")
	d := Or(a, a) // duplicates should be handled
	_ = d
	// Just ensure no panic
}
