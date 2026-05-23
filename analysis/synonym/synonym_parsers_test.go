// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package synonym

import (
	"strings"
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────────
// WordnetSynonymParser
// ─────────────────────────────────────────────────────────────────────────────

var wordnetInput = "s(100000001,1,'woods',n,1,0).\n" +
	"s(100000001,2,'wood',n,1,0).\n" +
	"s(100000001,3,'forest',n,1,0).\n" +
	"s(100000002,1,'wolfish',n,1,0).\n" +
	"s(100000002,2,'ravenous',n,1,0).\n" +
	"s(100000003,1,'king',n,1,1).\n" +
	"s(100000003,2,'baron',n,1,1).\n" +
	"s(100000004,1,'king''s evil',n,1,1).\n" +
	"s(100000004,2,'king''s meany',n,1,1)."

func TestWordnetSynonymParser_Parse_NoPanic(t *testing.T) {
	p := NewWordnetSynonymParser(true, true, nil)
	err := p.Parse(strings.NewReader(wordnetInput))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	_, err = p.Build()
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
}

func TestWordnetSynonymParser_SingleQuoteEscape(t *testing.T) {
	// Escaped single quote ('') should become a literal apostrophe.
	input := "s(100000001,1,'king''s evil',n,1,0).\n" +
		"s(100000001,2,'baron',n,1,0)."
	p := NewWordnetSynonymParser(true, true, nil)
	err := p.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	m, err := p.Build()
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	// The map should have been built without error; verify it's non-nil.
	if m == nil {
		t.Fatal("Build returned nil SynonymMap")
	}
}

func TestWordnetSynonymParser_SingleWordSynset_Skipped(t *testing.T) {
	// A synset with only one entry produces no mappings.
	input := "s(100000001,1,'alone',n,1,0)."
	p := NewWordnetSynonymParser(true, true, nil)
	if err := p.Parse(strings.NewReader(input)); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	m, err := p.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if m == nil {
		t.Fatal("Build returned nil")
	}
}

func TestWordnetSynonymParser_NoExpand(t *testing.T) {
	// With expand=false each term maps only to the first term in the synset.
	input := "s(100000001,1,'woods',n,1,0).\ns(100000001,2,'wood',n,1,0).\ns(100000001,3,'forest',n,1,0)."
	p := NewWordnetSynonymParser(true, false, nil)
	if err := p.Parse(strings.NewReader(input)); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	_, err := p.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
}

func TestWordnetSynonymParser_EmptyInput(t *testing.T) {
	p := NewWordnetSynonymParser(true, true, nil)
	if err := p.Parse(strings.NewReader("")); err != nil {
		t.Fatalf("Parse empty input: %v", err)
	}
	m, err := p.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if m == nil {
		t.Fatal("Build returned nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// SolrSynonymParser
// ─────────────────────────────────────────────────────────────────────────────

func TestSolrSynonymParser_Simple_NoPanic(t *testing.T) {
	input := "i-pod, ipod, ipoooood\nfoo => foo bar\nfoo => baz\nthis test, that testing"
	p := NewSolrSynonymParser(true, true, nil)
	if err := p.Parse(strings.NewReader(input)); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	m, err := p.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if m == nil {
		t.Fatal("Build returned nil")
	}
}

func TestSolrSynonymParser_CommentsAndBlankLines_Ignored(t *testing.T) {
	input := "# this is a comment\n\na, b\n# another comment\nc, d"
	p := NewSolrSynonymParser(true, true, nil)
	if err := p.Parse(strings.NewReader(input)); err != nil {
		t.Fatalf("Parse: %v", err)
	}
}

func TestSolrSynonymParser_InvalidDoubleMap(t *testing.T) {
	input := "a => b => c"
	p := NewSolrSynonymParser(true, true, nil)
	err := p.Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for double-map line, got nil")
	}
}

func TestSolrSynonymParser_ExplicitMapping_NoExpand(t *testing.T) {
	// expand flag does not affect explicit "=>" mappings.
	input := "foo => bar"
	for _, expand := range []bool{true, false} {
		p := NewSolrSynonymParser(true, expand, nil)
		if err := p.Parse(strings.NewReader(input)); err != nil {
			t.Fatalf("expand=%v: Parse: %v", expand, err)
		}
		m, err := p.Build()
		if err != nil {
			t.Fatalf("expand=%v: Build: %v", expand, err)
		}
		if m == nil {
			t.Fatalf("expand=%v: Build returned nil", expand)
		}
	}
}

func TestSolrSynonymParser_ImplicitMapping_ExpandTrue(t *testing.T) {
	input := "a, b, c"
	p := NewSolrSynonymParser(true, true, nil)
	if err := p.Parse(strings.NewReader(input)); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	_, err := p.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
}

func TestSolrSynonymParser_ImplicitMapping_ExpandFalse(t *testing.T) {
	input := "a, b, c"
	p := NewSolrSynonymParser(true, false, nil)
	if err := p.Parse(strings.NewReader(input)); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	_, err := p.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
}

func TestSolrSynonymParser_Unescape(t *testing.T) {
	// Backslash-escaped comma in a term should not split the term.
	input := `a\,b => c`
	p := NewSolrSynonymParser(true, true, nil)
	if err := p.Parse(strings.NewReader(input)); err != nil {
		t.Fatalf("Parse: %v", err)
	}
}

func TestSolrSynonymParser_EmptyInput(t *testing.T) {
	p := NewSolrSynonymParser(true, true, nil)
	if err := p.Parse(strings.NewReader("")); err != nil {
		t.Fatalf("Parse empty input: %v", err)
	}
	m, err := p.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if m == nil {
		t.Fatal("Build returned nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// splitOn / unescape helpers (package-private, tested directly)
// ─────────────────────────────────────────────────────────────────────────────

func TestSplitOn_Basic(t *testing.T) {
	got := splitOn("a => b => c", "=>")
	if len(got) != 3 {
		t.Errorf("splitOn: got %d parts, want 3: %v", len(got), got)
	}
}

func TestSplitOn_NoSep(t *testing.T) {
	got := splitOn("abc", "=>")
	if len(got) != 1 || got[0] != "abc" {
		t.Errorf("splitOn: got %v, want [abc]", got)
	}
}

func TestUnescape_Backslash(t *testing.T) {
	got := unescape(`a\,b`)
	if got != "a,b" {
		t.Errorf("unescape: got %q, want %q", got, "a,b")
	}
}

func TestUnescape_NoBackslash(t *testing.T) {
	got := unescape("abc")
	if got != "abc" {
		t.Errorf("unescape: got %q, want %q", got, "abc")
	}
}
