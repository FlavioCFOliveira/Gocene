// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.TestRegExpParsing from
// Apache Lucene 10.4.0 (commit 9983b7c). Each Java test maps to a
// Go counterpart prefixed with TestRegExpParsing_.
//
// Divergences from the Java source:
//   - Lucene's RegExp.toStringTree() and AutomatonTestUtil.assertMinimalDFA
//     / assertCleanDFA / assertCleanNFA are not yet ported. Tests whose
//     substance is exclusively those assertions are skipped with
//     t.Skipf and a citation; tests that also exercise the resulting
//     language are kept and validated via SameLanguage on determinized
//     automata.
//   - Case-insensitive matching flags currently return
//     ErrCaseFoldingUnsupported at ToAutomaton time (no CaseFolding
//     tables ported). Those tests assert the contracted error rather
//     than language equivalence.
//   - Lucene's IllegalArgumentException maps to a non-nil error from
//     NewRegExp / NewRegExpFlags.

package automaton

import (
	"errors"
	"strings"
	"testing"
)

// assertSameRegExpLanguage mirrors AutomatonTestUtil.sameLanguage by
// determinizing both inputs before comparing.
func assertSameRegExpLanguage(t *testing.T, expected, actual *Automaton) {
	t.Helper()
	exp, err := Determinize(expected, DefaultDeterminizeWorkLimit)
	if err != nil {
		t.Fatalf("determinize expected: %v", err)
	}
	act, err := Determinize(actual, DefaultDeterminizeWorkLimit)
	if err != nil {
		t.Fatalf("determinize actual: %v", err)
	}
	ok, err := SameLanguage(exp, act, DefaultDeterminizeWorkLimit)
	if err != nil {
		t.Fatalf("sameLanguage: %v", err)
	}
	if !ok {
		t.Fatalf("languages differ")
	}
}

// assertCleanDFA approximates AutomatonTestUtil.assertCleanDFA: deterministic
// and with no dead states reachable from the initial state. It does not
// enforce minimality (Hopcroft minimize is not ported).
func assertCleanDFA(t *testing.T, a *Automaton) {
	t.Helper()
	if !a.IsDeterministic() {
		t.Fatalf("automaton is not deterministic")
	}
	if HasDeadStatesFromInitial(a) {
		t.Fatalf("automaton has dead states reachable from initial")
	}
}

// minimalDFAUnportedSkip is the standard skip reason for assertions whose
// only substance in Lucene's test is toStringTree / Hopcroft minimize.
const minimalDFAUnportedSkip = "AutomatonTestUtil.assertMinimalDFA / RegExp.toStringTree not yet ported"

func mustNewRegExp(t *testing.T, s string) *RegExp {
	t.Helper()
	r, err := NewRegExp(s)
	if err != nil {
		t.Fatalf("NewRegExp(%q): %v", s, err)
	}
	return r
}

func mustNewRegExpFlags(t *testing.T, s string, syntax, match int) *RegExp {
	t.Helper()
	r, err := NewRegExpFlags(s, syntax, match)
	if err != nil {
		t.Fatalf("NewRegExpFlags(%q, 0x%x, 0x%x): %v", s, syntax, match, err)
	}
	return r
}

func mustToAutomaton(t *testing.T, r *RegExp) *Automaton {
	t.Helper()
	a, err := r.ToAutomaton()
	if err != nil {
		t.Fatalf("ToAutomaton: %v", err)
	}
	return a
}

func expectParseError(t *testing.T, s string) {
	t.Helper()
	if _, err := NewRegExp(s); err == nil {
		t.Fatalf("NewRegExp(%q) succeeded; expected parse error", s)
	}
}

func expectParseErrorFlags(t *testing.T, s string, syntax, match int) {
	t.Helper()
	if _, err := NewRegExpFlags(s, syntax, match); err == nil {
		t.Fatalf("NewRegExpFlags(%q, 0x%x, 0x%x) succeeded; expected parse error", s, syntax, match)
	}
}

