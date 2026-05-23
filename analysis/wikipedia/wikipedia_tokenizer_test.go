// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package wikipedia

import (
	"strings"
	"testing"
)

// collectWikiTokens uses WikipediaTokenizer to tokenize text and returns
// (text, type) pairs.
func collectWikiTokens(t *testing.T, text string) []struct{ text, typ string } {
	t.Helper()
	tok := NewWikipediaTokenizer()
	if err := tok.SetReader(strings.NewReader(text)); err != nil {
		t.Fatal(err)
	}
	if err := tok.Reset(); err != nil {
		t.Fatal(err)
	}
	var result []struct{ text, typ string }
	for {
		ok, err := tok.IncrementToken()
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			break
		}
		result = append(result, struct{ text, typ string }{
			tok.termAttr.String(),
			tok.typeAttr.GetType(),
		})
	}
	return result
}

// TestWikipediaTokenizer_Simple mirrors TestWikipediaTokenizer.testSimple.
func TestWikipediaTokenizer_Simple(t *testing.T) {
	text := "This is a [[Category:foo]]"
	tokens := collectWikiTokens(t, text)
	wantTexts := []string{"This", "is", "a", "foo"}
	wantTypes := []string{"<ALPHANUM>", "<ALPHANUM>", "<ALPHANUM>", "c"}
	if len(tokens) != len(wantTexts) {
		t.Fatalf("got %d tokens, want %d: %v", len(tokens), len(wantTexts), tokens)
	}
	for i, w := range wantTexts {
		if tokens[i].text != w {
			t.Errorf("token[%d].text = %q, want %q", i, tokens[i].text, w)
		}
		if tokens[i].typ != wantTypes[i] {
			t.Errorf("token[%d].type = %q, want %q", i, tokens[i].typ, wantTypes[i])
		}
	}
}

// TestWikipediaTokenizer_InternalLink verifies [[link]] extraction.
func TestWikipediaTokenizer_InternalLink(t *testing.T) {
	tokens := collectWikiTokens(t, "click [[link here]] done")
	found := false
	for _, tok := range tokens {
		if tok.text == "link" && tok.typ == "il" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected INTERNAL_LINK token for 'link', got: %v", tokens)
	}
}

// TestWikipediaTokenizer_ExternalLink verifies [url text] extraction.
func TestWikipediaTokenizer_ExternalLink(t *testing.T) {
	tokens := collectWikiTokens(t, "see [http://lucene.apache.org Lucene] now")
	found := false
	for _, tok := range tokens {
		if tok.typ == "elu" || tok.typ == "el" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected EXTERNAL_LINK_URL or EXTERNAL_LINK token, got: %v", tokens)
	}
}

// TestWikipediaTokenizer_Citation verifies <ref>...</ref> handling.
func TestWikipediaTokenizer_Citation(t *testing.T) {
	tokens := collectWikiTokens(t, "text <ref>Citation</ref> more")
	foundAlpha := false
	for _, tok := range tokens {
		if tok.text == "text" || tok.text == "more" {
			foundAlpha = true
		}
	}
	if !foundAlpha {
		t.Errorf("expected plain alphanum tokens around citation: %v", tokens)
	}
}

// TestWikipediaTokenizer_Heading verifies == heading == detection.
func TestWikipediaTokenizer_Heading(t *testing.T) {
	tokens := collectWikiTokens(t, "==Heading==")
	found := false
	for _, tok := range tokens {
		if tok.text == "Heading" && tok.typ == "h" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected HEADING token for 'Heading', got: %v", tokens)
	}
}

// TestWikipediaTokenizer_Plain verifies plain text tokenization.
func TestWikipediaTokenizer_Plain(t *testing.T) {
	tokens := collectWikiTokens(t, "hello world")
	if len(tokens) != 2 {
		t.Fatalf("want 2 tokens, got %d: %v", len(tokens), tokens)
	}
	if tokens[0].text != "hello" || tokens[1].text != "world" {
		t.Errorf("unexpected tokens: %v", tokens)
	}
}

// TestWikipediaTokenizerImpl_YYEOF verifies empty input returns YYEOF.
func TestWikipediaTokenizerImpl_YYEOF(t *testing.T) {
	impl := NewWikipediaTokenizerImpl(strings.NewReader(""))
	tok := impl.GetNextToken()
	if tok != YYEOF {
		t.Errorf("empty input: got %d, want YYEOF (%d)", tok, YYEOF)
	}
}
