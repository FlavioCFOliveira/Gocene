// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package phonetic

import (
	"strings"
	"unicode"
)

// DoubleMetaphone implements the Double Metaphone phonetic encoding algorithm.
//
// This is a Go port of
// org.apache.commons.codec.language.DoubleMetaphone from
// Apache Commons Codec 1.17.2.
type DoubleMetaphone struct {
	// MaxCodeLen is the maximum length of the returned code. Default is 4.
	MaxCodeLen int
}

// NewDoubleMetaphone creates a DoubleMetaphone with the default max code length of 4.
func NewDoubleMetaphone() *DoubleMetaphone {
	return &DoubleMetaphone{MaxCodeLen: 4}
}

// Encode encodes a value to its Double Metaphone primary code.
func (d *DoubleMetaphone) Encode(value string) string {
	primary, _ := d.DoubleMetaphoneValue(value)
	return primary
}

// DoubleMetaphoneValue returns both the primary and alternate Double Metaphone codes.
func (d *DoubleMetaphone) DoubleMetaphoneValue(value string) (string, string) {
	if value == "" {
		return "", ""
	}
	maxLen := d.MaxCodeLen
	if maxLen <= 0 {
		maxLen = 4
	}

	// Clean up the value: convert to upper case.
	value = cleanInput(value)
	if value == "" {
		return "", ""
	}

	slavoGermanic := isSlavoGermanic(value)

	// Skip initial non-alpha characters.
	index := 0

	if isVowel(value, index) {
		primary := "A"
		alternate := "A"
		index = 1
		index, primary, alternate = d.encode(value, index, primary, alternate, slavoGermanic, maxLen)
		return primary, alternate
	}

	primary := ""
	alternate := ""

	// Handle initial silent letters.
	switch {
	case value[0] == 'G' && len(value) > 1 && (value[1] == 'N'):
		// GN
	case value[0] == 'A' && len(value) > 1 && value[1] == 'E':
		// AE
		index++
		primary = "A"
		alternate = "A"
	case value[0] == 'W' && len(value) > 1 && value[1] == 'R':
		// WR -> R
		index++
	case value[0] == 'P' && len(value) > 1 && value[1] == 'N':
		// PN -> N
		index++
	case value[0] == 'K' && len(value) > 1 && value[1] == 'N':
		// KN -> N
		index++
	}

	index, primary, alternate = d.encode(value, index, primary, alternate, slavoGermanic, maxLen)
	return primary, alternate
}

// Primary returns the primary Double Metaphone code.
func (d *DoubleMetaphone) Primary(value string) string {
	p, _ := d.DoubleMetaphoneValue(value)
	return p
}

// Alternate returns the alternate Double Metaphone code.
func (d *DoubleMetaphone) Alternate(value string) string {
	_, a := d.DoubleMetaphoneValue(value)
	return a
}

// IsDoubleMetaphoneEqual reports whether two values have the same primary
// Double Metaphone code.
func (d *DoubleMetaphone) IsDoubleMetaphoneEqual(value1, value2 string, alternate bool) bool {
	v1, v1alt := d.DoubleMetaphoneValue(value1)
	v2, v2alt := d.DoubleMetaphoneValue(value2)
	if alternate {
		return v1alt == v2alt
	}
	return v1 == v2
}

// cleanInput strips leading/trailing whitespace and converts to upper case,
// keeping only ASCII letters (simplification matching Commons Codec behaviour
// for the phonetic encoding context used by Lucene).
func cleanInput(s string) string {
	s = strings.TrimSpace(s)
	return strings.ToUpper(s)
}

// isSlavoGermanic reports whether the string is Slavic or Germanic.
func isSlavoGermanic(s string) bool {
	return strings.Contains(s, "W") ||
		strings.Contains(s, "K") ||
		strings.Contains(s, "CZ") ||
		strings.Contains(s, "WITZ")
}

// isVowel reports whether position i in value is a vowel.
func isVowel(value string, i int) bool {
	if i < 0 || i >= len(value) {
		return false
	}
	c := value[i]
	return c == 'A' || c == 'E' || c == 'I' || c == 'O' || c == 'U' || c == 'Y'
}

