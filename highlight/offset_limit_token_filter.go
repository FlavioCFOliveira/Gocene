// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package highlight

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// attributeSourceProvider is a non-exported interface satisfied by every
// TokenStream that embeds [analysis.BaseTokenStream] — it gives the
// filter access to the typed attribute registry so it can read the
// [analysis.OffsetAttribute].
type attributeSourceProvider interface {
	GetAttributeSource() *util.AttributeSource
}

// OffsetLimitTokenFilter is a TokenFilter that stops the token stream once
// a token's start offset exceeds the configured limit. It mirrors
// org.apache.lucene.search.highlight.LimitTokenOffsetFilter from
// Apache Lucene 10.4.0.
//
// Unlike a simple token-count limiter, this filter compares the character
// start offset of each token (via [analysis.OffsetAttribute]) against
// maxStartOffset. Tokens whose start offset is strictly greater than
// maxStartOffset are discarded, and the stream ends.
type OffsetLimitTokenFilter struct {
	input          analysis.TokenStream
	maxStartOffset int
	offsetAttr     analysis.OffsetAttribute
	exhausted      bool
}

// NewOffsetLimitTokenFilter wraps input so that only tokens whose start
// offset is ≤ maxStartOffset pass through. If the input stream does not
// expose an [analysis.OffsetAttribute] the filter falls back to passing
// all tokens through — matching the defensive Lucene pattern where the
// attribute is always available on correctly-constructed streams.
func NewOffsetLimitTokenFilter(input analysis.TokenStream, maxStartOffset int) *OffsetLimitTokenFilter {
	f := &OffsetLimitTokenFilter{
		input:          input,
		maxStartOffset: maxStartOffset,
	}
	// Resolve the OffsetAttribute from the wrapped stream's attribute source.
	if asp, ok := input.(attributeSourceProvider); ok {
		if a := asp.GetAttributeSource().GetAttribute(analysis.OffsetAttributeType); a != nil {
			f.offsetAttr, _ = a.(analysis.OffsetAttribute)
		}
	}
	return f
}

// IncrementToken advances the wrapped stream and returns true only when
// the current token's start offset is within the configured limit.
func (f *OffsetLimitTokenFilter) IncrementToken() (bool, error) {
	if f.exhausted {
		return false, nil
	}
	ok, err := f.input.IncrementToken()
	if err != nil || !ok {
		return ok, err
	}
	// If we have an OffsetAttribute, check the start offset; if the
	// attribute is unavailable (legacy TokenStream), fall back to passing
	// all tokens through — this matches the defensive Lucene pattern.
	if f.offsetAttr != nil && f.offsetAttr.StartOffset() > f.maxStartOffset {
		f.exhausted = true
		return false, nil
	}
	return true, nil
}

// End forwards End to the wrapped stream.
func (f *OffsetLimitTokenFilter) End() error { return f.input.End() }

// Close forwards Close.
func (f *OffsetLimitTokenFilter) Close() error { return f.input.Close() }

var _ analysis.TokenStream = (*OffsetLimitTokenFilter)(nil)
