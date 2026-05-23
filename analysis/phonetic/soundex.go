// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package phonetic

import (
	"strings"
)

// Soundex implements the American Soundex phonetic encoding algorithm.
//
// This is a Go port of
// org.apache.commons.codec.language.Soundex from
// Apache Commons Codec 1.17.2.
type Soundex struct{}

// NewSoundex creates a new Soundex encoder.
func NewSoundex() *Soundex {
	return &Soundex{}
}

// soundexTable maps each letter A–Z to its Soundex code digit (0 = not coded).
var soundexTable = [26]byte{
	'0', // A
	'1', // B
	'2', // C
	'3', // D
	'0', // E
	'1', // F
	'2', // G
	'0', // H
	'0', // I
	'2', // J
	'2', // K
	'4', // L
	'5', // M
	'5', // N
	'0', // O
	'1', // P
	'2', // Q
	'6', // R
	'2', // S
	'3', // T
	'0', // U
	'1', // V
	'0', // W
	'2', // X
	'0', // Y
	'2', // Z
}

func soundexCode(c byte) byte {
	if c < 'A' || c > 'Z' {
		return '?'
	}
	return soundexTable[c-'A']
}

// Encode encodes a string to its American Soundex code.
func (s *Soundex) Encode(value string) string {
	if value == "" {
		return ""
	}
	upper := strings.ToUpper(value)
	// Strip non-letters.
	var buf []byte
	for i := 0; i < len(upper); i++ {
		c := upper[i]
		if c >= 'A' && c <= 'Z' {
			buf = append(buf, c)
		}
	}
	if len(buf) == 0 {
		return ""
	}

	// Keep the first letter.
	first := buf[0]
	result := make([]byte, 0, 4)
	result = append(result, first)

	prev := soundexCode(first)

	for i := 1; i < len(buf) && len(result) < 4; i++ {
		c := buf[i]
		code := soundexCode(c)
		if code == '0' {
			// H and W do not separate same-coded letters
			if c == 'H' || c == 'W' {
				continue
			}
			prev = code
			continue
		}
		if code != prev {
			result = append(result, code)
			prev = code
		}
	}

	// Pad to 4 characters with '0'.
	for len(result) < 4 {
		result = append(result, '0')
	}
	return string(result)
}

// RefinedSoundex implements the Refined Soundex phonetic encoding algorithm.
//
// This is a Go port of
// org.apache.commons.codec.language.RefinedSoundex from
// Apache Commons Codec 1.17.2.
type RefinedSoundex struct{}

// NewRefinedSoundex creates a new RefinedSoundex encoder.
func NewRefinedSoundex() *RefinedSoundex {
	return &RefinedSoundex{}
}

// refinedSoundexMapping is the US English Refined Soundex mapping string from
// Apache Commons Codec 1.17.2. Each position corresponds to A–Z.
// Value '0' represents a non-coded separator letter (vowels, H, W, Y).
const refinedSoundexMapping = "01360240043788015936020505"

// refinedSoundexCode returns the Refined Soundex code digit for an upper-case
// letter, or '0' if the letter is a separator.
func refinedSoundexCode(c byte) byte {
	if c < 'A' || c > 'Z' {
		return '0'
	}
	return refinedSoundexMapping[c-'A']
}

// Encode encodes a string to its Refined Soundex code.
func (rs *RefinedSoundex) Encode(value string) string {
	if value == "" {
		return ""
	}
	upper := strings.ToUpper(value)

	var result []byte
	var prev byte = 0
	first := true

	for i := 0; i < len(upper); i++ {
		c := upper[i]
		if c < 'A' || c > 'Z' {
			continue
		}
		code := refinedSoundexCode(c)
		if first {
			// First character: always add the letter itself.
			result = append(result, c)
			prev = 0 // reset prev so next code is always appended
			first = false
		} else {
			// Suppress duplicate adjacent codes.
			if code != prev {
				result = append(result, code)
				prev = code
			}
		}
	}
	return string(result)
}

// Caverphone2 implements the Caverphone 2.0 phonetic encoding algorithm.
//
// This is a Go port of
// org.apache.commons.codec.language.Caverphone2 from
// Apache Commons Codec 1.17.2.
type Caverphone2 struct{}

// NewCaverphone2 creates a new Caverphone2 encoder.
func NewCaverphone2() *Caverphone2 {
	return &Caverphone2{}
}

// Encode encodes a string using the Caverphone 2.0 algorithm.
// Returns a 10-character code.
func (c *Caverphone2) Encode(value string) string {
	return caverphone2Encode(value)
}

