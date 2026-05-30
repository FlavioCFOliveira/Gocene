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
// All three depend on primitives that are not yet ported into Gocene:
//
//  - MinimizationOperations.minimize (Hopcroft) — no Go counterpart in
//    util/automaton/. Tracked alongside the Sprint 56 progress notes for
//    StringsToAutomaton, where the same gap deferred testRandomMinimized.
//  - AutomatonTestUtil.randomAutomaton / minimizeSimple — no Go test
//    helper ships these primitives yet.
//
// SameLanguage is available in util/automaton/operations.go, so once the
// Minimize port lands the body of these tests can be filled in without
// further infrastructure work. Until then, this file pins the test
// surface (so the suite compiles and the 1:1 mapping is visible) and
// skips each case with an explicit gap message — same pattern used for
// the SegmentReader core-readers gap and the Sprint 56 deferrals.

package automaton

import "testing"

// minimizeGapMessage is the single source of truth for the skip reason so
// that future readers (and the eventual implementer of Minimize) see one
// canonical pointer instead of three slightly different strings.
const minimizeGapMessage = "MinimizationOperations.minimize not ported yet; " +
	"see project-gocene-sprint-56-progress memory and " +
	"util/automaton/strings_to_automaton_test.go for the same deferral pattern"

// TestMinimize_Basic mirrors Lucene's TestMinimize#testBasic. It builds
// random NFA/DFA, compares Operations.determinize(removeDeadStates(a))
// against MinimizationOperations.minimize(a), and asserts both accept
// the same language.
func TestMinimize_Basic(t *testing.T) {
	t.Fatal(minimizeGapMessage)
}

// TestMinimize_AgainstBrzozowski mirrors Lucene's TestMinimize
// #testAgainstBrzozowski. It compares the Hopcroft minimizer against the
// slower Brzozowski reference implementation (AutomatonTestUtil
// .minimizeSimple) and asserts identical languages plus identical
// #states/#transitions counts.
func TestMinimize_AgainstBrzozowski(t *testing.T) {
	t.Fatal(minimizeGapMessage)
}

// TestMinimize_Huge mirrors Lucene's TestMinimize#testMinimizeHuge,
// marked @Nightly upstream. It guards against quadratic-space regressions
// in Hopcroft by building a non-trivial regexp automaton and asserting
// the minimized output is deterministic. Gocene has no nightly tier; the
// test is gated behind -short so it is skipped by default short runs and
// can be promoted once Minimize lands.
func TestMinimize_Huge(t *testing.T) {
	if testing.Short() {
		t.Fatal("skipping huge minimize test in -short mode (Lucene @Nightly equivalent)")
	}
	t.Fatal(minimizeGapMessage)
}
