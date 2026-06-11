// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"math/rand"
)

// MockHoleInjectingTokenFilter randomly injects holes (similar to what a
// stopfilter would do) into a token stream by increasing position increments.
//
// Port of org.apache.lucene.tests.analysis.MockHoleInjectingTokenFilter.
type MockHoleInjectingTokenFilter struct {
	*BaseTokenFilter

	randomSeed int64
	random     *rand.Rand
	posIncAtt  PositionIncrementAttribute
	posLenAtt  PositionLengthAttribute
	maxPos     int
	pos        int
}

// NewMockHoleInjectingTokenFilter creates a MockHoleInjectingTokenFilter that
// uses random to decide when/what holes to inject. The random source is
// snapshotted (seeded) so that Reset() produces deterministic output.
func NewMockHoleInjectingTokenFilter(random *rand.Rand, input TokenStream) *MockHoleInjectingTokenFilter {
	seed := random.Int63()
	f := &MockHoleInjectingTokenFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		randomSeed:      seed,
	}
	if src := f.GetAttributeSource(); src != nil {
		if att := src.GetAttribute(PositionIncrementAttributeType); att != nil {
			if pia, ok := att.(PositionIncrementAttribute); ok {
				f.posIncAtt = pia
			}
		}
		if att := src.GetAttribute(PositionLengthAttributeType); att != nil {
			if pla, ok := att.(PositionLengthAttribute); ok {
				f.posLenAtt = pla
			}
		}
	}
	return f
}

// Reset re-seeds the internal random source deterministically.
func (f *MockHoleInjectingTokenFilter) Reset() error {
	if resetter, ok := f.input.(interface{ Reset() error }); ok {
		if err := resetter.Reset(); err != nil {
			return err
		}
	}
	f.random = rand.New(rand.NewSource(f.randomSeed))
	f.maxPos = -1
	f.pos = -1
	return nil
}

// IncrementToken returns the next token, possibly with a hole injected.
func (f *MockHoleInjectingTokenFilter) IncrementToken() (bool, error) {
	gotToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}
	if !gotToken {
		return false, nil
	}

	posInc := f.posIncAtt.GetPositionIncrement()
	nextPos := f.pos + posInc

	// Inject a hole only where it won't mess up the graph:
	// posInc > 0 (not a graph token) and maxPos <= nextPos (no graph tokens
	// extend beyond this position).
	if posInc > 0 && f.maxPos <= nextPos && f.random.Intn(5) == 3 {
		holeSize := f.random.Intn(5) + 1 // 1..5
		f.posIncAtt.SetPositionIncrement(posInc + holeSize)
		nextPos += holeSize
	}

	f.pos = nextPos
	posLen := 1
	if f.posLenAtt != nil {
		posLen = f.posLenAtt.GetPositionLength()
	}
	if newMax := f.pos + posLen; newMax > f.maxPos {
		f.maxPos = newMax
	}

	return true, nil
}

// Ensure MockHoleInjectingTokenFilter implements TokenFilter.
var _ TokenFilter = (*MockHoleInjectingTokenFilter)(nil)
