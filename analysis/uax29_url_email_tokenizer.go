// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"bufio"
	"io"
	"regexp"
	"strings"
	"unicode"
)

// UAX29URLEmailTokenizer is a tokenizer that implements UAX#29 word boundary rules
// with special handling for URLs and email addresses.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.standard.UAX29URLEmailTokenizer.
//
// The tokenizer follows Unicode Text Segmentation rules (UAX #29) for word boundaries,
// but recognizes and preserves URLs and email addresses as single tokens.
//
// URL recognition follows RFC 3986 and common web URL patterns including:
//   - http://, https://, ftp:// schemes
//   - Domain names with TLDs
//   - IP addresses
//   - Port numbers
//   - Paths, query strings, and fragments
//
// Email recognition follows RFC 5322 simplified patterns including:
//   - Local part with allowed characters
//   - @ symbol
//   - Domain part with TLD
//
// Example:
//
//	Input: "Visit https://example.com or contact user@example.com for info"
//	Output tokens: "Visit", "https://example.com", "or", "contact", "user@example.com", "for", "info"
//
//	Input: "Check out http://192.168.1.1:8080/path?query=1#frag"
//	Output tokens: "Check", "out", "http://192.168.1.1:8080/path?query=1#frag"
type UAX29URLEmailTokenizer struct {
	*BaseTokenizer

	// scanner reads from the input
	scanner *bufio.Scanner

	// termAttr holds the CharTermAttribute
	termAttr CharTermAttribute

	// offsetAttr holds the OffsetAttribute
	offsetAttr OffsetAttribute

	// posIncrAttr holds the PositionIncrementAttribute
	posIncrAttr PositionIncrementAttribute

	// currentOffset tracks the current position in input
	currentOffset int

	// currentToken holds the current token being built
	currentToken []rune

	// tokenStartOffset holds the start offset of current token
	tokenStartOffset int

	// maxTokenLength is the maximum token length
	maxTokenLength int

	// inputBuffer holds the current input for lookahead
	inputBuffer []rune

	// bufferPosition is the current position in the input buffer
	bufferPosition int
}

// Default maximum token length (same as Lucene's default)
const DefaultMaxTokenLength = 255

// URL pattern matching various URL formats
// This pattern recognizes:
// - Scheme (http, https, ftp, etc.)
// - Domain names, IP addresses
// - Optional port
// - Optional path, query, fragment
var urlPattern = regexp.MustCompile(`^(?i)([a-z][a-z0-9+.-]*://[^\s<>"{}|\^\[\]` + "`" + `]+)`)

// Email pattern matching RFC 5322 simplified format
// Matches: local-part@domain.tld
var emailPattern = regexp.MustCompile(`^(?i)([a-z0-9!#$%&'*+/=?^_` + "`" + `{|}~-]+(?:\.[a-z0-9!#$%&'*+/=?^_` + "`" + `{|}~-]+)*@[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)*)`)

// NewUAX29URLEmailTokenizer creates a new UAX29URLEmailTokenizer with default settings.
func NewUAX29URLEmailTokenizer() *UAX29URLEmailTokenizer {
	return NewUAX29URLEmailTokenizerWithMaxTokenLength(DefaultMaxTokenLength)
}

// NewUAX29URLEmailTokenizerWithMaxTokenLength creates a new UAX29URLEmailTokenizer
// with the specified maximum token length.
func NewUAX29URLEmailTokenizerWithMaxTokenLength(maxTokenLength int) *UAX29URLEmailTokenizer {
	t := &UAX29URLEmailTokenizer{
		BaseTokenizer:    NewBaseTokenizer(),
		maxTokenLength:   maxTokenLength,
		currentToken:     make([]rune, 0, 256),
		inputBuffer:      make([]rune, 0, 4096),
		bufferPosition:   0,
		currentOffset:    0,
		tokenStartOffset: 0,
	}

	// Add attributes
	t.termAttr = NewCharTermAttribute()
	t.offsetAttr = NewOffsetAttribute()
	t.posIncrAttr = NewPositionIncrementAttribute()

	t.AddAttribute(t.termAttr)
	t.AddAttribute(t.offsetAttr)
	t.AddAttribute(t.posIncrAttr)

	return t
}

