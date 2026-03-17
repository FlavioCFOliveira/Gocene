// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import "unicode"

// WordDelimiterIterator provides a BreakIterator-like API for iterating
// over subwords in text, according to WordDelimiterGraphFilter rules.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.miscellaneous.WordDelimiterIterator.
//
// The iterator identifies word boundaries based on:
//   - Case changes (camelCase, PascalCase)
//   - Delimiters (hyphens, underscores, etc.)
//   - Number-to-letter transitions
//
// Example: "PowerShot12Mpx" -> "Power", "Shot", "12", "Mpx"
type WordDelimiterIterator struct {
	// text is the character array being iterated
	text []rune

	// length is the length of the text
	length int

	// startBounds is the start position of text, excluding leading delimiters
	startBounds int

	// endBounds is the end position of text, excluding trailing delimiters
	endBounds int

	// current is the beginning of the current subword
	current int

	// end is the end of the current subword
	end int

	// hasFinalPossessive indicates if the string ends with a possessive such as 's
	hasFinalPossessive bool

	// splitOnCaseChange determines if case changes cause splits
	// If false, causes case changes to be ignored (subwords will only be generated
	// given SUBWORD_DELIM tokens). Defaults to true.
	splitOnCaseChange bool

	// splitOnNumerics determines if numeric transitions cause splits
	// If false, causes numeric changes to be ignored (subwords will only be generated
	// given SUBWORD_DELIM tokens). Defaults to true.
	splitOnNumerics bool

	// stemEnglishPossessive causes trailing "'s" to be removed for each subword
	// "O'Neil's" => "O", "Neil". Defaults to true.
	stemEnglishPossessive bool

	// charTypeTable maps characters to their types
	charTypeTable []byte

	// skipPossessive indicates if we need to skip over a possessive found in the last call to next()
	skipPossessive bool
}

// Word delimiter type constants (bitmask flags)
const (
	// LOWER indicates a lowercase letter
	LOWER = 0x01
	// UPPER indicates an uppercase letter
	UPPER = 0x02
	// DIGIT indicates a digit
	DIGIT = 0x04
	// SUBWORD_DELIM indicates a subword delimiter
	SUBWORD_DELIM = 0x08

	// ALPHA is a combination of LOWER and UPPER (for testing, not for setting bits)
	ALPHA = LOWER | UPPER
	// ALPHANUM is a combination of ALPHA and DIGIT (for testing, not for setting bits)
	ALPHANUM = ALPHA | DIGIT

	// DONE indicates the end of iteration
	DONE = -1
)

// DEFAULT_WORD_DELIM_TABLE is the default character type table for ASCII characters.
// It is initialized in init() and maps ASCII characters to their types.
var DEFAULT_WORD_DELIM_TABLE []byte

func init() {
	// Initialize the default word delimiter table for ASCII characters
	tab := make([]byte, 256)
	for i := 0; i < 256; i++ {
		code := byte(0)
		r := rune(i)
		if unicode.IsLower(r) {
			code |= LOWER
		} else if unicode.IsUpper(r) {
			code |= UPPER
		} else if unicode.IsDigit(r) {
			code |= DIGIT
		}
		if code == 0 {
			code = SUBWORD_DELIM
		}
		tab[i] = code
	}
	DEFAULT_WORD_DELIM_TABLE = tab
}

// NewWordDelimiterIterator creates a new WordDelimiterIterator with the default character type table.
//
// Parameters:
//   - splitOnCaseChange: if true, causes "PowerShot" to be two tokens ("Power-Shot" remains two parts regardless)
//   - splitOnNumerics: if true, causes "j2se" to be three tokens: "j", "2", "se"
//   - stemEnglishPossessive: if true, causes trailing "'s" to be removed for each subword: "O'Neil's" => "O", "Neil"
func NewWordDelimiterIterator(splitOnCaseChange, splitOnNumerics, stemEnglishPossessive bool) *WordDelimiterIterator {
	return NewWordDelimiterIteratorWithTable(
		DEFAULT_WORD_DELIM_TABLE,
		splitOnCaseChange,
		splitOnNumerics,
		stemEnglishPossessive,
	)
}

