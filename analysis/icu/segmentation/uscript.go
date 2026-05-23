// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation

import "unicode"

// UScript numeric codes — a subset of com.ibm.icu.lang.UScript constants
// covering the scripts used by DefaultICUTokenizerConfig.
//
// Deviation: ICU4J's UScript.getScript() reads a comprehensive Unicode data
// table bundled with the ICU data files. This Go implementation uses
// Go's stdlib unicode.Is() range tables for script detection, which are
// generated from the same Unicode standard but available without ICO4J.
const (
	UScriptCommon     = 0
	UScriptInherited  = 1
	UScriptArabic     = 2
	UScriptArmenian   = 3
	UScriptBengali    = 4
	UScriptBopomofo   = 5
	UScriptCherokee   = 6
	UScriptCoptic     = 7
	UScriptCyrillic   = 8
	UScriptDevanagari = 10
	UScriptEthiopic   = 11
	UScriptGeorgian   = 12
	UScriptGreek      = 14
	UScriptGujarati   = 15
	UScriptGurmukhi   = 16
	UScriptHan        = 17
	UScriptHangul     = 18
	UScriptHebrew     = 19
	UScriptHiragana   = 20
	UScriptKannada    = 21
	UScriptKatakana   = 22
	UScriptKhmer      = 23
	UScriptLao        = 24
	UScriptLatin      = 25
	UScriptMalayalam  = 26
	UScriptMongolian  = 27
	UScriptMyanmar    = 28
	UScriptOriya      = 31
	UScriptSinhala    = 33
	UScriptSyriac     = 34
	UScriptTamil      = 35
	UScriptTelugu     = 36
	UScriptThaana     = 37
	UScriptThai       = 38
	UScriptTibetan    = 39
	UScriptYi         = 41
	UScriptUnknown    = 103
	// Japanese is a composite code used by DefaultICUTokenizerConfig for
	// CJK combined treatment. Value matches UScript.JAPANESE = 105.
	UScriptJapanese = 105
)

// scriptEntries maps Unicode range tables to UScript codes. This compact
// lookup is sufficient for the scripts relevant to Lucene's tokenisation
// pipeline.
var scriptEntries = []struct {
	table *unicode.RangeTable
	code  int
}{
	{unicode.Arabic, UScriptArabic},
	{unicode.Armenian, UScriptArmenian},
	{unicode.Bengali, UScriptBengali},
	{unicode.Bopomofo, UScriptBopomofo},
	{unicode.Cherokee, UScriptCherokee},
	{unicode.Coptic, UScriptCoptic},
	{unicode.Cyrillic, UScriptCyrillic},
	{unicode.Devanagari, UScriptDevanagari},
	{unicode.Ethiopic, UScriptEthiopic},
	{unicode.Georgian, UScriptGeorgian},
	{unicode.Greek, UScriptGreek},
	{unicode.Gujarati, UScriptGujarati},
	{unicode.Gurmukhi, UScriptGurmukhi},
	{unicode.Han, UScriptHan},
	{unicode.Hangul, UScriptHangul},
	{unicode.Hebrew, UScriptHebrew},
	{unicode.Hiragana, UScriptHiragana},
	{unicode.Kannada, UScriptKannada},
	{unicode.Katakana, UScriptKatakana},
	{unicode.Khmer, UScriptKhmer},
	{unicode.Lao, UScriptLao},
	{unicode.Latin, UScriptLatin},
	{unicode.Malayalam, UScriptMalayalam},
	{unicode.Mongolian, UScriptMongolian},
	{unicode.Myanmar, UScriptMyanmar},
	{unicode.Oriya, UScriptOriya},
	{unicode.Sinhala, UScriptSinhala},
	{unicode.Syriac, UScriptSyriac},
	{unicode.Tamil, UScriptTamil},
	{unicode.Telugu, UScriptTelugu},
	{unicode.Thaana, UScriptThaana},
	{unicode.Thai, UScriptThai},
	{unicode.Tibetan, UScriptTibetan},
	{unicode.Yi, UScriptYi},
}

// GetScript returns the UScript numeric code for rune r, using Go's stdlib
// unicode range tables.
//
// Returns UScriptCommon for spaces, digits, and other script-neutral runes;
// UScriptInherited for combining marks; UScriptUnknown for unrecognised runes.
func GetScript(r rune) int {
	// Fast path for Basic Latin.
	if r < 0x0080 {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return UScriptLatin
		}
		// Digits, punctuation, control chars → Common.
		return UScriptCommon
	}
	// Combining / inherited marks.
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Mc, r) {
		return UScriptInherited
	}
	for _, e := range scriptEntries {
		if unicode.Is(e.table, r) {
			return e.code
		}
	}
	return UScriptUnknown
}

// IsCombiningMark reports whether r is a Unicode combining mark
// (General_Category Mc, Mn, or Me).
func IsCombiningMark(r rune) bool {
	return unicode.Is(unicode.Mc, r) || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r)
}

// IsSameScript reports whether script code sc is compatible with the current
// script (same script, or one of Common/Inherited).
func IsSameScript(currentScript, sc int) bool {
	return currentScript == sc ||
		currentScript <= UScriptInherited ||
		sc <= UScriptInherited
}
