// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation_test

import "testing"

// TestICUTokenizerCJK exercises CJK segmentation through the compiled
// Default.brk rules (RBBIBreakIterator). With cjkAsWords=false (the default
// non-combined mode) the Default.brk word-break rules emit one token per Han
// ideograph — matching Lucene's ICUTokenizer behaviour for Chinese text.
//
// Deviation: ICU4J's dictionary-based CJK *word* segmentation (the
// UScript.JAPANESE / BreakIterator.getWordInstance path, reached when
// cjkAsWords=true) has no CGO-free Go equivalent. The dictionary-driven tests
// from org.apache.lucene.analysis.icu.segmentation.TestICUTokenizerCJK
// (testSimpleChinese, testTraditionalChinese, testSimpleJapanese, testKorean,
// etc.) require that dictionary and remain unported. They are also marked
// @AwaitsFix(LUCENE-8222) upstream. See follow-up rmp task for the CJK
// dictionary word iterator.
func TestICUTokenizerCJK(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"simple chinese per char", "我是中国人", []string{"我", "是", "中", "国", "人"}},
		{"chinese with latin", "中文ABC", []string{"中", "文", "ABC"}},
		{"chinese numerics", "2009年", []string{"2009", "年"}},
		{"korean words", "안녕하세요 한글입니다", []string{"안녕하세요", "한글입니다"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tok := newLatinTokenizer() // cjkAsWords=false
			assertTokens(t, tok, tc.in, tc.want)
		})
	}
}
