// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package kuromoji

import (
	"unicode"

	"github.com/FlavioCFOliveira/Gocene/analysis/kuromoji/dict"
	"github.com/FlavioCFOliveira/Gocene/analysis/morph"
)

const (
	maxBacktraceBufferLen = 4096
	maxUnknownWordLength  = 1024
)

// positionNode represents a single back-trace node in the Viterbi lattice.
type positionNode struct {
	cost        int
	backPos     int
	backWordPos int
	backType    morph.TokenType
	backID      int
	backIndex   int
}

// latticePosition holds all nodes for a given input position.
type latticePosition struct {
	pos   int
	nodes []positionNode
}

func newLatticePosition(pos int) *latticePosition {
	return &latticePosition{pos: pos}
}

func (lp *latticePosition) getCount() int { return len(lp.nodes) }

func (lp *latticePosition) add(cost, backPos, backWordPos int, backType morph.TokenType, backID, backIndex int) {
	lp.nodes = append(lp.nodes, positionNode{
		cost:        cost,
		backPos:     backPos,
		backWordPos: backWordPos,
		backType:    backType,
		backID:      backID,
		backIndex:   backIndex,
	})
}

func (lp *latticePosition) getCost(idx int) int {
	if idx < len(lp.nodes) {
		return lp.nodes[idx].cost
	}
	return 0
}

func (lp *latticePosition) getBackPos(idx int) int {
	if idx < len(lp.nodes) {
		return lp.nodes[idx].backPos
	}
	return 0
}

func (lp *latticePosition) getBackWordPos(idx int) int {
	if idx < len(lp.nodes) {
		return lp.nodes[idx].backWordPos
	}
	return 0
}

func (lp *latticePosition) getBackType(idx int) morph.TokenType {
	if idx < len(lp.nodes) {
		return lp.nodes[idx].backType
	}
	return morph.TokenTypeUnknown
}

func (lp *latticePosition) getBackID(idx int) int {
	if idx < len(lp.nodes) {
		return lp.nodes[idx].backID
	}
	return 0
}

func (lp *latticePosition) getBackIndex(idx int) int {
	if idx < len(lp.nodes) {
		return lp.nodes[idx].backIndex
	}
	return 0
}

// Viterbi implements the rolling Viterbi search for Japanese morphological
// analysis.
//
// This is the Go port of the Japanese-specific
// org.apache.lucene.analysis.ja.ViterbiNBest, from Apache Lucene 10.4.0.
type Viterbi struct {
	sysDict            *dict.TokenInfoDictionary
	unkDict            *dict.UnknownDictionary
	connectionCosts    *dict.ConnectionCosts
	userDict           *dict.UserDictionary
	charDef            *dict.CharacterDefinition
	discardPunctuation bool
	searchMode         bool
	extendedMode       bool
	outputCompounds    bool

	input            []rune
	inputLen         int
	pos              int
	end              bool
	lastBacktracePos int
	positions        []*latticePosition
	pending          []*dict.Token
	wordIDRef        []int
}

// NewViterbi creates a Viterbi decoder with the given components.
func NewViterbi(
	sysDict *dict.TokenInfoDictionary,
	unkDict *dict.UnknownDictionary,
	connectionCosts *dict.ConnectionCosts,
	userDict *dict.UserDictionary,
	charDef *dict.CharacterDefinition,
	discardPunctuation bool,
	searchMode bool,
	extendedMode bool,
	outputCompounds bool,
) *Viterbi {
	return &Viterbi{
		sysDict:            sysDict,
		unkDict:            unkDict,
		connectionCosts:    connectionCosts,
		userDict:           userDict,
		charDef:            charDef,
		discardPunctuation: discardPunctuation,
		searchMode:         searchMode,
		extendedMode:       extendedMode,
		outputCompounds:    outputCompounds,
		positions:          make([]*latticePosition, maxBacktraceBufferLen),
	}
}

// ResetBuffer loads new input for analysis.
func (v *Viterbi) ResetBuffer(input []rune) {
	v.input = input
	v.inputLen = len(input)
}

// ResetState resets the decoder state for a new document.
func (v *Viterbi) ResetState() {
	v.pos = 0
	v.end = false
	v.lastBacktracePos = 0
	for i := range v.positions {
		v.positions[i] = nil
	}
	v.pending = v.pending[:0]
	v.wordIDRef = v.wordIDRef[:0]
	bos := newLatticePosition(0)
	bos.add(0, -1, -1, morph.TokenTypeKnown, 0, 0)
	v.setPosition(0, bos)
}

// GetPending returns the pending token slice.
func (v *Viterbi) GetPending() []*dict.Token { return v.pending }

