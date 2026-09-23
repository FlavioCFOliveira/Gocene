// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis_test

// Port of lucene/core/src/test/org/apache/lucene/analysis/standard/TestStandardAnalyzer.java
// (Apache Lucene 10.5.0), including its package-private helper class
// SpoonFeedMaxCharsReaderWrapper.
//
// Not ported, because they depend on test-framework components Gocene has
// not ported:
//   - testUnicodeWordBreaks (WordBreakTestUnicode_12_1_0);
//   - testUnicodeEmojiTests (EmojiTokenizationTestUnicode_12_1);
//   - testRandomStrings, testRandomHugeStrings, testRandomHugeStringsGraphAfter
//     (BaseTokenStreamTestCase.checkRandomData).
//
// BaseTokenStreamTestCase.assertAnalyzesTo first runs checkResetException
// and checkAnalysisConsistency, which are not ported; the helper below
// performs the assertTokenStreamContents part, with finalOffset =
// input.length() (UTF-16 code units) as Java passes it.
// LuceneTestCase.newAttributeFactory() randomises the attribute factory; that
// randomisation is not ported, so the default factory is used.

import (
	"bytes"
	"io"
	"math/rand/v2"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/api"
	"github.com/FlavioCFOliveira/Gocene/analysis/testutil"
)

func newStandardAnalyzerTestRandom(t *testing.T) *rand.Rand {
	t.Helper()
	seed := uint64(time.Now().UnixNano())
	t.Logf("random seed: %d", seed)
	return rand.New(rand.NewPCG(seed, 0x2545F4914F6CDD1D))
}

// stringAnalyzer is the part of Analyzer that tokenStream(String, String)
// maps to in Gocene.
type stringAnalyzer interface {
	api.Analyzer
	TokenStreamFromString(fieldName, text string) (api.TokenStream, error)
}

// newStandardTokenizerTestAnalyzer mirrors the Analyzer built in setUp(): a
// StandardTokenizer as the only component.
func newStandardTokenizerTestAnalyzer() *analysis.BaseAnalyzer {
	a := analysis.NewAnalyzer(analysis.GlobalReuseStrategy)
	a.CreateComponents = func(fieldName string) *analysis.TokenStreamComponents {
		tokenizer := analysis.NewStandardTokenizer()
		return &analysis.TokenStreamComponents{
			Source: func(r io.Reader) error {
				tokenizer.SetReader(r)
				return nil
			},
			Sink: tokenizer,
		}
	}
	return a
}

// utf16Length mirrors String.length().
func utf16Length(s string) int {
	return len(utf16.Encode([]rune(s)))
}

// assertAnalyzesTo mirrors the BaseTokenStreamTestCase.assertAnalyzesTo
// overloads used by this class: (a, input, output), (a, input, output,
// types) and (a, input, output, startOffsets, endOffsets).
func assertAnalyzesTo(t *testing.T, a stringAnalyzer, input string, output []string, extras ...any) {
	t.Helper()
	want := testutil.TokenStreamExpectations{Terms: output}
	switch len(extras) {
	case 0:
	case 1:
		want.Types = extras[0].([]string)
	case 2:
		want.StartOffsets = extras[0].([]int)
		want.EndOffsets = extras[1].([]int)
	default:
		t.Fatalf("assertAnalyzesTo: unsupported overload with %d extra arguments", len(extras))
	}
	want.FinalOffset = testutil.IntPtr(utf16Length(input))
	ts, err := a.TokenStreamFromString("dummy", input)
	if err != nil {
		t.Fatalf("tokenStream: %v", err)
	}
	testutil.AssertTokenStreamContents(t, ts, want.WithGraphOffsetsAreCorrect(true))
}

// checkOneTerm mirrors BaseTokenStreamTestCase.checkOneTerm.
func checkOneTerm(t *testing.T, a stringAnalyzer, input, expected string) {
	t.Helper()
	assertAnalyzesTo(t, a, input, []string{expected})
}