// SetReader sets the input source for this Tokenizer.
func (t *UAX29URLEmailTokenizer) SetReader(input io.Reader) error {
	t.BaseTokenizer.SetReader(input)
	t.scanner = bufio.NewScanner(input)
	t.scanner.Split(bufio.ScanRunes)
	t.currentOffset = 0
	t.bufferPosition = 0
	t.inputBuffer = t.inputBuffer[:0]
	t.currentToken = t.currentToken[:0]
	t.tokenStartOffset = 0

	// Pre-read all input for lookahead support
	for t.scanner.Scan() {
		r := []rune(t.scanner.Text())
		if len(r) > 0 {
			t.inputBuffer = append(t.inputBuffer, r[0])
		}
	}

	return t.scanner.Err()
}

// IncrementToken advances to the next token.
func (t *UAX29URLEmailTokenizer) IncrementToken() (bool, error) {
	if t.input == nil {
		return false, nil
	}

	// Clear attributes for new token
	t.ClearAttributes()
	t.currentToken = t.currentToken[:0]

	// Skip leading non-word characters
	for t.bufferPosition < len(t.inputBuffer) && !t.isWordChar(t.inputBuffer[t.bufferPosition]) {
		t.bufferPosition++
		t.currentOffset++
	}

	// Check if we've reached the end
	if t.bufferPosition >= len(t.inputBuffer) {
		return false, nil
	}

	t.tokenStartOffset = t.currentOffset

	// Try to match URL first (higher priority)
	if urlMatch := t.tryMatchURL(); urlMatch != "" {
		t.emitToken(urlMatch, t.tokenStartOffset, t.currentOffset)
		return true, nil
	}

	// Try to match email
	if emailMatch := t.tryMatchEmail(); emailMatch != "" {
		t.emitToken(emailMatch, t.tokenStartOffset, t.currentOffset)
		return true, nil
	}

	// Fall back to UAX#29 word tokenization
	return t.tokenizeWord()
}

// tryMatchURL attempts to match a URL at the current position.
// Returns the matched URL string or empty string if no match.
func (t *UAX29URLEmailTokenizer) tryMatchURL() string {
	if t.bufferPosition >= len(t.inputBuffer) {
		return ""
	}

	// Check if we start with a scheme
	remaining := string(t.inputBuffer[t.bufferPosition:])

	// Quick check for common schemes - need at least "x://"
	if len(remaining) < 4 {
		return ""
	}

	// Check for URL scheme pattern: letters followed by "://"
	// Common schemes: http, https, ftp, file, etc.
	isURL := false
	lowerRemaining := strings.ToLower(remaining)

	// Check for known schemes
	if strings.HasPrefix(lowerRemaining, "http://") ||
		strings.HasPrefix(lowerRemaining, "https://") ||
		strings.HasPrefix(lowerRemaining, "ftp://") ||
		strings.HasPrefix(lowerRemaining, "file://") {
		isURL = true
	} else {
		// Check for generic scheme pattern: [a-z][a-z0-9+.-]*://
		schemeEnd := strings.Index(remaining, "://")
		if schemeEnd > 0 && schemeEnd <= 20 {
			// Validate scheme characters
			validScheme := true
			for i := 0; i < schemeEnd; i++ {
				c := lowerRemaining[i]
				if i == 0 {
					if !((c >= 'a' && c <= 'z')) {
						validScheme = false
						break
					}
				} else {
					if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '+' || c == '.' || c == '-') {
						validScheme = false
						break
					}
				}
			}
			if validScheme {
				isURL = true
			}
		}
	}

	if !isURL {
		return ""
	}

	// Use regex to match URL
	loc := urlPattern.FindStringIndex(remaining)
	if loc == nil {
		return ""
	}

	url := remaining[loc[0]:loc[1]]

	// Check token length limit
	if len(url) > t.maxTokenLength {
		url = url[:t.maxTokenLength]
	}

	// Update positions
	urlRunes := []rune(url)
	t.bufferPosition += len(urlRunes)
	t.currentOffset += len(urlRunes)

	return url
}

