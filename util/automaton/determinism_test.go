// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.TestDeterminism from Apache
// Lucene 10.4.0 (commit 9983b7c). The Java original exercises:
//
//   - testRegexps: generates atLeast(500) random regexps via
//     AutomatonTestUtil.randomRegexp, compiles each to an automaton,
//     and runs assertAutomaton (which exercises determinize/complement/
//     union/intersection/minus/optional consistency).
//   - testAgainstSimple: generates atLeast(200) random automata via
//     AutomatonTestUtil.randomAutomaton, determinizes each with the
//     slow reference (determinizeSimple) and with the production
//     Operations.determinize, then asserts both accept the same language.
//
// With AutomatonTestUtil now fully ported (automaton_test_util_test.go),
// all prerequisites are satisfied.

package automaton

import (
	"math/rand"
	"testing"
)

// assertAutomaton validates a determinized automaton through a set of
// algebraic identities (from Lucene's TestDeterminism.assertAutomaton):
//
//  1. complement(complement(a)) = a
//  2. a ∪ a = a
//  3. a ∩ a = a
//  4. a \ a = ∅
//  5. optional(a) \ {ε} = a  (when a does not accept ε)
func assertAutomaton(t *testing.T, a *Automaton) {
	t.Helper()

	var err error
	a, err = Determinize(RemoveDeadStates(a), DefaultMaxDeterminizedStates)
	if err != nil {
		t.Fatalf("Determinize(a): %v", err)
	}

	// 1. complement(complement(a)) = a
	comp, err := Complement(a, DefaultMaxDeterminizedStates)
	if err != nil {
		t.Fatalf("Complement(a): %v", err)
	}
	equiv, err := Complement(comp, DefaultMaxDeterminizedStates)
	if err != nil {
		t.Fatalf("Complement(complement(a)): %v", err)
	}
	if !SameLanguageReference(a, equiv) {
		t.Error("complement(complement(a)) != a")
	}

	// 2. a ∪ a = a
	equiv, err = Determinize(
		RemoveDeadStates(Union([]*Automaton{a, a})),
		DefaultMaxDeterminizedStates,
	)
	if err != nil {
		t.Fatalf("Determinize(a ∪ a): %v", err)
	}
	if !SameLanguageReference(a, equiv) {
		t.Error("a ∪ a != a")
	}

	// 3. a ∩ a = a
	equiv, err = Determinize(
		RemoveDeadStates(Intersection(a, a)),
		DefaultMaxDeterminizedStates,
	)
	if err != nil {
		t.Fatalf("Determinize(a ∩ a): %v", err)
	}
	if !SameLanguageReference(a, equiv) {
		t.Error("a ∩ a != a")
	}

	// 4. a \ a = ∅
	empty, err := Minus(a, a, DefaultMaxDeterminizedStates)
	if err != nil {
		t.Fatalf("Minus(a, a): %v", err)
	}
	if !IsEmpty(empty) {
		t.Error("a \\ a != ∅")
	}

	// 5. as long as a doesn't accept the empty string, optional(a) \ {ε} = a
	if !Run(a, "") {
		optional := Optional(a)
		equiv, err = Minus(optional, MakeEmptyString(), DefaultMaxDeterminizedStates)
		if err != nil {
			t.Fatalf("Minus(optional(a), emptyString): %v", err)
		}
		if !SameLanguageReference(a, equiv) {
			t.Error("optional(a) \\ {ε} != a")
		}
	}
}

// TestDeterminism_Regexps mirrors Lucene's TestDeterminism#testRegexps.
// It generates atLeast(500) random regexps via RandomRegexp, compiles each
// to an automaton, and runs assertAutomaton on the result.
func TestDeterminism_Regexps(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	// Lucene uses atLeast(500)
	num := 500
	for i := 0; i < num; i++ {
		re := RandomRegexp(rng)
		r, err := NewRegExp(re)
		if err != nil {
			t.Fatalf("NewRegExp(%q) at iteration %d: %v", re, i, err)
		}
		a, err := r.ToAutomaton()
		if err != nil {
			t.Fatalf("ToAutomaton(%q) at iteration %d: %v", re, i, err)
		}
		assertAutomaton(t, a)
	}
}

// TestDeterminism_AgainstSimple mirrors Lucene's TestDeterminism
// #testAgainstSimple. It generates atLeast(200) random automata via
// RandomAutomaton, determinizes each with the slow reference
// (DeterminizeSimple) and with the production Operations.determinize,
// then asserts both accept the same language via SameLanguageReference.
func TestDeterminism_AgainstSimple(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	// Lucene uses atLeast(200)
	num := 200
	for i := 0; i < num; i++ {
		a := RandomAutomaton(rng)
		a = DeterminizeSimple(a)
		b, err := Determinize(a, DefaultMaxDeterminizedStates)
		if err != nil {
			t.Fatalf("Determinize at iteration %d: %v", i, err)
		}
		if !SameLanguageReference(a, b) {
			t.Errorf("iteration %d: DeterminizeSimple and Determinize disagree: "+
				"simple=%d states, production=%d states",
				i, a.NumStates(), b.NumStates())
		}
	}
}
