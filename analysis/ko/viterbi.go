// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package ko

import (
	"io"
	"unicode"

	"github.com/FlavioCFOliveira/Gocene/analysis/ko/dict"
	"github.com/FlavioCFOliveira/Gocene/analysis/morph"
)

const (
	maxBacktraceBufferLen = 4096
	maxUnknownWordLength  = 1024
)

// positionNode represents a single back-trace node in the Viterbi lattice.
type positionNode struct {
	cost      int
	backPos   int
	backWordPos int
	backType  morph.TokenType
	backID    int
	backIndex int
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

// rollingBuffer provides a circular buffer for reading characters from a Reader.
type rollingBuffer struct {
	data   []rune
	offset int // absolute position of data[0]
	end    int // absolute end position (exclusive)
}

func newRollingBuffer(initialCap int) *rollingBuffer {
	return &rollingBuffer{data: make([]rune, 0, initialCap)}
}

func (rb *rollingBuffer) reset(r io.Reader) {
	rb.data = rb.data[:0]
	rb.offset = 0
	rb.end = 0
	// store reader reference — we use a wrapper below
	_ = r
}

func (rb *rollingBuffer) get(pos int) int {
	idx := pos - rb.offset
	if idx < 0 || idx >= len(rb.data) {
		return -1
	}
	return int(rb.data[idx])
}

func (rb *rollingBuffer) slice(start, length int) []rune {
	idx := start - rb.offset
	if idx < 0 || idx+length > len(rb.data) {
		return nil
	}
	return rb.data[idx : idx+length]
}

func (rb *rollingBuffer) freeBefore(pos int) {
	newOffset := pos
	if newOffset > rb.offset {
		shift := newOffset - rb.offset
		if shift >= len(rb.data) {
			rb.data = rb.data[:0]
		} else {
			copy(rb.data, rb.data[shift:])
			rb.data = rb.data[:len(rb.data)-shift]
		}
		rb.offset = newOffset
	}
}

// Viterbi implements the rolling Viterbi search for Korean morphological
// analysis.
//
// This is the Go port of the Korean-specific
// org.apache.lucene.analysis.ko.Viterbi, which extends the base
// org.apache.lucene.analysis.morph.Viterbi, from Apache Lucene 10.4.0.
//
// Deviation: the Java base Viterbi is a generic class with complex rolling
// buffer and FST traversal machinery. This Go port provides a self-contained,
// structurally faithful port that operates on in-memory rune slices rather
// than streaming readers, matching the observable contract of the Java
// implementation for the same inputs.
type Viterbi struct {
	// sysDict is the system TokenInfoDictionary.
	sysDict *dict.TokenInfoDictionary
	// unkDict is the unknown-word dictionary.
	unkDict *dict.UnknownDictionary
	// connectionCosts is the Viterbi cost matrix.
	connectionCosts *dict.ConnectionCosts
	// userDict is the optional user dictionary.
	userDict *dict.UserDictionary
	// charDef is the character definition table.
	charDef *dict.CharacterDefinition
	// discardPunctuation controls whether punctuation is dropped.
	discardPunctuation bool
	// mode is the decompound mode.
	mode DecompoundMode
	// outputUnknownUnigrams controls unigram output for unknown words.
	outputUnknownUnigrams bool

	// input holds the current input characters.
	input []rune
	// inputLen is the length of input.
	inputLen int
	// pos is the current forward scan position.
	pos int
	// end marks the end of valid input.
	end bool
	// lastBacktracePos is the last position we back-traced to.
	lastBacktracePos int
	// positions is the Viterbi lattice indexed by absolute position.
	positions []*latticePosition
	// pending holds tokens waiting to be returned.
	pending []*Token
	// wordIDRef is a reusable slice for word ID lookups.
	wordIDRef []int
}

// NewViterbi creates a Viterbi decoder with the given components.
func NewViterbi(
	sysDict *dict.TokenInfoDictionary,
	unkDict *dict.UnknownDictionary,
	connectionCosts *dict.ConnectionCosts,
	userDict *dict.UserDictionary,
	charDef *dict.CharacterDefinition,
	discardPunctuation bool,
	mode DecompoundMode,
	outputUnknownUnigrams bool,
) *Viterbi {
	return &Viterbi{
		sysDict:               sysDict,
		unkDict:               unkDict,
		connectionCosts:       connectionCosts,
		userDict:              userDict,
		charDef:               charDef,
		discardPunctuation:    discardPunctuation,
		mode:                  mode,
		outputUnknownUnigrams: outputUnknownUnigrams,
		positions:             make([]*latticePosition, maxBacktraceBufferLen),
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
	// Add BOS (begin-of-sentence) position.
	bos := newLatticePosition(0)
	bos.add(0, -1, -1, morph.TokenTypeKnown, 0, 0)
	v.setPosition(0, bos)
}

// GetPending returns the pending token slice.
func (v *Viterbi) GetPending() []*Token { return v.pending }

// SetPending replaces the pending slice (used by the tokenizer after draining).
func (v *Viterbi) SetPending(p []*Token) { v.pending = p }

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
		// Force end back-trace.
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

	// Back-trace if we have enough buffer.
	if v.pos-v.lastBacktracePos >= maxBacktraceBufferLen/2 {
		if nextLP := v.getPosition(v.pos); nextLP != nil && nextLP.getCount() > 0 {
			v.backtrace(nextLP, 0)
		}
	}
	v.pos++
}

// scanSystemDict scans the system dictionary at the current position.
func (v *Viterbi) scanSystemDict() bool {
	if v.sysDict == nil || v.sysDict.GetFST() == nil {
		return false
	}
	fst := v.sysDict.GetFST()
	// Try all lengths from current pos.
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
		wordIDs := v.userDict.Lookup(v.input, v.pos, end-v.pos)
		for _, wordID := range wordIDs {
			v.addConnection(v.pos, end, wordID, morph.TokenTypeUser)
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

		// Look up unknown word IDs.
		if v.unkDict != nil {
			v.unkDict.LookupWordIDs(characterID, &v.wordIDRef)
			for _, wordID := range v.wordIDRef {
				v.addConnection(v.pos, v.pos+unknownWordLength, wordID, morph.TokenTypeUnknown)
			}
		} else {
			// No unk dict: emit a single unknown span.
			v.addConnectionRaw(v.pos, v.pos+unknownWordLength, 0, morph.TokenTypeUnknown)
		}
	}
}

// addConnection adds a connection arc with morphological dictionary lookup for cost.
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

	// Find best predecessor.
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

		if v.outputUnknownUnigrams && backType == morph.TokenTypeUnknown {
			for i := length - 1; i >= 0; i-- {
				token := NewDictionaryToken(
					morph.TokenTypeUnknown,
					morphAtts,
					int(dict.CharClassNGRAM),
					fragment,
					fragmentOffset+i,
					1,
					backWordPos+i,
					backWordPos+i+1,
				)
				v.pending = append(v.pending, &token.Token)
			}
		} else {
			token := NewDictionaryToken(
				backType,
				morphAtts,
				backID,
				fragment,
				fragmentOffset,
				length,
				backWordPos,
				backWordPos+length,
			)
			if token.GetPOSType() == POSTypeMorpheme || v.mode == DecompoundModeNone {
				if !v.shouldFilterToken(token) {
					v.pending = append(v.pending, &token.Token)
				}
			} else {
				morphemes := token.GetMorphemes()
				if morphemes == nil {
					v.pending = append(v.pending, &token.Token)
				} else {
					endOffset := backWordPos + length
					posLen := 0
					for i := len(morphemes) - 1; i >= 0; i-- {
						morpheme := morphemes[i]
						var compoundToken *DecompoundToken
						if token.GetPOSType() == POSTypeCompound {
							startOff := endOffset - len([]rune(morpheme.SurfaceForm))
							compoundToken = NewDecompoundToken(
								morpheme.PosTag,
								morpheme.SurfaceForm,
								startOff,
								endOffset,
								backType,
							)
							endOffset = startOff
						} else {
							compoundToken = NewDecompoundToken(
								morpheme.PosTag,
								morpheme.SurfaceForm,
								token.startOffset,
								token.endOffset,
								backType,
							)
						}
						if i == 0 && v.mode == DecompoundModeMixed {
							compoundToken.SetPositionIncrement(0)
						}
						posLen++
						v.pending = append(v.pending, &compoundToken.Token)
					}
					if v.mode == DecompoundModeMixed {
						if posLen > 1 {
							token.SetPositionLength(posLen)
						}
						v.pending = append(v.pending, &token.Token)
					}
				}
			}
		}

		// Emit whitespace token if discardPunctuation is false.
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
				spaceToken := NewDictionaryToken(
					morph.TokenTypeUnknown,
					v.getUnkMorphAttrs(),
					spaceWordID,
					fragment,
					offset,
					wsLen,
					backPos,
					backPos+wsLen,
				)
				v.pending = append(v.pending, &spaceToken.Token)
			}
		}

		pos = backPos
		bestIDX = nextBestIDX
	}

	v.lastBacktracePos = endPos
	// Free positions before endPos.
	for i := range v.positions {
		lp := v.positions[i]
		if lp != nil && lp.pos < endPos {
			v.positions[i] = nil
		}
	}
}

