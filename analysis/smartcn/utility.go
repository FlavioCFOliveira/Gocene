// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package smartcn

// Go port of org.apache.lucene.analysis.cn.smart.Utility (Apache Lucene 10.4.0).

// StringCharArray is the placeholder char array for ASCII strings.
var StringCharArray = []rune("未##串")

// NumberCharArray is the placeholder char array for ASCII numbers.
var NumberCharArray = []rune("未##数")

// StartCharArray is the sentence-begin sentinel.
var StartCharArray = []rune("始##始")

// EndCharArray is the sentence-end sentinel.
var EndCharArray = []rune("末##末")

// CommonDelimiter is the rune to which all punctuation is normalised by
// SegTokenFilter.
var CommonDelimiter = []rune{','}

// Spaces contains the space-like characters that are skipped during
// segmentation.
const Spaces = " 　\t\r\n"

// MaxFrequence is the maximum bigram frequency used in smoothing.
const MaxFrequence = 2079997 + 80000

// CompareArray compares two rune slices starting at the given offsets.
//
// Returns 0 when equal, 1 when larray > rarray, -1 when larray < rarray.
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

// CompareArrayByPrefix compares shortArray (starting at shortIndex) against
// longArray (starting at longIndex) treating shortArray as a prefix.
//
// Returns 0 if shortArray is a prefix of longArray; otherwise behaves as
// CompareArray.
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
