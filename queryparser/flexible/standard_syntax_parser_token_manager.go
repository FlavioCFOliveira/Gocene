// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

// StandardSyntaxParserTokenManager is the lexer for the flexible standard
// query parser. It processes a CharStream (represented here as a string) and
// produces a sequence of Token objects, each identified by one of the SSPc*
// kind constants defined in standard_syntax_parser_constants.go.
//
// Mirrors the JavaCC-generated class
// org.apache.lucene.queryparser.flexible.standard.parser.StandardSyntaxParserTokenManager.
type StandardSyntaxParserTokenManager struct {
	input     []rune
	pos       int
	lexState  int
	curToken  *SSPToken
	errorVerbose bool
}

// SSPToken represents a single scanned token. It mirrors the Token class
// produced by the JavaCC grammar.
type SSPToken struct {
	// Kind is the token kind (one of the SSPc* constants).
	Kind int
	// Image is the matched text.
	Image string
	// BeginLine is the 1-based line number of the first character.
	BeginLine int
	// BeginColumn is the 1-based column of the first character.
	BeginColumn int
	// EndLine is the 1-based line number of the last character.
	EndLine int
	// EndColumn is the 1-based column of the last character.
	EndColumn int
	// Next links to the following token in the stream; nil for the last token.
	Next *SSPToken
}

// LexStateNames provides a human-readable name for each lexical state.
var LexStateNames = []string{
	"Function",
	"Range",
	"DEFAULT",
}

// NewStandardSyntaxParserTokenManager constructs a token manager for the
// supplied input string, starting in the DEFAULT lexical state.
func NewStandardSyntaxParserTokenManager(input string) *StandardSyntaxParserTokenManager {
	return &StandardSyntaxParserTokenManager{
		input:    []rune(input),
		pos:      0,
		lexState: SSPsDefault,
	}
}

// NewStandardSyntaxParserTokenManagerState constructs a token manager for the
// supplied input string starting in the given lexical state.
func NewStandardSyntaxParserTokenManagerState(input string, lexState int) *StandardSyntaxParserTokenManager {
	return &StandardSyntaxParserTokenManager{
		input:    []rune(input),
		pos:      0,
		lexState: lexState,
	}
}

// SwitchTo changes the active lexical state. State must be one of
// SSPsFunction, SSPsRange, or SSPsDefault.
func (m *StandardSyntaxParserTokenManager) SwitchTo(state int) {
	m.lexState = state
}

// GetNextToken scans the input and returns the next token. Returns a token
// with Kind == SSPcEOF when the input is exhausted.
//
// This implementation recognises the core token kinds produced by the JavaCC
// grammar. It is sufficient for the query parsing done by StandardSyntaxParser
// and passes bytes through to the parser unchanged.
func (m *StandardSyntaxParserTokenManager) GetNextToken() (*SSPToken, error) {
	// Skip whitespace (all lexical states skip it between tokens).
	m.skipWhitespace()

	if m.pos >= len(m.input) {
		return &SSPToken{Kind: SSPcEOF, Image: ""}, nil
	}

	startPos := m.pos
	tok, err := m.scanToken()
	if err != nil {
		return nil, &StandardSyntaxParserTokenMgrError{
			Message:   err.Error(),
			ErrorCode: 0,
		}
	}
	_ = startPos
	return tok, nil
}

// scanToken dispatches to the appropriate scanner based on the current
// lexical state and the next character.
func (m *StandardSyntaxParserTokenManager) scanToken() (*SSPToken, error) {
	ch := m.input[m.pos]

	switch m.lexState {
	case SSPsRange:
		return m.scanRangeToken(ch)
	default:
		return m.scanDefaultToken(ch)
	}
}