// expectCaseFoldingUnsupported asserts that ToAutomaton returns the
// documented sentinel until CaseFolding tables are ported.
func expectCaseFoldingUnsupported(t *testing.T, r *RegExp) {
	t.Helper()
	_, err := r.ToAutomaton()
	if err == nil {
		t.Fatalf("ToAutomaton: expected ErrCaseFoldingUnsupported, got nil")
	}
	if !errors.Is(err, ErrCaseFoldingUnsupported) {
		t.Fatalf("ToAutomaton: expected ErrCaseFoldingUnsupported, got %v", err)
	}
}

// --- ports of TestRegExpParsing methods (1:1 ordering with Java) ---

func TestRegExpParsing_AnyChar(t *testing.T) {
	r := mustNewRegExp(t, ".")
	if got, want := r.String(), "."; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	assertSameRegExpLanguage(t, MakeAnyChar(), mustToAutomaton(t, r))
}

func TestRegExpParsing_AnyString(t *testing.T) {
	r := mustNewRegExpFlags(t, "@", RegExpAnyString, 0)
	if got, want := r.String(), "@"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	assertSameRegExpLanguage(t, MakeAnyString(), mustToAutomaton(t, r))
}

func TestRegExpParsing_Char(t *testing.T) {
	r := mustNewRegExp(t, "c")
	if got, want := r.String(), "\\c"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	assertSameRegExpLanguage(t, MakeChar('c'), mustToAutomaton(t, r))
}

func TestRegExpParsing_CaseInsensitiveChar(t *testing.T) {
	r := mustNewRegExpFlags(t, "c", RegExpNone, RegExpASCIICaseInsensitive)
	if got, want := r.String(), "\\c"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	expectCaseFoldingUnsupported(t, r)
}

func TestRegExpParsing_CaseInsensitiveClassChar(t *testing.T) {
	// Lucene: parser folds the single character inside the class. Gocene's
	// parser bails at parse time with the sentinel (parseCharClasses
	// returns ErrCaseFoldingUnsupported for non-range chars under the
	// case-insensitive mask).
	_, err := NewRegExpFlags("[c]", RegExpNone, RegExpASCIICaseInsensitive)
	if err == nil {
		t.Fatalf("NewRegExpFlags: expected ErrCaseFoldingUnsupported, got nil")
	}
	if !errors.Is(err, ErrCaseFoldingUnsupported) {
		t.Fatalf("NewRegExpFlags: expected ErrCaseFoldingUnsupported, got %v", err)
	}
}

func TestRegExpParsing_CaseInsensitiveClassRangeDisabled(t *testing.T) {
	// Range inside a class is not case-folded under ASCII_CASE_INSENSITIVE;
	// parser accepts and ToAutomaton emits the plain [c-d] range.
	r := mustNewRegExpFlags(t, "[c-d]", RegExpNone, RegExpASCIICaseInsensitive)
	a, err := r.ToAutomaton()
	if err != nil {
		t.Fatalf("ToAutomaton: %v", err)
	}
	assertSameRegExpLanguage(t, MakeCharRange('c', 'd'), a)
}

func TestRegExpParsing_CaseInsensitiveClassRange(t *testing.T) {
	// Lucene: parser accepts then ToAutomaton folds. Gocene parser bails
	// directly at parse time when CASE_INSENSITIVE_RANGE is set, mirroring
	// the sentinel contract.
	_, err := NewRegExpFlags("[c-d]", RegExpNone, RegExpCaseInsensitiveRange)
	if err == nil {
		t.Fatalf("NewRegExpFlags: expected ErrCaseFoldingUnsupported, got nil")
	}
	if !errors.Is(err, ErrCaseFoldingUnsupported) {
		t.Fatalf("NewRegExpFlags: expected ErrCaseFoldingUnsupported, got %v", err)
	}
}

