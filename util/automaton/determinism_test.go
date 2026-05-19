// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.TestDeterminism from Apache
// Lucene 10.4.0 (commit 9983b7c). The Java original is a thin randomized
// harness over two helpers from AutomatonTestUtil that are not yet
// ported into Gocene:
//
//   - AutomatonTestUtil.randomRegexp(Random) — emits a syntactically
//     valid regexp string drawn from RegExp's full grammar (Sprint 56
//     gap, same family as the random-NFA helpers referenced by
//     minimize_test.go and strings_to_automaton_test.go).
//   - AutomatonTestUtil.randomAutomaton(Random) — builds a random NFA
//     using internal primitives; ditto.
//   - AutomatonTestUtil.determinizeSimple(Automaton) — slow reference
//     determinizer used as the oracle for testAgainstSimple; the only
//     determinizer ported is Operations.determinize (the production
//     one being validated), so a self-comparison would be vacuous.
//
// Every assertion in the file already has a Go counterpart in
// util/automaton/operations.go (Determinize, RemoveDeadStates,
// Complement, Union, Intersection, Minus, IsEmpty, Optional,
// SameLanguage, Run) and util/automaton/automata.go (MakeEmptyString),
// so once the three test-util primitives above are ported the bodies
// can be filled in without further infrastructure work.
//
// Until then this file pins the test surface (so the suite compiles
// and the 1:1 mapping with the Java source is visible) and skips each
// case with an explicit gap message — same pattern used by
// minimize_test.go and the strings_to_automaton_test.go deferrals.

package automaton

import "testing"

// determinismGapMessage is the single source of truth for the skip
// reason so future readers (and the eventual implementer of the
// AutomatonTestUtil helpers) see one canonical pointer rather than
// three slightly different strings.
const determinismGapMessage = "AutomatonTestUtil.randomRegexp / randomAutomaton / determinizeSimple not ported yet; " +
	"see project-gocene-sprint-56-progress memory and " +
	"util/automaton/minimize_test.go for the same deferral pattern"

// TestDeterminism_Regexps mirrors Lucene's TestDeterminism#testRegexps.
// It generates atLeast(500) random regexps via
// AutomatonTestUtil.randomRegexp, compiles each to an automaton with
// RegExp.NONE, and runs assertAutomaton on the result (which exercises
// determinize/complement/union/intersection/minus/optional consistency).
func TestDeterminism_Regexps(t *testing.T) {
	t.Skip(determinismGapMessage)
}

// TestDeterminism_AgainstSimple mirrors Lucene's TestDeterminism
// #testAgainstSimple. It generates atLeast(200) random automata via
// AutomatonTestUtil.randomAutomaton, determinizes each with the slow
// reference (AutomatonTestUtil.determinizeSimple) and with the
// production Operations.determinize, then asserts both accept the same
// language. Skipped until the AutomatonTestUtil random/simple helpers
// are ported.
func TestDeterminism_AgainstSimple(t *testing.T) {
	t.Skip(determinismGapMessage)
}
