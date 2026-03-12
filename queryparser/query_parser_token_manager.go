// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package queryparser

import (
	"strings"
	"unicode"
)

// TokenType represents the type of a token in the query string.
type TokenType int

const (
	TokenTypeEOF TokenType = iota
	TokenTypeTerm
	TokenTypeField
	TokenTypeColon
	TokenTypeLParen
	TokenTypeRParen
	TokenTypeLBracket
	TokenTypeRBracket
	TokenTypeLBrace
	TokenTypeRBrace
	TokenTypeAND
	TokenTypeOR
	TokenTypeNOT
	TokenTypeTO
	TokenTypePlus
	TokenTypeMinus
	TokenTypeQuote
	TokenTypeWildcard
	TokenTypeFuzzy
	TokenTypeBoost
	TokenTypeProximity
	TokenTypeNumber
)

// String returns the string representation of the token type.
func (t TokenType) String() string {
	switch t {
	case TokenTypeEOF:
		return "EOF"
	case TokenTypeTerm:
		return "TERM"
	case TokenTypeField:
		return "FIELD"
	case TokenTypeColon:
		return "COLON"
	case TokenTypeLParen:
		return "LPAREN"
	case TokenTypeRParen:
		return "RPAREN"
	case TokenTypeLBracket:
		return "LBRACKET"
	case TokenTypeRBracket:
		return "RBRACKET"
	case TokenTypeLBrace:
		return "LBRACE"
	case TokenTypeRBrace:
		return "RBRACE"
	case TokenTypeAND:
		return "AND"
	case TokenTypeOR:
		return "OR"
	case TokenTypeNOT:
		return "NOT"
	case TokenTypeTO:
		return "TO"
	case TokenTypePlus:
		return "PLUS"
	case TokenTypeMinus:
		return "MINUS"
	case TokenTypeQuote:
		return "QUOTE"
	case TokenTypeWildcard:
		return "WILDCARD"
	case TokenTypeFuzzy:
		return "FUZZY"
	case TokenTypeBoost:
		return "BOOST"
	case TokenTypeProximity:
		return "PROXIMITY"
	case TokenTypeNumber:
		return "NUMBER"
	default:
		return "UNKNOWN"
	}
}

// Token represents a lexical token in the query string.
type Token struct {
	Type  TokenType
	Value string
	Pos   int // Position in the input string
}

// QueryParserTokenManager tokenizes query strings for the Lucene query parser.
// This is the Go port of Lucene's QueryParserTokenManager.
type QueryParserTokenManager struct {
	input string
	pos   int
	len   int
}

// NewQueryParserTokenManager creates a new token manager for the given input.
func NewQueryParserTokenManager(input string) *QueryParserTokenManager {
	return &QueryParserTokenManager{
		input: input,
		pos:   0,
		len:   len(input),
	}
}

// NextToken returns the next token from the input.
func (tm *QueryParserTokenManager) NextToken() Token {
	// Skip whitespace
	tm.skipWhitespace()

	if tm.pos >= tm.len {
		return Token{Type: TokenTypeEOF, Value: "", Pos: tm.pos}
	}

	ch := tm.current()
	startPos := tm.pos

	switch ch {
	case '(':
		tm.advance()
		return Token{Type: TokenTypeLParen, Value: "(", Pos: startPos}
	case ')':
		tm.advance()
		return Token{Type: TokenTypeRParen, Value: ")", Pos: startPos}
	case '[':
		tm.advance()
		return Token{Type: TokenTypeLBracket, Value: "[", Pos: startPos}
	case ']':
		tm.advance()
		return Token{Type: TokenTypeRBracket, Value: "]", Pos: startPos}
	case '{':
		tm.advance()
		return Token{Type: TokenTypeLBrace, Value: "{", Pos: startPos}
	case '}':
		tm.advance()
		return Token{Type: TokenTypeRBrace, Value: "}", Pos: startPos}
	case ':':
		tm.advance()
		return Token{Type: TokenTypeColon, Value: ":", Pos: startPos}
	case '+':
		tm.advance()
		return Token{Type: TokenTypePlus, Value: "+", Pos: startPos}
	case '-':
		tm.advance()
		return Token{Type: TokenTypeMinus, Value: "-", Pos: startPos}
	case '"':
		tm.advance()
		return Token{Type: TokenTypeQuote, Value: "\"", Pos: startPos}
	case '^':
		tm.advance()
		return Token{Type: TokenTypeBoost, Value: "^", Pos: startPos}
	case '~':
		tm.advance()
		return Token{Type: TokenTypeFuzzy, Value: "~", Pos: startPos}
	case '\\':
		// Escape character - read next char as literal
		tm.advance()
		if tm.pos < tm.len {
			escaped := string(tm.current())
			tm.advance()
			return Token{Type: TokenTypeTerm, Value: escaped, Pos: startPos}
		}
		return Token{Type: TokenTypeEOF, Value: "", Pos: startPos}
	}

	// Check for operators (AND, OR, NOT)
	if tm.isOperator() {
		return tm.readOperator()
	}

	// Check for number (after ^ or ~)
	if unicode.IsDigit(rune(ch)) {
		return tm.readNumber()
	}

	// Read term or field
	return tm.readTermOrField()
}

// GetNextToken is an alias for NextToken for compatibility.
func (tm *QueryParserTokenManager) GetNextToken() Token {
	return tm.NextToken()
}

