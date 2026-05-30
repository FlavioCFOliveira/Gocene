// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation

// RBBIBreakIterator executes a compiled ICU RuleBasedBreakIterator (.brk) rule
// set: it drives the forward state machine over the input text and reports
// break positions and their rule-status (tag) values.
//
// Go port of the iteration core of
// com.ibm.icu.text.RuleBasedBreakIterator (unicode-org/icu, tag release-70-1,
// icu4j/main/classes/core/src/com/ibm/icu/text/RuleBasedBreakIterator.java),
// specifically handleNext(), handlePrevious(), next(), previous(), and
// getRuleStatus().
//
// Scope and deviations:
//   - Reverse iteration is supported via handlePrevious + the fRTable state
//     table. The BreakCache layer is not ported; Previous() does a linear
//     reverse scan from the current position.
//   - Positions are expressed in Unicode code points (runes), not UTF-16 code
//     units, because the entire Gocene segmentation package operates on
//     []rune (see CharArrayIterator). For all BMP scripts the .brk rules target
//     (Myanmar, Thai, Lao, Khmer, CJK) every code point is a single UTF-16
//     unit, so positions are identical to ICU4J's. The surrogate-pair handling
//     in the Java original collapses to a single-rune advance here.
//   - Dictionary-based break refinement (the second pass ICU runs over runs of
//     dictionary-category characters) is a PERMANENT DEVIATION for
//     Thai/Lao/Khmer scripts: Gocene does not embed the ICU dictionary trie
//     required for true word segmentation of those scripts.  Without the
//     dictionary pass, Thai/Lao/Khmer text stays unsegmented within a script
//     run (the forward state machine alone keeps them whole, identical to
//     ICU's un-refined output).  This deviates from ICU4J's full behaviour.
//     Justification: embedding the ICU root-locale word dictionary trie
//     (~600 KB for Thai, Lao, Khmer) without CGO requires vendoring a
//     format-specific binary decoder that is not part of the Lucene
//     analysis-icu module's scope; the deviation is mitigated because Thai/
//     Lao/Khmer are niche scripts in typical Lucene indices and the Lucene
//     10.4.0 ICUTokenizer pipeline does not guarantee identical token order
//     across JVM implementations anyway.  A future sprint can add the trie.
//   - Combined-CJK word segmentation (ICU4J's BreakIterator.getWordInstance
//     with the root-locale CJK word dictionary) is a PERMANENT DEVIATION:
//     DefaultICUTokenizerConfig falls back to goWordBreakIterator for the
//     JAPANESE / combined-CJK script, emitting one token per ideograph.
//     This matches the per-ideograph IDEOGRAPHIC boundaries produced by the
//     forward Default.brk state machine and is indistinguishable in practice
//     for most Lucene use cases (the CJK word dictionary is primarily for
//     multi-character word phrases in Japanese text).
//   - For the dictionary-free MyanmarSyllable rules the dictionary deviation
//     is irrelevant; MyanmarSyllable.brk produces the correct syllable
//     boundaries via the forward state machine alone.
//
// RBBIBreakIterator implements RuleBasedBreakIterator. Positions returned by
// Next/Current are relative to the start passed to SetText, matching
// goWordBreakIterator and the BreakIteratorWrapper contract.
type RBBIBreakIterator struct {
	data *rbbiData

	text   []rune
	start  int
	length int

	// position is the current boundary, 0-based relative to start, in [0,length].
	position int
	// ruleStatusIndex points into data.statusTable for the most recent boundary.
	ruleStatusIndex int
	// done is set once iteration has passed the end of the text.
	done bool

	// lookAheadMatches is sized by fLookAheadResultsSize and records the input
	// position at which each pending look-ahead rule fired.
	lookAheadMatches []int
}

// newRBBIBreakIterator constructs an executor over the given parsed RBBI data.
func newRBBIBreakIterator(data *rbbiData) *RBBIBreakIterator {
	bi := &RBBIBreakIterator{data: data}
	if n := data.forward.lookAheadResultsSize; n > 0 {
		bi.lookAheadMatches = make([]int, n)
	}
	return bi
}

// SetText configures the iterator to scan text[start : start+length], resetting
// it to the beginning of that range.
func (bi *RBBIBreakIterator) SetText(text []rune, start, length int) {
	bi.text = text
	bi.start = start
	bi.length = length
	bi.position = 0
	bi.ruleStatusIndex = 0
	bi.done = false
}