// charAt returns the character at position i, or 0 if out of bounds.
func charAt(value string, i int) byte {
	if i < 0 || i >= len(value) {
		return 0
	}
	return value[i]
}

// regionMatch checks if value contains substr at offset start.
func regionMatch(value string, start int, substr string) bool {
	if start < 0 || start+len(substr) > len(value) {
		return false
	}
	return value[start:start+len(substr)] == substr
}

// stringAt checks if value contains any of the provided strings starting at start,
// where the match length falls in [length, length] (for Commons Codec compat,
// all substrings passed must be the same length, which is the usage here).
func stringAt(value string, start int, length int, candidates ...string) bool {
	if start < 0 || start+length > len(value) {
		return false
	}
	sub := value[start : start+length]
	for _, c := range candidates {
		if sub == c {
			return true
		}
	}
	return false
}

// contains checks whether sub is in value.
func containsStr(value, sub string) bool {
	return strings.Contains(value, sub)
}

// The following helper is used for variable-length matches in Commons Codec.
// stringAtVar checks if value starting at start matches any of the candidates
// (candidates may have different lengths).
func stringAtVar(value string, start int, candidates ...string) bool {
	for _, c := range candidates {
		if regionMatch(value, start, c) {
			return true
		}
	}
	return false
}

// encode processes the value from position index onward, building up the
// primary and alternate metaphone codes.
func (d *DoubleMetaphone) encode(value string, index int, primary, alternate string, slavoGermanic bool, maxLen int) (int, string, string) {
	n := len(value)
	for index < n {
		if len(primary) >= maxLen && len(alternate) >= maxLen {
			break
		}
		c := value[index]
		switch {
		case unicode.IsLetter(rune(c)) == false:
			index++
		case c == 'A', c == 'E', c == 'I', c == 'O', c == 'U', c == 'Y':
			// Vowels: only encoded at the beginning.
			// (already handled before entering this function for leading vowels)
			index++
		case c == 'B':
			// B: "-mb" is silent e.g. "dumb"
			primary += "P"
			alternate += "P"
			if charAt(value, index+1) == 'B' {
				index += 2
			} else {
				index++
			}
		case c == 'Ç':
			primary += "S"
			alternate += "S"
			index++
		case c == 'C':
			index, primary, alternate = d.encodeC(value, index, primary, alternate)
		case c == 'D':
			index, primary, alternate = d.encodeD(value, index, primary, alternate)
		case c == 'F':
			primary += "F"
			alternate += "F"
			if charAt(value, index+1) == 'F' {
				index += 2
			} else {
				index++
			}
		case c == 'G':
			index, primary, alternate = d.encodeG(value, index, primary, alternate, slavoGermanic)
		case c == 'H':
			index, primary, alternate = d.encodeH(value, index, primary, alternate)
		case c == 'J':
			index, primary, alternate = d.encodeJ(value, index, primary, alternate, slavoGermanic)
		case c == 'K':
			if charAt(value, index+1) == 'K' {
				index += 2
			} else {
				index++
			}
			primary += "K"
			alternate += "K"
		case c == 'L':
			index, primary, alternate = d.encodeL(value, index, primary, alternate)
		case c == 'M':
			index, primary, alternate = d.encodeM(value, index, primary, alternate)
		case c == 'N':
			if charAt(value, index+1) == 'N' {
				index += 2
			} else {
				index++
			}
			primary += "N"
			alternate += "N"
		case c == 'Ñ':
			index++
			primary += "N"
			alternate += "N"
		case c == 'P':
			index, primary, alternate = d.encodeP(value, index, primary, alternate)
		case c == 'Q':
			if charAt(value, index+1) == 'Q' {
				index += 2
			} else {
				index++
			}
			primary += "K"
			alternate += "K"
		case c == 'R':
			index, primary, alternate = d.encodeR(value, index, primary, alternate, slavoGermanic)
		case c == 'S':
			index, primary, alternate = d.encodeS(value, index, primary, alternate, slavoGermanic)
		case c == 'T':
			index, primary, alternate = d.encodeT(value, index, primary, alternate)
		case c == 'V':
			if charAt(value, index+1) == 'V' {
				index += 2
			} else {
				index++
			}
			primary += "F"
			alternate += "F"
		case c == 'W':
			index, primary, alternate = d.encodeW(value, index, primary, alternate)
		case c == 'X':
			primary += "KS"
			alternate += "KS"
			if charAt(value, index+1) == 'X' {
				index += 2
			} else {
				index++
			}
		case c == 'Z':
			index, primary, alternate = d.encodeZ(value, index, primary, alternate, slavoGermanic)
		default:
			index++
		}
	}

	if len(primary) > maxLen {
		primary = primary[:maxLen]
	}
	if len(alternate) > maxLen {
		alternate = alternate[:maxLen]
	}
	return index, primary, alternate
}