// tryMatchEmail attempts to match an email address at the current position.
// Returns the matched email string or empty string if no match.
func (t *UAX29URLEmailTokenizer) tryMatchEmail() string {
	if t.bufferPosition >= len(t.inputBuffer) {
		return ""
	}

	remaining := string(t.inputBuffer[t.bufferPosition:])

	// Quick check: must have @ symbol
	hasAt := false
	for _, r := range remaining {
		if r == '@' {
			hasAt = true
			break
		}
		if unicode.IsSpace(r) {
			break
		}
	}
	if !hasAt {
		return ""
	}

	// Use regex to match email
	loc := emailPattern.FindStringIndex(remaining)
	if loc == nil {
		return ""
	}

	email := remaining[loc[0]:loc[1]]

	// Validate: must contain @ and have something after it
	atIndex := -1
	for i, r := range email {
		if r == '@' {
			atIndex = i
			break
		}
	}
	if atIndex <= 0 || atIndex >= len(email)-1 {
		return ""
	}

	// Check token length limit
	if len(email) > t.maxTokenLength {
		email = email[:t.maxTokenLength]
	}

	// Update positions
	emailRunes := []rune(email)
	t.bufferPosition += len(emailRunes)
	t.currentOffset += len(emailRunes)

	return email
}

// tokenizeWord tokenizes a regular word following UAX#29 rules.
func (t *UAX29URLEmailTokenizer) tokenizeWord() (bool, error) {
	if t.bufferPosition >= len(t.inputBuffer) {
		return false, nil
	}

	startPos := t.bufferPosition
	startOffset := t.currentOffset

	// UAX#29 Word Boundary Rules (simplified implementation):
// WB1: sot ÷ (start of text always breaks)
// WB2: ÷ eot (end of text always breaks)
// WB3: CR × LF (carriage return doesn't break before line feed)
// WB3a: (CR | LF | NL) ÷
// WB3b: ÷ (CR | LF | NL)
// WB4: X (Extend | Format)* → X
// WB5: (ALetter | Hebrew_Letter) × (ALetter | Hebrew_Letter)
// WB6: (ALetter | Hebrew_Letter) × (MidLetter | MidNumLet | Single_Quote) (ALetter | Hebrew_Letter)
// WB7: (ALetter | Hebrew_Letter) (MidLetter | MidNumLet | Single_Quote) × (ALetter | Hebrew_Letter)
// WB7a: Hebrew_Letter × Single_Quote
// WB7b: Hebrew_Letter × Double_Quote Hebrew_Letter
// WB7c: Hebrew_Letter Double_Quote × Hebrew_Letter
// WB8: Numeric × Numeric
// WB9: (ALetter | Hebrew_Letter) × Numeric
// WB10: Numeric × (ALetter | Hebrew_Letter)
// WB11: Numeric (MidNum | MidNumLet) × Numeric
// WB12: Numeric × (MidNum | MidNumLet) Numeric
// WB13: Katakana × Katakana
// WB13a: (ALetter | Hebrew_Letter | Numeric | Katakana | ExtendNumLet) × ExtendNumLet
// WB13b: ExtendNumLet × (ALetter | Hebrew_Letter | Numeric | Katakana)
// WB14: Any ÷ Any

	// Build the token
	for t.bufferPosition < len(t.inputBuffer) {
		r := t.inputBuffer[t.bufferPosition]

		// Check for word boundary
		if t.isWordBoundary(r) {
			break
		}

		// Check token length limit
		if t.bufferPosition-startPos >= t.maxTokenLength {
			break
		}

		t.currentToken = append(t.currentToken, r)
		t.bufferPosition++
		t.currentOffset++
	}

	if len(t.currentToken) == 0 {
		return false, nil
	}

	t.emitToken(string(t.currentToken), startOffset, t.currentOffset)
	return true, nil
}

// isWordChar checks if a rune is a valid word character according to UAX#29.
func (t *UAX29URLEmailTokenizer) isWordChar(r rune) bool {
	// ALetter: alphabetic letters
	if unicode.IsLetter(r) {
		return true
	}
	// Numeric: decimal numbers
	if unicode.IsDigit(r) {
		return true
	}
	// ExtendNumLet: connector punctuation like underscore
	if r == '_' {
		return true
	}
	// Hebrew letters
	if isHebrewLetter(r) {
		return true
	}
	// Katakana
	if isKatakana(r) {
		return true
	}
	// Extend and Format characters
	if isExtendOrFormat(r) {
		return true
	}
	return false
}

// isWordBoundary checks if a rune indicates a word boundary.
func (t *UAX29URLEmailTokenizer) isWordBoundary(r rune) bool {
	// Whitespace is always a boundary
	if unicode.IsSpace(r) {
		return true
	}
	// Control characters are boundaries
	if unicode.IsControl(r) {
		return true
	}
	// Punctuation (except connector punctuation like underscore)
	if unicode.IsPunct(r) && r != '_' {
		return true
	}
	// Symbols (except those that are part of words)
	if unicode.IsSymbol(r) && r != '_' {
		return true
	}
	return false
}

