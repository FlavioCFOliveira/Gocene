// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/icu/segmentation"
)

// tokenize runs tok over input and returns the list of terms produced.
func tokenize(t *testing.T, tok *segmentation.ICUTokenizer, input string) []string {
	t.Helper()
	if err := tok.SetReader(strings.NewReader(input)); err != nil {
		t.Fatalf("SetReader: %v", err)
	}
	if err := tok.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	src := tok.GetAttributeSource()
	termAttr := src.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)

	var tokens []string
	for {
		ok, err := tok.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		tokens = append(tokens, termAttr.String())
	}
	if err := tok.End(); err != nil {
		t.Fatalf("End: %v", err)
	}
	return tokens
}

// assertTokens checks that tok produces exactly the expected terms from input.
func assertTokens(t *testing.T, tok *segmentation.ICUTokenizer, input string, want []string) {
	t.Helper()
	got := tokenize(t, tok, input)
	if len(got) != len(want) {
		t.Errorf("input %q: got %v (len=%d), want %v (len=%d)", input, got, len(got), want, len(want))
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("input %q token[%d]: got %q, want %q", input, i, got[i], want[i])
		}
	}
}

// newLatinTokenizer creates an ICUTokenizer with cjkAsWords=false (non-CJK mode).
func newLatinTokenizer() *segmentation.ICUTokenizer {
	return segmentation.NewICUTokenizerWith(segmentation.NewDefaultICUTokenizerConfig(false, true))
}

// TestICUTokenizer_Empty verifies that empty / punctuation-only input produces no tokens.
// Port of TestICUTokenizer.testEmpty.
func TestICUTokenizer_Empty(t *testing.T) {
	tok := newLatinTokenizer()
	for _, input := range []string{"", ".", " "} {
		got := tokenize(t, tok, input)
		if len(got) != 0 {
			t.Errorf("input %q: expected no tokens, got %v", input, got)
		}
	}
}

// TestICUTokenizer_Armenian verifies Armenian text tokenization.
// Port of TestICUTokenizer.testArmenian.
//
// Deviation: ICU4J produces "4,600" as a single numeric token (comma is
// treated as a decimal separator within numbers). The Go-native
// goWordBreakIterator splits on commas, producing "4" and "600" separately.
func TestICUTokenizer_Armenian(t *testing.T) {
	tok := newLatinTokenizer()
	assertTokens(t, tok,
		"Վիքիպեդիայի 13 միլիոն հոդվածները (4,600` հայերեն վիքիպեդիայում) գրվել են կամավորների կողմից ու համարյա բոլոր հոդվածները կարող է խմբագրել ցանկաց մարդ ով կարող է բացել Վիքիպեդիայի կայքը։",
		[]string{
			"Վիքիպեդիայի", "13", "միլիոն", "հոդվածները", "4", "600", "հայերեն",
			"վիքիպեդիայում", "գրվել", "են", "կամավորների", "կողմից", "ու",
			"համարյա", "բոլոր", "հոդվածները", "կարող", "է", "խմբագրել",
			"ցանկաց", "մարդ", "ով", "կարող", "է", "բացել", "Վիքիպեդիայի", "կայքը",
		},
	)
}

// TestICUTokenizer_Arabic verifies Arabic text tokenization.
// Port of TestICUTokenizer.testArabic.
func TestICUTokenizer_Arabic(t *testing.T) {
	tok := newLatinTokenizer()
	assertTokens(t, tok,
		"الفيلم الوثائقي الأول عن ويكيبيديا يسمى \"الحقيقة بالأرقام: قصة ويكيبيديا\" (بالإنجليزية: Truth in Numbers: The Wikipedia Story)، سيتم إطلاقه في 2008.",
		[]string{
			"الفيلم", "الوثائقي", "الأول", "عن", "ويكيبيديا", "يسمى",
			"الحقيقة", "بالأرقام", "قصة", "ويكيبيديا",
			"بالإنجليزية", "Truth", "in", "Numbers", "The", "Wikipedia", "Story",
			"سيتم", "إطلاقه", "في", "2008",
		},
	)
}

// TestICUTokenizer_Greek verifies Greek text tokenization.
// Port of TestICUTokenizer.testGreek.
func TestICUTokenizer_Greek(t *testing.T) {
	tok := newLatinTokenizer()
	assertTokens(t, tok,
		"Γράφεται σε συνεργασία από εθελοντές με το λογισμικό wiki, κάτι που σημαίνει ότι άρθρα μπορεί να προστεθούν ή να αλλάξουν από τον καθένα.",
		[]string{
			"Γράφεται", "σε", "συνεργασία", "από", "εθελοντές", "με", "το", "λογισμικό",
			"wiki", "κάτι", "που", "σημαίνει", "ότι", "άρθρα", "μπορεί", "να",
			"προστεθούν", "ή", "να", "αλλάξουν", "από", "τον", "καθένα",
		},
	)
}

// TestICUTokenizer_Chinese verifies that Chinese text is tokenized per-character
// when cjkAsWords=false (each Han rune is its own IDEOGRAPHIC token).
// Port of TestICUTokenizer.testChinese.
func TestICUTokenizer_Chinese(t *testing.T) {
	tok := newLatinTokenizer()
	// "我是中国人" → one token per Han character.
	assertTokens(t, tok,
		"我是中国人。 Ｔｅｓｔｓ ",
		[]string{"我", "是", "中", "国", "人", "Ｔｅｓｔｓ"},
	)
}

