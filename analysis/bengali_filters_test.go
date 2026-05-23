// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis_test

// TestBengaliFilters ports org.apache.lucene.analysis.bn.TestBengaliFilters
// (Apache Lucene 10.4.0).
//
// The Java test uses BaseTokenStreamFactoryTestCase helpers
// (tokenFilterFactory("BengaliNormalization"), etc.) to instantiate
// factories via SPI name lookup.  Gocene has no SPI; factories are
// instantiated directly.  The bogus-argument tests are omitted because
// Gocene factory constructors accept no parameters.

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	indicpkg "github.com/FlavioCFOliveira/Gocene/analysis/in"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// drainBengaliTerms drives a TokenStream and returns the term strings.
// It accesses the shared AttributeSource by walking the filter chain to
// the underlying tokenizer.
func drainBengaliTerms(t *testing.T, stream analysis.TokenStream) []string {
	t.Helper()
	defer stream.Close()

	type attrSrcGetter interface {
		GetAttributeSource() *util.AttributeSource
	}
	findAttrSrc := func(s analysis.TokenStream) *util.AttributeSource {
		cur := s
		for {
			if ag, ok := cur.(attrSrcGetter); ok {
				return ag.GetAttributeSource()
			}
			if tf, ok := cur.(interface{ GetInput() analysis.TokenStream }); ok {
				cur = tf.GetInput()
			} else {
				break
			}
		}
		return nil
	}

	var terms []string
	for {
		ok, err := stream.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		if attrSrc := findAttrSrc(stream); attrSrc != nil {
			if raw := attrSrc.GetAttribute(analysis.CharTermAttributeType); raw != nil {
				if cta, ok := raw.(analysis.CharTermAttribute); ok {
					terms = append(terms, cta.String())
				}
			}
		}
	}
	return terms
}

// tokenizeBengaliWhitespace creates a WhitespaceTokenizer loaded with input.
func tokenizeBengaliWhitespace(t *testing.T, input string) analysis.Tokenizer {
	t.Helper()
	tok := analysis.NewWhitespaceTokenizer()
	if err := tok.SetReader(strings.NewReader(input)); err != nil {
		t.Fatalf("SetReader: %v", err)
	}
	return tok
}

// TestBengaliFilters_IndicNormalizer mirrors TestBengaliFilters.testIndicNormalizer.
// IndicNormalizationFilter normalises "ত্‍ আমি" → ["ৎ" "আমি"].
func TestBengaliFilters_IndicNormalizer(t *testing.T) {
	tok := tokenizeBengaliWhitespace(t, "ত্‍ আমি")
	stream := indicpkg.NewIndicNormalizationFilterFactory().Create(tok)
	terms := drainBengaliTerms(t, stream)
	want := []string{"ৎ", "আমি"}
	if len(terms) != len(want) {
		t.Fatalf("got %v (%d), want %v (%d)", terms, len(terms), want, len(want))
	}
	for i, w := range want {
		if terms[i] != w {
			t.Errorf("term[%d] = %q, want %q", i, terms[i], w)
		}
	}
}

// TestBengaliFilters_BengaliNormalizer mirrors TestBengaliFilters.testBengaliNormalizer.
// After IndicNormalization, BengaliNormalizationFilter normalises
// "বাড়ী" → "বারি".
func TestBengaliFilters_BengaliNormalizer(t *testing.T) {
	tok := tokenizeBengaliWhitespace(t, "বাড়ী")
	indic := indicpkg.NewIndicNormalizationFilterFactory().Create(tok)
	stream := analysis.NewBengaliNormalizationFilterFactory().Create(indic)
	terms := drainBengaliTerms(t, stream)
	want := []string{"বারি"}
	if len(terms) != len(want) {
		t.Fatalf("got %v (%d), want %v (%d)", terms, len(terms), want, len(want))
	}
	if terms[0] != want[0] {
		t.Errorf("term = %q, want %q", terms[0], want[0])
	}
}

// TestBengaliFilters_Stemmer mirrors TestBengaliFilters.testStemmer.
// After IndicNormalization + BengaliNormalization, BengaliStemFilter
// reduces "বাড়ী" → "বার".
func TestBengaliFilters_Stemmer(t *testing.T) {
	tok := tokenizeBengaliWhitespace(t, "বাড়ী")
	indic := indicpkg.NewIndicNormalizationFilterFactory().Create(tok)
	norm := analysis.NewBengaliNormalizationFilterFactory().Create(indic)
	stream := analysis.NewBengaliStemFilterFactory().Create(norm)
	terms := drainBengaliTerms(t, stream)
	want := []string{"বার"}
	if len(terms) != len(want) {
		t.Fatalf("got %v (%d), want %v (%d)", terms, len(terms), want, len(want))
	}
	if terms[0] != want[0] {
		t.Errorf("term = %q, want %q", terms[0], want[0])
	}
}
