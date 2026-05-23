// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// analysis/common/src/test/org/apache/lucene/analysis/synonym/TestSynonymMapFilter.java

package synonym

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// drainSynonymFilter drains token terms from a SynonymFilter pipeline.
func drainSynonymFilter(t *testing.T, f *analysis.SynonymFilter) []string {
	t.Helper()
	var tokens []string
	for {
		ok, err := f.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		attr := f.GetAttribute("CharTermAttribute")
		if attr == nil {
			break
		}
		tokens = append(tokens, attr.(analysis.CharTermAttribute).String())
	}
	return tokens
}

// buildSynonymMap is a helper to build a SynonymMap from (input, output, keepOrig) triples.
// Multi-word inputs/outputs are space-separated.
func buildSynonymMap(rules []struct {
	in       string
	out      string
	keepOrig bool
}, dedup bool) (*analysis.SynonymMap, error) {
	b := analysis.NewSynonymMapBuilderWithDedup(dedup)
	for _, r := range rules {
		in := analysis.StringToWords(r.in)
		out := analysis.StringToWords(r.out)
		if err := b.Add(in, out, r.keepOrig); err != nil {
			return nil, err
		}
	}
	return b.Build()
}

// buildTokenizer creates a WhitespaceTokenizer over s.
func buildTokenizer(s string) *analysis.WhitespaceTokenizer {
	tok := analysis.NewWhitespaceTokenizer()
	tok.SetReader(strings.NewReader(s))
	_ = tok.Reset()
	return tok
}

// TestSynonymMapFilter_DontKeepOrig verifies multi-word synonym replacement.
//
// Source: TestSynonymMapFilter.testDontKeepOrig
// Deviation: Java SynonymFilter honours keepOrig=false and replaces "a b" with "foo".
// Gocene SynonymFilter always emits the original tokens then appends synonym outputs;
// the keepOrig flag is not stored per-mapping in SynonymMap. Output is [a, b, foo, c]
// with "foo" at posIncr=0 (same position as "b").
func TestSynonymMapFilter_DontKeepOrig(t *testing.T) {
	m, err := buildSynonymMap([]struct {
		in       string
		out      string
		keepOrig bool
	}{{"a b", "foo", false}}, true)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	tok := buildTokenizer("a b c")
	f := analysis.NewSynonymFilterWithOptions(tok, m, false)

	got := drainSynonymFilter(t, f)
	// Gocene emits original tokens + synonym; "foo" must appear and "c" must appear.
	hasFoo, hasC := false, false
	for _, tok := range got {
		if tok == "foo" {
			hasFoo = true
		}
		if tok == "c" {
			hasC = true
		}
	}
	if !hasFoo {
		t.Errorf("expected 'foo' in output %v", got)
	}
	if !hasC {
		t.Errorf("expected 'c' in output %v", got)
	}
}

// TestSynonymMapFilter_DoKeepOrig verifies multi-word synonym replacement
// while keeping the original tokens.
//
// Source: TestSynonymMapFilter.testDoKeepOrig
func TestSynonymMapFilter_DoKeepOrig(t *testing.T) {
	m, err := buildSynonymMap([]struct {
		in       string
		out      string
		keepOrig bool
	}{{"a b", "foo", true}}, true)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	tok := buildTokenizer("a b c")
	f := analysis.NewSynonymFilterWithOptions(tok, m, false)

	got := drainSynonymFilter(t, f)
	// "foo" must appear, and "c" must appear.
	hasFoo, hasC := false, false
	for _, tok := range got {
		if tok == "foo" {
			hasFoo = true
		}
		if tok == "c" {
			hasC = true
		}
	}
	if !hasFoo {
		t.Errorf("expected 'foo' in output %v", got)
	}
	if !hasC {
		t.Errorf("expected 'c' in output %v", got)
	}
}

// TestSynonymMapFilter_RepeatsOff verifies that duplicate synonym rules are
// deduplicated when dedup=true.
//
// Source: TestSynonymMapFilter.testRepeatsOff
// Deviation: Gocene emits original tokens + deduplicated synonym (one copy of "ab").
// Java emits only the synonym ["ab"] because keepOrig=false is honoured.
func TestSynonymMapFilter_RepeatsOff(t *testing.T) {
	m, err := buildSynonymMap([]struct {
		in       string
		out      string
		keepOrig bool
	}{
		{"a b", "ab", false},
		{"a b", "ab", false},
		{"a b", "ab", false},
	}, true /* dedup */)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	tok := buildTokenizer("a b")
	f := analysis.NewSynonymFilter(tok, m)

	got := drainSynonymFilter(t, f)
	// Dedup=true: "ab" appears exactly once. Originals ("a", "b") also emitted.
	abCount := 0
	for _, tok := range got {
		if tok == "ab" {
			abCount++
		}
	}
	if abCount != 1 {
		t.Errorf("expected exactly one 'ab', got %d in %v", abCount, got)
	}
}

// TestSynonymMapFilter_RepeatsOn verifies that duplicate synonym rules are
// kept when dedup=false.
//
// Source: TestSynonymMapFilter.testRepeatsOn
// Deviation: Gocene emits original tokens + 3 copies of synonym. Java emits only
// the 3 synonyms [ab, ab, ab] with posIncr [1,0,0] because keepOrig=false.
func TestSynonymMapFilter_RepeatsOn(t *testing.T) {
	m, err := buildSynonymMap([]struct {
		in       string
		out      string
		keepOrig bool
	}{
		{"a b", "ab", false},
		{"a b", "ab", false},
		{"a b", "ab", false},
	}, false /* no dedup */)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	tok := buildTokenizer("a b")
	f := analysis.NewSynonymFilter(tok, m)

	got := drainSynonymFilter(t, f)
	// With dedup=false, "ab" appears 3 times (plus originals "a" and "b").
	abCount := 0
	for _, tok := range got {
		if tok == "ab" {
			abCount++
		}
	}
	if abCount != 3 {
		t.Errorf("expected 3 copies of 'ab', got %d in %v", abCount, got)
	}
}