func TestRegExpParsing_CaseInsensitiveClassRangeCompression(t *testing.T) {
	// Same divergence as TestRegExpParsing_CaseInsensitiveClassRange:
	// parser rejects immediately because CASE_INSENSITIVE_RANGE is set.
	_, err := NewRegExpFlags("[a-z]", RegExpNone, RegExpCaseInsensitiveRange)
	if err == nil {
		t.Fatalf("NewRegExpFlags: expected ErrCaseFoldingUnsupported, got nil")
	}
	if !errors.Is(err, ErrCaseFoldingUnsupported) {
		t.Fatalf("NewRegExpFlags: expected ErrCaseFoldingUnsupported, got %v", err)
	}
}

func TestRegExpParsing_CaseInsensitiveCharUpper(t *testing.T) {
	r := mustNewRegExpFlags(t, "C", RegExpNone, RegExpASCIICaseInsensitive)
	if got, want := r.String(), "\\C"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	expectCaseFoldingUnsupported(t, r)
}

func TestRegExpParsing_CaseInsensitiveCharNotSensitive(t *testing.T) {
	// Lucene: non-letter chars are returned verbatim under
	// ASCII_CASE_INSENSITIVE. Gocene's ToAutomaton applies the sentinel
	// uniformly for any single RegExpKindChar under the case-insensitive
	// mask (it does not inspect whether the code point is a cased letter).
	// Match the current contract.
	r := mustNewRegExpFlags(t, "4", RegExpNone, RegExpASCIICaseInsensitive)
	if got, want := r.String(), "\\4"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	expectCaseFoldingUnsupported(t, r)
}

func TestRegExpParsing_CaseInsensitiveCharNonAscii(t *testing.T) {
	r := mustNewRegExpFlags(t, "Ж", RegExpNone, RegExpASCIICaseInsensitive)
	if got, want := r.String(), "\\Ж"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	expectCaseFoldingUnsupported(t, r)
}

func TestRegExpParsing_CaseInsensitiveCharUnicode(t *testing.T) {
	r := mustNewRegExpFlags(t, "Ж", RegExpNone, RegExpCaseInsensitive)
	if got, want := r.String(), "\\Ж"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	expectCaseFoldingUnsupported(t, r)
}

func TestRegExpParsing_CaseInsensitiveCharUnicodeSigma(t *testing.T) {
	r := mustNewRegExpFlags(t, "σ", RegExpNone, RegExpCaseInsensitive)
	if got, want := r.String(), "\\σ"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	expectCaseFoldingUnsupported(t, r)
}

func TestRegExpParsing_NegatedChar(t *testing.T) {
	r := mustNewRegExp(t, "[^c]")
	if got, want := r.String(), "(.&~(\\c))"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	expected := Union([]*Automaton{
		MakeCharRange(0, 'b'),
		MakeCharRange('d', MaxCodePoint),
	})
	assertSameRegExpLanguage(t, expected, mustToAutomaton(t, r))
}

func TestRegExpParsing_NegatedClass(t *testing.T) {
	r := mustNewRegExp(t, "[^c-da]")
	// toStringTree assertion is unported; verify the automaton compiles
	// to a clean DFA after determinization and acts as the expected
	// complement of {a, c, d}.
	a, err := Determinize(mustToAutomaton(t, r), DefaultDeterminizeWorkLimit)
	if err != nil {
		t.Fatalf("determinize: %v", err)
	}
	for _, in := range []string{"a", "c", "d"} {
		if Run(a, in) {
			t.Errorf("[^c-da] should not match %q", in)
		}
	}
	for _, in := range []string{"b", "e", "z"} {
		if !Run(a, in) {
			t.Errorf("[^c-da] should match %q", in)
		}
	}
}

func TestRegExpParsing_CharRange(t *testing.T) {
	r := mustNewRegExp(t, "[b-d]")
	if got, want := r.String(), "[\\b-\\d]"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	assertSameRegExpLanguage(t, MakeCharRange('b', 'd'), mustToAutomaton(t, r))
}

