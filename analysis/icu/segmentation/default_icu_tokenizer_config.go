// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation

import "github.com/FlavioCFOliveira/Gocene/analysis"

// DefaultICUTokenizerConfig is the default ICUTokenizerConfig that is
// generally applicable to many languages.
//
// Generally tokenizes Unicode text according to UAX#29, but with the following
// tailorings:
//   - Thai, Lao, Myanmar, Khmer, and CJK text is broken into words with a
//     dictionary (deviation: Go implementation uses a unicode-property-based
//     approximation; see goWordBreakIterator).
//
// Go port of
// org.apache.lucene.analysis.icu.segmentation.DefaultICUTokenizerConfig
// (Apache Lucene 10.4.0).
//
// Deviation: the Java implementation loads ICU4J compiled .brk rule files
// (Default.brk, MyanmarSyllable.brk) via getResourceAsStream. There is no
// CGO-free Go equivalent. This port uses goWordBreakIterator, a Go-native
// UAX#29 approximation backed by Go's stdlib unicode range tables.
// Dictionary-based segmentation for CJK/Thai/Myanmar is not available.
type DefaultICUTokenizerConfig struct {
	cjkAsWords     bool
	myanmarAsWords bool
}

// NewDefaultICUTokenizerConfig creates a new DefaultICUTokenizerConfig.
//
// cjkAsWords — if true, CJK text undergoes unified Japanese script handling
// and all Han+Hiragana+Katakana tokens are tagged as IDEOGRAPHIC.
//
// myanmarAsWords — if true, Myanmar text is tokenized as words; if false,
// it is tokenized as syllables. (In this Go port, both modes use the same
// goWordBreakIterator because no syllable iterator is available without ICU.)
func NewDefaultICUTokenizerConfig(cjkAsWords, myanmarAsWords bool) *DefaultICUTokenizerConfig {
	return &DefaultICUTokenizerConfig{
		cjkAsWords:     cjkAsWords,
		myanmarAsWords: myanmarAsWords,
	}
}

// CombineCJ reports whether Han, Hiragana, and Katakana should be
// combined into the Japanese script for CJK dictionary breaking.
func (c *DefaultICUTokenizerConfig) CombineCJ() bool {
	return c.cjkAsWords
}

// GetBreakIterator returns a new RuleBasedBreakIterator for the given
// UScript numeric code.
//
// All scripts use goWordBreakIterator, which approximates ICU4J's
// RuleBasedBreakIterator for word boundary detection.
func (c *DefaultICUTokenizerConfig) GetBreakIterator(script int) RuleBasedBreakIterator {
	return newGoWordBreakIterator()
}

// GetType returns the Lucene token type string for the given script and rule
// status pair.
func (c *DefaultICUTokenizerConfig) GetType(script, ruleStatus int) string {
	switch ruleStatus {
	case RuleStatusWordIdeo:
		return analysis.StandardTokenTypes[analysis.TokenTypeIdeographic]
	case RuleStatusWordKana:
		if script == UScriptHiragana {
			return analysis.StandardTokenTypes[analysis.TokenTypeHiragana]
		}
		return analysis.StandardTokenTypes[analysis.TokenTypeKatakana]
	case RuleStatusWordLetter:
		if script == UScriptHangul {
			return analysis.StandardTokenTypes[analysis.TokenTypeHangul]
		}
		return analysis.StandardTokenTypes[analysis.TokenTypeAlphanum]
	case RuleStatusWordNumber:
		return analysis.StandardTokenTypes[analysis.TokenTypeNum]
	case EmojiSequenceStatus:
		return analysis.StandardTokenTypes[analysis.TokenTypeEmoji]
	default:
		return "<OTHER>"
	}
}

// Ensure DefaultICUTokenizerConfig implements ICUTokenizerConfig.
var _ ICUTokenizerConfig = (*DefaultICUTokenizerConfig)(nil)
