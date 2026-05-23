// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation

// ScriptIterator locates ISO 15924 script boundaries in text.
//
// Go port of
// org.apache.lucene.analysis.icu.segmentation.ScriptIterator
// (Apache Lucene 10.4.0).
//
// Text is not broken between a combining mark and its base character: a
// combining mark inherits the script value of its base character (UTR #24).
//
// Deviation: The Java original uses UTF16.charAt() to iterate over a char[]
// in UTF-16. In Go the text is a []rune (Unicode code points), so surrogate
// handling is not needed. The GetScript lookup uses Go's stdlib unicode
// range tables instead of ICU4J's UScript.getScript().
type ScriptIterator struct {
	text  []rune
	start int
	limit int
	index int

	scriptStart int
	scriptLimit int
	scriptCode  int

	combineCJ bool
}

// NewScriptIterator creates a ScriptIterator.
//
// If combineCJ is true, Han, Hiragana, and Katakana are all reported as
// UScriptJapanese so the CJK dictionary break iterator handles them.
func NewScriptIterator(combineCJ bool) *ScriptIterator {
	return &ScriptIterator{combineCJ: combineCJ}
}

// GetScriptStart returns the start position of the current script run.
func (si *ScriptIterator) GetScriptStart() int { return si.scriptStart }

// GetScriptLimit returns the exclusive end position of the current script run.
func (si *ScriptIterator) GetScriptLimit() int { return si.scriptLimit }

// GetScriptCode returns the UScript numeric code of the current script run.
func (si *ScriptIterator) GetScriptCode() int { return si.scriptCode }

// SetText configures the iterator to scan text[start : start+length].
func (si *ScriptIterator) SetText(text []rune, start, length int) {
	si.text = text
	si.start = start
	si.index = start
	si.limit = start + length
	si.scriptStart = start
	si.scriptLimit = start
	si.scriptCode = -1 // INVALID_CODE
}

// Next advances to the next script run.
// Returns true if another run exists, false if exhausted.
func (si *ScriptIterator) Next() bool {
	if si.scriptLimit >= si.limit {
		return false
	}

	si.scriptCode = UScriptCommon
	si.scriptStart = si.scriptLimit

	for si.index < si.limit {
		r := si.text[si.index]
		sc := si.getScript(r)

		if IsSameScript(si.scriptCode, sc) || IsCombiningMark(r) {
			si.index++
			// Inherited / Common inherits the surrounding script.
			if si.scriptCode <= UScriptInherited && sc > UScriptInherited {
				si.scriptCode = sc
			}
		} else {
			break
		}
	}

	si.scriptLimit = si.index
	return true
}

// getScript returns the effective UScript code for rune r, applying the
// combineCJ rule.
func (si *ScriptIterator) getScript(r rune) int {
	sc := GetScript(r)
	if si.combineCJ {
		switch sc {
		case UScriptHan, UScriptHiragana, UScriptKatakana:
			return UScriptJapanese
		}
		// Full-width digits (U+FF10–U+FF19) are treated as Latin when using
		// CJK dictionary breaking.
		if r >= 0xFF10 && r <= 0xFF19 {
			return UScriptLatin
		}
	}
	return sc
}
