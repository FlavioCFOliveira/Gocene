// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package in provides analysis components for Indian languages.
package in

import "unicode"

// scriptData holds the flag and base code-point for a Unicode block.
type scriptData struct {
	flag      int
	base      rune
	decompSet map[int]bool // set of ch0 offsets that have decompositions
}

// unicodeBlockBase returns the base code-point of the Unicode block containing r,
// along with the registered scriptData, or nil if not a supported Indic script.
func blockOf(r rune) *scriptData {
	block := unicode.RangeTable{}
	_ = block
	// Use range-table lookup via unicode package.
	for _, sd := range scriptTable {
		if r >= sd.base && r < sd.base+0x80 {
			return sd
		}
	}
	return nil
}

var scriptTable []*scriptData

func init() {
	table := []*scriptData{
		{flag: 1, base: 0x0900},   // Devanagari
		{flag: 2, base: 0x0980},   // Bengali
		{flag: 4, base: 0x0A00},   // Gurmukhi
		{flag: 8, base: 0x0A80},   // Gujarati
		{flag: 16, base: 0x0B00},  // Oriya
		{flag: 32, base: 0x0B80},  // Tamil
		{flag: 64, base: 0x0C00},  // Telugu
		{flag: 128, base: 0x0C80}, // Kannada
		{flag: 256, base: 0x0D00}, // Malayalam
	}

	// Build decompSet for each script based on the decompositions table.
	for _, sd := range table {
		sd.decompSet = make(map[int]bool)
		for _, d := range decompositions {
			if d[4]&sd.flag != 0 {
				sd.decompSet[d[0]] = true
			}
		}
	}
	scriptTable = table
}

// decompositions encodes canonical decomposition rules for Indic scripts.
// Each row: {ch1_offset, ch2_offset, ch3_offset_or_neg1, result_offset, script_flags}.
// ch3 == 0xFF means ZWJ (U+200D).
// Ported from IndicNormalizer.java (Apache Lucene 10.4.0).
var decompositions = [][5]int{
	{0x05, 0x3E, 0x45, 0x11, 1 | 8},    // devanagari, gujarati vowel candra O
	{0x05, 0x3E, 0x46, 0x12, 1},         // devanagari short O
	{0x05, 0x3E, 0x47, 0x13, 1 | 8},     // devanagari, gujarati letter O
	{0x05, 0x3E, 0x48, 0x14, 1 | 8},     // devanagari letter AI, gujarati letter AU
	{0x05, 0x3E, -1, 0x06, 1 | 2 | 4 | 8 | 16}, // devanagari, bengali, gurmukhi, gujarati, oriya AA
	{0x05, 0x45, -1, 0x72, 1},           // devanagari letter candra A
	{0x05, 0x45, -1, 0x0D, 8},           // gujarati vowel candra E
	{0x05, 0x46, -1, 0x04, 1},           // devanagari letter short A
	{0x05, 0x47, -1, 0x0F, 8},           // gujarati letter E
	{0x05, 0x48, -1, 0x10, 4 | 8},       // gurmukhi, gujarati letter AI
	{0x05, 0x49, -1, 0x11, 1 | 8},       // devanagari, gujarati vowel candra O
	{0x05, 0x4A, -1, 0x12, 1},           // devanagari short O
	{0x05, 0x4B, -1, 0x13, 1 | 8},       // devanagari, gujarati letter O
	{0x05, 0x4C, -1, 0x14, 1 | 4 | 8},   // devanagari letter AI, gurmukhi letter AU, gujarati letter AU
	{0x06, 0x45, -1, 0x11, 1 | 8},       // devanagari, gujarati vowel candra O
	{0x06, 0x46, -1, 0x12, 1},           // devanagari short O
	{0x06, 0x47, -1, 0x13, 1 | 8},       // devanagari, gujarati letter O
	{0x06, 0x48, -1, 0x14, 1 | 8},       // devanagari letter AI, gujarati letter AU
	{0x07, 0x57, -1, 0x08, 256},         // malayalam letter II
	{0x09, 0x41, -1, 0x0A, 1},           // devanagari letter UU
	{0x09, 0x57, -1, 0x0A, 32 | 256},    // tamil, malayalam letter UU (some styles)
	{0x0E, 0x46, -1, 0x10, 256},         // malayalam letter AI
	{0x0F, 0x45, -1, 0x0D, 1},           // devanagari candra E
	{0x0F, 0x46, -1, 0x0E, 1},           // devanagari short E
	{0x0F, 0x47, -1, 0x10, 1},           // devanagari AI
	{0x0F, 0x57, -1, 0x10, 16},          // oriya AI
	{0x12, 0x3E, -1, 0x13, 256},         // malayalam letter OO
	{0x12, 0x4C, -1, 0x14, 64 | 128},    // telugu, kannada letter AU
	{0x12, 0x55, -1, 0x13, 64},          // telugu letter OO
	{0x12, 0x57, -1, 0x14, 32 | 256},    // tamil, malayalam letter AU
	{0x13, 0x57, -1, 0x14, 16},          // oriya letter AU
	{0x15, 0x3C, -1, 0x58, 1},           // devanagari qa
	{0x16, 0x3C, -1, 0x59, 1 | 4},       // devanagari, gurmukhi khha
	{0x17, 0x3C, -1, 0x5A, 1 | 4},       // devanagari, gurmukhi ghha
	{0x1C, 0x3C, -1, 0x5B, 1 | 4},       // devanagari, gurmukhi za
	{0x21, 0x3C, -1, 0x5C, 1 | 2 | 16},  // devanagari dddha, bengali, oriya rra
	{0x22, 0x3C, -1, 0x5D, 1 | 2 | 16},  // devanagari, bengali, oriya rha
	{0x23, 0x4D, 0xFF, 0x7A, 256},        // malayalam chillu nn
	{0x24, 0x4D, 0xFF, 0x4E, 2},          // bengali khanda ta
	{0x28, 0x3C, -1, 0x29, 1},            // devanagari nnna
	{0x28, 0x4D, 0xFF, 0x7B, 256},        // malayalam chillu n
	{0x2B, 0x3C, -1, 0x5E, 1 | 4},        // devanagari, gurmukhi fa
	{0x2F, 0x3C, -1, 0x5F, 1 | 2},        // devanagari, bengali yya
	{0x2C, 0x41, 0x41, 0x0B, 64},         // telugu letter vocalic R
	{0x30, 0x3C, -1, 0x31, 1},            // devanagari rra
	{0x30, 0x4D, 0xFF, 0x7C, 256},        // malayalam chillu rr
	{0x32, 0x4D, 0xFF, 0x7D, 256},        // malayalam chillu l
	{0x33, 0x3C, -1, 0x34, 1},            // devanagari llla
	{0x33, 0x4D, 0xFF, 0x7E, 256},        // malayalam chillu ll
	{0x35, 0x41, -1, 0x2E, 64},           // telugu letter MA
	{0x3E, 0x45, -1, 0x49, 1 | 8},        // devanagari, gujarati vowel sign candra O
	{0x3E, 0x46, -1, 0x4A, 1},            // devanagari vowel sign short O
	{0x3E, 0x47, -1, 0x4B, 1 | 8},        // devanagari, gujarati vowel sign O
	{0x3E, 0x48, -1, 0x4C, 1 | 8},        // devanagari, gujarati vowel sign AU
	{0x3F, 0x55, -1, 0x40, 128},           // kannada vowel sign II
	{0x41, 0x41, -1, 0x42, 4},             // gurmukhi vowel sign UU (when stacking)
	{0x46, 0x3E, -1, 0x4A, 32 | 256},      // tamil, malayalam vowel sign O
	{0x46, 0x42, 0x55, 0x4B, 128},         // kannada vowel sign OO
	{0x46, 0x42, -1, 0x4A, 128},           // kannada vowel sign O
	{0x46, 0x46, -1, 0x48, 256},           // malayalam vowel sign AI (if reordered twice)
	{0x46, 0x55, -1, 0x47, 64 | 128},      // telugu, kannada vowel sign EE
	{0x46, 0x56, -1, 0x48, 64 | 128},      // telugu, kannada vowel sign AI
	{0x46, 0x57, -1, 0x4C, 32 | 256},      // tamil, malayalam vowel sign AU
	{0x47, 0x3E, -1, 0x4B, 2 | 16 | 32 | 256}, // bengali, oriya vowel sign O, tamil, malayalam vowel sign OO
	{0x47, 0x57, -1, 0x4C, 2 | 16},        // bengali, oriya vowel sign AU
	{0x4A, 0x55, -1, 0x4B, 128},           // kannada vowel sign OO
	{0x72, 0x3F, -1, 0x07, 4},             // gurmukhi letter I
	{0x72, 0x40, -1, 0x08, 4},             // gurmukhi letter II
	{0x72, 0x47, -1, 0x0F, 4},             // gurmukhi letter EE
	{0x73, 0x41, -1, 0x09, 4},             // gurmukhi letter U
	{0x73, 0x42, -1, 0x0A, 4},             // gurmukhi letter UU
	{0x73, 0x4B, -1, 0x13, 4},             // gurmukhi letter OO
}