func (d *DoubleMetaphone) encodeC(value string, index int, primary, alternate string) (int, string, string) {
	// various C cases
	if index > 1 && !isVowel(value, index-2) &&
		stringAt(value, index-1, 3, "ACH") &&
		charAt(value, index+2) != 'I' &&
		(charAt(value, index+2) != 'E' ||
			stringAt(value, index-2, 6, "BACHER", "MACHER")) {
		primary += "K"
		alternate += "K"
		index += 2
		return index, primary, alternate
	}
	// special case "caesar"
	if index == 0 && stringAt(value, index, 6, "CAESAR") {
		primary += "S"
		alternate += "S"
		index += 2
		return index, primary, alternate
	}
	// italian "chianti"
	if stringAt(value, index, 4, "CHIA") {
		primary += "K"
		alternate += "K"
		index += 2
		return index, primary, alternate
	}
	if stringAt(value, index, 2, "CH") {
		// find CH
		if index > 0 && stringAt(value, index, 4, "CHAE") {
			primary += "K"
			alternate += "X"
			index += 2
			return index, primary, alternate
		}
		// greek roots
		if index == 0 &&
			(stringAt(value, index+1, 5, "HARAC", "HARIS") ||
				stringAt(value, index+1, 3, "HOR", "HYM", "HIA", "HEM")) &&
			!stringAt(value, 0, 5, "CHORE") {
			primary += "K"
			alternate += "K"
			index += 2
			return index, primary, alternate
		}
		// germanic, greek, or otherwise 'ch' for 'kh' sound
		if (stringAt(value, 0, 4, "VAN ", "VON ") || stringAt(value, 0, 3, "SCH")) ||
			stringAt(value, index-2, 6, "ORCHES", "ARCHIT", "ORCHID") ||
			stringAt(value, index+2, 1, "T", "S") ||
			((stringAt(value, index-1, 1, "A", "O", "U", "E") || index == 0) &&
				stringAt(value, index+2, 1,
					"L", "R", "N", "M", "B", "H", "F", "V", "W", " ")) {
			primary += "K"
			alternate += "K"
		} else {
			if index > 0 {
				if stringAt(value, 0, 2, "MC") {
					primary += "K"
					alternate += "K"
				} else {
					primary += "X"
					alternate += "K"
				}
			} else {
				primary += "X"
				alternate += "X"
			}
		}
		index += 2
		return index, primary, alternate
	}
	// e.g. "Czerny"
	if stringAt(value, index, 2, "CZ") &&
		!stringAt(value, index-2, 4, "WICZ") {
		primary += "S"
		alternate += "X"
		index += 2
		return index, primary, alternate
	}
	// e.g. "focaccia"
	if stringAt(value, index+1, 3, "CIA") {
		primary += "X"
		alternate += "X"
		index += 3
		return index, primary, alternate
	}
	// double C, but not McClellan
	if stringAt(value, index, 2, "CC") &&
		!(index == 1 && charAt(value, 0) == 'M') {
		// "bellocchio" but not "bacchus"
		if stringAt(value, index+2, 1, "I", "E", "H") &&
			!stringAt(value, index+2, 2, "HU") {
			// "accident" "accede"
			if (index == 1 && charAt(value, index-1) == 'A') ||
				stringAt(value, index-1, 5, "UCCEE", "UCCES") {
				primary += "KS"
				alternate += "KS"
			} else {
				primary += "X"
				alternate += "X"
			}
			index += 3
			return index, primary, alternate
		}
		// Pierce's rule
		primary += "K"
		alternate += "K"
		index += 2
		return index, primary, alternate
	}
	if stringAt(value, index, 2, "CK", "CG", "CQ") {
		primary += "K"
		alternate += "K"
		index += 2
		return index, primary, alternate
	}
	if stringAt(value, index, 2, "CI", "CE", "CY") {
		// Italian vs. English
		if stringAt(value, index, 3, "CIO", "CIE", "CIA") {
			primary += "S"
			alternate += "X"
		} else {
			primary += "S"
			alternate += "S"
		}
		index += 2
		return index, primary, alternate
	}
	// else
	primary += "K"
	alternate += "K"
	// name sent in is either Scpqx or CK or CG
	if stringAt(value, index+1, 2, " C", " Q", " G") {
		index += 3
	} else if stringAt(value, index+1, 1, "C", "K", "Q") &&
		!stringAt(value, index+1, 2, "CE", "CI") {
		index += 2
	} else {
		index++
	}
	return index, primary, alternate
}

