// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis/api"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Analyzer is a type alias for api.Analyzer to maintain backward compatibility
// within the analysis package.
type Analyzer = api.Analyzer

// AnalyzerInterface is a convenience interface for analyzer factories.
type AnalyzerInterface interface {
	api.Analyzer
}

// TokenStream is a type alias for api.TokenStream.
type TokenStream = api.TokenStream

// TokenStreamComponents holds the Tokenizer and TokenStream chain.
//
// This is the Go port of Lucene's Analyzer.TokenStreamComponents.
type TokenStreamComponents struct {
	// source is the Tokenizer that reads from the input
	source Tokenizer

	// sink is the final TokenStream in the chain (may be the source itself)
	sink api.TokenStream
}

// NewTokenStreamComponents creates TokenStreamComponents.
// If sink is nil, the source is used as the sink.
func NewTokenStreamComponents(source Tokenizer, sink api.TokenStream) *TokenStreamComponents {
	if sink == nil {
		sink = source
	}
	return &TokenStreamComponents{
		source: source,
		sink:   sink,
	}
}

// GetSource returns the Tokenizer source.
func (tsc *TokenStreamComponents) GetSource() Tokenizer {
	return tsc.source
}

// GetSink returns the final TokenStream.
func (tsc *TokenStreamComponents) GetSink() api.TokenStream {
	return tsc.sink
}

// SetSink sets the final TokenStream.
func (tsc *TokenStreamComponents) SetSink(sink api.TokenStream) {
	tsc.sink = sink
}

// BaseAnalyzer provides a base implementation for Analyzer.
//
// Embed this struct in concrete Analyzer implementations.
type BaseAnalyzer struct {
	// TokenizerFactory creates the tokenizer
	TokenizerFactory TokenizerFactory

	// TokenFilterFactories create token filters in order
	TokenFilterFactories []TokenFilterFactory

	// reuseTokenStream tracks whether to reuse the token stream
	reuseTokenStream bool

	// storedComponents holds components for reuse
	storedComponents *TokenStreamComponents
}

// NewAnalyzer creates a new BaseAnalyzer with default settings.
func NewAnalyzer() *BaseAnalyzer {
	return &BaseAnalyzer{
		reuseTokenStream:     false,
		TokenFilterFactories: make([]TokenFilterFactory, 0),
	}
}

// SetReuseTokenStream sets whether to reuse token streams.
func (a *BaseAnalyzer) SetReuseTokenStream(reuse bool) {
	a.reuseTokenStream = reuse
}

// IsReuseTokenStream returns whether token streams are reused.
func (a *BaseAnalyzer) IsReuseTokenStream() bool {
	return a.reuseTokenStream
}

// AddTokenFilter adds a token filter factory to the chain.
func (a *BaseAnalyzer) AddTokenFilter(factory TokenFilterFactory) {
	a.TokenFilterFactories = append(a.TokenFilterFactories, factory)
}

// TokenStream creates a TokenStream for analyzing text.
func (a *BaseAnalyzer) TokenStream(fieldName string, reader io.Reader) (api.TokenStream, error) {
	if a.TokenizerFactory == nil {
		return nil, nil
	}

	// Create tokenizer
	tokenizer := a.TokenizerFactory.Create(util.DefaultAttributeFactoryInstance)
	if err := tokenizer.SetReader(reader); err != nil {
		return nil, err
	}

	// Build filter chain
	var stream api.TokenStream = tokenizer
	for _, factory := range a.TokenFilterFactories {
		stream = factory.Create(stream)
	}

	return stream, nil
}

// Close releases resources.
func (a *BaseAnalyzer) Close() error {
	return nil
}

// AnalyzerFactory creates Analyzer instances.
type AnalyzerFactory interface {
	// Create creates a new Analyzer.
	Create() AnalyzerInterface
}