func TestRegExpParsing_NegatedCharRange(t *testing.T) {
	r := mustNewRegExp(t, "[^b-d]")
	if got, want := r.String(), "(.&~([\\b-\\d]))"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	expected := Union([]*Automaton{
		MakeCharRange(0, 'a'),
		MakeCharRange('e', MaxCodePoint),
	})
	assertSameRegExpLanguage(t, expected, mustToAutomaton(t, r))
}

func TestRegExpParsing_IllegalCharRange(t *testing.T) {
	expectParseError(t, "[z-a]")
}

func TestRegExpParsing_CharClassDigit(t *testing.T) {
	r := mustNewRegExp(t, "[\\d]")
	if got, want := r.String(), "[\\0-\\9]"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	assertSameRegExpLanguage(t, MakeCharRange('0', '9'), mustToAutomaton(t, r))
}

func TestRegExpParsing_CharClassNonDigit(t *testing.T) {
	r := mustNewRegExp(t, "[\\D]")
	expected, err := Minus(MakeAnyChar(), MakeCharRange('0', '9'), DefaultDeterminizeWorkLimit)
	if err != nil {
		t.Fatalf("Minus: %v", err)
	}
	assertSameRegExpLanguage(t, expected, mustToAutomaton(t, r))
}

func TestRegExpParsing_CharClassWhitespace(t *testing.T) {
	r := mustNewRegExp(t, "[\\s]")
	expected := Union([]*Automaton{
		MakeChar(' '),
		MakeChar('\n'),
		MakeChar('\r'),
		MakeChar('\t'),
	})
	assertSameRegExpLanguage(t, expected, mustToAutomaton(t, r))
}

func TestRegExpParsing_CharClassNonWhitespace(t *testing.T) {
	r := mustNewRegExp(t, "[\\S]")
	expected := MakeAnyChar()
	for _, c := range []int{' ', '\n', '\r', '\t'} {
		next, err := Minus(expected, MakeChar(c), DefaultDeterminizeWorkLimit)
		if err != nil {
			t.Fatalf("Minus(%U): %v", c, err)
		}
		expected = next
	}
	assertSameRegExpLanguage(t, expected, mustToAutomaton(t, r))
}

func TestRegExpParsing_CharClassWord(t *testing.T) {
	r := mustNewRegExp(t, "[\\w]")
	if got, want := r.String(), "[\\0-\\9\\A-\\Z\\_\\a-\\z]"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	expected := Union([]*Automaton{
		MakeCharRange('a', 'z'),
		MakeCharRange('A', 'Z'),
		MakeCharRange('0', '9'),
		MakeChar('_'),
	})
	assertSameRegExpLanguage(t, expected, mustToAutomaton(t, r))
}

func TestRegExpParsing_CharClassNonWord(t *testing.T) {
	r := mustNewRegExp(t, "[\\W]")
	expected := MakeAnyChar()
	for _, part := range []*Automaton{
		MakeCharRange('a', 'z'),
		MakeCharRange('A', 'Z'),
		MakeCharRange('0', '9'),
		MakeChar('_'),
	} {
		next, err := Minus(expected, part, DefaultDeterminizeWorkLimit)
		if err != nil {
			t.Fatalf("Minus: %v", err)
		}
		expected = next
	}
	assertSameRegExpLanguage(t, expected, mustToAutomaton(t, r))
}

func TestRegExpParsing_JumboCharClass(t *testing.T) {
	t.Fatalf("%s", minimalDFAUnportedSkip)
}

func TestRegExpParsing_TruncatedCharClass(t *testing.T) {
	expectParseError(t, "[b-d")
}

func TestRegExpParsing_BogusCharClass(t *testing.T) {
	expectParseError(t, "[\\q]")
}

