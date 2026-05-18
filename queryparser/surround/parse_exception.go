// Package surround implements Lucene's surround query parser for span-style
// proximity queries (e.g. `2W(term1, term2)` for "term1 within 2 of term2").
package surround

import "fmt"

// ParseException is returned when the surround parser encounters a syntax
// error. It mirrors org.apache.lucene.queryparser.surround.parser.ParseException.
type ParseException struct {
	Message      string
	Cause        error
	CurrentToken *Token
	ExpectedKind [][]int
	TokenImage   []string
}

func (e *ParseException) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "surround parse error"
}

func (e *ParseException) Unwrap() error { return e.Cause }

// NewParseException creates a plain-message ParseException.
func NewParseException(message string) *ParseException {
	return &ParseException{Message: message}
}

// NewParseExceptionWithCause wraps a lower-level error.
func NewParseExceptionWithCause(message string, cause error) *ParseException {
	return &ParseException{Message: message, Cause: cause}
}

// NewParseExceptionFromToken builds the JavaCC-style "Encountered ... at line X,
// column Y. Was expecting one of:" message used by the generated parser.
func NewParseExceptionFromToken(current *Token, expected [][]int, tokenImage []string) *ParseException {
	return &ParseException{
		Message:      formatParseExceptionMessage(current, expected, tokenImage),
		CurrentToken: current,
		ExpectedKind: expected,
		TokenImage:   tokenImage,
	}
}

func formatParseExceptionMessage(current *Token, expected [][]int, tokenImage []string) string {
	if current == nil {
		return "surround parse error"
	}
	tok := current.Next
	if tok == nil {
		tok = current
	}
	encountered := tok.Image
	line, col := tok.BeginLine, tok.BeginColumn
	msg := fmt.Sprintf("Encountered \"%s\" at line %d, column %d.", encountered, line, col)
	if len(expected) > 0 {
		msg += " Was expecting one of:"
		for _, e := range expected {
			for _, k := range e {
				if k >= 0 && k < len(tokenImage) {
					msg += " " + tokenImage[k]
				}
			}
		}
	}
	return msg
}
