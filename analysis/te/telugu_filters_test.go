// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package te_test

// TestTeluguFilters ports org.apache.lucene.analysis.te.TestTeluguFilters
// (Apache Lucene 10.4.0).
//
// The Java test uses BaseTokenStreamFactoryTestCase helpers
// (tokenFilterFactory("IndicNormalization"), etc.) to instantiate factories
// via SPI name lookup.  Gocene has no SPI; factories are instantiated
// directly.  The bogus-argument tests are omitted because Gocene factory
// constructors accept no parameters.

import (
	"strings"
	"testing"

	indicpkg "github.com/FlavioCFOliveira/Gocene/analysis/in"
	tepkg "github.com/FlavioCFOliveira/Gocene/analysis/te"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// tokenizeWhitespace creates a WhitespaceTokenizer loaded with the input string.
func tokenizeWhitespace(t *testing.T, input string) analysis.Tokenizer {
	t.Helper()
	tok := analysis.NewWhitespaceTokenizer()
	if err := tok.SetReader(strings.NewReader(input)); err != nil {
		t.Fatalf("SetReader: %v", err)
	}
	return tok
}

// drainTerms collects the term text from each token in the stream.
// It walks up to the tokenizer via GetInput() to reach the shared AttributeSource.
func drainTerms(t *testing.T, stream analysis.TokenStream) []string {
	t.Helper()
	defer stream.Close()

	// Find the AttributeSource: walk the filter chain until a node
	// satisfying the *util.AttributeSource getter is found.
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

// assertTerms checks that terms equals want element by element.
func assertTerms(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v (%d terms), want %v (%d terms)", got, len(got), want, len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("term[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestTeluguFilters_IndicNormalizer mirrors TestTeluguFilters.testIndicNormalizer.
// IndicNormalizationFilter must keep "ప్" unchanged and normalise "अाैर" → "और".
func TestTeluguFilters_IndicNormalizer(t *testing.T) {
	tok := tokenizeWhitespace(t, "ప్  अाैर")
	stream := indicpkg.NewIndicNormalizationFilterFactory().Create(tok)
	terms := drainTerms(t, stream)
	assertTerms(t, terms, []string{"ప్", "और"})
}

// TestTeluguFilters_TeluguNormalizer mirrors TestTeluguFilters.testTeluguNormalizer.
// After IndicNormalization, TeluguNormalizationFilter rewrites
// "వస్తువులు" → "వస్తుమలు".
func TestTeluguFilters_TeluguNormalizer(t *testing.T) {
	tok := tokenizeWhitespace(t, "వస్తువులు")
	indic := indicpkg.NewIndicNormalizationFilterFactory().Create(tok)
	stream := tepkg.NewTeluguNormalizationFilterFactory().Create(indic)
	terms := drainTerms(t, stream)
	assertTerms(t, terms, []string{"వస్తుమలు"})
}

// TestTeluguFilters_Stemmer mirrors TestTeluguFilters.testStemmer.
// After IndicNormalization + TeluguNormalization, TeluguStemFilter
// reduces "వస్తువులు" → "వస్తుమ".
func TestTeluguFilters_Stemmer(t *testing.T) {
	tok := tokenizeWhitespace(t, "వస్తువులు")
	indic := indicpkg.NewIndicNormalizationFilterFactory().Create(tok)
	norm := tepkg.NewTeluguNormalizationFilterFactory().Create(indic)
	stream := tepkg.NewTeluguStemFilterFactory().Create(norm)
	terms := drainTerms(t, stream)
	assertTerms(t, terms, []string{"వస్తుమ"})
}
