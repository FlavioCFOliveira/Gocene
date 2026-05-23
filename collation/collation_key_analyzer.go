// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// analysis/common/src/java/org/apache/lucene/collation/CollationKeyAnalyzer.java

package collation

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/collation/tokenattributes"
)

// CollationKeyAnalyzer is the Go port of
// org.apache.lucene.collation.CollationKeyAnalyzer.
//
// Configures KeywordTokenizer with CollationAttributeFactory so that
// each token is encoded as its collation key bytes rather than raw UTF-8.
//
// WARNING: Make sure you use exactly the same Collator at index and query
// time — CollationKeys are only comparable when produced by the same
// Collator. See the Java Javadoc for full compatibility caveats.
//
// For locale-sensitive sorting and range queries, prefer a Collator
// sourced from a versioned implementation (e.g. ICU4Go) to avoid
// silent behaviour changes across Go/OS upgrades.
type CollationKeyAnalyzer struct {
	analysis.BaseAnalyzer
	factory *CollationAttributeFactory
}

// Compile-time assertion: CollationKeyAnalyzer must satisfy Analyzer.
var _ analysis.Analyzer = (*CollationKeyAnalyzer)(nil)

// NewCollationKeyAnalyzer creates a CollationKeyAnalyzer that encodes
// every token via the supplied Collator.
func NewCollationKeyAnalyzer(collator tokenattributes.Collator) *CollationKeyAnalyzer {
	factory := NewCollationAttributeFactory(collator)
	a := &CollationKeyAnalyzer{
		BaseAnalyzer: *analysis.NewAnalyzer(),
		factory:      factory,
	}
	a.TokenizerFactory = analysis.NewKeywordTokenizerFactory()
	return a
}

// TokenStream creates a token stream for the field, using
// KeywordTokenizer. The stream emits a single token whose BytesRef is
// the collation key of the entire input.
//
// NOTE: The current Gocene KeywordTokenizer does not accept an
// AttributeFactory injection (unlike the Java version). Collation key
// encoding at the attribute level is deferred until KeywordTokenizer
// supports AttributeFactory injection. The stream structure (Tokenizer +
// no-op filter chain) is correct; byte-level encoding will be activated
// once the underlying tokenizer is wired to CollationAttributeFactory.
func (a *CollationKeyAnalyzer) TokenStream(fieldName string, reader io.Reader) (analysis.TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// Close releases resources held by this analyzer.
func (a *CollationKeyAnalyzer) Close() error {
	return a.BaseAnalyzer.Close()
}
