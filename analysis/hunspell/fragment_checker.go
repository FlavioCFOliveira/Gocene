// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

// FragmentChecker is an oracle for quickly ruling out word sub-ranges that
// cannot appear in valid words of the language.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.FragmentChecker from Apache Lucene 10.4.0.
type FragmentChecker interface {
	// HasImpossibleFragmentAround reports whether the range [start, end] in
	// word intersects any fragment that is impossible in the language.
	HasImpossibleFragmentAround(word []rune, start, end int) bool
}

// EverythingPossible is a FragmentChecker that always returns false (no
// fragments are impossible).
var EverythingPossible FragmentChecker = everythingPossible{}

type everythingPossible struct{}

func (everythingPossible) HasImpossibleFragmentAround(_ []rune, _, _ int) bool { return false }