// isHebrewLetter checks if a rune is a Hebrew letter.
func isHebrewLetter(r rune) bool {
	// Hebrew block: U+0590 to U+05FF
	return r >= 0x0590 && r <= 0x05FF
}

// isKatakana checks if a rune is a Katakana character.
func isKatakana(r rune) bool {
	// Katakana block: U+30A0 to U+30FF
	// Katakana Phonetic Extensions: U+31F0 to U+31FF
	return (r >= 0x30A0 && r <= 0x30FF) || (r >= 0x31F0 && r <= 0x31FF)
}

// isExtendOrFormat checks if a rune is an Extend or Format character.
func isExtendOrFormat(r rune) bool {
	// Extend characters include:
	// - Mn (Mark, Nonspacing)
	// - Mc (Mark, Spacing Combining)
	// - Me (Mark, Enclosing)
	// Format characters include:
	// - Cf (Format)
	// - ZWJ (Zero Width Joiner)
	// - ZWNJ (Zero Width Non-Joiner)

	// CategoryRangeTable is not available in Go's unicode package
	// This is a placeholder for future implementation
	_ = unicode.MaxASCII

	// Check for combining marks
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r) || unicode.Is(unicode.Me, r) {
		return true
	}
	// Check for format characters
	if unicode.Is(unicode.Cf, r) {
		return true
	}
	// Zero width joiner/non-joiner
	if r == 0x200C || r == 0x200D { // ZWNJ, ZWJ
		return true
	}
	return false
}

// emitToken emits a token with the given properties.
func (t *UAX29URLEmailTokenizer) emitToken(text string, startOffset, endOffset int) {
	t.termAttr.SetValue(text)
	t.offsetAttr.SetStartOffset(startOffset)
	t.offsetAttr.SetEndOffset(endOffset)
	t.posIncrAttr.SetPositionIncrement(1)
}

// Reset resets the tokenizer.
func (t *UAX29URLEmailTokenizer) Reset() error {
	t.BaseTokenizer.Reset()
	t.currentOffset = 0
	t.bufferPosition = 0
	t.inputBuffer = t.inputBuffer[:0]
	t.currentToken = t.currentToken[:0]
	t.tokenStartOffset = 0
	return nil
}

// End performs end-of-stream operations.
func (t *UAX29URLEmailTokenizer) End() error {
	if t.offsetAttr != nil {
		t.offsetAttr.SetEndOffset(t.currentOffset)
	}
	return nil
}

// GetMaxTokenLength returns the maximum token length.
func (t *UAX29URLEmailTokenizer) GetMaxTokenLength() int {
	return t.maxTokenLength
}

// SetMaxTokenLength sets the maximum token length.
func (t *UAX29URLEmailTokenizer) SetMaxTokenLength(maxTokenLength int) {
	t.maxTokenLength = maxTokenLength
}

// Ensure UAX29URLEmailTokenizer implements Tokenizer
var _ Tokenizer = (*UAX29URLEmailTokenizer)(nil)

// UAX29URLEmailTokenizerFactory creates UAX29URLEmailTokenizer instances.
type UAX29URLEmailTokenizerFactory struct {
	maxTokenLength int
}

// NewUAX29URLEmailTokenizerFactory creates a new UAX29URLEmailTokenizerFactory.
func NewUAX29URLEmailTokenizerFactory() *UAX29URLEmailTokenizerFactory {
	return &UAX29URLEmailTokenizerFactory{
		maxTokenLength: DefaultMaxTokenLength,
	}
}

// NewUAX29URLEmailTokenizerFactoryWithMaxLength creates a factory with custom max token length.
func NewUAX29URLEmailTokenizerFactoryWithMaxLength(maxTokenLength int) *UAX29URLEmailTokenizerFactory {
	return &UAX29URLEmailTokenizerFactory{
		maxTokenLength: maxTokenLength,
	}
}

// Create creates a new UAX29URLEmailTokenizer.
func (f *UAX29URLEmailTokenizerFactory) Create() Tokenizer {
	return NewUAX29URLEmailTokenizerWithMaxTokenLength(f.maxTokenLength)
}

// Ensure UAX29URLEmailTokenizerFactory implements TokenizerFactory
var _ TokenizerFactory = (*UAX29URLEmailTokenizerFactory)(nil)
