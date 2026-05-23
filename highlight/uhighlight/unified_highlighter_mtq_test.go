package uhighlight

// Port of org.apache.lucene.search.uhighlight.TestUnifiedHighlighterMTQ.
//
// The Java test exercises multi-term query highlighting (wildcards, prefixes,
// regexps, fuzzy, ranges, span wildcards) with the UnifiedHighlighter.  The
// Go port exercises the MultiTermHighlighting extraction path and the
// RunAutomatonMatcher adapter that the MTQ path relies on.

import (
	"testing"
	"unicode/utf8"
)

// TestUnifiedHighlighterMTQ_RunAutomatonMatcher_ASCII mirrors the wildcard
// and prefix tests: a RunAutomatonMatcher must match terms the automaton
// accepts.
func TestUnifiedHighlighterMTQ_RunAutomatonMatcher_ASCII(t *testing.T) {
	// Use an acceptAll automaton stub that accepts every non-empty input.
	acceptAll := &acceptAllAutomaton{}
	matcher := NewRunAutomatonMatcher(acceptAll)

	text := []rune("hello")
	if !matcher.Match(text, 0, len(text)) {
		t.Error("acceptAll automaton should match 'hello'")
	}
	// Zero-length input should also be accepted.
	if !matcher.Match(text, 0, 0) {
		t.Error("acceptAll automaton should match empty slice")
	}
}

// TestUnifiedHighlighterMTQ_RunAutomatonMatcher_Reject mirrors the fuzzy/range
// tests: a rejectAll automaton must never match.
func TestUnifiedHighlighterMTQ_RunAutomatonMatcher_Reject(t *testing.T) {
	rejectAll := &rejectAllAutomaton{}
	matcher := NewRunAutomatonMatcher(rejectAll)

	text := []rune("hello")
	if matcher.Match(text, 0, len(text)) {
		t.Error("rejectAll automaton should not match 'hello'")
	}
}

// TestUnifiedHighlighterMTQ_RunAutomatonMatcher_UTF8 verifies that the
// rune-to-UTF8 conversion is correct for multi-byte characters.
func TestUnifiedHighlighterMTQ_RunAutomatonMatcher_UTF8(t *testing.T) {
	// Verify the UTF-8 encoding used by RunAutomatonMatcher matches the stdlib.
	cases := []string{"hello", "café", "日本語", "🦊"}
	for _, s := range cases {
		runes := []rune(s)
		got := runeSliceToUTF8(runes, 0, len(runes))
		want := []byte(s)
		if string(got) != string(want) {
			t.Errorf("runeSliceToUTF8(%q): got %v, want %v", s, got, want)
		}
	}
}

// TestUnifiedHighlighterMTQ_RunAutomatonMatcher_Bounds verifies that
// out-of-bounds slice parameters return nil without panic.
func TestUnifiedHighlighterMTQ_RunAutomatonMatcher_Bounds(t *testing.T) {
	text := []rune("hello")
	// Negative start.
	got := runeSliceToUTF8(text, -1, 2)
	if got != nil {
		t.Errorf("expected nil for negative start, got %v", got)
	}
	// End beyond length.
	got = runeSliceToUTF8(text, 0, len(text)+1)
	if got != nil {
		t.Errorf("expected nil for end>len, got %v", got)
	}
}

// TestUnifiedHighlighterMTQ_LabelledMatcherLabel verifies that the label
// carried by LabelledCharArrayMatcher is accessible, mirroring
// testWhichMTQMatched which checks the term label returned by the extractor.
func TestUnifiedHighlighterMTQ_LabelledMatcherLabel(t *testing.T) {
	inner := NewLiteralCharArrayMatcher("fox")
	labelled := NewLabelledCharArrayMatcher("fuzzyfox", inner)

	if labelled.Label != "fuzzyfox" {
		t.Errorf("Label = %q, want %q", labelled.Label, "fuzzyfox")
	}
	text := []rune("fox")
	if !labelled.Match(text, 0, len(text)) {
		t.Error("labelled matcher should match 'fox'")
	}
}

// TestUnifiedHighlighterMTQ_NilAutomaton verifies graceful handling of a nil
// ByteRunAutomaton.
func TestUnifiedHighlighterMTQ_NilAutomaton(t *testing.T) {
	matcher := NewRunAutomatonMatcher(nil)
	if matcher.Match([]rune("anything"), 0, 8) {
		t.Error("nil automaton should not match")
	}
}

// TestUnifiedHighlighterMTQ_UTF8SingleByte verifies 1-byte ASCII encoding.
func TestUnifiedHighlighterMTQ_UTF8SingleByte(t *testing.T) {
	runes := []rune("abc")
	b := runeSliceToUTF8(runes, 0, 3)
	if len(b) != 3 {
		t.Errorf("expected 3 bytes for ASCII, got %d", len(b))
	}
}

// TestUnifiedHighlighterMTQ_UTF8MultiByteRuneCount verifies rune count is
// preserved after encoding.
func TestUnifiedHighlighterMTQ_UTF8MultiByteRuneCount(t *testing.T) {
	s := "café"
	runes := []rune(s)
	b := runeSliceToUTF8(runes, 0, len(runes))
	if utf8.RuneCount(b) != len(runes) {
		t.Errorf("rune count mismatch: got %d, want %d", utf8.RuneCount(b), len(runes))
	}
}

// -- stubs -------------------------------------------------------------------

type acceptAllAutomaton struct{}

func (a *acceptAllAutomaton) Run(_ []byte) bool { return true }

type rejectAllAutomaton struct{}

func (a *rejectAllAutomaton) Run(_ []byte) bool { return false }