// NewWordDelimiterIteratorWithTable creates a new WordDelimiterIterator with a custom character type table.
//
// Parameters:
//   - charTypeTable: table containing character types (should be at least 256 bytes for ASCII)
//   - splitOnCaseChange: if true, causes "PowerShot" to be two tokens
//   - splitOnNumerics: if true, causes "j2se" to be three tokens
//   - stemEnglishPossessive: if true, causes trailing "'s" to be removed
func NewWordDelimiterIteratorWithTable(charTypeTable []byte, splitOnCaseChange, splitOnNumerics, stemEnglishPossessive bool) *WordDelimiterIterator {
	return &WordDelimiterIterator{
		charTypeTable:         charTypeTable,
		splitOnCaseChange:     splitOnCaseChange,
		splitOnNumerics:       splitOnNumerics,
		stemEnglishPossessive: stemEnglishPossessive,
		end:                   DONE,
	}
}

// SetText resets the iterator with new text.
// This resets all state and prepares the iterator for a new input.
//
// Parameters:
//   - text: the new text to iterate over
//   - length: the length of the text (can be less than len(text) to use only a portion)
func (w *WordDelimiterIterator) SetText(text []rune, length int) {
	w.text = text
	w.length = length
	w.endBounds = length
	w.current = 0
	w.startBounds = 0
	w.end = 0
	w.skipPossessive = false
	w.hasFinalPossessive = false
	w.setBounds()
}

// Next advances to the next subword and returns the end position.
// Returns DONE if all subwords have been returned.
func (w *WordDelimiterIterator) Next() int {
	w.current = w.end
	if w.current == DONE {
		return DONE
	}

	if w.skipPossessive {
		w.current += 2
		w.skipPossessive = false
	}

	var lastType int

	// Skip leading delimiters
	for w.current < w.endBounds {
		lastType = w.charType(w.text[w.current])
		if !IsSubwordDelim(lastType) {
			break
		}
		w.current++
	}

	if w.current >= w.endBounds {
		w.end = DONE
		return DONE
	}

	// Find the end of the current subword
	for w.end = w.current + 1; w.end < w.endBounds; w.end++ {
		type_ := w.charType(w.text[w.end])
		if w.isBreak(lastType, type_) {
			break
		}
		lastType = type_
	}

	// Check if we need to skip a possessive in the next iteration
	if w.end < w.endBounds-1 && w.endsWithPossessive(w.end+2) {
		w.skipPossessive = true
	}

	return w.end
}

// Current returns the start position of the current subword.
func (w *WordDelimiterIterator) Current() int {
	return w.current
}

// End returns the end position of the current subword.
// Returns DONE if iteration is complete.
func (w *WordDelimiterIterator) End() int {
	return w.end
}

// Type returns the type of the current subword.
// This uses the type of the first character in the subword.
// Returns 0 if iteration is complete.
func (w *WordDelimiterIterator) Type() int {
	if w.end == DONE {
		return 0
	}

	type_ := w.charType(w.text[w.current])
	switch type_ {
	case LOWER, UPPER:
		return ALPHA
	default:
		return type_
	}
}

// IsSingleWord returns true if the current word contains only one subword.
// Note: it could be potentially surrounded by delimiters.
func (w *WordDelimiterIterator) IsSingleWord() bool {
	if w.hasFinalPossessive {
		return w.current == w.startBounds && w.end == w.endBounds-2
	}
	return w.current == w.startBounds && w.end == w.endBounds
}

// GetText returns the text being iterated.
func (w *WordDelimiterIterator) GetText() []rune {
	return w.text
}

// GetStartBounds returns the start position of text, excluding leading delimiters.
func (w *WordDelimiterIterator) GetStartBounds() int {
	return w.startBounds
}

// GetEndBounds returns the end position of text, excluding trailing delimiters.
func (w *WordDelimiterIterator) GetEndBounds() int {
	return w.endBounds
}

// GetCurrentSubword returns the current subword as a string.
// Returns empty string if iteration is complete.
func (w *WordDelimiterIterator) GetCurrentSubword() string {
	if w.end == DONE || w.current >= w.end {
		return ""
	}
	return string(w.text[w.current:w.end])
}

// isBreak determines whether the transition from lastType to type indicates a break.
func (w *WordDelimiterIterator) isBreak(lastType, type_ int) bool {
	// If the types share any bits, it's not a break
	if (type_ & lastType) != 0 {
		return false
	}

	if !w.splitOnCaseChange && IsAlpha(lastType) && IsAlpha(type_) {
		// ALPHA->ALPHA: always ignore if case isn't considered
		return false
	} else if lastType == UPPER && IsAlpha(type_) {
		// UPPER->letter: Don't split (handles cases like "URL" followed by lowercase)
		// Only applies when lastType is strictly UPPER (not ALPHA which includes UPPER)
		return false
	} else if !w.splitOnNumerics &&
		((IsAlpha(lastType) && IsDigit(type_)) || (IsDigit(lastType) && IsAlpha(type_))) {
		// ALPHA->NUMERIC, NUMERIC->ALPHA: Don't split
		return false
	}

	return true
}

