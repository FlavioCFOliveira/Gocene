// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"math/rand"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// MockGraphTokenFilter randomly inserts overlapped (posInc=0) tokens with
// posLength sometimes > 1. The chain must have an OffsetAttribute.
//
// Port of org.apache.lucene.tests.analysis.MockGraphTokenFilter.
type MockGraphTokenFilter struct {
	*LookaheadTokenFilter

	seed   int64
	random *rand.Rand
}

// NewMockGraphTokenFilter creates a MockGraphTokenFilter that uses random
// to decide when/what to inject. The random source is snapshotted (seeded)
// so that Reset() produces deterministic output for the same input.
func NewMockGraphTokenFilter(random *rand.Rand, input TokenStream) *MockGraphTokenFilter {
	seed := random.Int63()
	f := &MockGraphTokenFilter{
		LookaheadTokenFilter: NewLookaheadTokenFilter(input),
		seed:                 seed,
	}
	return f
}

// AfterPosition is called after all input tokens at the current position have
// been returned. With probability 1/7, it inserts a synthetic token with
// posInc=0 and a random position length (1..5).
func (f *MockGraphTokenFilter) AfterPosition() {
	if f.random == nil {
		return
	}
	if f.random.Intn(7) != 5 {
		return
	}

	posLength := f.random.Intn(5) + 1 // 1..5

	// Look ahead as needed until we figure out the right endOffset.
	endPosData := f.positions.Get(f.outputPos + posLength)
	for !f.end && endPosData.endOffset == -1 && f.inputPos <= (f.outputPos+posLength) {
		if _, err := f.PeekToken(); err != nil {
			break
		}
	}

	if endPosData.endOffset == -1 {
		// Tokens ended before our posLength, or posLength ended inside a hole.
		return
	}

	f.InsertToken()
	f.GetAttributeSource().ClearAttributes()
	if pl, ok := f.GetAttributeSource().GetAttribute(PositionLengthAttributeType).(PositionLengthAttribute); ok && pl != nil {
		pl.SetPositionLength(posLength)
	}
	if termAtt, ok := f.GetAttributeSource().GetAttribute(CharTermAttributeType).(CharTermAttribute); ok && termAtt != nil {
		termAtt.SetEmpty()
		termAtt.AppendString(util.RandomUnicodeString(f.random, 10))
	}
	if pi, ok := f.GetAttributeSource().GetAttribute(PositionIncrementAttributeType).(PositionIncrementAttribute); ok && pi != nil {
		pi.SetPositionIncrement(0)
	}
	if off, ok := f.GetAttributeSource().GetAttribute(OffsetAttributeType).(OffsetAttribute); ok && off != nil {
		startOffset := f.positions.Get(f.outputPos).startOffset
		if startOffset >= 0 {
			off.SetStartOffset(startOffset)
		}
		off.SetEndOffset(endPosData.endOffset)
	}
}

// Reset re-seeds the internal random source deterministically.
func (f *MockGraphTokenFilter) Reset() error {
	if err := f.LookaheadTokenFilter.Reset(); err != nil {
		return err
	}
	f.random = rand.New(rand.NewSource(f.seed))
	return nil
}

// IncrementToken returns the next token (possibly injected).
func (f *MockGraphTokenFilter) IncrementToken() (bool, error) {
	if f.random == nil {
		return false, nil
	}
	return f.NextToken()
}

// Ensure MockGraphTokenFilter implements TokenFilter.
var _ TokenFilter = (*MockGraphTokenFilter)(nil)
