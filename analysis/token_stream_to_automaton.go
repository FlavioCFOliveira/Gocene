// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"github.com/FlavioCFOliveira/Gocene/analysis/tokenattributes"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// TokenStreamToAutomaton consumes a TokenStream and creates an Automaton where the transition labels are UTF8
// bytes (or Unicode code points if unicodeArcs is true) from the TermToBytesRefAttribute.
// Between tokens we insert POS_SEP and for holes we insert HOLE.
type TokenStreamToAutomaton struct {
	preservePositionIncrements bool
	finalOffsetGapAsHole       bool
	unicodeArcs               bool
	tokenSeparator             rune
}

// NewTokenStreamToAutomaton creates a new TokenStreamToAutomaton with default settings.
func NewTokenStreamToAutomaton() *TokenStreamToAutomaton {
	return &TokenStreamToAutomaton{
		preservePositionIncrements: true,
	}
}

// NewTokenStreamToAutomatonWithSeparator creates a new TokenStreamToAutomaton with a custom separator.
func NewTokenStreamToAutomatonWithSeparator(sep rune) *TokenStreamToAutomaton {
	return &TokenStreamToAutomaton{
		preservePositionIncrements: true,
		tokenSeparator:            sep,
	}
}

func (ts *TokenStreamToAutomaton) SetPreservePositionIncrements(enable bool) {
	ts.preservePositionIncrements = enable
}

func (ts *TokenStreamToAutomaton) SetFinalOffsetGapAsHole(enable bool) {
	ts.finalOffsetGapAsHole = enable
}

func (ts *TokenStreamToAutomaton) SetUnicodeArcs(enable bool) {
	ts.unicodeArcs = enable
}

func (ts *TokenStreamToAutomaton) ToAutomaton(in TokenStream) (*automaton.Automaton, error) {
	builder := automaton.NewBuilder()
	builder.CreateState()

	termBytesAtt := in.GetAttributeSource().GetAttribute(TermToBytesRefAttributeType).(TermToBytesRefAttribute)
	posIncAtt := in.GetAttributeSource().GetAttribute(tokenattributes.PositionIncrementAttributeType).(tokenattributes.PositionIncrementAttribute)
	posLengthAtt := in.GetAttributeSource().GetAttribute(PositionLengthAttributeType).(PositionLengthAttribute)
	offsetAtt := in.GetAttributeSource().GetAttribute(OffsetAttributeType).(OffsetAttribute)

	if err := in.Reset(); err != nil {
		return nil, err
	}

	// Simple slice-based positions.
	posList := make([]*position, 0)
	pos := -1
	freedPos := 0
	maxOffset := 0

	for {
		ok, err := in.IncrementToken()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}

		posInc := posIncAtt.GetPositionIncrement()
		if !ts.preservePositionIncrements && posInc > 1 {
			posInc = 1
		}

		if posInc > 0 {
			pos += posInc
			for len(posList) <= pos {
				posList = append(posList, &position{arriving: -1, leaving: -1})
			}
			posData := posList[pos]

			if posData.arriving == -1 {
				if pos == 0 {
					posData.leaving = 0
				} else {
					posData.leaving = builder.CreateState()
					ts.addHoles(builder, posList, pos)
				}
			} else {
				posData.leaving = builder.CreateState()
				builder.AddTransitionSingle(posData.arriving, posData.leaving, POS_SEP)
				if posInc > 1 {
					ts.addHoles(builder, posList, pos)
				}
			}
			for freedPos <= pos {
				freePosData := posList[freedPos]
				if freePosData.arriving == -1 || freePosData.leaving == -1 {
					break
				}
				freedPos++
			}
		}

		endPos := pos + posLengthAtt.GetPositionLength()
		for len(posList) <= endPos {
			posList = append(posList, &position{arriving: -1, leaving: -1})
		}
		endPosData := posList[endPos]
		if endPosData.arriving == -1 {
			endPosData.arriving = builder.CreateState()
		}

		termUTF8 := termBytesAtt.GetBytesRef().ValidBytes()
		termLen := len(termUTF8)
		var termUnicode []int
		if ts.unicodeArcs {
			s := string(termUTF8)
			termUnicode = make([]int, 0, len(s))
			for _, r := range s {
				termUnicode = append(termUnicode, int(r))
			}
			termLen = len(termUnicode)
		}

		state := posList[pos].leaving
		for i := 0; i < termLen; i++ {
			var nextState int
			if i == termLen-1 {
				nextState = endPosData.arriving
			} else {
				nextState = builder.CreateState()
			}
			var c int
			if ts.unicodeArcs {
				c = termUnicode[i]
			} else {
				c = int(termUTF8[i] & 0xff)
			}
			builder.AddTransitionSingle(state, nextState, c)
			state = nextState
		}
		if offsetAtt.EndOffset() > maxOffset {
			maxOffset = offsetAtt.EndOffset()
		}
	}

	if err := in.End(); err != nil {
		return nil, err
	}

	endPosInc := posIncAtt.GetPositionIncrement()
	if endPosInc == 0 && ts.finalOffsetGapAsHole && offsetAtt.EndOffset() > maxOffset {
		endPosInc = 1
	} else if endPosInc > 0 && !ts.preservePositionIncrements {
		endPosInc = 0
	}

	var endState int
	if endPosInc > 0 {
		endState = builder.CreateState()
		lastState := endState
		for endPosInc > 0 {
			state1 := builder.CreateState()
			builder.AddTransitionSingle(lastState, state1, HOLE)
			endPosInc--
			if endPosInc == 0 {
				builder.SetAccept(state1, true)
				break
			}
			state2 := builder.CreateState()
			builder.AddTransitionSingle(state1, state2, POS_SEP)
			lastState = state2
		}
	} else {
		endState = -1
	}

	pos++
	for pos < len(posList) {
		posData := posList[pos]
		if posData.arriving != -1 {
			if endState != -1 {
				builder.AddTransitionSingle(posData.arriving, endState, POS_SEP)
			} else {
				builder.SetAccept(posData.arriving, true)
			}
		}
		pos++
	}

	return builder.Finish(), nil
}

type position struct {
	arriving int
	leaving  int
}

func (ts *TokenStreamToAutomaton) addHoles(builder *automaton.Builder, positions []*position, pos int) {
	posData := positions[pos]
	prevPosData := positions[pos-1]

	for posData.arriving == -1 || prevPosData.leaving == -1 {
		if posData.arriving == -1 {
			posData.arriving = builder.CreateState()
			builder.AddTransitionSingle(posData.arriving, posData.leaving, POS_SEP)
		}
		if prevPosData.leaving == -1 {
			if pos == 1 {
				prevPosData.leaving = 0
			} else {
				prevPosData.leaving = builder.CreateState()
			}
			if prevPosData.arriving != -1 {
				builder.AddTransitionSingle(prevPosData.arriving, prevPosData.leaving, POS_SEP)
			}
		}
		builder.AddTransitionSingle(prevPosData.leaving, posData.arriving, HOLE)
		pos--
		if pos <= 0 {
			break
		}
		posData = positions[pos]
		prevPosData = positions[pos-1]
	}
}
