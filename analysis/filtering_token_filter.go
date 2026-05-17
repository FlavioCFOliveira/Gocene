// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
)

// FilteringTokenFilter is the abstract base type for TokenFilters that may
// remove tokens. Subclasses (in Go, callers) implement the AcceptFn callback;
// IncrementToken returns only those tokens for which AcceptFn returned true,
// and accumulates the PositionIncrement of the skipped tokens so that the
// next accepted token reflects the gap.
//
// This is the Go port of Lucene's
// org.apache.lucene.analysis.FilteringTokenFilter.
//
// Idiomatic Go uses a callback rather than method override; concrete filters
// either embed *FilteringTokenFilter and configure AcceptFn at construction
// time, or compose with it directly.
type FilteringTokenFilter struct {
	*BaseTokenFilter

	// AcceptFn returns true if the current token should be emitted. The
	// callback may inspect any attribute on the shared AttributeSource via
	// the embedded BaseTokenFilter. Must be non-nil.
	AcceptFn func() (bool, error)

	// posIncrAttr is a cached lookup of the PositionIncrementAttribute on the
	// shared AttributeSource. May be nil if the underlying stream does not
	// expose a PositionIncrementAttribute.
	posIncrAttr PositionIncrementAttribute

	// skippedPositions accumulates the position increment of tokens that
	// AcceptFn rejected since the last accepted token (or since reset).
	skippedPositions int
}

// NewFilteringTokenFilter wraps the given input stream and configures the
// AcceptFn callback. The PositionIncrementAttribute, if present on the
// shared AttributeSource, is cached for use in IncrementToken.
func NewFilteringTokenFilter(input TokenStream, accept func() (bool, error)) *FilteringTokenFilter {
	f := &FilteringTokenFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		AcceptFn:        accept,
	}
	if src := f.GetAttributeSource(); src != nil {
		if attr := src.GetAttributeByType(reflect.TypeOf(&positionIncrementAttribute{})); attr != nil {
			if pi, ok := attr.(PositionIncrementAttribute); ok {
				f.posIncrAttr = pi
			}
		}
	}
	return f
}

// IncrementToken advances the underlying stream until AcceptFn returns true
// or the stream is exhausted. When a token is accepted, the
// PositionIncrement of the accepted token is bumped by the cumulative
// PositionIncrement of the skipped tokens so callers downstream see the
// correct positional gap.
func (f *FilteringTokenFilter) IncrementToken() (bool, error) {
	f.skippedPositions = 0
	for {
		ok, err := f.input.IncrementToken()
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
		accept, err := f.AcceptFn()
		if err != nil {
			return false, err
		}
		if accept {
			if f.posIncrAttr != nil && f.skippedPositions != 0 {
				f.posIncrAttr.SetPositionIncrement(
					f.posIncrAttr.GetPositionIncrement() + f.skippedPositions,
				)
			}
			return true, nil
		}
		if f.posIncrAttr != nil {
			f.skippedPositions += f.posIncrAttr.GetPositionIncrement()
		}
	}
}

// Reset resets the filter and forwards the call to the underlying input.
func (f *FilteringTokenFilter) Reset() error {
	f.skippedPositions = 0
	if resetter, ok := f.input.(interface{ Reset() error }); ok {
		return resetter.Reset()
	}
	return nil
}

// End forwards end-of-stream processing to the input and bumps the
// PositionIncrement by any trailing skipped positions, matching Lucene.
func (f *FilteringTokenFilter) End() error {
	if err := f.BaseTokenFilter.End(); err != nil {
		return err
	}
	if f.posIncrAttr != nil {
		f.posIncrAttr.SetPositionIncrement(
			f.posIncrAttr.GetPositionIncrement() + f.skippedPositions,
		)
	}
	return nil
}

// Ensure FilteringTokenFilter implements TokenFilter.
var _ TokenFilter = (*FilteringTokenFilter)(nil)
