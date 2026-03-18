// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"fmt"
	"io"
)

// CustomAnalyzer is a general-purpose configurable analyzer.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.custom.CustomAnalyzer.
//
// CustomAnalyzer allows building analyzers with custom tokenizers and token filters
// through a builder pattern. It provides flexibility to create analyzers tailored
// to specific use cases without creating new analyzer classes.
//
// Example:
//
//	analyzer, err := NewCustomAnalyzerBuilder().
//	    WithTokenizer(NewStandardTokenizerFactory()).
//	    AddTokenFilter(NewLowerCaseFilterFactory()).
//	    AddTokenFilter(NewStopFilterFactory()).
//	    Build()
//	if err != nil {
//	    // handle error
//	}
//	defer analyzer.Close()
type CustomAnalyzer struct {
	*BaseAnalyzer

	// tokenizerFactory creates the tokenizer
	tokenizerFactory TokenizerFactory

	// tokenFilterFactories create token filters in order
	tokenFilterFactories []TokenFilterFactory

	// charFilterFactories create char filters in order
	charFilterFactories []CharFilterFactory

	// positionIncrementGap is the position increment gap for multi-valued fields
	positionIncrementGap int

	// offsetGap is the offset gap for multi-valued fields
	offsetGap int
}

// NewCustomAnalyzer creates a new CustomAnalyzer with the given components.
func NewCustomAnalyzer(
	tokenizerFactory TokenizerFactory,
	charFilterFactories []CharFilterFactory,
	tokenFilterFactories []TokenFilterFactory,
) (*CustomAnalyzer, error) {
	if tokenizerFactory == nil {
		return nil, fmt.Errorf("tokenizer factory is required")
	}

	return &CustomAnalyzer{
		BaseAnalyzer:         NewAnalyzer(),
		tokenizerFactory:     tokenizerFactory,
		charFilterFactories:  charFilterFactories,
		tokenFilterFactories: tokenFilterFactories,
		positionIncrementGap: 0,
		offsetGap:            1,
	}, nil
}

// TokenStream creates a TokenStream for analyzing text.
func (a *CustomAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	// Apply char filters first
	var charReader io.Reader = reader
	for _, factory := range a.charFilterFactories {
		charReader = factory.Create(charReader)
	}

	// Create the tokenizer
	tokenizer := a.tokenizerFactory.Create()
	if err := tokenizer.SetReader(charReader); err != nil {
		return nil, err
	}

	// Build filter chain
	var stream TokenStream = tokenizer
	for _, factory := range a.tokenFilterFactories {
		stream = factory.Create(stream)
	}

	return stream, nil
}

// GetPositionIncrementGap returns the position increment gap for multi-valued fields.
func (a *CustomAnalyzer) GetPositionIncrementGap() int {
	return a.positionIncrementGap
}

// SetPositionIncrementGap sets the position increment gap for multi-valued fields.
func (a *CustomAnalyzer) SetPositionIncrementGap(gap int) {
	a.positionIncrementGap = gap
}

// GetOffsetGap returns the offset gap for multi-valued fields.
func (a *CustomAnalyzer) GetOffsetGap() int {
	return a.offsetGap
}

// SetOffsetGap sets the offset gap for multi-valued fields.
func (a *CustomAnalyzer) SetOffsetGap(gap int) {
	a.offsetGap = gap
}

// GetTokenizerFactory returns the tokenizer factory.
func (a *CustomAnalyzer) GetTokenizerFactory() TokenizerFactory {
	return a.tokenizerFactory
}

// GetTokenFilterFactories returns the token filter factories.
func (a *CustomAnalyzer) GetTokenFilterFactories() []TokenFilterFactory {
	return a.tokenFilterFactories
}

// GetCharFilterFactories returns the char filter factories.
func (a *CustomAnalyzer) GetCharFilterFactories() []CharFilterFactory {
	return a.charFilterFactories
}

