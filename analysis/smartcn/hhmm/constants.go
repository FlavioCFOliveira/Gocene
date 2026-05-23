// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hhmm

// Character type constants used by the SmartChineseAnalyzer.
//
// These mirror org.apache.lucene.analysis.cn.smart.CharType.
// They are defined here to avoid an import cycle between the hhmm and
// smartcn packages.
const (
	CharTypeDelimiter       = 0
	CharTypeLetter          = 1
	CharTypeDigit           = 2
	CharTypeHanzi           = 3
	CharTypeSpaceLike       = 4
	CharTypeFullwidthLetter = 5
	CharTypeFullwidthDigit  = 6
	CharTypeOther           = 7
	CharTypeSurrogate       = 8
)

// Token type constants used by the SmartChineseAnalyzer.
//
// These mirror org.apache.lucene.analysis.cn.smart.WordType.
const (
	WordTypeSentenceBegin   = 0
	WordTypeSentenceEnd     = 1
	WordTypeChineseWord     = 2
	WordTypeString          = 3
	WordTypeNumber          = 4
	WordTypeDelimiter       = 5
	WordTypeFullwidthString = 6
	WordTypeFullwidthNumber = 7
)

// Utility constants mirroring org.apache.lucene.analysis.cn.smart.Utility.
var (
	StringCharArray = []rune("未##串")
	NumberCharArray = []rune("未##数")
	StartCharArray  = []rune("始##始")
	EndCharArray    = []rune("末##末")
	CommonDelimiter = []rune{','}
)

// MaxFrequence is the maximum bigram frequency used in smoothing.
const MaxFrequence = 2079997 + 80000

// GetCharType returns the CharType constant for a given rune.
func GetCharType(ch rune) int {
	if ch >= 0xD800 && ch <= 0xDFFF {
		return CharTypeSurrogate
	}
	if ch >= 0x4E00 && ch <= 0x9FA5 {
		return CharTypeHanzi
	}
	if (ch >= 0x0041 && ch <= 0x005A) || (ch >= 0x0061 && ch <= 0x007A) {
		return CharTypeLetter
	}
	if ch >= 0x0030 && ch <= 0x0039 {
		return CharTypeDigit
	}
	if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' || ch == '　' {
		return CharTypeSpaceLike
	}
	if (ch >= 0x0021 && ch <= 0x00BB) ||
		(ch >= 0x2010 && ch <= 0x2642) ||
		(ch >= 0x3001 && ch <= 0x301E) {
		return CharTypeDelimiter
	}
	if (ch >= 0xFF21 && ch <= 0xFF3A) || (ch >= 0xFF41 && ch <= 0xFF5A) {
		return CharTypeFullwidthLetter
	}
	if ch >= 0xFF10 && ch <= 0xFF19 {
		return CharTypeFullwidthDigit
	}
	if ch >= 0xFE30 && ch <= 0xFF63 {
		return CharTypeDelimiter
	}
	return CharTypeOther
}

// CompareArray compares two rune slices starting at the given offsets.
func CompareArray(larray []rune, lstartIndex int, rarray []rune, rstartIndex int) int {
	if larray == nil {
		if rarray == nil || rstartIndex >= len(rarray) {
			return 0
		}
		return -1
	}
	if rarray == nil {
		if lstartIndex >= len(larray) {
			return 0
		}
		return 1
	}
	li, ri := lstartIndex, rstartIndex
	for li < len(larray) && ri < len(rarray) && larray[li] == rarray[ri] {
		li++
		ri++
	}
	if li == len(larray) {
		if ri == len(rarray) {
			return 0
		}
		return -1
	}
	if ri == len(rarray) {
		return 1
	}
	if larray[li] > rarray[ri] {
		return 1
	}
	return -1
}

// CompareArrayByPrefix compares shortArray as a prefix of longArray.
func CompareArrayByPrefix(shortArray []rune, shortIndex int, longArray []rune, longIndex int) int {
	if shortArray == nil {
		return 0
	}
	if longArray == nil {
		if shortIndex < len(shortArray) {
			return 1
		}
		return 0
	}
	si, li := shortIndex, longIndex
	for si < len(shortArray) && li < len(longArray) && shortArray[si] == longArray[li] {
		si++
		li++
	}
	if si == len(shortArray) {
		return 0
	}
	if li == len(longArray) {
		return 1
	}
	if shortArray[si] > longArray[li] {
		return 1
	}
	return -1
}
