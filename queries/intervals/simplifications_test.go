// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/intervals/TestSimplifications.java

package intervals

import (
	"testing"
)

// TestSimplifications_StringPhrases verifies BLOCK(term) => term.
func TestSimplifications_StringPhrases(t *testing.T) {
	actual := Phrase("term")
	expected := Term("term")
	if !actual.Equals(expected) {
		t.Errorf("Phrase(single) = %s; want %s", actual, expected)
	}
}

// TestSimplifications_SourcePhrases verifies PhraseOf(term) => term.
func TestSimplifications_SourcePhrases(t *testing.T) {
	actual := PhraseOf(Term("term"))
	expected := Term("term")
	if !actual.Equals(expected) {
		t.Errorf("PhraseOf(single) = %s; want %s", actual, expected)
	}
}

// TestSimplifications_Ordered verifies ORDERED(term) => term.
func TestSimplifications_Ordered(t *testing.T) {
	actual := Ordered(Term("term"))
	expected := Term("term")
	if !actual.Equals(expected) {
		t.Errorf("Ordered(single) = %s; want %s", actual, expected)
	}
}

// TestSimplifications_OrderedWithDuplicates verifies duplicate terms are preserved in ORDERED.
func TestSimplifications_OrderedWithDuplicates(t *testing.T) {
	actual := Ordered(Term("term"), Term("term"))
	if s := actual.String(); s != "ORDERED(term,term)" {
		t.Errorf("Ordered(term,term).String() = %q; want %q", s, "ORDERED(term,term)")
	}
	actual = Ordered(Term("term"), Term("term"), Term("bar"))
	if s := actual.String(); s != "ORDERED(term,term,bar)" {
		t.Errorf("Ordered(term,term,bar).String() = %q; want %q", s, "ORDERED(term,term,bar)")
	}
}

// TestSimplifications_Unordered verifies UNORDERED(term) => term.
func TestSimplifications_Unordered(t *testing.T) {
	actual := Unordered(Term("term"))
	expected := Term("term")
	if !actual.Equals(expected) {
		t.Errorf("Unordered(single) = %s; want %s", actual, expected)
	}
}

// TestSimplifications_UnorderedWithDuplicates verifies duplicate terms are preserved in UNORDERED.
func TestSimplifications_UnorderedWithDuplicates(t *testing.T) {
	actual := Unordered(Term("term"), Term("term"))
	if s := actual.String(); s != "UNORDERED(term,term)" {
		t.Errorf("Unordered(term,term).String() = %q; want %q", s, "UNORDERED(term,term)")
	}
	actual = Unordered(Term("term"), Term("term"), Term("bar"))
	if s := actual.String(); s != "UNORDERED(term,term,bar)" {
		t.Errorf("Unordered(term,term,bar).String() = %q; want %q", s, "UNORDERED(term,term,bar)")
	}
}

// TestSimplifications_UnorderedOverlaps verifies UNORDERED_NO_OVERLAPS(term, term) => ORDERED(term, term).
func TestSimplifications_UnorderedOverlaps(t *testing.T) {
	actual := UnorderedNoOverlaps(Term("term"), Term("term"))
	expected := Ordered(Term("term"), Term("term"))
	if !actual.Equals(expected) {
		t.Errorf("UnorderedNoOverlaps(term,term) = %s; want %s", actual, expected)
	}
}

// TestSimplifications_DisjunctionSingleton verifies Or(a) => a.
func TestSimplifications_DisjunctionSingleton(t *testing.T) {
	actual := Or(Term("a"))
	expected := Term("a")
	if !actual.Equals(expected) {
		t.Errorf("Or(single) = %s; want %s", actual, expected)
	}
}

// TestSimplifications_DisjunctionRemovesDuplicates verifies or(a, b, a) => or(a, b).
func TestSimplifications_DisjunctionRemovesDuplicates(t *testing.T) {
	actual := Or(Term("a"), Term("b"), Term("a"))
	expected := Or(Term("a"), Term("b"))
	if !actual.Equals(expected) {
		t.Errorf("Or(a,b,a) = %s; want Or(a,b) = %s", actual, expected)
	}
}

// TestSimplifications_PhraseSimplification verifies BLOCK(BLOCK(a,b), c) => BLOCK(a,b,c).
func TestSimplifications_PhraseSimplification(t *testing.T) {
	actual := PhraseOf(PhraseOf(Term("a"), Term("b")), Term("c"))
	expected := PhraseOf(Term("a"), Term("b"), Term("c"))
	if !actual.Equals(expected) {
		t.Errorf("PhraseOf(PhraseOf(a,b),c) = %s; want %s", actual, expected)
	}

	actual = PhraseOf(Term("a"), PhraseOf(Term("b"), PhraseOf(Term("c"), Term("d"))))
	expected = PhraseOf(Term("a"), Term("b"), Term("c"), Term("d"))
	if !actual.Equals(expected) {
		t.Errorf("nested phrase = %s; want %s", actual, expected)
	}
}

// TestSimplifications_DisjunctionSimplification verifies or(a, or(b, or(c, d))) => or(a, b, c, d).
func TestSimplifications_DisjunctionSimplification(t *testing.T) {
	actual := Or(Term("a"), Or(Term("b"), Or(Term("c"), Term("d"))))
	expected := Or(Term("a"), Term("b"), Term("c"), Term("d"))
	if !actual.Equals(expected) {
		t.Errorf("nested Or = %s; want %s", actual, expected)
	}
}

// TestSimplifications_MinShouldMatchSimplifications verifies AtLeast(2, a, b) => UNORDERED(a, b)
// and AtLeast(3, a, b) => NOMATCH.
func TestSimplifications_MinShouldMatchSimplifications(t *testing.T) {
	expected := Unordered(Term("a"), Term("b"))
	actual := AtLeast(2, Term("a"), Term("b"))
	if !actual.Equals(expected) {
		t.Errorf("AtLeast(2,a,b) = %s; want %s", actual, expected)
	}

	noMatch := AtLeast(3, Term("a"), Term("b"))
	const want = "NOMATCH(Too few sources to match minimum of [3]: [a, b])"
	if s := noMatch.String(); s != want {
		t.Errorf("AtLeast(3,a,b).String() = %q; want %q", s, want)
	}
}
