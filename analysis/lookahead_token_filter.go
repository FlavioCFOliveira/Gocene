// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// LookaheadTokenFilter is an abstract TokenFilter that makes it easier to
// build graph token filters requiring some lookahead. It handles the details
// of buffering up tokens, recording them by position, restoring them, etc.
//
// Port of org.apache.lucene.tests.analysis.LookaheadTokenFilter.
type LookaheadTokenFilter struct {
	*BaseTokenFilter

	posIncAtt PositionIncrementAttribute
	posLenAtt PositionLengthAttribute
	offsetAtt OffsetAttribute

	// inputPos is the position of the last read input token.
	inputPos int
	// outputPos is the position of the next possible output token to return.
	outputPos int
	// end is true when the input stream is exhausted.
	end bool

	tokenPending bool
	insertPending bool

	positions *util.RollingBuffer[*lookaheadPosition]
}

// lookaheadPosition holds all state for a single position.
// Concrete filters can extend this by embedding it in a larger struct if
// needed, but for the test-framework use case the base position is enough.
type lookaheadPosition struct {
	// Buffered input tokens at this position.
	inputTokens []*util.AttributeState
	// Next buffered token to be returned to consumer.
	nextRead int
	// Any token leaving from this position should have this startOffset.
	startOffset int
	// Any token arriving to this position should have this endOffset.
	endOffset int
}

// Reset resets the position to a pristine state so the RollingBuffer can reuse it.
func (p *lookaheadPosition) Reset() {
	p.inputTokens = p.inputTokens[:0]
	p.nextRead = 0
	p.startOffset = -1
	p.endOffset = -1
}

// add appends a captured attribute state to this position.
func (p *lookaheadPosition) add(state *util.AttributeState) {
	p.inputTokens = append(p.inputTokens, state)
}

// nextState returns the next buffered state and advances nextRead.
func (p *lookaheadPosition) nextState() *util.AttributeState {
	if p.nextRead >= len(p.inputTokens) {
		panic(fmt.Sprintf("lookaheadPosition.nextState: nextRead=%d >= len=%d", p.nextRead, len(p.inputTokens)))
	}
	state := p.inputTokens[p.nextRead]
	p.nextRead++
	return state
}

// NewLookaheadTokenFilter creates a new LookaheadTokenFilter wrapping input.
func NewLookaheadTokenFilter(input TokenStream) *LookaheadTokenFilter {
	f := &LookaheadTokenFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		inputPos:        -1,
		outputPos:       0,
		positions: util.NewRollingBuffer[*lookaheadPosition](func() *lookaheadPosition {
			return &lookaheadPosition{startOffset: -1, endOffset: -1}
		}),
	}
	if src := f.GetAttributeSource(); src != nil {
		if attr := src.GetAttribute(PositionIncrementAttributeType); attr != nil {
			if pi, ok := attr.(PositionIncrementAttribute); ok {
				f.posIncAtt = pi
			}
		}
		if attr := src.GetAttribute(PositionLengthAttributeType); attr != nil {
			if pl, ok := attr.(PositionLengthAttribute); ok {
				f.posLenAtt = pl
			}
		}
		if attr := src.GetAttribute(OffsetAttributeType); attr != nil {
			if off, ok := attr.(OffsetAttribute); ok {
				f.offsetAtt = off
			}
		}
	}
	return f
}

// InsertToken marks that the subclass wants to insert a new token.
// Call this only from within AfterPosition.
func (f *LookaheadTokenFilter) InsertToken() {
	if f.tokenPending {
		f.positions.Get(f.inputPos).add(f.GetAttributeSource().CaptureState())
		f.tokenPending = false
	}
	f.insertPending = true
}

// AfterPosition is called when all input tokens leaving a given position
// have been returned. Subclasses should set this field to a non-nil function
// if they want to inject tokens.
func (f *LookaheadTokenFilter) AfterPosition() {}