// setBounds sets the internal word bounds (removes leading and trailing delimiters).
// If a possessive is found, don't remove it yet, simply note it.
func (w *WordDelimiterIterator) setBounds() {
	// Skip leading delimiters
	for w.startBounds < w.length && IsSubwordDelim(w.charType(w.text[w.startBounds])) {
		w.startBounds++
	}

	// Skip trailing delimiters
	for w.endBounds > w.startBounds && IsSubwordDelim(w.charType(w.text[w.endBounds-1])) {
		w.endBounds--
	}

	// Check for possessive
	if w.endsWithPossessive(w.endBounds) {
		w.hasFinalPossessive = true
	}

	w.current = w.startBounds
}

// endsWithPossessive determines if the text at the given position indicates
// an English possessive which should be removed.
func (w *WordDelimiterIterator) endsWithPossessive(pos int) bool {
	if !w.stemEnglishPossessive || pos <= 2 {
		return false
	}

	// Check for "'s" or "'S" pattern with an alphabetic character before it
	if w.text[pos-2] == '\'' && (w.text[pos-1] == 's' || w.text[pos-1] == 'S') {
		// Must have an alphabetic character before the apostrophe
		if pos >= 3 && IsAlpha(w.charType(w.text[pos-3])) {
			// And either it's at the end, or followed by a delimiter
			if pos == w.endBounds || IsSubwordDelim(w.charType(w.text[pos])) {
				return true
			}
		}
	}
	return false
}

// charType returns the type of the given character.
func (w *WordDelimiterIterator) charType(ch rune) int {
	if int(ch) < len(w.charTypeTable) {
		return int(w.charTypeTable[ch])
	}
	return int(GetWordDelimiterType(ch))
}

// GetWordDelimiterType computes the type of the given character for characters
// outside the ASCII range.
func GetWordDelimiterType(ch rune) byte {
	// For ASCII range, use the default table
	if ch < 256 {
		return DEFAULT_WORD_DELIM_TABLE[ch]
	}

	// For non-ASCII, categorize based on Unicode properties
	// Check for uppercase ASCII (already handled above, but for completeness)
	if ch >= 'A' && ch <= 'Z' {
		return UPPER
	}
	// Check for lowercase ASCII
	if ch >= 'a' && ch <= 'z' {
		return LOWER
	}
	// Check for ASCII digits
	if ch >= '0' && ch <= '9' {
		return DIGIT
	}

	// For Unicode characters, use unicode package functions
	if unicode.IsUpper(ch) {
		return UPPER
	}
	if unicode.IsLower(ch) {
		return LOWER
	}
	if unicode.IsDigit(ch) {
		return DIGIT
	}
	// Check for letter-like characters (titlecase, modifier, other letters, marks)
	if unicode.IsLetter(ch) || unicode.IsMark(ch) {
		return ALPHA
	}
	// Check for number-like characters
	if unicode.IsNumber(ch) {
		return DIGIT
	}
	// Surrogate check (Go doesn't have surrogates like Java, but we handle the range)
	if ch >= 0xD800 && ch <= 0xDFFF {
		return ALPHA | DIGIT
	}
	// Everything else is a delimiter
	return SUBWORD_DELIM
}

// IsAlpha checks if the given word type includes ALPHA.
func IsAlpha(type_ int) bool {
	return (type_ & ALPHA) != 0
}

// IsDigit checks if the given word type includes DIGIT.
func IsDigit(type_ int) bool {
	return (type_ & DIGIT) != 0
}

// IsSubwordDelim checks if the given word type includes SUBWORD_DELIM.
func IsSubwordDelim(type_ int) bool {
	return (type_ & SUBWORD_DELIM) != 0
}

// IsUpper checks if the given word type includes UPPER.
func IsUpper(type_ int) bool {
	return (type_ & UPPER) != 0
}

// IsLower checks if the given word type includes LOWER.
func IsLower(type_ int) bool {
	return (type_ & LOWER) != 0
}

// IsAlphanum checks if the given word type includes ALPHANUM.
func IsAlphanum(type_ int) bool {
	return (type_ & ALPHANUM) != 0
}
