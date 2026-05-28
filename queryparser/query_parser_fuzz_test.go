// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package queryparser

import "testing"

// FuzzQueryParser fuzzes the classic Lucene query parser over arbitrary
// query strings.
//
// Property: Parse must never panic on any input. A parse error is an
// acceptable, expected outcome for malformed syntax; a panic is a bug.
// Query strings are untrusted input (typically supplied by end users),
// so the parser must degrade to an error rather than crash the host.
//
// The seed corpus covers valid queries and the syntactic edge cases that
// historically destabilise hand-written recursive-descent parsers:
// wildcards, fuzzy/boost suffixes, ranges, quoted phrases, unbalanced
// parentheses and brackets, dangling operators, escape sequences, and
// non-ASCII input.
func FuzzQueryParser(f *testing.F) {
	seeds := []string{
		// Valid, well-formed queries.
		"",
		"hello",
		"hello world",
		"content:hello",
		"hello AND world",
		"hello OR world",
		"NOT hello",
		"+hello -world",
		`"a phrase"`,
		"term*",
		"te?t",
		"roam~",
		"roam~2",
		"jakarta^4 apache",
		"(hello OR world) AND foo",
		"[a TO b]",
		"{a TO b}",
		"title:[2020 TO 2024]",
		"field:(one two three)",
		// Edge cases / malformed syntax (must error, never panic).
		"(",
		")",
		"((()",
		"[a TO",
		"{a TO b",
		`"unterminated`,
		"AND",
		"OR OR OR",
		"+",
		"-",
		"^",
		"~",
		"*",
		"?",
		":",
		"field:",
		":value",
		`a\:b`,
		`escaped\(paren`,
		"a^",
		"a~xyz",
		"a^^^2",
		"\x00\x01\x02",
		"café AND naïve",
		"日本語 OR 中文",
		"\t\n\r ",
		"a TO b TO c",
		"+++---",
		"((((((((((((((((((((",
		"))))))))))))))))))))",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, query string) {
		parser := NewQueryParserWithDefaultField("content")
		// Discard the result: the property under test is the absence of a
		// panic, not the parse outcome. Errors are valid.
		_, _ = parser.Parse(query)
	})
}