func caverphone2Encode(str string) string {
	if str == "" {
		return "1111111111"
	}

	// Lowercase, strip non-alpha
	var buf strings.Builder
	for _, r := range str {
		if r >= 'a' && r <= 'z' {
			buf.WriteRune(r)
		} else if r >= 'A' && r <= 'Z' {
			buf.WriteRune(r + 32)
		}
	}
	s := buf.String()
	if s == "" {
		return "1111111111"
	}

	// Step 1: Initial transformations
	// Remove trailing e
	if strings.HasSuffix(s, "e") {
		s = s[:len(s)-1]
	}

	s = repl(s, "cq", "2q")
	s = repl(s, "ci", "si")
	s = repl(s, "ce", "se")
	s = repl(s, "cy", "sy")
	s = repl(s, "tch", "2ch")
	s = repl(s, "c", "k")
	s = repl(s, "q", "k")
	s = repl(s, "x", "k")
	s = repl(s, "v", "f")
	s = repl(s, "dg", "2g")
	s = repl(s, "tio", "sio")
	s = repl(s, "tia", "sia")
	s = repl(s, "d", "t")
	s = repl(s, "ph", "fh")
	s = repl(s, "b", "p")
	s = repl(s, "sh", "s2")
	s = repl(s, "z", "s")
	if len(s) > 0 && (s[0] == 'a' || s[0] == 'e' || s[0] == 'i' || s[0] == 'o' || s[0] == 'u') {
		s = "A" + s[1:]
	}
	s = repl(s, "a", "3")
	s = repl(s, "e", "3")
	s = repl(s, "i", "3")
	s = repl(s, "o", "3")
	s = repl(s, "u", "3")
	s = repl(s, "j", "y")
	if strings.HasPrefix(s, "y3") {
		s = "Y3" + s[2:]
	}
	if strings.HasPrefix(s, "y") {
		s = "A" + s[1:]
	}
	s = repl(s, "y", "3")
	s = repl(s, "3gh3", "3kh3")
	s = repl(s, "gh", "22")
	s = repl(s, "g", "k")
	s = replRunOf2(s, "s")
	s = replRunOf2(s, "t")
	s = replRunOf2(s, "p")
	s = replRunOf2(s, "k")
	s = replRunOf2(s, "f")
	s = replRunOf2(s, "m")
	s = replRunOf2(s, "n")
	s = repl(s, "w3", "W3")
	s = repl(s, "wy", "Wy")
	s = repl(s, "wh3", "Wh3")
	s = repl(s, "why", "Why")
	s = repl(s, "w", "2")
	if strings.HasPrefix(s, "h") {
		s = "A" + s[1:]
	}
	s = repl(s, "h", "2")
	s = repl(s, "r3", "R3")
	s = repl(s, "ry", "Ry")
	s = repl(s, "r", "2")
	s = repl(s, "l3", "L3")
	s = repl(s, "ly", "Ly")
	s = repl(s, "l", "2")
	s = repl(s, "j", "y")
	s = repl(s, "y3", "Y3")
	s = repl(s, "y", "2")
	// remove 2s and 3s
	s = strings.Map(func(r rune) rune {
		if r == '2' || r == '3' {
			return -1
		}
		return r
	}, s)
	// pad to 10 chars with 1s
	for len(s) < 10 {
		s += "1"
	}
	return strings.ToUpper(s[:10])
}

func repl(s, old, newStr string) string {
	return strings.ReplaceAll(s, old, newStr)
}

func replRunOf2(s, c string) string {
	return strings.ReplaceAll(s, c+c, c)
}

// ColognePhonetic implements the Cologne Phonetic (Kölner Phonetik) algorithm.
//
// This is a Go port of
// org.apache.commons.codec.language.ColognePhonetic from
// Apache Commons Codec 1.17.2.
type ColognePhonetic struct{}

// NewColognePhonetic creates a new ColognePhonetic encoder.
func NewColognePhonetic() *ColognePhonetic {
	return &ColognePhonetic{}
}

// Encode encodes a string using Cologne Phonetic.
func (cp *ColognePhonetic) Encode(value string) string {
	return colognePhoneticEncode(value)
}

// cologneCode returns the Cologne Phonetic code for a character given
// its predecessor.
func cologneCodeFor(c, prev, next byte) byte {
	switch c {
	case 'A', 'E', 'I', 'J', 'O', 'U', 'Y':
		return '0'
	case 'H':
		return '?'
	case 'B':
		return '1'
	case 'P':
		if next == 'H' {
			return '3'
		}
		return '1'
	case 'D', 'T':
		if next == 'C' || next == 'S' || next == 'Z' {
			return '8'
		}
		return '2'
	case 'F', 'V', 'W':
		return '3'
	case 'G', 'K', 'Q':
		return '4'
	case 'C':
		if prev == 'S' || prev == 'Z' {
			return '8'
		}
		if c == 'C' {
			if next == 'A' || next == 'H' || next == 'K' || next == 'L' ||
				next == 'O' || next == 'Q' || next == 'R' || next == 'U' || next == 'X' {
				return '4'
			}
		}
		return '8'
	case 'X':
		if prev == 'C' || prev == 'K' || prev == 'Q' {
			return '8'
		}
		return '4' // represents KS
	case 'L':
		return '5'
	case 'M', 'N':
		return '6'
	case 'R':
		return '7'
	case 'S', 'Z':
		return '8'
	}
	return '?'
}

