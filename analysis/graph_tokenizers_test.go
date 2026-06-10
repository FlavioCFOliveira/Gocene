// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/analysis/TestGraphTokenizers.java
//
// Deviation: eight tests (MockGraphTokenFilter* and DoubleMockGraphTokenFilter*)
// depend on MockGraphTokenFilter, MockHoleInjectingTokenFilter, MockTokenizer,
// and AutomatonTestUtil — none of which are ported to Gocene yet. Those tests
// remain as t.Fatal stubs. TestGraphTokenizers_ToDot depends on TokenStreamToDot,
// also not yet ported.
//
// The fifteen CannedTokenStream-based tests are fully implemented using the
// available analysis/testutil.CannedTokenStream, analysis.TokenStreamToAutomaton,
// and util/automaton operations.

package analysis_test

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/testutil"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// ---- helpers ---------------------------------------------------------------

// mkTok builds a testutil.Token with the given term, position increment, and
// position length, mirroring the Java token(term, posInc, posLength) helper.
// Start and end offsets are 0. Uses NewTokenWithPosIncAndLength to ensure
// that both positionIncrementSet and positionLengthSet are true, so
// CannedTokenStream does not fall back to the default values of 1.
func mkTok(term string, posInc, posLen int) testutil.Token {
	return testutil.NewTokenWithPosIncAndLength(term, posInc, 0, 0, posLen)
}

// mkTokOffset builds a testutil.Token with explicit start/end offsets in
// addition to posInc and posLength, mirroring the Java
// token(term, posInc, posLength, startOffset, endOffset) overload.
func mkTokOffset(term string, posInc, posLen, start, end int) testutil.Token {
	return testutil.NewTokenWithPosIncAndLength(term, posInc, start, end, posLen)
}

// sep is the POS_SEP automaton used between adjacent tokens.
var sep = automaton.MakeChar(analysis.PosSep)

// hole is the HOLE automaton used for missing positions.
var hole = automaton.MakeChar(analysis.Hole)

// s2a converts a plain string to a single-string automaton.
func s2a(s string) *automaton.Automaton {
	return automaton.MakeString(s)
}

// joinStr concatenates automata for the strings with POS_SEP between them.
func joinStr(strings ...string) *automaton.Automaton {
	parts := make([]*automaton.Automaton, 0, 2*len(strings)-1)
	for i, s := range strings {
		if i > 0 {
			parts = append(parts, sep)
		}
		parts = append(parts, s2a(s))
	}
	return automaton.Concatenate(parts)
}

// joinA concatenates the given automata directly (no separators added).
func joinA(as ...*automaton.Automaton) *automaton.Automaton {
	return automaton.Concatenate(as)
}

// assertSameLanguage verifies that the automaton produced from ts by
// TokenStreamToAutomaton accepts the same language as expected.
func assertSameLanguage(t *testing.T, expected *automaton.Automaton, ts analysis.TokenStream) {
	t.Helper()

	conv := analysis.NewTokenStreamToAutomaton()
	actual, err := conv.ToAutomaton(ts)
	if err != nil {
		t.Fatalf("ToAutomaton: %v", err)
	}

	const workLimit = 10_000

	expectedDet, err := automaton.Determinize(automaton.RemoveDeadStates(expected), workLimit)
	if err != nil {
		t.Fatalf("Determinize(expected): %v", err)
	}
	actualDet, err := automaton.Determinize(automaton.RemoveDeadStates(actual), workLimit)
	if err != nil {
		t.Fatalf("Determinize(actual): %v", err)
	}

	same, err := automaton.SameLanguage(expectedDet, actualDet, workLimit)
	if err != nil {
		t.Fatalf("SameLanguage: %v", err)
	}
	if !same {
		t.Error("accepted language differs between expected and actual automaton")
	}
}

// ---- CannedTokenStream-based tests -----------------------------------------

// TestGraphTokenizers_SingleToken mirrors testSingleToken.
func TestGraphTokenizers_SingleToken(t *testing.T) {
	ts := testutil.NewCannedTokenStream(mkTok("abc", 1, 1))
	assertSameLanguage(t, s2a("abc"), ts)
}

