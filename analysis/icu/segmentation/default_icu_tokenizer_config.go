// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// defaultBreakDict and myanmarSyllableDict are the parsed runtime tables for
// the bundled ICU break-rule dictionaries, decoded once at package
// initialisation. They are read-only after init; per-call iterators clone the
// lightweight iterator state, sharing these immutable tables (mirroring how
// Lucene clones its static RuleBasedBreakIterator instances).
var (
	defaultBreakDict    *rbbiData
	myanmarSyllableDict *rbbiData
	breakDictInitErr    error
)

func init() {
	defaultBreakDict, breakDictInitErr = loadEmbeddedRBBI(EmbeddedDefaultBRKName)
	if breakDictInitErr == nil {
		myanmarSyllableDict, breakDictInitErr = loadEmbeddedRBBI(EmbeddedMyanmarSyllableBRKName)
	}
}

// loadEmbeddedRBBI parses the named embedded .brk blob into runtime RBBI tables.
func loadEmbeddedRBBI(name string) (*rbbiData, error) {
	dict, err := LoadEmbeddedBRK(name)
	if err != nil {
		return nil, fmt.Errorf("segmentation: loading %s: %w", name, err)
	}
	return dict.RBBIData()
}

// DefaultICUTokenizerConfig is the default ICUTokenizerConfig that is
// generally applicable to many languages.
//
// Generally tokenizes Unicode text according to UAX#29, with these tailorings:
//   - Most scripts use the compiled Default.brk rule set, executed natively by
//     RBBIBreakIterator. This yields true ICU4J word boundaries (e.g. internal
//     "," in numbers and "'/\"" in Hebrew words stay within the token).
//   - Myanmar text is broken into syllables via MyanmarSyllable.brk, or into
//     words via Default.brk when myanmarAsWords is set.
//
// Go port of
// org.apache.lucene.analysis.icu.segmentation.DefaultICUTokenizerConfig
// (Apache Lucene 10.4.0).
//
// Deviation: the combined-CJK case (UScript.JAPANESE, reached when cjkAsWords
// is true) uses ICU4J's dictionary-based BreakIterator.getWordInstance in
// Lucene. Gocene has no CGO-free equivalent, so that one case falls back to
// goWordBreakIterator (each Han character its own token). All other scripts,
// including non-combined CJK, run the compiled Default.brk rules.
type DefaultICUTokenizerConfig struct {
	cjkAsWords     bool
	myanmarAsWords bool
}

// NewDefaultICUTokenizerConfig creates a new DefaultICUTokenizerConfig.
//
// cjkAsWords — if true, CJK text undergoes unified Japanese script handling
// and all Han+Hiragana+Katakana tokens are tagged as IDEOGRAPHIC.
//
// myanmarAsWords — if true, Myanmar text is tokenized as words (Default.brk);
// if false, it is tokenized as syllables (MyanmarSyllable.brk).
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

// GetBreakIterator returns a new RuleBasedBreakIterator for the given UScript
// numeric code, mirroring DefaultICUTokenizerConfig.getBreakIterator.
//
//   - UScriptJapanese (the combined-CJK case): goWordBreakIterator fallback —
//     ICU4J's dictionary word iterator has no CGO-free Go equivalent.
//   - UScriptMyanmar: MyanmarSyllable.brk (syllables) unless myanmarAsWords is
//     set, in which case Default.brk (words).
//   - everything else: Default.brk.
//
// If the compiled tables failed to load at init, every script falls back to
// goWordBreakIterator so tokenisation still functions.
func (c *DefaultICUTokenizerConfig) GetBreakIterator(script int) RuleBasedBreakIterator {
	if breakDictInitErr != nil {
		return newGoWordBreakIterator()
	}
	switch script {
	case UScriptJapanese:
		return newGoWordBreakIterator()
	case UScriptMyanmar:
		if c.myanmarAsWords {
			return newRBBIBreakIterator(defaultBreakDict)
		}
		return newRBBIBreakIterator(myanmarSyllableDict)
	default:
		return newRBBIBreakIterator(defaultBreakDict)
	}
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