// LUCENE-5897: slow tokenization of strings of the form
// (\p{WB:ExtendNumLet}[\p{WB:Format}\p{WB:Extend}]*)+
func TestStandardAnalyzer_LargePartiallyMatchingToken(t *testing.T) {
	random := newStandardAnalyzerTestRandom(t)
	// TODO: get these lists of chars matching a property from ICU4J
	// http://www.unicode.org/Public/6.3.0/ucd/auxiliary/WordBreakProperty.txt
	wordBreakExtendNumLetChars := []rune("_‿⁀⁔︳︴﹍﹎﹏＿")

	// http://www.unicode.org/Public/6.3.0/ucd/auxiliary/WordBreakProperty.txt
	wordBreakFormatChars := []rune{ // only the first char in ranges
		0xAD, 0x600, 0x61C, 0x6DD, 0x70F, 0x180E, 0x200E, 0x202A, 0x2060, 0x2066, 0xFEFF, 0xFFF9,
		0x110BD, 0x1D173, 0xE0001, 0xE0020,
	}

	// http://www.unicode.org/Public/6.3.0/ucd/auxiliary/WordBreakProperty.txt
	wordBreakExtendChars := []rune{ // only the first char in ranges
		0x300, 0x483, 0x591, 0x5bf, 0x5c1, 0x5c4, 0x5c7, 0x610, 0x64b, 0x670, 0x6d6, 0x6df, 0x6e7,
		0x6ea, 0x711, 0x730, 0x7a6, 0x7eb, 0x816, 0x81b, 0x825, 0x829, 0x859, 0x8e4, 0x900, 0x93a,
		0x93e, 0x951, 0x962, 0x981, 0x9bc, 0x9be, 0x9c7, 0x9cb, 0x9d7, 0x9e2, 0xa01, 0xa3c, 0xa3e,
		0xa47, 0xa4b, 0xa51, 0xa70, 0xa75, 0xa81, 0xabc, 0xabe, 0xac7, 0xacb, 0xae2, 0xb01, 0xb3c,
		0xb3e, 0xb47, 0xb4b, 0xb56, 0xb62, 0xb82, 0xbbe, 0xbc6, 0xbca, 0xbd7, 0xc01, 0xc3e, 0xc46,
		0xc4a, 0xc55, 0xc62, 0xc82, 0xcbc, 0xcbe, 0xcc6, 0xcca, 0xcd5, 0xce2, 0xd02, 0xd3e, 0xd46,
		0xd4a, 0xd57, 0xd62, 0xd82, 0xdca, 0xdcf, 0xdd6, 0xdd8, 0xdf2, 0xe31, 0xe34, 0xe47, 0xeb1,
		0xeb4, 0xebb, 0xec8, 0xf18, 0xf35, 0xf37, 0xf39, 0xf3e, 0xf71, 0xf86, 0xf8d, 0xf99, 0xfc6,
		0x102b, 0x1056, 0x105e, 0x1062, 0x1067, 0x1071, 0x1082, 0x108f, 0x109a, 0x135d, 0x1712,
		0x1732, 0x1752, 0x1772, 0x17b4, 0x17dd, 0x180b, 0x18a9, 0x1920, 0x1930, 0x19b0, 0x19c8,
		0x1a17, 0x1a55, 0x1a60, 0x1a7f, 0x1b00, 0x1b34, 0x1b6b, 0x1b80, 0x1ba1, 0x1be6, 0x1c24,
		0x1cd0, 0x1cd4, 0x1ced, 0x1cf2, 0x1dc0, 0x1dfc, 0x200c, 0x20d0, 0x2cef, 0x2d7f, 0x2de0,
		0x302a, 0x3099, 0xa66f, 0xa674, 0xa69f, 0xa6f0, 0xa802, 0xa806, 0xa80b, 0xa823, 0xa880,
		0xa8b4, 0xa8e0, 0xa926, 0xa947, 0xa980, 0xa9b3, 0xaa29, 0xaa43, 0xaa4c, 0xaa7b, 0xaab0,
		0xaab2, 0xaab7, 0xaabe, 0xaac1, 0xaaeb, 0xaaf5, 0xabe3, 0xabec, 0xfb1e, 0xfe00, 0xfe20,
		0xff9e, 0x101fd, 0x10a01, 0x10a05, 0x10a0C, 0x10a38, 0x10a3F, 0x11000, 0x11001, 0x11038,
		0x11080, 0x11082, 0x110b0, 0x110b3, 0x110b7, 0x110b9, 0x11100, 0x11127, 0x1112c, 0x11180,
		0x11182, 0x111b3, 0x111b6, 0x111bF, 0x116ab, 0x116ac, 0x116b0, 0x116b6, 0x16f51, 0x16f8f,
		0x1d165, 0x1d167, 0x1d16d, 0x1d17b, 0x1d185, 0x1d1aa, 0x1d242, 0xe0100,
	}

	var builder strings.Builder
	numChars := 100*1024 + random.IntN(1024*1024-100*1024+1) // TestUtil.nextInt(random(), 100 * 1024, 1024 * 1024)
	for i := 0; i < numChars; {
		builder.WriteRune(wordBreakExtendNumLetChars[random.IntN(len(wordBreakExtendNumLetChars))])
		i++
		if random.IntN(2) == 0 {
			numFormatExtendChars := 1 + random.IntN(8) // TestUtil.nextInt(random(), 1, 8)
			for j := 0; j < numFormatExtendChars; j++ {
				var codepoint rune
				if random.IntN(2) == 0 {
					codepoint = wordBreakFormatChars[random.IntN(len(wordBreakFormatChars))]
				} else {
					codepoint = wordBreakExtendChars[random.IntN(len(wordBreakExtendChars))]
				}
				builder.WriteRune(codepoint)
				// Character.toChars(codepoint).length
				if codepoint > 0xFFFF {
					i += 2
				} else {
					i++
				}
			}
		}
	}
	text := builder.String()
	ts := analysis.NewStandardTokenizer()
	ts.SetReader(strings.NewReader(text))
	drainTokenizer(t, ts)

	newBufferSize := 200 + random.IntN(8192-200+1)              // TestUtil.nextInt(random(), 200, 8192)
	if err := ts.SetMaxTokenLength(newBufferSize); err != nil { // try a different buffer size
		t.Fatalf("setMaxTokenLength: %v", err)
	}
	ts.SetReader(strings.NewReader(text))
	drainTokenizer(t, ts)
}

