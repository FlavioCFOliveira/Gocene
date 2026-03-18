// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
	"unicode"
)

// CJKStopWords contains common CJK (Chinese, Japanese, Korean) stop words.
var CJKStopWords = []string{
	// Chinese
	"的", "了", "在", "是", "我", "有", "和", "就", "不", "人", "都", "一", "一个", "上", "也",
	"很", "到", "说", "要", "去", "你", "会", "着", "没有", "看", "好", "自己", "这", "那",
	// Japanese
	"の", "に", "は", "を", "た", "が", "で", "て", "と", "し", "れ", "さ", "ある", "いる",
	"も", "する", "から", "な", "こと", "として", "い", "や", "れる", "など", "なっ", "ない",
	// Korean
	"의", "가", "이", "은", "는", "을", "를", "에", "와", "과", "로", "으로", "에서",
	"하고", "한", "하다", "있다", "되다", "이다", "그", "이", "저", "것", "수", "등",
}

// CJKAnalyzer is an analyzer for CJK (Chinese, Japanese, Korean) language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.cjk.CJKAnalyzer.
//
// CJKAnalyzer uses the CJKTokenizer which tokenizes text by breaking CJK characters
// into bigrams (pairs of consecutive characters). This is a simple but effective
// approach for CJK text indexing.
type CJKAnalyzer struct {
	*BaseAnalyzer

	// stopWords is the set of stop words to filter
	stopWords *CharArraySet
}

// NewCJKAnalyzer creates a new CJKAnalyzer with default CJK stop words.
func NewCJKAnalyzer() *CJKAnalyzer {
	stopSet := GetWordSetFromStrings(CJKStopWords, true)
	return NewCJKAnalyzerWithWords(stopSet)
}

// NewCJKAnalyzerWithWords creates a CJKAnalyzer with custom stop words.
func NewCJKAnalyzerWithWords(stopWords *CharArraySet) *CJKAnalyzer {
	a := &CJKAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}

	// Set up the analysis chain
	a.TokenizerFactory = NewCJKTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))

	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *CJKAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *CJKAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *CJKAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

// Ensure CJKAnalyzer implements Analyzer
var _ Analyzer = (*CJKAnalyzer)(nil)
var _ AnalyzerInterface = (*CJKAnalyzer)(nil)

// CJKTokenizer tokenizes CJK text into bigrams.
//
// This tokenizer breaks CJK characters into pairs of consecutive characters
// (bigrams). For non-CJK text, it uses standard tokenization.
type CJKTokenizer struct {
	*BaseTokenizer

	// input holds the current input
	input []rune

	// position is the current position in the input
	position int

	// length is the length of the input
	length int
}

// NewCJKTokenizer creates a new CJKTokenizer.
func NewCJKTokenizer() *CJKTokenizer {
	t := &CJKTokenizer{}
	t.BaseTokenizer = NewBaseTokenizer()

	// Add attributes
	t.AddAttribute(NewCharTermAttribute())
	t.AddAttribute(NewOffsetAttribute())
	t.AddAttribute(NewPositionIncrementAttribute())

	return t
}

// SetReader sets the input reader.
func (t *CJKTokenizer) SetReader(reader io.Reader) error {
	if err := t.BaseTokenizer.SetReader(reader); err != nil {
		return err
	}

	// Read all input
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	t.input = []rune(string(data))
	t.position = 0
	t.length = len(t.input)

	return nil
}