func (v *Viterbi) getMorphAttrs(tokenType morph.TokenType) dict.KoMorphData {
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

func (v *Viterbi) getUnkMorphAttrs() dict.KoMorphData {
	if v.unkDict != nil {
		return v.unkDict.GetMorphAttributes()
	}
	return nil
}

func (v *Viterbi) shouldFilterToken(token *DictionaryToken) bool {
	if !v.discardPunctuation {
		return false
	}
	sf := token.GetSurfaceForm()
	off := token.GetOffset()
	if off < len(sf) {
		return isPunctuation(sf[off])
	}
	return false
}

// isPunctuation reports whether ch is a punctuation character (matching Java
// semantics for Character.getType — covers space separators, control, format,
// dash/start/end/connector/other punctuation, math/currency/modifier/other
// symbols, and initial/final quote punctuation).
func isPunctuation(ch rune) bool {
	if ch == 0x318D { // Hangul Letter Araea (interpunct)
		return true
	}
	return unicode.Is(unicode.Z, ch) || // separators
		unicode.Is(unicode.Cc, ch) || // control
		unicode.Is(unicode.Cf, ch) || // format
		unicode.Is(unicode.P, ch) || // punctuation (Pd Ps Pe Pc Po Pi Pf)
		unicode.Is(unicode.S, ch) // symbols (Sm Sc Sk So)
}

// isSameScript reports whether two runes belong to compatible Unicode scripts.
// COMMON and INHERITED are compatible with everything, matching Java's
// Character.UnicodeScript.of semantics.
func isSameScript(a, b rune) bool {
	if isCommonOrInherited(a) || isCommonOrInherited(b) {
		return true
	}
	// Use a simple block-based heuristic for the most common Korean scripts.
	// The primary use case is grouping hangul/hanja/alpha/numeric runs.
	return runeBlock(a) == runeBlock(b)
}

// runeBlock returns a coarse block identifier for grouping purposes.
func runeBlock(ch rune) int {
	switch {
	case ch >= 0xAC00 && ch <= 0xD7AF:
		return 1 // Hangul syllables
	case ch >= 0x1100 && ch <= 0x11FF:
		return 1 // Hangul jamo
	case ch >= 0xA960 && ch <= 0xA97F:
		return 1 // Hangul jamo extended-A
	case ch >= 0xD7B0 && ch <= 0xD7FF:
		return 1 // Hangul jamo extended-B
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
	case ch >= 0x0400 && ch <= 0x04FF:
		return 7 // Cyrillic
	case ch >= 0x0370 && ch <= 0x03FF:
		return 8 // Greek
	default:
		return int(ch) // unique block per character for anything else
	}
}

// isCommonOrInherited reports whether ch belongs to the Common or Inherited
// Unicode script.
func isCommonOrInherited(ch rune) bool {
	return unicode.Is(unicode.Common, ch) || unicode.Is(unicode.Inherited, ch)
}