// Ensure CustomAnalyzer implements Analyzer
var _ Analyzer = (*CustomAnalyzer)(nil)
var _ AnalyzerInterface = (*CustomAnalyzer)(nil)

// CustomAnalyzerBuilder builds CustomAnalyzer instances.
//
// Use this builder to construct analyzers with custom component chains.
type CustomAnalyzerBuilder struct {
	tokenizerFactory     TokenizerFactory
	charFilterFactories  []CharFilterFactory
	tokenFilterFactories []TokenFilterFactory
	positionIncrementGap int
	offsetGap            int
}

// NewCustomAnalyzerBuilder creates a new CustomAnalyzerBuilder.
func NewCustomAnalyzerBuilder() *CustomAnalyzerBuilder {
	return &CustomAnalyzerBuilder{
		charFilterFactories:  make([]CharFilterFactory, 0),
		tokenFilterFactories: make([]TokenFilterFactory, 0),
		positionIncrementGap: 0,
		offsetGap:            1,
	}
}

// WithTokenizer sets the tokenizer factory.
func (b *CustomAnalyzerBuilder) WithTokenizer(factory TokenizerFactory) *CustomAnalyzerBuilder {
	b.tokenizerFactory = factory
	return b
}

// AddCharFilter adds a char filter factory to the chain.
func (b *CustomAnalyzerBuilder) AddCharFilter(factory CharFilterFactory) *CustomAnalyzerBuilder {
	b.charFilterFactories = append(b.charFilterFactories, factory)
	return b
}

// AddTokenFilter adds a token filter factory to the chain.
func (b *CustomAnalyzerBuilder) AddTokenFilter(factory TokenFilterFactory) *CustomAnalyzerBuilder {
	b.tokenFilterFactories = append(b.tokenFilterFactories, factory)
	return b
}

// WithPositionIncrementGap sets the position increment gap.
func (b *CustomAnalyzerBuilder) WithPositionIncrementGap(gap int) *CustomAnalyzerBuilder {
	b.positionIncrementGap = gap
	return b
}

// WithOffsetGap sets the offset gap.
func (b *CustomAnalyzerBuilder) WithOffsetGap(gap int) *CustomAnalyzerBuilder {
	b.offsetGap = gap
	return b
}

// Build creates the CustomAnalyzer.
func (b *CustomAnalyzerBuilder) Build() (*CustomAnalyzer, error) {
	if b.tokenizerFactory == nil {
		return nil, fmt.Errorf("tokenizer is required")
	}

	analyzer := &CustomAnalyzer{
		BaseAnalyzer:         NewAnalyzer(),
		tokenizerFactory:     b.tokenizerFactory,
		charFilterFactories:  b.charFilterFactories,
		tokenFilterFactories: b.tokenFilterFactories,
		positionIncrementGap: b.positionIncrementGap,
		offsetGap:            b.offsetGap,
	}

	return analyzer, nil
}

// CustomAnalyzerFactory creates CustomAnalyzer instances.
type CustomAnalyzerFactory struct {
	builder *CustomAnalyzerBuilder
}

// NewCustomAnalyzerFactory creates a new CustomAnalyzerFactory.
func NewCustomAnalyzerFactory() *CustomAnalyzerFactory {
	return &CustomAnalyzerFactory{
		builder: NewCustomAnalyzerBuilder(),
	}
}

// GetBuilder returns the builder for configuration.
func (f *CustomAnalyzerFactory) GetBuilder() *CustomAnalyzerBuilder {
	return f.builder
}

// Create creates a new CustomAnalyzer.
func (f *CustomAnalyzerFactory) Create() AnalyzerInterface {
	analyzer, err := f.builder.Build()
	if err != nil {
		// In production, this should be handled better
		// For now, return a simple analyzer as fallback
		return NewSimpleAnalyzer()
	}
	return analyzer
}

// Ensure CustomAnalyzerFactory implements AnalyzerFactory
var _ AnalyzerFactory = (*CustomAnalyzerFactory)(nil)
