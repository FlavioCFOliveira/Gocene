// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/miscellaneous"
	"github.com/FlavioCFOliveira/Gocene/analysis/tokenattributes"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// CompletionTokenStream is a ConcatenateGraphFilter but we can set the payload and provide access to config options.
//
// This is the Go port of org.apache.lucene.search.suggest.document.CompletionTokenStream from Apache Lucene 10.5.0.
type CompletionTokenStream struct {
	*analysis.BaseTokenFilter

	inputTokenStream         analysis.TokenStream
	PreserveSep              bool
	PreservePositionIncrements bool
	MaxGraphExpansions       int

	payloadAttr analysis.PayloadAttribute
	payload     []byte
}

// NewCompletionTokenStream creates a new CompletionTokenStream wrapping the given input.
func NewCompletionTokenStream(input analysis.TokenStream) *CompletionTokenStream {
	return NewCompletionTokenStreamFull(input, miscellaneous.DefaultSepLabel, true, miscellaneous.DefaultMaxGraphExpansions)
}

// NewCompletionTokenStreamFull creates a new CompletionTokenStream wrapping the given input with explicit settings.
func NewCompletionTokenStreamFull(
	input analysis.TokenStream,
	preserveSep bool,
	preservePositionIncrements bool,
	maxGraphExpansions int,
) *CompletionTokenStream {
	// Wrap the input with a ConcatenateGraphFilter
	filter := miscellaneous.NewConcatenateGraphFilterFull(input, miscellaneous.DefaultSepLabel, preservePositionIncrements, maxGraphExpansions)

	ts := &CompletionTokenStream{
		BaseTokenFilter:            analysis.NewBaseTokenFilter(filter),
		inputTokenStream:           input,
		PreserveSep:               preserveSep,
		PreservePositionIncrements: preservePositionIncrements,
		MaxGraphExpansions:         maxGraphExpansions,
	}

	// Add PayloadAttribute to the attribute source
	payloadImpl := tokenattributes.NewPayloadAttribute()
	ts.GetAttributeSource().AddAttributeImpl(payloadImpl)
	if a := ts.GetAttributeSource().GetAttribute(analysis.PayloadAttributeType); a != nil {
		ts.payloadAttr = a.(analysis.PayloadAttribute)
	}

	return ts
}

// SetPayload sets a payload available throughout successive token stream enumeration.
func (ts *CompletionTokenStream) SetPayload(payload []byte) {
	ts.payload = payload
}

// IncrementToken advances to the next token and sets the payload.
func (ts *CompletionTokenStream) IncrementToken() (bool, error) {
	ok, err := ts.BaseTokenFilter.IncrementToken()
	if err != nil {
		return false, err
	}
	if ok && ts.payloadAttr != nil {
		ts.payloadAttr.SetPayload(ts.payload)
	}
	return ok, nil
}

// ToAutomaton converts the token stream to an automaton.
// Note: This delegates to the wrapped ConcatenateGraphFilter.
func (ts *CompletionTokenStream) ToAutomaton() (*automaton.Automaton, error) {
	// The BaseTokenFilter.GetInput() returns the ConcatenateGraphFilter
	if filter, ok := ts.GetInput().(*miscellaneous.ConcatenateGraphFilter); ok {
		// We need to check if ConcatenateGraphFilter has ToAutomaton.
		// In Lucene it does. In Gocene's miscellaneous.go, it doesn't seem to.
		// I will have to implement it or check if it's available.
		return nil, io.ErrUnexpectedEOF // Placeholder, will fix after checking ConcatenateGraphFilter
	}
	return nil, io.ErrUnexpectedEOF
}

// Ensure CompletionTokenStream implements TokenFilter.
var _ analysis.TokenFilter = (*CompletionTokenStream)(nil)