// SetPending replaces the pending slice (used by the tokenizer after draining).
func (v *Viterbi) SetPending(p []*dict.Token) { v.pending = p }

// IsEnd reports whether all input has been consumed and back-traced.
func (v *Viterbi) IsEnd() bool { return v.end }

// GetPos returns the current scan position.
func (v *Viterbi) GetPos() int { return v.pos }

func (v *Viterbi) getPosition(pos int) *latticePosition {
	idx := pos % maxBacktraceBufferLen
	lp := v.positions[idx]
	if lp != nil && lp.pos == pos {
		return lp
	}
	return nil
}

func (v *Viterbi) setPosition(pos int, lp *latticePosition) {
	v.positions[pos%maxBacktraceBufferLen] = lp
}

func (v *Viterbi) getOrCreatePosition(pos int) *latticePosition {
	lp := v.getPosition(pos)
	if lp == nil {
		lp = newLatticePosition(pos)
		v.setPosition(pos, lp)
	}
	return lp
}

// Forward advances the Viterbi search by one step, building the next lattice
// column and back-tracing when the buffer is large enough.
func (v *Viterbi) Forward() {
	if v.pos >= v.inputLen {
		endPos := v.pos
		endLp := v.getOrCreatePosition(endPos)
		if endLp.getCount() == 0 {
			endLp.add(0, v.lastBacktracePos, endPos, morph.TokenTypeKnown, 0, 0)
		}
		v.backtrace(endLp, 0)
		v.end = true
		return
	}

	posData := v.getPosition(v.pos)
	if posData == nil {
		v.pos++
		return
	}

	anyMatches := v.scanSystemDict()
	if v.userDict != nil {
		v.scanUserDict()
	}
	v.scanUnknownWord(anyMatches, posData)

	if v.pos-v.lastBacktracePos >= maxBacktraceBufferLen/2 {
		if nextLP := v.getPosition(v.pos); nextLP != nil && nextLP.getCount() > 0 {
			v.backtrace(nextLP, 0)
		}
	}
	v.pos++
}

// scanSystemDict scans the system dictionary at the current position.
func (v *Viterbi) scanSystemDict() bool {
	if v.sysDict == nil || v.sysDict.GetTokenInfoFST() == nil {
		return false
	}
	fst := v.sysDict.GetTokenInfoFST()
	anyMatches := false
	for end := v.pos + 1; end <= v.inputLen; end++ {
		token := v.input[v.pos:end]
		ordinal := fst.Lookup(token)
		if ordinal < 0 {
			break
		}
		wordIDs := v.sysDict.LookupWordIDs(int(ordinal))
		for _, wordID := range wordIDs {
			if v.addConnection(v.pos, end, wordID, morph.TokenTypeKnown) {
				anyMatches = true
			}
		}
	}
	return anyMatches
}

// scanUserDict scans the user dictionary at the current position.
func (v *Viterbi) scanUserDict() {
	if v.userDict == nil {
		return
	}
	for end := v.pos + 1; end <= v.inputLen; end++ {
		entries := v.userDict.Lookup(v.input, v.pos, end-v.pos)
		for _, entry := range entries {
			if len(entry) > 0 {
				v.addConnection(v.pos, end, entry[0], morph.TokenTypeUser)
			}
		}
	}
}

// scanUnknownWord processes unknown words at the current position.
func (v *Viterbi) scanUnknownWord(anyMatches bool, posData *latticePosition) {
	if posData.getCount() == 0 {
		return
	}
	if v.pos >= v.inputLen {
		return
	}
	firstChar := v.input[v.pos]

	if !anyMatches || (v.charDef != nil && v.charDef.IsInvoke(firstChar)) {
		characterID := 0
		if v.charDef != nil {
			characterID = int(v.charDef.CharacterClass(firstChar))
		}

		unknownWordLength := 1
		if v.charDef == nil || v.charDef.IsGroup(firstChar) {
			isPunct := isPunctuation(firstChar)
			isDigit := unicode.IsDigit(firstChar)
			for posAhead := v.pos + 1; unknownWordLength < maxUnknownWordLength; posAhead++ {
				if posAhead >= v.inputLen {
					break
				}
				ch := v.input[posAhead]
				sameScript := isSameScript(firstChar, ch)
				if sameScript &&
					isPunctuation(ch) == isPunct &&
					unicode.IsDigit(ch) == isDigit &&
					(v.charDef == nil || v.charDef.IsGroup(ch)) {
					unknownWordLength++
				} else {
					break
				}
			}
		}

		if v.unkDict != nil {
			v.unkDict.LookupWordIDs(characterID, &v.wordIDRef)
			for _, wordID := range v.wordIDRef {
				v.addConnection(v.pos, v.pos+unknownWordLength, wordID, morph.TokenTypeUnknown)
			}
		} else {
			v.addConnectionRaw(v.pos, v.pos+unknownWordLength, 0, morph.TokenTypeUnknown)
		}
	}
}