func (d *DoubleMetaphone) encodeD(value string, index int, primary, alternate string) (int, string, string) {
	if stringAt(value, index, 2, "DG") {
		if stringAt(value, index+2, 1, "I", "E", "Y") {
			// e.g. "edge"
			primary += "J"
			alternate += "J"
			index += 3
		} else {
			// e.g. "Edgar"
			primary += "TK"
			alternate += "TK"
			index += 2
		}
		return index, primary, alternate
	}
	if stringAt(value, index, 2, "DT", "DD") {
		primary += "T"
		alternate += "T"
		index += 2
		return index, primary, alternate
	}
	primary += "T"
	alternate += "T"
	index++
	return index, primary, alternate
}

func (d *DoubleMetaphone) encodeG(value string, index int, primary, alternate string, slavoGermanic bool) (int, string, string) {
	if charAt(value, index+1) == 'H' {
		if index > 0 && !isVowel(value, index-1) {
			primary += "K"
			alternate += "K"
			index += 2
			return index, primary, alternate
		}
		if index == 0 {
			// "ghislane", "ghiradelli"
			if charAt(value, index+2) == 'I' {
				primary += "J"
				alternate += "J"
			} else {
				primary += "K"
				alternate += "K"
			}
			index += 2
			return index, primary, alternate
		}
		// Parker's rule: "(ch)ges" -> "j"
		if (index > 1 && stringAt(value, index-2, 1, "B", "H", "D")) ||
			(index > 2 && stringAt(value, index-3, 1, "B", "H", "D")) ||
			(index > 3 && stringAt(value, index-4, 1, "B", "H")) {
			index += 2
			return index, primary, alternate
		}
		// e.g. "laugh", "McLaughlin", "cough", "gough", "rough", "tough"
		if index > 2 &&
			charAt(value, index-1) == 'U' &&
			stringAt(value, index-3, 1, "C", "G", "L", "R", "T") {
			primary += "F"
			alternate += "F"
		} else if index > 0 && charAt(value, index-1) != 'I' {
			primary += "K"
			alternate += "K"
		}
		index += 2
		return index, primary, alternate
	}
	if charAt(value, index+1) == 'N' {
		if index == 1 && isVowel(value, 0) && !slavoGermanic {
			primary += "KN"
			alternate += "N"
		} else {
			// not e.g. "cagney"
			if !stringAt(value, index+2, 2, "EY") &&
				charAt(value, index+1) != 'Y' && !slavoGermanic {
				primary += "N"
				alternate += "KN"
			} else {
				primary += "KN"
				alternate += "KN"
			}
		}
		index += 2
		return index, primary, alternate
	}
	// "tagliaro" should be encoded as "TKLR" or "TLR"
	if stringAt(value, index+1, 2, "LI") && !slavoGermanic {
		primary += "KL"
		alternate += "L"
		index += 2
		return index, primary, alternate
	}
	// initial "GY-", "GE-", "GI-"
	if index == 0 &&
		(charAt(value, index+1) == 'Y' ||
			stringAt(value, index+1, 2, "ES", "EP", "EB", "EL", "EY", "IB", "IL", "IN", "IE", "EI", "ER")) {
		primary += "K"
		alternate += "J"
		index += 2
		return index, primary, alternate
	}
	// "GER", "GEY"
	if (stringAt(value, index+1, 2, "ER") || charAt(value, index+1) == 'Y') &&
		!stringAt(value, 0, 6, "DANGER", "RANGER", "MANGER") &&
		!stringAt(value, index-1, 1, "E", "I") &&
		!stringAt(value, index-1, 3, "RGY", "OGY") {
		primary += "K"
		alternate += "J"
		index += 2
		return index, primary, alternate
	}
	// Italian e.g. "biaggi"
	if stringAt(value, index+1, 1, "E", "I", "Y") ||
		stringAt(value, index-1, 4, "AGGI", "OGGI") {
		// obvious germanic
		if stringAt(value, 0, 4, "VAN ", "VON ") ||
			stringAt(value, 0, 3, "SCH") ||
			stringAt(value, index+1, 2, "ET") {
			primary += "K"
			alternate += "K"
		} else {
			// always K for the germanic form
			if stringAt(value, index+1, 4, "IER ") {
				primary += "J"
				alternate += "J"
			} else {
				primary += "J"
				alternate += "K"
			}
		}
		index += 2
		return index, primary, alternate
	}
	if charAt(value, index+1) == 'G' {
		index += 2
	} else {
		index++
	}
	primary += "K"
	alternate += "K"
	return index, primary, alternate
}