func colognePhoneticEncode(str string) string {
	if str == "" {
		return ""
	}
	upper := strings.ToUpper(str)
	// Preprocess: replace Ä→A, Ö→O, Ü→U, ß→S
	upper = strings.NewReplacer(
		"Ä", "A", "Ö", "O", "Ü", "U", "ß", "S",
		"À", "A", "Á", "A", "Â", "A",
		"È", "E", "É", "E", "Ê", "E",
		"Ì", "I", "Í", "I", "Î", "I",
		"Ò", "O", "Ó", "O", "Ô", "O",
		"Ù", "U", "Ú", "U", "Û", "U",
	).Replace(upper)

	var letters []byte
	for i := 0; i < len(upper); i++ {
		c := upper[i]
		if c >= 'A' && c <= 'Z' {
			letters = append(letters, c)
		}
	}
	if len(letters) == 0 {
		return ""
	}

	var codes []byte
	for i, c := range letters {
		var prev, next byte
		if i > 0 {
			prev = letters[i-1]
		}
		if i+1 < len(letters) {
			next = letters[i+1]
		}
		code := cologneCodeFor(c, prev, next)
		if code == '?' {
			continue
		}
		if len(codes) == 0 {
			codes = append(codes, code)
		} else if code != codes[len(codes)-1] {
			codes = append(codes, code)
		}
	}

	// Remove leading '0'
	if len(codes) > 0 && codes[0] == '0' {
		codes = codes[1:]
	}
	return string(codes)
}

// Nysiis implements the NYSIIS (New York State Identification and Intelligence
// System) phonetic encoding algorithm.
//
// This is a Go port of
// org.apache.commons.codec.language.Nysiis from
// Apache Commons Codec 1.17.2.
type Nysiis struct {
	// Strict mode (true = 6 character limit, false = no limit)
	Strict bool
}

// NewNysiis creates a new Nysiis encoder in non-strict mode.
func NewNysiis() *Nysiis {
	return &Nysiis{}
}

// Encode encodes a string using the NYSIIS algorithm.
func (n *Nysiis) Encode(value string) string {
	return nysiisEncode(value, n.Strict)
}

func nysiisEncode(str string, strict bool) string {
	if str == "" {
		return ""
	}
	upper := strings.ToUpper(str)
	var letters []byte
	for i := 0; i < len(upper); i++ {
		c := upper[i]
		if c >= 'A' && c <= 'Z' {
			letters = append(letters, c)
		}
	}
	if len(letters) == 0 {
		return ""
	}
	s := string(letters)

	// Initial replacements.
	s = strings.TrimSuffix(s, "S")
	// prefix transformations
	for _, old := range [][]string{
		{"MAC", "MCC"},
		{"KN", "N"},
		{"K", "C"},
		{"PH", "FF"},
		{"PF", "FF"},
		{"SCH", "SSS"},
	} {
		if strings.HasPrefix(s, old[0]) {
			s = old[1] + s[len(old[0]):]
			break
		}
	}
	// suffix transformations
	for _, sfx := range [][]string{
		{"EE", "Y"},
		{"IE", "Y"},
		{"DT", "D"},
		{"RT", "D"},
		{"RD", "D"},
		{"NT", "N"},
		{"ND", "N"},
	} {
		if strings.HasSuffix(s, sfx[0]) {
			s = s[:len(s)-len(sfx[0])] + sfx[1]
			break
		}
	}

	// Build result.
	first := string(s[0])
	if len(s) > 1 {
		s = s[1:]
	} else {
		s = ""
	}

	// Internal replacements.
	s = strings.ReplaceAll(s, "EV", "AF")
	for _, c := range "AEIOU" {
		s = strings.ReplaceAll(s, string(c), "A")
	}
	s = strings.ReplaceAll(s, "Q", "G")
	s = strings.ReplaceAll(s, "Z", "S")
	s = strings.ReplaceAll(s, "M", "N")
	s = strings.ReplaceAll(s, "KN", "N")
	s = strings.ReplaceAll(s, "K", "C")
	s = strings.ReplaceAll(s, "SCH", "SSS")
	s = strings.ReplaceAll(s, "PH", "FF")
	// replace terminal E
	if strings.HasSuffix(s, "AY") {
		s = s[:len(s)-2] + "Y"
	}
	if strings.HasSuffix(s, "A") && len(s) > 0 {
		s = s[:len(s)-1]
	}

	// Remove duplicate adjacent characters.
	result := first
	prev := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != prev {
			result += string(c)
			prev = c
		}
	}

	if strict && len(result) > 6 {
		return result[:6]
	}
	return result
}