// scanDefaultToken handles tokens in the DEFAULT and Function states.
func (m *StandardSyntaxParserTokenManager) scanDefaultToken(ch rune) (*SSPToken, error) {
	switch ch {
	case '+':
		m.pos++
		return &SSPToken{Kind: SSPcPlus, Image: "+"}, nil
	case '-':
		m.pos++
		return &SSPToken{Kind: SSPcMinus, Image: "-"}, nil
	case '(':
		m.pos++
		return &SSPToken{Kind: SSPcLParen, Image: "("}, nil
	case ')':
		m.pos++
		return &SSPToken{Kind: SSPcRParen, Image: ")"}, nil
	case ':':
		m.pos++
		return &SSPToken{Kind: SSPcOpColon, Image: ":"}, nil
	case '=':
		m.pos++
		return &SSPToken{Kind: SSPcOpEqual, Image: "="}, nil
	case '^':
		m.pos++
		return &SSPToken{Kind: SSPcCarat, Image: "^"}, nil
	case '~':
		m.pos++
		return &SSPToken{Kind: SSPcTilde, Image: "~"}, nil
	case '[':
		m.pos++
		m.lexState = SSPsRange
		return &SSPToken{Kind: SSPcRangeInStart, Image: "["}, nil
	case '{':
		m.pos++
		m.lexState = SSPsRange
		return &SSPToken{Kind: SSPcRangeExStart, Image: "{"}, nil
	case '<':
		if m.peek(1) == '=' {
			m.pos += 2
			return &SSPToken{Kind: SSPcOpLessThanEq, Image: "<="}, nil
		}
		m.pos++
		return &SSPToken{Kind: SSPcOpLessThan, Image: "<"}, nil
	case '>':
		if m.peek(1) == '=' {
			m.pos += 2
			return &SSPToken{Kind: SSPcOpMoreThanEq, Image: ">="}, nil
		}
		m.pos++
		return &SSPToken{Kind: SSPcOpMoreThan, Image: ">"}, nil
	case '"':
		return m.scanQuoted()
	case '/':
		return m.scanRegexp()
	}

	// Try to scan a term/keyword/number.
	return m.scanTermOrKeyword()
}

// scanRangeToken handles tokens in the Range lexical state.
func (m *StandardSyntaxParserTokenManager) scanRangeToken(ch rune) (*SSPToken, error) {
	switch ch {
	case ']':
		m.pos++
		m.lexState = SSPsDefault
		return &SSPToken{Kind: SSPcRangeInEnd, Image: "]"}, nil
	case '}':
		m.pos++
		m.lexState = SSPsDefault
		return &SSPToken{Kind: SSPcRangeExEnd, Image: "}"}, nil
	case '"':
		tok, err := m.scanQuoted()
		if err != nil {
			return nil, err
		}
		tok.Kind = SSPcRangeQuoted
		return tok, nil
	}

	// Scan unquoted range token (RANGE_GOOP).
	start := m.pos
	for m.pos < len(m.input) {
		c := m.input[m.pos]
		if c == ']' || c == '}' || c == '"' || isWhitespace(c) {
			break
		}
		m.pos++
	}
	if m.pos == start {
		return nil, &StandardSyntaxParserTokenMgrError{
			Message:   "unexpected character in range: " + string(ch),
			ErrorCode: 0,
		}
	}
	img := string(m.input[start:m.pos])
	if img == "TO" {
		return &SSPToken{Kind: SSPcRangeTo, Image: img}, nil
	}
	return &SSPToken{Kind: SSPcRangeGoop, Image: img}, nil
}

// scanQuoted reads a double-quoted string and returns a QUOTED token.
func (m *StandardSyntaxParserTokenManager) scanQuoted() (*SSPToken, error) {
	start := m.pos
	m.pos++ // consume opening "
	for m.pos < len(m.input) {
		c := m.input[m.pos]
		m.pos++
		if c == '\\' && m.pos < len(m.input) {
			m.pos++ // skip escaped char
			continue
		}
		if c == '"' {
			return &SSPToken{Kind: SSPcQuoted, Image: string(m.input[start:m.pos])}, nil
		}
	}
	return nil, &StandardSyntaxParserTokenMgrError{
		Message:   "unterminated quoted string",
		ErrorCode: 0,
	}
}

