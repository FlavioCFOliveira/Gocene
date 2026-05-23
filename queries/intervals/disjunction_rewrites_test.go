// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/intervals/TestDisjunctionRewrites.java

package intervals

import (
	"testing"
)

// TestDisjunctionRewrites_DisjunctionSuffix verifies BLOCK(a,or(b, BLOCK(b, c))) => or(BLOCK(a, b), BLOCK(a, b, c)).
func TestDisjunctionRewrites_DisjunctionSuffix(t *testing.T) {
	actual := PhraseOf(Term("a"), Or(Term("b"), Phrase("b", "c")))
	expected := Or(Phrase("a", "b"), Phrase("a", "b", "c"))
	if !actual.Equals(expected) {
		t.Errorf("got %s; want %s", actual, expected)
	}
}

// TestDisjunctionRewrites_PhraseDisjunctionWithDifferentLengthClauses verifies
// BLOCK(a, or(b, BLOCK(b, c)), d) => or(BLOCK(a, b, d), BLOCK(a, b, c, d)).
func TestDisjunctionRewrites_PhraseDisjunctionWithDifferentLengthClauses(t *testing.T) {
	actual := PhraseOf(Term("a"), Or(Term("b"), PhraseOf(Term("b"), Term("c"))), Term("d"))
	expected := Or(Phrase("a", "b", "d"), Phrase("a", "b", "c", "d"))
	if !actual.Equals(expected) {
		t.Errorf("got %s; want %s", actual, expected)
	}
}

// TestDisjunctionRewrites_PhraseDisjunctionWithNestedDifferentLengthClauses verifies
// deep nesting rewrite.
func TestDisjunctionRewrites_PhraseDisjunctionWithNestedDifferentLengthClauses(t *testing.T) {
	expected := Or(
		PhraseOf(Term("a"), Or(Term("b"), Term("p"), Term("q")), Term("f"), Term("g")),
		PhraseOf(Term("a"), Ordered(Or(Term("b"), Term("c")), Term("d")), Term("f"), Term("g")),
	)
	actual := PhraseOf(
		Term("a"),
		Or(Ordered(Or(Term("b"), Term("c")), Term("d")), Term("b"), Term("p"), Term("q")),
		Term("f"),
		Term("g"),
	)
	if !actual.Equals(expected) {
		t.Errorf("got %s; want %s", actual, expected)
	}
}

// TestDisjunctionRewrites_DisjunctionRewritePreservesFilters verifies maxgaps preserves structure.
func TestDisjunctionRewrites_DisjunctionRewritePreservesFilters(t *testing.T) {
	actual := PhraseOf(
		Term("a"),
		MaxGaps(3, Or(Phrase("a", "b"), Phrase("c", "d"))),
		Term("c"),
	)
	expected := Or(
		PhraseOf(Term("a"), MaxGaps(3, Phrase("a", "b")), Term("c")),
		PhraseOf(Term("a"), MaxGaps(3, Phrase("c", "d")), Term("c")),
	)
	if !actual.Equals(expected) {
		t.Errorf("got %s; want %s", actual, expected)
	}
}

// TestDisjunctionRewrites_NestedMaxGaps verifies nested maxgaps rewrite.
func TestDisjunctionRewrites_NestedMaxGaps(t *testing.T) {
	actual := MaxGaps(3, Ordered(MaxGaps(4, Ordered(Term("a"), Or(Term("b"), Phrase("c", "d")))), Term("e")))
	expected := Or(
		MaxGaps(3, Ordered(MaxGaps(4, Ordered(Term("a"), Term("b"))), Term("e"))),
		MaxGaps(3, Ordered(MaxGaps(4, Ordered(Term("a"), Phrase("c", "d"))), Term("e"))),
	)
	if !actual.Equals(expected) {
		t.Errorf("got %s; want %s", actual, expected)
	}
}

// TestDisjunctionRewrites_NestedMaxWidth verifies maxwidth pull-up in a phrase context.
func TestDisjunctionRewrites_NestedMaxWidth(t *testing.T) {
	actual := PhraseOf(
		Term("a"),
		MaxWidth(4, Or(Ordered(Term("b"), Term("c")), Ordered(Term("d"), Term("e")))),
		Term("f"),
	)
	expected := Or(
		PhraseOf(Term("a"), MaxWidth(4, Ordered(Term("b"), Term("c"))), Term("f")),
		PhraseOf(Term("a"), MaxWidth(4, Ordered(Term("d"), Term("e"))), Term("f")),
	)
	if !actual.Equals(expected) {
		t.Errorf("got %s; want %s", actual, expected)
	}
}