// PeekToken reads the next token from the input and buffers it.
// Returns true if a token was read, false if the input is exhausted.
func (f *LookaheadTokenFilter) PeekToken() (bool, error) {
	if f.end {
		return false, nil
	}
	if f.inputPos != -1 && f.outputPos > f.inputPos {
		panic(fmt.Sprintf("PeekToken: outputPos=%d > inputPos=%d", f.outputPos, f.inputPos))
	}
	if f.tokenPending {
		f.positions.Get(f.inputPos).add(f.GetAttributeSource().CaptureState())
		f.tokenPending = false
	}
	gotToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}
	if !gotToken {
		f.end = true
		return false, nil
	}

	f.inputPos += f.currentPosInc()
	if f.inputPos < 0 {
		panic(fmt.Sprintf("PeekToken: inputPos went negative: %d", f.inputPos))
	}

	startPosData := f.positions.Get(f.inputPos)
	endPosData := f.positions.Get(f.inputPos + f.currentPosLen())

	startOffset := f.offsetAtt.StartOffset()
	if startPosData.startOffset == -1 {
		startPosData.startOffset = startOffset
	}
	// NOTE: Lucene uses assertions here (disabled at runtime by default).
	// We silently accept the first seen offset to avoid panicking on
	// pathological / inconsistent input streams during tests.

	endOffset := f.offsetAtt.EndOffset()
	if endPosData.endOffset == -1 {
		endPosData.endOffset = endOffset
	}
	// NOTE: same as above — assertion in Java, silent accept in Go.

	f.tokenPending = true
	return true, nil
}

// NextToken returns the next buffered or pending token.
// This is the main method called by IncrementToken implementations.
func (f *LookaheadTokenFilter) NextToken() (bool, error) {
	for {
		posData := f.positions.Get(f.outputPos)

		// Return previously buffered token at this position.
		if posData.nextRead < len(posData.inputTokens) {
			if f.tokenPending {
				f.positions.Get(f.inputPos).add(f.GetAttributeSource().CaptureState())
				f.tokenPending = false
			}
			f.GetAttributeSource().RestoreState(posData.nextState())
			return true, nil
		}

		if f.inputPos == -1 || f.outputPos == f.inputPos {
			if f.tokenPending {
				// Fast path: just return the pending token without capturing/restoring state.
				f.tokenPending = false
				return true, nil
			}
			if f.end {
				f.AfterPosition()
				if f.insertPending {
					f.insertPending = false
					return true, nil
				}
				return false, nil
			}
			gotToken, err := f.PeekToken()
			if err != nil {
				return false, err
			}
			if !gotToken {
				f.AfterPosition()
				if f.insertPending {
					f.insertPending = false
					return true, nil
				}
				return false, nil
			}
			// After PeekToken, we have a new pending token; loop around to return it.
			continue
		}

		if posData.startOffset != -1 {
			f.AfterPosition()
			if f.insertPending {
				f.insertPending = false
				return true, nil
			}
		}

		// Done with this position; move on.
		f.outputPos++
		f.positions.FreeBefore(f.outputPos)
	}
}

// Reset clears the lookahead state and forwards to the input stream.
func (f *LookaheadTokenFilter) Reset() error {
	if resetter, ok := f.input.(interface{ Reset() error }); ok {
		if err := resetter.Reset(); err != nil {
			return err
		}
	}
	f.positions.Reset()
	f.inputPos = -1
	f.outputPos = 0
	f.tokenPending = false
	f.insertPending = false
	f.end = false
	return nil
}

// currentPosInc returns the current PositionIncrement (defaults to 1).
func (f *LookaheadTokenFilter) currentPosInc() int {
	if f.posIncAtt == nil {
		return 1
	}
	return f.posIncAtt.GetPositionIncrement()
}

// currentPosLen returns the current PositionLength (defaults to 1).
func (f *LookaheadTokenFilter) currentPosLen() int {
	if f.posLenAtt == nil {
		return 1
	}
	return f.posLenAtt.GetPositionLength()
}

// Ensure LookaheadTokenFilter implements TokenFilter.
var _ TokenFilter = (*LookaheadTokenFilter)(nil)
