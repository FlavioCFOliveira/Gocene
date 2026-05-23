// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "testing"

// TestIsWhitespace verifies every code point from the Java reference table.
func TestIsWhitespace(t *testing.T) {
	whitespace := []rune{
		0x0009, 0x000A, 0x000B, 0x000C, 0x000D, 0x0020, 0x0085, 0x00A0,
		0x1680, 0x2000, 0x2001, 0x2002, 0x2003, 0x2004, 0x2005, 0x2006,
		0x2007, 0x2008, 0x2009, 0x200A, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000,
	}
	for _, r := range whitespace {
		if !IsWhitespace(r) {
			t.Errorf("IsWhitespace(U+%04X) = false, want true", r)
		}
	}
}

// TestIsWhitespace_NotWhitespace verifies common non-whitespace code points.
func TestIsWhitespace_NotWhitespace(t *testing.T) {
	notWhitespace := []rune{'a', 'Z', '0', '.', '!', 0x00A1, 0x200B, 0x3001}
	for _, r := range notWhitespace {
		if IsWhitespace(r) {
			t.Errorf("IsWhitespace(U+%04X) = true, want false", r)
		}
	}
}