// current returns the current character.
func (tm *QueryParserTokenManager) current() byte {
	if tm.pos >= tm.len {
		return 0
	}
	return tm.input[tm.pos]
}

// advance moves to the next character.
func (tm *QueryParserTokenManager) advance() {
	tm.pos++
}

// skipWhitespace skips whitespace characters.
func (tm *QueryParserTokenManager) skipWhitespace() {
	for tm.pos < tm.len && unicode.IsSpace(rune(tm.current())) {
		tm.advance()
	}
}

// isOperator checks if we're at an operator (AND, OR, NOT, TO).
func (tm *QueryParserTokenManager) isOperator() bool {
	if tm.pos >= tm.len {
		return false
	}
	ch := unicode.ToUpper(rune(tm.current()))
	return ch == 'A' || ch == 'O' || ch == 'N' || ch == 'T'
}

// readOperator reads an operator token.
func (tm *QueryParserTokenManager) readOperator() Token {
	startPos := tm.pos
	var sb strings.Builder

	for tm.pos < tm.len && unicode.IsLetter(rune(tm.current())) {
		sb.WriteByte(tm.current())
		tm.advance()
	}

	word := strings.ToUpper(sb.String())
	pos := startPos

	switch word {
	case "AND":
		return Token{Type: TokenTypeAND, Value: "AND", Pos: pos}
	case "OR":
		return Token{Type: TokenTypeOR, Value: "OR", Pos: pos}
	case "NOT":
		return Token{Type: TokenTypeNOT, Value: "NOT", Pos: pos}
	case "TO":
		return Token{Type: TokenTypeTO, Value: "TO", Pos: pos}
	default:
		// Not an operator, treat as term
		return Token{Type: TokenTypeTerm, Value: sb.String(), Pos: pos}
	}
}

// readNumber reads a number token.
func (tm *QueryParserTokenManager) readNumber() Token {
	startPos := tm.pos
	var sb strings.Builder

	// Read digits
	for tm.pos < tm.len && unicode.IsDigit(rune(tm.current())) {
		sb.WriteByte(tm.current())
		tm.advance()
	}

	// Read decimal point and more digits
	if tm.pos < tm.len && tm.current() == '.' {
		sb.WriteByte('.')
		tm.advance()
		for tm.pos < tm.len && unicode.IsDigit(rune(tm.current())) {
			sb.WriteByte(tm.current())
			tm.advance()
		}
	}

	return Token{Type: TokenTypeNumber, Value: sb.String(), Pos: startPos}
}

// readTermOrField reads a term or field token.
func (tm *QueryParserTokenManager) readTermOrField() Token {
	startPos := tm.pos
	var sb strings.Builder
	isField := false
	hasWildcard := false

	// Check if it starts with a valid field/term character
	if tm.pos < tm.len {
		ch := tm.current()
		// Fields can start with letter or underscore
		if unicode.IsLetter(rune(ch)) || ch == '_' {
			// Check if this might be a field (look ahead for :)
			savePos := tm.pos
			tempSb := strings.Builder{}
			tempSb.WriteByte(ch)
			tm.advance()

			for tm.pos < tm.len {
				nextCh := tm.current()
				if unicode.IsLetter(rune(nextCh)) || unicode.IsDigit(rune(nextCh)) || nextCh == '_' || nextCh == '-' {
					tempSb.WriteByte(nextCh)
					tm.advance()
				} else {
					break
				}
			}

			// Check if followed by colon
			if tm.pos < tm.len && tm.current() == ':' {
				isField = true
				sb = tempSb
			} else {
				// Not a field, restore position and continue as term
				tm.pos = savePos
				sb.Reset()
				sb.WriteByte(ch)
				tm.advance()
			}
		} else {
			// Term can start with various characters
			sb.WriteByte(ch)
			tm.advance()
		}
	}

	// Continue reading term characters
	for tm.pos < tm.len {
		ch := tm.current()
		if ch == '*' || ch == '?' {
			hasWildcard = true
			sb.WriteByte(ch)
			tm.advance()
		} else if tm.isTermChar(ch, isField) {
			sb.WriteByte(ch)
			tm.advance()
		} else {
			break
		}
	}

	term := sb.String()
	if isField {
		return Token{Type: TokenTypeField, Value: term, Pos: startPos}
	}
	if hasWildcard {
		return Token{Type: TokenTypeWildcard, Value: term, Pos: startPos}
	}
	return Token{Type: TokenTypeTerm, Value: term, Pos: startPos}
}

// isTermChar checks if a character is valid in a term.
func (tm *QueryParserTokenManager) isTermChar(ch byte, isField bool) bool {
	if isField {
		return unicode.IsLetter(rune(ch)) || unicode.IsDigit(rune(ch)) || ch == '_' || ch == '-'
	}
	// Terms can contain various characters (but not special ones)
	return !unicode.IsSpace(rune(ch)) &&
		ch != ':' && ch != '(' && ch != ')' &&
		ch != '[' && ch != ']' && ch != '{' && ch != '}' &&
		ch != '"' && ch != '+' && ch != '-' &&
		ch != '^' && ch != '~' && ch != '\\'
}

// Reset resets the token manager to the beginning of the input.
func (tm *QueryParserTokenManager) Reset(input string) {
	tm.input = input
	tm.pos = 0
	tm.len = len(input)
}

// GetInput returns the current input string.
func (tm *QueryParserTokenManager) GetInput() string {
	return tm.input
}
