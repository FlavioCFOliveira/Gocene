// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Source: lucene/core/src/java/org/apache/lucene/util/StrictStringTokenizer.java
// Purpose: Strict, empty-token-preserving tokenizer used by Version parsing.
//
// Port notes:
//   - The Java original is package-private and final. We mirror that scope by
//     exporting nothing besides a small constructor returning a value type.
//   - Java's tokenizer keys on a single char (UTF-16 code unit). Gocene only
//     needs this for ASCII delimiters ('.', '_', '-') used in Version strings,
//     so we accept a byte and perform a byte-indexed scan over the input. This
//     is byte-for-byte equivalent to the Java behavior whenever the input is
//     valid UTF-8 and the delimiter is an ASCII byte (< 0x80). Non-ASCII
//     delimiters are not supported and are not produced by any caller.
//   - Misuse (calling NextToken when no tokens remain) mirrors Java's
//     IllegalStateException by panicking. Matches the "fail-fast on programmer
//     error" contract that the upstream class enforces.

package util

import "strings"

// StrictStringTokenizer splits a string on every occurrence of a single ASCII
// delimiter byte, preserving empty tokens between consecutive delimiters and
// at the boundaries. Unlike Go's strings.Split it is iterator-style so a
// caller can stop early without allocating a slice of all tokens.
//
// Zero value is not usable; construct with NewStrictStringTokenizer.
type StrictStringTokenizer struct {
	s         string
	pos       int  // -1 once exhausted, otherwise next start offset (bytes)
	delimiter byte // ASCII delimiter byte
}

// NewStrictStringTokenizer returns a tokenizer that walks s, splitting on
// delimiter. The delimiter must be an ASCII byte (< 0x80); any other value is
// a programmer error and triggers a panic. ASCII-only is sufficient for the
// Version-string parsing contract documented on the upstream class.
func NewStrictStringTokenizer(s string, delimiter byte) StrictStringTokenizer {
	if delimiter >= 0x80 {
		panic("util: StrictStringTokenizer delimiter must be ASCII")
	}
	return StrictStringTokenizer{s: s, delimiter: delimiter}
}

// NextToken returns the next token. Calling NextToken after HasMoreTokens has
// returned false panics, mirroring the IllegalStateException raised by the
// Java original. The returned string is a substring of the input and shares
// its backing memory.
func (t *StrictStringTokenizer) NextToken() string {
	if t.pos < 0 {
		panic("util: StrictStringTokenizer: no more tokens")
	}

	// strings.IndexByte is a runtime intrinsic — single pass, no allocation.
	rel := strings.IndexByte(t.s[t.pos:], t.delimiter)
	if rel >= 0 {
		start := t.pos
		end := t.pos + rel
		t.pos = end + 1
		return t.s[start:end]
	}

	tok := t.s[t.pos:]
	t.pos = -1
	return tok
}

// HasMoreTokens reports whether NextToken can be called again without panic.
func (t *StrictStringTokenizer) HasMoreTokens() bool {
	return t.pos >= 0
}
