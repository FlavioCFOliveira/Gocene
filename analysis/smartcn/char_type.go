// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package smartcn provides analysis components for Chinese text using a
// Hidden Markov Model (HMM) segmenter.
//
// Go port of org.apache.lucene.analysis.cn.smart (Apache Lucene 10.4.0).
package smartcn

// Character type constants used internally by SmartChineseAnalyzer.
//
// Go port of org.apache.lucene.analysis.cn.smart.CharType.
const (
	// CharTypeDelimiter represents punctuation characters.
	CharTypeDelimiter = 0

	// CharTypeLetter represents ASCII letters.
	CharTypeLetter = 1

	// CharTypeDigit represents numeric digits.
	CharTypeDigit = 2

	// CharTypeHanzi represents Han ideographs.
	CharTypeHanzi = 3

	// CharTypeSpaceLike represents characters that act as a space.
	CharTypeSpaceLike = 4

	// CharTypeFullwidthLetter represents full-width letters.
	CharTypeFullwidthLetter = 5

	// CharTypeFullwidthDigit represents full-width alphanumeric characters.
	CharTypeFullwidthDigit = 6

	// CharTypeOther represents characters not fitting any other category.
	CharTypeOther = 7

	// CharTypeSurrogate represents surrogate characters.
	CharTypeSurrogate = 8
)