// TestDisjunctionRewrites_NestedFixField verifies fixField pull-up.
func TestDisjunctionRewrites_NestedFixField(t *testing.T) {
	actual := PhraseOf(Term("a"), FixField("field", Or(Phrase("a", "b"), Term("b"))), Term("c"))
	expected := Or(
		PhraseOf(Term("a"), FixField("field", Phrase("a", "b")), Term("c")),
		PhraseOf(Term("a"), FixField("field", Term("b")), Term("c")),
	)
	if !actual.Equals(expected) {
		t.Errorf("got %s; want %s", actual, expected)
	}
}

// TestDisjunctionRewrites_ContainedBy verifies containedBy pull-up.
func TestDisjunctionRewrites_ContainedBy(t *testing.T) {
	actual := ContainedBy(
		Or(Term("s"), Phrase("s", "t")),
		MaxGaps(4, Or(Ordered(Term("a"), Term("b")), Ordered(Term("c"), Term("d")))),
	)
	expected := Or(
		ContainedBy(Or(Term("s"), Phrase("s", "t")), MaxGaps(4, Ordered(Term("a"), Term("b")))),
		ContainedBy(Or(Term("s"), Phrase("s", "t")), MaxGaps(4, Ordered(Term("c"), Term("d")))),
	)
	if !actual.Equals(expected) {
		t.Errorf("got %s; want %s", actual, expected)
	}
}

// TestDisjunctionRewrites_Containing verifies containing pull-up.
func TestDisjunctionRewrites_Containing(t *testing.T) {
	actual := Containing(
		MaxGaps(4, Or(Ordered(Term("a"), Term("b")), Ordered(Term("c"), Term("d")))),
		Or(Term("s"), Phrase("s", "t")),
	)
	expected := Or(
		Containing(MaxGaps(4, Ordered(Term("a"), Term("b"))), Or(Term("s"), Phrase("s", "t"))),
		Containing(MaxGaps(4, Ordered(Term("c"), Term("d"))), Or(Term("s"), Phrase("s", "t"))),
	)
	if !actual.Equals(expected) {
		t.Errorf("got %s; want %s", actual, expected)
	}
}

// TestDisjunctionRewrites_NotContainedBy verifies notContainedBy pull-up.
func TestDisjunctionRewrites_NotContainedBy(t *testing.T) {
	actual := NotContainedBy(Or(Phrase("a", "b"), Term("a")), Or(Phrase("c", "d"), Term("d")))
	expected := Or(
		NotContainedBy(Or(Phrase("a", "b"), Term("a")), Phrase("c", "d")),
		NotContainedBy(Or(Phrase("a", "b"), Term("a")), Term("d")),
	)
	if !actual.Equals(expected) {
		t.Errorf("got %s; want %s", actual, expected)
	}
}

// TestDisjunctionRewrites_NotContaining verifies notContaining pull-up.
func TestDisjunctionRewrites_NotContaining(t *testing.T) {
	actual := NotContaining(Or(Phrase("a", "b"), Term("a")), Or(Phrase("c", "d"), Term("d")))
	expected := Or(
		NotContaining(Phrase("a", "b"), Or(Phrase("c", "d"), Term("d"))),
		NotContaining(Term("a"), Or(Phrase("c", "d"), Term("d"))),
	)
	if !actual.Equals(expected) {
		t.Errorf("got %s; want %s", actual, expected)
	}
}

// TestDisjunctionRewrites_BlockedRewrites verifies that OrNoRewrite prevents pull-ups.
func TestDisjunctionRewrites_BlockedRewrites(t *testing.T) {
	actual := PhraseOf(Term("a"), OrNoRewrite(Phrase("b", "c"), Term("c")))
	ifRewritten := Or(Phrase("a", "b", "c"), Phrase("a", "c"))
	if actual.Equals(ifRewritten) {
		t.Error("expected non-rewritten form to differ from rewritten form")
	}
}