// TestGraphTokenizers_MultipleHoles mirrors testMultipleHoles.
func TestGraphTokenizers_MultipleHoles(t *testing.T) {
	ts := testutil.NewCannedTokenStream(
		mkTok("a", 1, 1),
		mkTok("b", 3, 1),
	)
	expected := joinA(s2a("a"), sep, hole, sep, hole, sep, s2a("b"))
	assertSameLanguage(t, expected, ts)
}

// TestGraphTokenizers_SynOverMultipleHoles mirrors testSynOverMultipleHoles.
func TestGraphTokenizers_SynOverMultipleHoles(t *testing.T) {
	ts := testutil.NewCannedTokenStream(
		mkTok("a", 1, 1),
		mkTok("x", 0, 3),
		mkTok("b", 3, 1),
	)
	a1 := joinA(s2a("a"), sep, hole, sep, hole, sep, s2a("b"))
	a2 := joinA(s2a("x"), sep, s2a("b"))
	assertSameLanguage(t, automaton.Union([]*automaton.Automaton{a1, a2}), ts)
}

// TestGraphTokenizers_TwoTokens mirrors testTwoTokens.
func TestGraphTokenizers_TwoTokens(t *testing.T) {
	ts := testutil.NewCannedTokenStream(
		mkTok("abc", 1, 1),
		mkTok("def", 1, 1),
	)
	assertSameLanguage(t, joinStr("abc", "def"), ts)
}

// TestGraphTokenizers_Hole mirrors testHole.
func TestGraphTokenizers_Hole(t *testing.T) {
	ts := testutil.NewCannedTokenStream(
		mkTok("abc", 1, 1),
		mkTok("def", 2, 1),
	)
	expected := joinA(s2a("abc"), sep, hole, sep, s2a("def"))
	assertSameLanguage(t, expected, ts)
}

// TestGraphTokenizers_OverlappedTokensSausage mirrors testOverlappedTokensSausage.
func TestGraphTokenizers_OverlappedTokensSausage(t *testing.T) {
	ts := testutil.NewCannedTokenStream(
		mkTok("abc", 1, 1),
		mkTok("xyz", 0, 1),
	)
	assertSameLanguage(t, automaton.Union([]*automaton.Automaton{s2a("abc"), s2a("xyz")}), ts)
}

// TestGraphTokenizers_OverlappedTokensLattice mirrors testOverlappedTokensLattice.
func TestGraphTokenizers_OverlappedTokensLattice(t *testing.T) {
	ts := testutil.NewCannedTokenStream(
		mkTok("abc", 1, 1),
		mkTok("xyz", 0, 2),
		mkTok("def", 1, 1),
	)
	a1 := s2a("xyz")
	a2 := joinStr("abc", "def")
	assertSameLanguage(t, automaton.Union([]*automaton.Automaton{a1, a2}), ts)
}

// TestGraphTokenizers_SynOverHole mirrors testSynOverHole.
func TestGraphTokenizers_SynOverHole(t *testing.T) {
	ts := testutil.NewCannedTokenStream(
		mkTok("a", 1, 1),
		mkTok("X", 0, 2),
		mkTok("b", 2, 1),
	)
	a1 := automaton.Union([]*automaton.Automaton{
		joinA(s2a("a"), sep, hole),
		s2a("X"),
	})
	expected := joinA(a1, sep, s2a("b"))
	assertSameLanguage(t, expected, ts)
}

// TestGraphTokenizers_SynOverHole2 mirrors testSynOverHole2.
func TestGraphTokenizers_SynOverHole2(t *testing.T) {
	ts := testutil.NewCannedTokenStream(
		mkTok("xyz", 1, 1),
		mkTok("abc", 0, 3),
		mkTok("def", 2, 1),
	)
	expected := automaton.Union([]*automaton.Automaton{
		joinA(s2a("xyz"), sep, hole, sep, s2a("def")),
		s2a("abc"),
	})
	assertSameLanguage(t, expected, ts)
}

