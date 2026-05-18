package surround

import (
	"errors"
	"strings"
	"testing"
)

func TestParseExceptionBasic(t *testing.T) {
	e := NewParseException("bad")
	if e.Error() != "bad" {
		t.Errorf("Error()=%q", e.Error())
	}
	if e.Unwrap() != nil {
		t.Error("Unwrap should be nil")
	}
}

func TestParseExceptionWithCause(t *testing.T) {
	cause := errors.New("io")
	e := NewParseExceptionWithCause("wrap", cause)
	if !errors.Is(e, cause) {
		t.Error("errors.Is should match wrapped cause")
	}
}

func TestParseExceptionFromToken(t *testing.T) {
	cur := &Token{
		Next: &Token{Kind: Term, Image: "foo", BeginLine: 3, BeginColumn: 7},
	}
	expected := [][]int{{Term, AndOp}}
	msg := NewParseExceptionFromToken(cur, expected, TokenImage).Error()
	if !strings.Contains(msg, "line 3, column 7") {
		t.Errorf("missing location: %q", msg)
	}
	if !strings.Contains(msg, "\"AND\"") {
		t.Errorf("missing expected image: %q", msg)
	}
}

func TestTokenStringIsImage(t *testing.T) {
	tok := NewTokenWithImage(Term, "hello")
	if tok.String() != "hello" {
		t.Errorf("String()=%q", tok.String())
	}
}

func TestTokenMgrErrorMessage(t *testing.T) {
	e := NewTokenMgrErrorFull(false, 0, 2, 5, "ab", 'x', LexicalError)
	if !strings.Contains(e.Error(), "line 2, column 5") {
		t.Errorf("location missing: %q", e.Error())
	}
	if !strings.Contains(e.Error(), "after prefix \"ab\"") {
		t.Errorf("prefix missing: %q", e.Error())
	}
}

func TestTokenMgrErrorEOF(t *testing.T) {
	e := NewTokenMgrErrorFull(true, 0, 1, 1, "", 0, LexicalError)
	if !strings.Contains(e.Error(), "<EOF>") {
		t.Errorf("EOF marker missing: %q", e.Error())
	}
}

func TestAddEscapes(t *testing.T) {
	cases := map[string]string{
		"abc":     "abc",
		"a\tb":    "a\\tb",
		"\x01":    "\\u0001",
		"\"quo\"": "\\\"quo\\\"",
	}
	for in, want := range cases {
		got := addEscapes(in)
		if got != want {
			t.Errorf("addEscapes(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestTokenImageLookup(t *testing.T) {
	if GetTokenImage(AndOp) != "\"AND\"" {
		t.Error("AND image")
	}
	if GetTokenImage(EOF) != "<EOF>" {
		t.Error("EOF image")
	}
	if GetTokenImage(-1) != "<UNKNOWN>" {
		t.Error("unknown")
	}
}
