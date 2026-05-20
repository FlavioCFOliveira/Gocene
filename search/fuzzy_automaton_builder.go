// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/FuzzyAutomatonBuilder.java

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// FuzzyAutomatonBuilder builds a set of CompiledAutomaton for fuzzy matching
// on a given term, with specified maximum edit distance, fixed prefix and
// whether or not to allow transpositions.
//
// Mirrors org.apache.lucene.search.FuzzyAutomatonBuilder (Lucene 10.4.0).
//
// Deviations from Java:
//   - Java uses Character.MAX_CODE_POINT (0x10FFFF); Go uses automaton.MaxCodePoint.
//   - Java's TooComplexToDeterminizeException is not modelled: ToAutomatonWithPrefix
//     returns nil when n > MaximumSupportedLevenshteinDistance, which cannot
//     arise in practice because the constructor validates maxEdits against
//     MaximumSupportedLevenshteinDistance.
//   - stringToUTF32 leverages Go's native range-over-string codepoint iteration
//     instead of Java's CharCount / codePointAt surrogate-pair handling.
type FuzzyAutomatonBuilder struct {
	term       string
	maxEdits   int
	levBuilder *automaton.LevenshteinAutomata
	prefix     string
	termLength int
}

// NewFuzzyAutomatonBuilder constructs a FuzzyAutomatonBuilder.
//
// term is the source term string; maxEdits is the maximum Levenshtein distance
// (0..MaximumSupportedLevenshteinDistance); prefixLength is the exact-match
// prefix length (clamped to len(term)); transpositions=true counts adjacent
// transpositions as a single edit.
func NewFuzzyAutomatonBuilder(term string, maxEdits, prefixLength int, transpositions bool) (*FuzzyAutomatonBuilder, error) {
	if maxEdits < 0 || maxEdits > automaton.MaximumSupportedLevenshteinDistance {
		return nil, fmt.Errorf(
			"max edits must be 0..%d, inclusive; got: %d",
			automaton.MaximumSupportedLevenshteinDistance, maxEdits)
	}
	if prefixLength < 0 {
		return nil, fmt.Errorf("prefixLength cannot be less than 0")
	}

	codePoints := stringToUTF32(term)
	termLength := len(codePoints)

	if prefixLength > len(codePoints) {
		prefixLength = len(codePoints)
	}

	suffix := make([]int, len(codePoints)-prefixLength)
	copy(suffix, codePoints[prefixLength:])

	levBuilder := automaton.NewLevenshteinAutomata(suffix, automaton.MaxCodePoint, transpositions)
	prefixRunes := make([]rune, prefixLength)
	for i, cp := range codePoints[:prefixLength] {
		prefixRunes[i] = rune(cp)
	}
	prefix := string(prefixRunes)

	return &FuzzyAutomatonBuilder{
		term:       term,
		maxEdits:   maxEdits,
		levBuilder: levBuilder,
		prefix:     prefix,
		termLength: termLength,
	}, nil
}

// BuildAutomatonSet returns one CompiledAutomaton per edit distance from 0
// to maxEdits, inclusive (len = maxEdits+1).
//
// Mirrors FuzzyAutomatonBuilder.buildAutomatonSet (Lucene 10.4.0).
func (b *FuzzyAutomatonBuilder) BuildAutomatonSet() []*automaton.CompiledAutomaton {
	compiled := make([]*automaton.CompiledAutomaton, b.maxEdits+1)
	for i := 0; i <= b.maxEdits; i++ {
		a := b.levBuilder.ToAutomatonWithPrefix(i, b.prefix)
		compiled[i] = automaton.CompileFull(a, true, false, false)
	}
	return compiled
}

// BuildMaxEditAutomaton returns a CompiledAutomaton for the maximum edit distance.
//
// Mirrors FuzzyAutomatonBuilder.buildMaxEditAutomaton (Lucene 10.4.0).
func (b *FuzzyAutomatonBuilder) BuildMaxEditAutomaton() *automaton.CompiledAutomaton {
	a := b.levBuilder.ToAutomatonWithPrefix(b.maxEdits, b.prefix)
	return automaton.CompileFull(a, true, false, false)
}

// GetTermLength returns the number of Unicode codepoints in the source term.
//
// Mirrors FuzzyAutomatonBuilder.getTermLength (Lucene 10.4.0).
func (b *FuzzyAutomatonBuilder) GetTermLength() int {
	return b.termLength
}

// stringToUTF32 converts a Go string to a slice of Unicode codepoints.
//
// Mirrors FuzzyAutomatonBuilder.stringToUTF32 (Lucene 10.4.0).
func stringToUTF32(text string) []int {
	codePoints := make([]int, 0, len(text))
	for _, r := range text {
		codePoints = append(codePoints, int(r))
	}
	return codePoints
}