func (d *DoubleMetaphone) encodeH(value string, index int, primary, alternate string) (int, string, string) {
	// only keep if first or before vowel or after vowel
	if (index == 0 || isVowel(value, index-1)) && isVowel(value, index+1) {
		primary += "H"
		alternate += "H"
		index += 2
	} else {
		index++
	}
	return index, primary, alternate
}

func (d *DoubleMetaphone) encodeJ(value string, index int, primary, alternate string, slavoGermanic bool) (int, string, string) {
	// obvious Spanish, "jose", "san jacinto"
	if stringAt(value, index, 4, "JOSE") || stringAt(value, 0, 4, "SAN ") {
		if (index == 0 && charAt(value, index+4) == ' ') || len(value) == 4 ||
			stringAt(value, 0, 4, "SAN ") {
			primary += "H"
			alternate += "H"
		} else {
			primary += "J"
			alternate += "H"
		}
		index++
		return index, primary, alternate
	}
	if index == 0 && !stringAt(value, index, 4, "JOSE") {
		primary += "J"
		alternate += "A"
	} else {
		if isVowel(value, index-1) && !slavoGermanic &&
			(charAt(value, index+1) == 'A' || charAt(value, index+1) == 'O') {
			primary += "J"
			alternate += "H"
		} else {
			if index == len(value)-1 {
				primary += "J"
				// alternate is blank
			} else {
				if !stringAt(value, index+1, 1,
					"L", "T", "K", "S", "N", "M", "B", "Z") &&
					!stringAt(value, index-1, 1, "S", "K", "L") {
					primary += "J"
					alternate += "J"
				}
			}
		}
	}
	if charAt(value, index+1) == 'J' {
		index += 2
	} else {
		index++
	}
	return index, primary, alternate
}

func (d *DoubleMetaphone) encodeL(value string, index int, primary, alternate string) (int, string, string) {
	if charAt(value, index+1) == 'L' {
		// Spanish e.g. "cabrillo", "gallegos"
		if (index == len(value)-3 &&
			stringAt(value, index-1, 4, "ILLO", "ILLA", "ALLE")) ||
			((stringAt(value, len(value)-2, 2, "AS", "OS") ||
				stringAt(value, len(value)-1, 1, "A", "O")) &&
				stringAt(value, index-1, 4, "ALLE")) {
			primary += "L"
			// alternate is blank
			index += 2
			return index, primary, alternate
		}
		index += 2
	} else {
		index++
	}
	primary += "L"
	alternate += "L"
	return index, primary, alternate
}

