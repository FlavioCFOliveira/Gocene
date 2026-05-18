// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package highlight

import "github.com/FlavioCFOliveira/Gocene/analysis"

// OffsetLimitTokenFilter is a TokenFilter that stops the token stream once
// the cumulative number of tokens reaches the configured limit. Mirrors
// org.apache.lucene.search.highlight.OffsetLimitTokenFilter, adapted to the
// Gocene TokenStream interface which does not surface per-token offset
// state directly.
type OffsetLimitTokenFilter struct {
	input     analysis.TokenStream
	maxOffset int
	consumed  int
	exhausted bool
}

// NewOffsetLimitTokenFilter wraps input with a maximum number of tokens.
func NewOffsetLimitTokenFilter(input analysis.TokenStream, maxOffset int) *OffsetLimitTokenFilter {
	return &OffsetLimitTokenFilter{input: input, maxOffset: maxOffset}
}

// IncrementToken advances the wrapped stream until the token budget is
// exhausted.
func (f *OffsetLimitTokenFilter) IncrementToken() (bool, error) {
	if f.exhausted {
		return false, nil
	}
	if f.consumed >= f.maxOffset {
		f.exhausted = true
		return false, nil
	}
	ok, err := f.input.IncrementToken()
	if err != nil || !ok {
		return ok, err
	}
	f.consumed++
	return true, nil
}

// End forwards End to the wrapped stream.
func (f *OffsetLimitTokenFilter) End() error { return f.input.End() }

// Close forwards Close.
func (f *OffsetLimitTokenFilter) Close() error { return f.input.Close() }

var _ analysis.TokenStream = (*OffsetLimitTokenFilter)(nil)
