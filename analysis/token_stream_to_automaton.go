// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"unicode/utf8"

	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// PosSep is the transition label inserted between two adjacent tokens.
// Mirrors Lucene's TokenStreamToAutomaton.POS_SEP (0x001F, INFORMATION
// SEPARATOR ONE).
const PosSep = 0x001F

// Hole is the transition label inserted to represent a missing position
// (e.g. a token that was removed by a StopFilter). Mirrors Lucene's
// TokenStreamToAutomaton.HOLE (0x001E, INFORMATION SEPARATOR TWO).
const Hole = 0x001E

// TokenStreamToAutomaton consumes a TokenStream and produces an Automaton
// whose transition labels are either UTF-8 bytes or Unicode code points
// from the TermToBytesRefAttribute. PosSep arcs are inserted between
// adjacent tokens; Hole arcs are inserted for gaps in position increments.
//
// This is the Go port of Lucene's
// org.apache.lucene.analysis.TokenStreamToAutomaton.
type TokenStreamToAutomaton struct {
	preservePositionIncrements bool
	finalOffsetGapAsHole       bool
	unicodeArcs                bool
}

// NewTokenStreamToAutomaton creates a new converter with
// PreservePositionIncrements=true and UnicodeArcs=false, matching the Java
// defaults.
func NewTokenStreamToAutomaton() *TokenStreamToAutomaton {
	return &TokenStreamToAutomaton{
		preservePositionIncrements: true,
	}
}

// SetPreservePositionIncrements controls whether position increments larger
// than 1 produce explicit Hole transitions. Defaults to true.
func (c *TokenStreamToAutomaton) SetPreservePositionIncrements(enable bool) {
	c.preservePositionIncrements = enable
}

// SetFinalOffsetGapAsHole controls whether a trailing end-offset gap
// produces a Hole transition at the tail of the automaton. Defaults to
// false.
func (c *TokenStreamToAutomaton) SetFinalOffsetGapAsHole(enable bool) {
	c.finalOffsetGapAsHole = enable
}

// SetUnicodeArcs makes transition labels Unicode code points instead of
// UTF-8 bytes. Defaults to false.
func (c *TokenStreamToAutomaton) SetUnicodeArcs(enable bool) {
	c.unicodeArcs = enable
}

// ChangeToken is a hook subclasses may use to alter a token's byte content
// (for example, to escape control characters that would otherwise collide
// with POS_SEP or HOLE). The default implementation returns the input
// unchanged. Mirrors Lucene's protected changeToken(BytesRef).
func (c *TokenStreamToAutomaton) ChangeToken(in *util.BytesRef) *util.BytesRef {
	return in
}

// position holds the per-position state used by the conversion. The arriving
// state is the destination of incoming transitions; the leaving state is
// the source of outgoing transitions for tokens starting at this position.
type position struct {
	arriving int
	leaving  int
}

// Reset implements util.Resettable so the position can be recycled by the
// RollingBuffer.
func (p *position) Reset() {
	p.arriving = -1
	p.leaving = -1
}

