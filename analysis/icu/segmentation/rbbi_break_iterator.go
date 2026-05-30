// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation

// RBBIBreakIterator executes a compiled ICU RuleBasedBreakIterator (.brk) rule
// set: it drives the forward state machine over the input text and reports
// break positions and their rule-status (tag) values.
//
// Go port of the forward-iteration core of
// com.ibm.icu.text.RuleBasedBreakIterator (unicode-org/icu, tag release-70-1,
// icu4j/main/classes/core/src/com/ibm/icu/text/RuleBasedBreakIterator.java),
// specifically handleNext(), next(), and getRuleStatus().
//
// Scope and deviations:
//   - Forward iteration only. Reverse iteration (handlePrevious + the safe
//     reverse table) and the BreakCache optimisation layer are not ported.
//     The ICUTokenizer pipeline only ever iterates forward, so this is
//     sufficient for tokenisation. Reverse iteration is a documented follow-up.
//   - Positions are expressed in Unicode code points (runes), not UTF-16 code
//     units, because the entire Gocene segmentation package operates on
//     []rune (see CharArrayIterator). For all BMP scripts the .brk rules target
//     (Myanmar, Thai, Lao, Khmer, CJK) every code point is a single UTF-16
//     unit, so positions are identical to ICU4J's. The surrogate-pair handling
//     in the Java original collapses to a single-rune advance here.
//   - Dictionary-based break refinement (the second pass ICU runs over runs of
//     dictionary-category characters) is NOT performed. handleNext counts
//     dictionary characters but Gocene does not yet rewrite their boundaries;
//     for the dictionary-free MyanmarSyllable rules this is irrelevant, and for
//     CJK the rule machine already emits per-character IDEOGRAPHIC boundaries.
//     Dictionary refinement is a documented follow-up.
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

// Ensure RBBIBreakIterator implements RuleBasedBreakIterator.
var _ RuleBasedBreakIterator = (*RBBIBreakIterator)(nil)