// scanRegexp reads a /…/ regex literal.
func (m *StandardSyntaxParserTokenManager) scanRegexp() (*SSPToken, error) {
	start := m.pos
	m.pos++ // consume opening /
	for m.pos < len(m.input) {
		c := m.input[m.pos]
		m.pos++
		if c == '\\' && m.pos < len(m.input) {
			m.pos++
			continue
		}
		if c == '/' {
			return &SSPToken{Kind: SSPcRegexpTerm, Image: string(m.input[start:m.pos])}, nil
		}
	}
	return nil, &StandardSyntaxParserTokenMgrError{
		Message:   "unterminated regexp literal",
		ErrorCode: 0,
	}
}

// scanTermOrKeyword scans an unquoted term and resolves keywords.
func (m *StandardSyntaxParserTokenManager) scanTermOrKeyword() (*SSPToken, error) {
	ch := m.input[m.pos]
	// A number literal.
	if isDigit(ch) || (ch == '-' && m.pos+1 < len(m.input) && isDigit(m.input[m.pos+1])) {
		return m.scanNumber()
	}

	start := m.pos
	isWild := false
	for m.pos < len(m.input) {
		c := m.input[m.pos]
		if isWhitespace(c) || c == '+' || c == '-' || c == '(' || c == ')' ||
			c == ':' || c == '^' || c == '[' || c == '{' || c == '"' || c == '/' {
			break
		}
		if c == '*' || c == '?' {
			isWild = true
		}
		if c == '\\' && m.pos+1 < len(m.input) {
			m.pos += 2
			continue
		}
		m.pos++
	}
	if m.pos == start {
		return nil, &StandardSyntaxParserTokenMgrError{
			Message:   "unexpected character: " + string(ch),
			ErrorCode: 0,
		}
	}

	img := string(m.input[start:m.pos])

	// Resolve keywords.
	switch img {
	case "AND", "&&":
		return &SSPToken{Kind: SSPcAND, Image: img}, nil
	case "OR", "||":
		return &SSPToken{Kind: SSPcOR, Image: img}, nil
	case "NOT", "!":
		return &SSPToken{Kind: SSPcNOT, Image: img}, nil
	case "TO":
		return &SSPToken{Kind: SSPcRangeTo, Image: img}, nil
	}

	if isWild {
		return &SSPToken{Kind: SSPcTerm, Image: img}, nil
	}
	return &SSPToken{Kind: SSPcTerm, Image: img}, nil
}

// scanNumber scans a numeric literal.
func (m *StandardSyntaxParserTokenManager) scanNumber() (*SSPToken, error) {
	start := m.pos
	if m.pos < len(m.input) && m.input[m.pos] == '-' {
		m.pos++
	}
	for m.pos < len(m.input) && isDigit(m.input[m.pos]) {
		m.pos++
	}
	if m.pos < len(m.input) && m.input[m.pos] == '.' {
		m.pos++
		for m.pos < len(m.input) && isDigit(m.input[m.pos]) {
			m.pos++
		}
	}
	return &SSPToken{Kind: SSPcNumber, Image: string(m.input[start:m.pos])}, nil
}

// peek returns the character at pos+offset, or 0 if out of bounds.
func (m *StandardSyntaxParserTokenManager) peek(offset int) rune {
	if m.pos+offset < len(m.input) {
		return m.input[m.pos+offset]
	}
	return 0
}

// skipWhitespace advances past spaces, tabs, and newlines.
func (m *StandardSyntaxParserTokenManager) skipWhitespace() {
	for m.pos < len(m.input) && isWhitespace(m.input[m.pos]) {
		m.pos++
	}
}

func isWhitespace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}
