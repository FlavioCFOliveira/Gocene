// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package shingle

import (
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// ShingleAnalyzerWrapper wraps a ShingleFilter around another Analyzer.
//
// This is the Go port of
// org.apache.lucene.analysis.shingle.ShingleAnalyzerWrapper from
// Apache Lucene 10.4.0.
//
// Deviation: ShingleFilter in this codebase does not yet expose
// SetOutputUnigramsIfNoShingles or SetFillerToken; those fields are stored and
// applied when the filter is built but will have no effect until ShingleFilter
// gains those methods.
type ShingleAnalyzerWrapper struct {
	*analysis.AnalyzerWrapper

	delegate                     analysis.Analyzer
	minShingleSize               int
	maxShingleSize               int
	tokenSeparator               string
	outputUnigrams               bool
	outputUnigramsIfNoShingles   bool
	fillerToken                  string
}

// NewShingleAnalyzerWrapper creates a wrapper with default parameters
// (minShingleSize=2, maxShingleSize=2).
func NewShingleAnalyzerWrapper(delegate analysis.Analyzer) (*ShingleAnalyzerWrapper, error) {
	return NewShingleAnalyzerWrapperWithSizes(delegate, 2, 2)
}

// NewShingleAnalyzerWrapperWithSizes creates a wrapper with custom min/max shingle sizes.
func NewShingleAnalyzerWrapperWithSizes(
	delegate analysis.Analyzer,
	minShingleSize, maxShingleSize int,
) (*ShingleAnalyzerWrapper, error) {
	return NewShingleAnalyzerWrapperFull(delegate, minShingleSize, maxShingleSize, " ", true, false, "_")
}

// NewShingleAnalyzerWrapperFull creates a wrapper with all parameters.
func NewShingleAnalyzerWrapperFull(
	delegate analysis.Analyzer,
	minShingleSize, maxShingleSize int,
	tokenSeparator string,
	outputUnigrams, outputUnigramsIfNoShingles bool,
	fillerToken string,
) (*ShingleAnalyzerWrapper, error) {
	if maxShingleSize < 2 {
		return nil, fmt.Errorf("max shingle size must be >= 2")
	}
	if minShingleSize < 2 {
		return nil, fmt.Errorf("min shingle size must be >= 2")
	}
	if minShingleSize > maxShingleSize {
		return nil, fmt.Errorf("min shingle size must be <= max shingle size")
	}
	w := &ShingleAnalyzerWrapper{
		delegate:                   delegate,
		minShingleSize:             minShingleSize,
		maxShingleSize:             maxShingleSize,
		tokenSeparator:             tokenSeparator,
		outputUnigrams:             outputUnigrams,
		outputUnigramsIfNoShingles: outputUnigramsIfNoShingles,
		fillerToken:                fillerToken,
	}
	w.AnalyzerWrapper = analysis.NewAnalyzerWrapper(func(_ string) analysis.Analyzer {
		return delegate
	})
	w.AnalyzerWrapper.WrapTokenStream = func(fieldName string, in analysis.TokenStream) analysis.TokenStream {
		sf := analysis.NewShingleFilterWithSizes(in, w.minShingleSize, w.maxShingleSize)
		sf.SetTokenSeparator(w.tokenSeparator)
		sf.SetOutputUnigrams(w.outputUnigrams)
		return sf
	}
	return w, nil
}

// TokenStream returns a token stream for the given field and reader.
func (w *ShingleAnalyzerWrapper) TokenStream(fieldName string, reader io.Reader) (analysis.TokenStream, error) {
	return w.AnalyzerWrapper.TokenStream(fieldName, reader)
}

// GetWrappedAnalyzerFunc returns the wrapped analyzer for a given field.
func (w *ShingleAnalyzerWrapper) GetDelegate() analysis.Analyzer { return w.delegate }

// GetMinShingleSize returns the configured minimum shingle size.
func (w *ShingleAnalyzerWrapper) GetMinShingleSize() int { return w.minShingleSize }

// GetMaxShingleSize returns the configured maximum shingle size.
func (w *ShingleAnalyzerWrapper) GetMaxShingleSize() int { return w.maxShingleSize }

// GetTokenSeparator returns the configured token separator.
func (w *ShingleAnalyzerWrapper) GetTokenSeparator() string { return w.tokenSeparator }

// IsOutputUnigrams reports whether unigrams are output alongside shingles.
func (w *ShingleAnalyzerWrapper) IsOutputUnigrams() bool { return w.outputUnigrams }

// IsOutputUnigramsIfNoShingles reports whether unigrams are output when no
// shingles are available.
func (w *ShingleAnalyzerWrapper) IsOutputUnigramsIfNoShingles() bool {
	return w.outputUnigramsIfNoShingles
}

// GetFillerToken returns the filler token string.
func (w *ShingleAnalyzerWrapper) GetFillerToken() string { return w.fillerToken }

// Ensure ShingleAnalyzerWrapper implements Analyzer.
var _ analysis.Analyzer = (*ShingleAnalyzerWrapper)(nil)
