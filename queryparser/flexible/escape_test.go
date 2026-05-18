// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import "testing"

func TestEscapeQuerySyntaxImpl_Term(t *testing.T) {
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
		{"", ""},
	}

	for _, tc := range tests {
		got := e.Escape(tc.input, "en", EscapeTerm)
		if got != tc.want {
			t.Errorf("Escape(%q, EscapeTerm) = %q, want %q", tc.input, got, tc.want)
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
	}

	for _, tc := range tests {
		got := e.Escape(tc.input, "en", EscapeString)
		if got != tc.want {
			t.Errorf("Escape(%q, EscapeString) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
