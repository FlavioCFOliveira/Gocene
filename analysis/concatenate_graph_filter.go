// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// ConcatenateGraphFilter concatenates/joins every incoming token with a separator into one output token for every path
// through the token stream (which is a graph). In simple cases this yields one token, but in the
// presence of any tokens with a zero positionIncrement (e.g. synonyms) it will be more.
type ConcatenateGraphFilter struct {
	*BaseTokenStream

	inputTokenStream           TokenStream
	tokenSeparator             rune
	preservePositionIncrements bool
	maxGraphExpansions         int

	finiteStrings *automaton.LimitedFiniteStringsIterator
	charTermAttr  CharTermAttribute
	wasReset      bool
	endOffset     int
}

const SepLabel = 0x001f
const DefaultMaxGraphExpansions = automaton.DefaultDeterminizeWorkLimit
const DefaultTokenSeparator = rune(SepLabel)
const DefaultPreserveSep = true
const DefaultPreservePositionIncrements = true

// NewConcatenateGraphFilter creates a new ConcatenateGraphFilter.
func NewConcatenateGraphFilter(inputTokenStream TokenStream, tokenSeparator rune, preservePositionIncrements bool, maxGraphExpansions int) *ConcatenateGraphFilter {
	return &ConcatenateGraphFilter{
		BaseTokenStream:           NewBaseTokenStream(),
		inputTokenStream:           inputTokenStream,
		tokenSeparator:             tokenSeparator,
		preservePositionIncrements: preservePositionIncrements,
		maxGraphExpansions:         maxGraphExpansions,
		endOffset:                 -1,
	}
}

// NewConcatenateGraphFilterDefault creates a ConcatenateGraphFilter with default settings.
func NewConcatenateGraphFilterDefault(inputTokenStream TokenStream) *ConcatenateGraphFilter {
	return NewConcatenateGraphFilter(
		inputTokenStream,
		DEFAULT_TOKEN_SEPARATOR,
		DEFAULT_PRESERVE_POSITION_INCREMENTS,
		DEFAULT_MAX_GRAPH_EXPANSIONS,
	)
}

// NewConcatenateGraphFilterPreserve creates a ConcatenateGraphFilter with specified flags.
func NewConcatenateGraphFilterPreserve(inputTokenStream TokenStream, preserveSep, preservePositionIncrements bool, maxGraphExpansions int) *ConcatenateGraphFilter {
	sep := DEFAULT_TOKEN_SEPARATOR
	if !preserveSep {
		sep = 0
	}
	return NewConcatenateGraphFilter(inputTokenStream, sep, preservePositionIncrements, maxGraphExpansions)
}

func (f *ConcatenateGraphFilter) Reset() error {
	if err := f.BaseTokenStream.Reset(); err != nil {
		return err
	}
	f.charTermAttr = f.GetAttribute(CharTermAttributeType).(*CharTermAttribute)
	f.wasReset = true
	f.finiteStrings = nil
	return nil
}

func (f *ConcatenateGraphFilter) IncrementToken() (bool, error) {
	if f.finiteStrings == nil {
		if !f.wasReset {
			return false, fmt.Errorf("reset() missing before incrementToken")
		}
		auto, err := f.ToAutomaton(false)
		if err != nil {
			return false, err
		}
		f.finiteStrings = automaton.NewLimitedFiniteStringsIterator(auto, f.maxGraphExpansions)
		f.endOffset = f.inputTokenStream.GetAttribute(OffsetAttributeType).(*OffsetAttribute).EndOffset()
	}

	stringRef := f.finiteStrings.Next()
	if stringRef == nil {
		return false, nil
	}

	f.ClearAttributes()

	if f.finiteStrings.Size() > 1 {
		f.GetAttribute(PositionIncrementAttributeType).(*PositionIncrementAttribute).SetPositionIncrement(0)
	}

	f.GetAttribute(OffsetAttributeType).(*OffsetAttribute).SetOffset(0, f.endOffset)

	// Convert internal IntsRef to UTF-8 bytes then to UTF-16 for CharTermAttribute
	bytes := util.ToBytesRef(stringRef)
	if f.charTermAttr != nil {
		f.charTermAttr.SetEmpty()
		f.charTermAttr.AppendString(string(bytes))
	}

	return true, nil
}

func (f *ConcatenateGraphFilter) End() error {
	if err := f.BaseTokenStream.End(); err != nil {
		return err
	}
	if f.finiteStrings == nil {
		if err := f.inputTokenStream.End(); err != nil {
			return err
		}
	}
	if f.endOffset != -1 {
		f.GetAttribute(OffsetAttributeType).(*OffsetAttribute).SetOffset(0, f.endOffset)
	}
	return nil
}

func (f *ConcatenateGraphFilter) Close() error {
	if err := f.BaseTokenStream.Close(); err != nil {
		return err
	}
	if err := f.inputTokenStream.Close(); err != nil {
		return err
	}
	f.finiteStrings = nil
	f.wasReset = false
	f.endOffset = -1
	return nil
}

func (f *ConcatenateGraphFilter) ToAutomatonDefault() (*automaton.Automaton, error) {
	return f.ToAutomaton(false)
}

func (f *ConcatenateGraphFilter) ToAutomaton(unicodeAware bool) (*automaton.Automaton, error) {
	var tsta *TokenStreamToAutomaton
	if f.tokenSeparator != 0 {
		tsta = NewTokenStreamToAutomatonWithSeparator(f.tokenSeparator)
	} else {
		tsta = NewTokenStreamToAutomaton()
	}
	tsta.SetPreservePositionIncrements(f.preservePositionIncrements)
	tsta.SetUnicodeArcs(unicodeAware)

	auto, err := tsta.ToAutomaton(f.inputTokenStream)
	if err != nil {
		return nil, err
	}

	auto = f.replaceSep(auto, f.tokenSeparator)
	return automaton.Determinize(auto, f.maxGraphExpansions)
}

func (f *ConcatenateGraphFilter) replaceSep(a *automaton.Automaton, tokenSeparator rune) *automaton.Automaton {
	result := automaton.NewAutomaton()

	numStates := a.NumStates()
	for s := 0; s < numStates; s++ {
		result.CreateState()
		result.SetAccept(s, a.IsAccept(s))
	}

	topoSortStates, err := automaton.TopoSortStates(a)
	if err != nil {
		return a
	}

	t := automaton.NewTransition()
	for i := 0; i < len(topoSortStates); i++ {
		state := topoSortStates[len(topoSortStates)-1-i]
		count := a.InitTransition(state, t)
		for j := 0; j < count; j++ {
			a.GetNextTransition(t)
			if t.Min == POS_SEP {
				if tokenSeparator != 0 {
					result.AddTransition(state, t.Dest, int(tokenSeparator))
				} else {
					result.AddEpsilon(state, t.Dest)
				}
			} else if t.Min == HOLE {
				result.AddEpsilon(state, t.Dest)
			} else {
				result.AddTransition(state, t.Dest, t.Min, t.Max)
			}
		}
	}

	result.FinishState()
	return automaton.RemoveDeadStates(result)
}
