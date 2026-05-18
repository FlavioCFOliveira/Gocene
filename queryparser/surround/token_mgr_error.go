package surround

import "fmt"

// Error codes matching JavaCC's TokenMgrError.
const (
	LexicalError        = 0
	StaticLexerError    = 1
	InvalidLexicalState = 2
	LoopDetected        = 3
)

// TokenMgrError is the surround equivalent of JavaCC's TokenMgrError. In Java
// it extends Error; in Go it's a regular error type.
type TokenMgrError struct {
	Message    string
	ErrorCode  int
	EOFSeen    bool
	LexState   int
	ErrorLine  int
	ErrorCol   int
	ErrorAfter string
	CurChar    rune
}

func (e *TokenMgrError) Error() string { return e.Message }

// NewTokenMgrError builds a basic TokenMgrError.
func NewTokenMgrError(message string, errorCode int) *TokenMgrError {
	return &TokenMgrError{Message: message, ErrorCode: errorCode}
}

// NewTokenMgrErrorFull mirrors JavaCC's full constructor with location info.
func NewTokenMgrErrorFull(eofSeen bool, lexState, line, col int, errorAfter string, curChar rune, reason int) *TokenMgrError {
	return &TokenMgrError{
		Message:    LexicalErrorMsg(eofSeen, lexState, line, col, errorAfter, curChar),
		ErrorCode:  reason,
		EOFSeen:    eofSeen,
		LexState:   lexState,
		ErrorLine:  line,
		ErrorCol:   col,
		ErrorAfter: errorAfter,
		CurChar:    curChar,
	}
}

// LexicalErrorMsg formats a JavaCC-style "Lexical error at line X, column Y..." message.
func LexicalErrorMsg(eofSeen bool, lexState, line, col int, errorAfter string, curChar rune) string {
	encountered := "<EOF>"
	if !eofSeen {
		encountered = fmt.Sprintf("'%s' (%d)", addEscapes(string(curChar)), int(curChar))
	}
	after := ""
	if errorAfter != "" {
		after = fmt.Sprintf(" after prefix \"%s\"", addEscapes(errorAfter))
	}
	state := ""
	if lexState != 0 {
		state = fmt.Sprintf(" (in lexical state %d)", lexState)
	}
	return fmt.Sprintf("Lexical error at line %d, column %d. Encountered: %s%s%s",
		line, col, encountered, after, state)
}

// addEscapes mirrors JavaCC's addEscapes helper.
func addEscapes(str string) string {
	out := make([]rune, 0, len(str)*2)
	for _, ch := range str {
		switch ch {
		case '\b':
			out = append(out, '\\', 'b')
		case '\t':
			out = append(out, '\\', 't')
		case '\n':
			out = append(out, '\\', 'n')
		case '\f':
			out = append(out, '\\', 'f')
		case '\r':
			out = append(out, '\\', 'r')
		case '"':
			out = append(out, '\\', '"')
		case '\'':
			out = append(out, '\\', '\'')
		case '\\':
			out = append(out, '\\', '\\')
		default:
			if ch < 0x20 || ch > 0x7e {
				out = append(out, []rune(fmt.Sprintf("\\u%04x", ch))...)
			} else {
				out = append(out, ch)
			}
		}
	}
	return string(out)
}
