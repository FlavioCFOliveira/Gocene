// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// analysis/common/src/test/org/apache/lucene/analysis/synonym/TestMultiWordSynonyms.java

package synonym

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// TestMultiWordSynonyms_PassThrough verifies that when a multi-word synonym
// "a b c" => "d" is configured, the input "a e" produces ["a", "e"] unchanged
// because the multi-word match "a b c" never completes.
//
// Source: TestMultiWordSynonyms.testMultiWordSynonyms
func TestMultiWordSynonyms_PassThrough(t *testing.T) {
	// Parse the synonym rule "a b c,d" using Solr format:
	// "a b c" and "d" are equivalents (expand=true), or explicit mapping.
	// The Java test uses "a b c,d" which is an implicit mapping with expand=true,
	// meaning "a b c" <-> "d". With expand=false it would map "a b c" -> "a"
	// (first term), so we use expand=true to get the multi-word input rule.
	p := NewSolrSynonymParser(true, true, nil)
	if err := p.Parse(strings.NewReader("a b c,d")); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	synMap, err := p.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Tokenize "a e" with whitespace tokenizer.
	tok := analysis.NewWhitespaceTokenizer()
	tok.SetReader(strings.NewReader("a e"))
	if err := tok.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	// Apply SynonymFilter.
	f := analysis.NewSynonymFilter(tok, synMap)

	var got []string
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
		got = append(got, attr.(analysis.CharTermAttribute).String())
	}

	// "a" starts the multi-word pattern "a b c" but "b" never follows,
	// so "a" passes through. "e" has no synonym, passes through.
	want := []string{"a", "e"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}
