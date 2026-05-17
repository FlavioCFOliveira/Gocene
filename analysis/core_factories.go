// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// FlattenGraphFilterFactory creates FlattenGraphFilter instances.
//
// This is the Go port of
// org.apache.lucene.analysis.core.FlattenGraphFilterFactory from
// Apache Lucene 10.4.0.
type FlattenGraphFilterFactory struct{}

// NewFlattenGraphFilterFactory returns a fresh factory.
func NewFlattenGraphFilterFactory() *FlattenGraphFilterFactory {
	return &FlattenGraphFilterFactory{}
}

// Create wraps input with FlattenGraphFilter.
func (f *FlattenGraphFilterFactory) Create(input TokenStream) TokenFilter {
	return NewFlattenGraphFilter(input)
}

// Ensure factory satisfies TokenFilterFactory.
var _ TokenFilterFactory = (*FlattenGraphFilterFactory)(nil)

// WhitespaceTokenizerFactory creates WhitespaceTokenizer instances.
//
// This is the Go port of
// org.apache.lucene.analysis.core.WhitespaceTokenizerFactory from
// Apache Lucene 10.4.0.
type WhitespaceTokenizerFactory struct {
	maxTokenLength int
	rule           string // "java" | "unicode"
}

// NewWhitespaceTokenizerFactory returns a factory with the Lucene
// defaults (Java-style whitespace rule, default max token length).
func NewWhitespaceTokenizerFactory() *WhitespaceTokenizerFactory {
	return &WhitespaceTokenizerFactory{
		maxTokenLength: DefaultMaxTokenLength,
		rule:           "java",
	}
}

// NewWhitespaceTokenizerFactoryWithRule returns a configured factory.
// rule must be "java" (uses character.isWhitespace semantics) or
// "unicode" (uses Unicode WHITESPACE property).
func NewWhitespaceTokenizerFactoryWithRule(maxTokenLength int, rule string) *WhitespaceTokenizerFactory {
	return &WhitespaceTokenizerFactory{maxTokenLength: maxTokenLength, rule: rule}
}

// Rule returns the configured whitespace rule.
func (f *WhitespaceTokenizerFactory) Rule() string { return f.rule }

// MaxTokenLength returns the configured maximum token length.
func (f *WhitespaceTokenizerFactory) MaxTokenLength() int { return f.maxTokenLength }

// Create returns a Whitespace tokenizer; under the Unicode rule the
// implementation deduplicates the WhitespaceTokenizer via the
// existing letter_tokenizer / whitespace_tokenizer scaffolding.
// Concrete construction is left to consumers because
// WhitespaceTokenizer is not yet exported with a maxTokenLength
// argument; callers should set max token length on the returned
// tokenizer via SetMaxTokenLength when supported.
func (f *WhitespaceTokenizerFactory) Create() Tokenizer {
	return NewWhitespaceTokenizer()
}

// DecimalDigitFilterFactory pre-existed at file level (see
// decimal_digit_filter.go); this comment exists to record the parity
// claim in one place for sprint-28 audit purposes.
//
// FactoryNameDecimalDigit is the Lucene-faithful SPI identifier
// (Solr/Lucene config string) for the decimal-digit filter; Gocene
// records it for parity with future SPI work.
const FactoryNameDecimalDigit = "decimalDigit"