// drainTokenizer runs reset(); while (incrementToken()) {}; end(); close().
func drainTokenizer(t *testing.T, ts *analysis.StandardTokenizer) {
	t.Helper()
	if err := ts.Reset(); err != nil {
		t.Fatalf("reset: %v", err)
	}
	for {
		ok, err := ts.IncrementToken()
		if err != nil {
			t.Fatalf("incrementToken: %v", err)
		}
		if !ok {
			break
		}
	}
	if err := ts.End(); err != nil {
		t.Fatalf("end: %v", err)
	}
	if err := ts.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestStandardAnalyzer_HugeDoc(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(strings.Repeat(" ", 4094))
	sb.WriteString("testing 1234")
	input := sb.String()
	tokenizer := analysis.NewStandardTokenizer()
	tokenizer.SetReader(strings.NewReader(input))
	testutil.AssertTokenStreamContentsSimple(t, tokenizer, []string{"testing", "1234"})
}

func TestStandardAnalyzer_Armenian(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	assertAnalyzesTo(t,
		a,
		"Վիքիպեդիայի 13 միլիոն հոդվածները (4,600` հայերեն վիքիպեդիայում) գրվել են կամավորների կողմից ու համարյա բոլոր հոդվածները կարող է խմբագրել ցանկաց մարդ ով կարող է բացել Վիքիպեդիայի կայքը։",
		[]string{
			"Վիքիպեդիայի",
			"13",
			"միլիոն",
			"հոդվածները",
			"4,600",
			"հայերեն",
			"վիքիպեդիայում",
			"գրվել",
			"են",
			"կամավորների",
			"կողմից",
			"ու",
			"համարյա",
			"բոլոր",
			"հոդվածները",
			"կարող",
			"է",
			"խմբագրել",
			"ցանկաց",
			"մարդ",
			"ով",
			"կարող",
			"է",
			"բացել",
			"Վիքիպեդիայի",
			"կայքը",
		})
}

func TestStandardAnalyzer_Amharic(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	assertAnalyzesTo(t,
		a,
		"ዊኪፔድያ የባለ ብዙ ቋንቋ የተሟላ ትክክለኛና ነጻ መዝገበ ዕውቀት (ኢንሳይክሎፒዲያ) ነው። ማንኛውም",
		[]string{
			"ዊኪፔድያ",
			"የባለ",
			"ብዙ",
			"ቋንቋ",
			"የተሟላ",
			"ትክክለኛና",
			"ነጻ",
			"መዝገበ",
			"ዕውቀት",
			"ኢንሳይክሎፒዲያ",
			"ነው",
			"ማንኛውም",
		})
}

func TestStandardAnalyzer_Arabic(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	assertAnalyzesTo(t,
		a,
		"الفيلم الوثائقي الأول عن ويكيبيديا يسمى \"الحقيقة بالأرقام: قصة ويكيبيديا\" (بالإنجليزية: Truth in Numbers: The Wikipedia Story)، سيتم إطلاقه في 2008.",
		[]string{
			"الفيلم",
			"الوثائقي",
			"الأول",
			"عن",
			"ويكيبيديا",
			"يسمى",
			"الحقيقة",
			"بالأرقام",
			"قصة",
			"ويكيبيديا",
			"بالإنجليزية",
			"Truth",
			"in",
			"Numbers",
			"The",
			"Wikipedia",
			"Story",
			"سيتم",
			"إطلاقه",
			"في",
			"2008",
		})
}

func TestStandardAnalyzer_Aramaic(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	assertAnalyzesTo(t,
		a,
		"ܘܝܩܝܦܕܝܐ (ܐܢܓܠܝܐ: Wikipedia) ܗܘ ܐܝܢܣܩܠܘܦܕܝܐ ܚܐܪܬܐ ܕܐܢܛܪܢܛ ܒܠܫܢ̈ܐ ܣܓܝܐ̈ܐ܂ ܫܡܗ ܐܬܐ ܡܢ ܡ̈ܠܬܐ ܕ\"ܘܝܩܝ\" ܘ\"ܐܝܢܣܩܠܘܦܕܝܐ\"܀",
		[]string{
			"ܘܝܩܝܦܕܝܐ",
			"ܐܢܓܠܝܐ",
			"Wikipedia",
			"ܗܘ",
			"ܐܝܢܣܩܠܘܦܕܝܐ",
			"ܚܐܪܬܐ",
			"ܕܐܢܛܪܢܛ",
			"ܒܠܫܢ̈ܐ",
			"ܣܓܝܐ̈ܐ",
			"ܫܡܗ",
			"ܐܬܐ",
			"ܡܢ",
			"ܡ̈ܠܬܐ",
			"ܕ",
			"ܘܝܩܝ",
			"ܘ",
			"ܐܝܢܣܩܠܘܦܕܝܐ",
		})
}

func TestStandardAnalyzer_Bengali(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	assertAnalyzesTo(t,
		a,
		"এই বিশ্বকোষ পরিচালনা করে উইকিমিডিয়া ফাউন্ডেশন (একটি অলাভজনক সংস্থা)। উইকিপিডিয়ার শুরু ১৫ জানুয়ারি, ২০০১ সালে। এখন পর্যন্ত ২০০টিরও বেশী ভাষায় উইকিপিডিয়া রয়েছে।",
		[]string{
			"এই",
			"বিশ্বকোষ",
			"পরিচালনা",
			"করে",
			"উইকিমিডিয়া",
			"ফাউন্ডেশন",
			"একটি",
			"অলাভজনক",
			"সংস্থা",
			"উইকিপিডিয়ার",
			"শুরু",
			"১৫",
			"জানুয়ারি",
			"২০০১",
			"সালে",
			"এখন",
			"পর্যন্ত",
			"২০০টিরও",
			"বেশী",
			"ভাষায়",
			"উইকিপিডিয়া",
			"রয়েছে",
		})
}

func TestStandardAnalyzer_Farsi(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	assertAnalyzesTo(t,
		a,
		"ویکی پدیای انگلیسی در تاریخ ۲۵ دی ۱۳۷۹ به صورت مکملی برای دانشنامهٔ تخصصی نوپدیا نوشته شد.",
		[]string{
			"ویکی",
			"پدیای",
			"انگلیسی",
			"در",
			"تاریخ",
			"۲۵",
			"دی",
			"۱۳۷۹",
			"به",
			"صورت",
			"مکملی",
			"برای",
			"دانشنامهٔ",
			"تخصصی",
			"نوپدیا",
			"نوشته",
			"شد",
		})
}

func TestStandardAnalyzer_Greek(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	assertAnalyzesTo(t,
		a,
		"Γράφεται σε συνεργασία από εθελοντές με το λογισμικό wiki, κάτι που σημαίνει ότι άρθρα μπορεί να προστεθούν ή να αλλάξουν από τον καθένα.",
		[]string{
			"Γράφεται",
			"σε",
			"συνεργασία",
			"από",
			"εθελοντές",
			"με",
			"το",
			"λογισμικό",
			"wiki",
			"κάτι",
			"που",
			"σημαίνει",
			"ότι",
			"άρθρα",
			"μπορεί",
			"να",
			"προστεθούν",
			"ή",
			"να",
			"αλλάξουν",
			"από",
			"τον",
			"καθένα",
		})
}

func TestStandardAnalyzer_Thai(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	assertAnalyzesTo(t,
		a,
		"การที่ได้ต้องแสดงว่างานดี. แล้วเธอจะไปไหน? ๑๒๓๔",
		[]string{"การที่ได้ต้องแสดงว่างานดี", "แล้วเธอจะไปไหน", "๑๒๓๔"})
}

func TestStandardAnalyzer_Lao(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	assertAnalyzesTo(t,
		a,
		"ສາທາລະນະລັດ ປະຊາທິປະໄຕ ປະຊາຊົນລາວ",
		[]string{"ສາທາລະນະລັດ", "ປະຊາທິປະໄຕ", "ປະຊາຊົນລາວ"})
}

func TestStandardAnalyzer_Tibetan(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	assertAnalyzesTo(t,
		a,
		"སྣོན་མཛོད་དང་ལས་འདིས་བོད་ཡིག་མི་ཉམས་གོང་འཕེལ་དུ་གཏོང་བར་ཧ་ཅང་དགེ་མཚན་མཆིས་སོ། །",
		[]string{
			"སྣོན", "མཛོད", "དང", "ལས", "འདིས", "བོད", "ཡིག",
			"མི", "ཉམས", "གོང", "འཕེལ", "དུ", "གཏོང", "བར",
			"ཧ", "ཅང", "དགེ", "མཚན", "མཆིས", "སོ",
		})
}

// For chinese, tokenize as char (these can later form bigrams or whatever)
func TestStandardAnalyzer_Chinese(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	assertAnalyzesTo(t,
		a, "我是中国人。 １２３４ Ｔｅｓｔｓ ", []string{"我", "是", "中", "国", "人", "１２３４", "Ｔｅｓｔｓ"})
}

func TestStandardAnalyzer_Empty(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	assertAnalyzesTo(t, a, "", []string{})
	assertAnalyzesTo(t, a, ".", []string{})
	assertAnalyzesTo(t, a, " ", []string{})
}

// test various jira issues this analyzer is related to */
func TestStandardAnalyzer_LUCENE1545(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	/*
	 * Standard analyzer does not correctly tokenize combining character U+0364 COMBINING LATIN SMALL LETTRE E.
	 * The word "moͤchte" is incorrectly tokenized into "mo" "chte", the combining character is lost.
	 * Expected result is only on token "moͤchte".
	 */
	assertAnalyzesTo(t, a, "moͤchte", []string{"moͤchte"})
}

// Tests from StandardAnalyzer, just to show behavior is similar */
func TestStandardAnalyzer_AlphanumericSA(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	// alphanumeric tokens
	assertAnalyzesTo(t, a, "B2B", []string{"B2B"})
	assertAnalyzesTo(t, a, "2B", []string{"2B"})
}

func TestStandardAnalyzer_DelimitersSA(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	// other delimiters: "-", "/", ","
	assertAnalyzesTo(t,
		a, "some-dashed-phrase", []string{"some", "dashed", "phrase"})
	assertAnalyzesTo(t,
		a, "dogs,chase,cats", []string{"dogs", "chase", "cats"})
	assertAnalyzesTo(t, a, "ac/dc", []string{"ac", "dc"})
}

func TestStandardAnalyzer_ApostrophesSA(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	// internal apostrophes: O'Reilly, you're, O'Reilly's
	assertAnalyzesTo(t, a, "O'Reilly", []string{"O'Reilly"})
	assertAnalyzesTo(t, a, "you're", []string{"you're"})
	assertAnalyzesTo(t, a, "she's", []string{"she's"})
	assertAnalyzesTo(t, a, "Jim's", []string{"Jim's"})
	assertAnalyzesTo(t, a, "don't", []string{"don't"})
	assertAnalyzesTo(t, a, "O'Reilly's", []string{"O'Reilly's"})
}

func TestStandardAnalyzer_NumericSA(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	// floating point, serial, model numbers, ip addresses, etc.
	assertAnalyzesTo(t, a, "21.35", []string{"21.35"})
	assertAnalyzesTo(t, a, "R2D2 C3PO", []string{"R2D2", "C3PO"})
	assertAnalyzesTo(t, a, "216.239.63.104", []string{"216.239.63.104"})
	assertAnalyzesTo(t, a, "216.239.63.104", []string{"216.239.63.104"})
}

func TestStandardAnalyzer_TextWithNumbersSA(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	// numbers
	assertAnalyzesTo(t,
		a, "David has 5000 bones", []string{"David", "has", "5000", "bones"})
}

func TestStandardAnalyzer_VariousTextSA(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	// various
	assertAnalyzesTo(t,
		a, "C embedded developers wanted", []string{"C", "embedded", "developers", "wanted"})
	assertAnalyzesTo(t,
		a, "foo bar FOO BAR", []string{"foo", "bar", "FOO", "BAR"})
	assertAnalyzesTo(t,
		a, "foo      bar .  FOO <> BAR", []string{"foo", "bar", "FOO", "BAR"})
	assertAnalyzesTo(t, a, "\"QUOTED\" word", []string{"QUOTED", "word"})
}

func TestStandardAnalyzer_KoreanSA(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	// Korean words
	assertAnalyzesTo(t, a, "안녕하세요 한글입니다", []string{"안녕하세요", "한글입니다"})
}

func TestStandardAnalyzer_Offsets(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	assertAnalyzesTo(t,
		a,
		"David has 5000 bones",
		[]string{"David", "has", "5000", "bones"},
		[]int{0, 6, 10, 15},
		[]int{5, 9, 14, 20})
}

func TestStandardAnalyzer_Types(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	assertAnalyzesTo(t,
		a,
		"David has 5000 bones",
		[]string{"David", "has", "5000", "bones"},
		[]string{"<ALPHANUM>", "<ALPHANUM>", "<NUM>", "<ALPHANUM>"})
}

func TestStandardAnalyzer_Supplementary(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	assertAnalyzesTo(t,
		a,
		"𩬅艱鍟䇹愯瀛",
		[]string{"𩬅", "艱", "鍟", "䇹", "愯", "瀛"},
		[]string{
			"<IDEOGRAPHIC>",
			"<IDEOGRAPHIC>",
			"<IDEOGRAPHIC>",
			"<IDEOGRAPHIC>",
			"<IDEOGRAPHIC>",
			"<IDEOGRAPHIC>",
		})
}

func TestStandardAnalyzer_Korean(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	assertAnalyzesTo(t,
		a, "훈민정음", []string{"훈민정음"}, []string{"<HANGUL>"})
}

func TestStandardAnalyzer_Japanese(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	assertAnalyzesTo(t,
		a,
		"仮名遣い カタカナ",
		[]string{"仮", "名", "遣", "い", "カタカナ"},
		[]string{
			"<IDEOGRAPHIC>", "<IDEOGRAPHIC>", "<IDEOGRAPHIC>", "<HIRAGANA>", "<KATAKANA>",
		})
}

func TestStandardAnalyzer_CombiningMarks(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	checkOneTerm(t, a, "ざ", "ざ") // hiragana
	checkOneTerm(t, a, "ザ", "ザ") // katakana
	checkOneTerm(t, a, "壹゙", "壹゙") // ideographic
	checkOneTerm(t, a, "아゙", "아゙") // hangul
}

// Multiple consecutive chars in \p{WB:MidLetter}, \p{WB:MidNumLet}, and/or \p{MidNum} should
// trigger a token split.
func TestStandardAnalyzer_Mid(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	// ':' is in \p{WB:MidLetter}, which should trigger a split unless there is a Letter char on
	// both sides
	assertAnalyzesTo(t, a, "A:B", []string{"A:B"})
	assertAnalyzesTo(t, a, "A::B", []string{"A", "B"})

	// '.' is in \p{WB:MidNumLet}, which should trigger a split unless there is a Letter or Numeric
	// char on both sides
	assertAnalyzesTo(t, a, "1.2", []string{"1.2"})
	assertAnalyzesTo(t, a, "A.B", []string{"A.B"})
	assertAnalyzesTo(t, a, "1..2", []string{"1", "2"})
	assertAnalyzesTo(t, a, "A..B", []string{"A", "B"})

	// ',' is in \p{WB:MidNum}, which should trigger a split unless there is a Numeric char on both
	// sides
	assertAnalyzesTo(t, a, "1,2", []string{"1,2"})
	assertAnalyzesTo(t, a, "1,,2", []string{"1", "2"})

	// Mixed consecutive \p{WB:MidLetter} and \p{WB:MidNumLet} should trigger a split
	assertAnalyzesTo(t, a, "A.:B", []string{"A", "B"})
	assertAnalyzesTo(t, a, "A:.B", []string{"A", "B"})

	// Mixed consecutive \p{WB:MidNum} and \p{WB:MidNumLet} should trigger a split
	assertAnalyzesTo(t, a, "1,.2", []string{"1", "2"})
	assertAnalyzesTo(t, a, "1.,2", []string{"1", "2"})

	// '_' is in \p{WB:ExtendNumLet}

	assertAnalyzesTo(t, a, "A:B_A:B", []string{"A:B_A:B"})
	assertAnalyzesTo(t, a, "A:B_A::B", []string{"A:B_A", "B"})

	assertAnalyzesTo(t, a, "1.2_1.2", []string{"1.2_1.2"})
	assertAnalyzesTo(t, a, "A.B_A.B", []string{"A.B_A.B"})
	assertAnalyzesTo(t, a, "1.2_1..2", []string{"1.2_1", "2"})
	assertAnalyzesTo(t, a, "A.B_A..B", []string{"A.B_A", "B"})

	assertAnalyzesTo(t, a, "1,2_1,2", []string{"1,2_1,2"})
	assertAnalyzesTo(t, a, "1,2_1,,2", []string{"1,2_1", "2"})

	assertAnalyzesTo(t, a, "C_A.:B", []string{"C_A", "B"})
	assertAnalyzesTo(t, a, "C_A:.B", []string{"C_A", "B"})

	assertAnalyzesTo(t, a, "3_1,.2", []string{"3_1", "2"})
	assertAnalyzesTo(t, a, "3_1.,2", []string{"3_1", "2"})
}

// simple emoji */
func TestStandardAnalyzer_Emoji(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	assertAnalyzesTo(t,
		a,
		"💩 💩💩",
		[]string{"💩", "💩", "💩"},
		[]string{"<EMOJI>", "<EMOJI>", "<EMOJI>"})
}

// emoji zwj sequence */
func TestStandardAnalyzer_EmojiSequence(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	assertAnalyzesTo(t,
		a, "👩‍❤️‍👩", []string{"👩‍❤️‍👩"}, []string{"<EMOJI>"})
}

// emoji zwj sequence with fitzpatrick modifier */
func TestStandardAnalyzer_EmojiSequenceWithModifier(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	assertAnalyzesTo(t,
		a, "👨🏼‍⚕️", []string{"👨🏼‍⚕️"}, []string{"<EMOJI>"})
}

// regional indicator */
func TestStandardAnalyzer_EmojiRegionalIndicator(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	assertAnalyzesTo(t,
		a, "🇺🇸🇺🇸", []string{"🇺🇸", "🇺🇸"}, []string{"<EMOJI>", "<EMOJI>"})
}

// variation sequence */
func TestStandardAnalyzer_EmojiVariationSequence(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	assertAnalyzesTo(t,
		a, "#️⃣", []string{"#️⃣"}, []string{"<EMOJI>"})
	assertAnalyzesTo(t,
		a,
		"3️⃣",
		[]string{
			"3️⃣",
		},
		[]string{"<EMOJI>"})

	// text presentation sequences
	assertAnalyzesTo(t, a, "#\uFE0E", []string{}, []string{})
	assertAnalyzesTo(t,
		a,
		"3\uFE0E", // \uFE0E is included in \p{WB:Extend}
		[]string{
			"3\uFE0E",
		},
		[]string{"<NUM>"})
	assertAnalyzesTo(t,
		a,
		"\u2B55\uFE0E", // \u2B55 = HEAVY BLACK CIRCLE
		[]string{
			"\u2B55",
		},
		[]string{"<EMOJI>"})
	assertAnalyzesTo(t,
		a,
		"\u2B55\uFE0E\u200D\u2B55\uFE0E",
		[]string{"\u2B55", "\u200D\u2B55"},
		[]string{"<EMOJI>", "<EMOJI>"})
}

func TestStandardAnalyzer_EmojiTagSequence(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	assertAnalyzesTo(t,
		a, "🏴󠁧󠁢󠁥󠁮󠁧󠁿", []string{"🏴󠁧󠁢󠁥󠁮󠁧󠁿"}, []string{"<EMOJI>"})
}

func TestStandardAnalyzer_EmojiTokenization(t *testing.T) {
	a := newStandardTokenizerTestAnalyzer()
	defer a.Close()
	// simple emoji around latin
	assertAnalyzesTo(t,
		a,
		"poo💩poo",
		[]string{"poo", "💩", "poo"},
		[]string{"<ALPHANUM>", "<EMOJI>", "<ALPHANUM>"})
	// simple emoji around non-latin
	assertAnalyzesTo(t,
		a,
		"💩中國💩",
		[]string{"💩", "中", "國", "💩"},
		[]string{"<EMOJI>", "<IDEOGRAPHIC>", "<IDEOGRAPHIC>", "<EMOJI>"})
}

func TestStandardAnalyzer_Normalize(t *testing.T) {
	a := analysis.NewStandardAnalyzer()
	got, err := a.NormalizeText("dummy", "\"\\À3[]()! Cz@")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if want := []byte("\"\\à3[]()! cz@"); !bytes.Equal(want, got.ValidBytes()) {
		t.Fatalf("normalize: got %q, want %q", got.ValidBytes(), want)
	}
}

func TestStandardAnalyzer_MaxTokenLengthDefault(t *testing.T) {
	a := analysis.NewStandardAnalyzer()

	// exact max length:
	bString := strings.Repeat("b", analysis.StandardAnalyzerDefaultMaxTokenLength)
	// first bString is exact max default length; next one is 1 too long
	input := "x " + bString + " " + bString + "b"
	assertAnalyzesTo(t, a, input, []string{"x", bString, bString, "b"})
	if err := a.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestStandardAnalyzer_MaxTokenLengthNonDefault(t *testing.T) {
	a := analysis.NewStandardAnalyzer()
	a.SetMaxTokenLength(5)
	assertAnalyzesTo(t, a, "ab cd toolong xy z", []string{"ab", "cd", "toolo", "ng", "xy", "z"})
	if err := a.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestStandardAnalyzer_SplitSurrogatePairWithSpoonFeedReader(t *testing.T) {
	text := "12345678\U00010300" // U+D800 U+DF00 = U+10300 = 𐌀 (OLD ITALIC LETTER A)

	// Collect tokens with normal reader
	a := analysis.NewStandardAnalyzer()
	ts, err := a.TokenStreamFromString("dummy", text)
	if err != nil {
		t.Fatalf("tokenStream: %v", err)
	}
	var tokens []string
	termAtt := ts.GetAttributeSource().AddAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	mustNoErr(t, ts.Reset())
	for {
		ok, err := ts.IncrementToken()
		mustNoErr(t, err)
		if !ok {
			break
		}
		tokens = append(tokens, termAtt.String())
	}
	mustNoErr(t, ts.End())
	mustNoErr(t, ts.Close())

	// Tokens from a spoon-feed reader should be the same as from a normal
	// reader. The 9th unit is the first unit of the supplementary character,
	// so the 9-max spoon-feed reader will split it at a read boundary.
	reader := &spoonFeedMaxCharsReaderWrapper{maxChars: 9, in: strings.NewReader(text)}
	ts, err = a.TokenStream("dummy", reader)
	if err != nil {
		t.Fatalf("tokenStream: %v", err)
	}
	termAtt = ts.GetAttributeSource().AddAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	mustNoErr(t, ts.Reset())
	for tokenNum := 0; ; tokenNum++ {
		ok, err := ts.IncrementToken()
		mustNoErr(t, err)
		if !ok {
			break
		}
		if tokenNum >= len(tokens) {
			t.Fatalf("token #%d mismatch: extra token %q", tokenNum, termAtt.String())
		}
		if got := termAtt.String(); got != tokens[tokenNum] {
			t.Fatalf("token #%d mismatch: got %q, want %q", tokenNum, got, tokens[tokenNum])
		}
	}
	mustNoErr(t, ts.End())
	mustNoErr(t, ts.Close())
}

// spoonFeedMaxCharsReaderWrapper is the port of the package-private class
// SpoonFeedMaxCharsReaderWrapper: every read returns at most maxChars units.
// Java readers deliver UTF-16 units; Go readers deliver UTF-8 bytes, so the
// limit applies to bytes.
type spoonFeedMaxCharsReaderWrapper struct {
	in       io.Reader
	maxChars int
}

func (r *spoonFeedMaxCharsReaderWrapper) Close() error {
	if c, ok := r.in.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// Read returns the configured number of units if available.
func (r *spoonFeedMaxCharsReaderWrapper) Read(p []byte) (int, error) {
	return r.in.Read(p[:min(r.maxChars, len(p))])
}
