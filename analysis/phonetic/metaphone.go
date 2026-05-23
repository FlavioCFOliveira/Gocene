// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package phonetic

import (
	"strings"
)

// Metaphone implements the Metaphone phonetic encoding algorithm.
//
// This is a Go port of
// org.apache.commons.codec.language.Metaphone from
// Apache Commons Codec 1.17.2.
type Metaphone struct {
	// MaxCodeLen is the maximum length of the code. Default is 4.
	MaxCodeLen int
}

// NewMetaphone creates a Metaphone with the default max code length of 4.
func NewMetaphone() *Metaphone {
	return &Metaphone{MaxCodeLen: 4}
}

// Encode encodes a string to its Metaphone code.
func (m *Metaphone) Encode(value string) string {
	return m.metaphone(value)
}

func (m *Metaphone) metaphone(str string) string {
	if str == "" {
		return ""
	}

	maxLen := m.MaxCodeLen
	if maxLen <= 0 {
		maxLen = 4
	}

	// Convert to upper case, extract only letters.
	s := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' {
			return r - 32
		}
		if r >= 'A' && r <= 'Z' {
			return r
		}
		return -1
	}, str)

	if s == "" {
		return ""
	}

	// Initial pre-processing:
	// Drop initial silent consonant pairs.
	switch {
	case len(s) >= 2 && s[0] == 'A' && s[1] == 'E':
		s = s[1:]
	case len(s) >= 2 && s[0] == 'G' && s[1] == 'N':
		s = s[2:]
	case len(s) >= 2 && s[0] == 'K' && s[1] == 'N':
		s = s[1:]
	case len(s) >= 2 && s[0] == 'P' && s[1] == 'N':
		s = s[1:]
	case len(s) >= 2 && s[0] == 'W' && s[1] == 'R':
		s = s[1:]
	}

	// Initial vowel: use A as representative
	if len(s) > 0 && isMetaphoneVowel(s[0]) {
		result := string(s[0])
		s = s[1:]
		return metaEncode(s, result, maxLen)
	}

	return metaEncode(s, "", maxLen)
}

func isMetaphoneVowel(c byte) bool {
	return c == 'A' || c == 'E' || c == 'I' || c == 'O' || c == 'U'
}

func metaAt(s string, i int) byte {
	if i < 0 || i >= len(s) {
		return 0
	}
	return s[i]
}

func metaSlice(s string, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end > len(s) {
		end = len(s)
	}
	if start >= end {
		return ""
	}
	return s[start:end]
}

