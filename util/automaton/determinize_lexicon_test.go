// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.TestDeterminizeLexicon from
// Apache Lucene 10.4.0 (commit 9983b7c). The Java original builds a
// 5_000-term random Unicode lexicon, unions every MakeString automaton,
// determinizes the result with workLimit = 1_000_000, and asserts:
//
//  1. The determinized lexicon is finite — via AutomatonTestUtil.isFinite.
//  2. Every original term is accepted by the deterministic automaton —
//     via Operations.run.
//  3. (TEST_NIGHTLY only) Every term's UTF-8 byte sequence is accepted
//     by ByteRunAutomaton.
//
// Assertion (1) is the load-bearing correctness check that distinguishes
// this test from a plain membership smoke test: it pins the contract
// that Operations.determinize never introduces a cycle when the input is
// a finite-language union of MakeString automata. Without isFinite the
// remaining assertions degenerate into the same membership coverage
// already provided by strings_to_automaton_test.go and the byte-run
// assertions in byte_run_automaton-adjacent tests, so the port cannot
// be partially landed without silently weakening the contract.
//
// Gocene currently lacks two primitives required to faithfully port this
// test:
//
//   - AutomatonTestUtil.isFinite(Automaton) — no Go counterpart in
//     util/automaton/. The only Finite-related symbols ported so far are
//     FiniteStringsIterator / LimitedFiniteStringsIterator, which
//     enumerate finite languages but do not decide finiteness.
//   - The Java test relies on TestUtil.randomUnicodeString, whose
//     contract guarantees a string of valid Unicode code points
//     (excluding unpaired surrogates) suitable for round-tripping
//     through UTF-8. util.RandomUnicodeString in Gocene currently emits
//     unpaired surrogates (chars_ref.go), which would corrupt the
//     UTF-8 byte path in the nightly branch.
//
// Operations.Union and Operations.Determinize are available, so once
// IsFinite lands (and a surrogate-safe random Unicode helper is
// available — see strings_to_automaton_test.randomUnicodeTerms for the
// local pattern that skips the D800..DFFF range), the body can be
// filled in without further infrastructure work.
//
// Until then this file pins the test surface (so the suite compiles
// and the 1:1 mapping with the Java source is visible) and skips with
// an explicit gap message — same pattern used by determinism_test.go
// and minimize_test.go.

package automaton

import "testing"

// determinizeLexiconGapMessage is the single source of truth for the
// skip reason so that future readers (and the eventual implementer of
// IsFinite) see one canonical pointer rather than three slightly
// different strings.
const determinizeLexiconGapMessage = "AutomatonTestUtil.isFinite not ported yet (and util.RandomUnicodeString " +
	"emits unpaired surrogates incompatible with the nightly ByteRunAutomaton branch); " +
	"see project-gocene-sprint-56-progress memory and " +
	"util/automaton/determinism_test.go for the same deferral pattern"

// TestDeterminizeLexicon_Lexicon mirrors Lucene's
// TestDeterminizeLexicon#testLexicon. It builds atLeast(1) iterations
// of a 5_000-term random Unicode lexicon, unions every MakeString
// automaton, determinizes with workLimit = 1_000_000, asserts the
// result is finite (the load-bearing check) and that every original
// term is accepted by Operations.Run. The nightly branch additionally
// asserts UTF-8 byte acceptance through ByteRunAutomaton.
func TestDeterminizeLexicon_Lexicon(t *testing.T) {
	t.Fatal(determinizeLexiconGapMessage)
}
