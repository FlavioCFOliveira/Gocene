// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation

// EmojiSequenceStatus is the rule-status value emitted for emoji sequences by
// BreakIteratorWrapper, matching ICUTokenizerConfig.EMOJI_SEQUENCE_STATUS = 299.
const EmojiSequenceStatus = 299

// ICUTokenizerConfig allows per-script tailoring of word break iteration.
//
// Go port of
// org.apache.lucene.analysis.icu.segmentation.ICUTokenizerConfig
// (Apache Lucene 10.4.0).
type ICUTokenizerConfig interface {
	// GetBreakIterator returns a RuleBasedBreakIterator for the given
	// UScript numeric code.
	GetBreakIterator(script int) RuleBasedBreakIterator

	// GetType returns the Lucene token type string for the given script
	// and rule status.
	GetType(script, ruleStatus int) string

	// CombineCJ reports whether Han, Hiragana, and Katakana should be
	// treated as a single Japanese script.
	CombineCJ() bool
}
