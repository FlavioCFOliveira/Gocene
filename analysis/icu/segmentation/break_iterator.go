// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation

// RuleBasedBreakIterator is the Go interface for ICU4J's
// com.ibm.icu.text.RuleBasedBreakIterator.
//
// Deviation: ICU4J's RuleBasedBreakIterator is a concrete class that reads
// compiled binary rule files (.brk) and applies them via RBBI algorithm.
// Go has no CGO-free equivalent. This interface exposes the minimal subset
// required by BreakIteratorWrapper and CompositeBreakIterator; callers must
// supply a concrete implementation (or use DefaultICUTokenizerConfig which
// provides Go-native word break iterators).
type RuleBasedBreakIterator interface {
	// Next advances to the next break position and returns it, or Done.
	Next() int

	// Current returns the current break position.
	Current() int

	// GetRuleStatus returns the rule-status tag for the most recent break.
	// RuleBasedBreakIterator.WORD_NONE = 0, WORD_LETTER = 100-199,
	// WORD_NUMBER = 100, WORD_IDEO = 300-399, WORD_KANA = 200-299.
	GetRuleStatus() int

	// SetText sets the text to be analysed. start and length define the
	// sub-range within text to examine.
	SetText(text []rune, start, length int)

	// Clone returns an independent copy of this break iterator.
	Clone() RuleBasedBreakIterator
}

// Rule status constants matching com.ibm.icu.text.RuleBasedBreakIterator.
const (
	// RuleStatusWordNone — non-word break (spaces, punctuation, etc.)
	RuleStatusWordNone = 0
	// RuleStatusWordNumber — numeric content.
	RuleStatusWordNumber = 100
	// RuleStatusWordLetter — letter content.
	RuleStatusWordLetter = 200
	// RuleStatusWordKana — kana content.
	RuleStatusWordKana = 300
	// RuleStatusWordIdeo — ideographic content.
	RuleStatusWordIdeo = 400
)