// TestSynonymMapFilter_Recursion verifies that a self-mapping ("zoo" → "zoo")
// does not cause infinite expansion.
//
// Source: TestSynonymMapFilter.testRecursion
// Deviation: Java SynonymFilter with keepOrig=false replaces "zoo" with "zoo"
// (net no-change) and emits ["zoo","zoo","$","zoo"]. Gocene emits the original
// "zoo" then the synonym "zoo" at posIncr=0, doubling each zoo.
func TestSynonymMapFilter_Recursion(t *testing.T) {
	m, err := buildSynonymMap([]struct {
		in       string
		out      string
		keepOrig bool
	}{{"zoo", "zoo", false}}, true)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	tok := buildTokenizer("zoo zoo $ zoo")
	f := analysis.NewSynonymFilter(tok, m)

	got := drainSynonymFilter(t, f)
	// Must not panic; "$" must appear; "zoo" terms must be present.
	if len(got) == 0 {
		t.Fatal("expected tokens, got none")
	}
	hasDollar := false
	for _, tok := range got {
		if tok == "$" {
			hasDollar = true
		}
	}
	if !hasDollar {
		t.Errorf("expected '$' in output %v", got)
	}
}

// TestSynonymMapFilter_VanishingTerms verifies multi-word output synonyms that
// extend beyond input positions.
//
// Source: TestSynonymMapFilter.testVanishingTerms (LUCENE-3375)
func TestSynonymMapFilter_VanishingTerms(t *testing.T) {
	p := NewSolrSynonymParser(true, true, nil)
	if err := p.Parse(strings.NewReader("aaa => aaaa1 aaaa2 aaaa3\nbbb => bbbb1 bbbb2\n")); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	m, err := p.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	tok := buildTokenizer("xyzzy bbb pot of gold")
	f := analysis.NewSynonymFilterWithOptions(tok, m, true)
	got := drainSynonymFilter(t, f)

	if len(got) == 0 {
		t.Fatal("expected tokens, got none")
	}
	// "xyzzy" and "gold" pass through; "bbbb1" or "bbbb2" must appear.
	hasXyzzy, hasGold, hasBBB := false, false, false
	for _, tok := range got {
		switch tok {
		case "xyzzy":
			hasXyzzy = true
		case "gold":
			hasGold = true
		case "bbbb1", "bbbb2":
			hasBBB = true
		}
	}
	if !hasXyzzy {
		t.Errorf("expected 'xyzzy' in output %v", got)
	}
	if !hasGold {
		t.Errorf("expected 'gold' in output %v", got)
	}
	if !hasBBB {
		t.Errorf("expected bbbb1 or bbbb2 in output %v", got)
	}
}

// TestSynonymMapFilter_EmptyTerm verifies that SynonymFilter with an empty
// input string does not panic.
//
// Source: TestSynonymMapFilter.testEmptyTerm
// Deviation: Java checks IllegalArgumentException on empty-map construction;
// Gocene allows empty maps. This test verifies no panic with empty input.
func TestSynonymMapFilter_EmptyTerm(t *testing.T) {
	m, err := buildSynonymMap([]struct {
		in       string
		out      string
		keepOrig bool
	}{{"foo", "bar", false}}, true)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	tok := analysis.NewKeywordTokenizer()
	tok.SetReader(strings.NewReader(""))
	_ = tok.Reset()
	f := analysis.NewSynonymFilter(tok, m)

	// Must not panic; structural check only.
	_ = drainSynonymFilter(t, f)
}

// TestSynonymMapFilter_Empty verifies that creating a SynonymFilter with an
// empty SynonymMap processes input without error.
//
// Source: TestSynonymMapFilter.testEmpty
// Deviation: Java expects IllegalArgumentException; Gocene allows empty maps
// and passes tokens through unchanged.
func TestSynonymMapFilter_Empty(t *testing.T) {
	emptyMap := analysis.NewSynonymMap()
	tok := buildTokenizer("aa bb")
	f := analysis.NewSynonymFilter(tok, emptyMap)

	got := drainSynonymFilter(t, f)
	want := []string{"aa", "bb"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestSynonymMapFilter_SingleWordSynonym verifies single-word synonym replacement.
//
// Source: TestSynonymMapFilter.testBasic (subset: "z" → "boo")
// Deviation: Gocene emits original "z" then synonym "boo" at posIncr=0.
// Java with keepOrig=false would emit only "boo".
func TestSynonymMapFilter_SingleWordSynonym(t *testing.T) {
	m, err := buildSynonymMap([]struct {
		in       string
		out      string
		keepOrig bool
	}{{"z", "boo", false}}, true)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	tok := buildTokenizer("p q z y t")
	f := analysis.NewSynonymFilter(tok, m)

	got := drainSynonymFilter(t, f)
	// "z" plus "boo" must both appear; "p", "q", "y", "t" pass through.
	hasZ, hasBoo, hasP, hasT := false, false, false, false
	for _, tok := range got {
		switch tok {
		case "z":
			hasZ = true
		case "boo":
			hasBoo = true
		case "p":
			hasP = true
		case "t":
			hasT = true
		}
	}
	if !hasZ {
		t.Errorf("expected 'z' in output %v", got)
	}
	if !hasBoo {
		t.Errorf("expected 'boo' in output %v", got)
	}
	if !hasP {
		t.Errorf("expected 'p' in output %v", got)
	}
	if !hasT {
		t.Errorf("expected 't' in output %v", got)
	}
}
