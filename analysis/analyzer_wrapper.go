// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// AnalyzerWrapper is an Analyzer that wraps other Analyzers.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.AnalyzerWrapper.
//
// AnalyzerWrapper allows the wrapped Analyzer to be selected on a per-field
// basis via GetWrappedAnalyzer(fieldName). The TokenStream produced by the
// wrapped Analyzer may be further wrapped (e.g. with extra TokenFilters) via
// WrapTokenStream and the input Reader may be wrapped (e.g. with CharFilters)
// via WrapReader.
//
// In the Java original these are protected overridable methods. In Gocene
// they are exposed as function-typed fields with sane defaults so that the
// wrapper can be configured at construction time without subclassing.
//
// If you only need to delegate to other analyzers (without wrapping the
// TokenStream or Reader), use DelegatingAnalyzerWrapper instead.
type AnalyzerWrapper struct {
	// GetWrappedAnalyzer returns the wrapped Analyzer for the given field
	// name. Must be non-nil. The returned Analyzer is assumed to be non-nil.
	GetWrappedAnalyzer func(fieldName string) Analyzer

	// WrapTokenStream wraps or alters the given TokenStream produced by the
	// wrapped Analyzer. The default implementation returns the stream as-is.
	WrapTokenStream func(fieldName string, in TokenStream) TokenStream

	// WrapReader wraps or alters the given Reader before it is passed to the
	// wrapped Analyzer. The default implementation returns the reader as-is.
	WrapReader func(fieldName string, reader io.Reader) io.Reader
}

// NewAnalyzerWrapper creates a new AnalyzerWrapper that delegates to the
// Analyzer returned by getWrappedAnalyzer for every field.
// The hooks WrapTokenStream and WrapReader default to identity transforms;
// callers may set them on the returned struct to customise the behaviour.
func NewAnalyzerWrapper(getWrappedAnalyzer func(fieldName string) Analyzer) *AnalyzerWrapper {
	return &AnalyzerWrapper{
		GetWrappedAnalyzer: getWrappedAnalyzer,
		WrapTokenStream:    defaultWrapTokenStream,
		WrapReader:         defaultWrapReader,
	}
}

// TokenStream creates a TokenStream by delegating to the wrapped Analyzer,
// first wrapping the input reader via WrapReader and then wrapping the
// resulting TokenStream via WrapTokenStream.
func (w *AnalyzerWrapper) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	wrapped := w.GetWrappedAnalyzer(fieldName)
	r := reader
	if w.WrapReader != nil {
		r = w.WrapReader(fieldName, reader)
	}
	stream, err := wrapped.TokenStream(fieldName, r)
	if err != nil {
		return nil, err
	}
	if w.WrapTokenStream != nil {
		return w.WrapTokenStream(fieldName, stream), nil
	}
	return stream, nil
}

// Close releases resources held by this AnalyzerWrapper. The wrapped Analyzer
// is not closed; ownership of its lifecycle stays with the caller.
func (w *AnalyzerWrapper) Close() error {
	return nil
}

// GetPositionIncrementGap returns the position-increment gap of the wrapped
// Analyzer for the given field. Mirrors Lucene's pass-through behavior.
func (w *AnalyzerWrapper) GetPositionIncrementGap(fieldName string) int {
	if gap, ok := w.GetWrappedAnalyzer(fieldName).(interface {
		GetPositionIncrementGap(string) int
	}); ok {
		return gap.GetPositionIncrementGap(fieldName)
	}
	return 0
}

// GetOffsetGap returns the offset gap of the wrapped Analyzer for the given
// field. Mirrors Lucene's pass-through behavior.
func (w *AnalyzerWrapper) GetOffsetGap(fieldName string) int {
	if gap, ok := w.GetWrappedAnalyzer(fieldName).(interface {
		GetOffsetGap(string) int
	}); ok {
		return gap.GetOffsetGap(fieldName)
	}
	return 1
}

// defaultWrapTokenStream is the no-op default for WrapTokenStream.
func defaultWrapTokenStream(_ string, in TokenStream) TokenStream {
	return in
}

// defaultWrapReader is the no-op default for WrapReader.
func defaultWrapReader(_ string, reader io.Reader) io.Reader {
	return reader
}

// Ensure AnalyzerWrapper implements Analyzer.
var _ Analyzer = (*AnalyzerWrapper)(nil)
