package queryparser

import (
	"errors"
	"strings"
	"testing"
)

func TestParseException(t *testing.T) {
	tests := []struct {
		name     string
		exc      *ParseException
		wantMsg  string
		wantQuery string
	}{
		{
			name:    "simple message",
			exc:     NewParseException("syntax error"),
			wantMsg: "syntax error",
		},
		{
			name:      "with query context",
			exc:       NewParseExceptionWithQuery("field:value", errors.New("invalid token")),
			wantMsg:   "Cannot parse 'field:value': invalid token",
			wantQuery: "field:value",
		},
		{
			name:    "with location",
			exc:     NewParseExceptionWithLocation("query", 1, 5, errors.New("unexpected char")),
			wantMsg: "Cannot parse 'query' at line 1, column 5: unexpected char",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(tt.exc.Error(), tt.wantMsg) {
				t.Errorf("Error() = %v, want containing %v", tt.exc.Error(), tt.wantMsg)
			}
			if tt.wantQuery != "" && tt.exc.Query != tt.wantQuery {
				t.Errorf("Query = %v, want %v", tt.exc.Query, tt.wantQuery)
			}
		})
	}
}

func TestParseExceptionUnwrap(t *testing.T) {
	cause := errors.New("original error")
	exc := NewParseExceptionWithCause("wrapped", cause)

	if !errors.Is(exc, cause) {
		t.Error("expected Unwrap to return the cause")
	}
}

func TestTokenMgrError(t *testing.T) {
	tests := []struct {
		name    string
		err     *TokenMgrError
		wantMsg string
		wantCode int
	}{
		{
			name:     "lexical error",
			err:      NewTokenMgrError("Invalid character", LexicalError),
			wantMsg:  "Invalid character",
			wantCode: LexicalError,
		},
		{
			name:     "static lexer error",
			err:      NewTokenMgrError("Static token manager", StaticLexerError),
			wantMsg:  "Static token manager",
			wantCode: StaticLexerError,
		},
		{
			name:     "invalid lexical state",
			err:      NewTokenMgrError("Invalid state", InvalidLexicalState),
			wantMsg:  "Invalid state",
			wantCode: InvalidLexicalState,
		},
		{
			name:     "loop detected",
			err:      NewTokenMgrError("Infinite loop", LoopDetected),
			wantMsg:  "Infinite loop",
			wantCode: LoopDetected,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.wantMsg {
				t.Errorf("Error() = %v, want %v", tt.err.Error(), tt.wantMsg)
			}
			if tt.err.ErrorCode != tt.wantCode {
				t.Errorf("ErrorCode = %v, want %v", tt.err.ErrorCode, tt.wantCode)
			}
		})
	}
}

func TestTokenMgrErrorFull(t *testing.T) {
	err := NewTokenMgrErrorFull(false, 0, 1, 10, "prefix", 'x', LexicalError)

	if err.ErrorCode != LexicalError {
		t.Errorf("ErrorCode = %v, want %v", err.ErrorCode, LexicalError)
	}
	if err.ErrorLine != 1 {
		t.Errorf("ErrorLine = %v, want %v", err.ErrorLine, 1)
	}
	if err.ErrorCol != 10 {
		t.Errorf("ErrorCol = %v, want %v", err.ErrorCol, 10)
	}
	if !strings.Contains(err.Error(), "Lexical error") {
		t.Error("Expected error message to contain 'Lexical error'")
	}
	if !strings.Contains(err.Error(), "line 1") {
		t.Error("Expected error message to contain 'line 1'")
	}
	if !strings.Contains(err.Error(), "column 10") {
		t.Error("Expected error message to contain 'column 10'")
	}
}

func TestLexicalErrorMsg(t *testing.T) {
	tests := []struct {
		name        string
		eofSeen     bool
		lexState    int
		errorLine   int
		errorColumn int
		errorAfter  string
		curChar     rune
		wantParts   []string
	}{
		{
			name:        "EOF error",
			eofSeen:     true,
			lexState:    0,
			errorLine:   5,
			errorColumn: 20,
			errorAfter:  "",
			curChar:     0,
			wantParts:   []string{"Lexical error", "line 5", "column 20", "<EOF>"},
		},
		{
			name:        "character error with prefix",
			eofSeen:     false,
			lexState:    0,
			errorLine:   3,
			errorColumn: 15,
			errorAfter:  "field:",
			curChar:     '@',
			wantParts:   []string{"Lexical error", "line 3", "column 15", "'@'", "after prefix", "field:"},
		},
		{
			name:        "error with lexical state",
			eofSeen:     false,
			lexState:    2,
			errorLine:   1,
			errorColumn: 5,
			errorAfter:  "",
			curChar:     '#',
			wantParts:   []string{"Lexical error", "lexical state 2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := LexicalErrorMsg(tt.eofSeen, tt.lexState, tt.errorLine, tt.errorColumn, tt.errorAfter, tt.curChar)
			for _, part := range tt.wantParts {
				if !strings.Contains(msg, part) {
					t.Errorf("LexicalErrorMsg() = %v, want containing %v", msg, part)
				}
			}
		})
	}
}

func TestAddEscapes(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"hello\tworld", "hello\\tworld"},
		{"hello\nworld", "hello\\nworld"},
		{"hello\rworld", "hello\\rworld"},
		{"hello\bworld", "hello\\bworld"},
		{"hello\fworld", "hello\\fworld"},
		{`hello"world`, `hello\"world`},
		{`hello'world`, `hello\'world`},
		{`hello\world`, `hello\\world`},
		{"hello\x01world", "hello\\u0001world"},
		{"hello\x7fworld", "hello\\u007fworld"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := addEscapes(tt.input)
			if result != tt.expected {
				t.Errorf("addEscapes(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