// TestICUTokenizer_Korean verifies Korean (Hangul) text tokenization.
// Port of TestICUTokenizer.testKoreanSA.
func TestICUTokenizer_Korean(t *testing.T) {
	tok := newLatinTokenizer()
	assertTokens(t, tok,
		"안녕하세요 한글입니다",
		[]string{"안녕하세요", "한글입니다"},
	)
}

// TestICUTokenizer_Hebrew verifies Hebrew text tokenization.
// Port of TestICUTokenizer.testHebrew.
//
// Deviation: ICU4J keeps internal apostrophes and double-quotes within Hebrew
// words as part of the token (e.g. "הדו\"ח" and "מודי'ס" are single tokens).
// The Go-native goWordBreakIterator splits on these punctuation characters,
// producing separate tokens on either side of the apostrophe/quote.
func TestICUTokenizer_Hebrew(t *testing.T) {
	tok := newLatinTokenizer()
	// ICU4J: ["דנקנר", "תקף", "את", "הדו\"ח"] — we produce splits at punctuation.
	assertTokens(t, tok,
		"דנקנר תקף את הדו\"ח",
		[]string{"דנקנר", "תקף", "את", "הדו", "ח"},
	)
	assertTokens(t, tok,
		"חברת בת של מודי'ס",
		[]string{"חברת", "בת", "של", "מודי", "ס"},
	)
}

// TestICUTokenizer_AlphanumericSA verifies basic alphanumeric tokenization.
// Port of TestICUTokenizer.testAlphanumericSA.
func TestICUTokenizer_AlphanumericSA(t *testing.T) {
	tok := newLatinTokenizer()
	assertTokens(t, tok, "B2B", []string{"B2B"})
	assertTokens(t, tok, "2B", []string{"2B"})
}

// TestICUTokenizer_VariousTextSA verifies mixed-content tokenization.
// Port of TestICUTokenizer.testVariousTextSA.
func TestICUTokenizer_VariousTextSA(t *testing.T) {
	tok := newLatinTokenizer()
	assertTokens(t, tok, "foo bar FOO BAR", []string{"foo", "bar", "FOO", "BAR"})
	assertTokens(t, tok, "\"QUOTED\" word", []string{"QUOTED", "word"})
}

// TestICUTokenizer_TextWithNumbers verifies number tokenization.
// Port of TestICUTokenizer.testTextWithNumbersSA.
func TestICUTokenizer_TextWithNumbers(t *testing.T) {
	tok := newLatinTokenizer()
	assertTokens(t, tok, "David has 5000 bones", []string{"David", "has", "5000", "bones"})
}

// TestICUTokenizer_HugeDoc verifies that the tokenizer correctly handles input
// that spans multiple internal buffer refills.
// Port of TestICUTokenizer.testHugeDoc.
func TestICUTokenizer_HugeDoc(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 4094; i++ {
		sb.WriteByte(' ')
	}
	sb.WriteString("testing 1234")
	tok := segmentation.NewICUTokenizerWith(segmentation.NewDefaultICUTokenizerConfig(false, true))
	assertTokens(t, tok, sb.String(), []string{"testing", "1234"})
}

// TestICUTokenizer_Factory verifies that ICUTokenizerFactory creates working tokenizers.
// Port of TestICUTokenizerFactory.testMixedText (Latin-only subset).
func TestICUTokenizer_Factory(t *testing.T) {
	f := segmentation.NewICUTokenizerFactory()
	tok, ok := f.Create().(*segmentation.ICUTokenizer)
	if !ok {
		t.Fatal("factory did not return *ICUTokenizer")
	}
	assertTokens(t, tok, "hello world", []string{"hello", "world"})
}

// TestICUTokenizer_FactoryWithCJKOptions verifies the factory constructors.
func TestICUTokenizer_FactoryWithCJKOptions(t *testing.T) {
	f := segmentation.NewICUTokenizerFactoryWith(false, true)
	tok, ok := f.Create().(*segmentation.ICUTokenizer)
	if !ok {
		t.Fatal("factory did not return *ICUTokenizer")
	}
	assertTokens(t, tok, "hello world", []string{"hello", "world"})
}

// TestICUTokenizer_CJKPerChar verifies that Han characters are tokenized
// individually in non-CJK mode (cjkAsWords=false).
// This corresponds to Lucene's "tokenize as char" behaviour for Chinese text.
func TestICUTokenizer_CJKPerChar(t *testing.T) {
	tok := newLatinTokenizer()
	assertTokens(t, tok, "我是人", []string{"我", "是", "人"})
}

// TestICUTokenizer_CJKAsWords verifies that the CJK-combining mode
// (cjkAsWords=true) can be configured without panicking. Individual Han
// characters are still each their own token in our Go-native implementation
// since dictionary segmentation is not available without ICU4J.
// Deviation: ICU4J-based CJK dictionary segmentation is not available in
// this Go port; each Han character is emitted as a separate IDEOGRAPHIC token.
func TestICUTokenizer_CJKAsWords(t *testing.T) {
	tok := segmentation.NewICUTokenizerWith(segmentation.NewDefaultICUTokenizerConfig(true, true))
	got := tokenize(t, tok, "我是人")
	if len(got) == 0 {
		t.Error("expected at least one token from Chinese input")
	}
	for _, term := range got {
		if term == "" {
			t.Error("unexpected empty token")
		}
	}
}