// addConnection is a convenience wrapper around addConnectionRaw.
func (v *Viterbi) addConnection(fromPos, toPos, wordID int, tokenType morph.TokenType) bool {
	return v.addConnectionRaw(fromPos, toPos, wordID, tokenType)
}

// addConnectionRaw adds a connection arc from fromPos to toPos.
func (v *Viterbi) addConnectionRaw(fromPos, toPos, wordID int, tokenType morph.TokenType) bool {
	if fromPos >= v.inputLen+1 || toPos > v.inputLen {
		return false
	}
	fromLP := v.getPosition(fromPos)
	if fromLP == nil || fromLP.getCount() == 0 {
		return false
	}
	var wordCost int
	var rightID int
	switch tokenType {
	case morph.TokenTypeKnown:
		if v.sysDict != nil && v.sysDict.GetMorphAttributes() != nil {
			wordCost = v.sysDict.GetMorphAttributes().WordCost(wordID)
			rightID = v.sysDict.GetMorphAttributes().RightID(wordID)
		}
	case morph.TokenTypeUnknown:
		if v.unkDict != nil && v.unkDict.GetMorphAttributes() != nil {
			wordCost = v.unkDict.GetMorphAttributes().WordCost(wordID)
			rightID = v.unkDict.GetMorphAttributes().RightID(wordID)
		}
	case morph.TokenTypeUser:
		if v.userDict != nil && v.userDict.GetMorphAttributes() != nil {
			wordCost = v.userDict.GetMorphAttributes().WordCost(wordID)
			rightID = v.userDict.GetMorphAttributes().RightID(wordID)
		}
	}

	bestCost := int(^uint(0) >> 1) // MaxInt
	bestIdx := -1
	for idx := 0; idx < fromLP.getCount(); idx++ {
		var leftID int
		switch tokenType {
		case morph.TokenTypeKnown:
			if v.sysDict != nil && v.sysDict.GetMorphAttributes() != nil {
				leftID = v.sysDict.GetMorphAttributes().LeftID(wordID)
			}
		case morph.TokenTypeUnknown:
			if v.unkDict != nil && v.unkDict.GetMorphAttributes() != nil {
				leftID = v.unkDict.GetMorphAttributes().LeftID(wordID)
			}
		case morph.TokenTypeUser:
			if v.userDict != nil && v.userDict.GetMorphAttributes() != nil {
				leftID = v.userDict.GetMorphAttributes().LeftID(wordID)
			}
		}
		connCost := 0
		if v.connectionCosts != nil {
			connCost = v.connectionCosts.Get(rightID, leftID)
		}
		totalCost := fromLP.getCost(idx) + wordCost + connCost
		if totalCost < bestCost {
			bestCost = totalCost
			bestIdx = idx
		}
	}
	if bestIdx == -1 {
		return false
	}

	toLp := v.getOrCreatePosition(toPos)
	toLp.add(bestCost, fromPos, fromPos, tokenType, wordID, bestIdx)
	return true
}

