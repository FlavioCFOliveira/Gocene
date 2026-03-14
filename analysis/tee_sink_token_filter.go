// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// TeeSinkTokenFilter is a TokenFilter that tees tokens to multiple sinks.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.TeeSinkTokenFilter.
//
// TeeSinkTokenFilter allows tokens to be consumed by multiple consumers
// simultaneously. This is useful when you need to process the same token
// stream in different ways.
type TeeSinkTokenFilter struct {
	*BaseTokenFilter

	// sinks is the list of registered sinks
	sinks []*TokenSink
}

// TokenSink is a sink that receives tokens from a TeeSinkTokenFilter.
type TokenSink struct {
	// tokens stores the received tokens
	tokens []TokenData

	// currentPos is the current read position
	currentPos int
}

// TokenData represents a single token with its attributes.
type TokenData struct {
	Term              string
	StartOffset       int
	EndOffset         int
	PositionIncrement int
}

// NewTeeSinkTokenFilter creates a new TeeSinkTokenFilter wrapping the given input.
func NewTeeSinkTokenFilter(input TokenStream) *TeeSinkTokenFilter {
	return &TeeSinkTokenFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		sinks:         make([]*TokenSink, 0),
	}
}

// NewSinkTokenStream creates a new sink that will receive tokens from this filter.
func (f *TeeSinkTokenFilter) NewSinkTokenStream() *TokenSink {
	sink := &TokenSink{
		tokens:     make([]TokenData, 0),
		currentPos: 0,
	}
	f.sinks = append(f.sinks, sink)
	return sink
}

// IncrementToken processes the next token and tees it to all sinks.
func (f *TeeSinkTokenFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}

	if hasToken {
		// Capture token data
		token := TokenData{}

		if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				token.Term = termAttr.String()
			}
		}

		if attr := f.GetAttributeSource().GetAttribute("OffsetAttribute"); attr != nil {
			if offsetAttr, ok := attr.(OffsetAttribute); ok {
				token.StartOffset = offsetAttr.StartOffset()
				token.EndOffset = offsetAttr.EndOffset()
			}
		}

		if attr := f.GetAttributeSource().GetAttribute("PositionIncrementAttribute"); attr != nil {
			if posAttr, ok := attr.(PositionIncrementAttribute); ok {
				token.PositionIncrement = posAttr.GetPositionIncrement()
			}
		}

		// Tee to all sinks
		for _, sink := range f.sinks {
			sink.tokens = append(sink.tokens, token)
		}
	}

	return hasToken, nil
}

// GetSinks returns all registered sinks.
func (f *TeeSinkTokenFilter) GetSinks() []*TokenSink {
	return f.sinks
}

// TokenSink methods

// IncrementToken returns the next token from the sink.
func (s *TokenSink) IncrementToken() (bool, *TokenData) {
	if s.currentPos >= len(s.tokens) {
		return false, nil
	}
	token := &s.tokens[s.currentPos]
	s.currentPos++
	return true, token
}

// Reset resets the sink to allow re-iteration.
func (s *TokenSink) Reset() {
	s.currentPos = 0
}

// GetTokens returns all tokens in the sink.
func (s *TokenSink) GetTokens() []TokenData {
	return s.tokens
}

// Size returns the number of tokens in the sink.
func (s *TokenSink) Size() int {
	return len(s.tokens)
}
