// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package smartcn

// Token type constants used internally by SmartChineseAnalyzer.
//
// Go port of org.apache.lucene.analysis.cn.smart.WordType.
const (
	// WordTypeSentenceBegin marks the start of a sentence.
	WordTypeSentenceBegin = 0

	// WordTypeSentenceEnd marks the end of a sentence.
	WordTypeSentenceEnd = 1

	// WordTypeChineseWord marks a Chinese word.
	WordTypeChineseWord = 2

	// WordTypeString marks an ASCII string.
	WordTypeString = 3

	// WordTypeNumber marks an ASCII alphanumeric token.
	WordTypeNumber = 4

	// WordTypeDelimiter marks a punctuation symbol.
	WordTypeDelimiter = 5

	// WordTypeFullwidthString marks a full-width string.
	WordTypeFullwidthString = 6

	// WordTypeFullwidthNumber marks a full-width alphanumeric token.
	WordTypeFullwidthNumber = 7
)
