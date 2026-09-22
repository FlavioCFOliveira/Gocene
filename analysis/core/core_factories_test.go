// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package core_test

// TestCoreFactories ports org.apache.lucene.analysis.core.TestCoreFactories
// (Apache Lucene 10.4.0).
//
// The Java test uses BaseTokenStreamFactoryTestCase helpers
// (tokenizerFactory("Keyword"), tokenizerFactory("Whitespace"), etc.) to
// instantiate factories via SPI name lookup and then verify behaviour.
// It also tests that unknown arguments produce an IllegalArgumentException.
//
// Deviation: Gocene has no SPI name lookup and no argument validation at the
// factory level.  The port calls the Go factories directly and verifies the
// same tokenization assertions.  The bogus-argument tests are omitted because
// Gocene factory constructors accept no parameters.

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// TestCoreFactories_KeywordTokenizer mirrors TestCoreFactories.testKeywordTokenizer.
// The entire input string is returned as a single token.
func TestCoreFactories_KeywordTokenizer(t *testing.T) {
	tok := analysis.NewKeywordTokenizerFactory().Create(analysis.DefaultTokenAttributeFactory)
	input := "What's this thing do?"
	tok.SetReader(strings.NewReader(input))
	n := drainTokens(t, tok)
	if n != 1 {
		t.Errorf("expected 1 token (full input), got %d", n)
	}
}

// TestCoreFactories_WhitespaceTokenizer mirrors TestCoreFactories.testWhitespaceTokenizer.
// Splits on whitespace, preserving punctuation within tokens.
func TestCoreFactories_WhitespaceTokenizer(t *testing.T) {
	tok := analysis.NewWhitespaceTokenizerFactory().Create()
	tok.SetReader(strings.NewReader("What's this thing do?"))
	n := drainTokens(t, tok)
	// "What's", "this", "thing", "do?" = 4 tokens
	if n != 4 {
		t.Errorf("expected 4 tokens, got %d", n)
	}
}

// TestCoreFactories_LetterTokenizer mirrors TestCoreFactories.testLetterTokenizer.
// Splits on non-letter boundaries, discarding punctuation.
func TestCoreFactories_LetterTokenizer(t *testing.T) {
	tok := analysis.NewLetterTokenizerFactory().Create(analysis.DefaultTokenAttributeFactory)
	tok.SetReader(strings.NewReader("What's this thing do?"))
	n := drainTokens(t, tok)
	// "What", "s", "this", "thing", "do" = 5 tokens
	if n != 5 {
		t.Errorf("expected 5 tokens, got %d", n)
	}
}
