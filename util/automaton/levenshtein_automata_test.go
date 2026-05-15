// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package automaton

import "testing"

func TestLevenshteinAutomata_Distance1(t *testing.T) {
	la := NewLevenshteinAutomataString("foo", false)
	a := la.ToAutomaton(1)
	if a == nil {
		t.Fatal("expected non-nil automaton for distance 1")
	}
	det, err := Determinize(a, DefaultDeterminizeWorkLimit)
	if err != nil {
		t.Fatalf("determinize: %v", err)
	}
	// Within edit distance 1 of "foo".
	for _, in := range []string{"foo", "fo", "foa", "foox", "fxo"} {
		if !Run(det, in) {
			t.Errorf("expected %q to match foo~1", in)
		}
	}
	for _, in := range []string{"bar", "foooo", "bzz"} {
		if Run(det, in) {
			t.Errorf("expected %q not to match foo~1", in)
		}
	}
}

func TestLevenshteinAutomata_Distance2(t *testing.T) {
	la := NewLevenshteinAutomataString("hello", false)
	a := la.ToAutomaton(2)
	if a == nil {
		t.Fatal("expected non-nil automaton for distance 2")
	}
	det, err := Determinize(a, DefaultDeterminizeWorkLimit)
	if err != nil {
		t.Fatalf("determinize: %v", err)
	}
	if !Run(det, "hello") {
		t.Error("expected exact match")
	}
	if !Run(det, "helo") {
		t.Error("expected 'helo' within distance 2")
	}
	if !Run(det, "hellos") {
		t.Error("expected 'hellos' within distance 2")
	}
	if Run(det, "world") {
		t.Error("expected 'world' not to match hello~2")
	}
}

func TestLevenshteinAutomata_DistanceZero(t *testing.T) {
	la := NewLevenshteinAutomataString("abc", false)
	a := la.ToAutomaton(0)
	if a == nil {
		t.Fatal("expected non-nil automaton for distance 0")
	}
	if !Run(a, "abc") {
		t.Error("expected exact match at distance 0")
	}
	if Run(a, "ab") {
		t.Error("expected no match at distance 0 for shorter input")
	}
}

func TestLevenshteinAutomata_TransposeDistance1(t *testing.T) {
	la := NewLevenshteinAutomataString("foo", true)
	a := la.ToAutomaton(1)
	det, err := Determinize(a, DefaultDeterminizeWorkLimit)
	if err != nil {
		t.Fatalf("determinize: %v", err)
	}
	// Transpose of 'fo' inside 'foo' gives 'ofo' (within distance 1 via transpose).
	if !Run(det, "ofo") {
		t.Error("expected 'ofo' within transpose-aware distance 1 of 'foo'")
	}
}