func metaEncode(s string, result string, maxLen int) string {
	n := len(s)
	for i := 0; i < n && len(result) < maxLen; i++ {
		c := s[i]

		// Drop duplicate adjacent letters, except for C.
		if c != 'C' && i > 0 && s[i-1] == c {
			continue
		}

		switch c {
		case 'A', 'E', 'I', 'O', 'U':
			// Vowels after first position are dropped.
			continue
		case 'B':
			// Drop B if after M at end.
			if i+1 == n && i > 0 && s[i-1] == 'M' {
				continue
			}
			result += "B"
		case 'C':
			if metaAt(s, i+1) == 'I' || metaAt(s, i+1) == 'E' || metaAt(s, i+1) == 'Y' {
				if metaAt(s, i+1) == 'I' && metaAt(s, i+2) == 'A' {
					result += "X"
				} else {
					result += "S"
				}
				if metaAt(s, i+1) == 'E' || metaAt(s, i+1) == 'I' {
					i++
				}
			} else if metaAt(s, i+1) == 'H' {
				result += "X"
				i++
			} else if metaSlice(s, i+1, i+3) == "IA" {
				result += "X"
			} else {
				result += "K"
				if metaAt(s, i+1) == 'K' {
					i++
				}
			}
		case 'D':
			if metaAt(s, i+1) == 'G' {
				if metaAt(s, i+2) == 'E' || metaAt(s, i+2) == 'I' || metaAt(s, i+2) == 'Y' {
					result += "J"
					i += 2
				} else {
					result += "TK"
					i++
				}
			} else {
				result += "T"
			}
		case 'F':
			result += "F"
			if metaAt(s, i+1) == 'F' {
				i++
			}
		case 'G':
			if metaAt(s, i+1) == 'H' {
				if i > 0 && !isMetaphoneVowel(s[i-1]) {
					result += "K"
					i++
				} else if i == 0 {
					if metaAt(s, i+2) == 'E' || metaAt(s, i+2) == 'I' {
						result += "K"
					} else {
						result += "K"
					}
					i++
				}
				// else silent
			} else if metaAt(s, i+1) == 'N' {
				if i == 0 {
					if isMetaphoneVowel(metaAt(s, i+2)) {
						result += "KN"
						i++
					}
					// else: silent GN
				} else {
					if metaSlice(s, i+2, i+4) != "EY" && metaAt(s, i+1) == 'N' {
						// check for non-silent
					} else {
						result += "N"
					}
					i++
				}
			} else if (metaAt(s, i+1) == 'E' || metaAt(s, i+1) == 'I' || metaAt(s, i+1) == 'Y') &&
				!(i > 0 && s[i-1] == 'G') {
				result += "K"
			} else if i > 0 && s[i-1] != 'G' {
				result += "K"
			} else if i == 0 {
				result += "K"
			}
		case 'H':
			// Keep H before a vowel if not after a vowel.
			if isMetaphoneVowel(metaAt(s, i+1)) {
				if i == 0 || !isMetaphoneVowel(s[i-1]) {
					result += "H"
				}
			}
		case 'J':
			result += "J"
		case 'K':
			if i == 0 || s[i-1] != 'C' {
				result += "K"
			}
		case 'L':
			result += "L"
			if metaAt(s, i+1) == 'L' {
				i++
			}
		case 'M':
			result += "M"
		case 'N':
			result += "N"
			if metaAt(s, i+1) == 'N' {
				i++
			}
		case 'P':
			if metaAt(s, i+1) == 'H' {
				result += "F"
				i++
			} else {
				result += "P"
				if metaAt(s, i+1) == 'P' {
					i++
				}
			}
		case 'Q':
			result += "K"
			if metaAt(s, i+1) == 'Q' {
				i++
			}
		case 'R':
			result += "R"
			if metaAt(s, i+1) == 'R' {
				i++
			}
		case 'S':
			if metaSlice(s, i+1, i+3) == "IO" || metaSlice(s, i+1, i+3) == "IA" {
				result += "X"
			} else if metaAt(s, i+1) == 'H' ||
				(metaAt(s, i+1) == 'C' && metaAt(s, i+2) == 'H') {
				result += "X"
				if metaAt(s, i+1) == 'H' {
					i++
				} else {
					i += 2
				}
			} else {
				result += "S"
				if metaAt(s, i+1) == 'S' || metaAt(s, i+1) == 'Z' {
					i++
				}
			}
		case 'T':
			if metaSlice(s, i+1, i+3) == "IA" || metaSlice(s, i+1, i+3) == "IO" {
				result += "X"
			} else if metaAt(s, i+1) == 'H' {
				result += "0"
				i++
			} else if !(metaAt(s, i+1) == 'C' && metaAt(s, i+2) == 'H') {
				result += "T"
				if metaAt(s, i+1) == 'T' || metaAt(s, i+1) == 'D' {
					i++
				}
			}
		case 'V':
			result += "F"
			if metaAt(s, i+1) == 'V' {
				i++
			}
		case 'W', 'Y':
			if isMetaphoneVowel(metaAt(s, i+1)) {
				result += string(c)
			}
		case 'X':
			result += "KS"
		case 'Z':
			result += "S"
		}
	}

	if len(result) > maxLen {
		return result[:maxLen]
	}
	return result
}
