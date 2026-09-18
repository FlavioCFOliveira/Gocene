// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/tokenattributes"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// CompletionTokenStream is a ConcatenateGraphFilter but we can set the payload and provide access to config options.
//
// This is the Go port of org.apache.lucene.search.suggest.document.CompletionTokenStream from Apache Lucene 10.5.0.
type CompletionTokenStream struct {
	*analysis.BaseTokenFilter

	inputTokenStream           analysis.TokenStream
	PreserveSep                bool
	PreservePositionIncrements bool
	MaxGraphExpansions         int

	payloadAttr analysis.PayloadAttribute
	payload     []byte
}

// NewCompletionTokenStream creates a new CompletionTokenStream wrapping the given input.
func NewCompletionTokenStream(input analysis.TokenStream) *CompletionTokenStream {
	return NewCompletionTokenStreamFull(
		input,
		analysis.DefaultPreserveSep,
		analysis.DefaultPreservePositionIncrements,
		analysis.DefaultMaxGraphExpansions,
	)
}

// NewCompletionTokenStreamFull creates a new CompletionTokenStream wrapping the given input with explicit settings.
func NewCompletionTokenStreamFull(
	input analysis.TokenStream,
	preserveSep bool,
	preservePositionIncrements bool,
	maxGraphExpansions int,
) *CompletionTokenStream {
	// Wrap the input with a ConcatenateGraphFilter
	filter := analysis.NewConcatenateGraphFilterPreserve(input, preserveSep, preservePositionIncrements, maxGraphExpansions)

	ts := &CompletionTokenStream{
		BaseTokenFilter:            analysis.NewBaseTokenFilter(filter),
		inputTokenStream:           input,
		PreserveSep:                preserveSep,
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

// ToAutomatonDefault delegates to ConcatenateGraphFilter.toAutomaton().
//
// Mirrors CompletionTokenStream.toAutomaton() of Apache Lucene 10.5.0. Java
// overloads toAutomaton(); Go cannot, so the no-argument form carries the
// Default suffix already used by analysis.ConcatenateGraphFilter.
func (ts *CompletionTokenStream) ToAutomatonDefault() (*automaton.Automaton, error) {
	return ts.ToAutomaton(false)
}

// ToAutomaton delegates to ConcatenateGraphFilter.toAutomaton(boolean).
//
// Mirrors CompletionTokenStream.toAutomaton(boolean) of Apache Lucene 10.5.0.
func (ts *CompletionTokenStream) ToAutomaton(unicodeAware bool) (*automaton.Automaton, error) {
	filter, ok := ts.GetInput().(*analysis.ConcatenateGraphFilter)
	if !ok {
		return nil, fmt.Errorf("completion token stream: input is %T, not *analysis.ConcatenateGraphFilter", ts.GetInput())
	}
	return filter.ToAutomaton(unicodeAware)
}

// Ensure CompletionTokenStream implements TokenFilter.
var _ analysis.TokenFilter = (*CompletionTokenStream)(nil)