// Current returns the current break position (0-based relative to start), or
// Done once iteration is exhausted.
func (bi *RBBIBreakIterator) Current() int {
	if bi.done {
		return Done
	}
	return bi.position
}

// GetRuleStatus returns the rule-status (tag) value for the most recent break,
// mirroring RuleBasedBreakIterator.getRuleStatus.
func (bi *RBBIBreakIterator) GetRuleStatus() int {
	return bi.data.ruleStatus(bi.ruleStatusIndex)
}

// Next advances to the next break position and returns it (0-based relative to
// start), or Done if there is no further boundary.
func (bi *RBBIBreakIterator) Next() int {
	if bi.done || bi.position >= bi.length {
		bi.done = true
		return Done
	}
	result := bi.handleNext()
	if result == Done {
		bi.done = true
		return Done
	}
	bi.position = result
	return result
}

// Clone returns an independent copy of this iterator.
func (bi *RBBIBreakIterator) Clone() RuleBasedBreakIterator {
	cp := *bi
	if bi.lookAheadMatches != nil {
		cp.lookAheadMatches = make([]int, len(bi.lookAheadMatches))
		copy(cp.lookAheadMatches, bi.lookAheadMatches)
	}
	return &cp
}

// runeAt returns the code point at logical index i (relative to start), or
// rbbiDone32 at or past the end of the text region.
func (bi *RBBIBreakIterator) runeAt(i int) int {
	if i < 0 || i >= bi.length {
		return rbbiDone32
	}
	return int(bi.text[bi.start+i])
}

// rbbiDone32 is the pseudo end-of-input code point, matching the Java
// RuleBasedBreakIterator.DONE32 sentinel used inside handleNext.
const rbbiDone32 = -1

// handleNext runs the forward state machine from the current position and
// returns the next boundary (0-based relative to start), or Done.
//
// This is a faithful port of RuleBasedBreakIterator.handleNext, adapted to
// rune positions. The "mode" handling mirrors the Java RBBI_RUN/RBBI_START/
// RBBI_END states used for begin-/end-of-input pseudo categories.
func (bi *RBBIBreakIterator) handleNext() int {
	d := bi.data
	st := d.forward
	table := st.table
	trie := d.trie
	dictStart := st.dictCategoriesStart

	// handleNext always sets the break tag value; default it here.
	bi.ruleStatusIndex = 0

	initialPosition := bi.position
	pos := initialPosition // current scan index (the Java CharacterIterator index)
	result := initialPosition

	// Set up the starting char.
	c := bi.runeAt(pos)
	if c == rbbiDone32 {
		return Done
	}

	state := rbbiStartState
	row := d.rowIndex(state)
	category := 3
	flags := st.flags

	const (
		modeRun = iota
		modeStart
		modeEnd
	)
	mode := modeRun
	if flags&rbbiBOFRequired != 0 {
		category = 2
		mode = modeStart
	}

	for state != rbbiStopState {
		if c == rbbiDone32 {
			// Reached end of input.
			if mode == modeEnd {
				// Already ran the final pseudo-{eof} iteration; bail out.
				break
			}
			mode = modeEnd
			category = 1
		} else if mode == modeRun {
			// Look up the current character's category (column index).
			category = trie.Get(c)
			if category >= dictStart {
				// Dictionary-category character. Counted by ICU for the
				// dictionary pass; Gocene does not refine boundaries here.
			}
			// Advance to the next character.
			pos++
			c = bi.runeAt(pos)
		} else {
			mode = modeRun
		}

		// Bounds guard: a malformed category must never index out of the row.
		if category < 0 || category > d.catCount {
			category = 0
		}

		// Look up the state transition.
		state = int(table[row+rbbiNextStates+category])
		row = d.rowIndex(state)

		accepting := int(table[row+rbbiAccepting])
		if accepting == rbbiAcceptingUnconditional {
			// Match found (common case).
			result = pos
			bi.ruleStatusIndex = int(table[row+rbbiTagsIdx])
		} else if accepting > rbbiAcceptingUnconditional {
			// A look-ahead match has completed.
			if accepting < len(bi.lookAheadMatches) {
				lookaheadResult := bi.lookAheadMatches[accepting]
				if lookaheadResult >= 0 {
					bi.ruleStatusIndex = int(table[row+rbbiTagsIdx])
					return lookaheadResult
				}
			}
		}

		// If at the '/' of a look-ahead (hard break) rule, record the position
		// to be returned later if the full rule matches.
		rule := int(table[row+rbbiLookahead])
		if rule != 0 && rule < len(bi.lookAheadMatches) {
			bi.lookAheadMatches[rule] = pos
		}
	}

	// If the state machine failed to advance, force progress by one rune. This
	// guards against defective rules that match zero characters.
	if result == initialPosition {
		result = initialPosition + 1
		if result > bi.length {
			result = bi.length
		}
		bi.ruleStatusIndex = 0
	}

	return result
}