// TestGraphTokenizers_OverlappedTokensLattice2 mirrors testOverlappedTokensLattice2.
func TestGraphTokenizers_OverlappedTokensLattice2(t *testing.T) {
	ts := testutil.NewCannedTokenStream(
		mkTok("abc", 1, 1),
		mkTok("xyz", 0, 3),
		mkTok("def", 1, 1),
		mkTok("ghi", 1, 1),
	)
	a1 := s2a("xyz")
	a2 := joinStr("abc", "def", "ghi")
	assertSameLanguage(t, automaton.Union([]*automaton.Automaton{a1, a2}), ts)
}

// TestGraphTokenizers_StartsWithHole mirrors testStartsWithHole.
func TestGraphTokenizers_StartsWithHole(t *testing.T) {
	ts := testutil.NewCannedTokenStream(mkTok("abc", 2, 1))
	expected := joinA(hole, sep, s2a("abc"))
	assertSameLanguage(t, expected, ts)
}

// TestGraphTokenizers_EndsWithHole mirrors testEndsWithHole.
// The Java test uses CannedTokenStream(finalPosInc=1, finalOffset=0, tokens).
func TestGraphTokenizers_EndsWithHole(t *testing.T) {
	ts := testutil.NewCannedTokenStreamWithFinal(1, 0, mkTok("abc", 2, 1))
	expected := joinA(hole, sep, s2a("abc"), sep, hole)
	assertSameLanguage(t, expected, ts)
}

// TestGraphTokenizers_SynHangingOverEnd mirrors testSynHangingOverEnd.
func TestGraphTokenizers_SynHangingOverEnd(t *testing.T) {
	ts := testutil.NewCannedTokenStream(
		mkTok("a", 1, 1),
		mkTok("X", 0, 10),
	)
	assertSameLanguage(t, automaton.Union([]*automaton.Automaton{s2a("a"), s2a("X")}), ts)
}

// TestGraphTokenizers_TokenStreamGraphWithHoles mirrors testTokenStreamGraphWithHoles.
func TestGraphTokenizers_TokenStreamGraphWithHoles(t *testing.T) {
	ts := testutil.NewCannedTokenStream(
		mkTok("abc", 1, 1),
		mkTok("xyz", 1, 8),
		mkTok("def", 1, 1),
		mkTok("ghi", 1, 1),
	)
	expected := automaton.Union([]*automaton.Automaton{
		joinA(s2a("abc"), sep, s2a("xyz")),
		joinA(s2a("abc"), sep, hole, sep, s2a("def"), sep, s2a("ghi")),
	})
	assertSameLanguage(t, expected, ts)
}

// ---- ToDot test (TokenStreamToDot not yet ported) --------------------------

// TestGraphTokenizers_ToDot mirrors testToDot (Lucene 10.4.0).
// It depends on TokenStreamToDot which is not yet ported to Gocene.
func TestGraphTokenizers_ToDot(t *testing.T) {
	// Port of TestGraphTokenizers.testToDot — verifies that TokenStreamToDot
	// produces a non-empty DOT graph for a simple input.
	analyzer := analysis.NewWhitespaceAnalyzer()
	stream, err := analyzer.TokenStream("f", strings.NewReader("hello world"))
	if err != nil {
		t.Fatalf("TokenStream: %v", err)
	}
	defer stream.Close()

	var buf strings.Builder
	dot := analysis.NewTokenStreamToDot("hello world", stream, &buf)
	if err := dot.ToDot(); err != nil {
		t.Fatalf("ToDot: %v", err)
	}

	dotOutput := buf.String()
	if !strings.Contains(dotOutput, "digraph tokens") {
		t.Errorf("expected digraph header, got:\n%s", dotOutput)
	}
	if dotOutput == "" {
		t.Error("expected non-empty DOT output")
	}
}

// ---- MockGraphTokenFilter tests (infrastructure not yet ported) ------------

