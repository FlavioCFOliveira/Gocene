// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// TestElision ports org.apache.lucene.analysis.util.TestElision
// (Apache Lucene 10.4.0).

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// FrenchDefaultArticles is the French elision article list from
// org.apache.lucene.analysis.fr.FrenchAnalyzer.DEFAULT_ARTICLES.
// Deviation: Gocene's FrenchAnalyzer does not export DEFAULT_ARTICLES;
// the constant is inlined here for test use only.
var frenchDefaultArticles = GetWordSetFromStrings(
	[]string{"l", "m", "t", "qu", "n", "s", "j", "d", "c",
		"jusqu", "quoiqu", "lorsqu", "puisqu"},
	false,
)

// drainElisionTokens drives a TokenStream to exhaustion, collecting term
// strings via the shared AttributeSource.
func drainElisionTokens(t *testing.T, stream TokenStream) []string {
	t.Helper()
	defer stream.Close()

	type attrSrcGetter interface {
		GetAttributeSource() *util.AttributeSource
	}
	findAttrSrc := func(s TokenStream) *util.AttributeSource {
		cur := s
		for {
			if ag, ok := cur.(attrSrcGetter); ok {
				return ag.GetAttributeSource()
			}
			if tf, ok := cur.(interface{ GetInput() TokenStream }); ok {
				cur = tf.GetInput()
			} else {
				break
			}
		}
		return nil
	}

	var tokens []string
	for {
		ok, err := stream.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		if attrSrc := findAttrSrc(stream); attrSrc != nil {
			if raw := attrSrc.GetAttribute(CharTermAttributeType); raw != nil {
				if cta, ok := raw.(CharTermAttribute); ok {
					tokens = append(tokens, cta.String())
				}
			}
		}
	}
	return tokens
}

// TestElision_Elision mirrors TestElision.testElision.
// ElisionFilter strips known French elision particles ("l'", "M'") while
// preserving apostrophes that are not particle prefixes ("O'brian").
func TestElision_Elision(t *testing.T) {
	text := "Plop, juste pour voir l'embrouille avec O'brian. M'enfin."
	tok := NewStandardTokenizer()
	if err := tok.SetReader(strings.NewReader(text)); err != nil {
		t.Fatalf("SetReader: %v", err)
	}
	articles := GetWordSetFromStrings([]string{"l", "M"}, false)
	filter := NewElisionFilter(tok, articles)
	tokens := drainElisionTokens(t, filter)

	// Expected: "Plop" "juste" "pour" "voir" "embrouille" "avec" "O'brian" "enfin"
	// Index 4 = "embrouille" (l' stripped)
	// Index 6 = "O'brian"    (O is not an article, apostrophe preserved)
	// Index 7 = "enfin"      (M' stripped)
	if len(tokens) < 8 {
		t.Fatalf("expected at least 8 tokens, got %d: %v", len(tokens), tokens)
	}
	if tokens[4] != "embrouille" {
		t.Errorf("tokens[4] = %q, want %q", tokens[4], "embrouille")
	}
	if tokens[6] != "O'brian" {
		t.Errorf("tokens[6] = %q, want %q", tokens[6], "O'brian")
	}
	if tokens[7] != "enfin" {
		t.Errorf("tokens[7] = %q, want %q", tokens[7], "enfin")
	}
}

// TestElision_EmptyTerm mirrors TestElision.testEmptyTerm.
// ElisionFilter on an empty string via KeywordTokenizer must return one empty token.
func TestElision_EmptyTerm(t *testing.T) {
	tok := NewKeywordTokenizer()
	if err := tok.SetReader(strings.NewReader("")); err != nil {
		t.Fatalf("SetReader: %v", err)
	}
	filter := NewElisionFilter(tok, frenchDefaultArticles)
	tokens := drainElisionTokens(t, filter)

	if len(tokens) != 1 {
		t.Fatalf("expected 1 token for empty input, got %d: %v", len(tokens), tokens)
	}
	if tokens[0] != "" {
		t.Errorf("token = %q, want empty string", tokens[0])
	}
}