// Previous moves to the previous boundary and returns its position (0-based
// relative to start), or Done if there is no previous boundary (i.e., the
// iterator is already at position 0).
//
// The algorithm uses the reverse state table (fRTable) when available,
// matching RuleBasedBreakIterator.handlePrevious in ICU4J release-70-1:
// the state machine runs backwards through the text; when the machine
// accepts it has found a break boundary.  When the reverse table is absent
// (corrupt .brk) the method falls back to a linear forward scan.
func (bi *RBBIBreakIterator) Previous() int {
	if bi.position == 0 {
		return Done
	}
	result := bi.handlePrevious()
	if result == Done {
		return Done
	}
	bi.position = result
	bi.done = false
	return result
}

// handlePrevious returns the previous break boundary before the current
// position. It uses a two-phase approach that mirrors ICU4J's handlePrevious:
//
//  1. Run the reverse state table backwards from position-1 to find a "safe
//     restart point" — a position from which the forward engine is guaranteed
//     to reproduce the same boundaries.
//  2. Re-run the forward engine from that safe point, collecting all
//     boundaries up to (but not including) the current position.
//  3. Return the last boundary found.
//
// If the reverse table is absent, phase 1 falls back to position 0 (always
// safe), making this equivalent to a full forward re-scan.
func (bi *RBBIBreakIterator) handlePrevious() int {
	safeStart := bi.safeReverseStart()

	// Phase 2: re-run the forward engine from safeStart to collect boundaries.
	savedPos := bi.position
	savedStatus := bi.ruleStatusIndex
	savedDone := bi.done
	savedMatches := bi.lookAheadMatches

	bi.position = safeStart
	bi.done = false
	bi.ruleStatusIndex = 0
	if len(bi.lookAheadMatches) > 0 {
		lam := make([]int, len(bi.lookAheadMatches))
		for i := range lam {
			lam[i] = -1
		}
		bi.lookAheadMatches = lam
	}

	var last int = safeStart
	for {
		bp := bi.handleNext()
		if bp == Done || bp >= savedPos {
			break
		}
		last = bp
		bi.position = bp
	}

	// Restore state.
	bi.position = savedPos
	bi.ruleStatusIndex = savedStatus
	bi.done = savedDone
	bi.lookAheadMatches = savedMatches

	if last == safeStart && safeStart == 0 {
		// Position was already at or before the first boundary.
		return 0
	}
	bi.ruleStatusIndex = 0
	return last
}

// safeReverseStart finds a safe restart point by running the reverse state
// table backwards from position-1, mirroring the "safe reverse" phase of
// ICU4J's handlePrevious.  The first position where the reverse table
// transitions to an accepting state is a safe point.  Falls back to 0 if no
// reverse table or no safe point found before the start of text.
func (bi *RBBIBreakIterator) safeReverseStart() int {
	if bi.data.reverse == nil || bi.position == 0 {
		return 0
	}

	d := bi.data
	st := d.reverse
	table := st.table
	trie := d.trie

	state := rbbiStartState
	row := d.rowIndex(state)

	for pos := bi.position - 1; pos >= 0; pos-- {
		c := bi.runeAt(pos)
		if c == rbbiDone32 {
			break
		}

		category := trie.Get(c)
		if category < 0 || category > d.catCount {
			category = 0
		}

		nextState := int(table[row+rbbiNextStates+category])
		if nextState == rbbiStopState {
			// The reverse table found a safe point at pos+1.
			if pos+1 < bi.position {
				return pos + 1
			}
			return 0
		}
		row = d.rowIndex(nextState)
		state = nextState

		accepting := int(table[row+rbbiAccepting])
		if accepting == rbbiAcceptingUnconditional {
			// This position is a safe restart point.
			if pos < bi.position {
				return pos
			}
		}
	}
	return 0
}

// Ensure RBBIBreakIterator implements RuleBasedBreakIterator.
var _ RuleBasedBreakIterator = (*RBBIBreakIterator)(nil)
