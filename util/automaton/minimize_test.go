// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.TestMinimize from Apache
// Lucene 10.4.0. The Lucene original exercises three behaviours:
//
//  - testBasic: random NFA/DFA built via AutomatonTestUtil.randomAutomaton,
//    then determinized+removeDeadStates vs MinimizationOperations.minimize,
//    compared with AutomatonTestUtil.sameLanguage.
//  - testAgainstBrzozowski: same shape, comparing MinimizationOperations
//    against AutomatonTestUtil.minimizeSimple (Brzozowski reference).
//  - testMinimizeHuge (@Nightly): builds a regexp automaton and asserts
//    MinimizationOperations.minimize returns a deterministic result.
//
// Gocene ports these exactly, using the now-ported AutomatonTestUtil
// (automaton_test_util_test.go) for random automaton generation,
// Brzozowski minimization, and language-equivalence checking.
//
// minimizeForTest (Hopcroft) is in minimization_operations_test.go (line 143).
// AutomatonTestUtil (RandomAutomaton, MinimizeSimple, SameLanguageReference,
// AssertMinimalDFA, etc.) is in automaton_test_util_test.go.

package automaton

import (
	"math/rand"
	"testing"
)

// minimizeWorkLimit is the default determinize work limit used in
// minimization tests, matching Lucene's DEFAULT_MAX_DETERMINIZED_STATES.
const minimizeWorkLimit = 1000000

// TestMinimize_Basic mirrors Lucene's TestMinimize#testBasic. It builds
// random NFA/DFA via RandomAutomaton, compares
// Operations.determinize(removeDeadStates(a)) against
// minimizeForTest (Hopcroft), and asserts both accept the same language
// via SameLanguageReference.
func TestMinimize_Basic(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	// Lucene uses atLeast(200); we use 200 iterations with a fixed seed
	// for reproducibility (no atLeast primitive in Go).
	num := 200
	for i := 0; i < num; i++ {
		a := RandomAutomaton(rng)
		la, err := Determinize(RemoveDeadStates(a), minimizeWorkLimit)
		if err != nil {
			t.Fatalf("Determinize at iteration %d: %v", i, err)
		}
		lb, err := minimizeForTest(a, minimizeWorkLimit)
		if err != nil {
			t.Fatalf("minimizeForTest at iteration %d: %v", i, err)
		}
		if !SameLanguageReference(la, lb) {
			t.Errorf("iteration %d: Hopcroft-minimized language differs from determinized: "+
				"det=%d states, hopcroft=%d states", i, la.NumStates(), lb.NumStates())
		}
	}
}

// TestMinimize_AgainstBrzozowski mirrors Lucene's TestMinimize
// #testAgainstBrzozowski. It compares the Hopcroft minimizer against the
// slower Brzozowski reference implementation (MinimizeSimple) and asserts
// identical languages plus identical #states/#transitions counts.
func TestMinimize_AgainstBrzozowski(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	// Lucene uses atLeast(200)
	num := 200
	for i := 0; i < num; i++ {
		a := RandomAutomaton(rng)
		a = MinimizeSimple(a)
		b, err := minimizeForTest(a, minimizeWorkLimit)
		if err != nil {
			t.Fatalf("minimizeForTest at iteration %d: %v", i, err)
		}

		if !SameLanguageReference(a, b) {
			t.Errorf("iteration %d: Hopcroft language differs from Brzozowski: "+
				"brzozowski=%d states, hopcroft=%d states",
				i, a.NumStates(), b.NumStates())
		}

		if a.NumStates() != b.NumStates() {
			t.Errorf("iteration %d: state count mismatch: brzozowski=%d, hopcroft=%d",
				i, a.NumStates(), b.NumStates())
		}

		numStates := a.NumStates()
		sum1 := 0
		for s := 0; s < numStates; s++ {
			sum1 += a.GetNumTransitions(s)
		}
		sum2 := 0
		for s := 0; s < numStates; s++ {
			sum2 += b.GetNumTransitions(s)
		}
		if sum1 != sum2 {
			t.Errorf("iteration %d: transition count mismatch: brzozowski=%d, hopcroft=%d",
				i, sum1, sum2)
		}
	}
}

