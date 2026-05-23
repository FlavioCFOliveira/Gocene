// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package hhmm provides the Hidden Markov Model segmenter internals for
// SmartChineseAnalyzer.
//
// Go port of org.apache.lucene.analysis.cn.smart.hhmm (Apache Lucene 10.4.0).
package hhmm

import "slices"

// SegToken is the SmartChineseAnalyzer internal token type.
//
// Go port of org.apache.lucene.analysis.cn.smart.hhmm.SegToken.
type SegToken struct {
	// CharArray contains the token text as runes.
	CharArray []rune

	// StartOffset is the start offset into the original sentence.
	StartOffset int

	// EndOffset is the end offset into the original sentence.
	EndOffset int

	// WordType is the token type (see smartcn word-type constants).
	WordType int

	// Weight is the word frequency.
	Weight int

	// Index is the position of this token in the token-list table,
	// assigned during segmentation.
	Index int
}

// NewSegToken creates a new SegToken from a rune array.
func NewSegToken(charArray []rune, start, end, wordType, weight int) *SegToken {
	return &SegToken{
		CharArray:   charArray,
		StartOffset: start,
		EndOffset:   end,
		WordType:    wordType,
		Weight:      weight,
	}
}

// Equal reports whether t equals other by value (mirrors Java equals).
func (t *SegToken) Equal(other *SegToken) bool {
	if t == other {
		return true
	}
	if other == nil {
		return false
	}
	return slices.Equal(t.CharArray, other.CharArray) &&
		t.EndOffset == other.EndOffset &&
		t.Index == other.Index &&
		t.StartOffset == other.StartOffset &&
		t.Weight == other.Weight &&
		t.WordType == other.WordType
}