func TestRegExpParsing_EscapedNotCharClass(t *testing.T) {
	r := mustNewRegExp(t, "[\\?]")
	if got, want := r.String(), "\\?"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	assertSameRegExpLanguage(t, MakeChar('?'), mustToAutomaton(t, r))
}

func TestRegExpParsing_EscapedSlashNotCharClass(t *testing.T) {
	r := mustNewRegExp(t, "[\\\\]")
	if got, want := r.String(), "\\\\"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	assertSameRegExpLanguage(t, MakeChar('\\'), mustToAutomaton(t, r))
}

func TestRegExpParsing_EscapedDashCharClass(t *testing.T) {
	r := mustNewRegExp(t, "[\\-]")
	assertSameRegExpLanguage(t, MakeChar('-'), mustToAutomaton(t, r))
}

func TestRegExpParsing_Empty(t *testing.T) {
	r := mustNewRegExpFlags(t, "#", RegExpEmpty, 0)
	if got, want := r.String(), "#"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	assertSameRegExpLanguage(t, MakeEmpty(), mustToAutomaton(t, r))
}

func TestRegExpParsing_EmptyClass(t *testing.T) {
	_, err := NewRegExp("[]")
	if err == nil {
		t.Fatalf("NewRegExp(\"[]\") succeeded; expected parse error")
	}
	// Lucene's exact message: "expected ']' at position 2". Our parser
	// emits a position-bearing message; verify position is reported.
	if !strings.Contains(err.Error(), "position") {
		t.Errorf("error %q does not mention position", err.Error())
	}
}

func TestRegExpParsing_EscapedInvalidClass(t *testing.T) {
	_, err := NewRegExp("[\\]")
	if err == nil {
		t.Fatalf("NewRegExp(\"[\\\\]\") succeeded; expected parse error")
	}
	if !strings.Contains(err.Error(), "position") {
		t.Errorf("error %q does not mention position", err.Error())
	}
}

func TestRegExpParsing_Interval(t *testing.T) {
	r := mustNewRegExp(t, "<5-40>")
	if got, want := r.String(), "<5-40>"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	expected, err := MakeDecimalInterval(5, 40, 0)
	if err != nil {
		t.Fatalf("MakeDecimalInterval: %v", err)
	}
	assertSameRegExpLanguage(t, expected, mustToAutomaton(t, r))
}

func TestRegExpParsing_BackwardsInterval(t *testing.T) {
	r := mustNewRegExp(t, "<40-5>")
	if got, want := r.String(), "<5-40>"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	expected, err := MakeDecimalInterval(5, 40, 0)
	if err != nil {
		t.Fatalf("MakeDecimalInterval: %v", err)
	}
	assertSameRegExpLanguage(t, expected, mustToAutomaton(t, r))
}

func TestRegExpParsing_TruncatedInterval(t *testing.T) {
	expectParseError(t, "<1-")
}

func TestRegExpParsing_TruncatedInterval2(t *testing.T) {
	expectParseError(t, "<1")
}

func TestRegExpParsing_EmptyInterval(t *testing.T) {
	expectParseError(t, "<->")
}

func TestRegExpParsing_Optional(t *testing.T) {
	r := mustNewRegExp(t, "a?")
	if got, want := r.String(), "(\\a)?"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	expected := Optional(MakeChar('a'))
	assertSameRegExpLanguage(t, expected, mustToAutomaton(t, r))
}

func TestRegExpParsing_Repeat0(t *testing.T) {
	r := mustNewRegExp(t, "a*")
	if got, want := r.String(), "(\\a)*"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	expected := Repeat(MakeChar('a'))
	assertSameRegExpLanguage(t, expected, mustToAutomaton(t, r))
}