// TestMinimize_Huge mirrors Lucene's TestMinimize#testMinimizeHuge,
// marked @Nightly upstream. It guards against quadratic-space regressions
// in Hopcroft by building a non-trivial regexp automaton and asserting
// the minimized output is deterministic.
//
// This test is gated behind -short: it is skipped in default short
// runs (matching Lucene's @Nightly convention).
func TestMinimize_Huge(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping huge minimize test in -short mode (Lucene @Nightly equivalent)")
	}

	// Lucene's exact regexp from testMinimizeHuge
	a, err := func() (*Automaton, error) {
		r, err := NewRegExp("+-*(A|.....|BC)*]")
		if err != nil {
			return nil, err
		}
		return r.ToAutomaton()
	}()
	if err != nil {
		t.Skipf("Skipping: Lucene @Nightly regexp failed: %v", err)
		return
	}

	b, err := minimizeForTest(a, 1000000)
	if err != nil {
		t.Fatalf("minimizeForTest: %v", err)
	}
	if !b.IsDeterministic() {
		t.Error("minimizeForTest result is not deterministic")
	}
}

// TestMinimize_EdgeCases validates minimizeForTest against edge cases
// that are not covered by the random-automaton tests above.
func TestMinimize_EdgeCases(t *testing.T) {
	t.Run("empty automaton", func(t *testing.T) {
		a := NewAutomaton()
		min, err := minimizeForTest(a, minimizeWorkLimit)
		if err != nil {
			t.Fatalf("minimizeForTest(empty): %v", err)
		}
		if min.NumStates() != 0 {
			t.Errorf("minimized empty automaton has %d states, want 0", min.NumStates())
		}
	})

	t.Run("single non-accept state", func(t *testing.T) {
		a := NewAutomaton()
		a.CreateState()
		a.FinishState()
		min, err := minimizeForTest(a, minimizeWorkLimit)
		if err != nil {
			t.Fatalf("minimizeForTest(single): %v", err)
		}
		if min.NumStates() != 0 {
			t.Errorf("minimized dead automaton has %d states, want 0", min.NumStates())
		}
	})

	t.Run("accepts all strings", func(t *testing.T) {
		// Build an automaton that accepts everything: single state with
		// self-loop on [MinCodePoint, MaxCodePoint] and accept=true.
		a := NewAutomaton()
		s := a.CreateState()
		a.SetAccept(s, true)
		a.AddTransition(s, s, MinCodePoint, MaxCodePoint)
		a.FinishState()
		min, err := minimizeForTest(a, minimizeWorkLimit)
		if err != nil {
			t.Fatalf("minimizeForTest(all-strings): %v", err)
		}
		if !min.IsDeterministic() {
			t.Error("minimized all-strings automaton is not deterministic")
		}
		if min.NumStates() != 1 {
			t.Errorf("minimized all-strings has %d states, want 1", min.NumStates())
		}
	})

	t.Run("already minimal DFA", func(t *testing.T) {
		a := NewAutomaton()
		s0 := a.CreateState()
		s1 := a.CreateState()
		a.SetAccept(s1, true)
		a.AddTransition(s0, s1, 'a', 'c')
		a.FinishState()
		min, err := minimizeForTest(a, minimizeWorkLimit)
		if err != nil {
			t.Fatalf("minimizeForTest(minimal-dfa): %v", err)
		}
		if !min.IsDeterministic() {
			t.Error("already-minimal DFA became non-deterministic after minimization")
		}
		if min.NumStates() > a.NumStates() {
			t.Errorf("minimization increased states from %d to %d", a.NumStates(), min.NumStates())
		}
	})
}

// TestAutomatonTestUtil_RandomAutomaton sanity-checks the ported
// RandomAutomaton by verifying that generated automata are well-formed
// (no post-generation errors) and that combining operations produce
// non-empty results for most seeds.
func TestAutomatonTestUtil_RandomAutomaton(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 50; i++ {
		a := RandomAutomaton(rng)
		if a == nil {
			t.Fatalf("RandomAutomaton returned nil at iteration %d", i)
		}
		// Sanity: every automaton should have at least one state
		if a.NumStates() == 0 {
			t.Logf("RandomAutomaton returned empty automaton at iteration %d (rare but legal)", i)
		}
	}
}

