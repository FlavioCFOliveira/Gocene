// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// Sprint 54 Phase 3 promoted the six small token attributes
// (TypeAttribute, PayloadAttribute, FlagsAttribute, KeywordAttribute,
// PositionLengthAttribute, TermFrequencyAttribute) to Lucene-faithful
// interface+impl pairs. Equals, HashCode, ReflectWith and the
// validated setters that used to live in this file are now methods on
// the concrete impls in token_attributes.go.
//
// This file retains a small shared helper (javaStringHash) that those
// impls share, plus the parity history note above so the rationale is
// not lost.

// javaStringHash computes the Java {@code String.hashCode()} of s.
// Implementation: starts at 0 and iterates each UTF-16 code unit; for
// pure ASCII strings (the common Lucene case for type tokens like
// "word") the value is identical to iterating bytes.
func javaStringHash(s string) int {
	h := 0
	for i := 0; i < len(s); i++ {
		h = h*31 + int(int8(s[i]))
	}
	return h
}
