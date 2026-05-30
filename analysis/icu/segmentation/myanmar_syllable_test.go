// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis/icu/segmentation"
)

// newMyanmarSyllableTokenizer creates an ICUTokenizer in Myanmar syllable mode
// (cjkAsWords=false, myanmarAsWords=false), matching the Java setUp of
// org.apache.lucene.analysis.icu.segmentation.TestMyanmarSyllable, which uses
// new DefaultICUTokenizerConfig(false, false).
func newMyanmarSyllableTokenizer() *segmentation.ICUTokenizer {
	return segmentation.NewICUTokenizerWith(segmentation.NewDefaultICUTokenizerConfig(false, false))
}

// TestMyanmarSyllable ports org.apache.lucene.analysis.icu.segmentation.
// TestMyanmarSyllable. These boundaries are produced by executing the compiled
// MyanmarSyllable.brk rules (RBBIBreakIterator), and match ICU4J exactly.
func TestMyanmarSyllable(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		// dictionary break would be သက်ဝင်|လှုပ်ရှား|စေ|ပြီး; syllable break is:
		{"basics", "သက်ဝင်လှုပ်ရှားစေပြီး", []string{"သက်", "ဝင်", "လှုပ်", "ရှား", "စေ", "ပြီး"}},
		{"C", "ကက", []string{"က", "က"}},
		{"CF", "ကံကံ", []string{"ကံ", "ကံ"}},
		{"CCA", "ကင်ကင်", []string{"ကင်", "ကင်"}},
		{"CCAF", "ကင်းကင်း", []string{"ကင်း", "ကင်း"}},
		{"CV", "ကာကာ", []string{"ကာ", "ကာ"}},
		{"CVF", "ကားကား", []string{"ကား", "ကား"}},
		{"CVVA", "ကော်ကော်", []string{"ကော်", "ကော်"}},
		{"CVVCA", "ကောင်ကောင်", []string{"ကောင်", "ကောင်"}},
		{"CVVCAF", "ကောင်းကောင်း", []string{"ကောင်း", "ကောင်း"}},
		{"CM", "ကျကျ", []string{"ကျ", "ကျ"}},
		{"CMF", "ကျံကျံ", []string{"ကျံ", "ကျံ"}},
		{"CMCA", "ကျင်ကျင်", []string{"ကျင်", "ကျင်"}},
		{"CMCAF", "ကျင်းကျင်း", []string{"ကျင်း", "ကျင်း"}},
		{"CMV", "ကျာကျာ", []string{"ကျာ", "ကျာ"}},
		{"CMVF", "ကျားကျား", []string{"ကျား", "ကျား"}},
		{"CMVVA", "ကျော်ကျော်", []string{"ကျော်", "ကျော်"}},
		{"CMVVCA", "ကြောင်ကြောင်", []string{"ကြောင်", "ကြောင်"}},
		{"CMVVCAF", "ကြောင်းကြောင်း", []string{"ကြောင်း", "ကြောင်း"}},
		{"I", "ဪဪ", []string{"ဪ", "ဪ"}},
		{"E", "ဣဣ", []string{"ဣ", "ဣ"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tok := newMyanmarSyllableTokenizer()
			assertTokens(t, tok, tc.in, tc.want)
		})
	}
}