// TestGraphTokenizers_MockGraphTokenFilterBasic mirrors testMockGraphTokenFilterBasic.
func TestGraphTokenizers_MockGraphTokenFilterBasic(t *testing.T) {
	for iter := 0; iter < 10; iter++ {
		// Build two independent streams with the same seed and input.
		seed := rand.NewSource(int64(iter))
		tz1 := analysis.NewWhitespaceTokenizer()
		tz1.SetReader(strings.NewReader("a b c d e f g h i j k"))
		mgf1 := analysis.NewMockGraphTokenFilter(rand.New(seed), tz1)
		if err := mgf1.Reset(); err != nil {
			t.Fatalf("iter %d: Reset error (mgf1): %v", iter, err)
		}

		var tokens1 []string
		for {
			hasToken, err := mgf1.IncrementToken()
			if err != nil {
				t.Fatalf("iter %d: IncrementToken error: %v", iter, err)
			}
			if !hasToken {
				break
			}
			if termAtt, ok := mgf1.GetAttributeSource().GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute); ok && termAtt != nil {
				tokens1 = append(tokens1, termAtt.String())
			}
		}

		seed2 := rand.NewSource(int64(iter))
		tz2 := analysis.NewWhitespaceTokenizer()
		tz2.SetReader(strings.NewReader("a b c d e f g h i j k"))
		mgf2 := analysis.NewMockGraphTokenFilter(rand.New(seed2), tz2)
		if err := mgf2.Reset(); err != nil {
			t.Fatalf("iter %d: Reset error (mgf2): %v", iter, err)
		}
		var tokens2 []string
		for {
			hasToken, err := mgf2.IncrementToken()
			if err != nil {
				t.Fatalf("iter %d: IncrementToken error (second stream): %v", iter, err)
			}
			if !hasToken {
				break
			}
			if termAtt, ok := mgf2.GetAttributeSource().GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute); ok && termAtt != nil {
				tokens2 = append(tokens2, termAtt.String())
			}
		}

		if len(tokens1) != len(tokens2) {
			t.Fatalf("iter %d: token count mismatch: %d vs %d", iter, len(tokens1), len(tokens2))
		}
		for i := range tokens1 {
			if tokens1[i] != tokens2[i] {
				t.Fatalf("iter %d: token mismatch at %d: %q vs %q", iter, i, tokens1[i], tokens2[i])
			}
		}
	}
}

// TestGraphTokenizers_MockGraphTokenFilterOnGraphInput mirrors testMockGraphTokenFilterOnGraphInput.
func TestGraphTokenizers_MockGraphTokenFilterOnGraphInput(t *testing.T) {
	// Build a graph input: "a" (posLen=2), "b", "c".
	tokens := []testutil.Token{
		testutil.NewTokenWithPosIncAndLength("a", 1, 0, 1, 2),
		testutil.NewTokenWithPosIncAndLength("b", 1, 1, 2, 1),
		testutil.NewTokenWithPosIncAndLength("c", 0, 1, 2, 1),
	}

	// Run two independent streams with the same seed and input.
	cts1 := testutil.NewCannedTokenStream(tokens...)
	mgf1 := analysis.NewMockGraphTokenFilter(rand.New(rand.NewSource(42)), cts1)
	if err := mgf1.Reset(); err != nil {
		t.Fatalf("Reset error (mgf1): %v", err)
	}
	var out1 []testutil.Token
	for {
		hasToken, err := mgf1.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken error: %v", err)
		}
		if !hasToken {
			break
		}
		out1 = append(out1, tokenFromSource(mgf1.GetAttributeSource()))
	}

	cts2 := testutil.NewCannedTokenStream(tokens...)
	mgf2 := analysis.NewMockGraphTokenFilter(rand.New(rand.NewSource(42)), cts2)
	if err := mgf2.Reset(); err != nil {
		t.Fatalf("Reset error (mgf2): %v", err)
	}
	var out2 []testutil.Token
	for {
		hasToken, err := mgf2.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken error (second stream): %v", err)
		}
		if !hasToken {
			break
		}
		out2 = append(out2, tokenFromSource(mgf2.GetAttributeSource()))
	}

	if len(out1) != len(out2) {
		t.Fatalf("token count mismatch: %d vs %d", len(out1), len(out2))
	}
	for i := range out1 {
		if !tokenEqual(out1[i], out2[i]) {
			t.Fatalf("token mismatch at %d: %+v vs %+v", i, out1[i], out2[i])
		}
	}
}