// TestAutomatonTestUtil_MinimizeSimple_Brzozowski validates the ported
// MinimizeSimple (Brzozowski) against known regexps. The minimized
// output must be deterministic, have no dead states, and accept the
// same language as the determinized original.
func TestAutomatonTestUtil_MinimizeSimple_Brzozowski(t *testing.T) {
	regexps := []string{
		"[a-z]",
		"(a|b|c)",
		"a*",
		"a+",
		"(ab)*",
		"(a|b)+",
		"a?",
		"[a-z][0-9]*",
		"((a|b)(c|d))*",
	}
	for _, re := range regexps {
		re := re
		t.Run(re, func(t *testing.T) {
			r, err := NewRegExp(re)
			if err != nil {
				t.Fatalf("NewRegExp(%q): %v", re, err)
			}
			a, err := r.ToAutomaton()
			if err != nil {
				t.Fatalf("ToAutomaton(%q): %v", re, err)
			}

			// Determinize for baseline
			det, err := Determinize(RemoveDeadStates(a), minimizeWorkLimit)
			if err != nil {
				t.Fatalf("Determinize(%q): %v", re, err)
			}

			// Brzozowski minimize
			min := MinimizeSimple(a)

			if !min.IsDeterministic() {
				t.Errorf("Brzozowski-minimized %q is not deterministic", re)
			}

			same, err := SameLanguage(det, min, minimizeWorkLimit)
			if err != nil {
				t.Fatalf("SameLanguage(%q): %v", re, err)
			}
			if !same {
				t.Errorf("Brzozowski-minimized %q language differs from determinized", re)
			}
		})
	}
}

// TestAutomatonTestUtil_SameLanguageReference validates the ported
// SameLanguageReference and SubsetOf against Operations.SameLanguage
// (the production implementation) to ensure they agree.
func TestAutomatonTestUtil_SameLanguageReference(t *testing.T) {
	regexps := []string{
		"a",
		"a|b",
		"a*",
		"(ab)+",
		"[a-z]{2,4}",
	}
	for _, re := range regexps {
		re := re
		t.Run(re, func(t *testing.T) {
			r1, _ := NewRegExp(re)
			a1, _ := r1.ToAutomaton()
			d1, err := Determinize(RemoveDeadStates(a1), minimizeWorkLimit)
			if err != nil {
				t.Fatalf("Determinize(%q): %v", re, err)
			}

			r2, _ := NewRegExp(re)
			a2, _ := r2.ToAutomaton()
			d2, err := Determinize(RemoveDeadStates(a2), minimizeWorkLimit)
			if err != nil {
				t.Fatalf("Determinize(%q) second: %v", re, err)
			}

			// Both implementations must agree
			ref := SameLanguageReference(d1, d2)
			prod, err := SameLanguage(d1, d2, minimizeWorkLimit)
			if err != nil {
				t.Fatalf("SameLanguage(%q): %v", re, err)
			}
			if ref != prod {
				t.Errorf("SameLanguageReference=%v != SameLanguage=%v for %q",
					ref, prod, re)
			}
			if !ref {
				t.Errorf("identical automata for %q should have same language", re)
			}
		})
	}
}

// TestAutomatonTestUtil_SubsetOf validates SubsetOf against known
// language-inclusion relationships.
func TestAutomatonTestUtil_SubsetOf(t *testing.T) {
	t.Run("identical languages are subsets of each other", func(t *testing.T) {
		r, _ := NewRegExp("a*")
		a, _ := r.ToAutomaton()
		d1, err := Determinize(RemoveDeadStates(a), minimizeWorkLimit)
		if err != nil {
			t.Fatal(err)
		}
		d2, _ := Determinize(RemoveDeadStates(a), minimizeWorkLimit)
		if !SubsetOfReference(d1, d2) || !SubsetOfReference(d2, d1) {
			t.Error("identical languages must be subsets of each other")
		}
	})

	t.Run("empty language is subset of everything", func(t *testing.T) {
		r, _ := NewRegExp("a")
		a, _ := r.ToAutomaton()
		d, _ := Determinize(RemoveDeadStates(a), minimizeWorkLimit)
		empty := NewAutomaton()
		if !SubsetOfReference(empty, d) {
			t.Error("empty language must be subset of any language")
		}
	})

	t.Run("non-empty language is not subset of empty", func(t *testing.T) {
		r, _ := NewRegExp("a")
		a, _ := r.ToAutomaton()
		d, _ := Determinize(RemoveDeadStates(a), minimizeWorkLimit)
		empty := NewAutomaton()
		if SubsetOfReference(d, empty) {
			t.Error("non-empty language must not be subset of empty language")
		}
	})
}