func (d *DoubleMetaphone) encodeM(value string, index int, primary, alternate string) (int, string, string) {
	if (stringAt(value, index-1, 3, "UMB") &&
		(index+1 == len(value)-1 || stringAt(value, index+2, 2, "ER"))) ||
		charAt(value, index+1) == 'M' {
		if charAt(value, index+1) == 'M' {
			index += 2
		} else {
			index++
		}
	} else {
		index++
	}
	primary += "M"
	alternate += "M"
	return index, primary, alternate
}

func (d *DoubleMetaphone) encodeP(value string, index int, primary, alternate string) (int, string, string) {
	if charAt(value, index+1) == 'H' {
		primary += "F"
		alternate += "F"
		index += 2
		return index, primary, alternate
	}
	// also account for "Campbell", "raspberry"
	if stringAt(value, index+1, 1, "P", "B") {
		index += 2
	} else {
		index++
	}
	primary += "P"
	alternate += "P"
	return index, primary, alternate
}

func (d *DoubleMetaphone) encodeR(value string, index int, primary, alternate string, slavoGermanic bool) (int, string, string) {
	// French e.g. "rogier", but exclude "hochmeier"
	if index == len(value)-1 && !slavoGermanic &&
		stringAt(value, index-2, 2, "IE") &&
		!stringAt(value, index-4, 2, "ME", "MA") {
		// alternate is blank
		alternate += "R"
	} else {
		primary += "R"
		alternate += "R"
	}
	if charAt(value, index+1) == 'R' {
		index += 2
	} else {
		index++
	}
	return index, primary, alternate
}

func (d *DoubleMetaphone) encodeS(value string, index int, primary, alternate string, slavoGermanic bool) (int, string, string) {
	// special cases "island", "isle", "carlisle", "carlysle"
	if stringAt(value, index-1, 3, "ISL", "YSL") {
		index++
		return index, primary, alternate
	}
	// special case "sugar-"
	if index == 0 && stringAt(value, index, 5, "SUGAR") {
		primary += "X"
		alternate += "S"
		index++
		return index, primary, alternate
	}
	if stringAt(value, index, 2, "SH") {
		// Germanic
		if stringAt(value, index+1, 4, "HEIM", "HOEK", "HOLM", "HOLZ") {
			primary += "S"
			alternate += "S"
		} else {
			primary += "X"
			alternate += "X"
		}
		index += 2
		return index, primary, alternate
	}
	// Italian & Armenian
	if stringAt(value, index, 3, "SIO", "SIA") ||
		stringAt(value, index, 4, "SIAN") {
		if !slavoGermanic {
			primary += "S"
			alternate += "X"
		} else {
			primary += "S"
			alternate += "S"
		}
		index += 3
		return index, primary, alternate
	}
	// German & anglicised "Sz"
	if (index == 0 && stringAt(value, index+1, 1, "M", "N", "L", "W")) ||
		stringAt(value, index+1, 1, "Z") {
		primary += "S"
		alternate += "X"
		if stringAt(value, index+1, 1, "Z") {
			index += 2
		} else {
			index++
		}
		return index, primary, alternate
	}
	if stringAt(value, index, 2, "SC") {
		index, primary, alternate = d.encodeSC(value, index, primary, alternate)
		return index, primary, alternate
	}
	// french e.g. "resnais", "artois"
	if index == len(value)-1 &&
		stringAt(value, index-2, 2, "AI", "OI") {
		// alternate is blank
		alternate += "S"
	} else {
		primary += "S"
		alternate += "S"
	}
	if stringAt(value, index+1, 1, "S", "Z") {
		index += 2
	} else {
		index++
	}
	return index, primary, alternate
}