// tokenFromSource extracts a testutil.Token from an AttributeSource.
func tokenFromSource(src *util.AttributeSource) testutil.Token {
	var tok testutil.Token
	if termAtt, ok := src.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute); ok && termAtt != nil {
		tok.Text = termAtt.String()
	}
	if off, ok := src.GetAttribute(analysis.OffsetAttributeType).(analysis.OffsetAttribute); ok && off != nil {
		tok.StartOffset = off.StartOffset()
		tok.EndOffset = off.EndOffset()
	}
	if pi, ok := src.GetAttribute(analysis.PositionIncrementAttributeType).(analysis.PositionIncrementAttribute); ok && pi != nil {
		tok.PositionIncrement = pi.GetPositionIncrement()
	}
	if pl, ok := src.GetAttribute(analysis.PositionLengthAttributeType).(analysis.PositionLengthAttribute); ok && pl != nil {
		tok.PositionLength = pl.GetPositionLength()
	}
	return tok
}

// tokenEqual compares two testutil.Tokens for equality.
func tokenEqual(a, b testutil.Token) bool {
	return a.Text == b.Text && a.StartOffset == b.StartOffset && a.EndOffset == b.EndOffset &&
		a.PositionIncrement == b.PositionIncrement && a.PositionLength == b.PositionLength
}

// TestGraphTokenizers_MockGraphTokenFilterBeforeHoles mirrors testMockGraphTokenFilterBeforeHoles.
// Requires MockGraphTokenFilter and MockTokenizer (not yet ported to Gocene).
func TestGraphTokenizers_MockGraphTokenFilterBeforeHoles(t *testing.T) {
	t.Fatal("requires MockGraphTokenFilter/MockTokenizer infrastructure (not yet ported to Gocene)")
}

// TestGraphTokenizers_MockGraphTokenFilterAfterHoles mirrors testMockGraphTokenFilterAfterHoles.
// Requires MockGraphTokenFilter and MockTokenizer (not yet ported to Gocene).
func TestGraphTokenizers_MockGraphTokenFilterAfterHoles(t *testing.T) {
	t.Fatal("requires MockGraphTokenFilter/MockTokenizer infrastructure (not yet ported to Gocene)")
}

// TestGraphTokenizers_MockGraphTokenFilterRandom mirrors testMockGraphTokenFilterRandom.
// Requires MockGraphTokenFilter and MockTokenizer (not yet ported to Gocene).
func TestGraphTokenizers_MockGraphTokenFilterRandom(t *testing.T) {
	t.Fatal("requires MockGraphTokenFilter/MockTokenizer infrastructure (not yet ported to Gocene)")
}

// TestGraphTokenizers_DoubleMockGraphTokenFilterRandom mirrors testDoubleMockGraphTokenFilterRandom.
// Requires MockGraphTokenFilter and MockTokenizer (not yet ported to Gocene).
func TestGraphTokenizers_DoubleMockGraphTokenFilterRandom(t *testing.T) {
	t.Fatal("requires MockGraphTokenFilter/MockTokenizer infrastructure (not yet ported to Gocene)")
}

// TestGraphTokenizers_MockGraphTokenFilterBeforeHolesRandom mirrors testMockGraphTokenFilterBeforeHolesRandom.
// Requires MockGraphTokenFilter, MockHoleInjectingTokenFilter, and MockTokenizer.
func TestGraphTokenizers_MockGraphTokenFilterBeforeHolesRandom(t *testing.T) {
	t.Fatal("requires MockGraphTokenFilter/MockHoleInjectingTokenFilter/MockTokenizer infrastructure (not yet ported to Gocene)")
}

// TestGraphTokenizers_MockGraphTokenFilterAfterHolesRandom mirrors testMockGraphTokenFilterAfterHolesRandom.
// Requires MockGraphTokenFilter, MockHoleInjectingTokenFilter, and MockTokenizer.
func TestGraphTokenizers_MockGraphTokenFilterAfterHolesRandom(t *testing.T) {
	t.Fatal("requires MockGraphTokenFilter/MockHoleInjectingTokenFilter/MockTokenizer infrastructure (not yet ported to Gocene)")
}

// mkTokOffset is used by TestGraphTokenizers_ToDot but retained here so the
// symbol is not orphaned when the ToDot test is eventually implemented.
var _ = mkTokOffset