// IncrementToken processes the next token.
func (t *CJKTokenizer) IncrementToken() (bool, error) {
	// Skip whitespace
	for t.position < t.length && unicode.IsSpace(t.input[t.position]) {
		t.position++
	}

	if t.position >= t.length {
		return false, nil
	}

	// Check if current character is CJK
	if isCJK(t.input[t.position]) {
		// Create bigram
		start := t.position
		end := start + 2
		if end > t.length {
			end = t.length
		}

		// Set token text
		tokenText := string(t.input[start:end])
		if termAttr := t.GetAttributeSource().GetAttribute("CharTermAttribute"); termAttr != nil {
			if cta, ok := termAttr.(CharTermAttribute); ok {
				cta.SetEmpty()
				cta.AppendString(tokenText)
			}
		}

		// Set offset
		if offsetAttr := t.GetAttributeSource().GetAttribute("OffsetAttribute"); offsetAttr != nil {
			if oa, ok := offsetAttr.(OffsetAttribute); ok {
				oa.SetStartOffset(start)
				oa.SetEndOffset(end)
			}
		}

		t.position++
		return true, nil
	}

	// Non-CJK: read until next CJK or whitespace
	start := t.position
	for t.position < t.length && !isCJK(t.input[t.position]) && !unicode.IsSpace(t.input[t.position]) {
		t.position++
	}

	if t.position > start {
		// Set token text
		tokenText := string(t.input[start:t.position])
		if termAttr := t.GetAttributeSource().GetAttribute("CharTermAttribute"); termAttr != nil {
			if cta, ok := termAttr.(CharTermAttribute); ok {
				cta.SetEmpty()
				cta.AppendString(tokenText)
			}
		}

		// Set offset
		if offsetAttr := t.GetAttributeSource().GetAttribute("OffsetAttribute"); offsetAttr != nil {
			if oa, ok := offsetAttr.(OffsetAttribute); ok {
				oa.SetStartOffset(start)
				oa.SetEndOffset(t.position)
			}
		}

		return true, nil
	}

	return false, nil
}

// Reset resets the tokenizer.
func (t *CJKTokenizer) Reset() error {
	t.position = 0
	return nil
}

// isCJK checks if a rune is a CJK character.
func isCJK(r rune) bool {
	// CJK Unified Ideographs
	if r >= 0x4E00 && r <= 0x9FFF {
		return true
	}
	// CJK Unified Ideographs Extension A
	if r >= 0x3400 && r <= 0x4DBF {
		return true
	}
	// Hiragana
	if r >= 0x3040 && r <= 0x309F {
		return true
	}
	// Katakana
	if r >= 0x30A0 && r <= 0x30FF {
		return true
	}
	// Hangul Syllables
	if r >= 0xAC00 && r <= 0xD7AF {
		return true
	}
	// Hangul Jamo
	if r >= 0x1100 && r <= 0x11FF {
		return true
	}
	// CJK Symbols and Punctuation
	if r >= 0x3000 && r <= 0x303F {
		return true
	}
	return false
}

// CJKTokenizerFactory creates CJKTokenizer instances.
type CJKTokenizerFactory struct{}

// NewCJKTokenizerFactory creates a new CJKTokenizerFactory.
func NewCJKTokenizerFactory() *CJKTokenizerFactory {
	return &CJKTokenizerFactory{}
}

// Create creates a new CJKTokenizer.
func (f *CJKTokenizerFactory) Create() Tokenizer {
	return NewCJKTokenizer()
}

// Ensure CJKTokenizerFactory implements TokenizerFactory
var _ TokenizerFactory = (*CJKTokenizerFactory)(nil)

// CJKAnalyzerFactory creates CJKAnalyzer instances.
type CJKAnalyzerFactory struct {
	stopWords *CharArraySet
}

// NewCJKAnalyzerFactory creates a new CJKAnalyzerFactory with default stop words.
func NewCJKAnalyzerFactory() *CJKAnalyzerFactory {
	return &CJKAnalyzerFactory{
		stopWords: GetWordSetFromStrings(CJKStopWords, true),
	}
}

// NewCJKAnalyzerFactoryWithWords creates a new CJKAnalyzerFactory with custom stop words.
func NewCJKAnalyzerFactoryWithWords(stopWords *CharArraySet) *CJKAnalyzerFactory {
	return &CJKAnalyzerFactory{
		stopWords: stopWords,
	}
}

// Create creates a new CJKAnalyzer.
func (f *CJKAnalyzerFactory) Create() AnalyzerInterface {
	return NewCJKAnalyzerWithWords(f.stopWords)
}

// Ensure CJKAnalyzerFactory implements AnalyzerFactory
var _ AnalyzerFactory = (*CJKAnalyzerFactory)(nil)