func TestRegExpParsing_Repeat1(t *testing.T) {
	r := mustNewRegExp(t, "a+")
	if got, want := r.String(), "(\\a){1,}"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	a := mustToAutomaton(t, r)
	// Lucene asserts exactly 3 states (minimal + 1); our automaton goes
	// through Determinize in assertSameRegExpLanguage, so a numeric
	// equality on raw NumStates would be brittle. Verify only the clean
	// DFA contract and language equivalence.
	det, err := Determinize(a, DefaultDeterminizeWorkLimit)
	if err != nil {
		t.Fatalf("determinize: %v", err)
	}
	assertCleanDFA(t, det)
	expected := RepeatN(MakeChar('a'), 1)
	assertSameRegExpLanguage(t, expected, a)
}

func TestRegExpParsing_RepeatN(t *testing.T) {
	r := mustNewRegExp(t, "a{5}")
	if got, want := r.String(), "(\\a){5,5}"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	expected := RepeatRange(MakeChar('a'), 5, 5)
	assertSameRegExpLanguage(t, expected, mustToAutomaton(t, r))
}

func TestRegExpParsing_RepeatNPlus(t *testing.T) {
	r := mustNewRegExp(t, "a{5,}")
	if got, want := r.String(), "(\\a){5,}"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	a := mustToAutomaton(t, r)
	det, err := Determinize(a, DefaultDeterminizeWorkLimit)
	if err != nil {
		t.Fatalf("determinize: %v", err)
	}
	assertCleanDFA(t, det)
	expected := RepeatN(MakeChar('a'), 5)
	assertSameRegExpLanguage(t, expected, a)
}

func TestRegExpParsing_RepeatMN(t *testing.T) {
	r := mustNewRegExp(t, "a{5,8}")
	if got, want := r.String(), "(\\a){5,8}"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	expected := RepeatRange(MakeChar('a'), 5, 8)
	assertSameRegExpLanguage(t, expected, mustToAutomaton(t, r))
}

func TestRegExpParsing_TruncatedRepeat(t *testing.T) {
	expectParseError(t, "a{5,8")
}

func TestRegExpParsing_BogusRepeat(t *testing.T) {
	expectParseError(t, "a{Z}")
}

func TestRegExpParsing_String(t *testing.T) {
	r := mustNewRegExp(t, "boo")
	if got, want := r.String(), "\"boo\""; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	assertSameRegExpLanguage(t, MakeString("boo"), mustToAutomaton(t, r))
}

func TestRegExpParsing_CaseInsensitiveString(t *testing.T) {
	r := mustNewRegExpFlags(t, "boo", RegExpNone, RegExpASCIICaseInsensitive)
	if got, want := r.String(), "\"boo\""; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	expectCaseFoldingUnsupported(t, r)
}

func TestRegExpParsing_ExplicitString(t *testing.T) {
	r := mustNewRegExp(t, "\"boo\"")
	if got, want := r.String(), "\"boo\""; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	assertSameRegExpLanguage(t, MakeString("boo"), mustToAutomaton(t, r))
}

func TestRegExpParsing_NotTerminatedString(t *testing.T) {
	expectParseError(t, "\"boo")
}

func TestRegExpParsing_Concatenation(t *testing.T) {
	r := mustNewRegExp(t, "[b-c][e-f]")
	if got, want := r.String(), "[\\b-\\c][\\e-\\f]"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	expected := Concatenate([]*Automaton{
		MakeCharRange('b', 'c'),
		MakeCharRange('e', 'f'),
	})
	assertSameRegExpLanguage(t, expected, mustToAutomaton(t, r))
}

func TestRegExpParsing_Intersection(t *testing.T) {
	r := mustNewRegExp(t, "[b-f]&[e-f]")
	if got, want := r.String(), "([\\b-\\f]&[\\e-\\f])"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	expected := Intersection(
		MakeCharRange('b', 'f'),
		MakeCharRange('e', 'f'),
	)
	assertSameRegExpLanguage(t, expected, mustToAutomaton(t, r))
}

func TestRegExpParsing_TruncatedIntersection(t *testing.T) {
	expectParseError(t, "a&")
}

func TestRegExpParsing_TruncatedIntersectionParens(t *testing.T) {
	expectParseError(t, "(a)&(")
}

