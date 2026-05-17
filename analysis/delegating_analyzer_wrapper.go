// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// DelegatingAnalyzerWrapper is an AnalyzerWrapper that does not allow
// wrapping the TokenStream or the Reader. By disallowing that, the wrapper
// can safely delegate all per-field state to the wrapped Analyzer instead of
// holding its own copy.
//
// This is the Go port of Lucene's
// org.apache.lucene.analysis.DelegatingAnalyzerWrapper.
//
// Typical use: a PerFieldAnalyzerWrapper that simply chooses which Analyzer
// to use based on the field name.
type DelegatingAnalyzerWrapper struct {
	// GetWrappedAnalyzer returns the wrapped Analyzer for the given field
	// name. Must be non-nil. The returned Analyzer is assumed to be non-nil.
	GetWrappedAnalyzer func(fieldName string) Analyzer
}

// NewDelegatingAnalyzerWrapper creates a new DelegatingAnalyzerWrapper that
// delegates to the Analyzer returned by getWrappedAnalyzer for every field.
func NewDelegatingAnalyzerWrapper(getWrappedAnalyzer func(fieldName string) Analyzer) *DelegatingAnalyzerWrapper {
	return &DelegatingAnalyzerWrapper{
		GetWrappedAnalyzer: getWrappedAnalyzer,
	}
}

// TokenStream delegates to the wrapped Analyzer without modifying the input
// reader or the resulting TokenStream.
func (d *DelegatingAnalyzerWrapper) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return d.GetWrappedAnalyzer(fieldName).TokenStream(fieldName, reader)
}

// Close releases resources held by this DelegatingAnalyzerWrapper. The
// wrapped Analyzer is not closed; ownership of its lifecycle stays with the
// caller.
func (d *DelegatingAnalyzerWrapper) Close() error {
	return nil
}

// GetPositionIncrementGap returns the position-increment gap of the wrapped
// Analyzer for the given field. Mirrors Lucene's pass-through behavior.
func (d *DelegatingAnalyzerWrapper) GetPositionIncrementGap(fieldName string) int {
	if gap, ok := d.GetWrappedAnalyzer(fieldName).(interface {
		GetPositionIncrementGap(string) int
	}); ok {
		return gap.GetPositionIncrementGap(fieldName)
	}
	return 0
}

// GetOffsetGap returns the offset gap of the wrapped Analyzer for the given
// field. Mirrors Lucene's pass-through behavior.
func (d *DelegatingAnalyzerWrapper) GetOffsetGap(fieldName string) int {
	if gap, ok := d.GetWrappedAnalyzer(fieldName).(interface {
		GetOffsetGap(string) int
	}); ok {
		return gap.GetOffsetGap(fieldName)
	}
	return 1
}

// Ensure DelegatingAnalyzerWrapper implements Analyzer.
var _ Analyzer = (*DelegatingAnalyzerWrapper)(nil)
