// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// Analyzer is the abstract base class for all analyzers.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.Analyzer.
//
// An Analyzer is responsible for creating a TokenStream that processes
// text. The TokenStream is created by the TokenStreamComponents, which
// consists of a Tokenizer (source) and zero or more TokenFilters.
//
// Typical usage:
//
//	analyzer := NewStandardAnalyzer()
//	stream := analyzer.TokenStream("field", strings.NewReader("text to analyze"))
//	defer stream.Close()
//
//	for stream.IncrementToken() {
//		// Process token
//	}
type Analyzer interface {
	// TokenStream creates a TokenStream for analyzing text from a Reader.
	// The fieldName parameter identifies the field being analyzed.
	TokenStream(fieldName string, reader io.Reader) (TokenStream, error)

	// Close releases resources held by this Analyzer.
	Close() error
}

// AnalyzerInterface is a convenience interface for analyzer factories.
type AnalyzerInterface interface {
	Analyzer
}

// TokenStreamComponents holds the Tokenizer and TokenStream chain.
//
// This is the Go port of Lucene's Analyzer.TokenStreamComponents.
type TokenStreamComponents struct {
	// source is the Tokenizer that reads from the input
	source Tokenizer

	// sink is the final TokenStream in the chain (may be the source itself)
	sink TokenStream
}

// NewTokenStreamComponents creates TokenStreamComponents.
// If sink is nil, the source is used as the sink.
func NewTokenStreamComponents(source Tokenizer, sink TokenStream) *TokenStreamComponents {
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
func (tsc *TokenStreamComponents) GetSink() TokenStream {
	return tsc.sink
}

// SetSink sets the final TokenStream.
func (tsc *TokenStreamComponents) SetSink(sink TokenStream) {
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
func (a *BaseAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	if a.TokenizerFactory == nil {
		return nil, nil
	}

	// Create tokenizer
	tokenizer := a.TokenizerFactory.Create()
	if err := tokenizer.SetReader(reader); err != nil {
		return nil, err
	}

	// Build filter chain
	var stream TokenStream = tokenizer
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
