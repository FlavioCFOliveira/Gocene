// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// KoreanStopWords contains common Korean stop words.
var KoreanStopWords = []string{
	"의", "가", "이", "은", "는", "을", "를", "에", "와", "과", "로", "으로", "에서",
	"하고", "한", "하다", "있다", "되다", "이다", "그", "이", "저", "것", "수", "등",
	"및", "또는", "또한", "그리고", "그러나", "하지만", "그래서", "때문에", "따라서",
	"또", "그러면", "그런데", "그럼", "그러므로", "그래도", "그러니까", "그러니",
	"이것", "저것", "그것", "여기", "거기", "저기", "어디", "무엇", "누구", "언제",
	"어떻게", "왜", "얼마나", "몇", "모든", "각", "모두", "전체", "대부분", "일부",
	"일반", "특히", "주로", "보통", "거의", "매우", "너무", "아주", "정말", "참",
	"단지", "오직", "오로지", "다만", "물론", "반드시", "아마", "아마도", "혹시",
	"혹은", "예를", "들어", "예", "들면", "즉", "또는", "또한", "게다가", "더욱",
	"더", "가장", "최고", "최대", "최소", "최근", "처음", "마지막", "다음", "이전",
	"현재", "지금", "오늘", "내일", "어제", "그제", "모레", "항상", "자주", "가끔",
	"때때로", "종종", "이미", "벌써", "아직", "곧", "바로", "즉시", "당장", "갑자기",
	"천천히", "빨리", "느리게", "잘", "못", "안", "없이", "없는", "있는", "같은",
	"다른", "새로운", "오래된", "큰", "작은", "많은", "적은", "좋은", "나쁜",
	"높은", "낮은", "긴", "짧은", "넓은", "좁은", "깊은", "얕은", "어두운", "밝은",
	"무거운", "가벼운", "뜨거운", "차가운", "건조한", "젖은", "깨끗한", "더러운",
	"쉬운", "어려운", "빠른", "느린", "강한", "약한", "건강한", "아픈", "행복한",
	"슬픈", "기쁜", "화난", "두려운", "놀란", "피곤한", "졸린", "배고픈", "목마른",
}

// KoreanAnalyzer is an analyzer for Korean language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.ko.KoreanAnalyzer.
//
// KoreanAnalyzer uses the StandardTokenizer with Korean stop words removal.
// Note: For proper Korean text segmentation, a specialized tokenizer like
// Nori would be needed. This implementation provides basic support using
// the StandardTokenizer.
type KoreanAnalyzer struct {
	*BaseAnalyzer

	// stopWords is the set of stop words to filter
	stopWords *CharArraySet
}

// NewKoreanAnalyzer creates a new KoreanAnalyzer with default Korean stop words.
func NewKoreanAnalyzer() *KoreanAnalyzer {
	stopSet := GetWordSetFromStrings(KoreanStopWords, true)
	return NewKoreanAnalyzerWithWords(stopSet)
}

// NewKoreanAnalyzerWithWords creates a KoreanAnalyzer with custom stop words.
func NewKoreanAnalyzerWithWords(stopWords *CharArraySet) *KoreanAnalyzer {
	a := &KoreanAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}

	// Set up the analysis chain
	// Note: For proper Korean, a specialized tokenizer should be used
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))

	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *KoreanAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *KoreanAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *KoreanAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

// Ensure KoreanAnalyzer implements Analyzer
var _ Analyzer = (*KoreanAnalyzer)(nil)
var _ AnalyzerInterface = (*KoreanAnalyzer)(nil)

// KoreanAnalyzerFactory creates KoreanAnalyzer instances.
type KoreanAnalyzerFactory struct {
	stopWords *CharArraySet
}

// NewKoreanAnalyzerFactory creates a new KoreanAnalyzerFactory with default stop words.
func NewKoreanAnalyzerFactory() *KoreanAnalyzerFactory {
	return &KoreanAnalyzerFactory{
		stopWords: GetWordSetFromStrings(KoreanStopWords, true),
	}
}

// NewKoreanAnalyzerFactoryWithWords creates a new KoreanAnalyzerFactory with custom stop words.
func NewKoreanAnalyzerFactoryWithWords(stopWords *CharArraySet) *KoreanAnalyzerFactory {
	return &KoreanAnalyzerFactory{
		stopWords: stopWords,
	}
}

// Create creates a new KoreanAnalyzer.
func (f *KoreanAnalyzerFactory) Create() AnalyzerInterface {
	return NewKoreanAnalyzerWithWords(f.stopWords)
}

// Ensure KoreanAnalyzerFactory implements AnalyzerFactory
var _ AnalyzerFactory = (*KoreanAnalyzerFactory)(nil)
