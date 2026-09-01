// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"testing"
)

func TestEscapeQuerySyntaxImpl_Normal(t *testing.T) {
	e := NewEscapeQuerySyntaxImpl()

	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"foo:bar", `foo\:bar`},
		{"foo+bar", `foo\+bar`},
		{"foo-bar", `foo\-bar`},
		{"foo*bar", "foo*bar"}, // wildcards not escaped in term context
		{"foo?bar", "foo?bar"}, // wildcards not escaped in term context
		{"+foo", `\+foo`},      // first char escape
		{"-foo", `\-foo`},      // first char escape
		{"@foo", `\@foo`},      // first char escape
		{"AND", `\AND`},        // parser word
		{"or", `\or`},          // parser word case-insensitive
		{"", ""},
	}

	for _, tc := range tests {
		got := e.Escape(tc.input, "en", EscapeNormal)
		if got != tc.want {
			t.Errorf("Escape(%q, EscapeNormal) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestEscapeQuerySyntaxImpl_String(t *testing.T) {
	e := NewEscapeQuerySyntaxImpl()

	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"foo*bar", `foo\*bar`}, // wildcards ARE escaped in string context
		{"foo?bar", `foo\?bar`},
		{"foo:bar", `foo\:bar`},
		{"\"quote\"", `\"quote\"`},
	}

	for _, tc := range tests {
		got := e.Escape(tc.input, "en", EscapeString)
		if got != tc.want {
			t.Errorf("Escape(%q, EscapeString) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestDiscardEscapeChar(t *testing.T) {
	tests := []struct {
		input string
		want  string
		err   bool
	}{
		{`hello`, "hello", false},
		{`hello\ world`, "hello world", false},
		{`hello\\world`, `hello\world`, false},
		{`A`, "A", false},
		{`\u0041`, `\A`, false},
		{`AB`, "AB", false},
		{`\u004`, "", true},       // truncated unicode
		{`hello\`, "", true},      // trailing escape
	}

	for _, tc := range tests {
		ucs, err := DiscardEscapeChar(tc.input)
		if (err != nil) != tc.err {
			t.Errorf("DiscardEscapeChar(%q) err = %v, want err = %v", tc.input, err, tc.err)
			continue
		}
		if err == nil && ucs.String() != tc.want {
			t.Errorf("DiscardEscapeChar(%q) = %q, want %q", tc.input, ucs.String(), tc.want)
		}
	}
}
