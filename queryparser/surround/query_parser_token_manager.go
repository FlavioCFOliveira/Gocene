package surround

import (
	"strings"
	"unicode"
)

// QueryParserTokenManager is the hand-written lexer for the surround query
// language. It produces the same Token kinds emitted by JavaCC's generated
// QueryParserTokenManager (see query_parser_constants.go for the kind table)
// and feeds them to QueryParser.
type QueryParserTokenManager struct {
	input []rune
	pos   int
	line  int
	col   int
}

// NewQueryParserTokenManager builds a token manager over an input string.
func NewQueryParserTokenManager(input string) *QueryParserTokenManager {
	return &QueryParserTokenManager{input: []rune(input), line: 1, col: 1}
}

// NextToken returns the next non-whitespace token. It returns an EOF token
// when the input is exhausted.
func (m *QueryParserTokenManager) NextToken() (*Token, error) {
	m.skipWhitespace()
	if m.pos >= len(m.input) {
		return &Token{Kind: EOF, BeginLine: m.line, BeginColumn: m.col, EndLine: m.line, EndColumn: m.col, Image: ""}, nil
	}
	start := m.pos
	startLine, startCol := m.line, m.col
	c := m.input[m.pos]
	switch c {
	case '(':
		m.advance()
		return m.tokenFrom(OpenParen, "(", startLine, startCol), nil
	case ')':
		m.advance()
		return m.tokenFrom(CloseParen, ")", startLine, startCol), nil
	case ',':
		m.advance()
		return m.tokenFrom(Comma, ",", startLine, startCol), nil
	case ':':
		m.advance()
		return m.tokenFrom(Colon, ":", startLine, startCol), nil
	case '^':
		m.advance()
		return m.tokenFrom(Caret, "^", startLine, startCol), nil
	case '"':
		return m.readQuoted(startLine, startCol)
	}
	if unicode.IsDigit(c) {
		return m.readNumberOrDistance(startLine, startCol)
	}
	if isTermStartChar(c) {
		return m.readTermOrKeyword(startLine, startCol, start)
	}
	// Skip unknown char, treat as TERM of length 1.
	m.advance()
	return m.tokenFrom(Term, string(c), startLine, startCol), nil
}

func (m *QueryParserTokenManager) readQuoted(startLine, startCol int) (*Token, error) {
	m.advance()
	var b strings.Builder
	for m.pos < len(m.input) {
		c := m.input[m.pos]
		if c == '"' {
			m.advance()
			return m.tokenFrom(QuotedToken, "\""+b.String()+"\"", startLine, startCol), nil
		}
		if c == '\\' && m.pos+1 < len(m.input) {
			b.WriteRune(m.input[m.pos+1])
			m.pos += 2
			m.col += 2
			continue
		}
		b.WriteRune(c)
		m.advance()
	}
	return nil, NewTokenMgrError(LexicalErrorMsg(true, 0, startLine, startCol, "", 0), LexicalError)
}

func (m *QueryParserTokenManager) readNumberOrDistance(startLine, startCol int) (*Token, error) {
	start := m.pos
	for m.pos < len(m.input) && unicode.IsDigit(m.input[m.pos]) {
		m.advance()
	}
	if m.pos < len(m.input) && m.input[m.pos] == '.' && m.pos+1 < len(m.input) && unicode.IsDigit(m.input[m.pos+1]) {
		m.advance()
		for m.pos < len(m.input) && unicode.IsDigit(m.input[m.pos]) {
			m.advance()
		}
		return m.tokenFrom(NumberToken, string(m.input[start:m.pos]), startLine, startCol), nil
	}
	numText := string(m.input[start:m.pos])
	if m.pos < len(m.input) {
		next := m.input[m.pos]
		if next == 'W' || next == 'w' {
			m.advance()
			return m.tokenFrom(W, numText+string(next), startLine, startCol), nil
		}
		if next == 'N' || next == 'n' {
			m.advance()
			return m.tokenFrom(N, numText+string(next), startLine, startCol), nil
		}
	}
	return m.tokenFrom(NumberToken, numText, startLine, startCol), nil
}

func (m *QueryParserTokenManager) readTermOrKeyword(startLine, startCol, start int) (*Token, error) {
	hasWildcard := false
	for m.pos < len(m.input) {
		c := m.input[m.pos]
		if c == '*' || c == '?' {
			hasWildcard = true
			m.advance()
			continue
		}
		if !isTermChar(c) {
			break
		}
		m.advance()
	}
	text := string(m.input[start:m.pos])
	if !hasWildcard {
		switch strings.ToUpper(text) {
		case "OR":
			return m.tokenFrom(OrOp, text, startLine, startCol), nil
		case "AND":
			return m.tokenFrom(AndOp, text, startLine, startCol), nil
		case "NOT":
			return m.tokenFrom(NotOp, text, startLine, startCol), nil
		}
		return m.tokenFrom(Term, text, startLine, startCol), nil
	}
	if strings.HasSuffix(text, "*") && !strings.ContainsAny(text[:len(text)-1], "*?") {
		return m.tokenFrom(Truncterm, text, startLine, startCol), nil
	}
	return m.tokenFrom(Suffixterm, text, startLine, startCol), nil
}

func (m *QueryParserTokenManager) tokenFrom(kind int, image string, startLine, startCol int) *Token {
	return &Token{
		Kind:        kind,
		Image:       image,
		BeginLine:   startLine,
		BeginColumn: startCol,
		EndLine:     m.line,
		EndColumn:   m.col,
	}
}

func (m *QueryParserTokenManager) skipWhitespace() {
	for m.pos < len(m.input) && unicode.IsSpace(m.input[m.pos]) {
		m.advance()
	}
}

func (m *QueryParserTokenManager) advance() {
	if m.pos >= len(m.input) {
		return
	}
	if m.input[m.pos] == '\n' {
		m.line++
		m.col = 1
	} else {
		m.col++
	}
	m.pos++
}

// isTermStartChar reports whether r can begin a TERM token.
func isTermStartChar(r rune) bool {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return true
	}
	switch r {
	case '_', '-', '.', '/', '\\':
		return true
	}
	return false
}

// isTermChar reports whether r can continue a TERM token (excluding the
// wildcard characters which require special handling).
func isTermChar(r rune) bool {
	if isTermStartChar(r) {
		return true
	}
	return false
}