// IndicNormalizer normalises the Unicode representation of text in Indian languages.
//
// Go port of org.apache.lucene.analysis.in.IndicNormalizer (Apache Lucene 10.4.0).
//
// Follows guidelines from Unicode 5.2, chapter 6, South Asian Scripts I.
type IndicNormalizer struct{}

// NewIndicNormalizer creates a new IndicNormalizer.
func NewIndicNormalizer() *IndicNormalizer { return &IndicNormalizer{} }

// Normalize normalises text[:length] in-place and returns the new length.
func (n *IndicNormalizer) Normalize(text []rune, length int) int {
	for i := 0; i < length; i++ {
		sd := blockOf(text[i])
		if sd == nil {
			continue
		}
		ch := int(text[i] - sd.base)
		if sd.decompSet[ch] {
			length = n.compose(ch, sd, text, i, length)
		}
	}
	return length
}

func (n *IndicNormalizer) compose(ch0 int, sd *scriptData, text []rune, pos, length int) int {
	if pos+1 >= length {
		return length
	}
	sd1 := blockOf(text[pos+1])
	if sd1 != sd {
		return length
	}
	ch1 := int(text[pos+1] - sd.base)

	ch2 := -1
	if pos+2 < length {
		if text[pos+2] == '‍' {
			ch2 = 0xFF
		} else {
			sd2 := blockOf(text[pos+2])
			if sd2 == sd {
				ch2 = int(text[pos+2] - sd.base)
			}
		}
	}

	for _, d := range decompositions {
		if d[0] == ch0 && (d[4]&sd.flag) != 0 {
			if d[1] == ch1 && (d[2] < 0 || d[2] == ch2) {
				text[pos] = sd.base + rune(d[3])
				length = stemmerDelete(text, pos+1, length)
				if d[2] >= 0 {
					length = stemmerDelete(text, pos+1, length)
				}
				return length
			}
		}
	}
	return length
}

// stemmerDelete removes the element at pos from text[:length] and returns
// the new length.
func stemmerDelete(text []rune, pos, length int) int {
	if pos < 0 || pos >= length {
		return length
	}
	copy(text[pos:], text[pos+1:length])
	return length - 1
}
