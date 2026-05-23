// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hhmm


// SegTokenFilter normalises SegTokens: converts full-width Latin to
// half-width, lowercases Latin, and converts all punctuation to
// CommonDelimiter.
//
// Go port of org.apache.lucene.analysis.cn.smart.hhmm.SegTokenFilter.
type SegTokenFilter struct{}

// NewSegTokenFilter creates a new SegTokenFilter.
func NewSegTokenFilter() *SegTokenFilter {
	return &SegTokenFilter{}
}

// Filter normalises the input token in place and returns it.
func (f *SegTokenFilter) Filter(token *SegToken) *SegToken {
	switch token.WordType {
	case WordTypeFullwidthNumber, WordTypeFullwidthString:
		// Convert full-width → half-width, then lowercase Latin.
		for i, ch := range token.CharArray {
			if ch >= 0xFF10 {
				token.CharArray[i] -= 0xFEE0
				ch = token.CharArray[i]
			}
			if ch >= 0x0041 && ch <= 0x005A {
				token.CharArray[i] += 0x0020
			}
		}
	case WordTypeString:
		// Lowercase Latin.
		for i, ch := range token.CharArray {
			if ch >= 0x0041 && ch <= 0x005A {
				token.CharArray[i] += 0x0020
			}
		}
	case WordTypeDelimiter:
		// All punctuation → CommonDelimiter.
		token.CharArray = CommonDelimiter
	}
	return token
}