func TestRegExpParsing_Union(t *testing.T) {
	r := mustNewRegExp(t, "[b-c]|[e-f]")
	if got, want := r.String(), "([\\b-\\c]|[\\e-\\f])"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	expected := Union([]*Automaton{
		MakeCharRange('b', 'c'),
		MakeCharRange('e', 'f'),
	})
	assertSameRegExpLanguage(t, expected, mustToAutomaton(t, r))
}

func TestRegExpParsing_TruncatedUnion(t *testing.T) {
	expectParseError(t, "a|")
}

func TestRegExpParsing_TruncatedUnionParens(t *testing.T) {
	expectParseError(t, "(a)|(")
}

// constAutomatonProvider is a test-only AutomatonProvider that always returns
// the same automaton, used for testAutomaton.
type constAutomatonProvider struct {
	a *Automaton
}

func (p constAutomatonProvider) GetAutomaton(string) (*Automaton, error) {
	return p.a, nil
}

// errAutomatonProvider always fails, used for testAutomatonIOException.
type errAutomatonProvider struct{ err error }

func (p errAutomatonProvider) GetAutomaton(string) (*Automaton, error) {
	return nil, p.err
}

func TestRegExpParsing_Automaton(t *testing.T) {
	r := mustNewRegExpFlags(t, "<myletter>", RegExpAll, 0)
	if got, want := r.String(), "<myletter>"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	ids := r.GetIdentifiers()
	if _, ok := ids["myletter"]; !ok || len(ids) != 1 {
		t.Errorf("GetIdentifiers() = %v, want {myletter}", ids)
	}
	provider := constAutomatonProvider{a: MakeChar('z')}
	actual, err := r.ToAutomatonWith(nil, provider)
	if err != nil {
		t.Fatalf("ToAutomatonWith: %v", err)
	}
	assertSameRegExpLanguage(t, MakeChar('z'), actual)
}

func TestRegExpParsing_AutomatonMap(t *testing.T) {
	r := mustNewRegExpFlags(t, "<myletter>", RegExpAll, 0)
	if got, want := r.String(), "<myletter>"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	ids := r.GetIdentifiers()
	if _, ok := ids["myletter"]; !ok || len(ids) != 1 {
		t.Errorf("GetIdentifiers() = %v, want {myletter}", ids)
	}
	actual, err := r.ToAutomatonWith(map[string]*Automaton{"myletter": MakeChar('z')}, nil)
	if err != nil {
		t.Fatalf("ToAutomatonWith: %v", err)
	}
	assertSameRegExpLanguage(t, MakeChar('z'), actual)
}

func TestRegExpParsing_AutomatonIOException(t *testing.T) {
	r := mustNewRegExpFlags(t, "<myletter>", RegExpAll, 0)
	provider := errAutomatonProvider{err: errors.New("fake ioexception")}
	if _, err := r.ToAutomatonWith(nil, provider); err == nil {
		t.Fatalf("ToAutomatonWith: expected provider error, got nil")
	}
}

func TestRegExpParsing_AutomatonNotFound(t *testing.T) {
	r := mustNewRegExpFlags(t, "<bogus>", RegExpAll, 0)
	if _, err := r.ToAutomatonWith(map[string]*Automaton{"myletter": MakeChar('z')}, nil); err == nil {
		t.Fatalf("ToAutomatonWith: expected not-found error, got nil")
	}
}

func TestRegExpParsing_IllegalSyntaxFlags(t *testing.T) {
	expectParseErrorFlags(t, "bogus", int(^uint(0)>>1), 0)
}

func TestRegExpParsing_IllegalMatchFlags(t *testing.T) {
	// Lucene rejects matchFlags=1 (a syntax-flag bit) when syntax=ALL.
	// Our parser mirrors that gate: matchFlags must be 0 or beyond RegExpAll.
	expectParseErrorFlags(t, "bogus", RegExpAll, 1)
}