// backtrace walks backwards from endPosData to reconstruct tokens.
func (v *Viterbi) backtrace(endPosData *latticePosition, fromIDX int) {
	endPos := endPosData.pos
	if endPos == v.lastBacktracePos {
		return
	}

	fragment := v.input[v.lastBacktracePos:endPos]
	pos := endPos
	bestIDX := fromIDX

	for pos > v.lastBacktracePos {
		posData := v.getPosition(pos)
		if posData == nil {
			break
		}
		if bestIDX >= posData.getCount() {
			break
		}

		backPos := posData.getBackPos(bestIDX)
		backWordPos := posData.getBackWordPos(bestIDX)
		length := pos - backWordPos
		backType := posData.getBackType(bestIDX)
		backID := posData.getBackID(bestIDX)
		nextBestIDX := posData.getBackIndex(bestIDX)
		fragmentOffset := backWordPos - v.lastBacktracePos
		if fragmentOffset < 0 || fragmentOffset > len(fragment) {
			pos = backPos
			bestIDX = nextBestIDX
			continue
		}

		morphAtts := v.getMorphAttrs(backType)

		if v.extendedMode && backType == morph.TokenTypeUnknown {
			for i := length - 1; i >= 0; i-- {
				token := dict.NewToken(
					fragment,
					fragmentOffset+i,
					1,
					backWordPos+i,
					backWordPos+i+1,
					int(dict.CharClassNGRAM),
					morph.TokenTypeUnknown,
					morphAtts,
				)
				v.pending = append(v.pending, token)
			}
		} else {
			token := dict.NewToken(
				fragment,
				fragmentOffset,
				length,
				backWordPos,
				backWordPos+length,
				backID,
				backType,
				morphAtts,
			)
			if !v.shouldFilterToken(token) {
				v.pending = append(v.pending, token)
			}
		}

		if !v.discardPunctuation && backWordPos != backPos {
			offset := backPos - v.lastBacktracePos
			wsLen := backWordPos - backPos
			if offset >= 0 && offset+wsLen <= len(fragment) {
				spaceCharClass := 0
				if v.charDef != nil {
					spaceCharClass = int(v.charDef.CharacterClass(' '))
				}
				spaceWordIDs := v.wordIDRef[:0]
				if v.unkDict != nil {
					v.unkDict.LookupWordIDs(spaceCharClass, &spaceWordIDs)
				}
				spaceWordID := 0
				if len(spaceWordIDs) > 0 {
					spaceWordID = spaceWordIDs[0]
				}
				spaceToken := dict.NewToken(
					fragment,
					offset,
					wsLen,
					backPos,
					backPos+wsLen,
					spaceWordID,
					morph.TokenTypeUnknown,
					v.getUnkMorphAttrs(),
				)
				v.pending = append(v.pending, spaceToken)
			}
		}

		pos = backPos
		bestIDX = nextBestIDX
	}

	v.lastBacktracePos = endPos
	for i := range v.positions {
		lp := v.positions[i]
		if lp != nil && lp.pos < endPos {
			v.positions[i] = nil
		}
	}
}

func (v *Viterbi) getMorphAttrs(tokenType morph.TokenType) dict.JaMorphData {
	switch tokenType {
	case morph.TokenTypeKnown:
		if v.sysDict != nil {
			return v.sysDict.GetMorphAttributes()
		}
	case morph.TokenTypeUnknown:
		if v.unkDict != nil {
			return v.unkDict.GetMorphAttributes()
		}
	case morph.TokenTypeUser:
		if v.userDict != nil {
			return v.userDict.GetMorphAttributes()
		}
	}
	return nil
}

func (v *Viterbi) getUnkMorphAttrs() dict.JaMorphData {
	if v.unkDict != nil {
		return v.unkDict.GetMorphAttributes()
	}
	return nil
}

func (v *Viterbi) shouldFilterToken(token *dict.Token) bool {
	if !v.discardPunctuation {
		return false
	}
	sf := token.SurfaceForm
	off := token.Offset
	if off < len(sf) {
		return isPunctuation(sf[off])
	}
	return false
}

// isPunctuation reports whether ch is a punctuation character.
func isPunctuation(ch rune) bool {
	return unicode.Is(unicode.Z, ch) ||
		unicode.Is(unicode.Cc, ch) ||
		unicode.Is(unicode.Cf, ch) ||
		unicode.Is(unicode.P, ch) ||
		unicode.Is(unicode.S, ch)
}

// isSameScript reports whether two runes belong to compatible Unicode scripts.
func isSameScript(a, b rune) bool {
	if isCommonOrInherited(a) || isCommonOrInherited(b) {
		return true
	}
	return runeBlock(a) == runeBlock(b)
}

// runeBlock returns a coarse block identifier for grouping purposes.
func runeBlock(ch rune) int {
	switch {
	case ch >= 0x3040 && ch <= 0x309F:
		return 2 // Hiragana
	case ch >= 0x30A0 && ch <= 0x30FF:
		return 3 // Katakana
	case ch >= 0x4E00 && ch <= 0x9FFF:
		return 4 // CJK unified ideographs
	case ch >= 0x3400 && ch <= 0x4DBF:
		return 4 // CJK extension A
	case ch >= 0xF900 && ch <= 0xFAFF:
		return 4 // CJK compatibility ideographs
	case ch >= 0x0041 && ch <= 0x007A:
		return 5 // Latin
	case ch >= 0xFF21 && ch <= 0xFF5A:
		return 5 // Fullwidth Latin
	case ch >= 0x0030 && ch <= 0x0039:
		return 6 // ASCII digits
	case ch >= 0xFF10 && ch <= 0xFF19:
		return 6 // Fullwidth digits
	default:
		return int(ch)
	}
}

func isCommonOrInherited(ch rune) bool {
	return unicode.Is(unicode.Common, ch) || unicode.Is(unicode.Inherited, ch)
}