func (d *DoubleMetaphone) encodeSC(value string, index int, primary, alternate string) (int, string, string) {
	// Schlesinger's rule
	if charAt(value, index+2) == 'H' {
		// dutch origin, e.g. "school", "schooner"
		if stringAt(value, index+3, 2, "OO", "ER", "EN", "UY", "ED", "EM") {
			// "schermerhorn", "schenker"
			if stringAt(value, index+3, 2, "ER", "EN") {
				primary += "X"
				alternate += "SK"
			} else {
				primary += "SK"
				alternate += "SK"
			}
			index += 3
			return index, primary, alternate
		}
		if index == 0 && !isVowel(value, 3) && charAt(value, 3) != 'W' {
			primary += "X"
			alternate += "S"
		} else {
			primary += "X"
			alternate += "X"
		}
		index += 3
		return index, primary, alternate
	}
	if stringAt(value, index+2, 1, "I", "E", "Y") {
		primary += "S"
		alternate += "S"
		index += 3
		return index, primary, alternate
	}
	primary += "SK"
	alternate += "SK"
	index += 3
	return index, primary, alternate
}

func (d *DoubleMetaphone) encodeT(value string, index int, primary, alternate string) (int, string, string) {
	if stringAt(value, index, 4, "TION") ||
		stringAt(value, index, 3, "TIA", "TCH") {
		primary += "X"
		alternate += "X"
		index += 3
		return index, primary, alternate
	}
	if stringAt(value, index, 2, "TH") ||
		stringAt(value, index, 3, "TTH") {
		// special case "thomas", "thames" or germanic
		if stringAt(value, index+2, 2, "OM", "AM") ||
			stringAt(value, 0, 4, "VAN ", "VON ") ||
			stringAt(value, 0, 3, "SCH") {
			primary += "T"
			alternate += "T"
		} else {
			primary += "0"
			alternate += "T"
		}
		index += 2
		return index, primary, alternate
	}
	if stringAt(value, index+1, 1, "T", "D") {
		index += 2
	} else {
		index++
	}
	primary += "T"
	alternate += "T"
	return index, primary, alternate
}

func (d *DoubleMetaphone) encodeW(value string, index int, primary, alternate string) (int, string, string) {
	// can also be in middle of word
	if stringAt(value, index, 2, "WR") {
		primary += "R"
		alternate += "R"
		index += 2
		return index, primary, alternate
	}
	if index == 0 && (isVowel(value, index+1) || stringAt(value, index, 2, "WH")) {
		// Wasserman should match Vasserman
		if isVowel(value, index+1) {
			primary += "A"
			alternate += "F"
		} else {
			// need Uomo to match Womo
			primary += "A"
			alternate += "A"
		}
		index++
		return index, primary, alternate
	}
	// Arnow should match Arnoff
	if (index == len(value)-1 && isVowel(value, index-1)) ||
		stringAt(value, index-1, 5, "EWSKI", "EWSKY", "OWSKI", "OWSKY") ||
		stringAt(value, 0, 3, "SCH") {
		// alternate is blank
		alternate += "F"
		index++
		return index, primary, alternate
	}
	// Polish e.g. "filipowicz"
	if stringAt(value, index, 4, "WICZ", "WITZ") {
		primary += "TS"
		alternate += "FX"
		index += 4
		return index, primary, alternate
	}
	index++
	return index, primary, alternate
}

func (d *DoubleMetaphone) encodeZ(value string, index int, primary, alternate string, slavoGermanic bool) (int, string, string) {
	// Chinese Pinyin e.g. "zhao"
	if charAt(value, index+1) == 'H' {
		primary += "J"
		alternate += "J"
		index += 2
		return index, primary, alternate
	}
	if stringAt(value, index+1, 2, "ZO", "ZI", "ZA") ||
		(slavoGermanic && index > 0 && charAt(value, index-1) != 'T') {
		primary += "S"
		alternate += "TS"
	} else {
		primary += "S"
		alternate += "S"
	}
	if charAt(value, index+1) == 'Z' {
		index += 2
	} else {
		index++
	}
	return index, primary, alternate
}
