// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package tokenattributes

// scriptName maps a UScript numeric code to its full Unicode script name.
//
// This table covers the scripts used by StandardTokenizer and
// DefaultICUTokenizerConfig; it is not exhaustive.
//
// Deviation: In Java, UScript.getName() and UScript.getShortName() consult
// the embedded ICU data files. This Go table is a compile-time constant
// derived from ISO 15924 and the ICU source for the scripts that matter
// in Lucene's tokenisation pipeline.
var scriptName = map[int]string{
	0:   "Common",
	1:   "Inherited",
	2:   "Arabic",
	3:   "Armenian",
	4:   "Bengali",
	5:   "Bopomofo",
	6:   "Cherokee",
	7:   "Coptic",
	8:   "Cyrillic",
	9:   "Deseret",
	10:  "Devanagari",
	11:  "Ethiopic",
	12:  "Georgian",
	13:  "Gothic",
	14:  "Greek",
	15:  "Gujarati",
	16:  "Gurmukhi",
	17:  "Han",
	18:  "Hangul",
	19:  "Hebrew",
	20:  "Hiragana",
	21:  "Kannada",
	22:  "Katakana",
	23:  "Khmer",
	24:  "Lao",
	25:  "Latin",
	26:  "Malayalam",
	27:  "Mongolian",
	28:  "Myanmar",
	29:  "Ogham",
	30:  "Old Italic",
	31:  "Oriya",
	32:  "Runic",
	33:  "Sinhala",
	34:  "Syriac",
	35:  "Tamil",
	36:  "Telugu",
	37:  "Thaana",
	38:  "Thai",
	39:  "Tibetan",
	40:  "Canadian Aboriginal",
	41:  "Yi",
	42:  "Tagalog",
	43:  "Hanunoo",
	44:  "Buhid",
	45:  "Tagbanwa",
	46:  "Braille",
	47:  "Cypriot",
	48:  "Limbu",
	49:  "Linear B",
	50:  "Osmanya",
	51:  "Shavian",
	52:  "Tai Le",
	53:  "Ugaritic",
	55:  "Buginese",
	56:  "Coptic",
	57:  "New Tai Lue",
	58:  "Glagolitic",
	59:  "Tifinagh",
	60:  "Syloti Nagri",
	61:  "Old Persian",
	62:  "Kharoshthi",
	63:  "Unknown",
	64:  "Balinese",
	65:  "Cuneiform",
	66:  "Phoenician",
	67:  "Phags Pa",
	68:  "Nko",
	69:  "Sundanese",
	70:  "Batak",
	71:  "Lepcha",
	72:  "Ol Chiki",
	73:  "Vai",
	74:  "Saurashtra",
	75:  "Kayah Li",
	76:  "Rejang",
	77:  "Lycian",
	78:  "Carian",
	79:  "Lydian",
	80:  "Cham",
	// Japanese is a composite (Han+Hiragana+Katakana) used by
	// DefaultICUTokenizerConfig; value matches UScript.JAPANESE = 105.
	105: "Japanese",
}

// scriptShortName maps a UScript numeric code to its ISO 15924 abbreviated
// script identifier.
var scriptShortName = map[int]string{
	0:   "Zyyy",
	1:   "Zinh",
	2:   "Arab",
	3:   "Armn",
	4:   "Beng",
	5:   "Bopo",
	6:   "Cher",
	7:   "Copt",
	8:   "Cyrl",
	9:   "Dsrt",
	10:  "Deva",
	11:  "Ethi",
	12:  "Geor",
	13:  "Goth",
	14:  "Grek",
	15:  "Gujr",
	16:  "Guru",
	17:  "Hani",
	18:  "Hang",
	19:  "Hebr",
	20:  "Hira",
	21:  "Knda",
	22:  "Kana",
	23:  "Khmr",
	24:  "Laoo",
	25:  "Latn",
	26:  "Mlym",
	27:  "Mong",
	28:  "Mymr",
	29:  "Ogam",
	30:  "Ital",
	31:  "Orya",
	32:  "Runr",
	33:  "Sinh",
	34:  "Syrc",
	35:  "Taml",
	36:  "Telu",
	37:  "Thaa",
	38:  "Thai",
	39:  "Tibt",
	40:  "Cans",
	41:  "Yiii",
	42:  "Tglg",
	43:  "Hano",
	44:  "Buhd",
	45:  "Tagb",
	46:  "Brai",
	47:  "Cprt",
	48:  "Limb",
	49:  "Linb",
	50:  "Osma",
	51:  "Shaw",
	52:  "Tale",
	53:  "Ugar",
	55:  "Bugi",
	56:  "Copt",
	57:  "Talu",
	58:  "Glag",
	59:  "Tfng",
	60:  "Sylo",
	61:  "Xpeo",
	62:  "Khar",
	63:  "Zzzz",
	64:  "Bali",
	65:  "Xsux",
	66:  "Phnx",
	67:  "Phag",
	68:  "Nkoo",
	69:  "Sund",
	70:  "Batk",
	71:  "Lepc",
	72:  "Olck",
	73:  "Vaii",
	74:  "Saur",
	75:  "Kali",
	76:  "Rjng",
	77:  "Lyci",
	78:  "Cari",
	79:  "Lydi",
	80:  "Cham",
	105: "Jpan",
}

// ScriptGetName returns the full script name for the given UScript code.
// Returns "Unknown" for unrecognised codes.
func ScriptGetName(code int) string {
	if name, ok := scriptName[code]; ok {
		return name
	}
	return "Unknown"
}

// ScriptGetShortName returns the ISO 15924 abbreviated identifier for the
// given UScript code. Returns "Zzzz" (Unknown) for unrecognised codes.
func ScriptGetShortName(code int) string {
	if name, ok := scriptShortName[code]; ok {
		return name
	}
	return "Zzzz"
}