// ToAutomaton consumes the given TokenStream (which must already have been
// Reset by the caller — Lucene calls reset() inside but Gocene leaves that
// to the caller for parity with how TokenStream/Tokenizer Reset works in
// this codebase) and returns the constructed Automaton.
func (c *TokenStreamToAutomaton) ToAutomaton(in TokenStream) (*automaton.Automaton, error) {
	builder := automaton.NewBuilder()
	builder.CreateState()

	src, ok := in.(interface{ GetAttributeSource() *AttributeSource })
	if !ok {
		return nil, errNoAttributeSource
	}
	as := src.GetAttributeSource()
	// TermToBytesRefAttribute is an interface; locate any registered
	// attribute that implements it (CharTermAttribute does).
	var termBytesAtt TermToBytesRefAttribute
	for _, ty := range as.GetAttributeClasses() {
		if attr := as.GetAttributeByType(ty); attr != nil {
			if tb, ok := attr.(TermToBytesRefAttribute); ok {
				termBytesAtt = tb
				break
			}
		}
	}
	posIncAtt, _ := as.GetAttribute("PositionIncrementAttribute").(PositionIncrementAttribute)
	posLengthAttRaw := as.GetAttributeByType(PositionLengthAttributeType)
	var posLengthAtt PositionLengthAttribute
	if posLengthAttRaw != nil {
		posLengthAtt, _ = posLengthAttRaw.(PositionLengthAttribute)
	}
	offsetAtt, _ := as.GetAttribute("OffsetAttribute").(OffsetAttribute)

	if termBytesAtt == nil {
		return nil, errMissingTermBytes
	}

	if resetter, ok := in.(interface{ Reset() error }); ok {
		if err := resetter.Reset(); err != nil {
			return nil, err
		}
	}

	positions := util.NewRollingBuffer[*position](func() *position {
		return &position{arriving: -1, leaving: -1}
	})

	pos := -1
	freedPos := 0
	var posData *position
	maxOffset := 0

	for {
		hasToken, err := in.IncrementToken()
		if err != nil {
			return nil, err
		}
		if !hasToken {
			break
		}

		posInc := 1
		if posIncAtt != nil {
			posInc = posIncAtt.GetPositionIncrement()
		}
		if !c.preservePositionIncrements && posInc > 1 {
			posInc = 1
		}

		if posInc > 0 {
			pos += posInc
			posData = positions.Get(pos)
			if posData.arriving == -1 {
				if pos == 0 {
					posData.leaving = 0
				} else {
					posData.leaving = builder.CreateState()
					c.addHoles(builder, positions, pos)
				}
			} else {
				posData.leaving = builder.CreateState()
				builder.AddTransitionSingle(posData.arriving, posData.leaving, PosSep)
				if posInc > 1 {
					c.addHoles(builder, positions, pos)
				}
			}
			for freedPos <= pos {
				freePosData := positions.Get(freedPos)
				if freePosData.arriving == -1 || freePosData.leaving == -1 {
					break
				}
				positions.FreeBefore(freedPos)
				freedPos++
			}
		}

		posLen := 1
		if posLengthAtt != nil {
			posLen = posLengthAtt.GetPositionLength()
		}
		endPos := pos + posLen

		termUTF8 := c.ChangeToken(termBytesAtt.GetBytesRef())
		var termUnicode []int

		endPosData := positions.Get(endPos)
		if endPosData.arriving == -1 {
			endPosData.arriving = builder.CreateState()
		}

		termLen := termUTF8.Length
		if c.unicodeArcs {
			utf16 := termUTF8.String()
			termUnicode = make([]int, 0, len([]rune(utf16)))
			for _, r := range utf16 {
				termUnicode = append(termUnicode, int(r))
			}
			termLen = len(termUnicode)
		}

		state := posData.leaving
		for byteIdx := 0; byteIdx < termLen; byteIdx++ {
			var nextState int
			if byteIdx == termLen-1 {
				nextState = endPosData.arriving
			} else {
				nextState = builder.CreateState()
			}
			var lbl int
			if c.unicodeArcs {
				lbl = termUnicode[byteIdx]
			} else {
				lbl = int(termUTF8.Bytes[termUTF8.Offset+byteIdx]) & 0xff
			}
			builder.AddTransitionSingle(state, nextState, lbl)
			state = nextState
		}

		if offsetAtt != nil {
			endOff := offsetAtt.EndOffset()
			if endOff > maxOffset {
				maxOffset = endOff
			}
		}
	}

	if err := in.End(); err != nil {
		return nil, err
	}

	endPosInc := 0
	if posIncAtt != nil {
		endPosInc = posIncAtt.GetPositionIncrement()
	}
	endOff := 0
	if offsetAtt != nil {
		endOff = offsetAtt.EndOffset()
	}
	if endPosInc == 0 && c.finalOffsetGapAsHole && endOff > maxOffset {
		endPosInc = 1
	} else if endPosInc > 0 && !c.preservePositionIncrements {
		endPosInc = 0
	}

	endState := -1
	if endPosInc > 0 {
		endState = builder.CreateState()
		lastState := endState
		for {
			state1 := builder.CreateState()
			builder.AddTransitionSingle(lastState, state1, Hole)
			endPosInc--
			if endPosInc == 0 {
				builder.SetAccept(state1, true)
				break
			}
			state2 := builder.CreateState()
			builder.AddTransitionSingle(state1, state2, PosSep)
			lastState = state2
		}
	}

	pos++
	for pos <= positions.MaxPos() {
		posData = positions.Get(pos)
		if posData.arriving != -1 {
			if endState != -1 {
				builder.AddTransitionSingle(posData.arriving, endState, PosSep)
			} else {
				builder.SetAccept(posData.arriving, true)
			}
		}
		pos++
	}

	return builder.Finish(), nil
}

// addHoles inserts Hole transitions between any unfilled positions up to
// (but not including) pos, walking backwards until the first filled
// position. Mirrors Lucene's static helper of the same name.
func (c *TokenStreamToAutomaton) addHoles(
	builder *automaton.Builder,
	positions *util.RollingBuffer[*position],
	pos int,
) {
	posData := positions.Get(pos)
	prevPosData := positions.Get(pos - 1)

	for posData.arriving == -1 || prevPosData.leaving == -1 {
		if posData.arriving == -1 {
			posData.arriving = builder.CreateState()
			builder.AddTransitionSingle(posData.arriving, posData.leaving, PosSep)
		}
		if prevPosData.leaving == -1 {
			if pos == 1 {
				prevPosData.leaving = 0
			} else {
				prevPosData.leaving = builder.CreateState()
			}
			if prevPosData.arriving != -1 {
				builder.AddTransitionSingle(prevPosData.arriving, prevPosData.leaving, PosSep)
			}
		}
		builder.AddTransitionSingle(prevPosData.leaving, posData.arriving, Hole)
		pos--
		if pos <= 0 {
			break
		}
		posData = prevPosData
		prevPosData = positions.Get(pos - 1)
	}
}

// utf8RuneLen is the canonical UTF-8 byte length of a single rune; helper
// kept here to make the dependency on unicode/utf8 explicit.
//
// by future ChangeToken overrides that need to compute byte costs.
//
//nolint:unused // retained for symmetry with the Java reference; may be used
func utf8RuneLen(r rune) int { return utf8.RuneLen(r) }

// Sentinel errors so callers can introspect failures.
var (
	errNoAttributeSource = errAutomatonConvertF("token stream has no AttributeSource")
	errMissingTermBytes  = errAutomatonConvertF("token stream is missing TermToBytesRefAttribute")
)

// errAutomatonConvertF wraps a string as a simple error value (kept local
// to avoid a circular dependency on errors package conventions elsewhere).
func errAutomatonConvertF(s string) error { return automatonConvertErr(s) }

// automatonConvertErr is a typed string error; implements the error
// interface via its String method.
type automatonConvertErr string

func (e automatonConvertErr) Error() string { return string(e) }