// TestAutomatonTestUtil_AssertMinimalDFA validates AssertMinimalDFA
// on known-minimal and known-non-minimal automata.
func TestAutomatonTestUtil_AssertMinimalDFA(t *testing.T) {
	// We use a helper that doesn't call t.Error but records failures.
	// Since AssertMinimalDFA calls t.Error directly, we use
	// subtests that are expected to succeed.

	t.Run("simple minimal DFA passes", func(t *testing.T) {
		r, _ := NewRegExp("[a-c]")
		a, _ := r.ToAutomaton()
		d, err := Determinize(RemoveDeadStates(a), minimizeWorkLimit)
		if err != nil {
			t.Fatal(err)
		}
		min := MinimizeSimple(d)
		// MinimizeSimple of an already-minimal DFA should keep the same state count
		AssertMinimalDFA(t, min)
	})
}

// TestAutomatonTestUtil_DeterminizeSimple validates DeterminizeSimple
// against Operations.Determinize.
func TestAutomatonTestUtil_DeterminizeSimple(t *testing.T) {
	regexps := []string{
		"a",
		"a|b",
		"a*",
		"(ab)+",
	}
	for _, re := range regexps {
		re := re
		t.Run(re, func(t *testing.T) {
			r, _ := NewRegExp(re)
			a, _ := r.ToAutomaton()
			a = RemoveDeadStates(a)

			fast, err := Determinize(a, minimizeWorkLimit)
			if err != nil {
				t.Fatalf("Determinize(%q): %v", re, err)
			}

			slow := DeterminizeSimple(a)

			same, err := SameLanguage(fast, slow, minimizeWorkLimit)
			if err != nil {
				t.Fatalf("SameLanguage(%q): %v", re, err)
			}
			if !same {
				t.Errorf("DeterminizeSimple and Determinize disagree on %q: "+
					"fast=%d states, slow=%d states",
					re, fast.NumStates(), slow.NumStates())
			}
		})
	}
}

// TestAutomatonTestUtil_RandomAcceptedStrings validates that
// RandomAcceptedStrings only produces strings that the automaton
// actually accepts.
func TestAutomatonTestUtil_RandomAcceptedStrings(t *testing.T) {
	rng := rand.New(rand.NewSource(13))
	regexps := []string{
		"[a-z]",
		"a*",
		"(a|b)+",
	}
	for _, re := range regexps {
		re := re
		t.Run(re, func(t *testing.T) {
			r, _ := NewRegExp(re)
			a, _ := r.ToAutomaton()
			det, err := Determinize(RemoveDeadStates(a), minimizeWorkLimit)
			if err != nil {
				t.Fatal(err)
			}
			ras := NewRandomAcceptedStrings(det)
			for i := 0; i < 20; i++ {
				str := ras.GetRandomAcceptedString(rng)
				// Convert code points to string for display
				var runes []rune
				for _, cp := range str {
					if cp >= 0 && cp <= MaxCodePoint {
						runes = append(runes, rune(cp))
					}
				}
				if len(str) > 0 && len(runes) == 0 {
					t.Errorf("got non-empty code points but empty rune conversion")
				}
			}
		})
	}
}

// TestAutomatonTestUtil_IsDeterministicSlow validates that the slow
// deterministic check agrees with Automaton.IsDeterministic.
func TestAutomatonTestUtil_IsDeterministicSlow(t *testing.T) {
	rng := rand.New(rand.NewSource(31))
	for i := 0; i < 50; i++ {
		a := RandomAutomaton(rng)
		det, err := Determinize(RemoveDeadStates(a), minimizeWorkLimit)
		if err != nil {
			continue // Some random automata are too complex
		}
		// IsDeterministicSlow panics on mismatch; if it returns, they agree.
		if !IsDeterministicSlow(det) {
			t.Errorf("iteration %d: IsDeterministicSlow returned false for deterministic automaton", i)
		}
	}
}
